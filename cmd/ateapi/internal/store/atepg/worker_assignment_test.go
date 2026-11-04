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
	"errors"
	"testing"

	"go.ate.dev/substrate/api/ateapi/v1"
	"go.ate.dev/substrate/cmd/ateapi/internal/store"
	"go.ate.dev/substrate/internal/resources"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

// TestSaveWorker_RejectsAStaleWrite proves the precondition saveWorker states
// on top of the row lock its callers hold: a Worker read before someone else
// wrote it cannot overwrite that write.
func TestSaveWorker_RejectsAStaleWrite(t *testing.T) {
	requirePool(t)
	ctx := context.Background()

	p, err := Connect(ctx, testConnectConfig("public"))
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer p.pool.Close()
	defer p.Close()
	clearAll(t, p)

	created, err := p.CreateWorker(ctx, &ateapi.Worker{
		Metadata:        &ateapi.ResourceMetadata{Name: "stale-write-worker"},
		WorkerNamespace: "ns",
		WorkerPool:      "pool",
		WorkerPod:       "pod",
	})
	if err != nil {
		t.Fatalf("CreateWorker failed: %v", err)
	}

	// Move the stored Worker on, so the copy above is a version behind.
	if _, err := p.UpdateWorker(ctx, created.GetMetadata().GetName(), store.PreconditionFrom(created), func(toUpdate *ateapi.Worker) error {
		toUpdate.Labels = map[string]string{"tier": "batch"}
		return nil
	}); err != nil {
		t.Fatalf("UpdateWorker failed: %v", err)
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}
	defer tx.Rollback(ctx)
	if err := saveWorker(ctx, tx, created); !errors.Is(err, store.ErrVersionConflict) {
		t.Errorf("saveWorker() with a stale Worker = %v, want ErrVersionConflict", err)
	}
}

// TestSaveWorker_RejectsAVanishedWorker keeps a deleted row from being an
// update of nothing.
func TestSaveWorker_RejectsAVanishedWorker(t *testing.T) {
	requirePool(t)
	ctx := context.Background()

	p, err := Connect(ctx, testConnectConfig("public"))
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer p.pool.Close()
	defer p.Close()
	clearAll(t, p)

	created, err := p.CreateWorker(ctx, &ateapi.Worker{
		Metadata:        &ateapi.ResourceMetadata{Name: "vanished-worker"},
		WorkerNamespace: "ns",
		WorkerPool:      "pool",
		WorkerPod:       "pod",
	})
	if err != nil {
		t.Fatalf("CreateWorker failed: %v", err)
	}
	if _, err := p.DeleteWorker(ctx, created.GetMetadata().GetName(), store.DeletePreconditions{}); err != nil {
		t.Fatalf("DeleteWorker failed: %v", err)
	}

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}
	defer tx.Rollback(ctx)
	if err := saveWorker(ctx, tx, created); !errors.Is(err, store.ErrVersionConflict) {
		t.Errorf("saveWorker() on a deleted Worker = %v, want ErrVersionConflict", err)
	}
}

// TestBindActorToWorker_DropsUnknownWorkerFields checks that the bind reads the
// Worker the way every other read does: fields this binary does not know are
// dropped, so admit sees and the bind writes the same Worker UpdateWorker would.
func TestBindActorToWorker_DropsUnknownWorkerFields(t *testing.T) {
	s := setupPostgresPersistence(t)
	ctx := context.Background()
	createTestAtespace(t, s, "team-a")
	if _, err := s.CreateWorker(ctx, &ateapi.Worker{Metadata: &ateapi.ResourceMetadata{Name: "w"}}); err != nil {
		t.Fatalf("CreateWorker failed: %v", err)
	}
	// A field a newer replica could have written.
	unknown := protowire.AppendVarint(protowire.AppendTag(nil, 9999, protowire.VarintType), 1)
	if _, err := s.pool.Exec(ctx, `UPDATE workers SET proto = proto || $1 WHERE name = 'w'`, unknown); err != nil {
		t.Fatalf("planting an unknown field: %v", err)
	}
	actor, err := s.CreateActor(ctx, &ateapi.Actor{
		Metadata: &ateapi.ResourceMetadata{Atespace: "team-a", Name: "a"},
		Status:   &ateapi.ActorStatus{State: ateapi.ActorState_ACTOR_STATE_SUSPENDED},
	})
	if err != nil {
		t.Fatalf("CreateActor failed: %v", err)
	}
	assignment := &ateapi.ActorAssignment{
		Actor:    &ateapi.ObjectRef{Atespace: "team-a", Name: "a"},
		ActorUid: actor.GetMetadata().GetUid(),
	}
	var admitted *ateapi.Worker
	admit := func(w *ateapi.Worker) error { admitted = w; return nil }
	if _, err := s.BindActorToWorker(ctx, "w", assignment, admit, resources.ActorRefFromActor(actor), store.PreconditionFrom(actor), nil); err != nil {
		t.Fatalf("BindActorToWorker failed: %v", err)
	}
	if n := len(admitted.ProtoReflect().GetUnknown()); n != 0 {
		t.Errorf("admit saw %d bytes of unknown fields, want 0", n)
	}
	var raw []byte
	if err := s.pool.QueryRow(ctx, `SELECT proto FROM workers WHERE name = 'w'`).Scan(&raw); err != nil {
		t.Fatalf("reading worker row: %v", err)
	}
	stored := &ateapi.Worker{}
	if err := proto.Unmarshal(raw, stored); err != nil {
		t.Fatalf("unmarshaling worker row: %v", err)
	}
	if n := len(stored.ProtoReflect().GetUnknown()); n != 0 {
		t.Errorf("bind wrote back %d bytes of unknown fields, want 0", n)
	}
}
