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
	"fmt"

	"github.com/jackc/pgx/v5"
	"go.ate.dev/substrate/api/ateapi/v1"
	"go.ate.dev/substrate/cmd/ateapi/internal/store"
	"go.ate.dev/substrate/internal/resources"
	"google.golang.org/protobuf/proto"
)

func (p *Persistence) BindActorToWorker(ctx context.Context, workerName string, assignment *ateapi.ActorAssignment, admit func(*ateapi.Worker) error, actorRef resources.ActorRef, precondition store.Precondition, mutate func(*ateapi.Actor) error) (*ateapi.Actor, error) {
	actorUID := assignment.GetActorUid()
	if actorUID == "" {
		return nil, fmt.Errorf("binding an assignment with no actor_uid to worker %s", workerName)
	}
	if err := precondition.Validate(); err != nil {
		return nil, err
	}
	if precondition.UID != actorUID {
		return nil, fmt.Errorf("binding actor %s to worker %s under a precondition for actor %s", actorUID, workerName, precondition.UID)
	}
	// The store assigns identity. atespace is empty because Workers are
	// global-scoped; the name is the Actor's UID, which is also the row key.
	assignment.Metadata = newMetadata(&ateapi.ResourceMetadata{Name: actorUID})
	assignmentBytes, err := proto.Marshal(assignment)
	if err != nil {
		return nil, fmt.Errorf("marshaling assignment: %w", err)
	}

	var updated *ateapi.Actor
	_, err = p.inWorkerTx(ctx, store.WorkerEventUpdated, func(ctx context.Context, tx pgx.Tx) (*ateapi.Worker, error) {
		worker, err := lockWorker(ctx, tx, workerName)
		if errors.Is(err, store.ErrNotFound) {
			return nil, store.ErrWorkerNotFound
		}
		if err != nil {
			return nil, err
		}
		if worker.Status == nil {
			worker.Status = &ateapi.WorkerStatus{}
		}

		// The row lock holds the answer until commit.
		if admit != nil {
			if err := admit(worker); err != nil {
				return nil, err
			}
		}
		// The Actor's version check settles concurrent binds of it: every bind
		// updates the Actor, so all but the first fail it.
		if updated, err = updateActor(ctx, tx, actorRef, store.DeletePreconditions(precondition), mutate, false); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO worker_assignments (actor_uid, worker_name, proto)
			VALUES ($1, $2, $3)`,
			actorUID, workerName, assignmentBytes); err != nil {
			return nil, fmt.Errorf("binding actor %s to worker %s: %w", actorUID, workerName, err)
		}
		worker.Status.Allocated = resources.AddToAllocated(worker.Status.Allocated, assignment, +1)
		if err := writeLockedWorker(ctx, tx, workerName, worker, worker.GetMetadata(), false); err != nil {
			return nil, err
		}
		return worker, nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (p *Persistence) UpdateActorAndRelease(ctx context.Context, actorRef resources.ActorRef, precondition store.Precondition, mutate func(*ateapi.Actor) error) (*ateapi.Actor, error) {
	if err := precondition.Validate(); err != nil {
		return nil, err
	}
	actorUID := precondition.UID
	var updated *ateapi.Actor
	_, err := p.inWorkerTx(ctx, store.WorkerEventUpdated, func(ctx context.Context, tx pgx.Tx) (*ateapi.Worker, error) {
		released, err := releaseAssignment(ctx, tx, actorUID)
		if err != nil {
			return nil, err
		}
		// A row released since the caller read the Actor also moved the
		// Actor on, so its version check fails here.
		if updated, err = updateActor(ctx, tx, actorRef, store.DeletePreconditions(precondition), mutate, false); err != nil {
			return nil, err
		}
		return released, nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// releaseAssignment deletes actorUID's assignment, wherever it is, and returns the
// Worker that held it as saved with the assignment no longer booked, or nil if
// no Worker held it. The Worker is locked before its assignment row, as
// BindActorToWorker locks it, so the first read only finds which Worker to
// lock.
func releaseAssignment(ctx context.Context, tx pgx.Tx, actorUID string) (*ateapi.Worker, error) {
	_, workerName, err := getAssignment(ctx, tx, actorUID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	worker, err := lockWorker(ctx, tx, workerName)
	if errors.Is(err, store.ErrNotFound) {
		// A Worker is deleted only once it holds no assignments, so the row
		// was released after the first read.
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var protoBytes []byte
	err = tx.QueryRow(ctx, `
		DELETE FROM worker_assignments
		WHERE actor_uid = $1 AND worker_name = $2
		RETURNING proto`, actorUID, workerName).Scan(&protoBytes)
	if errors.Is(err, pgx.ErrNoRows) {
		// Released after the first read.
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("releasing actor %s from worker %s: %w", actorUID, workerName, err)
	}
	assignment, err := unmarshalAssignment(protoBytes)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling released assignment: %w", err)
	}
	worker.Status.Allocated = resources.AddToAllocated(worker.Status.Allocated, assignment, -1)
	if err := writeLockedWorker(ctx, tx, workerName, worker, worker.GetMetadata(), false); err != nil {
		return nil, err
	}
	return worker, nil
}

// getAssignment reads the assignment for actorUID and names the worker
// holding it.
func getAssignment(ctx context.Context, q querier, actorUID string) (*ateapi.ActorAssignment, string, error) {
	var (
		protoBytes []byte
		workerName string
	)
	err := q.QueryRow(ctx, `SELECT proto, worker_name FROM worker_assignments WHERE actor_uid = $1`, actorUID).
		Scan(&protoBytes, &workerName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", store.ErrNotFound
		}
		return nil, "", fmt.Errorf("getting assignment for actor %s: %w", actorUID, err)
	}
	assignment, err := unmarshalAssignment(protoBytes)
	if err != nil {
		return nil, "", fmt.Errorf("unmarshaling assignment: %w", err)
	}
	return assignment, workerName, nil
}

// unmarshalAssignment decodes a stored assignment, dropping fields this binary
// does not know, as unmarshalStored does for resources. Assignments carry no
// defaults, so defaults.Apply does not apply.
func unmarshalAssignment(b []byte) (*ateapi.ActorAssignment, error) {
	assignment := &ateapi.ActorAssignment{}
	if err := (proto.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(b, assignment); err != nil {
		return nil, err
	}
	return assignment, nil
}

func (p *Persistence) GetWorkerAssignment(ctx context.Context, workerName, actorUID string) (*ateapi.ActorAssignment, error) {
	assignment, holder, err := getAssignment(ctx, p.pool, actorUID)
	if err != nil {
		return nil, err
	}
	if holder != workerName {
		return nil, store.ErrNotFound
	}
	return assignment, nil
}

func (p *Persistence) ListWorkerAssignments(ctx context.Context, workerName string, opts store.ListOptions) (store.ListResponse[*ateapi.ActorAssignment], error) {
	return listPage(ctx, p.pool, listQuery{
		what:     "assignments of worker " + workerName,
		item:     "actor",
		kind:     kindWorkerAssign,
		scope:    workerName,
		idParts:  1,
		keyParts: 1,
		firstSQL: `
			SELECT actor_uid, proto FROM worker_assignments
			WHERE worker_name = $1
			ORDER BY actor_uid
			LIMIT $2`,
		nextSQL: `
			SELECT actor_uid, proto FROM worker_assignments
			WHERE worker_name = $1 AND actor_uid > $2
			ORDER BY actor_uid
			LIMIT $3`,
		args: []any{workerName},
	}, opts, unmarshalAssignment)
}
