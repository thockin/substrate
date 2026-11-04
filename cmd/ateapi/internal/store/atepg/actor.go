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
)

func (p *Persistence) CreateActor(ctx context.Context, actor *ateapi.Actor) (*ateapi.Actor, error) {
	ref := resources.ActorRefFromActor(actor)
	if err := validateSnapshotURIs("actor "+ref.String(), actor.GetStatus().GetSnapshots()...); err != nil {
		return nil, err
	}
	// TODO: createRow clones the actor, which is wasteful: callers build it to
	// pass in, so it could be stored in place. Some contract tests rely on the
	// input staying unchanged.
	return createRow(ctx, p.pool, rowInsert{
		what:  "actor " + ref.String(),
		table: "actors",
		cols:  "atespace, name",
		key:   []any{ref.Atespace, ref.Name},
	}, actor)
}

func (p *Persistence) GetActor(ctx context.Context, actorRef resources.ActorRef) (*ateapi.Actor, error) {
	return getRow[*ateapi.Actor](ctx, p.pool, "actor "+actorRef.String(),
		`SELECT uid, version, proto FROM actors WHERE atespace = $1 AND name = $2`, actorRef.Atespace, actorRef.Name)
}

func (p *Persistence) UpdateActor(ctx context.Context, actorRef resources.ActorRef, precondition store.Precondition, mutate func(*ateapi.Actor) error) (*ateapi.Actor, error) {
	if err := precondition.Validate(); err != nil {
		return nil, err
	}
	return updateActor(ctx, p.pool, actorRef, store.DeletePreconditions(precondition), mutate, false)
}

func (p *Persistence) MarkActorForDeletion(ctx context.Context, actorRef resources.ActorRef, precondition store.DeletePreconditions, mutate func(*ateapi.Actor) error) (*ateapi.Actor, error) {
	return updateActor(ctx, p.pool, actorRef, precondition, mutate, true)
}

// updateActor applies mutate to the stored actor through q, which is the pool
// or a transaction the write must commit with.
func updateActor(ctx context.Context, q querier, actorRef resources.ActorRef, precondition store.DeletePreconditions, mutate func(*ateapi.Actor) error, markForDeletion bool) (*ateapi.Actor, error) {
	what := "actor " + actorRef.String()
	return updateRow(ctx, q, rowUpdate[*ateapi.Actor]{
		what:         what,
		table:        "actors",
		where:        "atespace = $1 AND name = $2",
		key:          []any{actorRef.Atespace, actorRef.Name},
		precondition: precondition,
		mutate:       mutate,
		// TODO: this should be done through DV and removed from here
		validate: func(mutated *ateapi.Actor) error {
			return validateSnapshotURIs(what, mutated.GetStatus().GetSnapshots()...)
		},
		markForDeletion: markForDeletion,
	})
}

func (p *Persistence) DeleteActor(ctx context.Context, actorRef resources.ActorRef, precondition store.DeletePreconditions) (*ateapi.Actor, error) {
	atespace, name := actorRef.Atespace, actorRef.Name
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning actor delete: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	// Locked so the Actor checked is the one the DELETE removes.
	var protoBytes []byte
	err = tx.QueryRow(ctx, `SELECT proto FROM actors WHERE atespace = $1 AND name = $2 FOR UPDATE`, atespace, name).Scan(&protoBytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("locking actor %s/%s: %w", atespace, name, err)
	}
	deleted := &ateapi.Actor{}
	if err := unmarshalStored(protoBytes, deleted); err != nil {
		return nil, fmt.Errorf("unmarshaling actor to delete: %w", err)
	}
	if err := precondition.Check(deleted.GetMetadata()); err != nil {
		return nil, err
	}
	if deleted.GetStatus().GetWorkerAssignment() != nil {
		return nil, fmt.Errorf("%w: actor %s/%s is still assigned to a worker", store.ErrFailedPrecondition, atespace, name)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM actors WHERE atespace = $1 AND name = $2`, atespace, name); err != nil {
		return nil, fmt.Errorf("deleting actor %s/%s: %w", atespace, name, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing actor delete %s/%s: %w", atespace, name, err)
	}
	return deleted, nil
}

func (p *Persistence) ListActors(ctx context.Context, atespace string, opts store.ListOptions) (store.ListResponse[*ateapi.Actor], error) {
	lq := listQuery{
		what:     "actors",
		item:     "actor",
		kind:     kindActor,
		idParts:  2,
		keyParts: 2,
		firstSQL: `
			SELECT atespace, name, proto FROM actors
			ORDER BY atespace, name
			LIMIT $1`,
		nextSQL: `
			SELECT atespace, name, proto FROM actors
			WHERE (atespace, name) > ($1, $2)
			ORDER BY atespace, name
			LIMIT $3`,
	}
	if atespace != "" {
		lq = listQuery{
			what:     fmt.Sprintf("actors in %q", atespace),
			item:     "actor",
			kind:     kindActor,
			scope:    atespace,
			idParts:  2,
			keyParts: 1,
			firstSQL: `
				SELECT atespace, name, proto FROM actors
				WHERE atespace = $1
				ORDER BY name
				LIMIT $2`,
			nextSQL: `
				SELECT atespace, name, proto FROM actors
				WHERE atespace = $1 AND name > $2
				ORDER BY name
				LIMIT $3`,
			args: []any{atespace},
		}
	}
	return listPage(ctx, p.pool, lq, opts, decodeStored[*ateapi.Actor]())
}
