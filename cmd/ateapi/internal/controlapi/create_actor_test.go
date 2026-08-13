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
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/agent-substrate/substrate/internal/ateattr"
	atev1alpha1 "github.com/agent-substrate/substrate/pkg/api/v1alpha1"
	"github.com/agent-substrate/substrate/pkg/proto/ateapipb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/wait"
)

// CreateActor is the only lifecycle op with the full identity (incl. version)
// available in the request, so the whole ate.* set should land on its span.
func TestCreateActor_StampsFullSpanIdentity(t *testing.T) {
	ns := namespaceForTest("ns-span-create")
	tc := setupTest(t, ns)
	defer tc.cleanup()
	createTemplate(t, tc, ns)

	attrs := recordRootSpanAttrs(t, func(ctx context.Context) {
		if _, err := tc.service.CreateActor(ctx, &ateapipb.CreateActorRequest{
			Actor: &ateapipb.Actor{
				Metadata:               &ateapipb.ResourceMetadata{Atespace: testAtespace, Name: testActorID},
				ActorTemplateNamespace: ns,
				ActorTemplateName:      "tmpl1",
			},
		}); err != nil {
			t.Fatalf("CreateActor: %v", err)
		}
	})

	assertSpanStr(t, attrs, ateattr.AtespaceKey, testAtespace)
	assertSpanStr(t, attrs, ateattr.ActorNameKey, testActorID)
	assertSpanStr(t, attrs, ateattr.TemplateNameKey, "tmpl1")
	assertSpanStr(t, attrs, ateattr.TemplateNamespaceKey, ns)
	// uid is server-assigned on create, so assert it is present and non-empty
	// rather than a fixed value.
	if v, ok := attrs[ateattr.ActorUIDKey]; !ok || v.Type() != attribute.STRING || v.AsString() == "" {
		t.Errorf("%s = %v, want non-empty server-assigned uid", ateattr.ActorUIDKey, v.Emit())
	}
	if v, ok := attrs[ateattr.ActorVersionKey]; !ok || v.Type() != attribute.INT64 || v.AsInt64() != 1 {
		t.Errorf("%s = %v, want int64 1", ateattr.ActorVersionKey, v.Emit())
	}
}

func TestValidateCreateActorRequest(t *testing.T) {
	valid := func(mutate ...func(*ateapipb.CreateActorRequest)) *ateapipb.CreateActorRequest {
		req := &ateapipb.CreateActorRequest{
			Actor: validActor(nil),
		}
		for _, m := range mutate {
			m(req)
		}
		return req
	}

	tests := []struct {
		name string
		line string
		req  *ateapipb.CreateActorRequest
		want field.ErrorList
	}{{
		"valid",
		line(),
		valid(),
		nil,
	}, {
		"missing actor",
		line(),
		valid(func(r *ateapipb.CreateActorRequest) { r.Actor = nil }),
		field.ErrorList{field.Required(field.NewPath("actor"), "")},
	}}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s (L%s)", tt.name, tt.line), func(t *testing.T) {
			assertValidateErr(t, validateCreateActorRequest(context.Background(), tt.req), tt.want)
		})
	}
}

func TestCreateActor_RejectsDifferentTemplateForDataSnapshot(t *testing.T) {
	ns := namespaceForTest("ns-data-snapshot-template")
	tc := setupTest(t, ns)
	defer tc.cleanup()
	createTemplate(t, tc, ns)
	createTemplateWithSelector(t, tc, ns, "tmpl2", nil)

	tmpl, err := tc.actorTemplateLister.ActorTemplates(ns).Get("tmpl1")
	if err != nil {
		t.Fatalf("Get source ActorTemplate: %v", err)
	}
	snapshot, err := tc.persistence.CreateActorSnapshot(context.Background(), &ateapipb.ActorSnapshot{
		Metadata:         &ateapipb.ResourceMetadata{Atespace: testAtespace, Name: "data-snapshot"},
		SourceActor:      &ateapipb.ObjectRef{Atespace: testAtespace, Name: "source"},
		ActorTemplateUid: string(tmpl.GetUID()),
		ContentScope:     ateapipb.SnapshotContentScope_SNAPSHOT_CONTENT_SCOPE_DATA,
		SnapshotUri:      "gs://snapshots/snapshots/" + testAtespace + "/data-snapshot",
	})
	if err != nil {
		t.Fatalf("CreateActorSnapshot: %v", err)
	}
	if _, err := tc.persistence.TagActorSnapshot(context.Background(), testAtespace, snapshot.GetMetadata().GetName(), &ateapipb.ActorSnapshotTag{
		Metadata: &ateapipb.ResourceMetadata{Atespace: testAtespace, Name: "data-snapshot"},
		Scope:    ateapipb.ActorSnapshotTagScope_ACTOR_SNAPSHOT_TAG_SCOPE_ATESPACE,
	}); err != nil {
		t.Fatalf("TagActorSnapshot: %v", err)
	}

	_, err = tc.service.CreateActor(context.Background(), &ateapipb.CreateActorRequest{
		Actor: &ateapipb.Actor{
			Metadata:               &ateapipb.ResourceMetadata{Atespace: testAtespace, Name: "clone"},
			ActorTemplateNamespace: ns,
			ActorTemplateName:      "tmpl2",
		},
		SourceSnapshot: &ateapipb.ActorSnapshotRef{Reference: &ateapipb.ActorSnapshotRef_Tag{Tag: &ateapipb.ObjectRef{Atespace: testAtespace, Name: "data-snapshot"}}},
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("CreateActor status = %v, want FailedPrecondition", status.Code(err))
	}
}

func TestCreateActor_RejectsSnapshotWithExternalVolumes(t *testing.T) {
	ns := namespaceForTest("ns-snapshot-external-volume")
	tc := setupTest(t, ns)
	defer tc.cleanup()
	template, err := tc.substrateClient.ApiV1alpha1().ActorTemplates(ns).Create(context.Background(), &atev1alpha1.ActorTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "tmpl1", Namespace: ns},
		Spec: atev1alpha1.ActorTemplateSpec{
			PauseImage:      "pause@sha256:abc",
			SnapshotsConfig: atev1alpha1.SnapshotsConfig{Location: "gs://snapshots"},
			Containers: []atev1alpha1.Container{{
				Name: "main", Image: "main@sha256:abc", VolumeMounts: []atev1alpha1.VolumeMount{{Name: "data", MountPath: "/data"}},
			}},
			Volumes: []atev1alpha1.Volume{{
				Name: "data",
				VolumeSource: atev1alpha1.VolumeSource{ExternalVolumeTemplate: &atev1alpha1.ExternalVolumeTemplate{
					Capacity: resource.MustParse("1Gi"), StorageClassName: "standard",
				}},
			}},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Create ActorTemplate: %v", err)
	}
	if err := wait.PollUntilContextTimeout(context.Background(), 100*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		got, err := tc.actorTemplateLister.ActorTemplates(ns).Get("tmpl1")
		return err == nil && len(got.Spec.Volumes) == 1, nil
	}); err != nil {
		t.Fatalf("wait for ActorTemplate update: %v", err)
	}
	snapshot, err := tc.persistence.CreateActorSnapshot(context.Background(), &ateapipb.ActorSnapshot{
		Metadata:         &ateapipb.ResourceMetadata{Atespace: testAtespace, Name: "external-volume-snapshot"},
		ActorTemplateUid: string(template.GetUID()),
		SnapshotUri:      "gs://snapshots/snapshots/" + testAtespace + "/external-volume-snapshot",
	})
	if err != nil {
		t.Fatalf("CreateActorSnapshot: %v", err)
	}
	tagRef := &ateapipb.ObjectRef{Atespace: testAtespace, Name: "external-volume-snapshot"}
	if _, err := tc.persistence.TagActorSnapshot(context.Background(), testAtespace, snapshot.GetMetadata().GetName(), &ateapipb.ActorSnapshotTag{
		Metadata: &ateapipb.ResourceMetadata{Atespace: tagRef.GetAtespace(), Name: tagRef.GetName()},
		Scope:    ateapipb.ActorSnapshotTagScope_ACTOR_SNAPSHOT_TAG_SCOPE_ATESPACE,
	}); err != nil {
		t.Fatalf("TagActorSnapshot: %v", err)
	}

	_, err = tc.service.CreateActor(context.Background(), &ateapipb.CreateActorRequest{
		Actor: &ateapipb.Actor{
			Metadata:               &ateapipb.ResourceMetadata{Atespace: testAtespace, Name: "clone"},
			ActorTemplateNamespace: ns,
			ActorTemplateName:      "tmpl1",
		},
		SourceSnapshot: &ateapipb.ActorSnapshotRef{Reference: &ateapipb.ActorSnapshotRef_Tag{Tag: tagRef}},
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("CreateActor status = %v, want FailedPrecondition", status.Code(err))
	}
}
