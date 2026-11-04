// Copyright 2026 Google LLC
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
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	"go.ate.dev/substrate/api/ateapi/v1"
	"go.ate.dev/substrate/cmd/ateapi/internal/store"
)

// sqlRecorder is a pgx tracer that records the text of every statement.
type sqlRecorder struct {
	mu  sync.Mutex
	sql []string
}

func (r *sqlRecorder) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sql = append(r.sql, data.SQL)
	return ctx
}

func (r *sqlRecorder) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// TestList_GenericPlansSeekToTheCursor checks that every page of every listing
// can seek straight to its cursor even under a generic plan, which PostgreSQL
// may cache for a prepared statement and reuse for any arguments. A plan that
// filters rows instead reads the whole range before the cursor on every page,
// so a full listing costs time quadratic in its length.
func TestList_GenericPlansSeekToTheCursor(t *testing.T) {
	requirePool(t)
	ctx := context.Background()
	schema := "list-generic-plans"
	pool := migratedPool(t, schema)
	recorder := &sqlRecorder{}
	p, err := NewPersistence(ctx, openPool(t, schema, recorder))
	if err != nil {
		t.Fatalf("NewPersistence failed: %v", err)
	}
	t.Cleanup(p.Close)
	setTestPolicyManager(t, p)

	// Two of everything, so a one-item page always has a next page.
	for _, atespace := range []string{"team-a", "team-b"} {
		createTestAtespace(t, p, atespace)
	}
	for _, name := range []string{"x1", "x2"} {
		createTestActorTemplate(t, p, "team-a", name)
		createTestTag(t, p, "team-a", name)
		if _, err := p.CreateActor(ctx, &ateapi.Actor{
			Metadata: &ateapi.ResourceMetadata{Atespace: "team-a", Name: name},
			Status:   &ateapi.ActorStatus{State: ateapi.ActorState_ACTOR_STATE_SUSPENDED},
		}); err != nil {
			t.Fatalf("CreateActor failed: %v", err)
		}
		if _, err := p.CreateWorker(ctx, &ateapi.Worker{Metadata: &ateapi.ResourceMetadata{Name: name}}); err != nil {
			t.Fatalf("CreateWorker failed: %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO worker_assignments (actor_uid, worker_name, proto) VALUES ($1, 'x1', '')`, name); err != nil {
			t.Fatalf("inserting assignment: %v", err)
		}
	}

	// Walk every listing one item per page, so each runs its first-page and
	// next-page statements.
	walk := func(name string, list func(store.ListOptions) (string, error)) {
		next, err := list(store.ListOptions{PageSize: 1})
		if err != nil {
			t.Fatalf("%s failed: %v", name, err)
		}
		if next == "" {
			t.Fatalf("%s returned no next page", name)
		}
		if _, err := list(store.ListOptions{PageSize: 1, PageToken: next}); err != nil {
			t.Fatalf("%s of the second page failed: %v", name, err)
		}
	}
	walk("ListAtespaces", func(o store.ListOptions) (string, error) {
		r, err := p.ListAtespaces(ctx, o)
		return r.NextPageToken, err
	})
	walk("ListWorkers", func(o store.ListOptions) (string, error) {
		r, err := p.ListWorkers(ctx, o)
		return r.NextPageToken, err
	})
	walk("ListWorkerAssignments", func(o store.ListOptions) (string, error) {
		r, err := p.ListWorkerAssignments(ctx, "x1", o)
		return r.NextPageToken, err
	})
	for _, atespace := range []string{"team-a", ""} {
		walk("ListActors", func(o store.ListOptions) (string, error) {
			r, err := p.ListActors(ctx, atespace, o)
			return r.NextPageToken, err
		})
		walk("ListActorTemplates", func(o store.ListOptions) (string, error) {
			r, err := p.ListActorTemplates(ctx, atespace, o)
			return r.NextPageToken, err
		})
		walk("ListTags", func(o store.ListOptions) (string, error) {
			r, err := p.ListTags(ctx, atespace, o)
			return r.NextPageToken, err
		})
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	defer conn.Release()
	// The tables are tiny, so make the planner show the index plan it would
	// pick for a large one.
	if _, err := conn.Exec(ctx, `SET enable_seqscan = off`); err != nil {
		t.Fatalf("disabling sequential scans: %v", err)
	}
	listings := 0
	seen := map[string]bool{}
	for _, sql := range recorder.sql {
		sql = normalizeSQL(sql)
		if !strings.Contains(sql, "proto FROM") || !strings.Contains(sql, "ORDER BY") || seen[sql] {
			continue
		}
		seen[sql] = true
		listings++
		// Sent as a simple query, because pgx would otherwise demand values
		// for the statement's parameters.
		results, err := conn.Conn().PgConn().Exec(ctx, `EXPLAIN (GENERIC_PLAN, FORMAT JSON) `+sql).ReadAll()
		if err != nil {
			t.Fatalf("explaining %q: %v", sql, err)
		}
		plan := string(results[0].Rows[0][0])
		// worker_assignments has no index on (worker_name, actor_uid), so its
		// cursor is always a filter over one Worker's assignments. A Worker
		// hosts few Actors, so that scan stays small.
		if strings.Contains(plan, `"Filter"`) && !strings.Contains(sql, "FROM worker_assignments") {
			t.Errorf("generic plan of %q filters rows instead of seeking:\n%s", sql, plan)
		}
	}
	// Each of the nine listings has a first-page and a next-page statement.
	if listings != 18 {
		t.Errorf("checked %d listing statements, want 18", listings)
	}
}
