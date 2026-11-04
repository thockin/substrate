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
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (p *Persistence) CreateEgressPolicy(ctx context.Context, actorRef resources.ActorRef, policy *ateapi.EgressPolicy) (*ateapi.EgressPolicy, error) {
	dbPolicy := proto.Clone(policy).(*ateapi.EgressPolicy)
	setCreateMetadata(dbPolicy.Metadata)
	protoBytes, err := proto.Marshal(dbPolicy)
	if err != nil {
		return nil, fmt.Errorf("marshaling egress policy: %w", err)
	}
	_, err = p.pool.Exec(ctx, `
		INSERT INTO actor_egress_policies (atespace, actor_name, uid, version, proto)
		VALUES ($1, $2, $3, $4, $5)`, actorRef.Atespace, actorRef.Name, dbPolicy.GetMetadata().GetUid(), dbPolicy.GetMetadata().GetVersion(), protoBytes)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, store.ErrAlreadyExists
		}
		if isForeignKeyViolation(err) {
			return nil, store.ErrFailedPrecondition
		}
		return nil, fmt.Errorf("inserting egress policy for %s: %w", actorRef, err)
	}
	return dbPolicy, nil
}

func (p *Persistence) GetEgressPolicy(ctx context.Context, actorRef resources.ActorRef) (*ateapi.EgressPolicy, error) {
	return getEgressPolicyRow(ctx, p.pool, `
		SELECT uid, version, proto FROM actor_egress_policies
		WHERE atespace = $1 AND actor_name = $2`, actorRef.Atespace, actorRef.Name)
}

func (p *Persistence) UpdateEgressPolicy(ctx context.Context, actorRef resources.ActorRef, precondition store.Precondition, mutate func(*ateapi.EgressPolicy) error) (*ateapi.EgressPolicy, error) {
	if err := precondition.Validate(); err != nil {
		return nil, err
	}
	return updateRow(ctx, p.pool, rowUpdate[*ateapi.EgressPolicy]{
		what:         "egress policy for " + actorRef.String(),
		table:        "actor_egress_policies",
		where:        "atespace = $1 AND actor_name = $2",
		key:          []any{actorRef.Atespace, actorRef.Name},
		precondition: store.DeletePreconditions(precondition),
		mutate:       mutate,
	})
}

func (p *Persistence) DeleteEgressPolicy(ctx context.Context, actorRef resources.ActorRef, precondition store.DeletePreconditions) (*ateapi.EgressPolicy, error) {
	deleted, err := deleteRow[*ateapi.EgressPolicy](ctx, p.pool, rowDelete{
		what:         "egress policy for " + actorRef.String(),
		table:        "actor_egress_policies",
		where:        "atespace = $1 AND actor_name = $2",
		key:          []any{actorRef.Atespace, actorRef.Name},
		precondition: precondition,
	})
	if err != nil {
		return nil, err
	}
	deleted.Metadata.DeleteTime = timestamppb.Now()
	return deleted, nil
}

func getEgressPolicyRow(ctx context.Context, q querier, query string, args ...any) (*ateapi.EgressPolicy, error) {
	var uid string
	var version int64
	var protoBytes []byte
	if err := q.QueryRow(ctx, query, args...).Scan(&uid, &version, &protoBytes); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("getting egress policy: %w", err)
	}
	return unmarshalEgressPolicy(uid, version, protoBytes)
}

func unmarshalEgressPolicy(uid string, version int64, protoBytes []byte) (*ateapi.EgressPolicy, error) {
	policy := &ateapi.EgressPolicy{}
	if err := unmarshalStored(protoBytes, policy); err != nil {
		return nil, fmt.Errorf("unmarshaling egress policy: %w", err)
	}
	if err := validateProtoMetadataMatchesColumns("egress policy", policy.GetMetadata(), uid, version); err != nil {
		return nil, err
	}
	return policy, nil
}
