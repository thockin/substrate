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
	"time"

	"go.ate.dev/substrate/api/ateapi/v1"
	"go.ate.dev/substrate/internal/preview"
	"go.ate.dev/substrate/internal/resources"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestValidateCreateActorTemplateRequest(t *testing.T) {
	withVolumeMount := withActorTemplateVolumeMount
	withDurableDirVolume := withActorTemplateDurableDirVolume
	withSnapshotConfig := withActorTemplateSnapshotConfig
	withGoldenEgressPolicy := withActorTemplateGoldenEgressPolicy
	withHTTPRule := withEgressPolicyTemplateHTTPRule
	withMultipleHTTPRules := withEgressPolicyTemplateMultipleHTTPRules
	withHTTPSRule := withEgressPolicyTemplateHTTPSRule
	withTLSPassthroughRule := withEgressPolicyTemplateTLSPassthroughRule
	tests := []struct {
		name string
		req  *ateapi.CreateActorTemplateRequest
		want field.ErrorList
	}{{
		"valid",
		&ateapi.CreateActorTemplateRequest{ActorTemplate: validActorTemplate()},
		nil,
	}, {
		"valid golden egress rules",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(
				withGoldenEgressPolicy(
					withHTTPRule(hostnames("api.example.com"), ports(80)))),
		},
		nil,
	}, {
		"missing golden egress rules",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(withGoldenEgressPolicy()),
		},
		field.ErrorList{field.Required(field.NewPath("actor_template", "golden_egress_policy", "rules"), "")},
	}, {
		"empty golden egress rules",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(
				withGoldenEgressPolicy(func(policy *ateapi.EgressPolicyTemplate) {
					policy.Rules = []*ateapi.EgressRule{}
				})),
		},
		field.ErrorList{field.Required(field.NewPath("actor_template", "golden_egress_policy", "rules"), "")},
	}, {
		"invalid golden egress hostname",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(
				withGoldenEgressPolicy(
					withHTTPRule(hostnames("bad/host"), ports(80)))),
		},
		field.ErrorList{
			field.Invalid(field.NewPath("actor_template", "golden_egress_policy", "rules").Index(0).Child("http", "hostnames").Index(0), nil, ""),
		},
	}, {
		"golden egress rules tie across protocols",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(
				withGoldenEgressPolicy(
					withHTTPSRule(hostnames("api.example.com"), ports(443)),
					withTLSPassthroughRule(hostnames("api.example.com"), ports(443)))),
		},
		field.ErrorList{
			field.Invalid(field.NewPath("actor_template", "golden_egress_policy", "rules").Index(1).Child("tls_passthrough", "hostnames").Index(0), nil, ""),
		},
	}, {
		"too many golden egress rules",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(
				withGoldenEgressPolicy(withMultipleHTTPRules(257))),
		},
		field.ErrorList{
			field.TooMany(field.NewPath("actor_template", "golden_egress_policy", "rules"), 257, 256).WithOrigin("maxItems"),
		},
	}, {
		"missing actor_template",
		&ateapi.CreateActorTemplateRequest{},
		field.ErrorList{field.Required(field.NewPath("actor_template"), "")},
	}, {
		"missing metadata.atespace",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(func(tmpl *ateapi.ActorTemplate) {
				tmpl.Metadata.Atespace = ""
			}),
		},
		field.ErrorList{field.Required(field.NewPath("actor_template", "metadata", "atespace"), "")},
	}, {
		"invalid metadata.atespace",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(func(tmpl *ateapi.ActorTemplate) {
				tmpl.Metadata.Atespace = "NS_1"
			}),
		},
		field.ErrorList{
			field.Invalid(field.NewPath("actor_template", "metadata", "atespace"), "NS_1", "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		"missing metadata.name",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(func(tmpl *ateapi.ActorTemplate) {
				tmpl.Metadata.Name = ""
			}),
		},
		field.ErrorList{field.Required(field.NewPath("actor_template", "metadata", "name"), "")},
	}, {
		"invalid metadata.name",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(func(tmpl *ateapi.ActorTemplate) {
				tmpl.Metadata.Name = "Tmpl_A"
			}),
		},
		field.ErrorList{
			field.Invalid(field.NewPath("actor_template", "metadata", "name"), "Tmpl_A", "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		"valid volumes fidelity",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(
				withSnapshotConfig(ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_VOLUMES)),
		},
		nil,
	}, {
		"valid rootfs fidelity",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(
				withSnapshotConfig(ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_ROOTFS)),
		},
		nil,
	}, {
		"valid memory fidelity",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(
				withSnapshotConfig(ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY)),
		},
		nil,
	}, {
		"invalid worker_selector label key",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(func(tmpl *ateapi.ActorTemplate) {
				tmpl.WorkerSelector = &ateapi.Selector{MatchLabels: map[string]string{"bad key": "v"}}
			}),
		},
		field.ErrorList{
			field.Invalid(field.NewPath("actor_template", "worker_selector", "match_labels"), "bad key", "").WithOrigin("format=k8s-label-key"),
		},
	}, {
		"no containers",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(func(tmpl *ateapi.ActorTemplate) {
				tmpl.Containers = nil
			}),
		},
		field.ErrorList{field.Required(field.NewPath("actor_template", "containers"), "")},
	}, {
		"container missing name",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(func(tmpl *ateapi.ActorTemplate) {
				tmpl.Containers[0].Name = ""
			}),
		},
		field.ErrorList{field.Required(field.NewPath("actor_template", "containers").Index(0).Child("name"), "")},
	}, {
		"container invalid name",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(func(tmpl *ateapi.ActorTemplate) {
				tmpl.Containers[0].Name = "Main_1"
			}),
		},
		field.ErrorList{
			field.Invalid(field.NewPath("actor_template", "containers").Index(0).Child("name"), "Main_1", "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		"container missing image",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(func(tmpl *ateapi.ActorTemplate) {
				tmpl.Containers[0].Image = ""
			}),
		},
		field.ErrorList{field.Required(field.NewPath("actor_template", "containers").Index(0).Child("image"), "")},
	}, {
		"volume mount referencing a declared volume",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(
				withDurableDirVolume("data"),
				withVolumeMount("data", "/var/data"),
			),
		},
		nil,
	}, {
		"volume mount referencing an undeclared volume",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(withVolumeMount("ghost-vol", "/var/data")),
		},
		field.ErrorList{
			field.Invalid(field.NewPath("actor_template", "containers").Index(0).Child("volume_mounts").Index(0).Child("name"), "ghost-vol", ""),
		},
	}, {
		"declared volume that no container mounts",
		&ateapi.CreateActorTemplateRequest{ActorTemplate: validActorTemplate(func(tmpl *ateapi.ActorTemplate) {
			tmpl.Volumes = []*ateapi.Volume{{Name: "data", DurableDir: &ateapi.DurableDirVolumeSource{}}}
		})},
		nil,
	}, {
		"valid storage_location",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(
				withSnapshotConfig(
					ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY,
					func(sc *ateapi.SnapshotConfig) {
						sc.StorageLocation = "gs://my-bucket/snapshots"
					})),
		},
		nil,
	}, {
		"storage_location without a bucket",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(
				withSnapshotConfig(
					ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY,
					func(sc *ateapi.SnapshotConfig) {
						sc.StorageLocation = "my-bucket/snapshots"
					})),
		},
		field.ErrorList{
			field.Invalid(field.NewPath("actor_template", "snapshot_config", "storage_location"), "my-bucket/snapshots", ""),
		},
	}, {
		"storage_location with a query",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(
				withSnapshotConfig(
					ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY,
					func(sc *ateapi.SnapshotConfig) {
						sc.StorageLocation = "gs://my-bucket/snapshots?versions=true"
					})),
		},
		field.ErrorList{
			field.Invalid(field.NewPath("actor_template", "snapshot_config", "storage_location"), "gs://my-bucket/snapshots?versions=true", ""),
		},
	}, {
		"storage_location under another actor's prefix",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(
				withSnapshotConfig(
					ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY,
					func(sc *ateapi.SnapshotConfig) {
						sc.StorageLocation = "gs://my-bucket/snapshots/atespaces/team-a/actors/0b4e7c4e-2f3a-4d43-9a5e-0f6d1c2b3a4d"
					})),
		},
		field.ErrorList{
			field.Invalid(field.NewPath("actor_template", "snapshot_config", "storage_location"), "gs://my-bucket/snapshots/atespaces/team-a/actors/0b4e7c4e-2f3a-4d43-9a5e-0f6d1c2b3a4d", ""),
		},
	}, {
		"storage_location with a dot in a path segment",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(
				withSnapshotConfig(
					ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY,
					func(sc *ateapi.SnapshotConfig) {
						sc.StorageLocation = "gs://my-bucket/snapshots.v1"
					})),
		},
		field.ErrorList{
			field.Invalid(field.NewPath("actor_template", "snapshot_config", "storage_location"), "gs://my-bucket/snapshots.v1", ""),
		},
	}, {
		// preferred_fidelity has no default of its own at this layer, so leaving it
		// unset is a required violation.
		"preferred_fidelity unset",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(
				withSnapshotConfig(ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_UNSPECIFIED)),
		},
		field.ErrorList{
			field.Required(field.NewPath("actor_template", "snapshot_config", "preferred_fidelity"), ""),
		},
	}, {
		"missing resources",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(func(tmpl *ateapi.ActorTemplate) {
				tmpl.Resources = nil
			}),
		},
		field.ErrorList{field.Required(field.NewPath("actor_template", "resources"), "")},
	}, {
		"missing cpu",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(func(tmpl *ateapi.ActorTemplate) {
				tmpl.Resources = &ateapi.ResourceRequirements{Limits: resources.CPUMemory(0, 512<<20)}
			}),
		},
		field.ErrorList{field.Required(field.NewPath("actor_template", "resources", "limits", "cpu_millicores"), "")},
	}, {
		"missing memory",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(func(tmpl *ateapi.ActorTemplate) {
				tmpl.Resources = &ateapi.ResourceRequirements{Limits: resources.CPUMemory(1000, 0)}
			}),
		},
		field.ErrorList{field.Required(field.NewPath("actor_template", "resources", "limits", "memory_bytes"), "")},
	}, {
		"empty resources",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(func(tmpl *ateapi.ActorTemplate) {
				tmpl.Resources = &ateapi.ResourceRequirements{}
			}),
		},
		field.ErrorList{
			field.Required(field.NewPath("actor_template", "resources", "limits", "cpu_millicores"), ""),
			field.Required(field.NewPath("actor_template", "resources", "limits", "memory_bytes"), ""),
		},
	}, {
		"nil golden egress rule",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(
				withGoldenEgressPolicy(func(policy *ateapi.EgressPolicyTemplate) {
					policy.Rules = []*ateapi.EgressRule{nil}
				})),
		},
		field.ErrorList{field.Required(field.NewPath("actor_template", "golden_egress_policy", "rules").Index(0), "")},
	}, {
		"nil golden egress replace_headers entry",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(
				withGoldenEgressPolicy(
					withHTTPRule(hostnames("api.example.com"), ports(80)),
					func(policy *ateapi.EgressPolicyTemplate) {
						policy.Rules[0].Http.Effects = &ateapi.HttpRuleEffects{ReplaceHeaders: []*ateapi.CredentialHeader{nil}}
					})),
		},
		field.ErrorList{
			field.Required(field.NewPath("actor_template", "golden_egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0), ""),
		},
	}, {
		// The check that a mount names a declared volume skips a mount with no
		// name; the required rule reports it.
		"volume mount missing name",
		&ateapi.CreateActorTemplateRequest{
			ActorTemplate: validActorTemplate(withVolumeMount("", "/var/data")),
		},
		field.ErrorList{
			field.Required(field.NewPath("actor_template", "containers").Index(0).Child("volume_mounts").Index(0).Child("name"), ""),
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := tt.want
			var wantWithPreview map[string]field.ErrorList
			if tt.req.GetActorTemplate().GetGoldenEgressPolicy() != nil {
				want = field.ErrorList{field.Forbidden(field.NewPath("actor_template", "golden_egress_policy"), "")}
				wantWithPreview = map[string]field.ErrorList{"GoldenEgressPolicy": tt.want, "ALL": tt.want}
			}
			if tt.req.GetActorTemplate().GetSnapshotConfig().GetStorageLocation() != "" {
				want = field.ErrorList{
					field.Forbidden(field.NewPath("actor_template", "snapshot_config", "storage_location"), ""),
				}
				wantWithPreview = map[string]field.ErrorList{"PerTemplateSnapshotLocation": tt.want, "ALL": tt.want}
			}
			assertValidate(t, func() field.ErrorList {
				return ValidateCreateActorTemplateRequest(context.Background(), tt.req)
			}, want, wantWithPreview)
		})
	}
}

func TestValidateCreateActorTemplateRequest_PreviewRestricted(t *testing.T) {
	withExternalVolume := withActorTemplateExternalVolume
	withSnapshotConfig := withActorTemplateSnapshotConfig
	setPreviewForTest(t, "ALL")
	req := &ateapi.CreateActorTemplateRequest{
		ActorTemplate: validActorTemplate(
			withExternalVolume("data", 10<<30, "fast-ssd"),
			withSnapshotConfig(
				ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY,
				func(sc *ateapi.SnapshotConfig) {
					sc.StorageLocation = "gs://my-bucket/snapshots"
				}),
		),
	}

	t.Run("allowed", func(t *testing.T) {
		assertValidateErr(t, ValidateCreateActorTemplateRequest(context.Background(), req), nil)
	})
	t.Run("restricted", func(t *testing.T) {
		ctx := preview.ContextWithOptOut(context.Background())
		want := field.ErrorList{
			field.Forbidden(field.NewPath("actor_template", "volumes").Index(0).Child("external"), ""),
			field.Forbidden(field.NewPath("actor_template", "snapshot_config", "storage_location"), ""),
		}
		assertValidateErr(t, ValidateCreateActorTemplateRequest(ctx, req), want)
	})
}

func TestValidateCreateActorTemplateRequest_PerTemplateSnapshotLocation(t *testing.T) {
	validSnapshotConfig := withActorTemplateSnapshotConfig
	storageLocation := field.NewPath("actor_template", "snapshot_config", "storage_location")
	withLocation := validSnapshotConfig(
		ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY,
		func(sc *ateapi.SnapshotConfig) {
			sc.StorageLocation = "gs://my-bucket/snapshots"
		})
	tests := []struct {
		name   string
		gates  string
		optOut bool
		mutate func(*ateapi.ActorTemplate)
		want   field.ErrorList
	}{{
		name:   "gate off, opted in, location set",
		mutate: withLocation,
		want:   field.ErrorList{field.Forbidden(storageLocation, "")},
	}, {
		name:   "gate off, opted out, location set",
		optOut: true,
		mutate: withLocation,
		want:   field.ErrorList{field.Forbidden(storageLocation, "")},
	}, {
		name:   "gate off, opted out, no location",
		optOut: true,
		mutate: validSnapshotConfig(ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY),
	}, {
		name:   "gate on, opted out, location set",
		gates:  "PerTemplateSnapshotLocation",
		optOut: true,
		mutate: withLocation,
		want:   field.ErrorList{field.Forbidden(storageLocation, "")},
	}, {
		name:   "gate on, opted out, no location",
		gates:  "PerTemplateSnapshotLocation",
		optOut: true,
		mutate: validSnapshotConfig(ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY),
	}, {
		name:   "gate on, opted in, location set",
		gates:  "PerTemplateSnapshotLocation",
		mutate: withLocation,
	}, {
		name:   "gate on, opted in, no location",
		gates:  "PerTemplateSnapshotLocation",
		mutate: validSnapshotConfig(ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setPreviewForTest(t, tt.gates)
			ctx := context.Background()
			if tt.optOut {
				ctx = preview.ContextWithOptOut(ctx)
			}
			var mutations []func(*ateapi.ActorTemplate)
			if tt.mutate != nil {
				mutations = append(mutations, tt.mutate)
			}
			req := &ateapi.CreateActorTemplateRequest{ActorTemplate: validActorTemplate(mutations...)}
			assertValidateErr(t, ValidateCreateActorTemplateRequest(ctx, req), tt.want)
		})
	}
}

// TestValidateCreateActorTemplateRequest_GoldenEgressNeedsGoldenSnapshots
// covers the cross-field rule that a golden egress policy needs golden
// snapshots. It holds whether or not the GoldenEgressPolicy preview is on.
func TestValidateCreateActorTemplateRequest_GoldenEgressNeedsGoldenSnapshots(t *testing.T) {
	withSnapshotConfig := withActorTemplateSnapshotConfig
	withGoldenEgressPolicy := withActorTemplateGoldenEgressPolicy
	withHTTPRule := withEgressPolicyTemplateHTTPRule
	goldenEgressPolicy := field.NewPath("actor_template", "golden_egress_policy")
	req := &ateapi.CreateActorTemplateRequest{
		ActorTemplate: validActorTemplate(
			withSnapshotConfig(
				ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY,
				func(sc *ateapi.SnapshotConfig) {
					sc.GoldenSnapshotConfig = &ateapi.GoldenSnapshotConfig{
						Mode: ateapi.GoldenSnapshotMode_GOLDEN_SNAPSHOT_MODE_DISABLED,
					}
				}),
			withGoldenEgressPolicy(
				withHTTPRule(hostnames("api.example.com"), ports(80)))),
	}
	assertValidate(t, func() field.ErrorList {
		return ValidateCreateActorTemplateRequest(context.Background(), req)
	}, field.ErrorList{
		// One error from the preview gate, one from the cross-field rule.
		field.Forbidden(goldenEgressPolicy, ""),
		field.Forbidden(goldenEgressPolicy, ""),
	}, map[string]field.ErrorList{
		"GoldenEgressPolicy": {field.Forbidden(goldenEgressPolicy, "")},
	})
}

func TestValidateGetActorTemplateRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *ateapi.GetActorTemplateRequest
		want field.ErrorList
	}{{
		"valid",
		&ateapi.GetActorTemplateRequest{ActorTemplate: &ateapi.ObjectRef{Atespace: "ns1", Name: "tmpl-a"}},
		nil,
	}, {
		"missing actor_template",
		&ateapi.GetActorTemplateRequest{},
		field.ErrorList{field.Required(field.NewPath("actor_template"), "")},
	}, {
		"missing atespace",
		&ateapi.GetActorTemplateRequest{ActorTemplate: &ateapi.ObjectRef{Name: "tmpl-a"}},
		field.ErrorList{field.Required(field.NewPath("actor_template", "atespace"), "")},
	}, {
		"missing name",
		&ateapi.GetActorTemplateRequest{ActorTemplate: &ateapi.ObjectRef{Atespace: "ns1"}},
		field.ErrorList{field.Required(field.NewPath("actor_template", "name"), "")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateGetActorTemplateRequest(context.Background(), tt.req), tt.want)
		})
	}
}

func TestValidateListActorTemplatesRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *ateapi.ListActorTemplatesRequest
		want field.ErrorList
	}{{
		"valid",
		&ateapi.ListActorTemplatesRequest{Options: &ateapi.ListOptions{PageSize: 10}},
		nil,
	}, {
		"zero page size",
		&ateapi.ListActorTemplatesRequest{},
		nil,
	}, {
		"valid atespace filter",
		&ateapi.ListActorTemplatesRequest{Atespace: "ns1"},
		nil,
	}, {
		"invalid atespace filter",
		&ateapi.ListActorTemplatesRequest{Atespace: "NS_1"},
		field.ErrorList{field.Invalid(field.NewPath("atespace"), "NS_1", "").WithOrigin("format=k8s-short-name")},
	}, {
		"negative page size",
		&ateapi.ListActorTemplatesRequest{Options: &ateapi.ListOptions{PageSize: -1}},
		field.ErrorList{field.Invalid(field.NewPath("options", "page_size"), int32(-1), "").WithOrigin("minimum")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateListActorTemplatesRequest(context.Background(), tt.req), tt.want)
		})
	}
}

func TestValidateDeleteActorTemplateRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *ateapi.DeleteActorTemplateRequest
		want field.ErrorList
	}{{
		"valid",
		&ateapi.DeleteActorTemplateRequest{ActorTemplate: &ateapi.ObjectRef{Atespace: "ns1", Name: "tmpl-a"}},
		nil,
	}, {
		"missing actor_template",
		&ateapi.DeleteActorTemplateRequest{},
		field.ErrorList{field.Required(field.NewPath("actor_template"), "")},
	}, {
		"missing atespace",
		&ateapi.DeleteActorTemplateRequest{ActorTemplate: &ateapi.ObjectRef{Name: "tmpl-a"}},
		field.ErrorList{field.Required(field.NewPath("actor_template", "atespace"), "")},
	}, {
		"missing name",
		&ateapi.DeleteActorTemplateRequest{ActorTemplate: &ateapi.ObjectRef{Atespace: "ns1"}},
		field.ErrorList{field.Required(field.NewPath("actor_template", "name"), "")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertValidateErr(t, ValidateDeleteActorTemplateRequest(context.Background(), tt.req), tt.want)
		})
	}
}

// TestValidateActorTemplate exercises the generated resource validation
// directly. The request handler still runs the hand-written validator; this
// pins each declarative rule as it is added, ahead of the conversion.
func TestValidateActorTemplate(t *testing.T) {
	withVolumeMount := withActorTemplateVolumeMount
	withDurableDirVolume := withActorTemplateDurableDirVolume
	withImageVolume := withActorTemplateImageVolume
	withExternalVolume := withActorTemplateExternalVolume
	withMultipleVolumeMounts := withActorTemplateMultipleVolumeMounts
	withMultipleDurableDirVolumes := withActorTemplateMultipleDurableDirVolumes
	withSnapshotConfig := withActorTemplateSnapshotConfig
	withMultipleEnvVars := func(n int) func(*ateapi.ActorTemplate) {
		return func(tmpl *ateapi.ActorTemplate) {
			c := tmpl.Containers[0]
			for i := range n {
				c.Env = append(c.Env, &ateapi.EnvVar{Name: fmt.Sprintf("VAR_%d", i)})
			}
		}
	}
	tests := []struct {
		name   string
		mutate func(*ateapi.ActorTemplate) // nil leaves the template valid
		// want is the result with no preview gates enabled.
		want field.ErrorList
		// wantWithPreview maps a --preview value to the result with those
		// gates enabled.
		wantWithPreview map[string]field.ErrorList
	}{{
		name: "valid",
	}, {
		name: "golden snapshot enabled without wakeup probe",
		mutate: withSnapshotConfig(
			ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY,
			func(sc *ateapi.SnapshotConfig) {
				sc.GoldenSnapshotConfig = &ateapi.GoldenSnapshotConfig{
					Mode: ateapi.GoldenSnapshotMode_GOLDEN_SNAPSHOT_MODE_ENABLED,
				}
			}),
	}, {
		name: "golden snapshot disabled",
		mutate: withSnapshotConfig(
			ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY,
			func(sc *ateapi.SnapshotConfig) {
				sc.GoldenSnapshotConfig = &ateapi.GoldenSnapshotConfig{
					Mode: ateapi.GoldenSnapshotMode_GOLDEN_SNAPSHOT_MODE_DISABLED,
				}
			}),
	}, {
		name: "golden snapshot mode unspecified",
		mutate: withSnapshotConfig(
			ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY,
			func(sc *ateapi.SnapshotConfig) {
				sc.GoldenSnapshotConfig = &ateapi.GoldenSnapshotConfig{}
			}),
		want: field.ErrorList{field.Required(field.NewPath("snapshot_config", "golden_snapshot_config", "mode"), "")},
	}, {
		name: "golden snapshot mode negative",
		mutate: withSnapshotConfig(
			ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY,
			func(sc *ateapi.SnapshotConfig) {
				sc.GoldenSnapshotConfig = &ateapi.GoldenSnapshotConfig{Mode: ateapi.GoldenSnapshotMode(-1)}
			}),
		want: field.ErrorList{
			field.Invalid(field.NewPath("snapshot_config", "golden_snapshot_config", "mode"), nil, "").WithOrigin("minimum"),
		},
	}, {
		name: "golden snapshot mode unknown",
		mutate: withSnapshotConfig(
			ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY,
			func(sc *ateapi.SnapshotConfig) {
				sc.GoldenSnapshotConfig = &ateapi.GoldenSnapshotConfig{Mode: ateapi.GoldenSnapshotMode(3)}
			}),
		want: field.ErrorList{
			field.Invalid(field.NewPath("snapshot_config", "golden_snapshot_config", "mode"), nil, "").WithOrigin("maximum"),
		},
	}, {
		name:   "missing metadata",
		mutate: func(tmpl *ateapi.ActorTemplate) { tmpl.Metadata = nil },
		want:   field.ErrorList{field.Required(field.NewPath("metadata"), "")},
	}, {
		name:   "missing metadata.atespace",
		mutate: func(tmpl *ateapi.ActorTemplate) { tmpl.Metadata.Atespace = "" },
		want:   field.ErrorList{field.Required(field.NewPath("metadata", "atespace"), "")},
	}, {
		name:   "invalid metadata.atespace",
		mutate: func(tmpl *ateapi.ActorTemplate) { tmpl.Metadata.Atespace = "NS1" },
		want: field.ErrorList{
			field.Invalid(field.NewPath("metadata", "atespace"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		name:   "invalid metadata.name",
		mutate: func(tmpl *ateapi.ActorTemplate) { tmpl.Metadata.Name = "TMPL A" },
		want: field.ErrorList{
			field.Invalid(field.NewPath("metadata", "name"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		name: "worker_selector with an invalid label value",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.WorkerSelector = &ateapi.Selector{MatchLabels: map[string]string{"tier": "Not Valid"}}
		},
		want: field.ErrorList{
			field.Invalid(field.NewPath("worker_selector", "match_labels").Key("tier"), nil, "").WithOrigin("format=k8s-label-value"),
		},
	}, {
		name:   "valid snapshot_config",
		mutate: withSnapshotConfig(ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY),
		want:   nil,
	}, {
		name: "storage_location too long",
		mutate: withSnapshotConfig(
			ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY,
			func(sc *ateapi.SnapshotConfig) {
				sc.StorageLocation = "gs://" + strings.Repeat("x", 1020)
			}),
		want: field.ErrorList{field.Forbidden(field.NewPath("snapshot_config", "storage_location"), "")},
		wantWithPreview: map[string]field.ErrorList{
			"PerTemplateSnapshotLocation": {
				field.TooLong(field.NewPath("snapshot_config", "storage_location"), nil, 1024).WithOrigin("maxLength"),
			},
		},
	}, {
		name:   "missing storage_location",
		mutate: withSnapshotConfig(ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_MEMORY),
		// Optional with or without the gate: an empty field falls back to
		// --snapshot-location.
		want:            nil,
		wantWithPreview: map[string]field.ErrorList{"PerTemplateSnapshotLocation": nil},
	}, {
		name:   "unspecified snapshot scope",
		mutate: withSnapshotConfig(ateapi.SnapshotFidelity_SNAPSHOT_FIDELITY_UNSPECIFIED),
		want: field.ErrorList{
			field.Required(field.NewPath("snapshot_config", "preferred_fidelity"), ""),
		},
	}, {
		name:   "preferred_fidelity outside the enum",
		mutate: withSnapshotConfig(ateapi.SnapshotFidelity(99)),
		want: field.ErrorList{
			field.Invalid(field.NewPath("snapshot_config", "preferred_fidelity"), nil, "").WithOrigin("maximum"),
		},
	}, {
		name:   "negative preferred_fidelity",
		mutate: withSnapshotConfig(ateapi.SnapshotFidelity(-1)),
		want: field.ErrorList{
			field.Invalid(field.NewPath("snapshot_config", "preferred_fidelity"), nil, "").WithOrigin("minimum"),
		},
	}, {
		name:   "no containers",
		mutate: func(tmpl *ateapi.ActorTemplate) { tmpl.Containers = nil },
		want:   field.ErrorList{field.Required(field.NewPath("containers"), "")},
	}, {
		name: "too many containers",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			for i := 0; i < 10; i++ {
				tmpl.Containers = append(tmpl.Containers, &ateapi.Container{
					Name:  fmt.Sprintf("c-%d", i),
					Image: "example.com/app:v1@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				})
			}
		},
		want: field.ErrorList{field.TooMany(field.NewPath("containers"), 11, 10).WithOrigin("maxItems")},
	}, {
		name: "duplicate container name",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers = append(tmpl.Containers, &ateapi.Container{
				Name:  "main",
				Image: "example.com/other:v1@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			})
		},
		want: field.ErrorList{field.Duplicate(field.NewPath("containers").Index(1), nil)},
	}, {
		name:   "the same volume mounted at two paths is allowed",
		mutate: combine(withVolumeMount("data", "/var/data"), withVolumeMount("data", "/mnt/data")),
	}, {
		name:   "two volumes at the same path are rejected",
		mutate: combine(withVolumeMount("data", "/var/data"), withVolumeMount("other", "/var/data")),
		want: field.ErrorList{
			field.Duplicate(field.NewPath("containers").Index(0).Child("volume_mounts").Index(1), nil),
		},
	}, {
		name:   "nested mount paths are rejected",
		mutate: combine(withVolumeMount("data", "/data"), withVolumeMount("config", "/data/config")),
		want: field.ErrorList{
			field.Invalid(field.NewPath("containers").Index(0).Child("volume_mounts").Index(1).Child("mount_path"), nil, ""),
		},
	}, {
		name: "the same path in different containers is allowed",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers = append(tmpl.Containers, &ateapi.Container{
				Name:  "sidecar",
				Image: "example.com/side:v1@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			})
			withVolumeMount("data", "/var/data")(tmpl)
			tmpl.Containers[1].VolumeMounts = []*ateapi.VolumeMount{{Name: "data", MountPath: "/var/data"}}
		},
	}, {
		name:   "duplicate volume name",
		mutate: combine(withDurableDirVolume("scratch"), withDurableDirVolume("scratch")),
		want:   field.ErrorList{field.Duplicate(field.NewPath("volumes").Index(1), nil)},
	}, {
		name: "too many command entries",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].Command = make([]string, 65)
		},
		want: field.ErrorList{
			field.TooMany(field.NewPath("containers").Index(0).Child("command"), 65, 64).WithOrigin("maxItems"),
		},
	}, {
		name: "command entry too long",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].Command = []string{strings.Repeat("x", 4097)}
		},
		want: field.ErrorList{
			field.TooLong(field.NewPath("containers").Index(0).Child("command").Index(0), nil, 4096).WithOrigin("maxLength"),
		},
	}, {
		name: "valid command and args",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].Command = []string{"/bin/app"}
			// Repeated argv values are legitimate; these lists are atomic,
			// not sets.
			tmpl.Containers[0].Args = []string{"--serve", "-v", "-v"}
		},
	}, {
		name: "too many args entries",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].Args = make([]string, 65)
		},
		want: field.ErrorList{
			field.TooMany(field.NewPath("containers").Index(0).Child("args"), 65, 64).WithOrigin("maxItems"),
		},
	}, {
		name: "args entry too long",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].Args = []string{strings.Repeat("x", 4097)}
		},
		want: field.ErrorList{
			field.TooLong(field.NewPath("containers").Index(0).Child("args").Index(0), nil, 4096).WithOrigin("maxLength"),
		},
	}, {
		name:   "too many volume_mounts",
		mutate: withMultipleVolumeMounts("data", 33),
		want: field.ErrorList{
			field.TooMany(field.NewPath("containers").Index(0).Child("volume_mounts"), 33, 32).WithOrigin("maxItems"),
		},
	}, {
		name: "image too long",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].Image = strings.Repeat("x", 513) + "@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
		},
		want: field.ErrorList{
			field.Invalid(field.NewPath("containers").Index(0).Child("image"), nil, ""),
			field.TooLong(field.NewPath("containers").Index(0).Child("image"), nil, 512).WithOrigin("maxLength"),
		},
	}, {
		name: "valid image: pinned by digest",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].Image = "example.com/app@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
		},
	}, {
		name: "container image missing digest",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].Image = "example.com/app:v1"
		},
		want: field.ErrorList{field.Invalid(field.NewPath("containers").Index(0).Child("image"), nil, "")},
	}, {
		name:   "container missing name",
		mutate: func(tmpl *ateapi.ActorTemplate) { tmpl.Containers[0].Name = "" },
		want:   field.ErrorList{field.Required(field.NewPath("containers").Index(0).Child("name"), "")},
	}, {
		name:   "container invalid name",
		mutate: func(tmpl *ateapi.ActorTemplate) { tmpl.Containers[0].Name = "Main_1" },
		want: field.ErrorList{
			field.Invalid(field.NewPath("containers").Index(0).Child("name"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		name:   "container missing image",
		mutate: func(tmpl *ateapi.ActorTemplate) { tmpl.Containers[0].Image = "" },
		want:   field.ErrorList{field.Required(field.NewPath("containers").Index(0).Child("image"), "")},
	}, {
		name: "valid env",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].Env = []*ateapi.EnvVar{{Name: "PORT", Value: "8080"}, {Name: "DEBUG"}}
		},
	}, {
		name:   "max number of envs",
		mutate: withMultipleEnvVars(32),
	}, {
		name:   "too many env entries",
		mutate: withMultipleEnvVars(33),
		want: field.ErrorList{
			field.TooMany(field.NewPath("containers").Index(0).Child("env"), 33, 32).WithOrigin("maxItems"),
		},
	}, {
		name: "env name with an equals sign",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].Env = []*ateapi.EnvVar{{Name: "FOO=BAR"}}
		},
		want: field.ErrorList{
			field.Invalid(field.NewPath("containers").Index(0).Child("env").Index(0).Child("name"), nil, ""),
		},
	}, {
		name: "env missing name",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].Env = []*ateapi.EnvVar{{Value: "8080"}}
		},
		want: field.ErrorList{
			field.Required(field.NewPath("containers").Index(0).Child("env").Index(0).Child("name"), ""),
		},
	}, {
		name:   "nil env entry",
		mutate: func(tmpl *ateapi.ActorTemplate) { tmpl.Containers[0].Env = []*ateapi.EnvVar{nil} },
		want:   field.ErrorList{field.Required(field.NewPath("containers").Index(0).Child("env").Index(0), "")},
	}, {
		name: "env name too long",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].Env = []*ateapi.EnvVar{{Name: strings.Repeat("A", 257)}}
		},
		want: field.ErrorList{
			field.TooLong(field.NewPath("containers").Index(0).Child("env").Index(0).Child("name"), nil, 256).WithOrigin("maxLength"),
		},
	}, {
		name: "env value too long",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].Env = []*ateapi.EnvVar{{Name: "BIG", Value: strings.Repeat("x", 32769)}}
		},
		want: field.ErrorList{
			field.TooLong(field.NewPath("containers").Index(0).Child("env").Index(0).Child("value"), nil, 32768).WithOrigin("maxLength"),
		},
	}, {
		name: "duplicate env name",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].Env = []*ateapi.EnvVar{{Name: "PORT", Value: "1"}, {Name: "PORT", Value: "2"}}
		},
		want: field.ErrorList{field.Duplicate(field.NewPath("containers").Index(0).Child("env").Index(1), nil)},
	}, {
		name: "valid security_context capabilities",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].SecurityContext = &ateapi.SecurityContext{
				Capabilities: &ateapi.Capabilities{
					Add:  []string{"NET_BIND_SERVICE"},
					Drop: []string{"ALL"},
				},
			}
		},
	}, {
		name: "capabilities add rejects ALL",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].SecurityContext = &ateapi.SecurityContext{
				Capabilities: &ateapi.Capabilities{Add: []string{"ALL"}},
			}
		},
		want: field.ErrorList{
			field.Invalid(field.NewPath("containers").Index(0).Child("security_context", "capabilities", "add").Index(0), nil, ""),
		},
	}, {
		name: "lowercase capability",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].SecurityContext = &ateapi.SecurityContext{
				Capabilities: &ateapi.Capabilities{Drop: []string{"net_raw"}},
			}
		},
		want: field.ErrorList{
			field.NotSupported[string](field.NewPath("containers").Index(0).Child("security_context", "capabilities", "drop").Index(0), nil, nil),
		},
	}, {
		name: "unknown capability",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].SecurityContext = &ateapi.SecurityContext{
				Capabilities: &ateapi.Capabilities{Add: []string{"NET_ADIMN"}},
			}
		},
		want: field.ErrorList{
			field.NotSupported[string](field.NewPath("containers").Index(0).Child("security_context", "capabilities", "add").Index(0), nil, nil),
		},
	}, {
		name: "capability outside the add allowlist",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].SecurityContext = &ateapi.SecurityContext{
				Capabilities: &ateapi.Capabilities{Add: []string{"MKNOD"}},
			}
		},
		want: field.ErrorList{
			field.NotSupported[string](field.NewPath("containers").Index(0).Child("security_context", "capabilities", "add").Index(0), nil, nil),
		},
	}, {
		name: "duplicate capability",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].SecurityContext = &ateapi.SecurityContext{
				Capabilities: &ateapi.Capabilities{Add: []string{"NET_BIND_SERVICE", "NET_BIND_SERVICE"}},
			}
		},
		want: field.ErrorList{
			field.Duplicate(field.NewPath("containers").Index(0).Child("security_context", "capabilities", "add").Index(1), nil),
		},
	}, {
		name: "too many capabilities",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			caps := make([]string, 65)
			for i := range caps {
				caps[i] = fmt.Sprintf("CAP%d", i)
			}
			tmpl.Containers[0].SecurityContext = &ateapi.SecurityContext{Capabilities: &ateapi.Capabilities{Add: caps}}
		},
		want: field.ErrorList{
			field.TooMany(field.NewPath("containers").Index(0).Child("security_context", "capabilities", "add"), 65, 64).WithOrigin("maxItems"),
		},
	}, {
		name: "valid wakeup probe",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].WakeupProbe = &ateapi.WakeupProbe{
				HttpGet:        &ateapi.HttpGetAction{Path: "/healthz", Port: 8080},
				TimeoutSeconds: 60,
			}
		},
	}, {
		name: "wakeup probe missing http_get",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].WakeupProbe = &ateapi.WakeupProbe{TimeoutSeconds: 60}
		},
		want: field.ErrorList{
			field.Required(field.NewPath("containers").Index(0).Child("wakeup_probe", "http_get"), ""),
		},
	}, {
		name: "missing wakeup probe timeout_seconds",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].WakeupProbe = &ateapi.WakeupProbe{
				HttpGet: &ateapi.HttpGetAction{Path: "/healthz", Port: 8080},
			}
		},
		want: field.ErrorList{
			field.Required(field.NewPath("containers").Index(0).Child("wakeup_probe", "timeout_seconds"), ""),
		},
	}, {
		name: "missing wakeup probe http_get.path",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].WakeupProbe = &ateapi.WakeupProbe{
				HttpGet:        &ateapi.HttpGetAction{Port: 8080},
				TimeoutSeconds: 60,
			}
		},
		want: field.ErrorList{
			field.Required(field.NewPath("containers").Index(0).Child("wakeup_probe", "http_get", "path"), ""),
		},
	}, {
		name: "wakeup probe timeout_seconds out of range",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].WakeupProbe = &ateapi.WakeupProbe{
				HttpGet:        &ateapi.HttpGetAction{Path: "/healthz", Port: 8080},
				TimeoutSeconds: 3601,
			}
		},
		want: field.ErrorList{
			field.Invalid(field.NewPath("containers").Index(0).Child("wakeup_probe", "timeout_seconds"), nil, "").WithOrigin("maximum"),
		},
	}, {
		name: "negative wakeup probe timeout_seconds",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].WakeupProbe = &ateapi.WakeupProbe{
				HttpGet:        &ateapi.HttpGetAction{Path: "/healthz", Port: 8080},
				TimeoutSeconds: -1,
			}
		},
		want: field.ErrorList{
			field.Invalid(field.NewPath("containers").Index(0).Child("wakeup_probe", "timeout_seconds"), nil, "").WithOrigin("minimum"),
		},
	}, {
		name: "wakeup probe missing port",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].WakeupProbe = &ateapi.WakeupProbe{
				HttpGet:        &ateapi.HttpGetAction{Path: "/healthz"},
				TimeoutSeconds: 60,
			}
		},
		want: field.ErrorList{
			field.Required(field.NewPath("containers").Index(0).Child("wakeup_probe", "http_get", "port"), ""),
		},
	}, {
		name: "negative wakeup probe port",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].WakeupProbe = &ateapi.WakeupProbe{
				HttpGet:        &ateapi.HttpGetAction{Path: "/healthz", Port: -1},
				TimeoutSeconds: 60,
			}
		},
		want: field.ErrorList{
			field.Invalid(field.NewPath("containers").Index(0).Child("wakeup_probe", "http_get", "port"), nil, "").WithOrigin("minimum"),
		},
	}, {
		name: "wakeup probe port out of range",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].WakeupProbe = &ateapi.WakeupProbe{
				HttpGet:        &ateapi.HttpGetAction{Path: "/healthz", Port: 65536},
				TimeoutSeconds: 60,
			}
		},
		want: field.ErrorList{
			field.Invalid(field.NewPath("containers").Index(0).Child("wakeup_probe", "http_get", "port"), nil, "").WithOrigin("maximum"),
		},
	}, {
		name: "wakeup probe path with query string",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].WakeupProbe = &ateapi.WakeupProbe{
				HttpGet:        &ateapi.HttpGetAction{Path: "/readyz?verbose=1", Port: 8080},
				TimeoutSeconds: 60,
			}
		},
		want: field.ErrorList{
			field.Invalid(field.NewPath("containers").Index(0).Child("wakeup_probe", "http_get", "path"), nil, ""),
		},
	}, {
		name: "wakeup probe path not starting with slash",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].WakeupProbe = &ateapi.WakeupProbe{
				HttpGet:        &ateapi.HttpGetAction{Path: "readyz", Port: 8080},
				TimeoutSeconds: 60,
			}
		},
		want: field.ErrorList{
			field.Invalid(field.NewPath("containers").Index(0).Child("wakeup_probe", "http_get", "path"), nil, ""),
		},
	}, {
		name:   "valid volume_mount",
		mutate: withVolumeMount("data", "/var/data"),
	}, {
		name:   "volume_mount missing name",
		mutate: withVolumeMount("", "/var/data"),
		want: field.ErrorList{
			field.Required(field.NewPath("containers").Index(0).Child("volume_mounts").Index(0).Child("name"), ""),
		},
	}, {
		name:   "volume_mount invalid name",
		mutate: withVolumeMount("Data_1", "/var/data"),
		want: field.ErrorList{
			field.Invalid(field.NewPath("containers").Index(0).Child("volume_mounts").Index(0).Child("name"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		name:   "volume_mount missing mount_path",
		mutate: withVolumeMount("data", ""),
		want: field.ErrorList{
			field.Required(field.NewPath("containers").Index(0).Child("volume_mounts").Index(0).Child("mount_path"), ""),
		},
	}, {
		name:   "relative mount_path",
		mutate: withVolumeMount("data", "var/data"),
		want: field.ErrorList{
			field.Invalid(field.NewPath("containers").Index(0).Child("volume_mounts").Index(0).Child("mount_path"), nil, ""),
		},
	}, {
		name:   "valid durable_dir volume",
		mutate: withDurableDirVolume("scratch"),
	}, {
		name:   "too many volumes",
		mutate: withMultipleDurableDirVolumes(33),
		want:   field.ErrorList{field.TooMany(field.NewPath("volumes"), 33, 32).WithOrigin("maxItems")},
	}, {
		name:   "volume missing name",
		mutate: withDurableDirVolume(""),
		want:   field.ErrorList{field.Required(field.NewPath("volumes").Index(0).Child("name"), "")},
	}, {
		name:   "volume invalid name",
		mutate: withDurableDirVolume("Scratch_1"),
		want: field.ErrorList{
			field.Invalid(field.NewPath("volumes").Index(0).Child("name"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		name: "volume with no source",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Volumes = []*ateapi.Volume{{Name: "scratch"}}
		},
		want: field.ErrorList{field.Invalid(field.NewPath("volumes").Index(0), nil, "one of").WithOrigin("union")},
	}, {
		name: "volume with two sources",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Volumes = []*ateapi.Volume{{
				Name:       "scratch",
				DurableDir: &ateapi.DurableDirVolumeSource{},
				Image: &ateapi.ImageVolumeSource{
					Reference: "example.com/app@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				},
			}}
		},
		want: field.ErrorList{field.Invalid(field.NewPath("volumes").Index(0), nil, "one of").WithOrigin("union")},
	}, {
		name:   "valid image volume",
		mutate: withImageVolume("tools", "example.com/app@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"),
	}, {
		name:   "image volume missing reference",
		mutate: withImageVolume("tools", ""),
		want:   field.ErrorList{field.Required(field.NewPath("volumes").Index(0).Child("image", "reference"), "")},
	}, {
		name:   "image volume reference missing digest",
		mutate: withImageVolume("tools", "example.com/app:v1"),
		want:   field.ErrorList{field.Invalid(field.NewPath("volumes").Index(0).Child("image", "reference"), nil, "")},
	}, {
		name:            "valid external volume template",
		mutate:          withExternalVolume("data", 10<<30, "fast-ssd"),
		want:            field.ErrorList{field.Forbidden(field.NewPath("volumes").Index(0).Child("external"), "")},
		wantWithPreview: map[string]field.ErrorList{"ALL": nil},
	}, {
		name:   "external volume template missing capacity",
		mutate: withExternalVolume("data", 0, "fast-ssd"),
		want:   field.ErrorList{field.Forbidden(field.NewPath("volumes").Index(0).Child("external"), "")},
		wantWithPreview: map[string]field.ErrorList{
			"ALL": {
				field.Required(field.NewPath("volumes").Index(0).Child("external", "template", "capacity_bytes"), ""),
			},
		},
	}, {
		name:   "external volume template negative capacity",
		mutate: withExternalVolume("data", -1, "fast-ssd"),
		want:   field.ErrorList{field.Forbidden(field.NewPath("volumes").Index(0).Child("external"), "")},
		wantWithPreview: map[string]field.ErrorList{
			"ALL": {
				field.Invalid(field.NewPath("volumes").Index(0).Child("external", "template", "capacity_bytes"), nil, "").WithOrigin("minimum"),
			},
		},
	}, {
		name:   "external volume template missing storage_class_name",
		mutate: withExternalVolume("data", 10<<30, ""),
		want:   field.ErrorList{field.Forbidden(field.NewPath("volumes").Index(0).Child("external"), "")},
		wantWithPreview: map[string]field.ErrorList{
			"ALL": {
				field.Required(field.NewPath("volumes").Index(0).Child("external", "template", "storage_class_name"), ""),
			},
		},
	}, {
		name:   "external volume template invalid storage_class_name",
		mutate: withExternalVolume("data", 10<<30, "Fast SSD"),
		want:   field.ErrorList{field.Forbidden(field.NewPath("volumes").Index(0).Child("external"), "")},
		wantWithPreview: map[string]field.ErrorList{
			"ALL": {
				field.Invalid(field.NewPath("volumes").Index(0).Child("external", "template", "storage_class_name"), nil, "").WithOrigin("format=k8s-long-name"),
			},
		},
	}, {
		name: "valid resources",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].Resources = &ateapi.ResourceRequirements{Limits: resources.CPUMemory(500, 2<<30)}
		},
	}, {
		name: "negative cpu",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].Resources = &ateapi.ResourceRequirements{Limits: &ateapi.Resources{CpuMillicores: -1}}
		},
		want: field.ErrorList{
			field.Invalid(field.NewPath("containers").Index(0).Child("resources", "limits", "cpu_millicores"), nil, "").WithOrigin("minimum"),
		},
	}, {
		name: "negative memory",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].Resources = &ateapi.ResourceRequirements{Limits: &ateapi.Resources{MemoryBytes: -1}}
		},
		want: field.ErrorList{
			field.Invalid(field.NewPath("containers").Index(0).Child("resources", "limits", "memory_bytes"), nil, "").WithOrigin("minimum"),
		},
	}, {
		name: "cpu above maximum",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].Resources = &ateapi.ResourceRequirements{
				Limits: &ateapi.Resources{CpuMillicores: 1000000},
			}
		},
		want: field.ErrorList{
			field.Invalid(field.NewPath("containers").Index(0).Child("resources", "limits", "cpu_millicores"), nil, "").WithOrigin("maximum"),
		},
	}, {
		name: "template-level resources validated too",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Resources = &ateapi.ResourceRequirements{
				Limits: &ateapi.Resources{CpuMillicores: 1000, MemoryBytes: -1},
			}
		},
		want: field.ErrorList{
			field.Invalid(field.NewPath("resources", "limits", "memory_bytes"), nil, "").WithOrigin("minimum"),
		},
	}, {
		name:   "nil container",
		mutate: func(tmpl *ateapi.ActorTemplate) { tmpl.Containers = append(tmpl.Containers, nil) },
		want:   field.ErrorList{field.Required(field.NewPath("containers").Index(1), "")},
	}, {
		name:   "nil volume",
		mutate: func(tmpl *ateapi.ActorTemplate) { tmpl.Volumes = []*ateapi.Volume{nil} },
		want:   field.ErrorList{field.Required(field.NewPath("volumes").Index(0), "")},
	}, {
		name:   "nil volume_mount",
		mutate: func(tmpl *ateapi.ActorTemplate) { tmpl.Containers[0].VolumeMounts = []*ateapi.VolumeMount{nil} },
		want:   field.ErrorList{field.Required(field.NewPath("containers").Index(0).Child("volume_mounts").Index(0), "")},
	}, {
		name: "wakeup probe path too long",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].WakeupProbe = &ateapi.WakeupProbe{
				HttpGet:        &ateapi.HttpGetAction{Path: "/" + strings.Repeat("p", 1024), Port: 8080},
				TimeoutSeconds: 60,
			}
		},
		want: field.ErrorList{
			field.TooLong(field.NewPath("containers").Index(0).Child("wakeup_probe", "http_get", "path"), nil, 1024).WithOrigin("maxLength"),
		},
	}, {
		name: "image volume reference too long",
		// A well-formed reference cannot reach the length limit, so this one
		// is also malformed.
		mutate: withImageVolume("tools", "example.com/"+strings.Repeat("x", 512)+"@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"),
		want: field.ErrorList{
			field.TooLong(field.NewPath("volumes").Index(0).Child("image", "reference"), nil, 512).WithOrigin("maxLength"),
			field.Invalid(field.NewPath("volumes").Index(0).Child("image", "reference"), nil, ""),
		},
	}, {
		name:   "mount_path too long",
		mutate: withVolumeMount("data", "/"+strings.Repeat("m", 4096)),
		want: field.ErrorList{
			field.TooLong(field.NewPath("containers").Index(0).Child("volume_mounts").Index(0).Child("mount_path"), nil, 4096).WithOrigin("maxLength"),
		},
	}, {
		name: "duplicate dropped capability",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].SecurityContext = &ateapi.SecurityContext{
				Capabilities: &ateapi.Capabilities{Drop: []string{"NET_RAW", "NET_RAW"}},
			}
		},
		want: field.ErrorList{
			field.Duplicate(field.NewPath("containers").Index(0).Child("security_context", "capabilities", "drop").Index(1), nil),
		},
	}, {
		name: "too many dropped capabilities",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			caps := make([]string, 65)
			for i := range caps {
				caps[i] = fmt.Sprintf("CAP%d", i)
			}
			tmpl.Containers[0].SecurityContext = &ateapi.SecurityContext{Capabilities: &ateapi.Capabilities{Drop: caps}}
		},
		want: field.ErrorList{
			field.TooMany(field.NewPath("containers").Index(0).Child("security_context", "capabilities", "drop"), 65, 64).WithOrigin("maxItems"),
		},
	}, {
		name: "security_context without capabilities",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Containers[0].SecurityContext = &ateapi.SecurityContext{}
		},
	}, {
		name: "external volume without a template",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Volumes = []*ateapi.Volume{{Name: "data", External: &ateapi.ExternalVolumeSource{}}}
		},
		want: field.ErrorList{field.Forbidden(field.NewPath("volumes").Index(0).Child("external"), "")},
		wantWithPreview: map[string]field.ErrorList{
			"ALL": {field.Invalid(field.NewPath("volumes").Index(0).Child("external"), nil, "one of").WithOrigin("union")},
		},
	}, {
		name: "status without golden_snapshot_status",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Status = &ateapi.ActorTemplateStatus{}
		},
	}, {
		name: "empty worker_selector match_labels",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.WorkerSelector = &ateapi.Selector{MatchLabels: map[string]string{}}
		},
		want: field.ErrorList{field.Invalid(field.NewPath("worker_selector"), nil, "one of").WithOrigin("union")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl := validActorTemplate()
			if tt.mutate != nil {
				tt.mutate(tmpl)
			}
			assertValidate(t, func() field.ErrorList {
				op := MakeCreateOp(context.Background())
				return Validate_ActorTemplate(context.Background(), op, nil, tmpl, nil)
			}, tt.want, tt.wantWithPreview)
		})
	}
}

// TestValidateActorTemplateUpdate covers the rules that compare an
// ActorTemplate to its stored value. TestValidateActorTemplate covers the rest.
func TestValidateActorTemplateUpdate(t *testing.T) {
	stored := func() *ateapi.ActorTemplate {
		return validActorTemplate(func(tmpl *ateapi.ActorTemplate) {
			tmpl.Metadata.Uid = "01234567-89ab-cdef-0123-456789abcdef"
			tmpl.Metadata.Version = 3
			tmpl.Metadata.CreateTime = timestamppb.New(time.Unix(1000, 0))
			tmpl.Metadata.UpdateTime = timestamppb.New(time.Unix(2000, 0))
			tmpl.Status = &ateapi.ActorTemplateStatus{}
		})
	}
	tests := []struct {
		name   string
		mutate func(*ateapi.ActorTemplate)
		want   field.ErrorList
	}{{
		name: "unchanged",
	}, {
		name: "golden snapshot status set",
		mutate: func(tmpl *ateapi.ActorTemplate) {
			tmpl.Status.GoldenSnapshotStatus = &ateapi.GoldenSnapshotStatus{ErrorMessage: "failed"}
		},
	}, {
		name:   "metadata cleared",
		mutate: func(tmpl *ateapi.ActorTemplate) { tmpl.Metadata = nil },
		want:   field.ErrorList{field.Required(field.NewPath("metadata"), "")},
	}, {
		name:   "metadata.atespace changed",
		mutate: func(tmpl *ateapi.ActorTemplate) { tmpl.Metadata.Atespace = "ns2" },
		want:   field.ErrorList{field.Invalid(field.NewPath("metadata", "atespace"), nil, "").WithOrigin("immutable")},
	}, {
		name:   "metadata.name changed",
		mutate: func(tmpl *ateapi.ActorTemplate) { tmpl.Metadata.Name = "tmpl-b" },
		want:   field.ErrorList{field.Invalid(field.NewPath("metadata", "name"), nil, "").WithOrigin("immutable")},
	}, {
		name:   "metadata.uid changed",
		mutate: func(tmpl *ateapi.ActorTemplate) { tmpl.Metadata.Uid = "fedcba98-7654-3210-fedc-ba9876543210" },
		want:   field.ErrorList{field.Invalid(field.NewPath("metadata", "uid"), nil, "").WithOrigin("immutable")},
	}, {
		name:   "metadata.create_time changed",
		mutate: func(tmpl *ateapi.ActorTemplate) { tmpl.Metadata.CreateTime = timestamppb.New(time.Unix(1500, 0)) },
		want: field.ErrorList{
			field.Invalid(field.NewPath("metadata", "create_time"), nil, "").WithOrigin("immutable"),
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newVal := stored()
			if tt.mutate != nil {
				tt.mutate(newVal)
			}
			assertValidate(t, func() field.ErrorList {
				return ValidateActorTemplateUpdate(context.Background(), nil, newVal, stored())
			}, tt.want, nil)
		})
	}
}

// validActorTemplate returns the smallest template that passes create
// validation; mutations tweak it per test case. The snapshot scopes and
// resume policy are set explicitly because TestValidateActorTemplate
// exercises validation directly, without defaulting.
func validActorTemplate(mutations ...func(*ateapi.ActorTemplate)) *ateapi.ActorTemplate {
	template := &ateapi.ActorTemplate{
		Metadata: &ateapi.ResourceMetadata{Atespace: "ns1", Name: "tmpl-a"},
		Containers: []*ateapi.Container{{
			Name:  "main",
			Image: "example.com/app:v1@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		}},
		Resources: &ateapi.ResourceRequirements{Limits: resources.CPUMemory(1000, 512<<20)},
	}
	for _, m := range mutations {
		m(template)
	}
	return template
}

// withActorTemplateGoldenEgressPolicy returns a modifier func (see
// validActorTemplate) which sets the template's golden_egress_policy to an
// empty policy and applies mods to it. The withEgressPolicyTemplate*Rule
// modifiers add rules.
func withActorTemplateGoldenEgressPolicy(mods ...func(*ateapi.EgressPolicyTemplate)) func(*ateapi.ActorTemplate) {
	return func(tmpl *ateapi.ActorTemplate) {
		tmpl.GoldenEgressPolicy = &ateapi.EgressPolicyTemplate{}
		for _, m := range mods {
			m(tmpl.GoldenEgressPolicy)
		}
	}
}

// withEgressPolicyTemplateHTTPRule returns a modifier func (see
// withActorTemplateGoldenEgressPolicy) which appends an HTTP rule.
func withEgressPolicyTemplateHTTPRule(hosts []string, p *ateapi.Ports) func(*ateapi.EgressPolicyTemplate) {
	return func(policy *ateapi.EgressPolicyTemplate) {
		policy.Rules = append(policy.Rules, &ateapi.EgressRule{Http: &ateapi.HttpRule{Hostnames: hosts, Ports: p}})
	}
}

// withEgressPolicyTemplateMultipleHTTPRules returns a modifier func (see
// withActorTemplateGoldenEgressPolicy) which appends n HTTP rules, each for a
// distinct hostname.
func withEgressPolicyTemplateMultipleHTTPRules(n int) func(*ateapi.EgressPolicyTemplate) {
	return func(policy *ateapi.EgressPolicyTemplate) {
		policy.Rules = append(policy.Rules, numberedHTTPRules(n)...)
	}
}

// withEgressPolicyTemplateHTTPSRule returns a modifier func (see
// withActorTemplateGoldenEgressPolicy) which appends an HTTPS rule.
func withEgressPolicyTemplateHTTPSRule(hosts []string, p *ateapi.Ports) func(*ateapi.EgressPolicyTemplate) {
	return func(policy *ateapi.EgressPolicyTemplate) {
		policy.Rules = append(policy.Rules, &ateapi.EgressRule{Https: &ateapi.HttpsRule{Hostnames: hosts, Ports: p}})
	}
}

// withEgressPolicyTemplateTLSPassthroughRule returns a modifier func (see
// withActorTemplateGoldenEgressPolicy) which appends a TLS passthrough rule.
func withEgressPolicyTemplateTLSPassthroughRule(hosts []string, p *ateapi.Ports) func(*ateapi.EgressPolicyTemplate) {
	return func(policy *ateapi.EgressPolicyTemplate) {
		policy.Rules = append(policy.Rules, &ateapi.EgressRule{
			TlsPassthrough: &ateapi.TlsPassthroughRule{Hostnames: hosts, Ports: p},
		})
	}
}

// withActorTemplateSnapshotConfig returns a modifier func (see
// validActorTemplate) which sets the template's snapshot_config to a valid
// value with fidelity as its required preferred_fidelity, and applies mods to
// it.
func withActorTemplateSnapshotConfig(fidelity ateapi.SnapshotFidelity,
	mods ...func(*ateapi.SnapshotConfig)) func(*ateapi.ActorTemplate) {
	return func(tmpl *ateapi.ActorTemplate) {
		tmpl.SnapshotConfig = &ateapi.SnapshotConfig{PreferredFidelity: fidelity}
		for _, m := range mods {
			m(tmpl.SnapshotConfig)
		}
	}
}

// withActorTemplateDurableDirVolume returns a modifier func (see
// validActorTemplate) which appends a durable_dir volume named name.
func withActorTemplateDurableDirVolume(name string) func(*ateapi.ActorTemplate) {
	return func(tmpl *ateapi.ActorTemplate) {
		tmpl.Volumes = append(tmpl.Volumes, &ateapi.Volume{Name: name, DurableDir: &ateapi.DurableDirVolumeSource{}})
	}
}

// withActorTemplateMultipleDurableDirVolumes returns a modifier func (see
// validActorTemplate) which appends n durable_dir volumes with distinct names.
func withActorTemplateMultipleDurableDirVolumes(n int) func(*ateapi.ActorTemplate) {
	return func(tmpl *ateapi.ActorTemplate) {
		for i := range n {
			withActorTemplateDurableDirVolume(fmt.Sprintf("vol-%d", i))(tmpl)
		}
	}
}

// withActorTemplateImageVolume returns a modifier func (see
// validActorTemplate) which appends a volume named name, backed by the image
// at reference.
func withActorTemplateImageVolume(name, reference string) func(*ateapi.ActorTemplate) {
	return func(tmpl *ateapi.ActorTemplate) {
		tmpl.Volumes = append(tmpl.Volumes, &ateapi.Volume{
			Name:  name,
			Image: &ateapi.ImageVolumeSource{Reference: reference},
		})
	}
}

// withActorTemplateVolumeMount returns a modifier func (see
// validActorTemplate) which mounts the volume named name at mountPath in the
// template's first container.
func withActorTemplateVolumeMount(name, mountPath string) func(*ateapi.ActorTemplate) {
	return func(tmpl *ateapi.ActorTemplate) {
		c := tmpl.Containers[0]
		c.VolumeMounts = append(c.VolumeMounts, &ateapi.VolumeMount{Name: name, MountPath: mountPath})
	}
}

// withActorTemplateMultipleVolumeMounts returns a modifier func (see
// validActorTemplate) which appends n mounts of the volume name at distinct
// paths to the first container.
func withActorTemplateMultipleVolumeMounts(name string, n int) func(*ateapi.ActorTemplate) {
	return func(tmpl *ateapi.ActorTemplate) {
		for i := range n {
			withActorTemplateVolumeMount(name, fmt.Sprintf("/mnt/p%d", i))(tmpl)
		}
	}
}

// withActorTemplateExternalVolume returns a modifier func (see
// validActorTemplate) which appends a volume named name, backed by an
// external volume created from a template with the given capacity and
// storage class.
func withActorTemplateExternalVolume(name string, capacityBytes int64, storageClassName string) func(*ateapi.ActorTemplate) {
	return func(tmpl *ateapi.ActorTemplate) {
		tmpl.Volumes = append(tmpl.Volumes, &ateapi.Volume{
			Name: name,
			External: &ateapi.ExternalVolumeSource{
				Template: &ateapi.ExternalVolumeTemplate{
					CapacityBytes:    capacityBytes,
					StorageClassName: storageClassName,
				},
			},
		})
	}
}

// validSystemInfoVolumeSource returns a SystemInfoVolumeSource with no data
// sources, since they are optional. The withSystemInfo* modifiers add them.
func validSystemInfoVolumeSource(mutate ...func(*ateapi.SystemInfoVolumeSource)) *ateapi.SystemInfoVolumeSource {
	s := &ateapi.SystemInfoVolumeSource{}
	for _, m := range mutate {
		m(s)
	}
	return s
}

// withSystemInfoActorMetadata returns a modifier func (see
// validSystemInfoVolumeSource) which appends an actor_metadata data source
// projecting items.
func withSystemInfoActorMetadata(items ...*ateapi.ActorMetadataItem) func(*ateapi.SystemInfoVolumeSource) {
	return func(s *ateapi.SystemInfoVolumeSource) {
		s.DataSources = append(s.DataSources, &ateapi.SystemInfoDataSource{
			ActorMetadata: &ateapi.ActorMetadataDataSource{Items: items},
		})
	}
}

// actorMetadataItem returns an ActorMetadataItem projecting field to path.
func actorMetadataItem(field ateapi.ActorMetadataField, path string) *ateapi.ActorMetadataItem {
	return &ateapi.ActorMetadataItem{Field: field, Path: path}
}

// numberedActorMetadataItems returns n name items at distinct paths.
func numberedActorMetadataItems(n int) []*ateapi.ActorMetadataItem {
	items := make([]*ateapi.ActorMetadataItem, n)
	for i := range items {
		items[i] = actorMetadataItem(ateapi.ActorMetadataField_ACTOR_METADATA_FIELD_NAME, fmt.Sprintf("p%d", i))
	}
	return items
}

// withSystemInfoTrustBundle returns a modifier func (see
// validSystemInfoVolumeSource) which appends a trust_bundle data source
// writing the named bundles to path.
func withSystemInfoTrustBundle(path string, names ...string) func(*ateapi.SystemInfoVolumeSource) {
	return func(s *ateapi.SystemInfoVolumeSource) {
		s.DataSources = append(s.DataSources, &ateapi.SystemInfoDataSource{
			TrustBundle: &ateapi.TrustBundleDataSource{Names: names, Path: path},
		})
	}
}

func TestValidateSystemInfoVolumeSource(t *testing.T) {
	// Most cases start from these two data sources, in this order.
	withActorMetadata := func() func(*ateapi.SystemInfoVolumeSource) {
		return withSystemInfoActorMetadata(
			&ateapi.ActorMetadataItem{Field: ateapi.ActorMetadataField_ACTOR_METADATA_FIELD_NAME, Path: "actor-name"},
			&ateapi.ActorMetadataItem{Field: ateapi.ActorMetadataField_ACTOR_METADATA_FIELD_UID, Path: "actor-uid"})
	}
	withTrustBundle := func() func(*ateapi.SystemInfoVolumeSource) {
		return withSystemInfoTrustBundle("trust-bundle.pem", "egress-mitm.ate.dev")
	}
	valid := validSystemInfoVolumeSource
	dsPath := field.NewPath("data_sources")
	itemsPath := dsPath.Index(0).Child("actor_metadata", "items")

	tests := []struct {
		name string
		obj  *ateapi.SystemInfoVolumeSource
		want field.ErrorList
	}{{
		name: "valid",
		obj:  valid(),
	}, {
		name: "valid with data sources",
		obj:  valid(withActorMetadata(), withTrustBundle()),
	}, {
		name: "many data sources",
		obj: valid(
			func(s *ateapi.SystemInfoVolumeSource) {
				for i := range 8 {
					withSystemInfoTrustBundle(fmt.Sprintf("tb%d.pem", i), "egress-mitm.ate.dev")(s)
				}
			},
		),
	}, {
		name: "too many data sources",
		obj: valid(
			func(s *ateapi.SystemInfoVolumeSource) {
				for i := range 9 {
					withSystemInfoTrustBundle(fmt.Sprintf("tb%d.pem", i), "egress-mitm.ate.dev")(s)
				}
			},
		),
		want: field.ErrorList{field.TooMany(dsPath, 9, 8).WithOrigin("maxItems")},
	}, {
		name: "no union member set",
		obj: valid(
			withActorMetadata(),
			func(s *ateapi.SystemInfoVolumeSource) {
				s.DataSources[0].ActorMetadata = nil
			},
		),
		want: field.ErrorList{field.Invalid(dsPath.Index(0), nil, "").WithOrigin("union")},
	}, {
		name: "both union members set",
		obj: valid(
			withActorMetadata(),
			func(s *ateapi.SystemInfoVolumeSource) {
				s.DataSources[0].TrustBundle = &ateapi.TrustBundleDataSource{
					Names: []string{"egress-mitm.ate.dev"},
					Path:  "tb2.pem",
				}
			},
		),
		want: field.ErrorList{field.Invalid(dsPath.Index(0), nil, "").WithOrigin("union")},
	}, {
		name: "empty ActorMetadata items",
		obj: valid(
			withActorMetadata(),
			func(s *ateapi.SystemInfoVolumeSource) {
				s.DataSources[0].ActorMetadata.Items = nil
			},
		),
		want: field.ErrorList{field.Required(itemsPath, "")},
	}, {
		name: "duplicate projected field",
		obj: valid(
			withActorMetadata(),
			func(s *ateapi.SystemInfoVolumeSource) {
				s.DataSources[0].ActorMetadata.Items[1].Field = ateapi.ActorMetadataField_ACTOR_METADATA_FIELD_NAME
			},
		),
		want: field.ErrorList{field.Duplicate(itemsPath.Index(1), nil)},
	}, {
		name: "unspecified item field",
		obj: valid(
			withActorMetadata(),
			func(s *ateapi.SystemInfoVolumeSource) {
				s.DataSources[0].ActorMetadata.Items[0].Field = ateapi.ActorMetadataField_ACTOR_METADATA_FIELD_UNSPECIFIED
			},
		),
		want: field.ErrorList{field.Required(itemsPath.Index(0).Child("field"), "")},
	}, {
		name: "item field outside the enum",
		obj: valid(
			withActorMetadata(),
			func(s *ateapi.SystemInfoVolumeSource) {
				s.DataSources[0].ActorMetadata.Items[0].Field = ateapi.ActorMetadataField(99)
			},
		),
		want: field.ErrorList{field.Invalid(itemsPath.Index(0).Child("field"), nil, "").WithOrigin("maximum")},
	}, {
		name: "negative item field",
		obj: valid(
			withActorMetadata(),
			func(s *ateapi.SystemInfoVolumeSource) {
				s.DataSources[0].ActorMetadata.Items[0].Field = ateapi.ActorMetadataField(-1)
			},
		),
		want: field.ErrorList{field.Invalid(itemsPath.Index(0).Child("field"), nil, "").WithOrigin("minimum")},
	}, {
		name: "empty item path",
		obj: valid(
			withActorMetadata(),
			func(s *ateapi.SystemInfoVolumeSource) {
				s.DataSources[0].ActorMetadata.Items[0].Path = ""
			},
		),
		want: field.ErrorList{field.Required(itemsPath.Index(0).Child("path"), "")},
	}, {
		name: "item path too long",
		obj: valid(
			withActorMetadata(),
			func(s *ateapi.SystemInfoVolumeSource) {
				s.DataSources[0].ActorMetadata.Items[0].Path = strings.Repeat("p", 256)
			},
		),
		want: field.ErrorList{field.TooLong(itemsPath.Index(0).Child("path"), nil, 255).WithOrigin("maxLength")},
	}, {
		name: "item path traversal",
		obj: valid(
			withActorMetadata(),
			func(s *ateapi.SystemInfoVolumeSource) {
				s.DataSources[0].ActorMetadata.Items[0].Path = "../../traversal-escape"
			},
		),
		want: field.ErrorList{field.Invalid(itemsPath.Index(0).Child("path"), nil, "")},
	}, {
		name: "trust bundle path traversal",
		obj: valid(
			withTrustBundle(),
			func(s *ateapi.SystemInfoVolumeSource) {
				s.DataSources[0].TrustBundle.Path = "../escape"
			},
		),
		want: field.ErrorList{field.Invalid(dsPath.Index(0).Child("trust_bundle", "path"), nil, "")},
	}, {
		name: "duplicate path within items",
		obj: valid(
			withActorMetadata(),
			func(s *ateapi.SystemInfoVolumeSource) {
				s.DataSources[0].ActorMetadata.Items[1].Path = "actor-name"
			},
		),
		want: field.ErrorList{field.Duplicate(itemsPath.Index(1).Child("path"), nil)},
	}, {
		name: "duplicate path across data sources",
		obj: valid(
			withActorMetadata(),
			withTrustBundle(),
			func(s *ateapi.SystemInfoVolumeSource) {
				s.DataSources[1].TrustBundle.Path = "actor-uid"
			},
		),
		want: field.ErrorList{field.Duplicate(dsPath.Index(1).Child("trust_bundle", "path"), nil)},
	}, {
		name: "second actor_metadata entry",
		obj: valid(
			withActorMetadata(),
			withTrustBundle(),
			withSystemInfoActorMetadata(
				&ateapi.ActorMetadataItem{Field: ateapi.ActorMetadataField_ACTOR_METADATA_FIELD_NAME, Path: "actor-name-2"},
			),
		),
		want: field.ErrorList{field.Forbidden(dsPath.Index(2).Child("actor_metadata"), "")},
	}, {
		name: "nil data source",
		obj: valid(func(s *ateapi.SystemInfoVolumeSource) {
			s.DataSources = []*ateapi.SystemInfoDataSource{nil}
		}),
		want: field.ErrorList{field.Required(dsPath.Index(0), "")},
	}, {
		name: "nil actor_metadata item",
		obj:  valid(withSystemInfoActorMetadata(nil)),
		want: field.ErrorList{field.Required(itemsPath.Index(0), "")},
	}, {
		name: "too many actor_metadata items",
		obj:  valid(withSystemInfoActorMetadata(numberedActorMetadataItems(9)...)),
		want: field.ErrorList{field.TooMany(itemsPath, 9, 8).WithOrigin("maxItems")},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := MakeCreateOp(context.Background())
			matcher := field.ErrorMatcher{}.ByType().ByField().ByOrigin()
			matcher.Test(t, tt.want, Validate_SystemInfoVolumeSource(context.Background(), op, nil, tt.obj, nil))
		})
	}
}

func TestValidateTrustBundleDataSource(t *testing.T) {
	valid := func(mutate ...func(*ateapi.TrustBundleDataSource)) *ateapi.TrustBundleDataSource {
		tb := &ateapi.TrustBundleDataSource{
			Names: []string{"egress-mitm.ate.dev"},
			Path:  "trust-bundle.pem",
		}
		for _, m := range mutate {
			m(tb)
		}
		return tb
	}

	tests := []struct {
		name string
		obj  *ateapi.TrustBundleDataSource
		want field.ErrorList
	}{{
		name: "valid",
		obj:  valid(),
	}, {
		name: "valid: multiple names",
		obj: valid(func(tb *ateapi.TrustBundleDataSource) {
			tb.Names = []string{"egress-mitm.ate.dev", "system-roots.ate.dev"}
		}),
	}, {
		name: "no names",
		obj:  valid(func(tb *ateapi.TrustBundleDataSource) { tb.Names = nil }),
		want: field.ErrorList{field.Required(field.NewPath("names"), "")},
	}, {
		name: "too many names",
		obj: valid(func(tb *ateapi.TrustBundleDataSource) {
			tb.Names = []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}
		}),
		want: field.ErrorList{field.TooMany(field.NewPath("names"), 9, 8).WithOrigin("maxItems")},
	}, {
		name: "empty name",
		obj:  valid(func(tb *ateapi.TrustBundleDataSource) { tb.Names = []string{""} }),
		want: field.ErrorList{field.TooShort(field.NewPath("names").Index(0), "", 1).WithOrigin("minLength")},
	}, {
		name: "name too long",
		obj:  valid(func(tb *ateapi.TrustBundleDataSource) { tb.Names = []string{strings.Repeat("n", 254)} }),
		want: field.ErrorList{field.TooLong(field.NewPath("names").Index(0), nil, 253).WithOrigin("maxLength")},
	}, {
		name: "empty path",
		obj:  valid(func(tb *ateapi.TrustBundleDataSource) { tb.Path = "" }),
		want: field.ErrorList{field.Required(field.NewPath("path"), "")},
	}, {
		name: "path too long",
		obj:  valid(func(tb *ateapi.TrustBundleDataSource) { tb.Path = strings.Repeat("p", 256) }),
		want: field.ErrorList{field.TooLong(field.NewPath("path"), nil, 255).WithOrigin("maxLength")},
	}, {
		name: "path traversal",
		obj:  valid(func(tb *ateapi.TrustBundleDataSource) { tb.Path = "../escape" }),
		want: field.ErrorList{field.Invalid(field.NewPath("path"), nil, "")},
	}, {
		name: "duplicate names",
		obj: valid(func(tb *ateapi.TrustBundleDataSource) {
			tb.Names = []string{"egress-mitm.ate.dev", "egress-mitm.ate.dev"}
		}),
		want: field.ErrorList{field.Duplicate(field.NewPath("names").Index(1), nil)},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := MakeCreateOp(context.Background())
			matcher := field.ErrorMatcher{}.ByType().ByField().ByOrigin()
			matcher.Test(t, tt.want, Validate_TrustBundleDataSource(context.Background(), op, nil, tt.obj, nil))
		})
	}
}

// validGoldenSnapshotStatus returns a GoldenSnapshotStatus with no fields
// set, since all of them are optional. The withGoldenSnapshotStatus* modifiers
// set them.
func validGoldenSnapshotStatus(mutate ...func(*ateapi.GoldenSnapshotStatus)) *ateapi.GoldenSnapshotStatus {
	s := &ateapi.GoldenSnapshotStatus{}
	for _, m := range mutate {
		m(s)
	}
	return s
}

// withGoldenSnapshotStatusGoldenTag returns a modifier func (see
// validGoldenSnapshotStatus) which sets the golden_tag to a valid value.
func withGoldenSnapshotStatusGoldenTag() func(*ateapi.GoldenSnapshotStatus) {
	return func(s *ateapi.GoldenSnapshotStatus) {
		s.GoldenTag = &ateapi.ObjectRef{Atespace: "team-a", Name: "01234567-89ab-cdef-0123-456789abcdef"}
	}
}

// withGoldenSnapshotStatusSnapshotTime returns a modifier func (see
// validGoldenSnapshotStatus) which sets the snapshot_time to a valid value.
func withGoldenSnapshotStatusSnapshotTime() func(*ateapi.GoldenSnapshotStatus) {
	return func(s *ateapi.GoldenSnapshotStatus) { s.SnapshotTime = &timestamppb.Timestamp{Seconds: 867} }
}

func TestValidateGoldenSnapshotStatus(t *testing.T) {
	withGoldenTag := withGoldenSnapshotStatusGoldenTag
	withSnapshotTime := withGoldenSnapshotStatusSnapshotTime
	valid := validGoldenSnapshotStatus
	msgPath := field.NewPath("error_message")

	tests := []struct {
		name string
		obj  *ateapi.GoldenSnapshotStatus
		want field.ErrorList
	}{
		{
			name: "valid",
			obj:  valid(),
		},
		{
			name: "valid with golden_tag and snapshot_time",
			obj:  valid(withGoldenTag(), withSnapshotTime()),
		},
		{
			name: "valid failed",
			obj: valid(
				withSnapshotTime(),
				func(s *ateapi.GoldenSnapshotStatus) {
					s.ErrorMessage = "GoldenActorCrashed: golden actor crashed before its snapshot was taken"
				},
			),
		},
		{
			name: "error_message at the bound",
			obj: valid(func(s *ateapi.GoldenSnapshotStatus) {
				s.ErrorMessage = strings.Repeat("x", 4096)
			}),
		},
		{
			name: "error_message too long",
			obj: valid(func(s *ateapi.GoldenSnapshotStatus) {
				s.ErrorMessage = strings.Repeat("x", 4097)
			}),
			want: field.ErrorList{field.TooLong(msgPath, nil, 4096).WithOrigin("maxLength")},
		},
		{
			name: "golden_tag missing atespace",
			obj: valid(
				withGoldenTag(),
				func(s *ateapi.GoldenSnapshotStatus) {
					s.GoldenTag.Atespace = ""
				},
			),
			want: field.ErrorList{field.Required(field.NewPath("golden_tag", "atespace"), "")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := MakeCreateOp(context.Background())
			assertValidateErr(t, Validate_GoldenSnapshotStatus(context.Background(), op, nil, tt.obj, nil), tt.want)
		})
	}
}

func TestValidateGoldenSnapshotStatusUpdate(t *testing.T) {
	withGoldenTag := withGoldenSnapshotStatusGoldenTag
	withSnapshotTime := withGoldenSnapshotStatusSnapshotTime
	valid := validGoldenSnapshotStatus

	tests := []struct {
		name   string
		oldObj *ateapi.GoldenSnapshotStatus // should always be valid
		newObj *ateapi.GoldenSnapshotStatus
		want   field.ErrorList
	}{
		{
			name:   "unchanged",
			oldObj: valid(withGoldenTag(), withSnapshotTime()),
			newObj: valid(withGoldenTag(), withSnapshotTime()),
		},
		{
			// The reconciler restarts the warmup clock on every resume, so the
			// deadline must stay mutable.
			name:   "snapshot_time changed",
			oldObj: valid(withSnapshotTime()),
			newObj: valid(
				withSnapshotTime(),
				func(s *ateapi.GoldenSnapshotStatus) {
					s.SnapshotTime = &timestamppb.Timestamp{Seconds: 5309}
				},
			),
		},
		{
			name:   "snapshot_time set",
			oldObj: valid(),
			newObj: valid(withSnapshotTime()),
		},
		{
			name:   "error_message set",
			oldObj: valid(),
			newObj: valid(func(s *ateapi.GoldenSnapshotStatus) {
				s.ErrorMessage = "GoldenActorCrashed: golden actor crashed before its snapshot was taken"
			}),
		},
		{
			name:   "error_message set too long",
			oldObj: valid(),
			newObj: valid(func(s *ateapi.GoldenSnapshotStatus) {
				s.ErrorMessage = strings.Repeat("x", 4097)
			}),
			want: field.ErrorList{field.TooLong(field.NewPath("error_message"), nil, 4096).WithOrigin("maxLength")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := MakeUpdateOp(context.Background())
			assertValidateErr(
				t,
				Validate_GoldenSnapshotStatus(context.Background(), op, nil, tt.newObj, tt.oldObj),
				tt.want,
			)
		})
	}
}
