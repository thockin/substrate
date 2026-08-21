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

	"github.com/agent-substrate/substrate/pkg/proto/ateapipb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/api/operation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func toGRPCStatusError(errs field.ErrorList) error {
	return status.Error(codes.InvalidArgument, errs.ToAggregate().Error())
}

func toGRPCInternalError(errs field.ErrorList) error {
	return status.Error(codes.Internal, errs.ToAggregate().Error())
}

func protoDeepEqual[T any](a, b T) bool {
	pa, ok := any(a).(proto.Message)
	if !ok {
		panic("protoDeepEqual: a is not a proto.Message")
	}
	pb, ok := any(b).(proto.Message)
	if !ok {
		panic("protoDeepEqual: b is not a proto.Message")
	}
	return proto.Equal(pa, pb)
}

// This exists only because nested subfield tags are not supported yet.
func ValidateCustom_UpdateActorRequest_Actor(ctx context.Context, op operation.Operation, fldPath *field.Path, actor, _ *ateapipb.Actor) field.ErrorList {
	if actor == nil || actor.Metadata == nil {
		return nil // handled by DV
	}

	// TODO: Once we drop the fieldmask, we can do a full validation of the
	// input actor and don't need this.
	errs := Validate_ResourceMetadata(ctx, op, fldPath.Child("metadata"), actor.Metadata, nil)

	// This is an update request, so the UID and version must be set.  When
	// those become optional or when we have nested subfield tags, we can drop
	// this.
	if actor.Metadata.Uid == "" {
		errs = append(errs, field.Required(field.NewPath("actor", "metadata", "uid"), ""))
	}
	if actor.Metadata.Version == 0 {
		errs = append(errs, field.Required(field.NewPath("actor", "metadata", "version"), ""))
	}
	return errs
}
