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
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"go.ate.dev/substrate/api/ateapi/v1"
	"go.ate.dev/substrate/internal/preview"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	testAtespace = "test-atespace"
)

const (
	// someActorUID stands in for the UID the store assigns an Actor, for tests
	// that need a well-formed snapshot URI but never exercise who owns it. Those
	// seed their Actor in a single call, before a real UID exists.
	someActorUID = "6b1f9d0c-4a2e-4d38-9c77-5e0a1b2c3d4e"
)

// assertValidate checks validate against want with no preview gates enabled.
// Then, for each entry in wantWithPreview, it enables the gates the key names
// and checks validate against that entry's errors. Only cases that set fields
// behind a preview gate need wantWithPreview.
func assertValidate(t *testing.T, validate func() field.ErrorList, want field.ErrorList, wantWithPreview map[string]field.ErrorList) {
	t.Helper()
	setPreviewForTest(t, "")
	assertValidateErr(t, validate(), want)
	for _, gates := range slices.Sorted(maps.Keys(wantWithPreview)) {
		t.Run("preview="+gates, func(t *testing.T) {
			t.Helper()
			setPreviewForTest(t, gates)
			assertValidateErr(t, validate(), wantWithPreview[gates])
		})
	}
}

// setPreviewForTest enables the preview gates named by gates, a --preview
// value such as "ALL", for the duration of t. An empty value enables none.
func setPreviewForTest(t *testing.T, gates string) {
	t.Helper()
	var values []string
	if gates != "" {
		values = strings.Split(gates, ",")
	}
	preview.InitForTest(t, values...)
}

func selectorLabelsOfSize(n int) map[string]string {
	labels := make(map[string]string, n)
	for i := 0; i < n; i++ {
		labels[fmt.Sprintf("k%d", i)] = "v"
	}
	return labels
}

func assertValidateErr(t *testing.T, got field.ErrorList, want field.ErrorList) {
	t.Helper()
	field.ErrorMatcher{}.ByType().ByField().ByOrigin().Test(t, want, got)
}

// hostnames returns its arguments, so a rule in a test table reads like
// withHTTPRule(hostnames("a.example.com"), ports(80)).
func hostnames(hosts ...string) []string {
	return hosts
}

// ports returns Ports with the given numbers.
func ports(nums ...int32) *ateapi.Ports {
	return &ateapi.Ports{Numbers: nums}
}

// allPorts returns Ports matching every port.
func allPorts() *ateapi.Ports {
	return &ateapi.Ports{All: &ateapi.AllPorts{}}
}

// numberedPorts returns n distinct port numbers.
func numberedPorts(n int) []int32 {
	nums := make([]int32, n)
	for i := range nums {
		nums[i] = int32(8000 + i)
	}
	return nums
}

// numberedAudiences returns n distinct audiences.
func numberedAudiences(n int) []string {
	auds := make([]string, n)
	for i := range auds {
		auds[i] = fmt.Sprintf("https://%d.example.com", i)
	}
	return auds
}

// numberedHTTPRules returns n HTTP rules, each for a distinct hostname.
func numberedHTTPRules(n int) []*ateapi.EgressRule {
	rules := make([]*ateapi.EgressRule, n)
	for i := range rules {
		rules[i] = &ateapi.EgressRule{
			Http: &ateapi.HttpRule{Hostnames: hostnames(fmt.Sprintf("host-%d.example.com", i))},
		}
	}
	return rules
}

// combine returns a modifier func which applies mods in order, for a test
// table field that holds a single modifier.
func combine[T any](mods ...func(T)) func(T) {
	return func(obj T) {
		for _, m := range mods {
			m(obj)
		}
	}
}

// actorRefGetter is a request that names an Actor.
type actorRefGetter interface {
	GetActor() *ateapi.ObjectRef
}

// withRequestActor returns a modifier func for a request of type T which
// applies mods to its actor ref. Instantiate it per request type, as in
// withRequestActor[*ateapi.GetActorEgressPolicyRequest].
func withRequestActor[T actorRefGetter](mods ...func(*ateapi.ObjectRef)) func(T) {
	return func(r T) {
		for _, m := range mods {
			m(r.GetActor())
		}
	}
}

// egressPolicyGetter is a request that carries an EgressPolicy.
type egressPolicyGetter interface {
	GetEgressPolicy() *ateapi.EgressPolicy
}

// withRequestEgressPolicy returns a modifier func for a request of type T
// which applies mods to its egress policy.
func withRequestEgressPolicy[T egressPolicyGetter](mods ...func(*ateapi.EgressPolicy)) func(T) {
	return func(r T) {
		for _, m := range mods {
			m(r.GetEgressPolicy())
		}
	}
}

// withObjectRefAtespace returns a modifier func which sets an ObjectRef's
// atespace.
func withObjectRefAtespace(atespace string) func(*ateapi.ObjectRef) {
	return func(ref *ateapi.ObjectRef) { ref.Atespace = atespace }
}

// withObjectRefName returns a modifier func which sets an ObjectRef's name.
func withObjectRefName(name string) func(*ateapi.ObjectRef) {
	return func(ref *ateapi.ObjectRef) { ref.Name = name }
}
