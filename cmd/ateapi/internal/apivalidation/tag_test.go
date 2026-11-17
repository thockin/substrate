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
	"strings"
	"testing"

	"go.ate.dev/substrate/api/ateapi/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateCreateTagRequest(t *testing.T) {
	withDurableSnapshot := withSnapshotDurableSnapshot
	ctx := context.Background()
	tests := []struct {
		name      string
		req       *ateapi.CreateTagRequest
		wantError field.ErrorList
	}{
		{
			name:      "valid",
			req:       &ateapi.CreateTagRequest{Tag: validTag()},
			wantError: nil,
		},
		{
			// Status is server-owned and scrubbed before this runs, but a
			// client that echoes back a tag it read is not to be tripped up by
			// one either.
			name: "valid with status",
			req: &ateapi.CreateTagRequest{
				Tag: validTag(withTagStatus(withTagStatusSnapshot())),
			},
			wantError: nil,
		},
		{
			// A reserved tag has no snapshot until the tag workflow records it.
			name: "valid with status but no snapshot",
			req: &ateapi.CreateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) {
					tag.Status = &ateapi.TagStatus{ActorTemplateUid: someActorUID}
				}),
			},
			wantError: nil,
		},
		{
			name: "invalid tag.status.snapshot",
			req: &ateapi.CreateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) {
					tag.Status = &ateapi.TagStatus{
						Snapshot:         badSnapshot(),
						ActorTemplateUid: someActorUID,
					}
				}),
			},
			wantError: field.ErrorList{
				field.Required(field.NewPath("tag", "status", "snapshot", "durable_snapshot", "object", "snapshot_uri"), ""),
				field.Invalid(field.NewPath("tag", "status", "snapshot", "fidelity"), nil, "").WithOrigin("maximum"),
			},
		},
		{
			name: "missing tag.status.snapshot.durable_snapshot",
			req: &ateapi.CreateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) {
					tag.Status = &ateapi.TagStatus{
						Snapshot:         validSnapshot(),
						ActorTemplateUid: someActorUID,
					}
				}),
			},
			wantError: field.ErrorList{
				field.Required(field.NewPath("tag", "status", "snapshot", "durable_snapshot"), ""),
			},
		},
		{
			name: "missing tag.status.actor_template_uid",
			req: &ateapi.CreateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) {
					tag.Status = &ateapi.TagStatus{Snapshot: validSnapshot(withDurableSnapshot())}
				}),
			},
			wantError: field.ErrorList{field.Required(field.NewPath("tag", "status", "actor_template_uid"), "")},
		},
		{
			name: "invalid tag.status.actor_template_uid",
			req: &ateapi.CreateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) {
					tag.Status = &ateapi.TagStatus{
						Snapshot:         validSnapshot(withDurableSnapshot()),
						ActorTemplateUid: "not-a-uuid",
					}
				}),
			},
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("tag", "status", "actor_template_uid"), nil, "").WithOrigin("format=k8s-uuid"),
			},
		},
		{
			// The tag has to name the atespace it lands in, like every other
			// create: the source Actor's atespace is checked against it, not
			// used as a default.
			name: "empty tag.metadata.atespace",
			req: &ateapi.CreateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) { tag.Metadata.Atespace = "" }),
			},
			wantError: field.ErrorList{field.Required(field.NewPath("tag", "metadata", "atespace"), "")},
		},
		{
			// Malformed, so it is both rejected on its own terms and unable to
			// match the source actor's.
			name: "invalid tag.metadata.atespace",
			req: &ateapi.CreateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) { tag.Metadata.Atespace = "NS1" }),
			},
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("tag", "metadata", "atespace"), nil, ""),
				field.Invalid(field.NewPath("tag", "metadata", "atespace"), nil, "").WithOrigin("format=k8s-short-name"),
			},
		},
		{
			name: "missing tag.source_actor",
			req: &ateapi.CreateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) { tag.SourceActor = nil }),
			},
			wantError: field.ErrorList{field.Required(field.NewPath("tag", "source_actor"), "")},
		},
		{
			// Only source_actor is reported: tag.metadata.atespace cannot be
			// held against a source that has none to compare it to.
			name: "missing tag.source_actor.atespace",
			req: &ateapi.CreateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) { tag.SourceActor.Atespace = "" }),
			},
			wantError: field.ErrorList{field.Required(field.NewPath("tag", "source_actor", "atespace"), "")},
		},
		{
			// A malformed source atespace is also one the tag's own atespace
			// cannot match, so both are reported.
			name: "invalid tag.source_actor.atespace",
			req: &ateapi.CreateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) { tag.SourceActor.Atespace = "NS1" }),
			},
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("tag", "metadata", "atespace"), nil, ""),
				field.Invalid(field.NewPath("tag", "source_actor", "atespace"), nil, "").WithOrigin("format=k8s-short-name"),
			},
		},
		{
			name: "missing tag.source_actor.name",
			req: &ateapi.CreateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) { tag.SourceActor.Name = "" }),
			},
			wantError: field.ErrorList{field.Required(field.NewPath("tag", "source_actor", "name"), "")},
		},
		{
			name: "invalid tag.source_actor.name",
			req: &ateapi.CreateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) { tag.SourceActor.Name = "ID1" }),
			},
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("tag", "source_actor", "name"), nil, "").WithOrigin("format=k8s-short-name"),
			},
		},
		{
			name:      "missing tag",
			req:       &ateapi.CreateTagRequest{},
			wantError: field.ErrorList{field.Required(field.NewPath("tag"), "")},
		},
		{
			name: "missing tag.metadata",
			req: &ateapi.CreateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) { tag.Metadata = nil }),
			},
			wantError: field.ErrorList{field.Required(field.NewPath("tag", "metadata"), "")},
		},
		{
			name: "missing tag.metadata.name",
			req: &ateapi.CreateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) { tag.Metadata.Name = "" }),
			},
			wantError: field.ErrorList{field.Required(field.NewPath("tag", "metadata", "name"), "")},
		},
		{
			name: "invalid tag.metadata.name",
			req: &ateapi.CreateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) { tag.Metadata.Name = "TAG1" }),
			},
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("tag", "metadata", "name"), nil, "").WithOrigin("format=k8s-short-name"),
			},
		},
		{
			// A tag somewhere else than its source Actor could not be resolved
			// back to it.
			name: "tag.metadata.atespace is not the source actor's",
			req: &ateapi.CreateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) { tag.Metadata.Atespace = "ns2" }),
			},
			wantError: field.ErrorList{field.Invalid(field.NewPath("tag", "metadata", "atespace"), nil, "")},
		},
		{
			name: "unset tag.scope",
			req: &ateapi.CreateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) {
					tag.Scope = ateapi.TagScope_TAG_SCOPE_UNSPECIFIED
				}),
			},
			wantError: field.ErrorList{field.Required(field.NewPath("tag", "scope"), "")},
		},
		{
			name: "tag.scope above the enum",
			req: &ateapi.CreateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) { tag.Scope = ateapi.TagScope(7) }),
			},
			wantError: field.ErrorList{field.Invalid(field.NewPath("tag", "scope"), nil, "").WithOrigin("maximum")},
		},
		{
			name: "negative tag.scope",
			req: &ateapi.CreateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) { tag.Scope = ateapi.TagScope(-1) }),
			},
			wantError: field.ErrorList{field.Invalid(field.NewPath("tag", "scope"), nil, "").WithOrigin("minimum")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateCreateTagRequest(ctx, tt.req), tt.wantError)
		})
	}
}

func TestValidateUpdateTagRequest(t *testing.T) {
	withMetadata := withTagMetadata
	withUID := withMetadataUID
	withVersion := withMetadataVersion
	ctx := context.Background()
	tests := []struct {
		name      string
		req       *ateapi.UpdateTagRequest
		wantError field.ErrorList
	}{
		{
			name:      "valid",
			req:       &ateapi.UpdateTagRequest{Tag: validTag()},
			wantError: nil,
		},
		{
			name: "valid with preconditions",
			req: &ateapi.UpdateTagRequest{
				Tag: validTag(withMetadata(withUID(), withVersion())),
			},
			wantError: nil,
		},
		{
			name:      "missing tag",
			req:       &ateapi.UpdateTagRequest{},
			wantError: field.ErrorList{field.Required(field.NewPath("tag"), "")},
		},
		{
			name: "missing tag.metadata",
			req: &ateapi.UpdateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) { tag.Metadata = nil }),
			},
			wantError: field.ErrorList{field.Required(field.NewPath("tag", "metadata"), "")},
		},
		{
			name: "missing tag.metadata.atespace",
			req: &ateapi.UpdateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) { tag.Metadata.Atespace = "" }),
			},
			wantError: field.ErrorList{field.Required(field.NewPath("tag", "metadata", "atespace"), "")},
		},
		{
			name: "invalid tag.metadata.atespace",
			req: &ateapi.UpdateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) { tag.Metadata.Atespace = "NS1" }),
			},
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("tag", "metadata", "atespace"), nil, "").WithOrigin("format=k8s-short-name"),
			},
		},
		{
			name: "missing tag.metadata.name",
			req: &ateapi.UpdateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) { tag.Metadata.Name = "" }),
			},
			wantError: field.ErrorList{field.Required(field.NewPath("tag", "metadata", "name"), "")},
		},
		{
			name: "invalid tag.metadata.name",
			req: &ateapi.UpdateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) { tag.Metadata.Name = "TAG1" }),
			},
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("tag", "metadata", "name"), nil, "").WithOrigin("format=k8s-short-name"),
			},
		},
		{
			// The preconditions are the store's to insist on, not the schema's:
			// an update carrying neither is rejected as a blind write once it
			// reaches the store. See TestUpdateTag_BlindWrite.
			name: "missing tag.metadata.uid precondition",
			req: &ateapi.UpdateTagRequest{
				Tag: validTag(withMetadata(withVersion())),
			},
			wantError: nil,
		},
		{
			name: "invalid tag.metadata.uid precondition",
			req: &ateapi.UpdateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) { tag.Metadata.Uid = "not-a-uuid" }),
			},
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("tag", "metadata", "uid"), nil, "").WithOrigin("format=k8s-uuid"),
			},
		},
		{
			name: "missing tag.metadata.version precondition",
			req: &ateapi.UpdateTagRequest{
				Tag: validTag(withMetadata(withUID())),
			},
			wantError: nil,
		},
		{
			name: "negative tag.metadata.version precondition",
			req: &ateapi.UpdateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) { tag.Metadata.Version = -1 }),
			},
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("tag", "metadata", "version"), nil, "").WithOrigin("minimum"),
			},
		},
		{
			// Nothing about the tag body is held against the request: it is
			// checked against the stored tag instead, so a source or a scope
			// the request gets wrong is caught there.
			name: "tag body is not checked against the request",
			req: &ateapi.UpdateTagRequest{
				Tag: validTag(func(tag *ateapi.Tag) {
					tag.SourceActor = nil
					tag.Scope = ateapi.TagScope_TAG_SCOPE_UNSPECIFIED
				}),
			},
			wantError: nil,
		},
		{
			// Status is checked as part of the whole tag, against the stored
			// one.
			name: "tag.status is not checked against the request",
			req: &ateapi.UpdateTagRequest{
				Tag: validTag(
					withTagStatus(func(status *ateapi.TagStatus) {
						status.Snapshot = badSnapshot()
					}),
				),
			},
			wantError: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateUpdateTagRequest(ctx, tt.req), tt.wantError)
		})
	}
}

// TestValidateTagUpdate covers the second half of an update:
// the merged tag the server is about to write, checked against the one it
// replaces. This is where everything the request could not be held to lands.
func TestValidateTagUpdate(t *testing.T) {
	withStatus := withTagStatus
	withSnapshot := withTagStatusSnapshot
	ctx := context.Background()
	tagPath := field.NewPath("tag")
	pending := withStatus(
		withSnapshot(func(s *ateapi.Snapshot) {
			s.DurableSnapshot.State = ateapi.SnapshotStorageState_SNAPSHOT_STORAGE_STATE_PENDING
		}),
	)
	completed := withStatus(
		withSnapshot(func(s *ateapi.Snapshot) {
			s.DurableSnapshot.State = ateapi.SnapshotStorageState_SNAPSHOT_STORAGE_STATE_COMPLETED
		}),
	)
	tests := []struct {
		name      string
		oldVal    *ateapi.Tag
		newVal    *ateapi.Tag
		wantError field.ErrorList
	}{
		{
			name:      "unchanged",
			oldVal:    validTag(),
			newVal:    validTag(),
			wantError: nil,
		},
		{
			name:   "publishing is allowed",
			oldVal: validTag(),
			newVal: validTag(func(tag *ateapi.Tag) {
				tag.Scope = ateapi.TagScope_TAG_SCOPE_PUBLISHED
			}),
			wantError: nil,
		},
		{
			name:   "unset tag.scope",
			oldVal: validTag(),
			newVal: validTag(func(tag *ateapi.Tag) {
				tag.Scope = ateapi.TagScope_TAG_SCOPE_UNSPECIFIED
			}),
			wantError: field.ErrorList{field.Required(tagPath.Child("scope"), "")},
		},
		{
			name:      "tag.scope above the enum",
			oldVal:    validTag(),
			newVal:    validTag(func(tag *ateapi.Tag) { tag.Scope = ateapi.TagScope(7) }),
			wantError: field.ErrorList{field.Invalid(tagPath.Child("scope"), nil, "").WithOrigin("maximum")},
		},
		{
			name:      "negative tag.scope",
			oldVal:    validTag(),
			newVal:    validTag(func(tag *ateapi.Tag) { tag.Scope = ateapi.TagScope(-1) }),
			wantError: field.ErrorList{field.Invalid(tagPath.Child("scope"), nil, "").WithOrigin("minimum")},
		},
		{
			// A tag never moves between snapshots, so it never moves between
			// sources either.
			name:   "repointed tag.source_actor",
			oldVal: validTag(),
			newVal: validTag(func(tag *ateapi.Tag) {
				tag.SourceActor = &ateapi.ObjectRef{Atespace: "ns1", Name: "id2"}
			}),
			wantError: field.ErrorList{field.Invalid(tagPath.Child("source_actor"), nil, "").WithOrigin("immutable")},
		},
		{
			// An update is a whole-object replace, so a client that drops the
			// source it read back would clear it.
			name:   "missing tag.source_actor",
			oldVal: validTag(),
			newVal: validTag(func(tag *ateapi.Tag) { tag.SourceActor = nil }),
			wantError: field.ErrorList{
				field.Invalid(tagPath.Child("source_actor"), nil, "").WithOrigin("immutable"),
				field.Required(tagPath.Child("source_actor"), ""),
			},
		},
		{
			// The tag is addressed through its atespace and name; moving it
			// would strand every reference to it.
			name:   "renamed tag",
			oldVal: validTag(),
			newVal: validTag(func(tag *ateapi.Tag) { tag.Metadata.Name = "tag2" }),
			wantError: field.ErrorList{
				field.Invalid(tagPath.Child("metadata", "name"), nil, "").WithOrigin("immutable"),
			},
		},
		{
			name:   "tag moved to another atespace",
			oldVal: validTag(),
			newVal: validTag(func(tag *ateapi.Tag) { tag.Metadata.Atespace = "ns2" }),
			wantError: field.ErrorList{
				field.Invalid(tagPath.Child("metadata", "atespace"), nil, "").WithOrigin("immutable"),
			},
		},
		{
			name:   "cleared tag.metadata.atespace",
			oldVal: validTag(),
			newVal: validTag(func(tag *ateapi.Tag) { tag.Metadata.Atespace = "" }),
			wantError: field.ErrorList{
				field.Required(tagPath.Child("metadata", "atespace"), ""),
				field.Invalid(tagPath.Child("metadata", "atespace"), nil, "").WithOrigin("immutable"),
			},
		},
		{
			name:      "missing tag.metadata",
			oldVal:    validTag(),
			newVal:    validTag(func(tag *ateapi.Tag) { tag.Metadata = nil }),
			wantError: field.ErrorList{field.Required(tagPath.Child("metadata"), "")},
		},
		{
			name:   "tag.status.snapshot recorded",
			oldVal: validTag(withStatus()),
			newVal: validTag(pending),
		},
		{
			name:   "tag.status.snapshot completed",
			oldVal: validTag(pending),
			newVal: validTag(completed),
		},
		{
			name:   "tag.status.snapshot moved back to pending",
			oldVal: validTag(completed),
			newVal: validTag(pending),
			wantError: field.ErrorList{
				field.Forbidden(tagPath.Child("status", "snapshot"), ""),
			},
		},
		{
			name:   "tag.status.snapshot repointed while completing",
			oldVal: validTag(pending),
			newVal: validTag(completed, func(tag *ateapi.Tag) {
				tag.Status.Snapshot.DurableSnapshot.Object.SnapshotUri = "gs://bucket/elsewhere"
			}),
			wantError: field.ErrorList{
				field.Forbidden(tagPath.Child("status", "snapshot"), ""),
			},
		},
		{
			name:   "tag.status.snapshot repointed",
			oldVal: validTag(completed),
			newVal: validTag(completed, func(tag *ateapi.Tag) {
				tag.Status.Snapshot.DurableSnapshot.Object.SnapshotUri = "gs://bucket/elsewhere"
			}),
			wantError: field.ErrorList{
				field.Forbidden(tagPath.Child("status", "snapshot"), ""),
			},
		},
		{
			name:   "tag.status.snapshot cleared",
			oldVal: validTag(completed),
			newVal: validTag(withStatus()),
			wantError: field.ErrorList{
				field.Invalid(tagPath.Child("status", "snapshot"), nil, "").WithOrigin("update"),
			},
		},
		{
			name:   "tag.status cleared",
			oldVal: validTag(completed),
			newVal: validTag(),
			wantError: field.ErrorList{
				field.Invalid(tagPath.Child("status"), nil, "").WithOrigin("update"),
			},
		},
		{
			name:   "tag.status.actor_template_uid changed",
			oldVal: validTag(withStatus()),
			newVal: validTag(
				withStatus(),
				func(tag *ateapi.Tag) {
					tag.Status.ActorTemplateUid = "fedcba98-7654-3210-fedc-ba9876543210"
				},
			),
			wantError: field.ErrorList{
				field.Invalid(tagPath.Child("status", "actor_template_uid"), nil, "").WithOrigin("immutable"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldVal := tt.oldVal
			assertValidateErr(t, ValidateTagUpdate(ctx, tagPath, tt.newVal, oldVal), tt.wantError)
		})
	}
}

func TestValidateListTagsRequest(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		req       *ateapi.ListTagsRequest
		wantError field.ErrorList
	}{
		{
			name:      "valid, atespace scoped",
			req:       &ateapi.ListTagsRequest{Atespace: "ns1"},
			wantError: nil,
		},
		{
			// Empty atespace means "all atespaces"
			// (kubectl ate get tags -A).
			name:      "valid, empty atespace means all atespaces",
			req:       &ateapi.ListTagsRequest{},
			wantError: nil,
		},
		{
			name: "invalid atespace",
			req:  &ateapi.ListTagsRequest{Atespace: "NS1"},
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("atespace"), nil, "").WithOrigin("format=k8s-short-name"),
			},
		},
		{
			name:      "valid, positive page_size",
			req:       &ateapi.ListTagsRequest{Atespace: "ns1", Options: &ateapi.ListOptions{PageSize: 10}},
			wantError: nil,
		},
		{
			name: "negative page_size",
			req:  &ateapi.ListTagsRequest{Atespace: "ns1", Options: &ateapi.ListOptions{PageSize: -1}},
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("options", "page_size"), nil, "").WithOrigin("minimum"),
			},
		},
		{
			name: "valid page_token",
			req: &ateapi.ListTagsRequest{
				Atespace: "ns1",
				Options:  &ateapi.ListOptions{PageToken: strings.Repeat("x", 256)},
			},
			wantError: nil,
		},
		{
			name: "too-large page_token",
			req: &ateapi.ListTagsRequest{
				Atespace: "ns1",
				Options:  &ateapi.ListOptions{PageToken: strings.Repeat("x", 257)},
			},
			wantError: field.ErrorList{
				field.TooLongCharacters(field.NewPath("options", "page_token"), "", 256).WithOrigin("maxLength"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateListTagsRequest(ctx, tt.req), tt.wantError)
		})
	}
}

func TestValidateGetTagRequest(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		req       *ateapi.GetTagRequest
		wantError field.ErrorList
	}{
		{
			name:      "valid",
			req:       &ateapi.GetTagRequest{Tag: &ateapi.ObjectRef{Atespace: "ns1", Name: "tag1"}},
			wantError: nil,
		},
		{
			name:      "missing tag",
			req:       &ateapi.GetTagRequest{},
			wantError: field.ErrorList{field.Required(field.NewPath("tag"), "")},
		},
		{
			name:      "missing tag.atespace",
			req:       &ateapi.GetTagRequest{Tag: &ateapi.ObjectRef{Name: "tag1"}},
			wantError: field.ErrorList{field.Required(field.NewPath("tag", "atespace"), "")},
		},
		{
			name: "invalid tag.atespace",
			req:  &ateapi.GetTagRequest{Tag: &ateapi.ObjectRef{Atespace: "NS1", Name: "tag1"}},
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("tag", "atespace"), nil, "").WithOrigin("format=k8s-short-name"),
			},
		},
		{
			name:      "missing tag.name",
			req:       &ateapi.GetTagRequest{Tag: &ateapi.ObjectRef{Atespace: "ns1"}},
			wantError: field.ErrorList{field.Required(field.NewPath("tag", "name"), "")},
		},
		{
			name: "invalid tag.name",
			req:  &ateapi.GetTagRequest{Tag: &ateapi.ObjectRef{Atespace: "ns1", Name: "TAG1"}},
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("tag", "name"), nil, "").WithOrigin("format=k8s-short-name"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateGetTagRequest(ctx, tt.req), tt.wantError)
		})
	}
}

func TestValidateDeleteTagRequest(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		req       *ateapi.DeleteTagRequest
		wantError field.ErrorList
	}{
		{
			name:      "valid",
			req:       &ateapi.DeleteTagRequest{Tag: &ateapi.ObjectRef{Atespace: "ns1", Name: "tag1"}},
			wantError: nil,
		},
		{
			name:      "missing tag",
			req:       &ateapi.DeleteTagRequest{},
			wantError: field.ErrorList{field.Required(field.NewPath("tag"), "")},
		},
		{
			name:      "missing tag.atespace",
			req:       &ateapi.DeleteTagRequest{Tag: &ateapi.ObjectRef{Name: "tag1"}},
			wantError: field.ErrorList{field.Required(field.NewPath("tag", "atespace"), "")},
		},
		{
			name: "invalid tag.atespace",
			req:  &ateapi.DeleteTagRequest{Tag: &ateapi.ObjectRef{Atespace: "NS1", Name: "tag1"}},
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("tag", "atespace"), nil, "").WithOrigin("format=k8s-short-name"),
			},
		},
		{
			name:      "missing tag.name",
			req:       &ateapi.DeleteTagRequest{Tag: &ateapi.ObjectRef{Atespace: "ns1"}},
			wantError: field.ErrorList{field.Required(field.NewPath("tag", "name"), "")},
		},
		{
			name: "invalid tag.name",
			req:  &ateapi.DeleteTagRequest{Tag: &ateapi.ObjectRef{Atespace: "ns1", Name: "TAG1"}},
			wantError: field.ErrorList{
				field.Invalid(field.NewPath("tag", "name"), nil, "").WithOrigin("format=k8s-short-name"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateDeleteTagRequest(ctx, tt.req), tt.wantError)
		})
	}
}

// validTag returns a Tag with only its required fields set. The withTag*
// modifiers set the optional fields.
func validTag(mutate ...func(*ateapi.Tag)) *ateapi.Tag {
	tag := &ateapi.Tag{
		Metadata:    validResourceMetadata(withMetadataAtespace("as")),
		Scope:       ateapi.TagScope_TAG_SCOPE_ATESPACE,
		SourceActor: &ateapi.ObjectRef{Atespace: "as", Name: "nm"},
	}
	for _, m := range mutate {
		m(tag)
	}
	return tag
}

// withTagMetadata returns a modifier func (see validTag) which applies mods to
// the tag's metadata.
func withTagMetadata(mods ...func(*ateapi.ResourceMetadata)) func(*ateapi.Tag) {
	return func(tag *ateapi.Tag) {
		for _, m := range mods {
			m(tag.Metadata)
		}
	}
}

// withTagStatus returns a modifier func (see validTag) which sets the tag's
// status to a valid value with no snapshot.
func withTagStatus(mods ...func(*ateapi.TagStatus)) func(*ateapi.Tag) {
	return func(tag *ateapi.Tag) {
		tag.Status = &ateapi.TagStatus{ActorTemplateUid: someActorUID}
		for _, m := range mods {
			m(tag.Status)
		}
	}
}

// withTagStatusSnapshot returns a modifier func (see withTagStatus) which sets
// the tag's snapshot to a valid, COMPLETED snapshot owned by the tag.
func withTagStatusSnapshot(mods ...func(*ateapi.Snapshot)) func(*ateapi.TagStatus) {
	return func(status *ateapi.TagStatus) {
		status.Snapshot = validSnapshot(
			withSnapshotDurableSnapshot(),
			func(s *ateapi.Snapshot) {
				s.Owner = ateapi.SnapshotOwner_SNAPSHOT_OWNER_TAG
			},
		)
		for _, m := range mods {
			m(status.Snapshot)
		}
	}
}
