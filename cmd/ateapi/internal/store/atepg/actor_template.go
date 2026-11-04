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

func (p *Persistence) CreateActorTemplate(ctx context.Context, template *ateapi.ActorTemplate) (*ateapi.ActorTemplate, error) {
	atespace, name := template.GetMetadata().GetAtespace(), template.GetMetadata().GetName()
	dbTemplate := proto.Clone(template).(*ateapi.ActorTemplate)
	if dbTemplate.Metadata == nil {
		dbTemplate.Metadata = &ateapi.ResourceMetadata{}
	}
	setCreateMetadata(dbTemplate.Metadata)
	protoBytes, err := proto.Marshal(dbTemplate)
	if err != nil {
		return nil, fmt.Errorf("marshaling actor template: %w", err)
	}
	_, err = p.pool.Exec(ctx, `
		INSERT INTO actor_templates (atespace, name, uid, version, proto)
		VALUES ($1, $2, $3, $4, $5)`,
		atespace, name, dbTemplate.GetMetadata().GetUid(), dbTemplate.GetMetadata().GetVersion(), protoBytes)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, store.ErrAlreadyExists
		}
		if isForeignKeyViolation(err) {
			return nil, store.ErrFailedPrecondition
		}
		return nil, fmt.Errorf("inserting actor template %s/%s: %w", atespace, name, err)
	}
	return dbTemplate, nil
}

func (p *Persistence) GetActorTemplate(ctx context.Context, templateRef resources.ActorTemplateRef) (*ateapi.ActorTemplate, error) {
	var protoBytes []byte
	err := p.pool.QueryRow(ctx, `SELECT proto FROM actor_templates WHERE atespace = $1 AND name = $2`, templateRef.Atespace, templateRef.Name).Scan(&protoBytes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("getting actor template %s: %w", templateRef, err)
	}
	out := &ateapi.ActorTemplate{}
	if err := unmarshalStored(protoBytes, out); err != nil {
		return nil, fmt.Errorf("unmarshaling actor template: %w", err)
	}
	return out, nil
}

func validateUpdateActorTemplateMutation(storedTemplate, mutatedTemplate *ateapi.ActorTemplate) error {
	if stored, mutated := storedTemplate.GetMetadata().GetAtespace(), mutatedTemplate.GetMetadata().GetAtespace(); stored != mutated {
		return fmt.Errorf("metadata.atespace is immutable: mutation changed it from %q to %q", stored, mutated)
	}
	if stored, mutated := storedTemplate.GetMetadata().GetName(), mutatedTemplate.GetMetadata().GetName(); stored != mutated {
		return fmt.Errorf("metadata.name is immutable: mutation changed it from %q to %q", stored, mutated)
	}
	return nil
}

func (p *Persistence) UpdateActorTemplate(ctx context.Context, templateRef resources.ActorTemplateRef, precondition store.Precondition, mutate func(*ateapi.ActorTemplate) error) (*ateapi.ActorTemplate, error) {
	if err := precondition.Validate(); err != nil {
		return nil, err
	}
	return p.updateActorTemplate(ctx, templateRef, store.DeletePreconditions(precondition), mutate, false)
}

func (p *Persistence) MarkActorTemplateForDeletion(ctx context.Context, templateRef resources.ActorTemplateRef, precondition store.DeletePreconditions, mutate func(*ateapi.ActorTemplate) error) (*ateapi.ActorTemplate, error) {
	return p.updateActorTemplate(ctx, templateRef, precondition, mutate, true)
}

func (p *Persistence) updateActorTemplate(ctx context.Context, templateRef resources.ActorTemplateRef, precondition store.DeletePreconditions, mutate func(*ateapi.ActorTemplate) error, markForDeletion bool) (*ateapi.ActorTemplate, error) {
	return updateRow(ctx, p.pool, rowUpdate[*ateapi.ActorTemplate]{
		what:            "actor template " + templateRef.String(),
		table:           "actor_templates",
		where:           "atespace = $1 AND name = $2",
		key:             []any{templateRef.Atespace, templateRef.Name},
		precondition:    precondition,
		mutate:          mutate,
		validateChange:  validateUpdateActorTemplateMutation,
		markForDeletion: markForDeletion,
	})
}

func (p *Persistence) ListActorTemplates(ctx context.Context, atespace string, opts store.ListOptions) (store.ListResponse[*ateapi.ActorTemplate], error) {
	lq := listQuery{
		what:     "actor templates",
		item:     "actor template",
		kind:     kindActorTemplate,
		idParts:  2,
		keyParts: 2,
		firstSQL: `
			SELECT atespace, name, proto FROM actor_templates
			ORDER BY atespace, name
			LIMIT $1`,
		nextSQL: `
			SELECT atespace, name, proto FROM actor_templates
			WHERE (atespace, name) > ($1, $2)
			ORDER BY atespace, name
			LIMIT $3`,
	}
	if atespace != "" {
		lq = listQuery{
			what:     fmt.Sprintf("actor templates in %q", atespace),
			item:     "actor template",
			kind:     kindActorTemplate,
			scope:    atespace,
			idParts:  2,
			keyParts: 1,
			firstSQL: `
				SELECT atespace, name, proto FROM actor_templates
				WHERE atespace = $1
				ORDER BY name
				LIMIT $2`,
			nextSQL: `
				SELECT atespace, name, proto FROM actor_templates
				WHERE atespace = $1 AND name > $2
				ORDER BY name
				LIMIT $3`,
			args: []any{atespace},
		}
	}
	return listPage(ctx, p.pool, lq, opts, decodeStored[*ateapi.ActorTemplate]())
}

func (p *Persistence) DeleteActorTemplate(ctx context.Context, templateRef resources.ActorTemplateRef, precondition store.DeletePreconditions) (*ateapi.ActorTemplate, error) {
	var protoBytes []byte
	err := p.pool.QueryRow(ctx, `
		DELETE FROM actor_templates AS t
		WHERE t.atespace = $1 AND t.name = $2
		  AND ($3::text = '' OR t.uid = $3::text)
		  AND ($4::bigint = 0 OR t.version = $4::bigint)
		RETURNING t.proto`, templateRef.Atespace, templateRef.Name, precondition.UID, precondition.Version).Scan(&protoBytes)
	if errors.Is(err, pgx.ErrNoRows) {
		var uid string
		var version int64
		err := p.pool.QueryRow(ctx, `SELECT uid, version FROM actor_templates WHERE atespace = $1 AND name = $2`, templateRef.Atespace, templateRef.Name).Scan(&uid, &version)
		return nil, mapDeleteError(err, uid, version, precondition)
	}
	if err != nil {
		return nil, fmt.Errorf("deleting actor template %s: %w", templateRef, err)
	}
	out := &ateapi.ActorTemplate{}
	if err := unmarshalStored(protoBytes, out); err != nil {
		return nil, fmt.Errorf("unmarshaling deleted actor template: %w", err)
	}
	return out, nil
}
