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
	"fmt"
	"runtime"
	"testing"

	"github.com/agent-substrate/substrate/pkg/proto/ateapipb"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func assertValidateErr(t *testing.T, got field.ErrorList, want field.ErrorList) {
	t.Helper()
	field.ErrorMatcher{}.ByType().ByField().ByOrigin().Test(t, want, got)
}

func line() string {
	_, _, line, ok := runtime.Caller(1)
	var s string
	if ok {
		s = fmt.Sprintf("%d", line)
	} else {
		s = "<??>"
	}
	return s
}

func validActor(mutate func(*ateapipb.Actor)) *ateapipb.Actor {
	a := &ateapipb.Actor{
		Metadata:               &ateapipb.ResourceMetadata{Atespace: "my-atespace", Name: "my-name"},
		ActorTemplateNamespace: "actor-template-ns",
		ActorTemplateName:      "actor-template-name",
		WorkerAssignment: &ateapipb.WorkerAssignment{
			WorkerNamespace: "worker-ns",
			WorkerPool:      "worker-pool",
			WorkerPod:       "worker-pod",
			WorkerPodIp:     "1.2.3.4",
			WorkerPodUid:    "01234567-89ab-cdef-0123-456789abcdef",
		},
	}
	if mutate != nil {
		mutate(a)
	}
	return a
}
