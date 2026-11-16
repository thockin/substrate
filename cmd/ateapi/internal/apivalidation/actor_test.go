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
	"fmt"
	"strings"
	"testing"

	"go.ate.dev/substrate/api/ateapi/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateCreateActorRequest(t *testing.T) {
	// This test verifies validation of user input for creation.  Since status
	// is scrubbed on input, we don't need to test the status field here, other
	// than that it is optional. TestValidateActorUpdate covers status
	// validation and updates.
	validReq := func(actor *ateapi.Actor, mods ...func(actor *ateapi.CreateActorRequest)) *ateapi.CreateActorRequest {
		req := &ateapi.CreateActorRequest{
			Actor: actor,
		}
		for _, m := range mods {
			m(req)
		}
		return req
	}
	withStatus := withActorStatus
	withMetadata := withActorMetadata
	withActorTemplate := withActorActorTemplate
	withSourceTag := withActorSourceTag
	withWorkerSelector := withActorWorkerSelector

	tests := []struct {
		name string
		req  *ateapi.CreateActorRequest
		want field.ErrorList
	}{{
		"valid",
		validReq(validActor()),
		nil,
	}, {
		"valid with status",
		validReq(validActor(withStatus())),
		nil, // ignored on input
	}, {
		"missing actor",
		&ateapi.CreateActorRequest{Actor: nil},
		field.ErrorList{field.Required(field.NewPath("actor"), "")},
	}, {
		"missing actor.metadata",
		validReq(validActor(func(a *ateapi.Actor) { a.Metadata = nil })),
		field.ErrorList{field.Required(field.NewPath("actor", "metadata"), "")},
	}, {
		"missing actor.metadata.atespace",
		validReq(validActor(withMetadata(func(m *ateapi.ResourceMetadata) { m.Atespace = "" }))),
		field.ErrorList{field.Required(field.NewPath("actor", "metadata", "atespace"), "")},
	}, {
		"invalid actor.metadata.atespace",
		validReq(validActor(withMetadata(func(m *ateapi.ResourceMetadata) { m.Atespace = "NS1" }))),
		field.ErrorList{
			field.Invalid(field.NewPath("actor", "metadata", "atespace"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		"missing actor.metadata.name",
		validReq(validActor(withMetadata(func(m *ateapi.ResourceMetadata) { m.Name = "" }))),
		field.ErrorList{field.Required(field.NewPath("actor", "metadata", "name"), "")},
	}, {
		"invalid actor.metadata.name",
		validReq(validActor(withMetadata(func(m *ateapi.ResourceMetadata) { m.Name = "ID1" }))),
		field.ErrorList{
			field.Invalid(field.NewPath("actor", "metadata", "name"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		"valid actor.actor_template",
		validReq(validActor(withActorTemplate("as", "tmpl"))),
		nil,
	}, {
		"missing actor.actor_template",
		validReq(validActor(func(a *ateapi.Actor) { a.ActorTemplate = nil })),
		field.ErrorList{field.Required(field.NewPath("actor", "actor_template"), "")},
	}, {
		"missing actor.actor_template.atespace",
		validReq(validActor(withActorTemplate("", "tmpl"))),
		field.ErrorList{field.Required(field.NewPath("actor", "actor_template", "atespace"), "")},
	}, {
		"invalid actor.actor_template.atespace",
		validReq(validActor(withActorTemplate("invalid value", "tmpl"))),
		field.ErrorList{
			field.Invalid(field.NewPath("actor", "actor_template", "atespace"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		"missing actor.actor_template.name",
		validReq(validActor(withActorTemplate("as", ""))),
		field.ErrorList{field.Required(field.NewPath("actor", "actor_template", "name"), "")},
	}, {
		"invalid actor.actor_template.name",
		validReq(validActor(withActorTemplate("as", "invalid value"))),
		field.ErrorList{
			field.Invalid(field.NewPath("actor", "actor_template", "name"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		"valid worker_selector",
		validReq(validActor(withWorkerSelector(map[string]string{"tier": "1"}))),
		nil,
	}, {
		"worker_selector with nil match_labels",
		validReq(validActor(func(a *ateapi.Actor) { a.WorkerSelector = &ateapi.Selector{} })),
		field.ErrorList{field.Invalid(field.NewPath("actor", "worker_selector"), nil, "one of").WithOrigin("union")},
	}, {
		"worker_selector with empty match_labels",
		validReq(validActor(withWorkerSelector(map[string]string{}))),
		field.ErrorList{field.Invalid(field.NewPath("actor", "worker_selector"), nil, "one of").WithOrigin("union")},
	}, {
		"worker_selector with exactly max match_labels",
		validReq(validActor(withWorkerSelector(selectorLabelsOfSize(10)))),
		nil,
	}, {
		"too many worker_selector.match_labels",
		validReq(validActor(withWorkerSelector(selectorLabelsOfSize(11)))),
		field.ErrorList{
			field.TooMany(field.NewPath("actor", "worker_selector", "match_labels"), 11, 10).WithOrigin("maxProperties"),
		},
	}, {
		"invalid worker_selector label key",
		validReq(validActor(withWorkerSelector(map[string]string{"bad key!": "1"}))),
		field.ErrorList{
			field.Invalid(field.NewPath("actor", "worker_selector", "match_labels"), "bad key!", "").WithOrigin("format=k8s-label-key"),
		},
	}, {
		"invalid worker_selector label value",
		validReq(validActor(withWorkerSelector(map[string]string{"tier": "not valid!"}))),
		field.ErrorList{
			field.Invalid(field.NewPath("actor", "worker_selector", "match_labels").Key("tier"), "not valid!", "").WithOrigin("format=k8s-label-value"),
		},
	}, {
		"valid actor.source_tag",
		validReq(validActor(withSourceTag("as", "tag"))),
		nil,
	}, {
		"missing actor.source_tag.atespace",
		validReq(validActor(withSourceTag("", "tag"))),
		field.ErrorList{field.Required(field.NewPath("actor", "source_tag", "atespace"), "")},
	}, {
		"invalid actor.source_tag.atespace",
		validReq(validActor(withSourceTag("invalid value", "tag"))),
		field.ErrorList{
			field.Invalid(field.NewPath("actor", "source_tag", "atespace"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		"missing actor.source_tag.name",
		validReq(validActor(withSourceTag("as", ""))),
		field.ErrorList{field.Required(field.NewPath("actor", "source_tag", "name"), "")},
	}, {
		"invalid actor.source_tag.name",
		validReq(validActor(withSourceTag("as", "invalid value"))),
		field.ErrorList{
			field.Invalid(field.NewPath("actor", "source_tag", "name"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateCreateActorRequest(context.Background(), tt.req), tt.want)
		})
	}
}

func TestValidateActorUpdate(t *testing.T) {
	// This test validates input and output fields, including status.  It also
	// tests updates to all fields.  This is where the majority of validation
	// test cases should live.
	validInput := validActor
	withStatus := withActorStatus
	validOutput := func(mods ...func(*ateapi.Actor)) *ateapi.Actor {
		allMods := []func(*ateapi.Actor){withStatus()} // this needs to go first
		allMods = append(allMods, mods...)
		a := validActor(allMods...)
		return a
	}
	withMetadata := withActorMetadata
	withWorkerSelector := withActorWorkerSelector
	withActorTemplate := withActorActorTemplate
	withSourceTag := withActorSourceTag
	withState := withActorStatusState
	withAssignedNode := withActorStatusAssignedNode
	withLastAssignedGeneration := withActorStatusLastAssignedGeneration
	withSnapshot := withActorStatusSnapshot
	withMultipleSnapshots := withActorStatusMultipleSnapshots
	withExternalVolume := withActorStatusExternalVolume
	withMultipleExternalVolumes := withActorStatusMultipleExternalVolumes
	withCrash := withActorStatusCrash
	withCrashMessage := withActorCrashMessage
	withWorkerAssignment := withActorStatusWorkerAssignment
	withPodIPs := withWorkerAssignmentPodIPs
	withWorkerEpoch := withWorkerAssignmentEpoch
	withSpiffeID := withWorkerAssignmentSpiffeID
	withSandboxRuntime := withWorkerAssignmentSandboxRuntime
	withSandboxClass := withSandboxRuntimeClass

	tests := []struct {
		name   string
		oldVal *ateapi.Actor
		newVal *ateapi.Actor
		// want is the result with no preview gates enabled.
		want field.ErrorList
		// wantWithPreview maps a --preview value to the result with those
		// gates enabled.
		wantWithPreview map[string]field.ErrorList
	}{{
		name:   "valid",
		oldVal: validInput(),
		newVal: validOutput(),
	}, {
		name:   "missing actor.metadata",
		oldVal: validInput(),
		newVal: validOutput(func(a *ateapi.Actor) { a.Metadata = nil }),
		want:   field.ErrorList{field.Required(field.NewPath("metadata"), "")},
	}, {
		name:   "missing actor.metadata.atespace",
		oldVal: validInput(),
		newVal: validOutput(withMetadata(func(m *ateapi.ResourceMetadata) { m.Atespace = "" })),
		want: field.ErrorList{
			field.Required(field.NewPath("metadata", "atespace"), ""),
			field.Invalid(field.NewPath("metadata", "atespace"), nil, "").WithOrigin("immutable"),
		},
	}, {
		name:   "invalid actor.metadata.atespace",
		oldVal: validInput(),
		newVal: validOutput(withMetadata(func(m *ateapi.ResourceMetadata) { m.Atespace = "invalid value" })),
		want:   field.ErrorList{field.Invalid(field.NewPath("metadata", "atespace"), nil, "").WithOrigin("immutable")},
	}, {
		name:   "missing actor.metadata.name",
		oldVal: validInput(),
		newVal: validOutput(withMetadata(func(m *ateapi.ResourceMetadata) { m.Name = "" })),
		want: field.ErrorList{
			field.Required(field.NewPath("metadata", "name"), ""),
			field.Invalid(field.NewPath("metadata", "name"), nil, "").WithOrigin("immutable"),
		},
	}, {
		name:   "invalid actor.metadata.name",
		oldVal: validInput(),
		newVal: validOutput(withMetadata(func(m *ateapi.ResourceMetadata) { m.Name = "invalid value" })),
		want:   field.ErrorList{field.Invalid(field.NewPath("metadata", "name"), nil, "").WithOrigin("immutable")},
	}, {
		name:   "change actor.actor_template is allowed",
		oldVal: validInput(withActorTemplate("as1", "nm1")),
		newVal: validOutput(withActorTemplate("as2", "nm2")),
	}, {
		name:   "clear actor.actor_template",
		oldVal: validInput(withActorTemplate("as", "nm")),
		newVal: validOutput(func(a *ateapi.Actor) { a.ActorTemplate = nil }),
		want:   field.ErrorList{field.Required(field.NewPath("actor_template"), "")},
	}, {
		name:   "add actor.source_tag",
		oldVal: validInput(),
		newVal: validOutput(withSourceTag("as", "nm")),
		want:   field.ErrorList{field.Invalid(field.NewPath("source_tag"), nil, "").WithOrigin("immutable")},
	}, {
		name:   "clear actor.source_tag",
		oldVal: validInput(withSourceTag("as", "nm")),
		newVal: validOutput(func(a *ateapi.Actor) { a.SourceTag = nil }),
		want:   field.ErrorList{field.Invalid(field.NewPath("source_tag"), nil, "").WithOrigin("immutable")},
	}, {
		name:   "change actor.source_tag",
		oldVal: validInput(withSourceTag("as1", "nm1")),
		newVal: validOutput(withSourceTag("as2", "nm2")),
		want:   field.ErrorList{field.Invalid(field.NewPath("source_tag"), nil, "").WithOrigin("immutable")},
	}, {
		name:   "set valid worker_selector",
		oldVal: validInput(),
		newVal: validOutput(withWorkerSelector(map[string]string{"tier": "1"})),
	}, {
		name:   "clear worker_selector",
		oldVal: validInput(withWorkerSelector(map[string]string{"tier": "1"})),
		newVal: validOutput(),
	}, {
		name:   "modify worker_selector",
		oldVal: validInput(withWorkerSelector(map[string]string{"tier": "1"})),
		newVal: validOutput(withWorkerSelector(map[string]string{"tier": "2"})),
	}, {
		name:   "invalid worker_selector with nil match_labels",
		oldVal: validInput(),
		newVal: validOutput(func(a *ateapi.Actor) { a.WorkerSelector = &ateapi.Selector{} }),
		want:   field.ErrorList{field.Invalid(field.NewPath("worker_selector"), nil, "one of").WithOrigin("union")},
	}, {
		name:   "invalid worker_selector label key",
		oldVal: validInput(),
		newVal: validOutput(withWorkerSelector(map[string]string{"bad key": "2"})),
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker_selector", "match_labels"), nil, "").WithOrigin("format=k8s-label-key"),
		},
	}, {
		name:   "invalid worker_selector label value",
		oldVal: validInput(),
		newVal: validOutput(withWorkerSelector(map[string]string{"tier": "bad value"})),
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker_selector", "match_labels").Key("tier"), nil, "").WithOrigin("format=k8s-label-value"),
		},
	}, {
		name:   "too many worker_selector.match_labels",
		oldVal: validInput(),
		newVal: validOutput(withWorkerSelector(selectorLabelsOfSize(11))),
		want: field.ErrorList{
			field.TooMany(field.NewPath("worker_selector", "match_labels"), 11, 10).WithOrigin("maxProperties"),
		},
	}, {
		name:   "unspecified actor.status",
		oldVal: validInput(withStatus()),
		newVal: validOutput(func(a *ateapi.Actor) { a.Status = nil }),
		want:   field.ErrorList{field.Required(field.NewPath("status"), "")},
	}, {
		name:   "unspecified actor.status.state",
		oldVal: validInput(),
		newVal: validOutput(withStatus(withState(ateapi.ActorState_ACTOR_STATE_UNSPECIFIED))),
		want:   field.ErrorList{field.Required(field.NewPath("status", "state"), "")},
	}, {
		name:   "change actor.status.state",
		oldVal: validOutput(withStatus(withState(ateapi.ActorState_ACTOR_STATE_RUNNING))),
		newVal: validOutput(withStatus(withState(ateapi.ActorState_ACTOR_STATE_CRASHED))),
	}, {
		name:   "negative actor.status.state",
		oldVal: validInput(),
		newVal: validOutput(withStatus(withState(-1))),
		want:   field.ErrorList{field.Invalid(field.NewPath("status", "state"), nil, "").WithOrigin("minimum")},
	}, {
		name:   "just out of bounds actor.status.state",
		oldVal: validInput(),
		newVal: validOutput(withStatus(withState(ateapi.ActorState_ACTOR_STATE_REVERTING + 1))),
		want:   field.ErrorList{field.Invalid(field.NewPath("status", "state"), nil, "").WithOrigin("maximum")},
	}, {
		name:   "invalid actor.status.state",
		oldVal: validInput(),
		newVal: validOutput(withStatus(withState(1234567890))),
		want:   field.ErrorList{field.Invalid(field.NewPath("status", "state"), nil, "").WithOrigin("maximum")},
	}, {
		name:   "set valid actor.status.worker_assignment, IPv4",
		oldVal: validInput(withStatus()),
		newVal: validOutput(withStatus(withWorkerAssignment(withPodIPs("1.2.3.4")))),
	}, {
		name:   "set valid actor.status.worker_assignment, IPv6",
		oldVal: validInput(withStatus()),
		newVal: validOutput(withStatus(withWorkerAssignment(withPodIPs("1234::5678")))),
	}, {
		name:   "set valid actor.status.worker_assignment, dual-stack",
		oldVal: validInput(withStatus()),
		newVal: validOutput(withStatus(withWorkerAssignment(withPodIPs("1234::5678", "1.2.3.4")))),
	}, {
		name:   "set valid actor.status.worker_assignment with every optional field",
		oldVal: validInput(withStatus()),
		newVal: validOutput(
			withStatus(
				withWorkerAssignment(
					withWorkerEpoch(3),
					withSpiffeID("spiffe://cluster.local/ns/ns/sa/ateworker"),
					withSandboxRuntime(),
				),
			),
		),
	}, {
		name:   "invalid actor.status.worker_assignment.worker_pod_ips: two IPv6",
		oldVal: validInput(withStatus()),
		newVal: validOutput(withStatus(withWorkerAssignment(withPodIPs("1234::5678", "1234::9")))),
		want:   field.ErrorList{field.Invalid(field.NewPath("status", "worker_assignment", "worker_pod_ips").Index(1), nil, "")},
	}, {
		name:   "invalid actor.status.worker_assignment.worker_pod_ips: too many",
		oldVal: validInput(withStatus()),
		newVal: validOutput(withStatus(withWorkerAssignment(withPodIPs("1.2.3.4", "1234::5678", "1.2.3.5")))),
		want:   field.ErrorList{field.TooMany(field.NewPath("status", "worker_assignment", "worker_pod_ips"), 3, 2).WithOrigin("maxItems")},
	}, {
		// because we have manual IP format validation, let's be sure
		name:   "invalid actor.status.worker_assignment.worker_pod_ips: leading 0s",
		oldVal: validInput(),
		newVal: validOutput(withStatus(withWorkerAssignment(withPodIPs("001.002.003.004")))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("status", "worker_assignment", "worker_pod_ips").Index(0), nil, "").WithOrigin("format=ip-strict"),
		},
	}, {
		// because we have manual IP format validation, let's be sure
		name:   "invalid actor.status.worker_assignment.worker_pod_ips: non-canonical",
		oldVal: validInput(),
		newVal: validOutput(withStatus(withWorkerAssignment(withPodIPs("0012::0034")))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("status", "worker_assignment", "worker_pod_ips").Index(0), nil, "").WithOrigin("format=ip-strict"),
		},
	}, {
		name:   "set valid actor.status.worker_assignment.worker_spiffe_id",
		oldVal: validInput(withStatus()),
		newVal: validOutput(withStatus(withWorkerAssignment(withSpiffeID("spiffe://cluster.local/ns/ns/sa/ateworker")))),
	}, {
		name:   "invalid actor.status.worker_assignment.worker_spiffe_id",
		oldVal: validInput(withStatus()),
		newVal: validOutput(withStatus(withWorkerAssignment(withSpiffeID("ns/ns/sa/ateworker")))),
		want:   field.ErrorList{field.Invalid(field.NewPath("status", "worker_assignment", "worker_spiffe_id"), nil, "")},
	}, {
		name:   "actor.status.worker_assignment.worker_spiffe_id too long",
		oldVal: validInput(withStatus()),
		newVal: validOutput(
			withStatus(
				withWorkerAssignment(withSpiffeID("spiffe://cluster.local/" + strings.Repeat("x", 2048))),
			),
		),
		want: field.ErrorList{
			field.TooLong(field.NewPath("status", "worker_assignment", "worker_spiffe_id"), nil, 2048).WithOrigin("maxLength"),
		},
	}, {
		name:   "negative actor.status.worker_assignment.worker_epoch",
		oldVal: validInput(withStatus()),
		newVal: validOutput(withStatus(withWorkerAssignment(withWorkerEpoch(-1)))),
		want:   field.ErrorList{field.Invalid(field.NewPath("status", "worker_assignment", "worker_epoch"), nil, "").WithOrigin("minimum")},
	}, {
		name:   "set valid actor.status.worker_assignment.sandbox_runtime",
		oldVal: validInput(withStatus()),
		newVal: validOutput(withStatus(withWorkerAssignment(withSandboxRuntime()))),
	}, {
		name:   "invalid actor.status.worker_assignment.sandbox_runtime.sandbox_class",
		oldVal: validInput(withStatus()),
		newVal: validOutput(withStatus(withWorkerAssignment(withSandboxRuntime(withSandboxClass("plain"))))),
		want: field.ErrorList{
			field.NotSupported[string](field.NewPath("status", "worker_assignment", "sandbox_runtime", "sandbox_class"), nil, nil),
		},
	}, {
		name:   "missing actor.status.worker_assignment.sandbox_runtime.sandbox_class",
		oldVal: validInput(withStatus()),
		newVal: validOutput(withStatus(withWorkerAssignment(withSandboxRuntime(withSandboxClass(""))))),
		want:   field.ErrorList{field.Required(field.NewPath("status", "worker_assignment", "sandbox_runtime", "sandbox_class"), "")},
	}, {
		name:   "actor.status.worker_assignment.sandbox_runtime.sandbox_class too long",
		oldVal: validInput(withStatus()),
		newVal: validOutput(
			withStatus(
				withWorkerAssignment(withSandboxRuntime(withSandboxClass(strings.Repeat("g", 64)))),
			),
		),
		want: field.ErrorList{
			field.TooLong(field.NewPath("status", "worker_assignment", "sandbox_runtime", "sandbox_class"), nil, 63).WithOrigin("maxLength"),
			field.NotSupported[string](field.NewPath("status", "worker_assignment", "sandbox_runtime", "sandbox_class"), nil, nil),
		},
	}, {
		name:   "missing actor.status.worker_assignment.sandbox_runtime.version",
		oldVal: validInput(withStatus()),
		newVal: validOutput(
			withStatus(
				withWorkerAssignment(
					withSandboxRuntime(func(r *ateapi.ActorSandboxRuntime) { r.Version = nil }),
				),
			),
		),
		want: field.ErrorList{field.Required(field.NewPath("status", "worker_assignment", "sandbox_runtime", "version"), "")},
	}, {
		name:   "nil actor.status.worker_assignment.sandbox_runtime.version.attributes entry",
		oldVal: validInput(withStatus()),
		newVal: validOutput(
			withStatus(
				withWorkerAssignment(
					withSandboxRuntime(func(r *ateapi.ActorSandboxRuntime) {
						r.Version.Attributes = append(r.Version.Attributes, nil)
					}),
				),
			),
		),
		want: field.ErrorList{field.Required(field.NewPath("status", "worker_assignment", "sandbox_runtime", "version", "attributes").Index(1), "")},
	}, {
		name:   "clear actor.status.worker_assignment",
		oldVal: validInput(withStatus(withWorkerAssignment())),
		newVal: validOutput(),
	}, {
		name:   "modify actor.status.worker_assignment worker process",
		oldVal: validInput(withStatus(withWorkerAssignment())),
		newVal: validOutput(withStatus(withWorkerAssignment(withWorkerEpoch(2), withPodIPs("10.9.8.7")))),
	}, {
		name:   "modify actor.status.worker_assignment.worker",
		oldVal: validInput(withStatus(withWorkerAssignment())),
		newVal: validOutput(
			withStatus(
				withWorkerAssignment(func(wa *ateapi.WorkerAssignment) { wa.Worker.Name = "worker2" }),
			),
		),
		want: field.ErrorList{field.Invalid(field.NewPath("status", "worker_assignment", "worker"), nil, "").WithOrigin("update")},
	}, {
		name:   "modify actor.status.worker_assignment.worker_namespace",
		oldVal: validInput(withStatus(withWorkerAssignment())),
		newVal: validOutput(
			withStatus(
				withWorkerAssignment(func(wa *ateapi.WorkerAssignment) { wa.WorkerNamespace = "ns2" }),
			),
		),
		want: field.ErrorList{field.Invalid(field.NewPath("status", "worker_assignment", "worker_namespace"), nil, "").WithOrigin("update")},
	}, {
		name:   "modify actor.status.worker_assignment.worker_pool",
		oldVal: validInput(withStatus(withWorkerAssignment())),
		newVal: validOutput(
			withStatus(
				withWorkerAssignment(func(wa *ateapi.WorkerAssignment) { wa.WorkerPool = "pool2" }),
			),
		),
		want: field.ErrorList{field.Invalid(field.NewPath("status", "worker_assignment", "worker_pool"), nil, "").WithOrigin("update")},
	}, {
		name:   "modify actor.status.worker_assignment.worker_pod",
		oldVal: validInput(withStatus(withWorkerAssignment())),
		newVal: validOutput(
			withStatus(
				withWorkerAssignment(func(wa *ateapi.WorkerAssignment) { wa.WorkerPod = "pod2" }),
			),
		),
		want: field.ErrorList{field.Invalid(field.NewPath("status", "worker_assignment", "worker_pod"), nil, "").WithOrigin("update")},
	}, {
		name:   "modify actor.status.worker_assignment.worker_pod_uid",
		oldVal: validInput(withStatus(withWorkerAssignment())),
		newVal: validOutput(
			withStatus(
				withWorkerAssignment(func(wa *ateapi.WorkerAssignment) {
					wa.WorkerPodUid = "87654321-4321-4321-4321-cba987654321"
				}),
			),
		),
		want: field.ErrorList{field.Invalid(field.NewPath("status", "worker_assignment", "worker_pod_uid"), nil, "").WithOrigin("update")},
	}, {
		name:   "modify actor.status.worker_assignment.node_name",
		oldVal: validInput(withStatus(withWorkerAssignment())),
		newVal: validOutput(
			withStatus(
				withWorkerAssignment(func(wa *ateapi.WorkerAssignment) { wa.NodeName = "node2" }),
			),
		),
		want: field.ErrorList{field.Invalid(field.NewPath("status", "worker_assignment", "node_name"), nil, "").WithOrigin("update")},
	}, {
		name:   "modify actor.status.worker_assignment.worker_spiffe_id",
		oldVal: validInput(withStatus(withWorkerAssignment(withSpiffeID("spiffe://cluster.local/ns/ns/sa/a")))),
		newVal: validOutput(withStatus(withWorkerAssignment(withSpiffeID("spiffe://cluster.local/ns/ns/sa/b")))),
		want:   field.ErrorList{field.Invalid(field.NewPath("status", "worker_assignment", "worker_spiffe_id"), nil, "").WithOrigin("update")},
	}, {
		name:   "modify actor.status.worker_assignment.sandbox_runtime",
		oldVal: validInput(withStatus(withWorkerAssignment(withSandboxRuntime()))),
		newVal: validOutput(
			withStatus(
				withWorkerAssignment(
					withSandboxRuntime(func(r *ateapi.ActorSandboxRuntime) { r.Version.SchemaVersion = "v2" }),
				),
			),
		),
		want: field.ErrorList{field.Invalid(field.NewPath("status", "worker_assignment", "sandbox_runtime"), nil, "").WithOrigin("update")},
	}, {
		name:   "empty actor.status.worker_assignment",
		oldVal: validInput(),
		newVal: validOutput(
			withStatus(func(s *ateapi.ActorStatus) {
				s.WorkerAssignment = &ateapi.WorkerAssignment{}
			}),
		),
		want: field.ErrorList{
			field.Required(field.NewPath("status", "worker_assignment", "worker"), ""),
			field.Required(field.NewPath("status", "worker_assignment", "worker_namespace"), ""),
			field.Required(field.NewPath("status", "worker_assignment", "worker_pool"), ""),
			field.Required(field.NewPath("status", "worker_assignment", "worker_pod"), ""),
			field.Required(field.NewPath("status", "worker_assignment", "worker_pod_uid"), ""),
			field.Required(field.NewPath("status", "worker_assignment", "worker_pod_ips"), ""),
			field.Required(field.NewPath("status", "worker_assignment", "node_name"), ""),
		},
	}, {
		name:   "invalid actor.status.worker_assignment",
		oldVal: validInput(),
		newVal: validOutput(
			withStatus(
				withWorkerAssignment(func(wa *ateapi.WorkerAssignment) {
					wa.Worker = &ateapi.ObjectRef{Atespace: "not-allowed", Name: "bad value"}
					wa.WorkerNamespace = "invalid namespace"
					wa.WorkerPool = "invalid pool"
					wa.WorkerPod = "invalid pod"
					wa.WorkerPodUid = "invalid UUID"
					wa.WorkerPodIps = []string{"invalid IP"}
					wa.NodeName = "invalid node"
				}),
			),
		),
		want: field.ErrorList{
			field.Forbidden(field.NewPath("status", "worker_assignment", "worker", "atespace"), ""),
			field.Invalid(field.NewPath("status", "worker_assignment", "worker", "name"), nil, "").WithOrigin("format=k8s-short-name"),
			field.Invalid(field.NewPath("status", "worker_assignment", "worker_namespace"), nil, "").WithOrigin("format=k8s-short-name"),
			field.Invalid(field.NewPath("status", "worker_assignment", "worker_pool"), nil, "").WithOrigin("format=k8s-long-name"),
			field.Invalid(field.NewPath("status", "worker_assignment", "worker_pod"), nil, "").WithOrigin("format=k8s-long-name"),
			field.Invalid(field.NewPath("status", "worker_assignment", "worker_pod_uid"), nil, "").WithOrigin("format=k8s-uuid"),
			field.Invalid(field.NewPath("status", "worker_assignment", "worker_pod_ips").Index(0), nil, "").WithOrigin("format=ip-strict"),
			field.Invalid(field.NewPath("status", "worker_assignment", "node_name"), nil, "").WithOrigin("format=k8s-long-name"),
		},
	}, {
		name:   "valid actor.status.assigned_node",
		oldVal: validInput(),
		newVal: validOutput(withStatus(withAssignedNode("node-1"))),
	}, {
		name:   "invalid actor.status.assigned_node",
		oldVal: validInput(),
		newVal: validOutput(withStatus(withAssignedNode("NOT A NODE"))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("status", "assigned_node"), nil, "").WithOrigin("format=k8s-long-name"),
		},
	}, {
		name:   "valid actor.status.last_assigned_generation",
		oldVal: validInput(),
		newVal: validOutput(withStatus(withLastAssignedGeneration(1))),
	}, {
		name:   "increasing actor.status.last_assigned_generation",
		oldVal: validInput(withStatus(withLastAssignedGeneration(2))),
		newVal: validOutput(withStatus(withLastAssignedGeneration(3))),
	}, {
		name:   "unchanged actor.status.last_assigned_generation",
		oldVal: validInput(withStatus(withLastAssignedGeneration(3))),
		newVal: validOutput(withStatus(withLastAssignedGeneration(3))),
	}, {
		name:   "decreasing actor.status.last_assigned_generation",
		oldVal: validInput(withStatus(withLastAssignedGeneration(3))),
		newVal: validOutput(withStatus(withLastAssignedGeneration(1))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("status", "last_assigned_generation"), nil, "").WithOrigin("monotonic"),
		},
	}, {
		name:   "unsetting actor.status.last_assigned_generation",
		oldVal: validInput(withStatus(withLastAssignedGeneration(3))),
		newVal: validOutput(withStatus(withLastAssignedGeneration(0))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("status", "last_assigned_generation"), nil, "").WithOrigin("update"),
		},
	}, {
		name:   "negative actor.status.last_assigned_generation",
		oldVal: validInput(),
		newVal: validOutput(withStatus(withLastAssignedGeneration(-1))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("status", "last_assigned_generation"), nil, "").WithOrigin("minimum"),
		},
	}, {
		name:   "valid empty actor.status.crash",
		oldVal: validInput(),
		newVal: validOutput(withStatus(withCrash())),
	}, {
		name:   "valid actor.status.crash",
		oldVal: validInput(),
		newVal: validOutput(
			withStatus(
				withCrash(
					withCrashMessage("container main exited with status 137"),
					func(c *ateapi.ActorCrash) { c.CrashTime = &timestamppb.Timestamp{Seconds: 867} },
				),
			),
		),
	}, {
		name:   "actor.status.crash.message too long",
		oldVal: validInput(),
		newVal: validOutput(withStatus(withCrash(withCrashMessage(strings.Repeat("x", 4097))))),
		want: field.ErrorList{
			field.TooLong(field.NewPath("status", "crash", "message"), nil, 4096).WithOrigin("maxLength"),
		},
	}, {
		name:   "valid actor.status.snapshots with durable object snapshot",
		oldVal: validInput(),
		newVal: validOutput(withStatus(withSnapshot(withSnapshotDurableSnapshot()))),
	}, {
		name:   "valid actor.status.snapshots actor_template_uid",
		oldVal: validInput(),
		newVal: validOutput(
			withStatus(
				withSnapshot(func(snap *ateapi.Snapshot) {
					snap.ActorTemplateUid = "01234567-89ab-cdef-0123-456789abcdef"
				}),
			),
		),
	}, {
		// Each suspend restamps the UID of the template the snapshot was captured under.
		name: "changing actor.status.snapshots.actor_template_uid is allowed",
		oldVal: validInput(
			withStatus(
				withSnapshot(func(snap *ateapi.Snapshot) {
					snap.ActorTemplateUid = "01234567-89ab-cdef-0123-456789abcdef"
				}),
			),
		),
		newVal: validOutput(
			withStatus(
				withSnapshot(func(snap *ateapi.Snapshot) {
					snap.ActorTemplateUid = "fedcba98-7654-3210-fedc-ba9876543210"
				}),
			),
		),
	}, {
		name:   "invalid actor.status.snapshots.actor_template_uid",
		oldVal: validInput(),
		newVal: validOutput(
			withStatus(
				withSnapshot(func(snap *ateapi.Snapshot) { snap.ActorTemplateUid = "not-a-uuid" }),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("status", "snapshots").Index(0).Child("actor_template_uid"), nil, "").WithOrigin("format=k8s-uuid"),
		},
	}, {
		name:   "valid actor.status.snapshots uuid and locality",
		oldVal: validInput(),
		newVal: validOutput(withStatus(withSnapshot(withSnapshotLocality()))),
	}, {
		name:   "missing actor.status.snapshots.uuid",
		oldVal: validInput(),
		newVal: validOutput(withStatus(withSnapshot(func(snap *ateapi.Snapshot) { snap.Uuid = "" }))),
		want:   field.ErrorList{field.Required(field.NewPath("status", "snapshots").Index(0).Child("uuid"), "")},
	}, {
		name:   "invalid actor.status.snapshots.uuid",
		oldVal: validInput(),
		newVal: validOutput(
			withStatus(
				withSnapshot(func(snap *ateapi.Snapshot) {
					snap.Uuid = "SNAP 1"
				}),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("status", "snapshots").Index(0).Child("uuid"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		name:   "invalid actor.status.snapshots.locality",
		oldVal: validInput(),
		newVal: validOutput(
			withStatus(withSnapshot(func(snap *ateapi.Snapshot) { snap.Locality = "NODE 1" })),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("status", "snapshots").Index(0).Child("locality"), nil, "").WithOrigin("format=k8s-long-name"),
		},
	}, {
		name:   "duplicate actor.status.snapshots generation",
		oldVal: validInput(),
		newVal: validOutput(withStatus(withSnapshot(), withSnapshot())),
		want:   field.ErrorList{field.Duplicate(field.NewPath("status", "snapshots").Index(1), nil)},
	}, {
		name:   "nil actor.status.snapshots entry",
		oldVal: validInput(),
		newVal: validOutput(withStatus(func(s *ateapi.ActorStatus) { s.Snapshots = append(s.Snapshots, nil) })),
		want:   field.ErrorList{field.Required(field.NewPath("status", "snapshots").Index(0), "")},
	}, {
		name:   "too many actor.status.snapshots",
		oldVal: validInput(),
		newVal: validOutput(
			withStatus(withMultipleSnapshots(17)),
		),
		want: field.ErrorList{field.TooMany(field.NewPath("status", "snapshots"), 17, 16).WithOrigin("maxItems")},
	}, {
		name:   "valid actor.status.snapshots.fidelity",
		oldVal: validInput(),
		newVal: validOutput(
			withStatus(
				withSnapshot(func(snap *ateapi.Snapshot) {
					snap.Fidelity = ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_VOLUMES
				}),
			),
		),
	}, {
		name:   "missing actor.status.snapshots.fidelity",
		oldVal: validInput(),
		newVal: validOutput(
			withStatus(
				withSnapshot(func(snap *ateapi.Snapshot) {
					snap.Fidelity = ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_UNSPECIFIED
				}),
			),
		),
		want: field.ErrorList{field.Required(field.NewPath("status", "snapshots").Index(0).Child("fidelity"), "")},
	}, {
		name:   "negative actor.status.snapshots.fidelity",
		oldVal: validInput(),
		newVal: validOutput(
			withStatus(withSnapshot(func(snap *ateapi.Snapshot) { snap.Fidelity = -1 })),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("status", "snapshots").Index(0).Child("fidelity"), nil, "").WithOrigin("minimum"),
		},
	}, {
		name:   "invalid actor.status.snapshots.fidelity",
		oldVal: validInput(),
		newVal: validOutput(
			withStatus(withSnapshot(func(snap *ateapi.Snapshot) { snap.Fidelity = 4 })),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("status", "snapshots").Index(0).Child("fidelity"), nil, "").WithOrigin("maximum"),
		},
	}, {
		// Status is written by the server, so it is not preview-gated: the
		// volume cases below expect the same result with and without preview.
		name:            "no volumes",
		oldVal:          validInput(withStatus()),
		newVal:          validOutput(),
		want:            nil,
		wantWithPreview: map[string]field.ErrorList{"ALL": nil},
	}, {
		name:   "empty external_volumes",
		oldVal: validInput(withStatus()),
		newVal: validOutput(
			withStatus(func(s *ateapi.ActorStatus) {
				s.ExternalVolumes = &ateapi.ExternalVolumes{}
			}),
		),
		want:            nil,
		wantWithPreview: map[string]field.ErrorList{"ALL": nil},
	}, {
		// Set-once fields permit the nil->set transition, so a volume added
		// in an update validates like one added at creation.
		name:            "adding a volume",
		oldVal:          validInput(withStatus()),
		newVal:          validOutput(withStatus(withExternalVolume())),
		want:            nil,
		wantWithPreview: map[string]field.ErrorList{"ALL": nil},
	}, {
		name:            "adding a volume to an actor with no status",
		oldVal:          validInput(),
		newVal:          validOutput(withStatus(withExternalVolume())),
		want:            nil,
		wantWithPreview: map[string]field.ErrorList{"ALL": nil},
	}, {
		name:   "provisioning transition on an existing volume",
		oldVal: validInput(withStatus(withExternalVolume())),
		newVal: validOutput(
			withStatus(
				withExternalVolume(
					withExternalVolumeStorageVolumeID(),
					func(v *ateapi.ExternalVolumeStatus) {
						v.VolumeContext = map[string]string{"attachment": "iqn.2026-08.io.ate:vol-a"}
					},
				),
			),
		),
		want:            nil,
		wantWithPreview: map[string]field.ErrorList{"ALL": nil},
	}, {
		name:   "duplicate external_volumes volume_name",
		oldVal: validInput(withStatus()),
		newVal: validOutput(
			withStatus(
				withExternalVolume(),
				withExternalVolume(),
			),
		),
		want: field.ErrorList{field.Duplicate(field.NewPath("status", "external_volumes", "volumes").Index(1), nil)},
		wantWithPreview: map[string]field.ErrorList{
			"ALL": {field.Duplicate(field.NewPath("status", "external_volumes", "volumes").Index(1), nil)},
		},
	}, {
		name:   "invalid external_volumes entry",
		oldVal: validInput(withStatus()),
		newVal: validOutput(
			withStatus(
				withExternalVolume(func(v *ateapi.ExternalVolumeStatus) {
					v.Name = "NOT A VOLUME"
				}),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("status", "external_volumes", "volumes").Index(0).Child("name"), nil, "").WithOrigin("format=k8s-short-name"),
		},
		wantWithPreview: map[string]field.ErrorList{
			"ALL": {field.Invalid(field.NewPath("status", "external_volumes", "volumes").Index(0).Child("name"), nil, "").WithOrigin("format=k8s-short-name")},
		},
	}, {
		name:   "nil external_volumes entry",
		oldVal: validInput(withStatus()),
		newVal: validOutput(
			withStatus(func(s *ateapi.ActorStatus) {
				s.ExternalVolumes = &ateapi.ExternalVolumes{Volumes: []*ateapi.ExternalVolumeStatus{nil}}
			}),
		),
		want: field.ErrorList{field.Required(field.NewPath("status", "external_volumes", "volumes").Index(0), "")},
		wantWithPreview: map[string]field.ErrorList{
			"ALL": {field.Required(field.NewPath("status", "external_volumes", "volumes").Index(0), "")},
		},
	}, {
		name:   "too many external_volumes",
		oldVal: validInput(withStatus()),
		newVal: validOutput(
			withStatus(withMultipleExternalVolumes(33)),
		),
		want: field.ErrorList{field.TooMany(field.NewPath("status", "external_volumes", "volumes"), 33, 32).WithOrigin("maxItems")},
		wantWithPreview: map[string]field.ErrorList{
			"ALL": {field.TooMany(field.NewPath("status", "external_volumes", "volumes"), 33, 32).WithOrigin("maxItems")},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidate(t, func() field.ErrorList {
				return ValidateActorUpdate(context.Background(), nil, tt.newVal, tt.oldVal, true)
			}, tt.want, tt.wantWithPreview)
		})
	}
}

func TestValidateGetActorRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *ateapi.GetActorRequest
		want field.ErrorList
	}{{
		"valid",
		&ateapi.GetActorRequest{Actor: &ateapi.ObjectRef{Atespace: "ns1", Name: "id1"}},
		nil,
	}, {
		"missing actor",
		&ateapi.GetActorRequest{},
		field.ErrorList{field.Required(field.NewPath("actor"), "")},
	}, {
		"missing actor.atespace",
		&ateapi.GetActorRequest{Actor: &ateapi.ObjectRef{Name: "id1"}},
		field.ErrorList{field.Required(field.NewPath("actor", "atespace"), "")},
	}, {
		"invalid actor.atespace",
		&ateapi.GetActorRequest{Actor: &ateapi.ObjectRef{Atespace: "NS1", Name: "id1"}},
		field.ErrorList{
			field.Invalid(field.NewPath("actor", "atespace"), "NS1", "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		"missing actor.name",
		&ateapi.GetActorRequest{Actor: &ateapi.ObjectRef{Atespace: "ns1"}},
		field.ErrorList{field.Required(field.NewPath("actor", "name"), "")},
	}, {
		"invalid actor.name",
		&ateapi.GetActorRequest{Actor: &ateapi.ObjectRef{Atespace: "ns1", Name: "ID1"}},
		field.ErrorList{field.Invalid(field.NewPath("actor", "name"), "ID1", "").WithOrigin("format=k8s-short-name")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateGetActorRequest(context.Background(), tt.req), tt.want)
		})
	}
}

func TestValidateListActorsRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *ateapi.ListActorsRequest
		want field.ErrorList
	}{{
		"valid, atespace scoped",
		&ateapi.ListActorsRequest{Atespace: "ns1"},
		nil,
	}, {
		// Empty atespace means "all atespaces" (kubectl ate get actors -A).
		"valid, empty atespace means all atespaces",
		&ateapi.ListActorsRequest{},
		nil,
	}, {
		"invalid atespace",
		&ateapi.ListActorsRequest{Atespace: "NS1"},
		field.ErrorList{field.Invalid(field.NewPath("atespace"), "NS1", "").WithOrigin("format=k8s-short-name")},
	}, {
		"valid, positive page_size",
		&ateapi.ListActorsRequest{Atespace: "ns1", Options: &ateapi.ListOptions{PageSize: 10}},
		nil,
	}, {
		"negative page_size",
		&ateapi.ListActorsRequest{Atespace: "ns1", Options: &ateapi.ListOptions{PageSize: -1}},
		field.ErrorList{field.Invalid(field.NewPath("options", "page_size"), int32(-1), "").WithOrigin("minimum")},
	}, {
		"valid page_token",
		&ateapi.ListActorsRequest{Atespace: "ns1", Options: &ateapi.ListOptions{PageToken: strings.Repeat("x", 256)}},
		nil,
	}, {
		"too-large page_token",
		&ateapi.ListActorsRequest{Atespace: "ns1", Options: &ateapi.ListOptions{PageToken: strings.Repeat("x", 257)}},
		field.ErrorList{
			field.TooLongCharacters(field.NewPath("options", "page_token"), "", 256).WithOrigin("maxLength"),
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateListActorsRequest(context.Background(), tt.req), tt.want)
		})
	}
}

func TestValidateUpdateActorRequest(t *testing.T) {
	withUID := withMetadataUID
	withVersion := withMetadataVersion
	// This test verifies validation of user input for update.  Since status
	// is scrubbed on input, we don't need to test the status field here, other
	// than that it is optional. TestValidateActorUpdate covers status
	// validation and updates.
	validReq := func(actor *ateapi.Actor, mods ...func(actor *ateapi.UpdateActorRequest)) *ateapi.UpdateActorRequest {
		req := &ateapi.UpdateActorRequest{
			Actor: actor,
		}
		for _, m := range mods {
			m(req)
		}
		return req
	}
	withStatus := withActorStatus
	withMetadata := withActorMetadata

	tests := []struct {
		name string
		req  *ateapi.UpdateActorRequest
		want field.ErrorList
	}{{
		"valid",
		validReq(validActor()),
		nil,
	}, {
		"valid with preconditions",
		validReq(validActor(withMetadata(withUID()), withMetadata(withVersion()))),
		nil,
	}, {
		"valid with status",
		validReq(validActor(withStatus())),
		nil, // ignored on input
	}, {
		"missing actor",
		&ateapi.UpdateActorRequest{Actor: nil},
		field.ErrorList{field.Required(field.NewPath("actor"), "")},
	}, {
		"missing actor.metadata",
		validReq(validActor(func(a *ateapi.Actor) { a.Metadata = nil })),
		field.ErrorList{field.Required(field.NewPath("actor", "metadata"), "")},
	}, {
		"missing actor.metadata.atespace",
		validReq(validActor(withMetadata(func(m *ateapi.ResourceMetadata) { m.Atespace = "" }))),
		field.ErrorList{field.Required(field.NewPath("actor", "metadata", "atespace"), "")},
	}, {
		"invalid actor.metadata.atespace",
		validReq(validActor(withMetadata(func(m *ateapi.ResourceMetadata) { m.Atespace = "NS1" }))),
		field.ErrorList{
			field.Invalid(field.NewPath("actor", "metadata", "atespace"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		"missing actor.metadata.name",
		validReq(validActor(withMetadata(func(m *ateapi.ResourceMetadata) { m.Name = "" }))),
		field.ErrorList{field.Required(field.NewPath("actor", "metadata", "name"), "")},
	}, {
		"invalid actor.metadata.name",
		validReq(validActor(withMetadata(func(m *ateapi.ResourceMetadata) { m.Name = "ID1" }))),
		field.ErrorList{
			field.Invalid(field.NewPath("actor", "metadata", "name"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		"missing actor.metadata.uid precondition",
		validReq(validActor(withMetadata(withVersion()))),
		nil,
	}, {
		"invalid actor.metadata.uid precondition",
		validReq(validActor(withMetadata(func(m *ateapi.ResourceMetadata) { m.Uid = "not-a-uuid" }))),
		field.ErrorList{
			field.Invalid(field.NewPath("actor", "metadata", "uid"), "not-a-uuid", "").WithOrigin("format=k8s-uuid"),
		},
	}, {
		"missing actor.metadata.version precondition",
		validReq(validActor(withMetadata(withUID()))),
		nil,
	}, {
		"negative actor.metadata.version precondition",
		validReq(validActor(withMetadata(func(m *ateapi.ResourceMetadata) { m.Version = -1 }))),
		field.ErrorList{
			field.Invalid(field.NewPath("actor", "metadata", "version"), int64(-1), "").WithOrigin("minimum"),
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateUpdateActorRequest(context.Background(), tt.req), tt.want)
		})
	}
}

func TestValidateDeleteActorRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *ateapi.DeleteActorRequest
		want field.ErrorList
	}{{
		"valid",
		&ateapi.DeleteActorRequest{Actor: &ateapi.ObjectRef{Atespace: "ns1", Name: "id1"}},
		nil,
	}, {
		"missing actor",
		&ateapi.DeleteActorRequest{},
		field.ErrorList{field.Required(field.NewPath("actor"), "")},
	}, {
		"missing actor.atespace",
		&ateapi.DeleteActorRequest{Actor: &ateapi.ObjectRef{Name: "id1"}},
		field.ErrorList{field.Required(field.NewPath("actor", "atespace"), "")},
	}, {
		"invalid actor.atespace",
		&ateapi.DeleteActorRequest{Actor: &ateapi.ObjectRef{Atespace: "NS1", Name: "id1"}},
		field.ErrorList{
			field.Invalid(field.NewPath("actor", "atespace"), "NS1", "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		"missing actor.name",
		&ateapi.DeleteActorRequest{Actor: &ateapi.ObjectRef{Atespace: "ns1"}},
		field.ErrorList{field.Required(field.NewPath("actor", "name"), "")},
	}, {
		"invalid actor.name",
		&ateapi.DeleteActorRequest{Actor: &ateapi.ObjectRef{Atespace: "ns1", Name: "ID1"}},
		field.ErrorList{field.Invalid(field.NewPath("actor", "name"), "ID1", "").WithOrigin("format=k8s-short-name")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateDeleteActorRequest(context.Background(), tt.req), tt.want)
		})
	}
}

func TestValidateResumeActorRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *ateapi.ResumeActorRequest
		want field.ErrorList
	}{{
		"valid",
		&ateapi.ResumeActorRequest{Actor: &ateapi.ObjectRef{Atespace: "ns1", Name: "id1"}},
		nil,
	}, {
		"missing actor",
		&ateapi.ResumeActorRequest{},
		field.ErrorList{field.Required(field.NewPath("actor"), "")},
	}, {
		"missing actor.atespace",
		&ateapi.ResumeActorRequest{Actor: &ateapi.ObjectRef{Name: "id1"}},
		field.ErrorList{field.Required(field.NewPath("actor", "atespace"), "")},
	}, {
		"invalid actor.atespace",
		&ateapi.ResumeActorRequest{Actor: &ateapi.ObjectRef{Atespace: "NS1", Name: "id1"}},
		field.ErrorList{
			field.Invalid(field.NewPath("actor", "atespace"), "NS1", "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		"missing actor.name",
		&ateapi.ResumeActorRequest{Actor: &ateapi.ObjectRef{Atespace: "ns1"}},
		field.ErrorList{field.Required(field.NewPath("actor", "name"), "")},
	}, {
		"invalid actor.name",
		&ateapi.ResumeActorRequest{Actor: &ateapi.ObjectRef{Atespace: "ns1", Name: "ID1"}},
		field.ErrorList{field.Invalid(field.NewPath("actor", "name"), "ID1", "").WithOrigin("format=k8s-short-name")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateResumeActorRequest(context.Background(), tt.req), tt.want)
		})
	}
}

func TestValidateSuspendActorRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *ateapi.SuspendActorRequest
		want field.ErrorList
	}{{
		"valid",
		&ateapi.SuspendActorRequest{Actor: &ateapi.ObjectRef{Atespace: "ns1", Name: "id1"}},
		nil,
	}, {
		"missing actor",
		&ateapi.SuspendActorRequest{},
		field.ErrorList{field.Required(field.NewPath("actor"), "")},
	}, {
		"missing actor.atespace",
		&ateapi.SuspendActorRequest{Actor: &ateapi.ObjectRef{Name: "id1"}},
		field.ErrorList{field.Required(field.NewPath("actor", "atespace"), "")},
	}, {
		"invalid actor.atespace",
		&ateapi.SuspendActorRequest{Actor: &ateapi.ObjectRef{Atespace: "NS1", Name: "id1"}},
		field.ErrorList{
			field.Invalid(field.NewPath("actor", "atespace"), "NS1", "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		"missing actor.name",
		&ateapi.SuspendActorRequest{Actor: &ateapi.ObjectRef{Atespace: "ns1"}},
		field.ErrorList{field.Required(field.NewPath("actor", "name"), "")},
	}, {
		"invalid actor.name",
		&ateapi.SuspendActorRequest{Actor: &ateapi.ObjectRef{Atespace: "ns1", Name: "ID1"}},
		field.ErrorList{field.Invalid(field.NewPath("actor", "name"), "ID1", "").WithOrigin("format=k8s-short-name")},
	}, {
		"synchronous mode",
		&ateapi.SuspendActorRequest{
			Actor:   &ateapi.ObjectRef{Atespace: "ns1", Name: "id1"},
			Options: &ateapi.SuspendOptions{Mode: ateapi.DurabilityMode_DURABILITY_MODE_SYNCHRONOUS},
		},
		nil,
	}, {
		"asynchronous mode",
		&ateapi.SuspendActorRequest{
			Actor:   &ateapi.ObjectRef{Atespace: "ns1", Name: "id1"},
			Options: &ateapi.SuspendOptions{Mode: ateapi.DurabilityMode_DURABILITY_MODE_ASYNCHRONOUS},
		},
		nil,
	}, {
		"mode unspecified",
		&ateapi.SuspendActorRequest{
			Actor:   &ateapi.ObjectRef{Atespace: "ns1", Name: "id1"},
			Options: &ateapi.SuspendOptions{},
		},
		nil,
	}, {
		"mode negative",
		&ateapi.SuspendActorRequest{
			Actor:   &ateapi.ObjectRef{Atespace: "ns1", Name: "id1"},
			Options: &ateapi.SuspendOptions{Mode: ateapi.DurabilityMode(-1)},
		},
		field.ErrorList{field.Invalid(field.NewPath("options", "mode"), nil, "").WithOrigin("minimum")},
	}, {
		"mode unknown",
		&ateapi.SuspendActorRequest{
			Actor:   &ateapi.ObjectRef{Atespace: "ns1", Name: "id1"},
			Options: &ateapi.SuspendOptions{Mode: ateapi.DurabilityMode(3)},
		},
		field.ErrorList{field.Invalid(field.NewPath("options", "mode"), nil, "").WithOrigin("maximum")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateSuspendActorRequest(context.Background(), tt.req), tt.want)
		})
	}
}

// validActor returns a minimal Actor which should pass input validation.
func validActor(mods ...func(*ateapi.Actor)) *ateapi.Actor {
	a := &ateapi.Actor{
		Metadata:      &ateapi.ResourceMetadata{Atespace: "ns1", Name: "id1"},
		ActorTemplate: &ateapi.ObjectRef{Atespace: "ns1", Name: "tmpl1"},
	}
	for _, m := range mods {
		m(a)
	}
	return a
}

// withActorMetadata returns a modifier func (see validActor) which applies
// mods to the actor's metadata.
func withActorMetadata(mods ...func(*ateapi.ResourceMetadata)) func(*ateapi.Actor) {
	return func(a *ateapi.Actor) {
		for _, m := range mods {
			m(a.Metadata)
		}
	}
}

// withActorStatus returns a modifier func (see validActor) which sets the
// actor's status to a valid value with only its required state, and applies
// mods to it.
func withActorStatus(mods ...func(*ateapi.ActorStatus)) func(*ateapi.Actor) {
	return func(a *ateapi.Actor) {
		a.Status = &ateapi.ActorStatus{
			State: ateapi.ActorState_ACTOR_STATE_SUSPENDED,
		}
		for _, m := range mods {
			m(a.Status)
		}
	}
}

// withActorWorkerSelector returns a modifier func (see validActor) which sets
// the actor's worker_selector to match labels.
func withActorWorkerSelector(labels map[string]string) func(*ateapi.Actor) {
	return func(a *ateapi.Actor) {
		a.WorkerSelector = &ateapi.Selector{
			MatchLabels: labels,
		}
	}
}

// withActorActorTemplate returns a modifier func (see validActor) which sets
// the actor's actor_template.
func withActorActorTemplate(atespace, name string) func(*ateapi.Actor) {
	return func(a *ateapi.Actor) { a.ActorTemplate = &ateapi.ObjectRef{Atespace: atespace, Name: name} }
}

// withActorSourceTag returns a modifier func (see validActor) which sets the
// actor's source_tag.
func withActorSourceTag(atespace, name string) func(*ateapi.Actor) {
	return func(a *ateapi.Actor) { a.SourceTag = &ateapi.ObjectRef{Atespace: atespace, Name: name} }
}

// withActorStatusState returns a modifier func (see withActorStatus) which
// sets the state.
func withActorStatusState(state ateapi.ActorState) func(*ateapi.ActorStatus) {
	return func(s *ateapi.ActorStatus) { s.State = state }
}

// withActorStatusAssignedNode returns a modifier func (see withActorStatus)
// which sets the assigned_node.
func withActorStatusAssignedNode(node string) func(*ateapi.ActorStatus) {
	return func(s *ateapi.ActorStatus) { s.AssignedNode = node }
}

// withActorStatusLastAssignedGeneration returns a modifier func (see
// withActorStatus) which sets the last_assigned_generation.
func withActorStatusLastAssignedGeneration(gen int32) func(*ateapi.ActorStatus) {
	return func(s *ateapi.ActorStatus) { s.LastAssignedGeneration = gen }
}

// withActorStatusSnapshot returns a modifier func (see withActorStatus)
// which appends a valid snapshot with mods applied (see validSnapshot).
func withActorStatusSnapshot(mods ...func(*ateapi.Snapshot)) func(*ateapi.ActorStatus) {
	return func(s *ateapi.ActorStatus) { s.Snapshots = append(s.Snapshots, validSnapshot(mods...)) }
}

// withActorStatusMultipleSnapshots returns a modifier func (see
// withActorStatus) which appends n valid snapshots with distinct generations.
func withActorStatusMultipleSnapshots(n int) func(*ateapi.ActorStatus) {
	return func(s *ateapi.ActorStatus) {
		for i := range n {
			withActorStatusSnapshot(func(snap *ateapi.Snapshot) { snap.Generation = int32(i + 1) })(s)
		}
	}
}

// withActorStatusExternalVolume returns a modifier func (see
// withActorStatus) which appends a valid external volume with mods applied
// (see validExternalVolume).
func withActorStatusExternalVolume(mods ...func(*ateapi.ExternalVolumeStatus)) func(*ateapi.ActorStatus) {
	return func(s *ateapi.ActorStatus) {
		if s.ExternalVolumes == nil {
			s.ExternalVolumes = &ateapi.ExternalVolumes{}
		}
		s.ExternalVolumes.Volumes = append(s.ExternalVolumes.Volumes, validExternalVolume(mods...))
	}
}

// withActorStatusMultipleExternalVolumes returns a modifier func (see
// withActorStatus) which appends n valid external volumes with distinct names.
func withActorStatusMultipleExternalVolumes(n int) func(*ateapi.ActorStatus) {
	return func(s *ateapi.ActorStatus) {
		for i := range n {
			withActorStatusExternalVolume(func(v *ateapi.ExternalVolumeStatus) { v.Name = fmt.Sprintf("vol-%d", i) })(s)
		}
	}
}

// withActorStatusCrash returns a modifier func (see withActorStatus) which
// sets the crash to an empty value, since all its fields are optional, and
// applies mods to it.
func withActorStatusCrash(mods ...func(*ateapi.ActorCrash)) func(*ateapi.ActorStatus) {
	return func(s *ateapi.ActorStatus) {
		s.Crash = &ateapi.ActorCrash{}
		for _, m := range mods {
			m(s.Crash)
		}
	}
}

// withActorCrashMessage returns a modifier func (see withActorStatusCrash)
// which sets the message.
func withActorCrashMessage(msg string) func(*ateapi.ActorCrash) {
	return func(c *ateapi.ActorCrash) { c.Message = msg }
}

// withActorStatusWorkerAssignment returns a modifier func (see
// withActorStatus) which sets the worker_assignment to a valid value with
// only its required fields, and applies mods to it.
func withActorStatusWorkerAssignment(mods ...func(*ateapi.WorkerAssignment)) func(*ateapi.ActorStatus) {
	return func(s *ateapi.ActorStatus) {
		s.WorkerAssignment = &ateapi.WorkerAssignment{
			Worker:          &ateapi.ObjectRef{Name: "worker"},
			WorkerNamespace: "ns",
			WorkerPool:      "pool",
			WorkerPod:       "pod",
			WorkerPodUid:    "12345678-1234-1234-1234-123456789abc",
			WorkerPodIps:    []string{"1.2.3.4"},
			NodeName:        "node1",
		}
		for _, m := range mods {
			m(s.WorkerAssignment)
		}
	}
}

// withWorkerAssignmentPodIPs returns a modifier func (see
// withActorStatusWorkerAssignment) which sets the worker_pod_ips.
func withWorkerAssignmentPodIPs(ips ...string) func(*ateapi.WorkerAssignment) {
	return func(wa *ateapi.WorkerAssignment) { wa.WorkerPodIps = ips }
}

// withWorkerAssignmentEpoch returns a modifier func (see
// withActorStatusWorkerAssignment) which sets the worker_epoch.
func withWorkerAssignmentEpoch(epoch int64) func(*ateapi.WorkerAssignment) {
	return func(wa *ateapi.WorkerAssignment) { wa.WorkerEpoch = epoch }
}

// withWorkerAssignmentSpiffeID returns a modifier func (see
// withActorStatusWorkerAssignment) which sets the worker_spiffe_id.
func withWorkerAssignmentSpiffeID(id string) func(*ateapi.WorkerAssignment) {
	return func(wa *ateapi.WorkerAssignment) { wa.WorkerSpiffeId = id }
}

// withWorkerAssignmentSandboxRuntime returns a modifier func (see
// withActorStatusWorkerAssignment) which sets the sandbox_runtime to a valid
// gvisor runtime, and applies mods to it.
func withWorkerAssignmentSandboxRuntime(mods ...func(*ateapi.ActorSandboxRuntime)) func(*ateapi.WorkerAssignment) {
	return func(wa *ateapi.WorkerAssignment) {
		wa.SandboxRuntime = &ateapi.ActorSandboxRuntime{SandboxClass: "gvisor", Version: validSandboxCompat()}
		for _, m := range mods {
			m(wa.SandboxRuntime)
		}
	}
}

// withSandboxRuntimeClass returns a modifier func (see
// withWorkerAssignmentSandboxRuntime) which sets the sandbox_class.
func withSandboxRuntimeClass(class string) func(*ateapi.ActorSandboxRuntime) {
	return func(r *ateapi.ActorSandboxRuntime) { r.SandboxClass = class }
}

func TestValidateMintActorJWTRequest(t *testing.T) {
	valid := func(mods ...func(*ateapi.MintActorJwtRequest)) *ateapi.MintActorJwtRequest {
		r := &ateapi.MintActorJwtRequest{
			Actor:             &ateapi.ObjectRef{Atespace: "ns1", Name: "id1"},
			Audiences:         []string{"https://example.com"},
			ExpirationSeconds: 300,
		}
		for _, m := range mods {
			m(r)
		}
		return r
	}
	withExpiration := func(seconds int64) func(*ateapi.MintActorJwtRequest) {
		return func(r *ateapi.MintActorJwtRequest) { r.ExpirationSeconds = seconds }
	}
	withAudiences := func(audiences ...string) func(*ateapi.MintActorJwtRequest) {
		return func(r *ateapi.MintActorJwtRequest) { r.Audiences = audiences }
	}
	tests := []struct {
		name string
		req  *ateapi.MintActorJwtRequest
		want field.ErrorList
	}{{
		name: "valid",
		req:  valid(),
	}, {
		name: "missing actor",
		req:  valid(func(r *ateapi.MintActorJwtRequest) { r.Actor = nil }),
		want: field.ErrorList{field.Required(field.NewPath("actor"), "")},
	}, {
		name: "valid with several audiences",
		req:  valid(withAudiences("https://a.example.com", "https://b.example.com")),
	}, {
		name: "missing audiences",
		req:  valid(withAudiences()),
		want: field.ErrorList{field.Required(field.NewPath("audiences"), "")},
	}, {
		name: "duplicate audience",
		req:  valid(withAudiences("https://example.com", "https://example.com")),
		want: field.ErrorList{field.Duplicate(field.NewPath("audiences").Index(1), nil)},
	}, {
		name: "too many audiences",
		req:  valid(withAudiences(numberedAudiences(17)...)),
		want: field.ErrorList{field.TooMany(field.NewPath("audiences"), 17, 16).WithOrigin("maxItems")},
	}, {
		name: "audience too long",
		req:  valid(withAudiences("https://" + strings.Repeat("a", 505))),
		want: field.ErrorList{field.TooLong(field.NewPath("audiences").Index(0), nil, 512).WithOrigin("maxLength")},
	}, {
		name: "minimum expiration",
		req:  valid(withExpiration(300)),
	}, {
		name: "maximum expiration",
		req:  valid(withExpiration(3600)),
	}, {
		name: "missing expiration",
		req:  valid(withExpiration(0)),
		want: field.ErrorList{field.Required(field.NewPath("expiration_seconds"), "")},
	}, {
		name: "negative expiration",
		req:  valid(withExpiration(-1)),
		want: field.ErrorList{field.Invalid(field.NewPath("expiration_seconds"), int64(-1), "").WithOrigin("minimum")},
	}, {
		name: "expiration below minimum",
		req:  valid(withExpiration(299)),
		want: field.ErrorList{field.Invalid(field.NewPath("expiration_seconds"), int64(299), "").WithOrigin("minimum")},
	}, {
		name: "expiration above maximum",
		req:  valid(withExpiration(3601)),
		want: field.ErrorList{field.Invalid(field.NewPath("expiration_seconds"), int64(3601), "").WithOrigin("maximum")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateMintActorJWTRequest(context.Background(), tt.req), tt.want)
		})
	}
}

func TestValidateRevertActorRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *ateapi.RevertActorRequest
		want field.ErrorList
	}{{
		"valid",
		&ateapi.RevertActorRequest{Actor: &ateapi.ObjectRef{Atespace: "ns1", Name: "id1"}},
		nil,
	}, {
		"missing actor",
		&ateapi.RevertActorRequest{},
		field.ErrorList{field.Required(field.NewPath("actor"), "")},
	}, {
		"missing actor.atespace",
		&ateapi.RevertActorRequest{Actor: &ateapi.ObjectRef{Name: "id1"}},
		field.ErrorList{field.Required(field.NewPath("actor", "atespace"), "")},
	}, {
		"invalid actor.atespace",
		&ateapi.RevertActorRequest{Actor: &ateapi.ObjectRef{Atespace: "NS1", Name: "id1"}},
		field.ErrorList{
			field.Invalid(field.NewPath("actor", "atespace"), "NS1", "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		"missing actor.name",
		&ateapi.RevertActorRequest{Actor: &ateapi.ObjectRef{Atespace: "ns1"}},
		field.ErrorList{field.Required(field.NewPath("actor", "name"), "")},
	}, {
		"invalid actor.name",
		&ateapi.RevertActorRequest{Actor: &ateapi.ObjectRef{Atespace: "ns1", Name: "ID1"}},
		field.ErrorList{field.Invalid(field.NewPath("actor", "name"), "ID1", "").WithOrigin("format=k8s-short-name")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateRevertActorRequest(context.Background(), tt.req), tt.want)
		})
	}
}

func TestValidateMintWorkerActorCertificateRequest(t *testing.T) {
	valid := func(mods ...func(*ateapi.MintWorkerActorCertificateRequest)) *ateapi.MintWorkerActorCertificateRequest {
		r := &ateapi.MintWorkerActorCertificateRequest{
			Actor:                     &ateapi.ObjectRef{Atespace: "ns1", Name: "id1"},
			ActorUid:                  someActorUID,
			CertificateSigningRequest: []byte("csr"),
			Worker:                    &ateapi.ObjectRef{Name: "worker-1"},
		}
		for _, m := range mods {
			m(r)
		}
		return r
	}
	tests := []struct {
		name string
		req  *ateapi.MintWorkerActorCertificateRequest
		want field.ErrorList
	}{{
		name: "valid",
		req:  valid(),
	}, {
		name: "missing actor",
		req:  valid(func(r *ateapi.MintWorkerActorCertificateRequest) { r.Actor = nil }),
		want: field.ErrorList{field.Required(field.NewPath("actor"), "")},
	}, {
		name: "missing actor.atespace",
		req:  valid(func(r *ateapi.MintWorkerActorCertificateRequest) { r.Actor.Atespace = "" }),
		want: field.ErrorList{field.Required(field.NewPath("actor", "atespace"), "")},
	}, {
		name: "missing actor_uid",
		req:  valid(func(r *ateapi.MintWorkerActorCertificateRequest) { r.ActorUid = "" }),
		want: field.ErrorList{field.Required(field.NewPath("actor_uid"), "")},
	}, {
		name: "invalid actor_uid",
		req:  valid(func(r *ateapi.MintWorkerActorCertificateRequest) { r.ActorUid = "not-a-uuid" }),
		want: field.ErrorList{field.Invalid(field.NewPath("actor_uid"), nil, "").WithOrigin("format=k8s-uuid")},
	}, {
		name: "missing certificate_signing_request",
		req:  valid(func(r *ateapi.MintWorkerActorCertificateRequest) { r.CertificateSigningRequest = nil }),
		want: field.ErrorList{field.Required(field.NewPath("certificate_signing_request"), "")},
	}, {
		name: "certificate_signing_request at the limit",
		req: valid(func(r *ateapi.MintWorkerActorCertificateRequest) {
			r.CertificateSigningRequest = make([]byte, 16384)
		}),
	}, {
		name: "certificate_signing_request too long",
		req: valid(func(r *ateapi.MintWorkerActorCertificateRequest) {
			r.CertificateSigningRequest = make([]byte, 16385)
		}),
		want: field.ErrorList{
			field.TooLong(field.NewPath("certificate_signing_request"), nil, 16384).WithOrigin("maxBytes"),
		},
	}, {
		name: "missing worker",
		req:  valid(func(r *ateapi.MintWorkerActorCertificateRequest) { r.Worker = nil }),
		want: field.ErrorList{field.Required(field.NewPath("worker"), "")},
	}, {
		name: "worker.atespace set",
		req:  valid(func(r *ateapi.MintWorkerActorCertificateRequest) { r.Worker.Atespace = "team-a" }),
		want: field.ErrorList{field.Forbidden(field.NewPath("worker", "atespace"), "")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateMintWorkerActorCertificateRequest(context.Background(), tt.req), tt.want)
		})
	}
}

func TestValidateMintWorkerActorJwtRequest(t *testing.T) {
	valid := func(mods ...func(*ateapi.MintWorkerActorJwtRequest)) *ateapi.MintWorkerActorJwtRequest {
		r := &ateapi.MintWorkerActorJwtRequest{
			Actor:     &ateapi.ObjectRef{Atespace: "ns1", Name: "id1"},
			ActorUid:  someActorUID,
			Audiences: []string{"https://example.com"},
		}
		for _, m := range mods {
			m(r)
		}
		return r
	}
	withAudiences := func(audiences ...string) func(*ateapi.MintWorkerActorJwtRequest) {
		return func(r *ateapi.MintWorkerActorJwtRequest) { r.Audiences = audiences }
	}
	tests := []struct {
		name string
		req  *ateapi.MintWorkerActorJwtRequest
		want field.ErrorList
	}{{
		name: "valid",
		req:  valid(),
	}, {
		name: "missing actor",
		req:  valid(func(r *ateapi.MintWorkerActorJwtRequest) { r.Actor = nil }),
		want: field.ErrorList{field.Required(field.NewPath("actor"), "")},
	}, {
		name: "missing actor.atespace",
		req:  valid(func(r *ateapi.MintWorkerActorJwtRequest) { r.Actor.Atespace = "" }),
		want: field.ErrorList{field.Required(field.NewPath("actor", "atespace"), "")},
	}, {
		name: "missing actor_uid",
		req:  valid(func(r *ateapi.MintWorkerActorJwtRequest) { r.ActorUid = "" }),
		want: field.ErrorList{field.Required(field.NewPath("actor_uid"), "")},
	}, {
		name: "invalid actor_uid",
		req:  valid(func(r *ateapi.MintWorkerActorJwtRequest) { r.ActorUid = "not-a-uuid" }),
		want: field.ErrorList{field.Invalid(field.NewPath("actor_uid"), nil, "").WithOrigin("format=k8s-uuid")},
	}, {
		name: "valid with several audiences",
		req:  valid(withAudiences("https://a.example.com", "https://b.example.com")),
	}, {
		name: "missing audiences",
		req:  valid(withAudiences()),
		want: field.ErrorList{field.Required(field.NewPath("audiences"), "")},
	}, {
		name: "duplicate audience",
		req:  valid(withAudiences("https://example.com", "https://example.com")),
		want: field.ErrorList{field.Duplicate(field.NewPath("audiences").Index(1), nil)},
	}, {
		name: "too many audiences",
		req:  valid(withAudiences(numberedAudiences(17)...)),
		want: field.ErrorList{field.TooMany(field.NewPath("audiences"), 17, 16).WithOrigin("maxItems")},
	}, {
		name: "empty audience",
		req:  valid(withAudiences("")),
		want: field.ErrorList{field.TooShort(field.NewPath("audiences").Index(0), "", 1).WithOrigin("minLength")},
	}, {
		name: "audience too long",
		req:  valid(withAudiences("https://" + strings.Repeat("a", 505))),
		want: field.ErrorList{field.TooLong(field.NewPath("audiences").Index(0), nil, 512).WithOrigin("maxLength")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateMintWorkerActorJwtRequest(context.Background(), tt.req), tt.want)
		})
	}
}

// validExternalVolume returns an ExternalVolumeStatus with only its required
// fields set. withExternalVolumeStorageVolumeID sets the optional one.
func validExternalVolume(mutate ...func(*ateapi.ExternalVolumeStatus)) *ateapi.ExternalVolumeStatus {
	v := &ateapi.ExternalVolumeStatus{
		Name:       "my-vol",
		VolumeType: "mock",
	}
	for _, m := range mutate {
		m(v)
	}
	return v
}

// withExternalVolumeStorageVolumeID returns a modifier func (see
// validExternalVolume) which sets the storage_volume_id to a valid value.
func withExternalVolumeStorageVolumeID() func(*ateapi.ExternalVolumeStatus) {
	return func(v *ateapi.ExternalVolumeStatus) { v.StorageVolumeId = "valid-storage-id" }
}

func TestValidateExternalVolume(t *testing.T) {
	valid := validExternalVolume

	tests := []struct {
		name string
		obj  *ateapi.ExternalVolumeStatus
		want field.ErrorList
	}{{
		name: "valid external volume",
		obj:  valid(),
	}, {
		name: "missing volume name",
		obj:  valid(func(v *ateapi.ExternalVolumeStatus) { v.Name = "" }),
		want: field.ErrorList{field.Required(field.NewPath("name"), "")},
	}, {
		name: "invalid volume name",
		obj:  valid(func(v *ateapi.ExternalVolumeStatus) { v.Name = "NOT A VOLUME" }),
		want: field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name")},
	}, {
		name: "valid external volume with storage_volume_id",
		obj:  valid(withExternalVolumeStorageVolumeID()),
	}, {
		name: "invalid storage volume id with null U+0000",
		obj:  valid(func(v *ateapi.ExternalVolumeStatus) { v.StorageVolumeId = "vol\x00id" }),
		want: field.ErrorList{field.Invalid(field.NewPath("storage_volume_id"), nil, "")},
	}, {
		name: "storage volume id too long",
		obj:  valid(func(v *ateapi.ExternalVolumeStatus) { v.StorageVolumeId = strings.Repeat("x", 257) }),
		want: field.ErrorList{field.TooLong(field.NewPath("storage_volume_id"), nil, 256).WithOrigin("maxLength")},
	}, {
		name: "missing volume type",
		obj:  valid(func(v *ateapi.ExternalVolumeStatus) { v.VolumeType = "" }),
		want: field.ErrorList{field.Required(field.NewPath("volume_type"), "")},
	}, {
		name: "invalid volume type with uppercase",
		obj:  valid(func(v *ateapi.ExternalVolumeStatus) { v.VolumeType = "MockPlugin" }),
		want: field.ErrorList{field.Invalid(field.NewPath("volume_type"), nil, "")},
	}, {
		name: "valid volume type with 253 characters",
		obj:  valid(func(v *ateapi.ExternalVolumeStatus) { v.VolumeType = strings.Repeat("a", 253) }),
	}, {
		name: "invalid volume type exceeding 253 characters",
		obj:  valid(func(v *ateapi.ExternalVolumeStatus) { v.VolumeType = strings.Repeat("a", 254) }),
		want: field.ErrorList{
			field.Invalid(field.NewPath("volume_type"), nil, ""),
			field.TooLong(field.NewPath("volume_type"), nil, 253).WithOrigin("maxLength"),
		},
	}, {
		name: "valid volume with substrate.io prefixed volume type",
		obj:  valid(func(v *ateapi.ExternalVolumeStatus) { v.VolumeType = "substrate.io/mock" }),
	}, {
		name: "storage volume id at the bound",
		obj:  valid(func(v *ateapi.ExternalVolumeStatus) { v.StorageVolumeId = strings.Repeat("x", 256) }),
	}, {
		name: "volume_context with many entries, as a CSI driver may return",
		obj: valid(func(v *ateapi.ExternalVolumeStatus) {
			ctxMap := make(map[string]string, 100)
			for i := range 100 {
				ctxMap[fmt.Sprintf("key-%03d", i)] = "v"
			}
			v.VolumeContext = ctxMap
		}),
	}, {
		name: "volume_context with one long value, as a CSI driver may return",
		obj: valid(func(v *ateapi.ExternalVolumeStatus) {
			v.VolumeContext = map[string]string{"attachment": strings.Repeat("v", 4000)}
		}),
	}, {
		name: "volume_context over the size bound",
		obj: valid(func(v *ateapi.ExternalVolumeStatus) {
			v.VolumeContext = map[string]string{"attachment": strings.Repeat("v", 40*1024)}
		}),
		want: field.ErrorList{field.TooLong(field.NewPath("volume_context"), nil, 40*1024)},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := MakeCreateOp(context.Background())
			assertValidateErr(t, Validate_ExternalVolumeStatus(context.Background(), op, nil, tt.obj, nil), tt.want)
		})
	}
}

func TestValidateExternalVolume_Update(t *testing.T) {
	withStorageVolumeID := withExternalVolumeStorageVolumeID
	valid := validExternalVolume

	tests := []struct {
		name   string
		oldObj *ateapi.ExternalVolumeStatus
		newObj *ateapi.ExternalVolumeStatus
		want   field.ErrorList
	}{{
		name:   "unchanged volume is valid",
		oldObj: valid(withStorageVolumeID()),
		newObj: valid(withStorageVolumeID()),
	}, {
		name:   "name changed is invalid",
		oldObj: valid(func(v *ateapi.ExternalVolumeStatus) { v.Name = "vol1" }),
		newObj: valid(func(v *ateapi.ExternalVolumeStatus) { v.Name = "vol2" }),
		want:   field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("update")},
	}, {
		name:   "storage_volume_id transition from empty to non-empty is valid",
		oldObj: valid(func(v *ateapi.ExternalVolumeStatus) { v.StorageVolumeId = "" }),
		newObj: valid(func(v *ateapi.ExternalVolumeStatus) { v.StorageVolumeId = "vol-id-1" }),
	}, {
		name:   "storage_volume_id changed once set is invalid",
		oldObj: valid(func(v *ateapi.ExternalVolumeStatus) { v.StorageVolumeId = "vol-id-1" }),
		newObj: valid(func(v *ateapi.ExternalVolumeStatus) { v.StorageVolumeId = "vol-id-2" }),
		want:   field.ErrorList{field.Invalid(field.NewPath("storage_volume_id"), nil, "").WithOrigin("update")},
	}, {
		name:   "storage_volume_id unset once set is invalid",
		oldObj: valid(func(v *ateapi.ExternalVolumeStatus) { v.StorageVolumeId = "vol-id-1" }),
		newObj: valid(func(v *ateapi.ExternalVolumeStatus) { v.StorageVolumeId = "" }),
		want:   field.ErrorList{field.Invalid(field.NewPath("storage_volume_id"), nil, "").WithOrigin("update")},
	}, {
		name:   "volume_type changed is invalid",
		oldObj: valid(func(v *ateapi.ExternalVolumeStatus) { v.VolumeType = "mock" }),
		newObj: valid(func(v *ateapi.ExternalVolumeStatus) { v.VolumeType = "pd.csi.storage.gke.io" }),
		want:   field.ErrorList{field.Invalid(field.NewPath("volume_type"), nil, "").WithOrigin("update")},
	}, {
		name:   "volume_context changed is valid",
		oldObj: valid(func(v *ateapi.ExternalVolumeStatus) { v.VolumeContext = nil }),
		newObj: valid(func(v *ateapi.ExternalVolumeStatus) { v.VolumeContext = map[string]string{"foo": "bar"} }),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := MakeUpdateOp(context.Background())
			assertValidateErr(
				t,
				Validate_ExternalVolumeStatus(context.Background(), op, nil, tt.newObj, tt.oldObj),
				tt.want,
			)
		})
	}
}
