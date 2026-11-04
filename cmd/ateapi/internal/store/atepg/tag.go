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

package atepg

import (
	"context"
	"fmt"

	"go.ate.dev/substrate/api/ateapi/v1"
	"go.ate.dev/substrate/cmd/ateapi/internal/store"
	"go.ate.dev/substrate/internal/resources"
	"google.golang.org/protobuf/proto"
)

func (p *Persistence) GetTag(ctx context.Context, tagRef resources.TagRef) (*ateapi.Tag, error) {
	return getRow[*ateapi.Tag](ctx, p.pool, fmt.Sprintf("tag %s/%s", tagRef.Atespace, tagRef.Name),
		`SELECT uid, version, proto FROM tags WHERE atespace = $1 AND name = $2`, tagRef.Atespace, tagRef.Name)
}

func (p *Persistence) ListTags(ctx context.Context, atespace string, opts store.ListOptions) (store.ListResponse[*ateapi.Tag], error) {
	lq := listQuery{
		what:     "tags",
		item:     "tag",
		kind:     kindTag,
		idParts:  2,
		keyParts: 2,
		firstSQL: `
			SELECT atespace, name, proto FROM tags
			ORDER BY atespace, name
			LIMIT $1`,
		nextSQL: `
			SELECT atespace, name, proto FROM tags
			WHERE (atespace, name) > ($1, $2)
			ORDER BY atespace, name
			LIMIT $3`,
	}
	if atespace != "" {
		lq = listQuery{
			what:     fmt.Sprintf("tags in %q", atespace),
			item:     "tag",
			kind:     kindTag,
			scope:    atespace,
			idParts:  2,
			keyParts: 1,
			firstSQL: `
				SELECT atespace, name, proto FROM tags
				WHERE atespace = $1
				ORDER BY name
				LIMIT $2`,
			nextSQL: `
				SELECT atespace, name, proto FROM tags
				WHERE atespace = $1 AND name > $2
				ORDER BY name
				LIMIT $3`,
			args: []any{atespace},
		}
	}
	return listPage(ctx, p.pool, lq, opts, decodeStored[*ateapi.Tag]())
}

func (p *Persistence) CreateTag(ctx context.Context, tag *ateapi.Tag) (*ateapi.Tag, error) {
	atespace, name := tag.GetMetadata().GetAtespace(), tag.GetMetadata().GetName()
	what := fmt.Sprintf("tag %s/%s", atespace, name)
	if err := validateSnapshotURIs(what, tag.GetStatus().GetSnapshot()); err != nil {
		return nil, err
	}
	return createRow(ctx, p.pool, rowInsert{
		what:  what,
		table: "tags",
		cols:  "atespace, name",
		key:   []any{atespace, name},
	}, tag)
}

func isAllowedTagSnapshotTransition(stored, mutated *ateapi.Snapshot) bool {
	if proto.Equal(stored, mutated) || stored == nil {
		return true
	}
	if mutated == nil || stored.GetDurableSnapshot() == nil || mutated.GetDurableSnapshot() == nil {
		return false
	}
	if stored.GetDurableSnapshot().GetState() != ateapi.SnapshotStorageState_SNAPSHOT_STORAGE_STATE_PENDING ||
		mutated.GetDurableSnapshot().GetState() != ateapi.SnapshotStorageState_SNAPSHOT_STORAGE_STATE_COMPLETED {
		return false
	}
	expected := proto.CloneOf(stored)
	expected.DurableSnapshot.State = ateapi.SnapshotStorageState_SNAPSHOT_STORAGE_STATE_COMPLETED
	return proto.Equal(expected, mutated)
}

func validateUpdateTagMutation(storedTag, mutatedTag *ateapi.Tag) error {
	if stored, mutated := storedTag.GetMetadata().GetAtespace(), mutatedTag.GetMetadata().GetAtespace(); stored != mutated {
		return fmt.Errorf("metadata.atespace is immutable: mutation changed it from %q to %q", stored, mutated)
	}
	if stored, mutated := storedTag.GetMetadata().GetName(), mutatedTag.GetMetadata().GetName(); stored != mutated {
		return fmt.Errorf("metadata.name is immutable: mutation changed it from %q to %q", stored, mutated)
	}
	if stored, mutated := storedTag.GetStatus().GetSnapshot(), mutatedTag.GetStatus().GetSnapshot(); !isAllowedTagSnapshotTransition(stored, mutated) {
		return fmt.Errorf("status.snapshot is immutable once set: mutation changed it from %s to %s", stored, mutated)
	}
	if stored, mutated := storedTag.GetStatus().GetActorTemplateUid(), mutatedTag.GetStatus().GetActorTemplateUid(); stored != mutated {
		return fmt.Errorf("status.actor_template_uid is immutable: mutation changed it from %q to %q", stored, mutated)
	}
	return nil
}

func (p *Persistence) UpdateTag(ctx context.Context, tagRef resources.TagRef, precondition store.Precondition, mutate func(*ateapi.Tag) error) (*ateapi.Tag, error) {
	if err := precondition.Validate(); err != nil {
		return nil, err
	}
	return p.updateTag(ctx, tagRef, store.DeletePreconditions(precondition), mutate, false)
}

func (p *Persistence) MarkTagForDeletion(ctx context.Context, tagRef resources.TagRef, precondition store.DeletePreconditions, mutate func(*ateapi.Tag) error) (*ateapi.Tag, error) {
	return p.updateTag(ctx, tagRef, precondition, mutate, true)
}

func (p *Persistence) updateTag(ctx context.Context, tagRef resources.TagRef, precondition store.DeletePreconditions, mutate func(*ateapi.Tag) error, markForDeletion bool) (*ateapi.Tag, error) {
	what := fmt.Sprintf("tag %s/%s", tagRef.Atespace, tagRef.Name)
	return updateRow(ctx, p.pool, rowUpdate[*ateapi.Tag]{
		what:         what,
		table:        "tags",
		where:        "atespace = $1 AND name = $2",
		key:          []any{tagRef.Atespace, tagRef.Name},
		precondition: precondition,
		mutate:       mutate,
		// TODO: this should be done through DV and removed from here
		validateChange: func(stored, mutated *ateapi.Tag) error {
			if err := validateUpdateTagMutation(stored, mutated); err != nil {
				return fmt.Errorf("%w: %w", store.ErrImmutableField, err)
			}
			return nil
		},
		validate: func(mutated *ateapi.Tag) error {
			return validateSnapshotURIs(what, mutated.GetStatus().GetSnapshot())
		},
		markForDeletion: markForDeletion,
	})
}

func (p *Persistence) DeleteTag(ctx context.Context, tagRef resources.TagRef, precondition store.DeletePreconditions) (*ateapi.Tag, error) {
	return deleteRow[*ateapi.Tag](ctx, p.pool, rowDelete{
		what:         fmt.Sprintf("tag %s/%s", tagRef.Atespace, tagRef.Name),
		table:        "tags",
		where:        "atespace = $1 AND name = $2",
		key:          []any{tagRef.Atespace, tagRef.Name},
		precondition: precondition,
	})
}
