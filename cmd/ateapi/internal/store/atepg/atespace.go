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
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (p *Persistence) CreateAtespace(ctx context.Context, atespace *ateapi.Atespace) (*ateapi.Atespace, error) {
	name := atespace.GetMetadata().GetName()
	return createRow(ctx, p.pool, rowInsert{
		what:  fmt.Sprintf("atespace %q", name),
		table: "atespaces",
		cols:  "name",
		key:   []any{name},
	}, atespace)
}

func (p *Persistence) GetAtespace(ctx context.Context, name string) (*ateapi.Atespace, error) {
	return getRow[*ateapi.Atespace](ctx, p.pool, fmt.Sprintf("atespace %q", name),
		`SELECT uid, version, proto FROM atespaces WHERE name = $1`, name)
}

func (p *Persistence) ListAtespaces(ctx context.Context, opts store.ListOptions) (store.ListResponse[*ateapi.Atespace], error) {
	return listPage(ctx, p.pool, listQuery{
		what:     "atespaces",
		item:     "atespace",
		kind:     kindAtespace,
		idParts:  1,
		keyParts: 1,
		firstSQL: `
			SELECT name, proto FROM atespaces
			ORDER BY name
			LIMIT $1`,
		nextSQL: `
			SELECT name, proto FROM atespaces
			WHERE name > $1
			ORDER BY name
			LIMIT $2`,
	}, opts, decodeStored[*ateapi.Atespace]())
}

func (p *Persistence) DeleteAtespace(ctx context.Context, name string, precondition store.DeletePreconditions) (*ateapi.Atespace, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning atespace delete: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	// The foreign keys of actors, actor templates and tags restrict the
	// delete, so it fails while the atespace holds any of them.
	out, err := deleteRow[*ateapi.Atespace](ctx, tx, rowDelete{
		what:         fmt.Sprintf("atespace %q", name),
		table:        "atespaces",
		where:        "name = $1",
		key:          []any{name},
		precondition: precondition,
	})
	if isForeignKeyViolation(err) {
		return nil, store.ErrFailedPrecondition
	}
	if err != nil {
		return nil, err
	}
	if err := p.policyManager.DeleteAtespacePolicies(ctx, tx, name); err != nil {
		return nil, fmt.Errorf("deleting authorization tuples for atespace %q: %w", name, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing atespace delete %q: %w", name, err)
	}
	out.Metadata.DeleteTime = timestamppb.Now()
	return out, nil
}
