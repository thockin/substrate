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
	"github.com/agent-substrate/substrate/pkg/proto/ateapipb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// newMetadataForUpdate creates new derived metadata for an update operation.
// All update operations MUST call this.
func newMetadataForUpdate(current *ateapipb.ResourceMetadata) *ateapipb.ResourceMetadata {
	next := proto.Clone(current).(*ateapipb.ResourceMetadata)
	next.Version = current.GetVersion() + 1
	next.UpdateTime = timestamppb.Now()
	return next
}
