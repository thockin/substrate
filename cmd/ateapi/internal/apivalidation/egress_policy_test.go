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
	"go.ate.dev/substrate/cmd/ateapi/internal/defaults"
	"go.ate.dev/substrate/internal/resources"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func validEgressPolicy(mutate ...func(*ateapi.EgressPolicy)) *ateapi.EgressPolicy {
	p := &ateapi.EgressPolicy{
		Metadata: &ateapi.ResourceMetadata{Atespace: testAtespace, Name: "default"},
	}
	for _, m := range mutate {
		m(p)
	}
	return p
}

// withEgressPolicyMetadata returns a modifier func (see validEgressPolicy)
// which applies mods to the policy's metadata.
func withEgressPolicyMetadata(mods ...func(*ateapi.ResourceMetadata)) func(*ateapi.EgressPolicy) {
	return func(p *ateapi.EgressPolicy) {
		for _, m := range mods {
			m(p.Metadata)
		}
	}
}

// withEgressPolicyHTTPRule returns a modifier func (see validEgressPolicy)
// which appends an HTTP rule for hosts, with no ports or effects, and applies
// mods to it.
func withEgressPolicyHTTPRule(hosts []string, mods ...func(*ateapi.HttpRule)) func(*ateapi.EgressPolicy) {
	return func(p *ateapi.EgressPolicy) {
		r := &ateapi.HttpRule{Hostnames: hosts}
		for _, m := range mods {
			m(r)
		}
		p.Rules = append(p.Rules, &ateapi.EgressRule{Http: r})
	}
}

// withHTTPRulePorts returns a modifier func (see withEgressPolicyHTTPRule)
// which sets the rule's ports.
func withHTTPRulePorts(p *ateapi.Ports) func(*ateapi.HttpRule) {
	return func(r *ateapi.HttpRule) { r.Ports = p }
}

// withHTTPRuleEffects returns a modifier func (see withEgressPolicyHTTPRule)
// which sets the rule's effects to replace headers.
func withHTTPRuleEffects(headers ...*ateapi.CredentialHeader) func(*ateapi.HttpRule) {
	return func(r *ateapi.HttpRule) {
		r.Effects = &ateapi.HttpRuleEffects{ReplaceHeaders: headers}
	}
}

// withEgressPolicyHTTPSRule returns a modifier func (see validEgressPolicy)
// which appends an HTTPS rule for hosts, with no ports or effects, and
// applies mods to it.
func withEgressPolicyHTTPSRule(hosts []string, mods ...func(*ateapi.HttpsRule)) func(*ateapi.EgressPolicy) {
	return func(p *ateapi.EgressPolicy) {
		r := &ateapi.HttpsRule{Hostnames: hosts}
		for _, m := range mods {
			m(r)
		}
		p.Rules = append(p.Rules, &ateapi.EgressRule{Https: r})
	}
}

// withHTTPSRulePorts returns a modifier func (see withEgressPolicyHTTPSRule)
// which sets the rule's ports.
func withHTTPSRulePorts(p *ateapi.Ports) func(*ateapi.HttpsRule) {
	return func(r *ateapi.HttpsRule) { r.Ports = p }
}

// withHTTPSRuleEffects returns a modifier func (see
// withEgressPolicyHTTPSRule) which sets the rule's effects to replace headers.
func withHTTPSRuleEffects(headers ...*ateapi.CredentialHeader) func(*ateapi.HttpsRule) {
	return func(r *ateapi.HttpsRule) {
		r.Effects = &ateapi.HttpRuleEffects{ReplaceHeaders: headers}
	}
}

// withEgressPolicyTLSPassthroughRule returns a modifier func (see
// validEgressPolicy) which appends a TLS passthrough rule for hosts on ports.
func withEgressPolicyTLSPassthroughRule(hosts []string, p *ateapi.Ports) func(*ateapi.EgressPolicy) {
	return func(policy *ateapi.EgressPolicy) {
		policy.Rules = append(policy.Rules, &ateapi.EgressRule{
			TlsPassthrough: &ateapi.TlsPassthroughRule{Hostnames: hosts, Ports: p},
		})
	}
}

// credentialHeader is header replaced with prefix followed by the credential
// at credentialURI.
func credentialHeader(header, prefix, credentialURI string) *ateapi.CredentialHeader {
	return &ateapi.CredentialHeader{Header: header, Prefix: prefix, CredentialUri: credentialURI}
}

// actorJWTHeader is header replaced with prefix followed by an actor JWT for
// audiences that lasts expirationSeconds.
func actorJWTHeader(header, prefix string, expirationSeconds int64, audiences ...string) *ateapi.CredentialHeader {
	return &ateapi.CredentialHeader{
		Header:   header,
		Prefix:   prefix,
		ActorJwt: &ateapi.ActorJwtSource{Audiences: audiences, ExpirationSeconds: expirationSeconds},
	}
}

func TestValidateCreateActorEgressPolicyRequest(t *testing.T) {
	withActor := withRequestActor[*ateapi.CreateActorEgressPolicyRequest]
	withAtespace := withObjectRefAtespace
	withName := withObjectRefName
	withPolicy := withRequestEgressPolicy[*ateapi.CreateActorEgressPolicyRequest]
	withPolicyMetadata := withEgressPolicyMetadata
	withHTTPRule := withEgressPolicyHTTPRule
	validReq := func(mods ...func(*ateapi.CreateActorEgressPolicyRequest)) *ateapi.CreateActorEgressPolicyRequest {
		r := &ateapi.CreateActorEgressPolicyRequest{
			Actor:        &ateapi.ObjectRef{Atespace: testAtespace, Name: "actor"},
			EgressPolicy: validEgressPolicy(),
		}
		for _, m := range mods {
			m(r)
		}
		return r
	}

	tests := []struct {
		name string
		req  *ateapi.CreateActorEgressPolicyRequest
		want field.ErrorList
	}{{
		name: "valid",
		req:  validReq(),
	}, {
		name: "valid with a rule",
		req:  validReq(withPolicy(withHTTPRule(hostnames("api.example.com")))),
	}, {
		name: "missing actor",
		req:  validReq(func(r *ateapi.CreateActorEgressPolicyRequest) { r.Actor = nil }),
		want: field.ErrorList{field.Required(field.NewPath("actor"), "")},
	}, {
		name: "missing actor atespace",
		req:  validReq(withActor(withAtespace(""))),
		want: field.ErrorList{field.Required(field.NewPath("actor", "atespace"), "")},
	}, {
		name: "invalid actor atespace",
		req:  validReq(withActor(withAtespace("invalid value"))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("actor", "atespace"), nil, "").WithOrigin("format=k8s-short-name"),
			field.Invalid(field.NewPath("egress_policy", "metadata", "atespace"), nil, ""),
		},
	}, {
		name: "missing actor name",
		req:  validReq(withActor(withName(""))),
		want: field.ErrorList{field.Required(field.NewPath("actor", "name"), "")},
	}, {
		name: "invalid actor name",
		req:  validReq(withActor(withName("invalid value"))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("actor", "name"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		name: "missing policy",
		req:  validReq(func(r *ateapi.CreateActorEgressPolicyRequest) { r.EgressPolicy = nil }),
		want: field.ErrorList{field.Required(field.NewPath("egress_policy"), "")},
	}, {
		name: "missing metadata",
		req:  validReq(func(r *ateapi.CreateActorEgressPolicyRequest) { r.EgressPolicy.Metadata = nil }),
		want: field.ErrorList{field.Required(field.NewPath("egress_policy", "metadata"), "")},
	}, {
		name: "missing policy atespace",
		req:  validReq(withPolicy(withPolicyMetadata(withMetadataAtespace("")))),
		want: field.ErrorList{field.Required(field.NewPath("egress_policy", "metadata", "atespace"), "")},
	}, {
		name: "missing default name",
		req:  validReq(withPolicy(withPolicyMetadata(withMetadataName("")))),
		want: field.ErrorList{field.Required(field.NewPath("egress_policy", "metadata", "name"), "")},
	}, {
		name: "wrong policy name",
		req:  validReq(withPolicy(withPolicyMetadata(withMetadataName("other")))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "metadata", "name"), "other", `must be "default"`).WithOrigin("custom=default"),
		},
	}, {
		name: "mismatched policy atespace",
		req:  validReq(withPolicy(withPolicyMetadata(withMetadataAtespace("other")))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "metadata", "atespace"), "other", "must match actor.atespace"),
		},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertValidateErr(t, ValidateCreateActorEgressPolicyRequest(context.Background(), tc.req), tc.want)
		})
	}
}

func TestValidateGetActorEgressPolicyRequest(t *testing.T) {
	withActor := withRequestActor[*ateapi.GetActorEgressPolicyRequest]
	withAtespace := withObjectRefAtespace
	withName := withObjectRefName
	validReq := func(mods ...func(*ateapi.GetActorEgressPolicyRequest)) *ateapi.GetActorEgressPolicyRequest {
		r := &ateapi.GetActorEgressPolicyRequest{
			Actor: &ateapi.ObjectRef{Atespace: testAtespace, Name: "actor"},
			Name:  resources.SingletonName,
		}
		for _, m := range mods {
			m(r)
		}
		return r
	}
	tests := []struct {
		name string
		req  *ateapi.GetActorEgressPolicyRequest
		want field.ErrorList
	}{{
		name: "valid",
		req:  validReq(),
	}, {
		name: "missing actor",
		req:  validReq(func(r *ateapi.GetActorEgressPolicyRequest) { r.Actor = nil }),
		want: field.ErrorList{field.Required(field.NewPath("actor"), "")},
	}, {
		name: "missing atespace",
		req:  validReq(withActor(withAtespace(""))),
		want: field.ErrorList{field.Required(field.NewPath("actor", "atespace"), "")},
	}, {
		name: "invalid atespace",
		req:  validReq(withActor(withAtespace("invalid value"))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("actor", "atespace"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		name: "missing name",
		req:  validReq(withActor(withName(""))),
		want: field.ErrorList{field.Required(field.NewPath("actor", "name"), "")},
	}, {
		name: "invalid name",
		req:  validReq(withActor(withName("invalid value"))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("actor", "name"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		name: "missing singleton name",
		req:  validReq(func(r *ateapi.GetActorEgressPolicyRequest) { r.Name = "" }),
		want: field.ErrorList{field.Required(field.NewPath("name"), "")},
	}, {
		name: "wrong singleton name",
		req:  validReq(func(r *ateapi.GetActorEgressPolicyRequest) { r.Name = "other" }),
		want: field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("custom=default")},
	}, {
		name: "invalid singleton name",
		req:  validReq(func(r *ateapi.GetActorEgressPolicyRequest) { r.Name = "Not A Name" }),
		want: field.ErrorList{
			field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name"),
			field.Invalid(field.NewPath("name"), nil, "").WithOrigin("custom=default"),
		},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertValidateErr(t, ValidateGetActorEgressPolicyRequest(context.Background(), tc.req), tc.want)
		})
	}
}

func TestValidateUpdateActorEgressPolicyRequest(t *testing.T) {
	withActor := withRequestActor[*ateapi.UpdateActorEgressPolicyRequest]
	withAtespace := withObjectRefAtespace
	withPolicy := withRequestEgressPolicy[*ateapi.UpdateActorEgressPolicyRequest]
	withPolicyMetadata := withEgressPolicyMetadata
	withHTTPRule := withEgressPolicyHTTPRule
	validReq := func(mods ...func(*ateapi.UpdateActorEgressPolicyRequest)) *ateapi.UpdateActorEgressPolicyRequest {
		r := &ateapi.UpdateActorEgressPolicyRequest{
			Actor:        &ateapi.ObjectRef{Atespace: testAtespace, Name: "actor"},
			EgressPolicy: validEgressPolicy(),
		}
		for _, m := range mods {
			m(r)
		}
		return r
	}
	tests := []struct {
		name string
		req  *ateapi.UpdateActorEgressPolicyRequest
		want field.ErrorList
	}{{
		name: "valid",
		req:  validReq(),
	}, {
		name: "valid with a rule",
		req:  validReq(withPolicy(withHTTPRule(hostnames("api.example.com")))),
	}, {
		name: "missing actor",
		req:  validReq(func(r *ateapi.UpdateActorEgressPolicyRequest) { r.Actor = nil }),
		want: field.ErrorList{field.Required(field.NewPath("actor"), "")},
	}, {
		name: "missing actor atespace",
		req:  validReq(withActor(withAtespace(""))),
		want: field.ErrorList{field.Required(field.NewPath("actor", "atespace"), "")},
	}, {
		name: "missing policy",
		req:  validReq(func(r *ateapi.UpdateActorEgressPolicyRequest) { r.EgressPolicy = nil }),
		want: field.ErrorList{field.Required(field.NewPath("egress_policy"), "")},
	}, {
		name: "mismatched atespace",
		req:  validReq(withPolicy(withPolicyMetadata(withMetadataAtespace("other")))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "metadata", "atespace"), "other", "must match actor.atespace"),
		},
	}, {
		name: "invalid rule",
		req:  validReq(withPolicy(withHTTPRule(hostnames()))),
		want: field.ErrorList{
			field.Required(field.NewPath("egress_policy", "rules").Index(0).Child("http", "hostnames"), ""),
		},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertValidateErr(t, ValidateUpdateActorEgressPolicyRequest(context.Background(), tc.req), tc.want)
		})
	}
}

func TestValidateEgressPolicyUpdate(t *testing.T) {
	withHTTPRule := withEgressPolicyHTTPRule
	withMetadata := withEgressPolicyMetadata
	withUID := withMetadataUID
	tests := []struct {
		name   string
		oldVal *ateapi.EgressPolicy
		newVal *ateapi.EgressPolicy
		want   field.ErrorList
	}{{
		name:   "unchanged",
		oldVal: validEgressPolicy(withMetadata(withUID())),
		newVal: validEgressPolicy(withMetadata(withUID())),
	}, {
		name:   "rules changed",
		oldVal: validEgressPolicy(withMetadata(withUID()), withHTTPRule(hostnames("api.example.com"))),
		newVal: validEgressPolicy(withMetadata(withUID()), withHTTPRule(hostnames("other.example.com"))),
	}, {
		name:   "invalid rule",
		oldVal: validEgressPolicy(withMetadata(withUID())),
		newVal: validEgressPolicy(withMetadata(withUID()), withHTTPRule(hostnames())),
		want: field.ErrorList{
			field.Required(field.NewPath("egress_policy", "rules").Index(0).Child("http", "hostnames"), ""),
		},
	}, {
		name:   "metadata.atespace changed",
		oldVal: validEgressPolicy(withMetadata(withUID())),
		newVal: validEgressPolicy(
			withMetadata(withUID()),
			withMetadata(func(m *ateapi.ResourceMetadata) { m.Atespace = "other" }),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "metadata", "atespace"), nil, "").WithOrigin("immutable"),
		},
	}, {
		name:   "metadata.uid changed",
		oldVal: validEgressPolicy(withMetadata(withUID())),
		newVal: validEgressPolicy(
			withMetadata(withUID()),
			withMetadata(func(m *ateapi.ResourceMetadata) { m.Uid = "fedcba98-7654-3210-fedc-ba9876543210" }),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "metadata", "uid"), nil, "").WithOrigin("immutable"),
		},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertValidateErr(
				t,
				ValidateEgressPolicyUpdate(context.Background(), field.NewPath("egress_policy"), tc.newVal, tc.oldVal),
				tc.want,
			)
		})
	}
}

func TestValidateDeleteActorEgressPolicyRequest(t *testing.T) {
	withActor := withRequestActor[*ateapi.DeleteActorEgressPolicyRequest]
	withAtespace := withObjectRefAtespace
	withName := withObjectRefName
	validReq := func(mods ...func(*ateapi.DeleteActorEgressPolicyRequest)) *ateapi.DeleteActorEgressPolicyRequest {
		r := &ateapi.DeleteActorEgressPolicyRequest{
			Actor: &ateapi.ObjectRef{Atespace: testAtespace, Name: "actor"},
			Name:  resources.SingletonName,
		}
		for _, m := range mods {
			m(r)
		}
		return r
	}
	tests := []struct {
		name string
		req  *ateapi.DeleteActorEgressPolicyRequest
		want field.ErrorList
	}{{
		name: "valid",
		req:  validReq(),
	}, {
		name: "missing actor",
		req:  validReq(func(r *ateapi.DeleteActorEgressPolicyRequest) { r.Actor = nil }),
		want: field.ErrorList{field.Required(field.NewPath("actor"), "")},
	}, {
		name: "missing atespace",
		req:  validReq(withActor(withAtespace(""))),
		want: field.ErrorList{field.Required(field.NewPath("actor", "atespace"), "")},
	}, {
		name: "invalid atespace",
		req:  validReq(withActor(withAtespace("invalid value"))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("actor", "atespace"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		name: "missing name",
		req:  validReq(withActor(withName(""))),
		want: field.ErrorList{field.Required(field.NewPath("actor", "name"), "")},
	}, {
		name: "invalid name",
		req:  validReq(withActor(withName("invalid value"))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("actor", "name"), nil, "").WithOrigin("format=k8s-short-name"),
		},
	}, {
		name: "missing singleton name",
		req:  validReq(func(r *ateapi.DeleteActorEgressPolicyRequest) { r.Name = "" }),
		want: field.ErrorList{field.Required(field.NewPath("name"), "")},
	}, {
		name: "wrong singleton name",
		req:  validReq(func(r *ateapi.DeleteActorEgressPolicyRequest) { r.Name = "other" }),
		want: field.ErrorList{field.Invalid(field.NewPath("name"), nil, "").WithOrigin("custom=default")},
	}, {
		name: "invalid singleton name",
		req:  validReq(func(r *ateapi.DeleteActorEgressPolicyRequest) { r.Name = "Not A Name" }),
		want: field.ErrorList{
			field.Invalid(field.NewPath("name"), nil, "").WithOrigin("format=k8s-short-name"),
			field.Invalid(field.NewPath("name"), nil, "").WithOrigin("custom=default"),
		},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertValidateErr(t, ValidateDeleteActorEgressPolicyRequest(context.Background(), tc.req), tc.want)
		})
	}
}

func TestValidateEgressPolicyRules(t *testing.T) {
	withHTTPRule := withEgressPolicyHTTPRule
	withPorts := withHTTPRulePorts
	withEffects := withHTTPRuleEffects
	withHTTPSRule := withEgressPolicyHTTPSRule
	withHTTPSPorts := withHTTPSRulePorts
	withHTTPSEffects := withHTTPSRuleEffects
	withTLSPassthroughRule := withEgressPolicyTLSPassthroughRule
	const badPattern = `must be a DNS hostname, optionally with a complete leftmost-label wildcard, or "*"`
	// authHeader replaces Authorization with a Kubernetes secret.
	authHeader := func() *ateapi.CredentialHeader {
		return credentialHeader("Authorization", "Bearer ", "ate-secret://kubernetes.io/provider/ns/name")
	}
	// numberedHostnames returns n distinct hostnames.
	numberedHostnames := func(n int) []string {
		var hosts []string
		for i := range n {
			hosts = append(hosts, fmt.Sprintf("api%d.example.com", i))
		}
		return hosts
	}
	// numberedHeaders returns n distinct headers, each replaced with a secret.
	numberedHeaders := func(n int) []*ateapi.CredentialHeader {
		var headers []*ateapi.CredentialHeader
		for i := range n {
			headers = append(headers, credentialHeader(
				fmt.Sprintf("X-Header-%d", i),
				"",
				"ate-secret://example.com/provider/secret",
			))
		}
		return headers
	}

	tests := []struct {
		name   string
		policy *ateapi.EgressPolicy
		want   field.ErrorList
	}{{
		name:   "valid with no rules",
		policy: validEgressPolicy(),
	}, {
		name:   "valid http rule",
		policy: validEgressPolicy(withHTTPRule(hostnames("api.example.com"))),
	}, {
		name:   "valid http rule with effects",
		policy: validEgressPolicy(withHTTPRule(hostnames("api.example.com"), withEffects(authHeader()))),
	}, {
		name: "many rules",
		policy: validEgressPolicy(func(p *ateapi.EgressPolicy) {
			for _, host := range numberedHostnames(256) {
				withHTTPRule(hostnames(host))(p)
			}
		}),
	}, {
		name: "too many rules",
		policy: validEgressPolicy(func(p *ateapi.EgressPolicy) {
			for _, host := range numberedHostnames(257) {
				withHTTPRule(hostnames(host))(p)
			}
		}),
		want: field.ErrorList{
			field.TooMany(field.NewPath("egress_policy", "rules"), 257, 256).WithOrigin("maxItems"),
		},
	}, {
		name:   "nil rule",
		policy: validEgressPolicy(func(p *ateapi.EgressPolicy) { p.Rules = []*ateapi.EgressRule{nil} }),
		want:   field.ErrorList{field.Required(field.NewPath("egress_policy", "rules").Index(0), "")},
	}, {
		name:   "no protocol",
		policy: validEgressPolicy(func(p *ateapi.EgressPolicy) { p.Rules = []*ateapi.EgressRule{{}} }),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0), nil, "one of").WithOrigin("union"),
		},
	}, {
		name: "two protocols",
		policy: validEgressPolicy(
			withHTTPRule(hostnames("api.example.com")),
			func(p *ateapi.EgressPolicy) {
				p.Rules[0].Https = &ateapi.HttpsRule{Hostnames: hostnames("api.example.com")}
			},
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0), nil, "one of").WithOrigin("union"),
		},
	}, {
		name: "https rule",
		policy: validEgressPolicy(
			withHTTPSRule(hostnames("api.example.com", "*.example.org"), withHTTPSPorts(ports(443, 8443))),
		),
	}, {
		name:   "tls passthrough rule",
		policy: validEgressPolicy(withTLSPassthroughRule(hostnames("api.example.com", "*"), ports(443))),
	}, {
		name:   "tls passthrough rule without ports",
		policy: validEgressPolicy(withTLSPassthroughRule(hostnames("api.example.com"), nil)),
		want: field.ErrorList{
			field.Required(field.NewPath("egress_policy", "rules").Index(0).Child("tls_passthrough", "ports"), ""),
		},
	}, {
		name:   "empty pattern list",
		policy: validEgressPolicy(withHTTPRule(hostnames())),
		want: field.ErrorList{
			field.Required(field.NewPath("egress_policy", "rules").Index(0).Child("http", "hostnames"), ""),
		},
	}, {
		name:   "long pattern list",
		policy: validEgressPolicy(withHTTPRule(numberedHostnames(256))),
	}, {
		name:   "too-long pattern list",
		policy: validEgressPolicy(withHTTPRule(numberedHostnames(257))),
		want: field.ErrorList{
			field.TooMany(field.NewPath("egress_policy", "rules").Index(0).Child("http", "hostnames"), 257, 256).WithOrigin("maxItems"),
		},
	}, {
		name:   "missing pattern",
		policy: validEgressPolicy(withHTTPRule(hostnames(""))),
		want: field.ErrorList{
			field.Required(field.NewPath("egress_policy", "rules").Index(0).Child("http", "hostnames").Index(0), ""),
		},
	}, {
		name:   "duplicate pattern",
		policy: validEgressPolicy(withHTTPRule(hostnames("api.example.com", "api.example.com"))),
		want: field.ErrorList{
			field.Duplicate(field.NewPath("egress_policy", "rules").Index(0).Child("http", "hostnames").Index(1), "api.example.com"),
		},
	}, {
		name:   "invalid pattern",
		policy: validEgressPolicy(withHTTPRule(hostnames("https://example.com"))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "hostnames").Index(0), "https://example.com", badPattern),
		},
	}, {
		name:   "uppercase pattern",
		policy: validEgressPolicy(withHTTPRule(hostnames("API.EXAMPLE.COM"))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "hostnames").Index(0), "API.EXAMPLE.COM", badPattern),
		},
	}, {
		name:   "pattern with trailing dot",
		policy: validEgressPolicy(withHTTPRule(hostnames("api.example.com."))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "hostnames").Index(0), "api.example.com.", badPattern),
		},
	}, {
		name:   "IP literal pattern",
		policy: validEgressPolicy(withHTTPRule(hostnames("192.0.2.1"))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "hostnames").Index(0), "192.0.2.1", badPattern),
		},
	}, {
		name:   "pattern with port",
		policy: validEgressPolicy(withHTTPRule(hostnames("example.com:443"))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "hostnames").Index(0), "example.com:443", badPattern),
		},
	}, {
		name:   "wildcard without effects",
		policy: validEgressPolicy(withHTTPRule(hostnames("*.example.com"))),
	}, {
		name:   "wildcard with effects",
		policy: validEgressPolicy(withHTTPRule(hostnames("*.example.com"), withEffects(authHeader()))),
	}, {
		name:   "star pattern with effects",
		policy: validEgressPolicy(withHTTPRule(hostnames("*"), withEffects(authHeader()))),
	}, {
		name:   "invalid wildcard",
		policy: validEgressPolicy(withHTTPRule(hostnames("api.*.example.com"))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "hostnames").Index(0), "api.*.example.com", badPattern),
		},
	}, {
		name:   "https rule without hostnames",
		policy: validEgressPolicy(withHTTPSRule(hostnames())),
		want: field.ErrorList{
			field.Required(field.NewPath("egress_policy", "rules").Index(0).Child("https", "hostnames"), ""),
		},
	}, {
		name:   "https rule with too many hostnames",
		policy: validEgressPolicy(withHTTPSRule(numberedHostnames(257))),
		want: field.ErrorList{
			field.TooMany(field.NewPath("egress_policy", "rules").Index(0).Child("https", "hostnames"), 257, 256).WithOrigin("maxItems"),
		},
	}, {
		name:   "https rule with a duplicate hostname",
		policy: validEgressPolicy(withHTTPSRule(hostnames("api.example.com", "api.example.com"))),
		want: field.ErrorList{
			field.Duplicate(field.NewPath("egress_policy", "rules").Index(0).Child("https", "hostnames").Index(1), nil),
		},
	}, {
		name:   "https rule with an invalid hostname",
		policy: validEgressPolicy(withHTTPSRule(hostnames("https://example.com"))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("https", "hostnames").Index(0), "https://example.com", badPattern),
		},
	}, {
		name:   "tls passthrough rule without hostnames",
		policy: validEgressPolicy(withTLSPassthroughRule(hostnames(), ports(443))),
		want: field.ErrorList{
			field.Required(field.NewPath("egress_policy", "rules").Index(0).Child("tls_passthrough", "hostnames"), ""),
		},
	}, {
		name:   "tls passthrough rule with too many hostnames",
		policy: validEgressPolicy(withTLSPassthroughRule(numberedHostnames(257), ports(443))),
		want: field.ErrorList{
			field.TooMany(field.NewPath("egress_policy", "rules").Index(0).Child("tls_passthrough", "hostnames"), 257, 256).WithOrigin("maxItems"),
		},
	}, {
		name:   "tls passthrough rule with a duplicate hostname",
		policy: validEgressPolicy(withTLSPassthroughRule(hostnames("api.example.com", "api.example.com"), ports(443))),
		want: field.ErrorList{
			field.Duplicate(field.NewPath("egress_policy", "rules").Index(0).Child("tls_passthrough", "hostnames").Index(1), nil),
		},
	}, {
		name:   "invalid sni pattern",
		policy: validEgressPolicy(withTLSPassthroughRule(hostnames("*example.com"), ports(443))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("tls_passthrough", "hostnames").Index(0), "*example.com", badPattern),
		},
	}, {
		name:   "ports",
		policy: validEgressPolicy(withHTTPRule(hostnames("api.example.com"), withPorts(ports(80, 8080)))),
	}, {
		name:   "any port",
		policy: validEgressPolicy(withHTTPRule(hostnames("api.example.com"), withPorts(allPorts()))),
	}, {
		name: "all and numbers together",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withPorts(&ateapi.Ports{All: &ateapi.AllPorts{}, Numbers: []int32{80}}),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "ports"), nil, "one of").WithOrigin("union"),
		},
	}, {
		name:   "empty ports",
		policy: validEgressPolicy(withHTTPRule(hostnames("api.example.com"), withPorts(&ateapi.Ports{}))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "ports"), nil, "one of").WithOrigin("union"),
		},
	}, {
		name: "empty numbers",
		policy: validEgressPolicy(
			withHTTPRule(hostnames("api.example.com"), withPorts(&ateapi.Ports{Numbers: []int32{}})),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "ports"), nil, "one of").WithOrigin("union"),
		},
	}, {
		name:   "port zero",
		policy: validEgressPolicy(withHTTPRule(hostnames("api.example.com"), withPorts(ports(0)))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "ports", "numbers").Index(0), int32(0), "").WithOrigin("minimum"),
		},
	}, {
		name:   "port out of range",
		policy: validEgressPolicy(withHTTPRule(hostnames("api.example.com"), withPorts(ports(65536)))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "ports", "numbers").Index(0), int32(65536), "").WithOrigin("maximum"),
		},
	}, {
		name:   "duplicate port",
		policy: validEgressPolicy(withHTTPRule(hostnames("api.example.com"), withPorts(ports(80, 80)))),
		want: field.ErrorList{
			field.Duplicate(field.NewPath("egress_policy", "rules").Index(0).Child("http", "ports", "numbers").Index(1), int32(80)),
		},
	}, {
		name: "too many ports",
		policy: validEgressPolicy(func(p *ateapi.EgressPolicy) {
			var many []int32
			for i := range 17 {
				many = append(many, int32(8000+i))
			}
			withHTTPRule(hostnames("api.example.com"), withPorts(ports(many...)))(p)
		}),
		want: field.ErrorList{
			field.TooMany(field.NewPath("egress_policy", "rules").Index(0).Child("http", "ports", "numbers"), 17, 16).WithOrigin("maxItems"),
		},
	}, {
		name:   "negative https port",
		policy: validEgressPolicy(withHTTPSRule(hostnames("api.example.com"), withHTTPSPorts(ports(-1)))),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("https", "ports", "numbers").Index(0), int32(-1), "").WithOrigin("minimum"),
		},
	}, {
		name:   "tls passthrough with empty ports",
		policy: validEgressPolicy(withTLSPassthroughRule(hostnames("*"), &ateapi.Ports{})),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("tls_passthrough", "ports"), nil, "one of").WithOrigin("union"),
		},
	}, {
		name: "same pattern on different ports",
		policy: validEgressPolicy(
			withHTTPRule(hostnames("api.example.com")),
			withHTTPRule(hostnames("api.example.com"), withPorts(ports(8080))),
		),
	}, {
		name: "same pattern on the default port ties",
		policy: validEgressPolicy(
			withHTTPRule(hostnames("api.example.com")),
			withHTTPRule(hostnames("api.example.com")),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(1).Child("http", "hostnames").Index(0), "api.example.com", `ties with egress_policy.rules[0].http.hostnames[0] on port 80`),
		},
	}, {
		name: "http and https on their default ports do not tie",
		policy: validEgressPolicy(
			withHTTPRule(hostnames("api.example.com")),
			withHTTPSRule(hostnames("api.example.com")),
		),
	}, {
		name: "http and https on any port tie",
		policy: validEgressPolicy(
			withHTTPRule(hostnames("api.example.com"), withPorts(allPorts())),
			withHTTPSRule(hostnames("api.example.com"), withHTTPSPorts(allPorts())),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(1).Child("https", "hostnames").Index(0), "api.example.com", `ties with egress_policy.rules[0].http.hostnames[0] on every port`),
		},
	}, {
		name: "http and tls passthrough on every port tie",
		policy: validEgressPolicy(
			withHTTPRule(hostnames("*"), withPorts(allPorts())),
			withTLSPassthroughRule(hostnames("*"), allPorts()),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(1).Child("tls_passthrough", "hostnames").Index(0), "*", `ties with egress_policy.rules[0].http.hostnames[0] on every port`),
		},
	}, {
		name: "http on its default port and tls passthrough on every port",
		policy: validEgressPolicy(
			withHTTPRule(hostnames("*")),
			withTLSPassthroughRule(hostnames("*"), allPorts()),
		),
	}, {
		name: "https and tls passthrough on the same port tie",
		policy: validEgressPolicy(
			withHTTPRule(hostnames("api.example.com")),
			withHTTPSRule(hostnames("*.example.com")),
			withTLSPassthroughRule(hostnames("*.example.com"), ports(443)),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(2).Child("tls_passthrough", "hostnames").Index(0), "*.example.com", `ties with egress_policy.rules[1].https.hostnames[0] on port 443`),
		},
	}, {
		name: "two https rules on the same port tie",
		policy: validEgressPolicy(
			withHTTPSRule(hostnames("api.example.com")),
			withHTTPSRule(hostnames("api.example.com")),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(1).Child("https", "hostnames").Index(0), "api.example.com", `ties with egress_policy.rules[0].https.hostnames[0] on port 443`),
		},
	}, {
		name: "two tls passthrough rules on the same port tie",
		policy: validEgressPolicy(
			withTLSPassthroughRule(hostnames("api.example.com"), ports(443)),
			withTLSPassthroughRule(hostnames("api.example.com"), ports(443)),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(1).Child("tls_passthrough", "hostnames").Index(0), "api.example.com", `ties with egress_policy.rules[0].tls_passthrough.hostnames[0] on port 443`),
		},
	}, {
		name: "rules that share one of their ports tie on it",
		policy: validEgressPolicy(
			withHTTPRule(hostnames("api.example.com"), withPorts(ports(80, 8080))),
			withHTTPRule(hostnames("api.example.com"), withPorts(ports(8080))),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(1).Child("http", "hostnames").Index(0), "api.example.com", `ties with egress_policy.rules[0].http.hostnames[0] on port 8080`),
		},
	}, {
		/*
			// FIXME: ValidateCustom_EgressPolicy_Rules keys "every port" apart from
			// port numbers, so these do not tie yet and this case fails.
			name: "a rule on every port ties with one on a single port",
			policy: validEgressPolicy(
				withHTTPRule(hostnames("api.example.com"), withPorts(allPorts())),
				withHTTPRule(hostnames("api.example.com"), withPorts(ports(8080))),
			),
			want: field.ErrorList{
				field.Invalid(field.NewPath("egress_policy", "rules").Index(1).Child("http", "hostnames").Index(0), "api.example.com", `ties with egress_policy.rules[0].http.hostnames[0] on port 8080`),
			},
		}, { */
		// Ties compare patterns, not the hosts they match.
		name: "a wildcard and a host it matches do not tie",
		policy: validEgressPolicy(
			withHTTPRule(hostnames("*.example.com")),
			withHTTPRule(hostnames("api.example.com")),
		),
	}, {
		name: "valid header name Authorization",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("Authorization", "", "ate-secret://kubernetes.io/provider/ns/name")),
			),
		),
	}, {
		name: "valid header name x-custom_header",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("x-custom_header", "", "ate-secret://kubernetes.io/provider/ns/name")),
			),
		),
	}, {
		name: "valid header name !#$%&'*+-.^_`|~",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("!#$%&'*+-.^_`|~", "", "ate-secret://kubernetes.io/provider/ns/name")),
			),
		),
	}, {
		name: "header name with a colon",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("bad:header", "", "ate-secret://kubernetes.io/provider/ns/name")),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("header"), "bad:header", "must be an HTTP header name"),
		},
	}, {
		name: "header name with a non-ASCII letter",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("héader", "", "ate-secret://kubernetes.io/provider/ns/name")),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("header"), "héader", "must be an HTTP header name"),
		},
	}, {
		name: "valid prefix with a tab",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("Authorization", "value\tvalue", "ate-secret://kubernetes.io/provider/ns/name")),
			),
		),
	}, {
		name: "valid prefix with bytes above 0x7f",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("Authorization", "\u0080\u0081", "ate-secret://kubernetes.io/provider/ns/name")),
			),
		),
	}, {
		name: "prefix with a line feed",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("Authorization", "a\nb", "ate-secret://kubernetes.io/provider/ns/name")),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("prefix"), "a\nb", "must be a valid HTTP field value prefix"),
		},
	}, {
		name: "prefix with a NUL",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("Authorization", "a\x00b", "ate-secret://kubernetes.io/provider/ns/name")),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("prefix"), "a\x00b", "must be a valid HTTP field value prefix"),
		},
	}, {
		name: "prefix with a control character",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("Authorization", "a\x1fb", "ate-secret://kubernetes.io/provider/ns/name")),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("prefix"), "a\x1fb", "must be a valid HTTP field value prefix"),
		},
	}, {
		name: "prefix with a DEL",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("Authorization", "a\x7fb", "ate-secret://kubernetes.io/provider/ns/name")),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("prefix"), "a\x7fb", "must be a valid HTTP field value prefix"),
		},
	}, {
		name: "valid credential URI with another provider class",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("Authorization", "", "ate-secret://vault.example/provider/secret")),
			),
		),
	}, {
		name: "credential URI without a tail",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("Authorization", "", "ate-secret://kubernetes.io/provider")),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("credential_uri"), "ate-secret://kubernetes.io/provider", "must be ate-secret://<provider-class>/<provider-name>/<provider-specific-tail>"),
		},
	}, {
		name: "credential URI with an empty segment",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("Authorization", "", "ate-secret://kubernetes.io//provider/secret")),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("credential_uri"), "ate-secret://kubernetes.io//provider/secret", "must be ate-secret://<provider-class>/<provider-name>/<provider-specific-tail>"),
		},
	}, {
		name: "credential URI with a trailing slash",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("Authorization", "", "ate-secret://kubernetes.io/provider/secret/")),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("credential_uri"), "ate-secret://kubernetes.io/provider/secret/", "must be ate-secret://<provider-class>/<provider-name>/<provider-specific-tail>"),
		},
	}, {
		name: "credential URI with a port",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("Authorization", "", "ate-secret://kubernetes.io:443/provider/secret")),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("credential_uri"), "ate-secret://kubernetes.io:443/provider/secret", "must be ate-secret://<provider-class>/<provider-name>/<provider-specific-tail>"),
		},
	}, {
		name: "credential URI with a percent-encoded separator",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("Authorization", "", "ate-secret://kubernetes.io/provider/sec%2Fret")),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("credential_uri"), "ate-secret://kubernetes.io/provider/sec%2Fret", "must be ate-secret://<provider-class>/<provider-name>/<provider-specific-tail>"),
		},
	}, {
		name: "credential URI with any percent-encoding",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("Authorization", "", "ate-secret://kubernetes.io/provider/sec%2Dret")),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("credential_uri"), "ate-secret://kubernetes.io/provider/sec%2Dret", "must be ate-secret://<provider-class>/<provider-name>/<provider-specific-tail>"),
		},
	}, {
		name: "credential URI with user info",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("Authorization", "", "ate-secret://user@kubernetes.io/provider/secret")),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("credential_uri"), "ate-secret://user@kubernetes.io/provider/secret", "must be ate-secret://<provider-class>/<provider-name>/<provider-specific-tail>"),
		},
	}, {
		name: "credential URI with a query",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("Authorization", "", "ate-secret://kubernetes.io/provider/secret?version=2")),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("credential_uri"), "ate-secret://kubernetes.io/provider/secret?version=2", "must be ate-secret://<provider-class>/<provider-name>/<provider-specific-tail>"),
		},
	}, {
		name: "credential URI with a fragment",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("Authorization", "", "ate-secret://kubernetes.io/provider/secret#v2")),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("credential_uri"), "ate-secret://kubernetes.io/provider/secret#v2", "must be ate-secret://<provider-class>/<provider-name>/<provider-specific-tail>"),
		},
	}, {
		name: "credential URI with an uppercase provider class",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("Authorization", "", "ate-secret://Kubernetes.IO/provider/secret")),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("credential_uri"), "ate-secret://Kubernetes.IO/provider/secret", "must be ate-secret://<provider-class>/<provider-name>/<provider-specific-tail>"),
		},
	}, {
		name: "credential URI with an underscore in the provider class",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("Authorization", "", "ate-secret://kubernetes_io/provider/secret")),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("credential_uri"), "ate-secret://kubernetes_io/provider/secret", "must be ate-secret://<provider-class>/<provider-name>/<provider-specific-tail>"),
		},
	}, {
		name: "missing static header",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("", "Bearer ", "ate-secret://kubernetes.io/provider/ns/name")),
			),
		),
		want: field.ErrorList{
			field.Required(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("header"), ""),
		},
	}, {
		name: "invalid static header",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("bad header", "Bearer ", "ate-secret://kubernetes.io/provider/ns/name")),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("header"), "bad header", "must be an HTTP header name"),
		},
	}, {
		name: "duplicate header",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(
					authHeader(),
					credentialHeader("authorization", "", "ate-secret://example.com/provider/secret"),
				),
			),
		),
		want: field.ErrorList{
			field.Duplicate(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(1).Child("header"), "authorization"),
		},
	}, {
		name: "same header in later rule",
		policy: validEgressPolicy(
			withHTTPRule(hostnames("api.example.com"), withEffects(authHeader())),
			withHTTPRule(hostnames("other.example.com"), withEffects(authHeader())),
		),
	}, {
		name:   "many headers",
		policy: validEgressPolicy(withHTTPRule(hostnames("api.example.com"), withEffects(numberedHeaders(16)...))),
	}, {
		name:   "too many headers",
		policy: validEgressPolicy(withHTTPRule(hostnames("api.example.com"), withEffects(numberedHeaders(17)...))),
		want: field.ErrorList{
			field.TooMany(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers"), 17, 16).WithOrigin("maxItems"),
		},
	}, {
		name: "invalid prefix",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("Authorization", "Bearer\r", "ate-secret://kubernetes.io/provider/ns/name")),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("prefix"), "Bearer\r", "must be a valid HTTP field value prefix"),
		},
	}, {
		name: "no credential source",
		policy: validEgressPolicy(
			withHTTPRule(hostnames("api.example.com"), withEffects(credentialHeader("Authorization", "Bearer ", ""))),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0), nil, "one of").WithOrigin("union"),
		},
	}, {
		name: "invalid credential URI",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(credentialHeader("Authorization", "Bearer ", "https://example.com/secret")),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("credential_uri"), "https://example.com/secret", "must be ate-secret://<provider-class>/<provider-name>/<provider-specific-tail>"),
		},
	}, {
		name: "actor JWT on an http rule",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(actorJWTHeader("Authorization", "Bearer ", 900, "https://api.example.com")),
			),
		),
	}, {
		name: "actor JWT on an https rule",
		policy: validEgressPolicy(
			withHTTPSRule(
				hostnames("api.example.com"),
				withHTTPSEffects(actorJWTHeader("Authorization", "Bearer ", 900, "https://api.example.com")),
			),
		),
	}, {
		name: "actor JWT next to a credential URI",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(
					authHeader(),
					actorJWTHeader("X-Actor-Token", "Bearer ", 900, "https://api.example.com"),
				),
			),
		),
	}, {
		name: "credential URI and actor JWT in one header",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(&ateapi.CredentialHeader{
					Header:        "Authorization",
					Prefix:        "Bearer ",
					CredentialUri: "ate-secret://kubernetes.io/provider/ns/name",
					ActorJwt: &ateapi.ActorJwtSource{
						Audiences:         []string{"https://api.example.com"},
						ExpirationSeconds: 900,
					},
				}),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0), nil, "one of").WithOrigin("union"),
		},
	}, {
		name: "actor JWT without audiences",
		policy: validEgressPolicy(
			withHTTPRule(hostnames("api.example.com"), withEffects(actorJWTHeader("Authorization", "Bearer ", 900))),
		),
		want: field.ErrorList{
			field.Required(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("actor_jwt", "audiences"), ""),
		},
	}, {
		name: "actor JWT with an empty audience",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(actorJWTHeader("Authorization", "Bearer ", 900, "")),
			),
		),
		want: field.ErrorList{
			field.TooShort(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("actor_jwt", "audiences").Index(0), "", 1).WithOrigin("minLength"),
		},
	}, {
		name: "actor JWT with a long audience",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(actorJWTHeader("Authorization", "Bearer ", 900, strings.Repeat("a", 513))),
			),
		),
		want: field.ErrorList{
			field.TooLong(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("actor_jwt", "audiences").Index(0), "", 512).WithOrigin("maxLength"),
		},
	}, {
		name: "actor JWT with a repeated audience",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(actorJWTHeader("Authorization", "Bearer ", 900, "a", "a")),
			),
		),
		want: field.ErrorList{
			field.Duplicate(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("actor_jwt", "audiences").Index(1), "a"),
		},
	}, {
		name: "actor JWT with too many audiences",
		policy: validEgressPolicy(func(p *ateapi.EgressPolicy) {
			var audiences []string
			for i := range 17 {
				audiences = append(audiences, fmt.Sprintf("aud-%d", i))
			}
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(actorJWTHeader("Authorization", "Bearer ", 900, audiences...)),
			)(p)
		}),
		want: field.ErrorList{
			field.TooMany(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("actor_jwt", "audiences"), 17, 16).WithOrigin("maxItems"),
		},
	}, {
		name: "actor JWT without a lifetime",
		policy: validEgressPolicy(
			withHTTPRule(hostnames("api.example.com"), withEffects(actorJWTHeader("Authorization", "Bearer ", 0, "a"))),
		),
		want: field.ErrorList{
			field.Required(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("actor_jwt", "expiration_seconds"), ""),
		},
	}, {
		name: "actor JWT lifetime at its bounds",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(
					actorJWTHeader("Authorization", "Bearer ", 300, "a"),
					actorJWTHeader("X-Actor-Token", "Bearer ", 3600, "a"),
				),
			),
		),
	}, {
		name: "actor JWT lifetime too short",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(actorJWTHeader("Authorization", "Bearer ", 299, "a")),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("actor_jwt", "expiration_seconds"), 299, "").WithOrigin("minimum"),
		},
	}, {
		name: "actor JWT lifetime too long",
		policy: validEgressPolicy(
			withHTTPRule(
				hostnames("api.example.com"),
				withEffects(actorJWTHeader("Authorization", "Bearer ", 3601, "a")),
			),
		),
		want: field.ErrorList{
			field.Invalid(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(0).Child("actor_jwt", "expiration_seconds"), 3601, "").WithOrigin("maximum"),
		},
	}, {
		name:   "nil header",
		policy: validEgressPolicy(withHTTPRule(hostnames("api.example.com"), withEffects(authHeader(), nil))),
		want: field.ErrorList{
			field.Required(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects", "replace_headers").Index(1), ""),
		},
	}, {
		name:   "empty effects",
		policy: validEgressPolicy(withHTTPRule(hostnames("api.example.com"), withEffects())),
		want: field.ErrorList{
			field.Required(field.NewPath("egress_policy", "rules").Index(0).Child("http", "effects"), "at least one effect must be specified"),
		},
	}, {
		name:   "effects on an https rule",
		policy: validEgressPolicy(withHTTPSRule(hostnames("api.example.com"), withHTTPSEffects())),
		want: field.ErrorList{
			field.Required(field.NewPath("egress_policy", "rules").Index(0).Child("https", "effects"), "at least one effect must be specified"),
		},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := &ateapi.CreateActorEgressPolicyRequest{
				Actor:        &ateapi.ObjectRef{Atespace: testAtespace, Name: "actor"},
				EgressPolicy: tc.policy,
			}
			defaults.Apply(req.EgressPolicy)
			assertValidateErr(t, ValidateCreateActorEgressPolicyRequest(context.Background(), req), tc.want)
		})
	}
}
