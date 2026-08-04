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

package controlapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/agent-substrate/substrate/cmd/ateapi/internal/store"
	"github.com/agent-substrate/substrate/internal/resources"
	"github.com/agent-substrate/substrate/pkg/proto/ateapipb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/operation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// actorMutableFields lists the Actor field paths a client may name in an
// UpdateActor update_mask.
var actorMutableFields = mutableFields[*ateapipb.Actor]{
	"worker_selector": func(dst, src *ateapipb.Actor) { dst.WorkerSelector = src.GetWorkerSelector() },
}

func (s *Service) UpdateActor(ctx context.Context, req *ateapipb.UpdateActorRequest) (*ateapipb.Actor, error) {
	if errs := validateUpdateActorRequest(ctx, req); len(errs) > 0 {
		return nil, toGRPCStatusError(errs)
	}
	in := req.GetActor()
	actorRef := resources.ActorRefFromActor(in)
	setSpanActorRefAttributes(ctx, actorRef)

	updated, err := s.persistence.UpdateActor(ctx, actorRef, func(dbActor *ateapipb.Actor) error {
		if err := store.CheckActorPrecondition(dbActor, in.GetMetadata().GetUid(), in.GetMetadata().GetVersion()); err != nil {
			return err
		}
<<<<<<< HEAD
		applyUpdateMask(dbActor, in, req.GetUpdateMask(), actorMutableFields)
		return nil
	})
||||||| parent of e9f9c4fe (Prefactor: UpdateActor() - set metadata in RPC layer)
		return nil, fmt.Errorf("while getting actor: %w", err)
	}

	// UID and version preconditions
	if uid := in.GetMetadata().GetUid(); uid != "" && uid != actor.GetMetadata().GetUid() {
		return nil, status.Errorf(codes.Aborted, "Actor %s has uid %s, not %s", actorRef, actor.GetMetadata().GetUid(), uid)
	}

	expectedVersion := actor.GetMetadata().GetVersion()
	if version := in.GetMetadata().GetVersion(); version != 0 {
		expectedVersion = version
	}

	applyUpdateMask(actor, in, req.GetUpdateMask(), actorMutableFields)

	updated, err := s.persistence.UpdateActor(ctx, actor, expectedVersion)
=======
		return nil, fmt.Errorf("while getting actor: %w", err)
	}

	// UID and version preconditions
	if uid := in.GetMetadata().GetUid(); uid != "" && uid != actor.GetMetadata().GetUid() {
		return nil, status.Errorf(codes.Aborted, "Actor %s has uid %s, not %s", actorRef, actor.GetMetadata().GetUid(), uid)
	}

	expectedVersion := actor.GetMetadata().GetVersion()
	if version := in.GetMetadata().GetVersion(); version != 0 {
		expectedVersion = version
	}
	actor.Metadata = newMetadataForUpdate(actor.GetMetadata())

	applyUpdateMask(actor, in, req.GetUpdateMask(), actorMutableFields)

	updated, err := s.persistence.UpdateActor(ctx, actor, expectedVersion)
>>>>>>> e9f9c4fe (Prefactor: UpdateActor() - set metadata in RPC layer)
	if err != nil {
		if errors.Is(err, store.ErrVersionConflict) {
			return nil, status.Error(codes.Aborted, "concurrent update conflict, please retry")
		}
		if errors.Is(err, store.ErrUIDConflict) {
			return nil, status.Errorf(codes.Aborted, "actor %s/%s not found with uid %s", in.GetMetadata().GetAtespace(), in.GetMetadata().GetName(), in.GetMetadata().GetUid())
		}
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "actor %s not found", actorRef)
		}
		return nil, fmt.Errorf("while updating actor: %w", err)
	}

	setSpanActorAttributes(ctx, updated)
	return updated, nil
}

func validateUpdateActorRequest(ctx context.Context, req *ateapipb.UpdateActorRequest) field.ErrorList {
	// Call the generated validation.
	op := operation.Operation{Type: operation.Update}
	return Validate_UpdateActorRequest(ctx, op, nil, req, nil)
}
