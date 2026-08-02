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
	"testing"

	"github.com/agent-substrate/substrate/pkg/proto/ateapipb"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateCreateAtespaceRequest(t *testing.T) {
	valid := func(mutate func(*ateapipb.Atespace)) *ateapipb.CreateAtespaceRequest {
		a := &ateapipb.Atespace{
			Metadata: &ateapipb.ResourceMetadata{Atespace: "", Name: "as1"},
		}
		if mutate != nil {
			mutate(a)
		}
		return &ateapipb.CreateAtespaceRequest{Atespace: a}
	}

	tests := []struct {
		name string
		req  *ateapipb.CreateAtespaceRequest
		want field.ErrorList
	}{{
		"valid",
		valid(nil),
		nil,
	}, {
		"missing atespace",
		&ateapipb.CreateAtespaceRequest{},
		field.ErrorList{field.Required(field.NewPath("atespace"), "")},
	}, {
		"missing atespace.metadata",
		valid(func(a *ateapipb.Atespace) { a.Metadata = nil }),
		field.ErrorList{field.Required(field.NewPath("atespace", "metadata"), "")},
	}, {
		"metadata.atespace must be empty",
		valid(func(a *ateapipb.Atespace) { a.Metadata.Atespace = "ns1" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace", "metadata", "atespace"), "ns1", "")},
	}, {
		"missing metadata.name",
		valid(func(a *ateapipb.Atespace) { a.Metadata.Name = "" }),
		field.ErrorList{field.Required(field.NewPath("atespace", "metadata", "name"), "")},
	}, {
		"invalid metadata.name",
		valid(func(a *ateapipb.Atespace) { a.Metadata.Name = "Team_A" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace", "metadata", "name"), "Team_A", "")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, validateCreateAtespaceRequest(context.Background(), tt.req), tt.want)
		})
	}
}
