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
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"go.ate.dev/substrate/api/ateapi/v1"
	"go.ate.dev/substrate/cmd/ateapi/internal/store"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// resource is a stored resource message: a proto with a metadata field.
type resource interface {
	proto.Message
	GetMetadata() *ateapi.ResourceMetadata
}

// rowUpdate describes a read-modify-write of one resource row, for updateRow.
type rowUpdate[T resource] struct {
	// what names the row in errors, like "actor team-a/a".
	what string
	// table is the table, and where selects the row by its key, with the key
	// values in key as $1, $2, and so on.
	table string
	where string
	key   []any
	// precondition is checked against the stored row before mutate runs.
	precondition store.DeletePreconditions
	// mutate edits the stored object in place, and may be nil.
	mutate func(T) error
	// validate, if set, checks the mutated object before it is written.
	validate func(mutated T) error
	// markForDeletion sets metadata.delete_time if it is not already set.
	markForDeletion bool
}

// updateRow reads the row u addresses through q, checks u.precondition, runs
// u.mutate and the validation hooks, and writes the result back at the next version. It
// returns the object as written.
//
// The read takes no lock. The write is guarded on the uid and version that were
// read, so if another write lands in between, nothing is written and updateRow
// returns ErrVersionConflict. Under READ COMMITTED a guarded UPDATE that waits
// on a concurrent writer's row lock re-checks its WHERE clause against the row
// that writer committed, so the guard cannot pass on stale data. q may be a
// transaction, in which case the write commits or rolls back with it.
//
// mutate runs at most once. Whatever it does to the metadata is discarded: the
// written metadata is nextMetadata of the stored metadata.
//
// Returns ErrNotFound if the row is missing, the precondition's error if it
// fails, the error of mutate or a validation hook verbatim, or ErrVersionConflict if the
// row changed between the read and the write.
func updateRow[T resource](ctx context.Context, q querier, u rowUpdate[T]) (T, error) {
	var zero T
	var uid string
	var version int64
	var protoBytes []byte
	if err := q.QueryRow(ctx, `SELECT uid, version, proto FROM `+u.table+` WHERE `+u.where, u.key...).Scan(&uid, &version, &protoBytes); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return zero, store.ErrNotFound
		}
		return zero, fmt.Errorf("getting %s for update: %w", u.what, err)
	}
	obj := zero.ProtoReflect().Type().New().Interface().(T)
	if err := unmarshalStored(protoBytes, obj); err != nil {
		return zero, fmt.Errorf("unmarshaling %s for update: %w", u.what, err)
	}
	if err := validateProtoMetadataMatchesColumns(u.what, obj.GetMetadata(), uid, version); err != nil {
		return zero, err
	}
	if err := u.precondition.Check(obj.GetMetadata()); err != nil {
		return zero, err
	}
	storedMeta := proto.CloneOf(obj.GetMetadata())
	if u.mutate != nil {
		if err := u.mutate(obj); err != nil {
			return zero, err
		}
	}
	if u.validate != nil {
		if err := u.validate(obj); err != nil {
			return zero, err
		}
	}
	next := setUpdateMetadata(storedMeta, u.markForDeletion)
	setMetadata(obj, next)

	updatedBytes, err := proto.Marshal(obj)
	if err != nil {
		return zero, fmt.Errorf("marshaling %s: %w", u.what, err)
	}
	n := len(u.key)
	tag, err := q.Exec(ctx,
		fmt.Sprintf(`UPDATE %s SET version = $%d, proto = $%d WHERE %s AND uid = $%d AND version = $%d`, u.table, n+1, n+2, u.where, n+3, n+4),
		append(append([]any{}, u.key...), next.GetVersion(), updatedBytes, uid, version)...)
	if err != nil {
		return zero, fmt.Errorf("updating %s: %w", u.what, err)
	}
	switch tag.RowsAffected() {
	case 1:
		return obj, nil
	case 0:
		return zero, store.ErrVersionConflict
	default:
		return zero, fmt.Errorf("updating %s affected %d rows, want 1", u.what, tag.RowsAffected())
	}
}

// setMetadata replaces the metadata field of m.
func setMetadata(m resource, md *ateapi.ResourceMetadata) {
	r := m.ProtoReflect()
	r.Set(r.Descriptor().Fields().ByName("metadata"), protoreflect.ValueOfMessage(md.ProtoReflect()))
}

// rowDelete describes a guarded delete of one resource row, for deleteRow.
type rowDelete struct {
	// what names the row in errors, like "tag team-a/v1".
	what string
	// table is the table, and where selects the row by its key, with the key
	// values in key as $1, $2, and so on.
	table string
	where string
	key   []any
	// precondition is checked against the row in the same statement that
	// deletes it. Its zero value deletes whatever row holds the key.
	precondition store.DeletePreconditions
}

// deleteRow deletes the row d addresses if it passes d.precondition, and
// returns the object as it was stored.
//
// The check and the delete are one statement, so they see the same row. If
// nothing was deleted, a second read finds out why: ErrNotFound if there is no
// row, the precondition's error if the row fails it, or ErrVersionConflict if
// the row passes it now, which means the row changed between the statements.
// An error from the DELETE itself, such as a foreign key violation, is
// returned wrapped, so callers can still match it.
func deleteRow[T resource](ctx context.Context, q querier, d rowDelete) (T, error) {
	var zero T
	n := len(d.key)
	var protoBytes []byte
	err := q.QueryRow(ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE %s AND ($%d::text = '' OR uid = $%d::text) AND ($%d::bigint = 0 OR version = $%d::bigint) RETURNING proto`,
			d.table, d.where, n+1, n+1, n+2, n+2),
		append(append([]any{}, d.key...), d.precondition.UID, d.precondition.Version)...).Scan(&protoBytes)
	if errors.Is(err, pgx.ErrNoRows) {
		var uid string
		var version int64
		err := q.QueryRow(ctx, `SELECT uid, version FROM `+d.table+` WHERE `+d.where, d.key...).Scan(&uid, &version)
		if errors.Is(err, pgx.ErrNoRows) {
			return zero, store.ErrNotFound
		}
		if err != nil {
			return zero, fmt.Errorf("reading %s after a guarded delete matched nothing: %w", d.what, err)
		}
		if err := d.precondition.Check(&ateapi.ResourceMetadata{Uid: uid, Version: version}); err != nil {
			return zero, err
		}
		return zero, store.ErrVersionConflict
	}
	if err != nil {
		return zero, fmt.Errorf("deleting %s: %w", d.what, err)
	}
	obj := zero.ProtoReflect().Type().New().Interface().(T)
	if err := unmarshalStored(protoBytes, obj); err != nil {
		return zero, fmt.Errorf("unmarshaling deleted %s: %w", d.what, err)
	}
	// The row is already gone, so the uid and version columns are not checked
	// against the proto: failing now would report an error for a delete that
	// happened.
	return obj, nil
}

// getRow runs query, which selects the uid, version and proto columns of at
// most one row, and returns the decoded object. It checks that the uid and
// version columns match the proto. what names the row in errors, like "actor
// team-a/a".
//
// Returns ErrNotFound if query selects no row.
func getRow[T resource](ctx context.Context, q querier, what, query string, args ...any) (T, error) {
	var zero T
	var uid string
	var version int64
	var protoBytes []byte
	if err := q.QueryRow(ctx, query, args...).Scan(&uid, &version, &protoBytes); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return zero, store.ErrNotFound
		}
		return zero, fmt.Errorf("getting %s: %w", what, err)
	}
	obj := zero.ProtoReflect().Type().New().Interface().(T)
	if err := unmarshalStored(protoBytes, obj); err != nil {
		return zero, fmt.Errorf("unmarshaling %s: %w", what, err)
	}
	if err := validateProtoMetadataMatchesColumns(what, obj.GetMetadata(), uid, version); err != nil {
		return zero, err
	}
	return obj, nil
}

// rowInsert describes the insert of a new resource row, for createRow.
type rowInsert struct {
	// what names the row in errors, like "actor team-a/a".
	what string
	// table is the table, cols lists its key columns, like "atespace, name",
	// and key holds their values in the same order.
	table string
	cols  string
	key   []any
}

// createRow stores a copy of in as a new row and returns the copy. The copy's
// metadata keeps in's name and atespace, and gets a new uid, version 1, and
// create and update times of now. in is not modified.
//
// Returns ErrAlreadyExists if a row holds the key, or ErrFailedPrecondition if
// a foreign key fails, which means the row's parent, such as its atespace, does
// not exist. The foreign key also covers a parent deleted concurrently with a
// caller's own existence check, which a separate read could not.
func createRow[T resource](ctx context.Context, q querier, ins rowInsert, in T) (T, error) {
	var zero T
	obj := proto.CloneOf(in)
	md := newMetadata(in.GetMetadata())
	setMetadata(obj, md)
	protoBytes, err := proto.Marshal(obj)
	if err != nil {
		return zero, fmt.Errorf("marshaling %s: %w", ins.what, err)
	}
	n := len(ins.key)
	placeholders := ""
	for i := range n + 3 {
		if i > 0 {
			placeholders += ", "
		}
		placeholders += fmt.Sprintf("$%d", i+1)
	}
	_, err = q.Exec(ctx,
		fmt.Sprintf(`INSERT INTO %s (%s, uid, version, proto) VALUES (%s)`, ins.table, ins.cols, placeholders),
		append(append([]any{}, ins.key...), md.GetUid(), md.GetVersion(), protoBytes)...)
	switch {
	case err == nil:
		return obj, nil
	case isUniqueViolation(err):
		return zero, store.ErrAlreadyExists
	case isForeignKeyViolation(err):
		return zero, store.ErrFailedPrecondition
	default:
		return zero, fmt.Errorf("inserting %s: %w", ins.what, err)
	}
}
