// Copyright 2026 The Agent Substrate Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package atepg

import (
	"context"
	"sync"
	"testing"

	"go.ate.dev/substrate/api/ateapi/v1"
	"go.ate.dev/substrate/cmd/ateapi/internal/authz"
	"go.ate.dev/substrate/cmd/ateapi/internal/store"
	"go.ate.dev/substrate/internal/installdefaults"
	"go.ate.dev/substrate/internal/resources"
	"google.golang.org/protobuf/proto"
)

// clearAll truncates every table so the next test starts from an empty store
// without paying for a fresh database. Nothing in production mass-deletes
// state, so the statement lives here rather than on Persistence.
func clearAll(t *testing.T, p *Persistence) {
	t.Helper()
	if _, err := p.pool.Exec(context.Background(), `TRUNCATE atespaces, global_access_policy, atespace_access_policies, actors, actor_egress_policies, actor_templates, tags, workers, worker_assignments, leases, worker_outbox, worker_outbox_trim, tuple, changelog`); err != nil {
		t.Fatalf("truncating tables: %v", err)
	}
}

func setupPostgresPersistence(t *testing.T) *Persistence {
	t.Helper()
	ctx := context.Background()
	p, err := NewPersistence(ctx, requirePool(t))
	if err != nil {
		t.Fatalf("NewPersistence failed: %v", err)
	}
	t.Cleanup(p.Close)
	clearAll(t, p)
	setTestPolicyManager(t, p)
	// Registered after Close, so it runs before it.
	t.Cleanup(func() { checkInvariants(t, p) })
	return p
}

// invariantsWaived holds the stores whose test corrupts rows on purpose.
var invariantsWaived sync.Map

// waiveInvariants skips the end-of-test invariant check for p, for a test that
// writes rows the store never would.
func waiveInvariants(p *Persistence) { invariantsWaived.Store(p, true) }

// checkInvariants fails t if the stored state breaks a rule that every store
// operation must keep, whatever the test did through the store's API:
//
//   - Each resource row's uid and version columns match its proto's metadata,
//     and its key columns match the proto's name and atespace.
//   - Each Worker's status.allocated is the sum of its assignments, so binds
//     and releases moved the total with the rows.
//   - The outbox holds at most one row per transaction, which the watch cursor
//     relies on.
//
// Running it after every test turns each test into a check of these rules.
func checkInvariants(t *testing.T, p *Persistence) {
	t.Helper()
	if _, waived := invariantsWaived.Load(p); waived || t.Failed() {
		return
	}
	ctx := context.Background()

	keyed := []struct {
		table, atespace, name string
		msg                   func() proto.Message
	}{
		{"atespaces", "''", "name", func() proto.Message { return &ateapi.Atespace{} }},
		{"actors", "atespace", "name", func() proto.Message { return &ateapi.Actor{} }},
		{"actor_templates", "atespace", "name", func() proto.Message { return &ateapi.ActorTemplate{} }},
		{"tags", "atespace", "name", func() proto.Message { return &ateapi.Tag{} }},
		{"workers", "''", "name", func() proto.Message { return &ateapi.Worker{} }},
		// Policies are named by the object they attach to, not by their row.
		{"actor_egress_policies", "NULL", "NULL", func() proto.Message { return &ateapi.EgressPolicy{} }},
		{"atespace_access_policies", "NULL", "NULL", func() proto.Message { return &ateapi.AccessPolicy{} }},
		{"global_access_policy", "NULL", "NULL", func() proto.Message { return &ateapi.AccessPolicy{} }},
	}
	for _, k := range keyed {
		rows, err := p.pool.Query(ctx, `SELECT `+k.atespace+`, `+k.name+`, uid, version, proto FROM `+k.table)
		if err != nil {
			t.Errorf("invariants: reading %s: %v", k.table, err)
			continue
		}
		for rows.Next() {
			var atespace, name *string
			var uid string
			var version int64
			var b []byte
			if err := rows.Scan(&atespace, &name, &uid, &version, &b); err != nil {
				t.Errorf("invariants: scanning %s: %v", k.table, err)
				break
			}
			m := k.msg()
			if err := proto.Unmarshal(b, m); err != nil {
				t.Errorf("invariants: %s row uid %s does not decode: %v", k.table, uid, err)
				continue
			}
			md := m.(interface {
				GetMetadata() *ateapi.ResourceMetadata
			}).GetMetadata()
			if md.GetUid() != uid || md.GetVersion() != version {
				t.Errorf("invariants: %s row has uid %s version %d, but its proto has uid %s version %d", k.table, uid, version, md.GetUid(), md.GetVersion())
			}
			if (atespace != nil && md.GetAtespace() != *atespace) || (name != nil && md.GetName() != *name) {
				t.Errorf("invariants: %s row %v/%v holds a proto named %s/%s", k.table, *atespace, *name, md.GetAtespace(), md.GetName())
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Errorf("invariants: reading %s: %v", k.table, err)
		}
	}

	// One default page covers every test's Workers and assignments.
	workers, err := p.ListWorkers(ctx, store.ListOptions{})
	if err != nil {
		t.Errorf("invariants: listing workers: %v", err)
		return
	}
	for _, w := range workers.Items {
		name := w.GetMetadata().GetName()
		assignments, err := p.ListWorkerAssignments(ctx, name, store.ListOptions{})
		if err != nil {
			t.Errorf("invariants: listing assignments of worker %s: %v", name, err)
			continue
		}
		if got, want := w.GetStatus().GetAllocated(), resources.SumAllocated(assignments.Items); !proto.Equal(got, want) {
			t.Errorf("invariants: worker %s has allocated %v, but its %d assignments sum to %v", name, got, len(assignments.Items), want)
		}
	}

	var dupXids int
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM (SELECT xid FROM worker_outbox GROUP BY xid HAVING count(*) > 1) d`).Scan(&dupXids); err != nil {
		t.Errorf("invariants: reading worker_outbox: %v", err)
	} else if dupXids > 0 {
		t.Errorf("invariants: %d transactions appended more than one outbox row", dupXids)
	}
}

// setTestPolicyManager gives p an OpenFGA-backed PolicyManager, as the server
// always does.
func setTestPolicyManager(t *testing.T, p *Persistence) {
	t.Helper()
	fgaServer, err := authz.NewOpenFGAServer(p.pool)
	if err != nil {
		t.Fatalf("NewOpenFGAServer failed: %v", err)
	}
	t.Cleanup(fgaServer.Close)
	_, policyManager, err := authz.New(t.Context(), p.pool, fgaServer, nil, authz.SystemPrincipals{
		ControllerSPIFFEID:       installdefaults.ControllerSPIFFEID(installdefaults.SystemNamespace),
		EgressGatewaySPIFFEIDs:   []string{installdefaults.EgressSPIFFEID(installdefaults.SystemNamespace)},
		IngressRouterSPIFFEID:    installdefaults.RouterSPIFFEID(installdefaults.SystemNamespace),
		AteletSPIFFEID:           installdefaults.AteletSPIFFEID(installdefaults.SystemNamespace),
		APIServerSidecarSPIFFEID: installdefaults.APIServerSPIFFEID(installdefaults.SystemNamespace),
	})
	if err != nil {
		t.Fatalf("authz.New failed: %v", err)
	}
	p.SetPolicyManager(policyManager)
}

func newTestAtespace(name string) *ateapi.Atespace {
	return &ateapi.Atespace{Metadata: &ateapi.ResourceMetadata{Name: name}}
}

func createTestAtespace(t *testing.T, s *Persistence, name string) {
	t.Helper()
	if _, err := s.CreateAtespace(context.Background(), newTestAtespace(name)); err != nil {
		t.Fatalf("CreateAtespace(%q) failed: %v", name, err)
	}
}
