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

package apivalidation

import (
	"context"
	"math"
	"strings"
	"testing"

	"go.ate.dev/substrate/api/ateapi/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// validResourceMetadata returns ResourceMetadata with only its required name
// set. The withMetadata* modifiers set the optional fields.
func validResourceMetadata(mutate ...func(*ateapi.ResourceMetadata)) *ateapi.ResourceMetadata {
	rm := &ateapi.ResourceMetadata{
		Name: "nm",
	}
	for _, m := range mutate {
		m(rm)
	}
	return rm
}

// withMetadataAtespace returns a modifier func (see validResourceMetadata)
// which sets the atespace.
func withMetadataAtespace(atespace string) func(*ateapi.ResourceMetadata) {
	return func(rm *ateapi.ResourceMetadata) { rm.Atespace = atespace }
}

// withMetadataUID returns a modifier func (see validResourceMetadata) which
// sets the uid to a valid value.
func withMetadataUID() func(*ateapi.ResourceMetadata) {
	return func(rm *ateapi.ResourceMetadata) { rm.Uid = "01234567-89ab-cdef-0123-456789abcdef" }
}

// withMetadataVersion returns a modifier func (see validResourceMetadata)
// which sets the version to a valid value.
func withMetadataVersion() func(*ateapi.ResourceMetadata) {
	return func(rm *ateapi.ResourceMetadata) { rm.Version = 93 }
}

// withMetadataCreateTime returns a modifier func (see validResourceMetadata)
// which sets the create_time to a valid value.
func withMetadataCreateTime() func(*ateapi.ResourceMetadata) {
	return func(rm *ateapi.ResourceMetadata) { rm.CreateTime = &timestamppb.Timestamp{Seconds: 867} }
}

// withMetadataUpdateTime returns a modifier func (see validResourceMetadata)
// which sets the update_time to a valid value, after withMetadataCreateTime's.
func withMetadataUpdateTime() func(*ateapi.ResourceMetadata) {
	return func(rm *ateapi.ResourceMetadata) { rm.UpdateTime = &timestamppb.Timestamp{Seconds: 5309} }
}

func TestValidateResourceMetadataCreate(t *testing.T) {
	withCreateTime := withMetadataCreateTime
	withUpdateTime := withMetadataUpdateTime
	valid := validResourceMetadata

	// Focus this test on fields other than atespace and name.
	tests := []struct {
		name string
		obj  *ateapi.ResourceMetadata
		want field.ErrorList
	}{{
		name: "valid",
		obj:  valid(),
	}, {
		name: "valid with all optional fields",
		obj: valid(
			withMetadataAtespace("as"),
			withMetadataUID(),
			withMetadataVersion(),
			withCreateTime(),
			withUpdateTime(),
		),
	}, {
		name: "missing name",
		obj:  valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "" }),
		want: field.ErrorList{field.Required(field.NewPath("name"), "")},
	}, {
		name: "invalid uid: close but not valid",
		obj:  valid(func(rm *ateapi.ResourceMetadata) { rm.Uid = "aaaaaaaa-bbbbcccc-dddd-eeeeeeeeeeee" }),
		want: field.ErrorList{field.Invalid(field.NewPath("uid"), nil, "").WithOrigin("format=k8s-uuid")},
	}, {
		name: "invalid uid: not even close",
		obj:  valid(func(rm *ateapi.ResourceMetadata) { rm.Uid = "not a uid" }),
		want: field.ErrorList{field.Invalid(field.NewPath("uid"), nil, "").WithOrigin("format=k8s-uuid")},
	}, {
		name: "valid version: large",
		obj:  valid(func(rm *ateapi.ResourceMetadata) { rm.Version = math.MaxInt64 }),
	}, {
		name: "invalid version: negative",
		obj:  valid(func(rm *ateapi.ResourceMetadata) { rm.Version = -1 }),
		want: field.ErrorList{field.Invalid(field.NewPath("version"), nil, "").WithOrigin("minimum")},
	}, {
		name: "invalid createTime: seconds out of range",
		obj: valid(func(rm *ateapi.ResourceMetadata) {
			rm.CreateTime = &timestamppb.Timestamp{Seconds: 253402300800}
		}),
		want: field.ErrorList{field.Invalid(field.NewPath("create_time"), nil, "")},
	}, {
		name: "invalid updateTime: negative nanos",
		obj: valid(func(rm *ateapi.ResourceMetadata) {
			rm.UpdateTime = &timestamppb.Timestamp{Seconds: 5309, Nanos: -1}
		}),
		want: field.ErrorList{field.Invalid(field.NewPath("update_time"), nil, "")},
	}, {
		name: "invalid updateTime: precedes createTime",
		obj: valid(
			withCreateTime(),
			func(rm *ateapi.ResourceMetadata) { rm.UpdateTime = &timestamppb.Timestamp{Seconds: 866} },
		),
		want: field.ErrorList{field.Invalid(field.NewPath("update_time"), nil, "")},
	}, {
		name: "valid updateTime: equals createTime",
		obj: valid(
			withCreateTime(),
			func(rm *ateapi.ResourceMetadata) { rm.UpdateTime = &timestamppb.Timestamp{Seconds: 867} },
		),
		want: nil,
	}, {
		name: "valid updateTime: set without createTime",
		obj:  valid(withUpdateTime()),
		want: nil,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := MakeCreateOp(context.Background())
			matcher := field.ErrorMatcher{}.ByType().ByField().ByOrigin()
			matcher.Test(t, tt.want, Validate_ResourceMetadata(context.Background(), op, nil, tt.obj, nil))
		})
	}
}

func TestValidateResourceMetadataUpdate(t *testing.T) {
	withUID := withMetadataUID
	withVersion := withMetadataVersion
	withCreateTime := withMetadataCreateTime
	withUpdateTime := withMetadataUpdateTime
	withAtespace := withMetadataAtespace
	valid := validResourceMetadata

	// Focus this test on fields other than atespace and name.
	tests := []struct {
		name   string
		oldObj *ateapi.ResourceMetadata // should always be valid
		newObj *ateapi.ResourceMetadata
		want   field.ErrorList
	}{{
		name:   "valid",
		oldObj: valid(),
		newObj: valid(),
	}, {
		name: "valid with all optional fields",
		oldObj: valid(
			withAtespace("as"),
			withUID(),
			withVersion(),
			withCreateTime(),
			withUpdateTime(),
		),
		newObj: valid(
			withAtespace("as"),
			withUID(),
			withVersion(),
			withCreateTime(),
			withUpdateTime(),
		),
	}, {
		name:   "atespace: empty -> non-empty",
		oldObj: valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "" }),
		newObj: valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "present" }),
		want:   field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("immutable")},
	}, {
		name:   "atespace: non-empty -> empty",
		oldObj: valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "present" }),
		newObj: valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "" }),
		want:   field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("immutable")},
	}, {
		name:   "atespace: changed",
		oldObj: valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "value-1" }),
		newObj: valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "value-2" }),
		want:   field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("immutable")},
	}, {
		name:   "name: unset",
		oldObj: valid(),
		newObj: valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "" }),
		want: field.ErrorList{
			field.Required(field.NewPath("name"), ""),
			field.Invalid(field.NewPath("name"), nil, "").WithOrigin("immutable"),
		},
	}, {
		name:   "name: changed",
		oldObj: valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "value-1" }),
		newObj: valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "value-2" }),
		want:   field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("immutable")},
	}, {
		name:   "uid: unset",
		oldObj: valid(withUID()),
		newObj: valid(func(rm *ateapi.ResourceMetadata) { rm.Uid = "" }),
		want:   field.ErrorList{field.Invalid(field.NewPath("uid"), nil, "").WithOrigin("immutable")},
	}, {
		name:   "uid: changed to valid",
		oldObj: valid(func(rm *ateapi.ResourceMetadata) { rm.Uid = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" }),
		newObj: valid(func(rm *ateapi.ResourceMetadata) { rm.Uid = "11111111-2222-3333-4444-555555555555" }),
		want:   field.ErrorList{field.Invalid(field.NewPath("uid"), nil, "").WithOrigin("immutable")},
	}, {
		name:   "uid: changed to invalid",
		oldObj: valid(func(rm *ateapi.ResourceMetadata) { rm.Uid = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" }),
		newObj: valid(func(rm *ateapi.ResourceMetadata) { rm.Uid = "not a uid" }),
		want:   field.ErrorList{field.Invalid(field.NewPath("uid"), nil, "").WithOrigin("immutable")},
	}, {
		name:   "version: unset",
		oldObj: valid(withVersion()),
		newObj: valid(func(rm *ateapi.ResourceMetadata) { rm.Version = 0 }),
		want:   field.ErrorList{field.Invalid(field.NewPath("version"), nil, "").WithOrigin("update")},
	}, {
		name:   "version: changed to valid",
		oldObj: valid(func(rm *ateapi.ResourceMetadata) { rm.Version = 123 }),
		newObj: valid(func(rm *ateapi.ResourceMetadata) { rm.Version = 456 }),
		want:   nil,
	}, {
		name:   "version: changed non-monotonically",
		oldObj: valid(func(rm *ateapi.ResourceMetadata) { rm.Version = 456 }),
		newObj: valid(func(rm *ateapi.ResourceMetadata) { rm.Version = 123 }),
		want: field.ErrorList{
			field.Invalid(field.NewPath("version"), nil, "").WithOrigin("monotonic"),
		},
	}, {
		name:   "version: changed to invalid",
		oldObj: valid(func(rm *ateapi.ResourceMetadata) { rm.Version = 456 }),
		newObj: valid(func(rm *ateapi.ResourceMetadata) { rm.Version = -1 }),
		want: field.ErrorList{
			field.Invalid(field.NewPath("version"), nil, "").WithOrigin("monotonic"),
			field.Invalid(field.NewPath("version"), nil, "").WithOrigin("minimum"),
		},
	}, {
		name:   "create_time: unset",
		oldObj: valid(withCreateTime()),
		newObj: valid(func(rm *ateapi.ResourceMetadata) { rm.CreateTime = nil }),
		want:   field.ErrorList{field.Invalid(field.NewPath("create_time"), nil, "").WithOrigin("immutable")},
	}, {
		name:   "create_time: changed",
		oldObj: valid(withCreateTime(), func(rm *ateapi.ResourceMetadata) { rm.CreateTime.Seconds = 123 }),
		newObj: valid(withCreateTime(), func(rm *ateapi.ResourceMetadata) { rm.CreateTime.Seconds = 456 }),
		want:   field.ErrorList{field.Invalid(field.NewPath("create_time"), nil, "").WithOrigin("immutable")},
	}, {
		name:   "update_time: unset",
		oldObj: valid(withUpdateTime()),
		newObj: valid(func(rm *ateapi.ResourceMetadata) { rm.UpdateTime = nil }),
		want:   field.ErrorList{field.Invalid(field.NewPath("update_time"), nil, "").WithOrigin("update")},
	}, {
		name:   "update_time: changed to valid",
		oldObj: valid(withUpdateTime(), func(rm *ateapi.ResourceMetadata) { rm.UpdateTime.Seconds = 1000 }),
		newObj: valid(withUpdateTime(), func(rm *ateapi.ResourceMetadata) { rm.UpdateTime.Seconds = 6000 }),
		want:   nil,
	}, {
		name:   "update_time: changed to precede create_time",
		oldObj: valid(withCreateTime(), withUpdateTime()),
		newObj: valid(
			withCreateTime(),
			withUpdateTime(),
			func(rm *ateapi.ResourceMetadata) { rm.UpdateTime.Seconds = 866 },
		),
		want: field.ErrorList{field.Invalid(field.NewPath("update_time"), nil, "")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := MakeUpdateOp(context.Background())
			matcher := field.ErrorMatcher{}.ByType().ByField().ByOrigin()
			matcher.Test(t, tt.want, Validate_ResourceMetadata(context.Background(), op, nil, tt.newObj, tt.oldObj))
		})
	}
}

func TestValidateResourceMetadataNameAndAtespaceFormat(t *testing.T) {
	valid := validResourceMetadata

	// Focus this test on exhaustive testing of the name and atespace fields.
	tests := []struct {
		name string
		obj  *ateapi.ResourceMetadata
		want field.ErrorList
	}{{
		"valid",
		valid(),
		nil,
	}, {
		"valid atespace: alphabetic",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "myatespace" }),
		nil,
	}, {
		"valid atespace: dashes",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "my-ate-space" }),
		nil,
	}, {
		"valid atespace: repeat dashes",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "my---ate---space" }),
		nil,
	}, {
		"valid atespace: alphanumeric",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "my-123-atespace" }),
		nil,
	}, {
		"valid atespace: leading numeric",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "123-atespace" }),
		nil,
	}, {
		"valid atespace: trailing numeric",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "my-123" }),
		nil,
	}, {
		"valid atespace: fully numeric",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "123" }),
		nil,
	}, {
		"valid atespace: long",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = strings.Repeat("x", 63) }),
		nil,
	}, {
		"invalid atespace: uppercase",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "MYATESPACE" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: leading dash",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "-atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: trailing dash",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "my-" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: dots",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "my.atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: underscores",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "my_atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: bang",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "my!atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: at",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "my@atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: pound",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "my#atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: dollar",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "my$atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: percent",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "my%%atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: caret",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "my^atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: ampersand",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "my&atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: star",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = "my*atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: too long",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Atespace = strings.Repeat("x", 64) }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"valid name: alphabetic",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "myname" }),
		nil,
	}, {
		"valid name: dashes",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "my-na-me" }),
		nil,
	}, {
		"valid name: repeat dashes",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "my---na---me" }),
		nil,
	}, {
		"valid name: alphanumeric",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "my-123-name" }),
		nil,
	}, {
		"invalid name: leading numeric",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "123-name" }),
		nil,
	}, {
		"invalid name: trailing numeric",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "my-123" }),
		nil,
	}, {
		"invalid name: fully numeric",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "123" }),
		nil,
	}, {
		"valid name: long",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Name = strings.Repeat("x", 63) }),
		nil,
	}, {
		"invalid name: uppercase",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "MYNAME" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: leading dash",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "-name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: trailing dash",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "my-" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: dots",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "my.name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: underscores",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "my_name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: bang",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "my!name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: at",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "my@name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: pound",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "my#name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: dollar",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "my$name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: percent",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "my%%name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: caret",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "my^name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: ampersand",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "my&name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: star",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Name = "my*name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: too long",
		valid(func(rm *ateapi.ResourceMetadata) { rm.Name = strings.Repeat("x", 64) }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := proto.CloneOf(tt.obj) // avoid internal mutations
			op := MakeCreateOp(context.Background())
			matcher := field.ErrorMatcher{}.ByType().ByField().ByOrigin()
			matcher.Test(t, tt.want, Validate_ResourceMetadata(context.Background(), op, nil, obj, nil))
		})
	}
}

func TestValidateObjectRef(t *testing.T) {
	valid := func(mutate func(*ateapi.ObjectRef)) *ateapi.ObjectRef {
		r := &ateapi.ObjectRef{
			Name: "nm",
		}
		if mutate != nil {
			mutate(r)
		}
		return r
	}

	tests := []struct {
		name string
		ref  *ateapi.ObjectRef
		want field.ErrorList
	}{{
		"valid",
		valid(nil),
		nil,
	}, {
		"valid atespace: alphabetic",
		valid(func(r *ateapi.ObjectRef) { r.Atespace = "myatespace" }),
		nil,
	}, {
		"valid atespace: dashes",
		valid(func(r *ateapi.ObjectRef) { r.Atespace = "my-ate-space" }),
		nil,
	}, {
		"valid atespace: repeat dashes",
		valid(func(r *ateapi.ObjectRef) { r.Atespace = "my---ate---space" }),
		nil,
	}, {
		"valid atespace: alphanumeric",
		valid(func(r *ateapi.ObjectRef) { r.Atespace = "my-123-atespace" }),
		nil,
	}, {
		"valid atespace: leading numeric",
		valid(func(r *ateapi.ObjectRef) { r.Atespace = "123-atespace" }),
		nil,
	}, {
		"valid atespace: trailing numeric",
		valid(func(r *ateapi.ObjectRef) { r.Atespace = "my-123" }),
		nil,
	}, {
		"valid atespace: fully numeric",
		valid(func(r *ateapi.ObjectRef) { r.Atespace = "123" }),
		nil,
	}, {
		"valid atespace: long",
		valid(func(r *ateapi.ObjectRef) { r.Atespace = strings.Repeat("x", 63) }),
		nil,
	}, {
		"invalid atespace: uppercase",
		valid(func(r *ateapi.ObjectRef) { r.Atespace = "MYATESPACE" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: leading dash",
		valid(func(r *ateapi.ObjectRef) { r.Atespace = "-atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: trailing dash",
		valid(func(r *ateapi.ObjectRef) { r.Atespace = "my-" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: dots",
		valid(func(r *ateapi.ObjectRef) { r.Atespace = "my.atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: underscores",
		valid(func(r *ateapi.ObjectRef) { r.Atespace = "my_atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: bang",
		valid(func(r *ateapi.ObjectRef) { r.Atespace = "my!atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: at",
		valid(func(r *ateapi.ObjectRef) { r.Atespace = "my@atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: pound",
		valid(func(r *ateapi.ObjectRef) { r.Atespace = "my#atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: dollar",
		valid(func(r *ateapi.ObjectRef) { r.Atespace = "my$atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: percent",
		valid(func(r *ateapi.ObjectRef) { r.Atespace = "my%%atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: caret",
		valid(func(r *ateapi.ObjectRef) { r.Atespace = "my^atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: ampersand",
		valid(func(r *ateapi.ObjectRef) { r.Atespace = "my&atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: star",
		valid(func(r *ateapi.ObjectRef) { r.Atespace = "my*atespace" }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid atespace: too long",
		valid(func(r *ateapi.ObjectRef) { r.Atespace = strings.Repeat("x", 64) }),
		field.ErrorList{field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"missing name",
		valid(func(r *ateapi.ObjectRef) { r.Name = "" }),
		field.ErrorList{field.Required(field.NewPath("name"), "")},
	}, {
		"valid name: alphabetic",
		valid(func(r *ateapi.ObjectRef) { r.Name = "myname" }),
		nil,
	}, {
		"valid name: dashes",
		valid(func(r *ateapi.ObjectRef) { r.Name = "my-na-me" }),
		nil,
	}, {
		"valid name: repeat dashes",
		valid(func(r *ateapi.ObjectRef) { r.Name = "my---na---me" }),
		nil,
	}, {
		"valid name: alphanumeric",
		valid(func(r *ateapi.ObjectRef) { r.Name = "my-123-name" }),
		nil,
	}, {
		"invalid name: leading numeric",
		valid(func(r *ateapi.ObjectRef) { r.Name = "123-name" }),
		nil,
	}, {
		"invalid name: trailing numeric",
		valid(func(r *ateapi.ObjectRef) { r.Name = "my-123" }),
		nil,
	}, {
		"invalid name: fully numeric",
		valid(func(r *ateapi.ObjectRef) { r.Name = "123" }),
		nil,
	}, {
		"valid name: long",
		valid(func(r *ateapi.ObjectRef) { r.Name = strings.Repeat("x", 63) }),
		nil,
	}, {
		"invalid name: uppercase",
		valid(func(r *ateapi.ObjectRef) { r.Name = "MYNAME" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: leading dash",
		valid(func(r *ateapi.ObjectRef) { r.Name = "-name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: trailing dash",
		valid(func(r *ateapi.ObjectRef) { r.Name = "my-" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: dots",
		valid(func(r *ateapi.ObjectRef) { r.Name = "my.name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: underscores",
		valid(func(r *ateapi.ObjectRef) { r.Name = "my_name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: bang",
		valid(func(r *ateapi.ObjectRef) { r.Name = "my!name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: at",
		valid(func(r *ateapi.ObjectRef) { r.Name = "my@name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: pound",
		valid(func(r *ateapi.ObjectRef) { r.Name = "my#name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: dollar",
		valid(func(r *ateapi.ObjectRef) { r.Name = "my$name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: percent",
		valid(func(r *ateapi.ObjectRef) { r.Name = "my%%name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: caret",
		valid(func(r *ateapi.ObjectRef) { r.Name = "my^name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: ampersand",
		valid(func(r *ateapi.ObjectRef) { r.Name = "my&name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: star",
		valid(func(r *ateapi.ObjectRef) { r.Name = "my*name" }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		"invalid name: too long",
		valid(func(r *ateapi.ObjectRef) { r.Name = strings.Repeat("x", 64) }),
		field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := MakeCreateOp(context.Background())
			matcher := field.ErrorMatcher{}.ByType().ByField().ByOrigin()
			matcher.Test(t, tt.want, Validate_ObjectRef(context.Background(), op, nil, tt.ref, nil))
		})
	}
}

func TestValidateDeleteOptions(t *testing.T) {
	valid := func(mutate ...func(*ateapi.DeleteOptions)) *ateapi.DeleteOptions {
		tb := &ateapi.DeleteOptions{}
		for _, m := range mutate {
			m(tb)
		}
		return tb
	}

	tests := []struct {
		name string
		obj  *ateapi.DeleteOptions
		want field.ErrorList
	}{{
		name: "valid",
		obj:  valid(), // all optional fields
	}, {
		name: "valid version",
		obj:  valid(func(do *ateapi.DeleteOptions) { do.Version = 1 }),
		want: nil,
	}, {
		name: "invalid version",
		obj:  valid(func(do *ateapi.DeleteOptions) { do.Version = -1 }),
		want: field.ErrorList{field.Invalid(field.NewPath("version"), nil, "").WithOrigin("minimum")},
	}, {
		name: "valid uid",
		obj:  valid(func(do *ateapi.DeleteOptions) { do.Uid = "11111111-2222-3333-4444-555555555555" }),
		want: nil,
	}, {
		name: "invalid uid",
		obj:  valid(func(do *ateapi.DeleteOptions) { do.Uid = "not a uid" }),
		want: field.ErrorList{field.Invalid(field.NewPath("uid"), nil, "").WithOrigin("format=k8s-uuid")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := MakeCreateOp(context.Background())
			matcher := field.ErrorMatcher{}.ByType().ByField().ByOrigin()
			matcher.Test(t, tt.want, Validate_DeleteOptions(context.Background(), op, nil, tt.obj, nil))
		})
	}
}

func validObjectSnapshot(mutate ...func(*ateapi.ObjectSnapshot)) *ateapi.ObjectSnapshot {
	s := &ateapi.ObjectSnapshot{
		SnapshotUri: "gs://private/atespaces/as/actors/" + someActorUID + "/snapshots/snap-1",
	}
	for _, m := range mutate {
		m(s)
	}
	return s
}

func validSnapshotStorage(mutate ...func(*ateapi.SnapshotStorage)) *ateapi.SnapshotStorage {
	ss := &ateapi.SnapshotStorage{
		State:  ateapi.SnapshotStorageState_SNAPSHOT_STORAGE_STATE_COMPLETED,
		Object: validObjectSnapshot(),
	}
	for _, m := range mutate {
		m(ss)
	}
	return ss
}

// validSnapshot returns a Snapshot with only its required fields set. The
// withSnapshot* modifiers set the optional fields.
func validSnapshot(mutate ...func(*ateapi.Snapshot)) *ateapi.Snapshot {
	s := &ateapi.Snapshot{
		Uuid:       "snap-1",
		Generation: 1,
		Owner:      ateapi.SnapshotOwner_SNAPSHOT_OWNER_ACTOR,
		Fidelity:   ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY,
	}
	for _, m := range mutate {
		m(s)
	}
	return s
}

// withSnapshotLocality returns a modifier func (see validSnapshot) which sets
// the locality to a valid value.
func withSnapshotLocality() func(*ateapi.Snapshot) {
	return func(s *ateapi.Snapshot) { s.Locality = "node-1" }
}

// withSnapshotDurableSnapshot returns a modifier func (see validSnapshot)
// which sets the durable_snapshot to a valid value.
func withSnapshotDurableSnapshot(mods ...func(*ateapi.SnapshotStorage)) func(*ateapi.Snapshot) {
	return func(s *ateapi.Snapshot) { s.DurableSnapshot = validSnapshotStorage(mods...) }
}

// badSnapshot violates Snapshot's rules at once, so a caller can assert that a
// containing type reaches every field of it.
func badSnapshot(mutate ...func(*ateapi.Snapshot)) *ateapi.Snapshot {
	breakIt := func(s *ateapi.Snapshot) {
		s.Fidelity = ateapi.SnapshotFidelity(4)
		s.DurableSnapshot = &ateapi.SnapshotStorage{
			State:  ateapi.SnapshotStorageState_SNAPSHOT_STORAGE_STATE_COMPLETED,
			Object: &ateapi.ObjectSnapshot{SnapshotUri: ""},
		}
	}
	return validSnapshot(append([]func(*ateapi.Snapshot){breakIt}, mutate...)...)
}

func TestValidateSnapshot(t *testing.T) {
	withDurableSnapshot := withSnapshotDurableSnapshot
	valid := validSnapshot
	uuidPath := field.NewPath("uuid")
	genPath := field.NewPath("generation")
	ownerPath := field.NewPath("owner")
	fidelityPath := field.NewPath("fidelity")
	localityPath := field.NewPath("locality")
	durableSnapshotPath := field.NewPath("durable_snapshot")
	statePath := durableSnapshotPath.Child("state")
	objectPath := durableSnapshotPath.Child("object")
	uriPath := objectPath.Child("snapshot_uri")

	tests := []struct {
		name string
		obj  *ateapi.Snapshot
		want field.ErrorList
	}{
		{
			name: "valid",
			obj:  valid(),
		},
		{
			name: "valid with all optional fields",
			obj: valid(
				withSnapshotLocality(),
				withDurableSnapshot(),
				func(s *ateapi.Snapshot) {
					s.ActorTemplateUid = someActorUID
				},
			),
		},
		{
			name: "missing uuid",
			obj:  valid(func(s *ateapi.Snapshot) { s.Uuid = "" }),
			want: field.ErrorList{field.Required(uuidPath, "")},
		},
		{
			name: "invalid uuid",
			obj:  valid(func(s *ateapi.Snapshot) { s.Uuid = "SNAP 1" }),
			want: field.ErrorList{field.Invalid(uuidPath, nil, "").WithOrigin("format=k8s-short-name")},
		},
		{
			name: "invalid locality",
			obj:  valid(func(s *ateapi.Snapshot) { s.Locality = "NODE 1" }),
			want: field.ErrorList{field.Invalid(localityPath, nil, "").WithOrigin("format=k8s-long-name")},
		},
		{
			name: "missing generation",
			obj:  valid(func(s *ateapi.Snapshot) { s.Generation = 0 }),
			want: field.ErrorList{field.Required(genPath, "")},
		},
		{
			name: "negative generation",
			obj:  valid(func(s *ateapi.Snapshot) { s.Generation = -1 }),
			want: field.ErrorList{field.Invalid(genPath, nil, "").WithOrigin("minimum")},
		},
		{
			name: "missing owner",
			obj:  valid(func(s *ateapi.Snapshot) { s.Owner = ateapi.SnapshotOwner_SNAPSHOT_OWNER_UNSPECIFIED }),
			want: field.ErrorList{field.Required(ownerPath, "")},
		},
		{
			name: "owner above the enum",
			obj:  valid(func(s *ateapi.Snapshot) { s.Owner = ateapi.SnapshotOwner(3) }),
			want: field.ErrorList{field.Invalid(ownerPath, nil, "").WithOrigin("maximum")},
		},
		{
			name: "valid fidelity: volumes",
			obj: valid(func(s *ateapi.Snapshot) {
				s.Fidelity = ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_VOLUMES
			}),
		},
		{
			name: "missing fidelity",
			obj: valid(func(s *ateapi.Snapshot) {
				s.Fidelity = ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_UNSPECIFIED
			}),
			want: field.ErrorList{field.Required(fidelityPath, "")},
		},
		{
			name: "missing durable_snapshot.state",
			obj: valid(
				withDurableSnapshot(func(ss *ateapi.SnapshotStorage) {
					ss.State = ateapi.SnapshotStorageState_SNAPSHOT_STORAGE_STATE_UNSPECIFIED
				}),
			),
			want: field.ErrorList{field.Required(statePath, "")},
		},
		{
			name: "negative durable_snapshot.state",
			obj: valid(
				withDurableSnapshot(func(ss *ateapi.SnapshotStorage) {
					ss.State = ateapi.SnapshotStorageState(-1)
				}),
			),
			want: field.ErrorList{field.Invalid(statePath, nil, "").WithOrigin("minimum")},
		},
		{
			name: "durable_snapshot.state above the enum",
			obj: valid(
				withDurableSnapshot(func(ss *ateapi.SnapshotStorage) {
					ss.State = ateapi.SnapshotStorageState(3)
				}),
			),
			want: field.ErrorList{field.Invalid(statePath, nil, "").WithOrigin("maximum")},
		},
		{
			name: "missing durable_snapshot.object",
			obj: valid(
				withDurableSnapshot(func(ss *ateapi.SnapshotStorage) {
					ss.Object = nil
				}),
			),
			want: field.ErrorList{field.Required(objectPath, "")},
		},
		{
			name: "missing snapshot_uri",
			obj:  valid(withDurableSnapshot(func(ss *ateapi.SnapshotStorage) { ss.Object.SnapshotUri = "" })),
			want: field.ErrorList{field.Required(uriPath, "")},
		},
		{
			name: "snapshot_uri too long",
			obj: valid(
				withDurableSnapshot(func(ss *ateapi.SnapshotStorage) {
					ss.Object.SnapshotUri = "gs://" + strings.Repeat("x", 2044)
				}),
			),
			want: field.ErrorList{field.TooLong(uriPath, nil, 2048).WithOrigin("maxLength")},
		},
		{
			name: "fidelity above the enum",
			obj:  valid(func(s *ateapi.Snapshot) { s.Fidelity = ateapi.SnapshotFidelity(4) }),
			want: field.ErrorList{field.Invalid(fidelityPath, nil, "").WithOrigin("maximum")},
		},
		{
			name: "negative fidelity",
			obj:  valid(func(s *ateapi.Snapshot) { s.Fidelity = ateapi.SnapshotFidelity(-1) }),
			want: field.ErrorList{field.Invalid(fidelityPath, nil, "").WithOrigin("minimum")},
		},
		{
			name: "every field invalid",
			obj:  badSnapshot(),
			want: field.ErrorList{
				field.Required(uriPath, ""),
				field.Invalid(fidelityPath, nil, "").WithOrigin("maximum"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := MakeCreateOp(context.Background())
			assertValidateErr(t, Validate_Snapshot(context.Background(), op, nil, tt.obj, nil), tt.want)
		})
	}
}

func TestValidateSnapshotUpdate(t *testing.T) {
	withDurableSnapshot := withSnapshotDurableSnapshot
	valid := validSnapshot
	fidelityPath := field.NewPath("fidelity")
	uriPath := field.NewPath("durable_snapshot", "object", "snapshot_uri")

	tests := []struct {
		name   string
		oldObj *ateapi.Snapshot
		newObj *ateapi.Snapshot
		want   field.ErrorList
	}{
		{
			name:   "unchanged",
			oldObj: valid(),
			newObj: valid(),
		},
		{
			// Each field is only revalidated when it changes, so a row written
			// before these rules existed does not block updates to the rest of
			// the object.
			name:   "unchanged invalid fields are not revalidated",
			oldObj: badSnapshot(),
			newObj: badSnapshot(),
		},
		{
			name:   "fidelity changed to a valid value",
			oldObj: valid(),
			newObj: valid(func(s *ateapi.Snapshot) {
				s.Fidelity = ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_VOLUMES
			}),
		},
		{
			name:   "fidelity cleared",
			oldObj: valid(),
			newObj: valid(func(s *ateapi.Snapshot) {
				s.Fidelity = ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_UNSPECIFIED
			}),
			want: field.ErrorList{field.Required(fidelityPath, "")},
		},
		{
			name:   "fidelity changed to a value outside the enum",
			oldObj: valid(),
			newObj: valid(func(s *ateapi.Snapshot) { s.Fidelity = ateapi.SnapshotFidelity(4) }),
			want:   field.ErrorList{field.Invalid(fidelityPath, nil, "").WithOrigin("maximum")},
		},
		{
			name:   "snapshot_uri cleared",
			oldObj: valid(withDurableSnapshot()),
			newObj: valid(withDurableSnapshot(func(ss *ateapi.SnapshotStorage) { ss.Object.SnapshotUri = "" })),
			want:   field.ErrorList{field.Required(uriPath, "")},
		},
		{
			// The other side of the ratchet: a row that predates these rules
			// can still be repaired, one field at a time.
			name:   "fidelity repaired",
			oldObj: badSnapshot(),
			newObj: badSnapshot(func(s *ateapi.Snapshot) {
				s.Fidelity = ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY
			}),
		},
		{
			name:   "snapshot_uri changed to a value that is too long",
			oldObj: valid(withDurableSnapshot()),
			newObj: valid(
				withDurableSnapshot(func(ss *ateapi.SnapshotStorage) {
					ss.Object.SnapshotUri = "gs://" + strings.Repeat("x", 2044)
				}),
			),
			want: field.ErrorList{field.TooLong(uriPath, nil, 2048).WithOrigin("maxLength")},
		},
		{
			name:   "snapshot_uri repaired",
			oldObj: badSnapshot(),
			newObj: badSnapshot(func(s *ateapi.Snapshot) {
				s.DurableSnapshot.Object.SnapshotUri = validObjectSnapshot().SnapshotUri
			}),
		},
		{
			name:   "every field repaired",
			oldObj: badSnapshot(),
			newObj: valid(withDurableSnapshot()),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := MakeUpdateOp(context.Background())
			assertValidateErr(t, Validate_Snapshot(context.Background(), op, nil, tt.newObj, tt.oldObj), tt.want)
		})
	}
}

// TestValidateNestedSnapshot checks that every type that
// holds Snapshot has to descend into it, and report under
// the holder's own path.
func TestValidateNestedSnapshot(t *testing.T) {
	tests := []struct {
		name string
		// path is where the offending Snapshot sits in the holder.
		path     *field.Path
		validate func(ctx context.Context) field.ErrorList
	}{
		{
			name: "actor.status.snapshots",
			path: field.NewPath("status", "snapshots").Index(0),
			validate: func(ctx context.Context) field.ErrorList {
				// The live path: the server validates the Actor it is about to
				// write, as an update against the stored one.
				op := MakeUpdateOp(context.Background())
				oldVal := validActor(withActorStatus())
				newVal := validActor(
					withActorStatus(func(s *ateapi.ActorStatus) {
						s.Snapshots = []*ateapi.Snapshot{badSnapshot()}
					}),
				)
				return Validate_Actor(ctx, op, nil, newVal, oldVal)
			},
		},
		{
			name: "tag.status.snapshot",
			path: field.NewPath("status", "snapshot"),
			validate: func(ctx context.Context) field.ErrorList {
				op := MakeCreateOp(context.Background())
				obj := validTag(
					withTagStatus(func(status *ateapi.TagStatus) {
						status.Snapshot = badSnapshot()
					}),
				)
				return Validate_Tag(ctx, op, nil, obj, nil)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := field.ErrorList{
				field.Required(tt.path.Child("durable_snapshot", "object", "snapshot_uri"), ""),
				field.Invalid(tt.path.Child("fidelity"), nil, "").WithOrigin("maximum"),
			}
			assertValidateErr(t, tt.validate(context.Background()), want)
		})
	}
}
