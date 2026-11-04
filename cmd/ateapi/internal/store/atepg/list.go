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

package atepg

import (
	"context"
	"fmt"
	"strings"

	"go.ate.dev/substrate/cmd/ateapi/internal/store"
	"google.golang.org/protobuf/proto"
)

// listQuery describes one keyset-paginated listing.
type listQuery struct {
	// what names the listing in errors, like "actors in team-a", and item
	// names one row, like "actor".
	what, item string
	// kind and scope bind page tokens to this listing, so a token cannot be
	// replayed against another one.
	kind  resourceKind
	scope string
	// idParts is how many text columns name each row. sql selects them and
	// then the proto.
	idParts int
	// keyParts is how many of the trailing id columns form the sort key. The
	// key must be unique within the listing, so that "after the last key"
	// resumes exactly where the previous page stopped.
	keyParts int
	// sql is ordered by the key. It takes args, then keyParts nullable cursor
	// values (all NULL for the first page), then the row limit.
	sql  string
	args []any
}

// listPage returns the page of lq that opts addresses. decode turns a stored
// proto into an item.
//
// Each page is one statement, so a page is a consistent snapshot. A listing
// over several pages is not: rows written between pages show up or not
// depending on where they sort relative to the cursor.
func listPage[T proto.Message](ctx context.Context, q querier, lq listQuery, opts store.ListOptions, decode func([]byte) (T, error)) (store.ListResponse[T], error) {
	opts, err := store.NormalizeListOptions(opts)
	if err != nil {
		return store.ListResponse[T]{}, err
	}
	token, err := decodePageToken(opts.PageToken, lq.kind, lq.scope, lq.keyParts)
	if err != nil {
		return store.ListResponse[T]{}, err
	}
	args := append([]any{}, lq.args...)
	for i := range lq.keyParts {
		var last *string
		if len(token.Last) > 0 {
			last = &token.Last[i]
		}
		args = append(args, last)
	}
	// One extra row tells whether another page follows.
	args = append(args, int64(opts.PageSize)+1)

	rows, err := q.Query(ctx, lq.sql, args...)
	if err != nil {
		return store.ListResponse[T]{}, fmt.Errorf("listing %s: %w", lq.what, err)
	}
	defer rows.Close()

	var items []T
	var lastKey []string
	for rows.Next() {
		if len(items) == int(opts.PageSize) {
			return store.ListResponse[T]{
				Items:         items,
				NextPageToken: encodePageToken(lq.kind, lq.scope, lastKey),
			}, nil
		}
		id := make([]string, lq.idParts)
		var protoBytes []byte
		dest := make([]any, 0, lq.idParts+1)
		for i := range id {
			dest = append(dest, &id[i])
		}
		dest = append(dest, &protoBytes)
		if err := rows.Scan(dest...); err != nil {
			return store.ListResponse[T]{}, fmt.Errorf("scanning %s: %w", lq.what, err)
		}
		item, err := decode(protoBytes)
		if err != nil {
			return store.ListResponse[T]{}, fmt.Errorf("unmarshaling %s %s: %w", lq.item, strings.Join(id, "/"), err)
		}
		items = append(items, item)
		lastKey = id[lq.idParts-lq.keyParts:]
	}
	if err := rows.Err(); err != nil {
		return store.ListResponse[T]{}, fmt.Errorf("listing %s: %w", lq.what, err)
	}
	return store.ListResponse[T]{Items: items}, nil
}

// decodeStored returns a decoder for listPage that decodes resource rows with
// unmarshalStored.
func decodeStored[T proto.Message]() func([]byte) (T, error) {
	return func(b []byte) (T, error) {
		var zero T
		m := zero.ProtoReflect().Type().New().Interface().(T)
		if err := unmarshalStored(b, m); err != nil {
			return zero, err
		}
		return m, nil
	}
}
