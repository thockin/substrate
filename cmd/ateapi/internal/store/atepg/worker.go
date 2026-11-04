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
	"fmt"

	"github.com/jackc/pgx/v5"
	"go.ate.dev/substrate/api/ateapi/v1"
	"go.ate.dev/substrate/cmd/ateapi/internal/store"
	"google.golang.org/protobuf/proto"
)

func (p *Persistence) CreateWorker(ctx context.Context, worker *ateapi.Worker) (*ateapi.Worker, error) {
	name := worker.GetMetadata().GetName()
	return p.inWorkerTx(ctx, store.WorkerEventCreated, func(ctx context.Context, tx pgx.Tx) (*ateapi.Worker, error) {
		return createRow(ctx, tx, rowInsert{
			what:  "worker " + name,
			table: "workers",
			cols:  "name",
			key:   []any{name},
		}, worker)
	})
}

func (p *Persistence) GetWorker(ctx context.Context, name string) (*ateapi.Worker, error) {
	return getRow[*ateapi.Worker](ctx, p.pool, "worker "+name,
		`SELECT uid, version, proto FROM workers WHERE name = $1`, name)
}

// lockWorker reads a Worker and locks its row until tx ends. Every write to a
// Worker row takes this lock before it reads the row, so a writer always sees
// the latest committed Worker and nothing can change it before the writer
// commits. That is what serializes updates to a Worker's allocation, and lets
// admit and UpdateWorker's mutate act as a compare-and-set.
//
// Returns ErrNotFound if the row does not exist.
func lockWorker(ctx context.Context, tx pgx.Tx, name string) (*ateapi.Worker, error) {
	return getRow[*ateapi.Worker](ctx, tx, "worker "+name,
		`SELECT uid, version, proto FROM workers WHERE name = $1 FOR UPDATE`, name)
}

// writeLockedWorker writes worker over the row keyed name, at
// nextMetadata(stored, markForDeletion), where stored is the metadata
// lockWorker read, and sets worker.Metadata to what it wrote. The caller must hold the row lock from
// lockWorker in tx.
//
// The UPDATE is guarded on stored's uid and version. Under the lock the guard
// always holds, but it turns a caller that forgot the lock, or a row that is
// gone, into ErrVersionConflict instead of a lost update or a silent no-op.
func writeLockedWorker(ctx context.Context, tx pgx.Tx, name string, worker *ateapi.Worker, stored *ateapi.ResourceMetadata, markForDeletion bool) error {
	worker.Metadata = nextMetadata(stored, markForDeletion)
	protoBytes, err := proto.Marshal(worker)
	if err != nil {
		return fmt.Errorf("marshaling worker: %w", err)
	}
	tag, err := tx.Exec(ctx, `UPDATE workers SET version = $1, proto = $2 WHERE name = $3 AND uid = $4 AND version = $5`,
		worker.GetMetadata().GetVersion(), protoBytes, name, stored.GetUid(), stored.GetVersion())
	if err != nil {
		return fmt.Errorf("updating worker %s: %w", name, err)
	}
	if tag.RowsAffected() != 1 {
		return store.ErrVersionConflict
	}
	return nil
}

// UpdateWorker runs mutate against the Worker as locked by lockWorker, so a
// concurrent writer blocks on the row lock rather than interleaving. That makes
// a test inside mutate, such as an occupancy check, a compare-and-set. The
// test cannot be pushed into SQL, because the row stores an opaque proto.
func (p *Persistence) UpdateWorker(ctx context.Context, name string, precondition store.Precondition, mutate func(*ateapi.Worker) error) (*ateapi.Worker, error) {
	if err := precondition.Validate(); err != nil {
		return nil, err
	}
	return p.updateWorker(ctx, name, store.DeletePreconditions(precondition), mutate, false)
}

func (p *Persistence) MarkWorkerForDeletion(ctx context.Context, name string, precondition store.DeletePreconditions, mutate func(*ateapi.Worker) error) (*ateapi.Worker, error) {
	return p.updateWorker(ctx, name, precondition, mutate, true)
}

func (p *Persistence) updateWorker(ctx context.Context, name string, precondition store.DeletePreconditions, mutate func(*ateapi.Worker) error, markForDeletion bool) (*ateapi.Worker, error) {
	return p.inWorkerTx(ctx, store.WorkerEventUpdated, func(ctx context.Context, tx pgx.Tx) (*ateapi.Worker, error) {
		dbWorker, err := lockWorker(ctx, tx, name)
		if err != nil {
			return nil, err
		}
		if err := precondition.Check(dbWorker.GetMetadata()); err != nil {
			return nil, err
		}

		// mutate may edit anything it is given. Immutable fields are the
		// service layer's to enforce, via declarative validation.
		stored := proto.CloneOf(dbWorker.GetMetadata())
		if mutate != nil {
			if err := mutate(dbWorker); err != nil {
				return nil, err
			}
		}
		if err := writeLockedWorker(ctx, tx, name, dbWorker, stored, markForDeletion); err != nil {
			return nil, err
		}
		return dbWorker, nil
	})
}

func (p *Persistence) DeleteWorker(ctx context.Context, name string, precondition store.DeletePreconditions) (*ateapi.Worker, error) {
	return p.inWorkerTx(ctx, store.WorkerEventDeleted, func(ctx context.Context, tx pgx.Tx) (*ateapi.Worker, error) {
		// Locked rather than plainly read so the incarnation precondition was evaluated
		// against is the one the DELETE removes.
		deleted, err := lockWorker(ctx, tx, name)
		if err != nil {
			return nil, err
		}
		if err := precondition.Check(deleted.GetMetadata()); err != nil {
			return nil, err
		}
		// Binds and releases lock the Worker row first, so this sees every
		// assignment committed before the lock was taken.
		var assigned bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM worker_assignments WHERE worker_name = $1)`, name).Scan(&assigned); err != nil {
			return nil, fmt.Errorf("checking assignments of worker %s: %w", name, err)
		}
		if assigned {
			return nil, fmt.Errorf("%w: worker %s still holds assignments", store.ErrFailedPrecondition, name)
		}
		commandTag, err := tx.Exec(ctx, `DELETE FROM workers WHERE name = $1`, name)
		if err != nil {
			return nil, fmt.Errorf("deleting worker %s: %w", name, err)
		}
		if commandTag.RowsAffected() != 1 {
			return nil, fmt.Errorf("deleting worker %s affected %d rows, want 1", name, commandTag.RowsAffected())
		}
		return deleted, nil
	})
}

func (p *Persistence) ListWorkers(ctx context.Context, opts store.ListOptions) (store.ListResponse[*ateapi.Worker], error) {
	return listPage(ctx, p.pool, listQuery{
		what:     "workers",
		item:     "worker",
		kind:     kindWorker,
		idParts:  1,
		keyParts: 1,
		firstSQL: `
			SELECT name, proto FROM workers
			ORDER BY name
			LIMIT $1`,
		nextSQL: `
			SELECT name, proto FROM workers
			WHERE name > $1
			ORDER BY name
			LIMIT $2`,
	}, opts, decodeStored[*ateapi.Worker]())
}
