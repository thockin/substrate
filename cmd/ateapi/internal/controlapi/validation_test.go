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
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/agent-substrate/substrate/pkg/proto/ateapipb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/apimachinery/pkg/api/operation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateResourceMetadata(t *testing.T) {
	valid := func(mutate func(*ateapipb.ResourceMetadata)) *ateapipb.ResourceMetadata {
		m := &ateapipb.ResourceMetadata{
			Atespace:   "as",
			Name:       "nm",
			Uid:        "01234567-89ab-cdef-0123-456789abcdef",
			Version:    93,
			CreateTime: &timestamppb.Timestamp{Seconds: 867},
			UpdateTime: &timestamppb.Timestamp{Seconds: 5309},
		}
		if mutate != nil {
			mutate(m)
		}
		return m
	}

	tests := []struct {
		name string
		req  *ateapipb.ResourceMetadata
		want field.ErrorList
	}{{
		"valid",
		valid(nil),
		nil,
	}, {
		"valid atespace: empty",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = "" }),
		nil,
	}, {
		"valid atespace: alphabetic",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = "myatespace" }),
		nil,
	}, {
		"valid atespace: dashes",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = "my-ate-space" }),
		nil,
	}, {
		"valid atespace: repeat dashes",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = "my---ate---space" }),
		nil,
	}, {
		"valid atespace: alphanumeric",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = "my-123-atespace" }),
		nil,
	}, {
		"valid atespace: leading numeric",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = "123-atespace" }),
		nil,
	}, {
		"valid atespace: trailing numeric",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = "my-123" }),
		nil,
	}, {
		"valid atespace: fully numeric",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = "123" }),
		nil,
	}, {
		"valid atespace: long",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = strings.Repeat("x", 63) }),
		nil,
	}, {
		"invalid atespace: uppercase",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = "MYATESPACE" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: leading dash",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = "-atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: trailing dash",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = "my-" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: dots",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = "my.atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: underscores",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = "my_atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: bang",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = "my!atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: at",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = "my@atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: pound",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = "my#atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: dollar",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = "my$atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: percent",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = "my%%atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: caret",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = "my^atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: ampersand",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = "my&atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: star",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = "my*atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: too long",
		valid(func(m *ateapipb.ResourceMetadata) { m.Atespace = strings.Repeat("x", 64) }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"missing name",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = "" }),
		field.ErrorList{field.Required(field.NewPath("name"), "")},
	}, {
		"valid name: alphabetic",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = "myname" }),
		nil,
	}, {
		"valid name: dashes",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = "my-na-me" }),
		nil,
	}, {
		"valid name: repeat dashes",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = "my---na---me" }),
		nil,
	}, {
		"valid name: alphanumeric",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = "my-123-name" }),
		nil,
	}, {
		"invalid name: leading numeric",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = "123-name" }),
		nil,
	}, {
		"invalid name: trailing numeric",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = "my-123" }),
		nil,
	}, {
		"invalid name: fully numeric",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = "123" }),
		nil,
	}, {
		"valid name: long",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = strings.Repeat("x", 63) }),
		nil,
	}, {
		"invalid name: uppercase",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = "MYNAME" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: leading dash",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = "-name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: trailing dash",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = "my-" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: dots",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = "my.name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: underscores",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = "my_name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: bang",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = "my!name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: at",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = "my@name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: pound",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = "my#name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: dollar",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = "my$name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: percent",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = "my%%name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: caret",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = "my^name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: ampersand",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = "my&name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: star",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = "my*name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: too long",
		valid(func(m *ateapipb.ResourceMetadata) { m.Name = strings.Repeat("x", 64) }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"unspecified uid",
		valid(func(m *ateapipb.ResourceMetadata) { m.Uid = "" }),
		nil,
	}, {
		"invalid uid: close but not valid",
		valid(func(m *ateapipb.ResourceMetadata) { m.Uid = "aaaaaaaa-bbbbcccc-dddd-eeeeeeeeeeee" }),
		field.ErrorList{field.Invalid(field.NewPath("uid"), nil, "").WithOrigin("format=k8s-uuid")},
	}, {
		"invalid uid: not even close",
		valid(func(m *ateapipb.ResourceMetadata) { m.Uid = "not a uid" }),
		field.ErrorList{field.Invalid(field.NewPath("uid"), nil, "").WithOrigin("format=k8s-uuid")},
	}, {
		"unspecified version",
		valid(func(m *ateapipb.ResourceMetadata) { m.Version = 0 }),
		nil,
	}, {
		"valid version: large",
		valid(func(m *ateapipb.ResourceMetadata) { m.Version = math.MaxInt64 }),
		nil,
	}, {
		"invalid version: negative",
		valid(func(m *ateapipb.ResourceMetadata) { m.Version = -1 }),
		field.ErrorList{field.Invalid(field.NewPath("version"), nil, "").WithOrigin("minimum")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := operation.Operation{Type: operation.Create}
			matcher := field.ErrorMatcher{}.ByType().ByField().ByOrigin()
			matcher.Test(t, tt.want, Validate_ResourceMetadata(context.Background(), op, nil, tt.req, nil))
		})
	}
}

func TestValidateObjectRef(t *testing.T) {
	valid := func(mutate func(*ateapipb.ObjectRef)) *ateapipb.ObjectRef {
		r := &ateapipb.ObjectRef{
			Atespace: "as",
			Name:     "nm",
		}
		if mutate != nil {
			mutate(r)
		}
		return r
	}

	tests := []struct {
		name string
		ref  *ateapipb.ObjectRef
		want field.ErrorList
	}{{
		"valid",
		valid(nil),
		nil,
	}, {
		"valid atespace: empty",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = "" }),
		nil,
	}, {
		"valid atespace: alphabetic",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = "myatespace" }),
		nil,
	}, {
		"valid atespace: dashes",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = "my-ate-space" }),
		nil,
	}, {
		"valid atespace: repeat dashes",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = "my---ate---space" }),
		nil,
	}, {
		"valid atespace: alphanumeric",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = "my-123-atespace" }),
		nil,
	}, {
		"valid atespace: leading numeric",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = "123-atespace" }),
		nil,
	}, {
		"valid atespace: trailing numeric",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = "my-123" }),
		nil,
	}, {
		"valid atespace: fully numeric",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = "123" }),
		nil,
	}, {
		"valid atespace: long",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = strings.Repeat("x", 63) }),
		nil,
	}, {
		"invalid atespace: uppercase",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = "MYATESPACE" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: leading dash",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = "-atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: trailing dash",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = "my-" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: dots",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = "my.atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: underscores",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = "my_atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: bang",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = "my!atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: at",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = "my@atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: pound",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = "my#atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: dollar",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = "my$atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: percent",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = "my%%atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: caret",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = "my^atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: ampersand",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = "my&atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: star",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = "my*atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: too long",
		valid(func(r *ateapipb.ObjectRef) { r.Atespace = strings.Repeat("x", 64) }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"missing name",
		valid(func(r *ateapipb.ObjectRef) { r.Name = "" }),
		field.ErrorList{field.Required(field.NewPath("name"), "")},
	}, {
		"valid name: alphabetic",
		valid(func(r *ateapipb.ObjectRef) { r.Name = "myname" }),
		nil,
	}, {
		"valid name: dashes",
		valid(func(r *ateapipb.ObjectRef) { r.Name = "my-na-me" }),
		nil,
	}, {
		"valid name: repeat dashes",
		valid(func(r *ateapipb.ObjectRef) { r.Name = "my---na---me" }),
		nil,
	}, {
		"valid name: alphanumeric",
		valid(func(r *ateapipb.ObjectRef) { r.Name = "my-123-name" }),
		nil,
	}, {
		"invalid name: leading numeric",
		valid(func(r *ateapipb.ObjectRef) { r.Name = "123-name" }),
		nil,
	}, {
		"invalid name: trailing numeric",
		valid(func(r *ateapipb.ObjectRef) { r.Name = "my-123" }),
		nil,
	}, {
		"invalid name: fully numeric",
		valid(func(r *ateapipb.ObjectRef) { r.Name = "123" }),
		nil,
	}, {
		"valid name: long",
		valid(func(r *ateapipb.ObjectRef) { r.Name = strings.Repeat("x", 63) }),
		nil,
	}, {
		"invalid name: uppercase",
		valid(func(r *ateapipb.ObjectRef) { r.Name = "MYNAME" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: leading dash",
		valid(func(r *ateapipb.ObjectRef) { r.Name = "-name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: trailing dash",
		valid(func(r *ateapipb.ObjectRef) { r.Name = "my-" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: dots",
		valid(func(r *ateapipb.ObjectRef) { r.Name = "my.name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: underscores",
		valid(func(r *ateapipb.ObjectRef) { r.Name = "my_name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: bang",
		valid(func(r *ateapipb.ObjectRef) { r.Name = "my!name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: at",
		valid(func(r *ateapipb.ObjectRef) { r.Name = "my@name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: pound",
		valid(func(r *ateapipb.ObjectRef) { r.Name = "my#name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: dollar",
		valid(func(r *ateapipb.ObjectRef) { r.Name = "my$name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: percent",
		valid(func(r *ateapipb.ObjectRef) { r.Name = "my%%name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: caret",
		valid(func(r *ateapipb.ObjectRef) { r.Name = "my^name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: ampersand",
		valid(func(r *ateapipb.ObjectRef) { r.Name = "my&name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: star",
		valid(func(r *ateapipb.ObjectRef) { r.Name = "my*name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: too long",
		valid(func(r *ateapipb.ObjectRef) { r.Name = strings.Repeat("x", 64) }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := operation.Operation{Type: operation.Create}
			matcher := field.ErrorMatcher{}.ByType().ByField().ByOrigin()
			matcher.Test(t, tt.want, Validate_ObjectRef(context.Background(), op, nil, tt.ref, nil))
		})
	}
}

func TestValidateAtespace(t *testing.T) {
	tests := []struct {
		name string
		req  *ateapipb.Atespace
		want field.ErrorList
	}{{
		"valid",
		validAtespace(nil),
		nil,
	}, {
		"missing metadata",
		validAtespace(func(a *ateapipb.Atespace) { a.Metadata = nil }),
		field.ErrorList{field.Required(field.NewPath("metadata"), "")},
	}, {
		"metadata.atespace must be empty",
		validAtespace(func(a *ateapipb.Atespace) { a.Metadata.Atespace = "ns1" }),
		field.ErrorList{field.Forbidden(field.NewPath("metadata", "atespace"), "")},
	}, {
		"missing metadata.name",
		validAtespace(func(a *ateapipb.Atespace) { a.Metadata.Name = "" }),
		field.ErrorList{field.Required(field.NewPath("metadata", "name"), "")},
	}, {
		"invalid metadata.name",
		validAtespace(func(a *ateapipb.Atespace) { a.Metadata.Name = "Team_A" }),
		field.ErrorList{field.Invalid(field.NewPath("metadata", "name"), "", "").WithOrigin("format=k8s-short-name")},
	}, {
		"too-long metadata.name",
		validAtespace(func(a *ateapipb.Atespace) { a.Metadata.Name = strings.Repeat("x", 64) }),
		field.ErrorList{field.Invalid(field.NewPath("metadata", "name"), "", "").WithOrigin("format=k8s-short-name")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, validateAtespace(context.Background(), tt.req), tt.want)
		})
	}
}

func TestValidateActor(t *testing.T) {
	tests := []struct {
		name string
		line string
		req  *ateapipb.Actor
		want field.ErrorList
	}{{
		"valid",
		line(),
		validActor(nil),
		nil,
	}, {
		"missing metadata",
		line(),
		validActor(func(a *ateapipb.Actor) { a.Metadata = nil }),
		field.ErrorList{field.Required(field.NewPath("metadata"), "")},
	}, {
		"missing metadata.atespace",
		line(),
		validActor(func(a *ateapipb.Actor) { a.Metadata.Atespace = "" }),
		field.ErrorList{field.Required(field.NewPath("metadata", "atespace"), "")},
	}, {
		"invalid metadata.atespace",
		line(),
		validActor(func(a *ateapipb.Actor) { a.Metadata.Atespace = "NS1" }),
		field.ErrorList{field.Invalid(field.NewPath("metadata", "atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"missing metadata.name",
		line(),
		validActor(func(a *ateapipb.Actor) { a.Metadata.Name = "" }),
		field.ErrorList{field.Required(field.NewPath("metadata", "name"), "")},
	}, {
		"invalid metadata.name",
		line(),
		validActor(func(a *ateapipb.Actor) { a.Metadata.Name = "ID1" }),
		field.ErrorList{field.Invalid(field.NewPath("metadata", "name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"missing actor_template_namespace",
		line(),
		validActor(func(a *ateapipb.Actor) { a.ActorTemplateNamespace = "" }),
		field.ErrorList{field.Required(field.NewPath("actor_template_namespace"), "")},
	}, {
		"invalid actor_template_namespace",
		line(),
		validActor(func(a *ateapipb.Actor) { a.ActorTemplateNamespace = "invalid value" }),
		field.ErrorList{field.Invalid(field.NewPath("actor_template_namespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"missing actor_template_name",
		line(),
		validActor(func(a *ateapipb.Actor) { a.ActorTemplateName = "" }),
		field.ErrorList{field.Required(field.NewPath("actor_template_name"), "")},
	}, {
		"invalid actor_template_name",
		line(),
		validActor(func(a *ateapipb.Actor) { a.ActorTemplateName = "invalid value" }),
		field.ErrorList{field.Invalid(field.NewPath("actor_template_name"), nil, "").WithOrigin("format=k8s-long-name")},
	}, {
		"unspecified status",
		line(),
		validActor(func(a *ateapipb.Actor) { a.Status = 0 }),
		nil,
	}, {
		"negative status",
		line(),
		validActor(func(a *ateapipb.Actor) { a.Status = -1 }),
		field.ErrorList{field.Invalid(field.NewPath("status"), nil, "").WithOrigin("minimum")},
	}, {
		"valid status",
		line(),
		validActor(func(a *ateapipb.Actor) { a.Status = ateapipb.Actor_STATUS_RUNNING }),
		nil,
	}, {
		"invalid status",
		line(),
		validActor(func(a *ateapipb.Actor) { a.Status = 1234567890 }),
		field.ErrorList{field.Invalid(field.NewPath("status"), nil, "").WithOrigin("maximum")},
	}, {
		"unspecified worker_assignment",
		line(),
		validActor(func(a *ateapipb.Actor) { a.WorkerAssignment = nil }),
		nil,
	}, {
		"valid worker_assignmentat.worker_pod_ip IPv4",
		line(),
		validActor(func(a *ateapipb.Actor) { a.WorkerAssignment.WorkerPodIp = "1.2.3.4" }),
		nil,
	}, {
		"valid worker_assignmentat.worker_pod_ip IPv6",
		line(),
		validActor(func(a *ateapipb.Actor) { a.WorkerAssignment.WorkerPodIp = "1234::5678" }),
		nil,
	}, {
		"unspecified worker_assignment.worker_namespace",
		line(),
		validActor(func(a *ateapipb.Actor) { a.WorkerAssignment.WorkerNamespace = "" }),
		field.ErrorList{field.Required(field.NewPath("worker_assignment", "worker_namespace"), "")},
	}, {
		"invalid worker_assignment.worker_namespace",
		line(),
		validActor(func(a *ateapipb.Actor) { a.WorkerAssignment.WorkerNamespace = "invalid value" }),
		field.ErrorList{field.Invalid(field.NewPath("worker_assignment", "worker_namespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"unspecified worker_assignment.worker_pool",
		line(),
		validActor(func(a *ateapipb.Actor) { a.WorkerAssignment.WorkerPool = "" }),
		field.ErrorList{field.Required(field.NewPath("worker_assignment", "worker_pool"), "")},
	}, {
		"invalid worker_assignment.worker_pool",
		line(),
		validActor(func(a *ateapipb.Actor) { a.WorkerAssignment.WorkerPool = "invalid value" }),
		field.ErrorList{field.Invalid(field.NewPath("worker_assignment", "worker_pool"), nil, "").WithOrigin("format=k8s-long-name")},
	}, {
		"unspecified worker_assignment.worker_pod",
		line(),
		validActor(func(a *ateapipb.Actor) { a.WorkerAssignment.WorkerPod = "" }),
		field.ErrorList{field.Required(field.NewPath("worker_assignment", "worker_pod"), "")},
	}, {
		"invalid worker_assignment.worker_pod",
		line(),
		validActor(func(a *ateapipb.Actor) { a.WorkerAssignment.WorkerPod = "invalid value" }),
		field.ErrorList{field.Invalid(field.NewPath("worker_assignment", "worker_pod"), nil, "").WithOrigin("format=k8s-long-name")},
	}, {
		"unspecified worker_assignment.worker_pod_uid",
		line(),
		validActor(func(a *ateapipb.Actor) { a.WorkerAssignment.WorkerPodUid = "" }),
		field.ErrorList{field.Required(field.NewPath("worker_assignment", "worker_pod_uid"), "")},
	}, {
		"invalid worker_assignment.worker_pod_uid",
		line(),
		validActor(func(a *ateapipb.Actor) { a.WorkerAssignment.WorkerPodUid = "invalid value" }),
		field.ErrorList{field.Invalid(field.NewPath("worker_assignment", "worker_pod_uid"), nil, "").WithOrigin("format=k8s-uuid")},
	}, {
		"unspecified worker_assignment.worker_pod_ip",
		line(),
		validActor(func(a *ateapipb.Actor) { a.WorkerAssignment.WorkerPodIp = "" }),
		field.ErrorList{field.Required(field.NewPath("worker_assignment", "worker_pod_ip"), "")},
	}, {
		"invalid worker_assignment.worker_pod_ip",
		line(),
		validActor(func(a *ateapipb.Actor) { a.WorkerAssignment.WorkerPodIp = "invalid value" }),
		field.ErrorList{field.Invalid(field.NewPath("worker_assignment", "worker_pod_ip"), nil, "").WithOrigin("format=ip-strict")},
	}, {
		"valid in_progress_snapshot_name: short",
		line(),
		validActor(func(a *ateapipb.Actor) { a.InProgressSnapshotName = "a value" }),
		nil,
	}, {
		"valid in_progress_snapshot_name: long",
		line(),
		validActor(func(a *ateapipb.Actor) { a.InProgressSnapshotName = strings.Repeat("x", 256) }),
		nil,
	}, {
		"invalid in_progress_snapshot_name: too long",
		line(),
		validActor(func(a *ateapipb.Actor) { a.InProgressSnapshotName = strings.Repeat("x", 257) }),
		field.ErrorList{field.TooLongCharacters(field.NewPath("in_progress_snapshot_name"), "", 256).WithOrigin("maxLength")},
	}, {
		"worker_selector with nil match_labels",
		line(),
		validActor(func(a *ateapipb.Actor) { a.WorkerSelector = &ateapipb.Selector{} }),
		field.ErrorList{field.Invalid(field.NewPath("worker_selector"), nil, "one of").WithOrigin("union")},
	}, {
		"worker_selector with empty match_labels",
		line(),
		validActor(func(a *ateapipb.Actor) { a.WorkerSelector = &ateapipb.Selector{MatchLabels: map[string]string{}} }),
		field.ErrorList{field.Invalid(field.NewPath("worker_selector"), nil, "one of").WithOrigin("union")},
	}, {
		"valid worker_selector",
		line(),
		validActor(func(a *ateapipb.Actor) {
			a.WorkerSelector = &ateapipb.Selector{MatchLabels: map[string]string{"tier": "1"}}
		}),
		nil,
	}, {
		"invalid worker_selector label key",
		line(),
		validActor(func(a *ateapipb.Actor) {
			a.WorkerSelector = &ateapipb.Selector{MatchLabels: map[string]string{"bad key": "1"}}
		}),
		field.ErrorList{field.Invalid(field.NewPath("worker_selector", "match_labels"), nil, "").WithOrigin("format=k8s-label-key")},
	}, {
		"invalid worker_selector label value",
		line(),
		validActor(func(a *ateapipb.Actor) {
			a.WorkerSelector = &ateapipb.Selector{MatchLabels: map[string]string{"tier": "not valid!"}}
		}),
		field.ErrorList{field.Invalid(field.NewPath("worker_selector", "match_labels").Key("tier"), "not valid!", "").WithOrigin("format=k8s-label-value")},
	}, {
		"worker_selector with exactly max match_labels",
		line(),
		validActor(func(a *ateapipb.Actor) { a.WorkerSelector = &ateapipb.Selector{MatchLabels: selectorLabelsOfSize(10)} }),
		nil,
	}, {
		"too many worker_selector.match_labels",
		line(),
		validActor(func(a *ateapipb.Actor) {
			a.WorkerSelector = &ateapipb.Selector{MatchLabels: selectorLabelsOfSize(11)}
		}),
		field.ErrorList{field.TooMany(field.NewPath("worker_selector", "match_labels"), 11, 10).WithOrigin("maxProperties")},
	}}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s (L%s)", tt.name, tt.line), func(t *testing.T) {
			assertValidateErr(t, validateActor(context.Background(), tt.req), tt.want)
		})
	}
}
