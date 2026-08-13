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
	"github.com/agent-substrate/substrate/pkg/proto/ateapipb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/operation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func (s *Service) CreateAtespace(ctx context.Context, req *ateapipb.CreateAtespaceRequest) (*ateapipb.Atespace, error) {
	if errs := validateCreateAtespaceRequest(ctx, req); len(errs) > 0 {
		return nil, toGRPCStatusError(errs)
	}

	name := req.GetAtespace().GetMetadata().GetName()
	atespace := &ateapipb.Atespace{
		Metadata: &ateapipb.ResourceMetadata{
			Name: name,
		},
	}
	stored, err := s.impl.CreateAtespace(ctx, atespace)
	if err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			return nil, status.Errorf(codes.AlreadyExists, "Atespace %s already exists", name)
		}
		return nil, fmt.Errorf("while recording atespace: %w", err)
	}

	return stored, nil
}

func validateCreateAtespaceRequest(ctx context.Context, req *ateapipb.CreateAtespaceRequest) field.ErrorList {
	// Call the generated validation.
	op := operation.Operation{Type: operation.Create}
	return Validate_CreateAtespaceRequest(ctx, op, nil, req, nil)
}
