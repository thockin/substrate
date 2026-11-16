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

// Worker names are pod UIDs, which are opaque to everything above the syncer.
const (
	apiWorkerName      = "5f2c1a90-7b34-4e6d-8a11-0c3e9d5b7f42"
	apiOtherWorkerName = "1a7e4c83-6d20-4f95-b3c8-9e0a2f6d4b17"
)

// TestValidateCreateWorkerRequest pins the field paths validateWorker reports.
// TestCreateWorker_InvalidArgument drives the same rules through the RPC, but
// only observes the status code.
func TestValidateCreateWorkerRequest(t *testing.T) {
	// This test verifies validation of user input for creation. The RPC scrubs
	// status before validating, so status is absent from the valid shape; when
	// a request does carry one, it is validated like any other field.
	validReq := func(worker *ateapi.Worker) *ateapi.CreateWorkerRequest {
		return &ateapi.CreateWorkerRequest{Worker: worker}
	}
	withMetadata := withWorkerMetadata
	withSpiffeID := withWorkerSpiffeID
	withLabels := withWorkerLabels
	withStatus := withWorkerStatus
	withState := withWorkerStatusState
	withIPs := withWorkerStatusIPs
	withCapacity := withWorkerStatusCapacity
	withAllocated := withWorkerStatusAllocated

	tests := []struct {
		name string
		req  *ateapi.CreateWorkerRequest
		want field.ErrorList
	}{{
		name: "valid unassigned worker",
		req:  validReq(validWorker(apiWorkerName)),
	}, {
		name: "valid with status",
		req:  validReq(validWorker(apiWorkerName, withStatus())),
	}, {
		name: "missing worker",
		req:  &ateapi.CreateWorkerRequest{Worker: nil},
		want: field.ErrorList{field.Required(field.NewPath("worker"), "")},
	}, {
		name: "missing metadata",
		req:  validReq(validWorker(apiWorkerName, func(w *ateapi.Worker) { w.Metadata = nil })),
		want: field.ErrorList{field.Required(field.NewPath("worker", "metadata"), "")},
	}, {
		name: "missing metadata.name",
		req:  validReq(validWorker(apiWorkerName, withMetadata(func(m *ateapi.ResourceMetadata) { m.Name = "" }))),
		want: field.ErrorList{field.Required(field.NewPath("worker", "metadata", "name"), "")},
	}, {
		name: "invalid metadata.name",
		req: validReq(
			validWorker(apiWorkerName,
				withMetadata(func(m *ateapi.ResourceMetadata) {
					m.Name = "not a name"
				})),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "metadata", "name"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		name: "metadata.atespace set on a global-scoped Worker",
		req:  validReq(validWorker(apiWorkerName, withMetadata(withMetadataAtespace("team-a")))),
		want: field.ErrorList{field.Forbidden(field.NewPath("worker", "metadata", "atespace"), "")},
	}, {
		name: "missing worker_namespace",
		req:  validReq(validWorker(apiWorkerName, func(w *ateapi.Worker) { w.WorkerNamespace = "" })),
		want: field.ErrorList{field.Required(field.NewPath("worker", "worker_namespace"), "")},
	}, {
		name: "invalid worker_namespace",
		req:  validReq(validWorker(apiWorkerName, func(w *ateapi.Worker) { w.WorkerNamespace = "NS-1" })),
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "worker_namespace"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		name: "missing worker_pool",
		req:  validReq(validWorker(apiWorkerName, func(w *ateapi.Worker) { w.WorkerPool = "" })),
		want: field.ErrorList{field.Required(field.NewPath("worker", "worker_pool"), "")},
	}, {
		name: "invalid worker_pool",
		req:  validReq(validWorker(apiWorkerName, func(w *ateapi.Worker) { w.WorkerPool = "POOL_1" })),
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "worker_pool"), nil, "").WithOrigin("format=k8s-long-name"),
		},
	}, {
		name: "missing worker_pod",
		req:  validReq(validWorker(apiWorkerName, func(w *ateapi.Worker) { w.WorkerPod = "" })),
		want: field.ErrorList{field.Required(field.NewPath("worker", "worker_pod"), "")},
	}, {
		name: "invalid worker_pod",
		req:  validReq(validWorker(apiWorkerName, func(w *ateapi.Worker) { w.WorkerPod = "POD_1" })),
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "worker_pod"), nil, "").WithOrigin("format=k8s-long-name"),
		},
	}, {
		name: "missing worker_pod_uid",
		req:  validReq(validWorker(apiWorkerName, func(w *ateapi.Worker) { w.WorkerPodUid = "" })),
		want: field.ErrorList{field.Required(field.NewPath("worker", "worker_pod_uid"), "")},
	}, {
		name: "invalid worker_pod_uid",
		req:  validReq(validWorker(apiWorkerName, func(w *ateapi.Worker) { w.WorkerPodUid = "INVALID-UUID" })),
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "worker_pod_uid"), nil, "").WithOrigin("format=k8s-uuid"),
		},
	}, {
		name: "missing node_name",
		req:  validReq(validWorker(apiWorkerName, func(w *ateapi.Worker) { w.NodeName = "" })),
		want: field.ErrorList{field.Required(field.NewPath("worker", "node_name"), "")},
	}, {
		name: "invalid node_name",
		req:  validReq(validWorker(apiWorkerName, func(w *ateapi.Worker) { w.NodeName = "NODE_NAME" })),
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "node_name"), nil, "").WithOrigin("format=k8s-long-name"),
		},
	}, {
		name: "valid dual-stack status.ips",
		req:  validReq(validWorker(apiWorkerName, withStatus(withIPs("fd00::1", "10.1.2.3")))),
	}, {
		name: "invalid status.ips",
		req:  validReq(validWorker(apiWorkerName, withStatus(withIPs("not-an-ip")))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "status", "ips").Index(0), nil, "").WithOrigin("format=ip-strict"),
		},
	}, {
		name: "two status.ips in one family",
		req:  validReq(validWorker(apiWorkerName, withStatus(withIPs("10.1.2.3", "10.1.2.4")))),
		want: field.ErrorList{field.Invalid(field.NewPath("worker", "status", "ips").Index(1), nil, "")},
	}, {
		name: "too many status.ips",
		req:  validReq(validWorker(apiWorkerName, withStatus(withIPs("10.1.2.3", "fd00::1", "10.1.2.4")))),
		want: field.ErrorList{field.TooMany(field.NewPath("worker", "status", "ips"), 3, 2).WithOrigin("maxItems")},
	}, {
		// An invalid entry does not claim its family.
		name: "invalid status.ips entry before a valid one",
		req:  validReq(validWorker(apiWorkerName, withStatus(withIPs("not-an-ip", "10.1.2.3")))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "status", "ips").Index(0), nil, "").WithOrigin("format=ip-strict"),
		},
	}, {
		name: "two IPv6 status.ips",
		req:  validReq(validWorker(apiWorkerName, withStatus(withIPs("fd00::1", "fd00::2")))),
		want: field.ErrorList{field.Invalid(field.NewPath("worker", "status", "ips").Index(1), nil, "")},
	}, {
		name: "negative status epochs",
		req: validReq(
			validWorker(apiWorkerName,
				withStatus(func(s *ateapi.WorkerStatus) {
					s.IssuedEpoch = -1
					s.RegisteredEpoch = -1
					s.ObservedEpoch = -1
				})),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "status", "issued_epoch"), nil, "").WithOrigin("minimum"),
			field.Invalid(field.NewPath("worker", "status", "registered_epoch"), nil, "").WithOrigin("minimum"),
			field.Invalid(field.NewPath("worker", "status", "observed_epoch"), nil, "").WithOrigin("minimum"),
		},
	}, {
		name: "valid status.capacity",
		req: validReq(
			validWorker(apiWorkerName,
				withStatus(
					withCapacity(&ateapi.WorkerResources{
						Actors:        10,
						CpuMillicores: 999999,
						MemoryBytes:   4 << 30,
					}),
				)),
		),
	}, {
		name: "negative status.capacity.cpu_millicores",
		req: validReq(
			validWorker(apiWorkerName,
				withStatus(
					withCapacity(&ateapi.WorkerResources{
						CpuMillicores: -1,
					}),
				)),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "status", "capacity", "cpu_millicores"), nil, "").WithOrigin("minimum"),
		},
	}, {
		name: "status.capacity.cpu_millicores above the maximum",
		req: validReq(
			validWorker(apiWorkerName, withStatus(withCapacity(&ateapi.WorkerResources{CpuMillicores: 1000000}))),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "status", "capacity", "cpu_millicores"), nil, "").WithOrigin("maximum"),
		},
	}, {
		name: "negative status.allocated.memory_bytes",
		req:  validReq(validWorker(apiWorkerName, withStatus(withAllocated(&ateapi.WorkerResources{MemoryBytes: -1})))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "status", "allocated", "memory_bytes"), nil, "").WithOrigin("minimum"),
		},
	}, {
		name: "valid worker_spiffe_id",
		req:  validReq(validWorker(apiWorkerName, withSpiffeID("spiffe://cluster.local/ns/ate-system/sa/ateworker"))),
	}, {
		name: "invalid worker_spiffe_id",
		req:  validReq(validWorker(apiWorkerName, withSpiffeID("https://cluster.local/ns/ate-system/sa/ateworker"))),
		want: field.ErrorList{field.Invalid(field.NewPath("worker", "worker_spiffe_id"), nil, "")},
	}, {
		name: "worker_spiffe_id too long",
		req:  validReq(validWorker(apiWorkerName, withSpiffeID("spiffe://cluster.local/"+strings.Repeat("a", 2048)))),
		want: field.ErrorList{
			field.TooLong(field.NewPath("worker", "worker_spiffe_id"), nil, 2048).WithOrigin("maxLength"),
		},
	}, {
		name: "worker_spiffe_id with a trailing slash",
		req:  validReq(validWorker(apiWorkerName, withSpiffeID("spiffe://cluster.local/ns/ate-system/sa/ateworker/"))),
		want: field.ErrorList{field.Invalid(field.NewPath("worker", "worker_spiffe_id"), nil, "")},
	}, {
		name: "worker_spiffe_id with an uppercase trust domain",
		req:  validReq(validWorker(apiWorkerName, withSpiffeID("spiffe://Cluster.Local/ns/ate-system/sa/ateworker"))),
		want: field.ErrorList{field.Invalid(field.NewPath("worker", "worker_spiffe_id"), nil, "")},
	}, {
		name: "worker_spiffe_id without a trust domain",
		req:  validReq(validWorker(apiWorkerName, withSpiffeID("spiffe://"))),
		want: field.ErrorList{field.Invalid(field.NewPath("worker", "worker_spiffe_id"), nil, "")},
	}, {
		name: "valid labels",
		req: validReq(
			validWorker(apiWorkerName,
				withLabels(map[string]string{
					"tier":             "batch",
					"pool.ate.io/zone": "us-west1-c",
				})),
		),
	}, {
		name: "too many labels",
		req: validReq(
			validWorker(apiWorkerName, withLabels(numberedLabels(65))),
		),
		want: field.ErrorList{field.TooMany(field.NewPath("worker", "labels"), 65, 64).WithOrigin("maxProperties")},
	}, {
		name: "invalid label key",
		req:  validReq(validWorker(apiWorkerName, withLabels(map[string]string{"bad key!": "batch"}))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "labels"), "bad key!", "").WithOrigin("format=k8s-label-key"),
		},
	}, {
		name: "invalid label value",
		req:  validReq(validWorker(apiWorkerName, withLabels(map[string]string{"tier": "not valid!"}))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "labels").Key("tier"), "not valid!", "").WithOrigin("format=k8s-label-value"),
		},
	}, {
		name: "status needs a state",
		req:  validReq(validWorker(apiWorkerName, withStatus(withState(ateapi.WorkerState_WORKER_STATE_UNSPECIFIED)))),
		want: field.ErrorList{field.Required(field.NewPath("worker", "status", "state"), "")},
	}, {
		name: "valid draining status.state",
		req:  validReq(validWorker(apiWorkerName, withStatus(withState(ateapi.WorkerState_WORKER_STATE_DRAINING)))),
	}, {
		// UNAVAILABLE is the last state, so this is the upper bound.
		name: "valid unavailable status.state",
		req:  validReq(validWorker(apiWorkerName, withStatus(withState(ateapi.WorkerState_WORKER_STATE_UNAVAILABLE)))),
	}, {
		name: "status invalid state (too small)",
		req:  validReq(validWorker(apiWorkerName, withStatus(withState(-1)))),
		want: field.ErrorList{field.Invalid(field.NewPath("worker", "status", "state"), nil, "").WithOrigin("minimum")},
	}, {
		name: "status invalid state (too large)",
		req:  validReq(validWorker(apiWorkerName, withStatus(withState(99)))),
		want: field.ErrorList{field.Invalid(field.NewPath("worker", "status", "state"), nil, "").WithOrigin("maximum")},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertValidateErr(t, ValidateCreateWorkerRequest(context.Background(), tc.req), tc.want)
		})
	}
}

func TestValidateGetWorkerRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *ateapi.GetWorkerRequest
		want field.ErrorList
	}{{
		name: "valid",
		req:  &ateapi.GetWorkerRequest{Worker: workerRef(apiWorkerName)},
	}, {
		name: "missing worker",
		req:  &ateapi.GetWorkerRequest{},
		want: field.ErrorList{field.Required(field.NewPath("worker"), "")},
	}, {
		name: "missing worker.name",
		req:  &ateapi.GetWorkerRequest{Worker: &ateapi.ObjectRef{}},
		want: field.ErrorList{field.Required(field.NewPath("worker", "name"), "")},
	}, {
		name: "worker.atespace must be empty",
		req:  &ateapi.GetWorkerRequest{Worker: &ateapi.ObjectRef{Atespace: "team-a", Name: apiWorkerName}},
		want: field.ErrorList{field.Forbidden(field.NewPath("worker", "atespace"), "")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateGetWorkerRequest(context.Background(), tt.req), tt.want)
		})
	}
}

func TestValidateListWorkersRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *ateapi.ListWorkersRequest
		want field.ErrorList
	}{{
		name: "valid",
		req:  &ateapi.ListWorkersRequest{},
	}, {
		name: "valid with options",
		req:  &ateapi.ListWorkersRequest{Options: &ateapi.ListOptions{PageSize: 10, PageToken: "t"}},
	}, {
		name: "negative page_size",
		req:  &ateapi.ListWorkersRequest{Options: &ateapi.ListOptions{PageSize: -1}},
		want: field.ErrorList{field.Invalid(field.NewPath("options", "page_size"), nil, "").WithOrigin("minimum")},
	}, {
		name: "page_token too long",
		req:  &ateapi.ListWorkersRequest{Options: &ateapi.ListOptions{PageToken: strings.Repeat("t", 257)}},
		want: field.ErrorList{field.TooLong(field.NewPath("options", "page_token"), "", 256).WithOrigin("maxLength")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateListWorkersRequest(context.Background(), tt.req), tt.want)
		})
	}
}

func TestValidateListWorkerActorAssignmentsRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *ateapi.ListWorkerActorAssignmentsRequest
		want field.ErrorList
	}{{
		name: "valid",
		req:  &ateapi.ListWorkerActorAssignmentsRequest{Worker: workerRef(apiWorkerName)},
	}, {
		name: "valid with options",
		req: &ateapi.ListWorkerActorAssignmentsRequest{
			Worker:  workerRef(apiWorkerName),
			Options: &ateapi.ListOptions{PageSize: 10},
		},
	}, {
		name: "missing worker",
		req:  &ateapi.ListWorkerActorAssignmentsRequest{},
		want: field.ErrorList{field.Required(field.NewPath("worker"), "")},
	}, {
		name: "worker.atespace must be empty",
		req: &ateapi.ListWorkerActorAssignmentsRequest{
			Worker: &ateapi.ObjectRef{Atespace: "team-a", Name: apiWorkerName},
		},
		want: field.ErrorList{field.Forbidden(field.NewPath("worker", "atespace"), "")},
	}, {
		name: "negative page_size",
		req: &ateapi.ListWorkerActorAssignmentsRequest{
			Worker:  workerRef(apiWorkerName),
			Options: &ateapi.ListOptions{PageSize: -1},
		},
		want: field.ErrorList{field.Invalid(field.NewPath("options", "page_size"), nil, "").WithOrigin("minimum")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateListWorkerActorAssignmentsRequest(context.Background(), tt.req), tt.want)
		})
	}
}

// testWorkerRefRequest runs the cases for a request that carries nothing but a
// worker ref, which validate builds and validates.
func testWorkerRefRequest(t *testing.T, validate func(*ateapi.ObjectRef) field.ErrorList) {
	t.Helper()
	tests := []struct {
		name string
		ref  *ateapi.ObjectRef
		want field.ErrorList
	}{{
		name: "valid",
		ref:  workerRef(apiWorkerName),
	}, {
		name: "missing worker",
		ref:  nil,
		want: field.ErrorList{field.Required(field.NewPath("worker"), "")},
	}, {
		name: "missing worker.name",
		ref:  &ateapi.ObjectRef{},
		want: field.ErrorList{field.Required(field.NewPath("worker", "name"), "")},
	}, {
		name: "invalid worker.name",
		ref:  workerRef("UPPER"),
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "name"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		name: "worker.atespace must be empty",
		ref:  &ateapi.ObjectRef{Atespace: "team-a", Name: apiWorkerName},
		want: field.ErrorList{field.Forbidden(field.NewPath("worker", "atespace"), "")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, validate(tt.ref), tt.want)
		})
	}
}

func TestValidateDrainWorkerRequest(t *testing.T) {
	testWorkerRefRequest(t, func(ref *ateapi.ObjectRef) field.ErrorList {
		return ValidateDrainWorkerRequest(context.Background(), &ateapi.DrainWorkerRequest{Worker: ref})
	})
}

func TestValidateMarkWorkerAvailableRequest(t *testing.T) {
	testWorkerRefRequest(t, func(ref *ateapi.ObjectRef) field.ErrorList {
		return ValidateMarkWorkerAvailableRequest(context.Background(), &ateapi.MarkWorkerAvailableRequest{Worker: ref})
	})
}

func TestValidateMarkWorkerUnavailableRequest(t *testing.T) {
	testWorkerRefRequest(t, func(ref *ateapi.ObjectRef) field.ErrorList {
		return ValidateMarkWorkerUnavailableRequest(
			context.Background(),
			&ateapi.MarkWorkerUnavailableRequest{Worker: ref},
		)
	})
}

func TestValidateDeleteWorkerRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *ateapi.DeleteWorkerRequest
		want field.ErrorList
	}{{
		"valid, no options",
		&ateapi.DeleteWorkerRequest{Worker: workerRef(apiWorkerName)},
		nil,
	}, {
		"valid, both guards",
		&ateapi.DeleteWorkerRequest{
			Worker:  workerRef(apiWorkerName),
			Options: &ateapi.DeleteOptions{Uid: apiOtherWorkerName, Version: 3},
		},
		nil,
	}, {
		"missing worker",
		&ateapi.DeleteWorkerRequest{},
		field.ErrorList{field.Required(field.NewPath("worker"), "")},
	}, {
		"missing worker.name",
		&ateapi.DeleteWorkerRequest{Worker: &ateapi.ObjectRef{}},
		field.ErrorList{field.Required(field.NewPath("worker", "name"), "")},
	}, {
		"worker.atespace must be empty",
		&ateapi.DeleteWorkerRequest{Worker: &ateapi.ObjectRef{Atespace: "team-a", Name: apiWorkerName}},
		field.ErrorList{field.Forbidden(field.NewPath("worker", "atespace"), "")},
	}, {
		"invalid options.uid",
		&ateapi.DeleteWorkerRequest{
			Worker:  workerRef(apiWorkerName),
			Options: &ateapi.DeleteOptions{Uid: "not-a-uuid"},
		},
		field.ErrorList{field.Invalid(field.NewPath("options", "uid"), nil, "").WithOrigin("format=k8s-uuid")},
	}, {
		"negative options.version",
		&ateapi.DeleteWorkerRequest{
			Worker:  workerRef(apiWorkerName),
			Options: &ateapi.DeleteOptions{Version: -1},
		},
		field.ErrorList{field.Invalid(field.NewPath("options", "version"), nil, "").WithOrigin("minimum")},
	}, {
		// Zero values waive the guards, so they are never validated for shape.
		"zero options are waived, not validated",
		&ateapi.DeleteWorkerRequest{
			Worker:  workerRef(apiWorkerName),
			Options: &ateapi.DeleteOptions{},
		},
		nil,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateDeleteWorkerRequest(context.Background(), tt.req), tt.want)
		})
	}
}

func TestValidateUpdateWorkerRequest(t *testing.T) {
	// This test verifies validation of user input for update. The worker body
	// is deliberately not descended into here (updates are validated in two
	// steps); only the metadata that addresses the resource is checked.
	validReq := func(worker *ateapi.Worker) *ateapi.UpdateWorkerRequest {
		return &ateapi.UpdateWorkerRequest{Worker: worker}
	}
	withMetadata := withWorkerMetadata
	withUID := withMetadataUID
	withVersion := withMetadataVersion

	tests := []struct {
		name string
		req  *ateapi.UpdateWorkerRequest
		want field.ErrorList
	}{{
		// uid and version are preconditions the store requires; the request
		// validation deliberately leaves their presence to the store.
		"valid",
		validReq(validWorker(apiWorkerName)),
		nil,
	}, {
		"valid with preconditions",
		validReq(validWorker(apiWorkerName, withMetadata(withUID(), withVersion()))),
		nil,
	}, {
		"missing worker",
		&ateapi.UpdateWorkerRequest{},
		field.ErrorList{field.Required(field.NewPath("worker"), "")},
	}, {
		"missing metadata",
		validReq(validWorker(apiWorkerName, func(w *ateapi.Worker) { w.Metadata = nil })),
		field.ErrorList{field.Required(field.NewPath("worker", "metadata"), "")},
	}, {
		"missing metadata.name",
		validReq(validWorker(apiWorkerName, withMetadata(func(m *ateapi.ResourceMetadata) { m.Name = "" }))),
		field.ErrorList{field.Required(field.NewPath("worker", "metadata", "name"), "")},
	}, {
		"invalid metadata.name",
		validReq(validWorker(apiWorkerName, withMetadata(func(m *ateapi.ResourceMetadata) { m.Name = "Not A Name" }))),
		field.ErrorList{
			field.Invalid(field.NewPath("worker", "metadata", "name"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		"invalid metadata.uid",
		validReq(validWorker(apiWorkerName, withMetadata(func(m *ateapi.ResourceMetadata) { m.Uid = "not-a-uuid" }))),
		field.ErrorList{
			field.Invalid(field.NewPath("worker", "metadata", "uid"), nil, "").WithOrigin("format=k8s-uuid"),
		},
	}, {
		"negative metadata.version",
		validReq(validWorker(apiWorkerName, withMetadata(func(m *ateapi.ResourceMetadata) { m.Version = -1 }))),
		field.ErrorList{
			field.Invalid(field.NewPath("worker", "metadata", "version"), nil, "").WithOrigin("minimum"),
		},
	}, {
		"metadata.atespace set on a global-scoped Worker",
		validReq(validWorker(apiWorkerName, withMetadata(withMetadataAtespace("team-a")))),
		field.ErrorList{field.Forbidden(field.NewPath("worker", "metadata", "atespace"), "")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateUpdateWorkerRequest(context.Background(), tt.req), tt.want)
		})
	}
}

// TestValidateWorkerUpdate covers the final-object checks on a Worker against
// the stored one. Status reaches the store from the server, which no request
// validator sees, so this is the only check on it.
func TestValidateWorkerUpdate(t *testing.T) {
	withMetadata := withWorkerMetadata
	withUID := withMetadataUID
	withSpiffeID := withWorkerSpiffeID
	withLabels := withWorkerLabels
	withStatus := withWorkerStatus
	withSandboxClass := withWorkerStatusSandboxClass
	withRestorableRuntime := withSandboxClassRestorableRuntime
	withMultipleRestorableRuntimes := withSandboxClassMultipleRestorableRuntimes

	tests := []struct {
		name          string
		oldVal        *ateapi.Worker
		newVal        *ateapi.Worker
		requireStatus bool
		want          field.ErrorList
	}{{
		name:          "unchanged",
		oldVal:        validWorker(apiWorkerName, withStatus()),
		newVal:        validWorker(apiWorkerName, withStatus()),
		requireStatus: true,
	}, {
		// The server always sets status before storing, so only a direct call
		// shows the guard catching a worker without one.
		name:          "status cleared, required",
		oldVal:        validWorker(apiWorkerName, withStatus()),
		newVal:        validWorker(apiWorkerName),
		requireStatus: true,
		want:          field.ErrorList{field.Required(field.NewPath("worker", "status"), "")},
	}, {
		// Status is optional in the schema, and clearing it is not otherwise
		// constrained.
		name:   "status cleared, not required",
		oldVal: validWorker(apiWorkerName, withStatus()),
		newVal: validWorker(apiWorkerName),
	}, {
		name:          "labels changed",
		oldVal:        validWorker(apiWorkerName, withStatus(), withLabels(map[string]string{"tier": "batch"})),
		newVal:        validWorker(apiWorkerName, withStatus(), withLabels(map[string]string{"tier": "online"})),
		requireStatus: true,
	}, {
		name:          "labels cleared",
		oldVal:        validWorker(apiWorkerName, withStatus(), withLabels(map[string]string{"tier": "batch"})),
		newVal:        validWorker(apiWorkerName, withStatus()),
		requireStatus: true,
	}, {
		name:   "metadata.uid changed",
		oldVal: validWorker(apiWorkerName, withStatus(), withMetadata(withUID())),
		newVal: validWorker(
			apiWorkerName,
			withStatus(),
			withMetadata(func(m *ateapi.ResourceMetadata) { m.Uid = apiOtherWorkerName }),
		),
		requireStatus: true,
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "metadata", "uid"), nil, "").WithOrigin("immutable"),
		},
	}, {
		name:   "metadata.name changed",
		oldVal: validWorker(apiWorkerName, withStatus()),
		newVal: validWorker(
			apiWorkerName,
			withStatus(),
			withMetadata(func(m *ateapi.ResourceMetadata) { m.Name = apiOtherWorkerName }),
		),
		requireStatus: true,
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "metadata", "name"), nil, "").WithOrigin("immutable"),
		},
	}, {
		name:          "metadata.atespace set",
		oldVal:        validWorker(apiWorkerName, withStatus()),
		newVal:        validWorker(apiWorkerName, withStatus(), withMetadata(withMetadataAtespace("team-a"))),
		requireStatus: true,
		want: field.ErrorList{
			field.Forbidden(field.NewPath("worker", "metadata", "atespace"), ""),
			field.Invalid(field.NewPath("worker", "metadata", "atespace"), nil, "").WithOrigin("immutable"),
		},
	}, {
		name:          "worker_namespace changed",
		oldVal:        validWorker(apiWorkerName, withStatus()),
		newVal:        validWorker(apiWorkerName, withStatus(), func(w *ateapi.Worker) { w.WorkerNamespace = "other" }),
		requireStatus: true,
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "worker_namespace"), nil, "").WithOrigin("immutable"),
		},
	}, {
		name:          "worker_pool changed",
		oldVal:        validWorker(apiWorkerName, withStatus()),
		newVal:        validWorker(apiWorkerName, withStatus(), func(w *ateapi.Worker) { w.WorkerPool = "pool-2" }),
		requireStatus: true,
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "worker_pool"), nil, "").WithOrigin("immutable"),
		},
	}, {
		name:   "worker_pod changed",
		oldVal: validWorker(apiWorkerName, withStatus()),
		newVal: validWorker(
			apiWorkerName,
			withStatus(),
			func(w *ateapi.Worker) { w.WorkerPod = "worker-pod-2" },
		),
		requireStatus: true,
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "worker_pod"), nil, "").WithOrigin("immutable"),
		},
	}, {
		name:   "worker_pod_uid changed",
		oldVal: validWorker(apiWorkerName, withStatus()),
		newVal: validWorker(
			apiWorkerName,
			withStatus(),
			func(w *ateapi.Worker) { w.WorkerPodUid = apiOtherWorkerName },
		),
		requireStatus: true,
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "worker_pod_uid"), nil, "").WithOrigin("immutable"),
		},
	}, {
		name:          "node_name changed",
		oldVal:        validWorker(apiWorkerName, withStatus()),
		newVal:        validWorker(apiWorkerName, withStatus(), func(w *ateapi.Worker) { w.NodeName = "node-2" }),
		requireStatus: true,
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "node_name"), nil, "").WithOrigin("immutable"),
		},
	}, {
		name: "worker_spiffe_id changed",
		oldVal: validWorker(
			apiWorkerName,
			withStatus(),
			withSpiffeID("spiffe://cluster.local/ns/ate-system/sa/a"),
		),
		newVal: validWorker(
			apiWorkerName,
			withStatus(),
			withSpiffeID("spiffe://cluster.local/ns/ate-system/sa/b"),
		),
		requireStatus: true,
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "worker_spiffe_id"), nil, "").WithOrigin("immutable"),
		},
	}, {
		name:          "valid sandbox runtimes",
		oldVal:        validWorker(apiWorkerName, withStatus(withSandboxClass("gvisor"))),
		newVal:        validWorker(apiWorkerName, withStatus(withSandboxClass("gvisor", withMultipleRestorableRuntimes(32)))),
		requireStatus: true,
	}, {
		name:          "valid microvm sandbox class",
		oldVal:        validWorker(apiWorkerName, withStatus(withSandboxClass("gvisor"))),
		newVal:        validWorker(apiWorkerName, withStatus(withSandboxClass("microvm"))),
		requireStatus: true,
	}, {
		name:          "empty sandbox_classes",
		oldVal:        validWorker(apiWorkerName, withStatus(withSandboxClass("gvisor"))),
		newVal:        validWorker(apiWorkerName, withStatus()),
		requireStatus: true,
	}, {
		name:   "nil sandbox_classes entry",
		oldVal: validWorker(apiWorkerName, withStatus(withSandboxClass("gvisor"))),
		newVal: validWorker(apiWorkerName,
			withStatus(func(s *ateapi.WorkerStatus) {
				s.SandboxClasses = append(s.SandboxClasses, nil)
			})),
		requireStatus: true,
		want: field.ErrorList{
			field.Required(field.NewPath("worker", "status", "sandbox_classes").Index(0), ""),
		},
	}, {
		name:   "too many sandbox_classes",
		oldVal: validWorker(apiWorkerName, withStatus(withSandboxClass("gvisor"))),
		newVal: validWorker(apiWorkerName,
			withStatus(
				withSandboxClass("gvisor"),
				withSandboxClass("microvm"),
			)),
		requireStatus: true,
		want: field.ErrorList{
			field.TooMany(field.NewPath("worker", "status", "sandbox_classes"), 2, 1).WithOrigin("maxItems"),
		},
	}, {
		name:          "missing sandbox_classes.sandbox_class",
		oldVal:        validWorker(apiWorkerName, withStatus(withSandboxClass("gvisor"))),
		newVal:        validWorker(apiWorkerName, withStatus(withSandboxClass(""))),
		requireStatus: true,
		want: field.ErrorList{
			field.Required(field.NewPath("worker", "status", "sandbox_classes").Index(0).Child("sandbox_class"), ""),
		},
	}, {
		name:          "unsupported sandbox_classes.sandbox_class",
		oldVal:        validWorker(apiWorkerName, withStatus(withSandboxClass("gvisor"))),
		newVal:        validWorker(apiWorkerName, withStatus(withSandboxClass("kata"))),
		requireStatus: true,
		want: field.ErrorList{
			field.NotSupported[string](field.NewPath("worker", "status", "sandbox_classes").Index(0).Child("sandbox_class"), nil, nil),
		},
	}, {
		name:   "missing sandbox_classes.default_runtime",
		oldVal: validWorker(apiWorkerName, withStatus(withSandboxClass("gvisor"))),
		newVal: validWorker(apiWorkerName,
			withStatus(
				withSandboxClass("gvisor", func(c *ateapi.WorkerSandboxClass) {
					c.DefaultRuntime = nil
				}),
			)),
		requireStatus: true,
		want: field.ErrorList{
			field.Required(field.NewPath("worker", "status", "sandbox_classes").Index(0).Child("default_runtime"), ""),
		},
	}, {
		name:   "missing sandbox_classes.default_runtime.version",
		oldVal: validWorker(apiWorkerName, withStatus(withSandboxClass("gvisor"))),
		newVal: validWorker(apiWorkerName,
			withStatus(
				withSandboxClass("gvisor", func(c *ateapi.WorkerSandboxClass) {
					c.DefaultRuntime.Version = nil
				}),
			)),
		requireStatus: true,
		want: field.ErrorList{
			field.Required(field.NewPath("worker", "status", "sandbox_classes").Index(0).Child("default_runtime", "version"), ""),
		},
	}, {
		name:   "too many default_runtime attributes",
		oldVal: validWorker(apiWorkerName, withStatus(withSandboxClass("gvisor"))),
		newVal: validWorker(apiWorkerName,
			withStatus(
				withSandboxClass("gvisor", func(c *ateapi.WorkerSandboxClass) {
					c.DefaultRuntime.Version.Attributes = attributeEntries(33)
				}),
			)),
		requireStatus: true,
		want: field.ErrorList{
			field.TooMany(field.NewPath("worker", "status", "sandbox_classes").Index(0).Child("default_runtime", "version", "attributes"), 33, 32).WithOrigin("maxItems"),
		},
	}, {
		name:   "invalid restorable_runtimes entry",
		oldVal: validWorker(apiWorkerName, withStatus(withSandboxClass("gvisor"))),
		newVal: validWorker(apiWorkerName,
			withStatus(
				withSandboxClass("gvisor",
					withRestorableRuntime(func(r *ateapi.WorkerSandboxRuntime) {
						r.Version.SchemaVersion = ""
					})),
			)),
		requireStatus: true,
		want: field.ErrorList{
			field.Required(field.NewPath("worker", "status", "sandbox_classes").Index(0).Child("restorable_runtimes").Index(0).Child("version", "schema_version"), ""),
		},
	}, {
		name:          "too many restorable_runtimes",
		oldVal:        validWorker(apiWorkerName, withStatus(withSandboxClass("gvisor"))),
		newVal:        validWorker(apiWorkerName, withStatus(withSandboxClass("gvisor", withMultipleRestorableRuntimes(33)))),
		requireStatus: true,
		want: field.ErrorList{
			field.TooMany(field.NewPath("worker", "status", "sandbox_classes").Index(0).Child("restorable_runtimes"), 33, 32).WithOrigin("maxItems"),
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(
				t,
				ValidateWorkerUpdate(
					context.Background(),
					field.NewPath("worker"),
					tt.newVal,
					tt.oldVal,
					tt.requireStatus,
				),
				tt.want,
			)
		})
	}
}

func TestValidateAcquireWorkerEpochRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *ateapi.AcquireWorkerEpochRequest
		want field.ErrorList
	}{{
		name: "valid",
		req:  &ateapi.AcquireWorkerEpochRequest{Worker: workerRef(apiWorkerName)},
	}, {
		name: "missing worker",
		req:  &ateapi.AcquireWorkerEpochRequest{},
		want: field.ErrorList{field.Required(field.NewPath("worker"), "")},
	}, {
		name: "worker.atespace must be empty",
		req:  &ateapi.AcquireWorkerEpochRequest{Worker: &ateapi.ObjectRef{Atespace: "team-a", Name: apiWorkerName}},
		want: field.ErrorList{field.Forbidden(field.NewPath("worker", "atespace"), "")},
	}, {
		name: "missing worker.name",
		req:  &ateapi.AcquireWorkerEpochRequest{Worker: &ateapi.ObjectRef{}},
		want: field.ErrorList{field.Required(field.NewPath("worker", "name"), "")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateAcquireWorkerEpochRequest(context.Background(), tt.req), tt.want)
		})
	}
}

func TestValidateRegisterWorkerRequest(t *testing.T) {
	// valid returns the smallest valid request. capacity is required, but all
	// of its fields are optional.
	valid := func(mutate ...func(*ateapi.RegisterWorkerRequest)) *ateapi.RegisterWorkerRequest {
		r := &ateapi.RegisterWorkerRequest{
			Worker:   workerRef(apiWorkerName),
			Epoch:    1,
			Ips:      []string{"10.1.2.3"},
			Capacity: &ateapi.WorkerResources{},
		}
		for _, m := range mutate {
			m(r)
		}
		return r
	}
	withIPs := func(ips ...string) func(*ateapi.RegisterWorkerRequest) {
		return func(r *ateapi.RegisterWorkerRequest) { r.Ips = ips }
	}
	withCapacity := func(c *ateapi.WorkerResources) func(*ateapi.RegisterWorkerRequest) {
		return func(r *ateapi.RegisterWorkerRequest) { r.Capacity = c }
	}
	tests := []struct {
		name string
		req  *ateapi.RegisterWorkerRequest
		want field.ErrorList
	}{{
		name: "valid",
		req:  valid(),
	}, {
		name: "valid capacity",
		req:  valid(withCapacity(&ateapi.WorkerResources{Actors: 10, CpuMillicores: 2000, MemoryBytes: 4 << 30})),
	}, {
		name: "valid dual-stack ips",
		req:  valid(withIPs("fd00::1", "10.1.2.3")),
	}, {
		name: "missing worker",
		req:  valid(func(r *ateapi.RegisterWorkerRequest) { r.Worker = nil }),
		want: field.ErrorList{field.Required(field.NewPath("worker"), "")},
	}, {
		name: "worker.atespace must be empty",
		req:  valid(func(r *ateapi.RegisterWorkerRequest) { r.Worker.Atespace = "team-a" }),
		want: field.ErrorList{field.Forbidden(field.NewPath("worker", "atespace"), "")},
	}, {
		name: "missing worker.name",
		req:  valid(func(r *ateapi.RegisterWorkerRequest) { r.Worker.Name = "" }),
		want: field.ErrorList{field.Required(field.NewPath("worker", "name"), "")},
	}, {
		name: "invalid worker.name",
		req:  valid(func(r *ateapi.RegisterWorkerRequest) { r.Worker.Name = "UPPER" }),
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "name"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		name: "missing epoch",
		req:  valid(func(r *ateapi.RegisterWorkerRequest) { r.Epoch = 0 }),
		want: field.ErrorList{field.Required(field.NewPath("epoch"), "")},
	}, {
		name: "negative epoch",
		req:  valid(func(r *ateapi.RegisterWorkerRequest) { r.Epoch = -1 }),
		want: field.ErrorList{field.Invalid(field.NewPath("epoch"), nil, "").WithOrigin("minimum")},
	}, {
		name: "missing ips",
		req:  valid(withIPs()),
		want: field.ErrorList{field.Required(field.NewPath("ips"), "")},
	}, {
		name: "invalid ip",
		req:  valid(withIPs("not-an-ip")),
		want: field.ErrorList{field.Invalid(field.NewPath("ips").Index(0), nil, "").WithOrigin("format=ip-strict")},
	}, {
		name: "two ips in one family",
		req:  valid(withIPs("10.1.2.3", "10.1.2.4")),
		want: field.ErrorList{field.Invalid(field.NewPath("ips").Index(1), nil, "")},
	}, {
		// An invalid entry does not claim its family.
		name: "invalid ip before a valid one",
		req:  valid(withIPs("not-an-ip", "10.1.2.3")),
		want: field.ErrorList{field.Invalid(field.NewPath("ips").Index(0), nil, "").WithOrigin("format=ip-strict")},
	}, {
		name: "two IPv6 ips",
		req:  valid(withIPs("fd00::1", "fd00::2")),
		want: field.ErrorList{field.Invalid(field.NewPath("ips").Index(1), nil, "")},
	}, {
		name: "too many ips",
		req:  valid(withIPs("10.1.2.3", "fd00::1", "10.1.2.4")),
		want: field.ErrorList{field.TooMany(field.NewPath("ips"), 3, 2).WithOrigin("maxItems")},
	}, {
		name: "missing capacity",
		req:  valid(withCapacity(nil)),
		want: field.ErrorList{field.Required(field.NewPath("capacity"), "")},
	}, {
		name: "negative capacity.actors",
		req:  valid(withCapacity(&ateapi.WorkerResources{Actors: -1})),
		want: field.ErrorList{field.Invalid(field.NewPath("capacity", "actors"), nil, "").WithOrigin("minimum")},
	}, {
		name: "negative capacity.cpu_millicores",
		req:  valid(withCapacity(&ateapi.WorkerResources{CpuMillicores: -1})),
		want: field.ErrorList{
			field.Invalid(field.NewPath("capacity", "cpu_millicores"), nil, "").WithOrigin("minimum"),
		},
	}, {
		name: "capacity.cpu_millicores above the maximum",
		req:  valid(withCapacity(&ateapi.WorkerResources{CpuMillicores: 1000000})),
		want: field.ErrorList{
			field.Invalid(field.NewPath("capacity", "cpu_millicores"), nil, "").WithOrigin("maximum"),
		},
	}, {
		name: "negative capacity.memory_bytes",
		req:  valid(withCapacity(&ateapi.WorkerResources{MemoryBytes: -1})),
		want: field.ErrorList{field.Invalid(field.NewPath("capacity", "memory_bytes"), nil, "").WithOrigin("minimum")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateRegisterWorkerRequest(context.Background(), tt.req), tt.want)
		})
	}
}

func TestValidateSetWorkerSandboxRuntimesRequest(t *testing.T) {
	valid := func(mutate ...func(*ateapi.SetWorkerSandboxRuntimesRequest)) *ateapi.SetWorkerSandboxRuntimesRequest {
		r := &ateapi.SetWorkerSandboxRuntimesRequest{
			Worker:         workerRef(apiWorkerName),
			Epoch:          1,
			SandboxClasses: []*ateapi.WorkerSandboxClass{validSandboxClass("gvisor")},
		}
		for _, m := range mutate {
			m(r)
		}
		return r
	}
	withSandboxClasses := func(classes ...*ateapi.WorkerSandboxClass) func(*ateapi.SetWorkerSandboxRuntimesRequest) {
		return func(r *ateapi.SetWorkerSandboxRuntimesRequest) { r.SandboxClasses = classes }
	}
	withRestorableRuntime := withSandboxClassRestorableRuntime
	withMultipleRestorableRuntimes := withSandboxClassMultipleRestorableRuntimes
	withActorOverhead := withRuntimeActorOverhead
	// withAttributes returns a class whose default runtime has attrs.
	withAttributes := func(attrs ...*ateapi.AttributeEntry) func(*ateapi.WorkerSandboxClass) {
		return func(c *ateapi.WorkerSandboxClass) { c.DefaultRuntime.Version.Attributes = attrs }
	}
	tests := []struct {
		name string
		req  *ateapi.SetWorkerSandboxRuntimesRequest
		want field.ErrorList
	}{{
		name: "valid",
		req:  valid(),
	}, {
		name: "valid microvm sandbox class",
		req:  valid(withSandboxClasses(validSandboxClass("microvm"))),
	}, {
		name: "valid with restorable runtimes",
		req:  valid(withSandboxClasses(validSandboxClass("gvisor", withRestorableRuntime()))),
	}, {
		name: "valid with actor overhead",
		req: valid(
			withSandboxClasses(
				validSandboxClass("gvisor", func(c *ateapi.WorkerSandboxClass) {
					c.DefaultRuntime = sandboxRuntime(withActorOverhead(64<<20, 32<<20))
				}),
			),
		),
	}, {
		name: "valid with a fixed actor overhead only",
		req: valid(
			withSandboxClasses(
				validSandboxClass("gvisor", func(c *ateapi.WorkerSandboxClass) {
					c.DefaultRuntime = sandboxRuntime(withActorOverhead(64<<20, 0))
				}),
			),
		),
	}, {
		name: "actor overhead at the bounds",
		req: valid(
			withSandboxClasses(
				validSandboxClass("gvisor", func(c *ateapi.WorkerSandboxClass) {
					c.DefaultRuntime = sandboxRuntime(withActorOverhead(1<<40, 1<<30))
				}),
			),
		),
	}, {
		name: "negative actor overhead",
		req: valid(
			withSandboxClasses(
				validSandboxClass("gvisor", func(c *ateapi.WorkerSandboxClass) {
					c.DefaultRuntime = sandboxRuntime(withActorOverhead(-1, -1))
				}),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("sandbox_classes").Index(0).Child("default_runtime", "actor_overhead", "memory_bytes"), nil, "").WithOrigin("minimum"),
			field.Invalid(field.NewPath("sandbox_classes").Index(0).Child("default_runtime", "actor_overhead", "memory_bytes_per_gib"), nil, "").WithOrigin("minimum"),
		},
	}, {
		name: "actor overhead above the maximum",
		req: valid(
			withSandboxClasses(
				validSandboxClass("gvisor", func(c *ateapi.WorkerSandboxClass) {
					c.DefaultRuntime = sandboxRuntime(withActorOverhead(1<<40+1, 1<<30+1))
				}),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("sandbox_classes").Index(0).Child("default_runtime", "actor_overhead", "memory_bytes"), nil, "").WithOrigin("maximum"),
			field.Invalid(field.NewPath("sandbox_classes").Index(0).Child("default_runtime", "actor_overhead", "memory_bytes_per_gib"), nil, "").WithOrigin("maximum"),
		},
	}, {
		name: "missing worker",
		req:  valid(func(r *ateapi.SetWorkerSandboxRuntimesRequest) { r.Worker = nil }),
		want: field.ErrorList{field.Required(field.NewPath("worker"), "")},
	}, {
		name: "worker.atespace must be empty",
		req:  valid(func(r *ateapi.SetWorkerSandboxRuntimesRequest) { r.Worker.Atespace = "team-a" }),
		want: field.ErrorList{field.Forbidden(field.NewPath("worker", "atespace"), "")},
	}, {
		name: "missing epoch",
		req:  valid(func(r *ateapi.SetWorkerSandboxRuntimesRequest) { r.Epoch = 0 }),
		want: field.ErrorList{field.Required(field.NewPath("epoch"), "")},
	}, {
		name: "negative epoch",
		req:  valid(func(r *ateapi.SetWorkerSandboxRuntimesRequest) { r.Epoch = -1 }),
		want: field.ErrorList{field.Invalid(field.NewPath("epoch"), nil, "").WithOrigin("minimum")},
	}, {
		name: "missing sandbox_classes",
		req:  valid(withSandboxClasses()),
		want: field.ErrorList{field.Required(field.NewPath("sandbox_classes"), "")},
	}, {
		name: "empty sandbox_classes",
		req: valid(func(r *ateapi.SetWorkerSandboxRuntimesRequest) {
			r.SandboxClasses = []*ateapi.WorkerSandboxClass{}
		}),
		want: field.ErrorList{field.Required(field.NewPath("sandbox_classes"), "")},
	}, {
		name: "nil sandbox_classes entry",
		req:  valid(withSandboxClasses(nil)),
		want: field.ErrorList{field.Required(field.NewPath("sandbox_classes").Index(0), "")},
	}, {
		name: "too many sandbox_classes",
		req:  valid(withSandboxClasses(validSandboxClass("gvisor"), validSandboxClass("microvm"))),
		want: field.ErrorList{field.TooMany(field.NewPath("sandbox_classes"), 2, 1).WithOrigin("maxItems")},
	}, {
		name: "missing sandbox_classes.sandbox_class",
		req:  valid(withSandboxClasses(validSandboxClass(""))),
		want: field.ErrorList{field.Required(field.NewPath("sandbox_classes").Index(0).Child("sandbox_class"), "")},
	}, {
		name: "sandbox_classes.sandbox_class too long",
		req:  valid(withSandboxClasses(validSandboxClass(strings.Repeat("c", 64)))),
		want: field.ErrorList{
			field.TooLong(field.NewPath("sandbox_classes").Index(0).Child("sandbox_class"), "", 63).WithOrigin("maxLength"),
			field.NotSupported[string](field.NewPath("sandbox_classes").Index(0).Child("sandbox_class"), nil, nil),
		},
	}, {
		name: "unsupported sandbox_classes.sandbox_class",
		req:  valid(withSandboxClasses(validSandboxClass("kata"))),
		want: field.ErrorList{
			field.NotSupported[string](field.NewPath("sandbox_classes").Index(0).Child("sandbox_class"), nil, nil),
		},
	}, {
		name: "missing sandbox_classes.default_runtime",
		req: valid(
			withSandboxClasses(
				validSandboxClass("gvisor", func(c *ateapi.WorkerSandboxClass) {
					c.DefaultRuntime = nil
				}),
			),
		),
		want: field.ErrorList{field.Required(field.NewPath("sandbox_classes").Index(0).Child("default_runtime"), "")},
	}, {
		name: "missing default_runtime.version",
		req: valid(
			withSandboxClasses(
				validSandboxClass("gvisor", func(c *ateapi.WorkerSandboxClass) {
					c.DefaultRuntime.Version = nil
				}),
			),
		),
		want: field.ErrorList{
			field.Required(field.NewPath("sandbox_classes").Index(0).Child("default_runtime", "version"), ""),
		},
	}, {
		name: "missing default_runtime.schema_version",
		req: valid(
			withSandboxClasses(
				validSandboxClass("gvisor", func(c *ateapi.WorkerSandboxClass) {
					c.DefaultRuntime.Version.SchemaVersion = ""
				}),
			),
		),
		want: field.ErrorList{
			field.Required(field.NewPath("sandbox_classes").Index(0).Child("default_runtime", "version", "schema_version"), ""),
		},
	}, {
		name: "default_runtime.schema_version too long",
		req: valid(
			withSandboxClasses(
				validSandboxClass("gvisor", func(c *ateapi.WorkerSandboxClass) {
					c.DefaultRuntime.Version.SchemaVersion = strings.Repeat("v", 65)
				}),
			),
		),
		want: field.ErrorList{
			field.TooLong(field.NewPath("sandbox_classes").Index(0).Child("default_runtime", "version", "schema_version"), "", 64).WithOrigin("maxLength"),
		},
	}, {
		name: "missing default_runtime.attributes",
		req:  valid(withSandboxClasses(validSandboxClass("gvisor", withAttributes()))),
		want: field.ErrorList{
			field.Required(field.NewPath("sandbox_classes").Index(0).Child("default_runtime", "version", "attributes"), ""),
		},
	}, {
		name: "duplicate attribute key",
		req: valid(
			withSandboxClasses(
				validSandboxClass("gvisor",
					withAttributes(
						&ateapi.AttributeEntry{Key: "architecture", Value: "amd64"},
						&ateapi.AttributeEntry{Key: "architecture", Value: "arm64"},
					)),
			),
		),
		want: field.ErrorList{
			field.Duplicate(field.NewPath("sandbox_classes").Index(0).Child("default_runtime", "version", "attributes").Index(1), nil),
		},
	}, {
		name: "missing attribute key",
		req: valid(
			withSandboxClasses(
				validSandboxClass("gvisor",
					withAttributes(&ateapi.AttributeEntry{Value: "amd64"})),
			),
		),
		want: field.ErrorList{
			field.Required(field.NewPath("sandbox_classes").Index(0).Child("default_runtime", "version", "attributes").Index(0).Child("key"), ""),
		},
	}, {
		name: "attribute key too long",
		req: valid(
			withSandboxClasses(
				validSandboxClass("gvisor",
					withAttributes(&ateapi.AttributeEntry{Key: strings.Repeat("k", 129), Value: "v"})),
			),
		),
		want: field.ErrorList{
			field.TooLong(field.NewPath("sandbox_classes").Index(0).Child("default_runtime", "version", "attributes").Index(0).Child("key"), "", 128).WithOrigin("maxLength"),
		},
	}, {
		name: "missing attribute value",
		req:  valid(withSandboxClasses(validSandboxClass("gvisor", withAttributes(&ateapi.AttributeEntry{Key: "k"})))),
		want: field.ErrorList{
			field.Required(field.NewPath("sandbox_classes").Index(0).Child("default_runtime", "version", "attributes").Index(0).Child("value"), ""),
		},
	}, {
		name: "attribute value too long",
		req: valid(
			withSandboxClasses(
				validSandboxClass("gvisor",
					withAttributes(&ateapi.AttributeEntry{Key: "k", Value: strings.Repeat("v", 257)})),
			),
		),
		want: field.ErrorList{
			field.TooLong(field.NewPath("sandbox_classes").Index(0).Child("default_runtime", "version", "attributes").Index(0).Child("value"), "", 256).WithOrigin("maxLength"),
		},
	}, {
		name: "attributes at the bound",
		req:  valid(withSandboxClasses(validSandboxClass("gvisor", withAttributes(attributeEntries(32)...)))),
	}, {
		name: "too many attributes",
		req:  valid(withSandboxClasses(validSandboxClass("gvisor", withAttributes(attributeEntries(33)...)))),
		want: field.ErrorList{
			field.TooMany(field.NewPath("sandbox_classes").Index(0).Child("default_runtime", "version", "attributes"), 33, 32).WithOrigin("maxItems"),
		},
	}, {
		name: "nil restorable_runtimes entry",
		req: valid(
			withSandboxClasses(
				validSandboxClass("gvisor", func(c *ateapi.WorkerSandboxClass) {
					c.RestorableRuntimes = append(c.RestorableRuntimes, nil)
				}),
			),
		),
		want: field.ErrorList{
			field.Required(field.NewPath("sandbox_classes").Index(0).Child("restorable_runtimes").Index(0), ""),
		},
	}, {
		name: "invalid restorable_runtimes entry",
		req: valid(
			withSandboxClasses(
				validSandboxClass("gvisor",
					withRestorableRuntime(func(r *ateapi.WorkerSandboxRuntime) {
						r.Version.SchemaVersion = ""
					})),
			),
		),
		want: field.ErrorList{
			field.Required(field.NewPath("sandbox_classes").Index(0).Child("restorable_runtimes").Index(0).Child("version", "schema_version"), ""),
		},
	}, {
		name: "restorable_runtimes entry without a version",
		req: valid(
			withSandboxClasses(
				validSandboxClass("gvisor",
					withRestorableRuntime(func(r *ateapi.WorkerSandboxRuntime) {
						r.Version = nil
					})),
			),
		),
		want: field.ErrorList{
			field.Required(field.NewPath("sandbox_classes").Index(0).Child("restorable_runtimes").Index(0).Child("version"), ""),
		},
	}, {
		name: "restorable_runtimes at the bound",
		req:  valid(withSandboxClasses(validSandboxClass("gvisor", withMultipleRestorableRuntimes(32)))),
	}, {
		name: "too many restorable_runtimes",
		req:  valid(withSandboxClasses(validSandboxClass("gvisor", withMultipleRestorableRuntimes(33)))),
		want: field.ErrorList{
			field.TooMany(field.NewPath("sandbox_classes").Index(0).Child("restorable_runtimes"), 33, 32).WithOrigin("maxItems"),
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateSetWorkerSandboxRuntimesRequest(context.Background(), tt.req), tt.want)
		})
	}
}

func TestValidateRequestActorSuspendRequest(t *testing.T) {
	valid := func(mutate ...func(*ateapi.RequestActorSuspendRequest)) *ateapi.RequestActorSuspendRequest {
		r := &ateapi.RequestActorSuspendRequest{
			Worker:   workerRef(apiWorkerName),
			Actor:    &ateapi.ObjectRef{Atespace: "team-a", Name: "actor-1"},
			ActorUid: apiOtherWorkerName,
		}
		for _, m := range mutate {
			m(r)
		}
		return r
	}
	tests := []struct {
		name string
		req  *ateapi.RequestActorSuspendRequest
		want field.ErrorList
	}{{
		name: "valid",
		req:  valid(),
	}, {
		name: "missing worker",
		req:  valid(func(r *ateapi.RequestActorSuspendRequest) { r.Worker = nil }),
		want: field.ErrorList{field.Required(field.NewPath("worker"), "")},
	}, {
		name: "worker.atespace must be empty",
		req:  valid(func(r *ateapi.RequestActorSuspendRequest) { r.Worker.Atespace = "team-a" }),
		want: field.ErrorList{field.Forbidden(field.NewPath("worker", "atespace"), "")},
	}, {
		name: "missing worker.name",
		req:  valid(func(r *ateapi.RequestActorSuspendRequest) { r.Worker.Name = "" }),
		want: field.ErrorList{field.Required(field.NewPath("worker", "name"), "")},
	}, {
		name: "invalid worker.name",
		req:  valid(func(r *ateapi.RequestActorSuspendRequest) { r.Worker.Name = "UPPER" }),
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker", "name"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		name: "missing actor",
		req:  valid(func(r *ateapi.RequestActorSuspendRequest) { r.Actor = nil }),
		want: field.ErrorList{field.Required(field.NewPath("actor"), "")},
	}, {
		name: "missing actor.atespace",
		req:  valid(func(r *ateapi.RequestActorSuspendRequest) { r.Actor.Atespace = "" }),
		want: field.ErrorList{field.Required(field.NewPath("actor", "atespace"), "")},
	}, {
		name: "invalid actor.atespace",
		req:  valid(func(r *ateapi.RequestActorSuspendRequest) { r.Actor.Atespace = "../escape" }),
		want: field.ErrorList{
			field.Invalid(field.NewPath("actor", "atespace"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		name: "missing actor.name",
		req:  valid(func(r *ateapi.RequestActorSuspendRequest) { r.Actor.Name = "" }),
		want: field.ErrorList{field.Required(field.NewPath("actor", "name"), "")},
	}, {
		name: "invalid actor.name",
		req:  valid(func(r *ateapi.RequestActorSuspendRequest) { r.Actor.Name = "UPPER" }),
		want: field.ErrorList{
			field.Invalid(field.NewPath("actor", "name"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		name: "missing actor_uid",
		req:  valid(func(r *ateapi.RequestActorSuspendRequest) { r.ActorUid = "" }),
		want: field.ErrorList{field.Required(field.NewPath("actor_uid"), "")},
	}, {
		name: "invalid actor_uid",
		req:  valid(func(r *ateapi.RequestActorSuspendRequest) { r.ActorUid = "not-a-uuid" }),
		want: field.ErrorList{field.Invalid(field.NewPath("actor_uid"), nil, "").WithOrigin("format=k8s-uuid")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateRequestActorSuspendRequest(context.Background(), tt.req), tt.want)
		})
	}
}

func TestValidateReportActorSnapshotStateRequest(t *testing.T) {
	// report returns a SnapshotReport with only its required fields set.
	report := func(gen int32, mods ...func(*ateapi.SnapshotReport)) *ateapi.SnapshotReport {
		sr := &ateapi.SnapshotReport{
			Uuid:       fmt.Sprintf("snap-%d", gen),
			Generation: gen,
			Fidelity:   ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY,
			Object:     &ateapi.ObjectSnapshot{SnapshotUri: fmt.Sprintf("gs://bucket/actors/a/snap-%d", gen)},
		}
		for _, m := range mods {
			m(sr)
		}
		return sr
	}
	committed := func(sr *ateapi.SnapshotReport) { sr.LocalCommitTime = timestamppb.Now() }
	durable := func(sr *ateapi.SnapshotReport) { sr.DurableCompleteTime = timestamppb.Now() }
	// valid returns the smallest valid request. A state with neither
	// latest_local nor uploading must set latest_durable, so that is the one
	// report it carries. The with* modifiers below fill in the rest.
	valid := func(mutate ...func(*ateapi.ReportActorSnapshotStateRequest)) *ateapi.ReportActorSnapshotStateRequest {
		r := &ateapi.ReportActorSnapshotStateRequest{
			Actor:    &ateapi.ObjectRef{Atespace: "team-a", Name: "actor-1"},
			ActorUid: apiOtherWorkerName,
			State:    &ateapi.ActorSnapshotState{LatestDurable: report(1)},
		}
		for _, m := range mutate {
			m(r)
		}
		return r
	}
	withLatestLocal := func(mods ...func(*ateapi.SnapshotReport)) func(*ateapi.ReportActorSnapshotStateRequest) {
		return func(r *ateapi.ReportActorSnapshotStateRequest) { r.State.LatestLocal = report(2, mods...) }
	}
	withUploading := func(mods ...func(*ateapi.SnapshotReport)) func(*ateapi.ReportActorSnapshotStateRequest) {
		return func(r *ateapi.ReportActorSnapshotStateRequest) { r.State.Uploading = report(2, mods...) }
	}
	withLatestDurable := func(mods ...func(*ateapi.SnapshotReport)) func(*ateapi.ReportActorSnapshotStateRequest) {
		return func(r *ateapi.ReportActorSnapshotStateRequest) { r.State.LatestDurable = report(1, mods...) }
	}
	withUploadDueTime := func(r *ateapi.ReportActorSnapshotStateRequest) { r.State.UploadDueTime = timestamppb.Now() }
	tests := []struct {
		name string
		req  *ateapi.ReportActorSnapshotStateRequest
		want field.ErrorList
	}{{
		name: "valid",
		req:  valid(),
	}, {
		name: "valid with every field",
		req: valid(
			withLatestLocal(committed),
			withUploading(committed),
			withLatestDurable(committed, durable),
			withUploadDueTime,
		),
	}, {
		name: "valid detach: no local snapshot",
		req:  valid(withLatestDurable(committed, durable)),
	}, {
		name: "empty state",
		req:  valid(func(r *ateapi.ReportActorSnapshotStateRequest) { r.State = &ateapi.ActorSnapshotState{} }),
		want: field.ErrorList{field.Required(field.NewPath("state", "latest_durable"), "")},
	}, {
		name: "valid upload in flight with no local snapshot",
		req: valid(func(r *ateapi.ReportActorSnapshotStateRequest) {
			r.State = &ateapi.ActorSnapshotState{}
		}, withUploading(committed)),
	}, {
		// Only a report with no local snapshot and no upload must name a
		// durable one.
		name: "valid with only a local snapshot",
		req: valid(func(r *ateapi.ReportActorSnapshotStateRequest) {
			r.State = &ateapi.ActorSnapshotState{}
		}, withLatestLocal(committed)),
	}, {
		name: "missing actor",
		req:  valid(func(r *ateapi.ReportActorSnapshotStateRequest) { r.Actor = nil }),
		want: field.ErrorList{field.Required(field.NewPath("actor"), "")},
	}, {
		name: "missing actor.atespace",
		req:  valid(func(r *ateapi.ReportActorSnapshotStateRequest) { r.Actor.Atespace = "" }),
		want: field.ErrorList{field.Required(field.NewPath("actor", "atespace"), "")},
	}, {
		name: "invalid actor.name",
		req:  valid(func(r *ateapi.ReportActorSnapshotStateRequest) { r.Actor.Name = "UPPER" }),
		want: field.ErrorList{
			field.Invalid(field.NewPath("actor", "name"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		name: "missing actor_uid",
		req:  valid(func(r *ateapi.ReportActorSnapshotStateRequest) { r.ActorUid = "" }),
		want: field.ErrorList{field.Required(field.NewPath("actor_uid"), "")},
	}, {
		name: "invalid actor_uid",
		req:  valid(func(r *ateapi.ReportActorSnapshotStateRequest) { r.ActorUid = "not-a-uuid" }),
		want: field.ErrorList{field.Invalid(field.NewPath("actor_uid"), nil, "").WithOrigin("format=k8s-uuid")},
	}, {
		name: "missing state",
		req:  valid(func(r *ateapi.ReportActorSnapshotStateRequest) { r.State = nil }),
		want: field.ErrorList{field.Required(field.NewPath("state"), "")},
	}, {
		name: "missing uuid",
		req: valid(
			withLatestLocal(committed),
			func(r *ateapi.ReportActorSnapshotStateRequest) { r.State.LatestLocal.Uuid = "" },
		),
		want: field.ErrorList{field.Required(field.NewPath("state", "latest_local", "uuid"), "")},
	}, {
		name: "invalid uuid",
		req: valid(
			withLatestLocal(committed),
			func(r *ateapi.ReportActorSnapshotStateRequest) { r.State.LatestLocal.Uuid = "../escape" },
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("state", "latest_local", "uuid"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		name: "generation below 1",
		req: valid(
			withLatestLocal(committed),
			func(r *ateapi.ReportActorSnapshotStateRequest) { r.State.LatestLocal.Generation = -1 },
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("state", "latest_local", "generation"), nil, "").WithOrigin("minimum"),
		},
	}, {
		name: "missing fidelity",
		req: valid(
			withLatestLocal(committed),
			func(r *ateapi.ReportActorSnapshotStateRequest) {
				r.State.LatestLocal.Fidelity = ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_UNSPECIFIED
			},
		),
		want: field.ErrorList{field.Required(field.NewPath("state", "latest_local", "fidelity"), "")},
	}, {
		name: "missing generation",
		req: valid(
			withLatestLocal(committed),
			func(r *ateapi.ReportActorSnapshotStateRequest) { r.State.LatestLocal.Generation = 0 },
		),
		want: field.ErrorList{field.Required(field.NewPath("state", "latest_local", "generation"), "")},
	}, {
		name: "negative fidelity",
		req: valid(
			withLatestLocal(committed),
			func(r *ateapi.ReportActorSnapshotStateRequest) { r.State.LatestLocal.Fidelity = -1 },
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("state", "latest_local", "fidelity"), nil, "").WithOrigin("minimum"),
		},
	}, {
		name: "unknown fidelity",
		req: valid(
			withLatestLocal(committed),
			func(r *ateapi.ReportActorSnapshotStateRequest) { r.State.LatestLocal.Fidelity = 4 },
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("state", "latest_local", "fidelity"), nil, "").WithOrigin("maximum"),
		},
	}, {
		name: "missing object",
		req: valid(
			withLatestLocal(committed),
			func(r *ateapi.ReportActorSnapshotStateRequest) { r.State.LatestLocal.Object = nil },
		),
		want: field.ErrorList{field.Required(field.NewPath("state", "latest_local", "object"), "")},
	}, {
		name: "missing object.snapshot_uri",
		req: valid(
			withLatestLocal(committed),
			func(r *ateapi.ReportActorSnapshotStateRequest) { r.State.LatestLocal.Object.SnapshotUri = "" },
		),
		want: field.ErrorList{field.Required(field.NewPath("state", "latest_local", "object", "snapshot_uri"), "")},
	}, {
		name: "valid without local_commit_time",
		req:  valid(withLatestDurable(durable)),
	}, {
		name: "uploading is validated",
		req: valid(
			withUploading(committed),
			func(r *ateapi.ReportActorSnapshotStateRequest) { r.State.Uploading.Uuid = "" },
		),
		want: field.ErrorList{field.Required(field.NewPath("state", "uploading", "uuid"), "")},
	}, {
		name: "latest_durable is validated",
		req: valid(
			withLatestDurable(committed, durable),
			func(r *ateapi.ReportActorSnapshotStateRequest) { r.State.LatestDurable.Object = nil },
		),
		want: field.ErrorList{field.Required(field.NewPath("state", "latest_durable", "object"), "")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateReportActorSnapshotStateRequest(context.Background(), tt.req), tt.want)
		})
	}
}

func TestValidateReportActorCrashedRequest(t *testing.T) {
	valid := func(mutate ...func(*ateapi.ReportActorCrashedRequest)) *ateapi.ReportActorCrashedRequest {
		r := &ateapi.ReportActorCrashedRequest{
			Worker:   workerRef(apiWorkerName),
			Actor:    &ateapi.ObjectRef{Atespace: "team-a", Name: "actor-1"},
			ActorUid: apiOtherWorkerName,
			Message:  "container main exited with status 137",
		}
		for _, m := range mutate {
			m(r)
		}
		return r
	}
	tests := []struct {
		name string
		req  *ateapi.ReportActorCrashedRequest
		want field.ErrorList
	}{{
		name: "valid",
		req:  valid(),
	}, {
		name: "missing worker",
		req:  valid(func(r *ateapi.ReportActorCrashedRequest) { r.Worker = nil }),
		want: field.ErrorList{field.Required(field.NewPath("worker"), "")},
	}, {
		name: "worker.atespace must be empty",
		req:  valid(func(r *ateapi.ReportActorCrashedRequest) { r.Worker.Atespace = "team-a" }),
		want: field.ErrorList{field.Forbidden(field.NewPath("worker", "atespace"), "")},
	}, {
		name: "missing actor",
		req:  valid(func(r *ateapi.ReportActorCrashedRequest) { r.Actor = nil }),
		want: field.ErrorList{field.Required(field.NewPath("actor"), "")},
	}, {
		name: "missing actor.atespace",
		req:  valid(func(r *ateapi.ReportActorCrashedRequest) { r.Actor.Atespace = "" }),
		want: field.ErrorList{field.Required(field.NewPath("actor", "atespace"), "")},
	}, {
		name: "missing actor_uid",
		req:  valid(func(r *ateapi.ReportActorCrashedRequest) { r.ActorUid = "" }),
		want: field.ErrorList{field.Required(field.NewPath("actor_uid"), "")},
	}, {
		name: "invalid actor_uid",
		req:  valid(func(r *ateapi.ReportActorCrashedRequest) { r.ActorUid = "not-a-uuid" }),
		want: field.ErrorList{field.Invalid(field.NewPath("actor_uid"), nil, "").WithOrigin("format=k8s-uuid")},
	}, {
		name: "missing message",
		req:  valid(func(r *ateapi.ReportActorCrashedRequest) { r.Message = "" }),
		want: field.ErrorList{field.Required(field.NewPath("message"), "")},
	}, {
		name: "message at the limit",
		req:  valid(func(r *ateapi.ReportActorCrashedRequest) { r.Message = strings.Repeat("x", 4096) }),
	}, {
		name: "message too long",
		req:  valid(func(r *ateapi.ReportActorCrashedRequest) { r.Message = strings.Repeat("x", 4097) }),
		want: field.ErrorList{field.TooLong(field.NewPath("message"), "", 4096).WithOrigin("maxLength")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateReportActorCrashedRequest(context.Background(), tt.req), tt.want)
		})
	}
}

// validWorker returns a Worker with only its required fields set: named, with
// its pod coordinates filled in. The withWorker* modifiers set the optional
// fields.
func validWorker(name string, mods ...func(*ateapi.Worker)) *ateapi.Worker {
	w := &ateapi.Worker{
		Metadata:        &ateapi.ResourceMetadata{Name: name},
		WorkerNamespace: "ate-system",
		WorkerPool:      "pool-1",
		WorkerPod:       "worker-pod-1",
		WorkerPodUid:    name,
		NodeName:        "node-1",
	}
	for _, m := range mods {
		m(w)
	}
	return w
}

// withWorkerMetadata returns a modifier func (see validWorker) which applies
// mods to the worker's metadata.
func withWorkerMetadata(mods ...func(*ateapi.ResourceMetadata)) func(*ateapi.Worker) {
	return func(w *ateapi.Worker) {
		for _, m := range mods {
			m(w.Metadata)
		}
	}
}

// withWorkerSpiffeID returns a modifier func (see validWorker) which sets the
// worker_spiffe_id.
func withWorkerSpiffeID(id string) func(*ateapi.Worker) {
	return func(w *ateapi.Worker) { w.WorkerSpiffeId = id }
}

// withWorkerLabels returns a modifier func (see validWorker) which sets the
// labels.
func withWorkerLabels(labels map[string]string) func(*ateapi.Worker) {
	return func(w *ateapi.Worker) { w.Labels = labels }
}

// numberedLabels returns n distinct labels.
func numberedLabels(n int) map[string]string {
	labels := make(map[string]string, n)
	for i := range n {
		labels[fmt.Sprintf("key-%d", i)] = "v"
	}
	return labels
}

// withWorkerStatus returns a modifier func (see validWorker) which sets the
// worker's status to a valid value with only its required state, and applies
// mods to it.
func withWorkerStatus(mods ...func(*ateapi.WorkerStatus)) func(*ateapi.Worker) {
	return func(w *ateapi.Worker) {
		w.Status = &ateapi.WorkerStatus{
			State: ateapi.WorkerState_WORKER_STATE_ACTIVE,
		}
		for _, m := range mods {
			m(w.Status)
		}
	}
}

// withWorkerStatusState returns a modifier func (see withWorkerStatus) which
// sets the state.
func withWorkerStatusState(state ateapi.WorkerState) func(*ateapi.WorkerStatus) {
	return func(s *ateapi.WorkerStatus) { s.State = state }
}

// withWorkerStatusIPs returns a modifier func (see withWorkerStatus) which
// sets the ips.
func withWorkerStatusIPs(ips ...string) func(*ateapi.WorkerStatus) {
	return func(s *ateapi.WorkerStatus) { s.Ips = ips }
}

// withWorkerStatusCapacity returns a modifier func (see withWorkerStatus)
// which sets the capacity.
func withWorkerStatusCapacity(capacity *ateapi.WorkerResources) func(*ateapi.WorkerStatus) {
	return func(s *ateapi.WorkerStatus) { s.Capacity = capacity }
}

// withWorkerStatusAllocated returns a modifier func (see withWorkerStatus)
// which sets the allocated resources.
func withWorkerStatusAllocated(allocated *ateapi.WorkerResources) func(*ateapi.WorkerStatus) {
	return func(s *ateapi.WorkerStatus) { s.Allocated = allocated }
}

// withWorkerStatusSandboxClass returns a modifier func (see
// withWorkerStatus) which appends a valid sandbox class with mods applied
// (see validSandboxClass).
func withWorkerStatusSandboxClass(class string, mods ...func(*ateapi.WorkerSandboxClass)) func(*ateapi.WorkerStatus) {
	return func(s *ateapi.WorkerStatus) {
		s.SandboxClasses = append(s.SandboxClasses, validSandboxClass(class, mods...))
	}
}

func workerRef(name string) *ateapi.ObjectRef {
	return &ateapi.ObjectRef{Name: name}
}

// validSandboxClass returns a WorkerSandboxClass named class with only its
// required default runtime. The withSandboxClass* modifiers set the rest.
func validSandboxClass(class string, mods ...func(*ateapi.WorkerSandboxClass)) *ateapi.WorkerSandboxClass {
	c := &ateapi.WorkerSandboxClass{
		SandboxClass:   class,
		DefaultRuntime: sandboxRuntime(),
	}
	for _, m := range mods {
		m(c)
	}
	return c
}

// withSandboxClassRestorableRuntime returns a modifier func (see
// validSandboxClass) which appends a valid restorable runtime with mods
// applied (see sandboxRuntime).
func withSandboxClassRestorableRuntime(mods ...func(*ateapi.WorkerSandboxRuntime)) func(*ateapi.WorkerSandboxClass) {
	return func(c *ateapi.WorkerSandboxClass) {
		c.RestorableRuntimes = append(c.RestorableRuntimes, sandboxRuntime(mods...))
	}
}

// withSandboxClassMultipleRestorableRuntimes returns a modifier func (see
// validSandboxClass) which appends n valid restorable runtimes.
func withSandboxClassMultipleRestorableRuntimes(n int) func(*ateapi.WorkerSandboxClass) {
	return func(c *ateapi.WorkerSandboxClass) {
		for range n {
			withSandboxClassRestorableRuntime()(c)
		}
	}
}

// sandboxRuntime returns a WorkerSandboxRuntime with only its required
// version, and applies mods to it.
func sandboxRuntime(mods ...func(*ateapi.WorkerSandboxRuntime)) *ateapi.WorkerSandboxRuntime {
	r := &ateapi.WorkerSandboxRuntime{Version: validSandboxCompat()}
	for _, m := range mods {
		m(r)
	}
	return r
}

// withRuntimeActorOverhead returns a modifier func (see sandboxRuntime) which
// sets the actor_overhead.
func withRuntimeActorOverhead(memoryBytes, memoryBytesPerGiB int64) func(*ateapi.WorkerSandboxRuntime) {
	return func(r *ateapi.WorkerSandboxRuntime) {
		r.ActorOverhead = &ateapi.RuntimeOverhead{MemoryBytes: memoryBytes, MemoryBytesPerGib: memoryBytesPerGiB}
	}
}

// validSandboxCompat returns a v1 compatibility identity with one attribute.
func validSandboxCompat() *ateapi.VersionedSandboxCompat {
	return &ateapi.VersionedSandboxCompat{
		SchemaVersion: "v1",
		Attributes:    []*ateapi.AttributeEntry{{Key: "architecture", Value: "amd64"}},
	}
}

// attributeEntries returns n valid attributes with distinct keys.
func attributeEntries(n int) []*ateapi.AttributeEntry {
	attrs := make([]*ateapi.AttributeEntry, n)
	for i := range attrs {
		attrs[i] = &ateapi.AttributeEntry{Key: fmt.Sprintf("k%d", i), Value: "v"}
	}
	return attrs
}
