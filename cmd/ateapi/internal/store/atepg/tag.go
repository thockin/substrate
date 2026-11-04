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
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"go.ate.dev/substrate/api/ateapi/v1"
	"go.ate.dev/substrate/cmd/ateapi/internal/store"
	"go.ate.dev/substrate/internal/resources"
	"google.golang.org/protobuf/proto"
)

func (p *Persistence) GetTag(ctx context.Context, tagRef resources.TagRef) (*ateapi.Tag, error) {
	atespace, name := tagRef.Atespace, tagRef.Name
	var protoBytes []byte
	if err := p.pool.QueryRow(ctx, `
		SELECT proto FROM tags
		WHERE atespace = $1 AND name = $2`, atespace, name).Scan(&protoBytes); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("getting tag %s/%s: %w", atespace, name, err)
	}
	tag := &ateapi.Tag{}
	if err := unmarshalStored(protoBytes, tag); err != nil {
		return nil, fmt.Errorf("unmarshaling tag: %w", err)
	}
	return tag, nil
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
	atespace := tag.GetMetadata().GetAtespace()
	name := tag.GetMetadata().GetName()
	dbTag := proto.CloneOf(tag)
	setCreateMetadata(dbTag.Metadata)
	if err := validateSnapshotURIs(fmt.Sprintf("tag %s/%s", atespace, name), dbTag.GetStatus().GetSnapshot()); err != nil {
		return nil, err
	}
	protoBytes, err := proto.Marshal(dbTag)
	if err != nil {
		return nil, fmt.Errorf("marshaling tag: %w", err)
	}

	_, err = p.pool.Exec(ctx, `
		INSERT INTO tags (atespace, name, uid, version, proto)
		VALUES ($1, $2, $3, $4, $5)`, atespace, name,
		dbTag.GetMetadata().GetUid(), dbTag.GetMetadata().GetVersion(), protoBytes)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, store.ErrAlreadyExists
		}
		if isForeignKeyViolation(err) {
			switch pgErrConstraint(err) {
			case "tags_atespace_fk":
				return nil, store.ErrFailedPrecondition
			default:
				return nil, fmt.Errorf("inserting tag %s/%s violated unknown foreign key %q: %w", atespace, name, pgErrConstraint(err), err)
			}
		}
		return nil, fmt.Errorf("inserting tag %s/%s: %w", atespace, name, err)
	}
	return dbTag, nil
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
		validate: func(mutated *ateapi.Tag) error {
			return validateSnapshotURIs(what, mutated.GetStatus().GetSnapshot())
		},
		markForDeletion: markForDeletion,
	})
}

func (p *Persistence) DeleteTag(ctx context.Context, tagRef resources.TagRef, precondition store.DeletePreconditions) (*ateapi.Tag, error) {
	atespace, name := tagRef.Atespace, tagRef.Name
	var protoBytes []byte
	err := p.pool.QueryRow(ctx, `
		DELETE FROM tags
		WHERE atespace = $1 AND name = $2
		  AND ($3::text = '' OR uid = $3::text)
		  AND ($4::bigint = 0 OR version = $4::bigint)
		RETURNING proto`, atespace, name, precondition.UID, precondition.Version).Scan(&protoBytes)
	if errors.Is(err, pgx.ErrNoRows) {
		var uid string
		var version int64
		err := p.pool.QueryRow(ctx, `SELECT uid, version FROM tags WHERE atespace = $1 AND name = $2`, atespace, name).Scan(&uid, &version)
		return nil, mapDeleteError(err, uid, version, precondition)
	}
	if err != nil {
		return nil, fmt.Errorf("deleting tag %s/%s: %w", atespace, name, err)
	}
	tag := &ateapi.Tag{}
	if err := unmarshalStored(protoBytes, tag); err != nil {
		return nil, fmt.Errorf("unmarshaling deleted tag: %w", err)
	}
	return tag, nil
}
