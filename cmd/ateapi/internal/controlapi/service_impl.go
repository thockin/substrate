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
	"context"

	"github.com/agent-substrate/substrate/cmd/ateapi/internal/store"
	"github.com/agent-substrate/substrate/internal/resources"
	"github.com/agent-substrate/substrate/pkg/proto/ateapipb"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/apimachinery/pkg/api/operation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ServiceImpl implements store.Interface and provides the "middleware" layer
// between the RPC and storage layers.
type ServiceImpl struct {
	// TODO: name this field and explicitly pass-thru each method, to prevent
	// accidental pass-thru.
	store.Interface
}

var _ store.Interface = (*ServiceImpl)(nil)

// newServiceImpl creates an instance of the service's middleware
// implementation layer. This is what implements the internal interfaces called
// by the RPC layer, and which ultimately calls the storage layer.
func newServiceImpl(
	persistence store.Interface,
) *ServiceImpl {
	s := &ServiceImpl{
		Interface: persistence,
	}
	return s
}

func (s *ServiceImpl) CreateAtespace(ctx context.Context, atespace *ateapipb.Atespace) (*ateapipb.Atespace, error) {
	if atespace == nil {
		return nil, status.Error(codes.Internal, "nil atespace")
	}
	if errs := validateAtespace(ctx, atespace); len(errs) > 0 {
		return nil, toGRPCStatusError(errs)
	}

	dbAtespace := proto.CloneOf(atespace)
	setCreateMetadata(dbAtespace.Metadata)

	return s.Interface.CreateAtespace(ctx, dbAtespace)
}

func validateAtespace(ctx context.Context, atespace *ateapipb.Atespace) field.ErrorList {
	// Call the generated validation.
	op := operation.Operation{Type: operation.Create}
	return Validate_Atespace(ctx, op, nil, atespace, nil)
}

func (s *ServiceImpl) GetAtespace(ctx context.Context, name string) (*ateapipb.Atespace, error) {
	return s.Interface.GetAtespace(ctx, name)
}

// AtespaceExists reports whether the atespace object exists. This is a plain
// EXISTS check and is NOT atomic with respect to a concurrent DeleteAtespace.
// TODO: current usage of this outside of a transaction is suspect
func (s *ServiceImpl) AtespaceExists(ctx context.Context, name string) (bool, error) {
	return s.Interface.AtespaceExists(ctx, name)
}

func (s *ServiceImpl) ListAtespaces(ctx context.Context, pageSize int32, pageTokenStr string) ([]*ateapipb.Atespace, string, error) {
	return s.Interface.ListAtespaces(ctx, pageSize, pageTokenStr)
}

// DeleteAtespace deletes an empty atespace. Returns store.ErrNotFound if the
// atespace does not exist, or store.ErrFailedPrecondition if any Actor or
// ActorSnapshotTag still lives in it.
func (s *ServiceImpl) DeleteAtespace(ctx context.Context, name string) (*ateapipb.Atespace, error) {
	return s.Interface.DeleteAtespace(ctx, name)
}

func (s *ServiceImpl) WatchWorkers(ctx context.Context) (*store.WorkerWatch, error) {
	return s.Interface.WatchWorkers(ctx)
}

// DebugClearAll flushes all data from Redis.
func (s *ServiceImpl) DebugClearAll(ctx context.Context) error {
	return s.Interface.DebugClearAll(ctx)
}

func (s *ServiceImpl) GetActor(ctx context.Context, actorRef resources.ActorRef) (*ateapipb.Actor, error) {
	return s.Interface.GetActor(ctx, actorRef)
}

func (s *ServiceImpl) CreateActor(ctx context.Context, actor *ateapipb.Actor) (*ateapipb.Actor, error) {
	//
	if actor == nil {
		return nil, status.Error(codes.Internal, "nil actor")
	}
	if errs := validateActor(ctx, actor); len(errs) > 0 {
		return nil, toGRPCStatusError(errs)
	}

	dbActor := proto.CloneOf(actor)
	setCreateMetadata(dbActor.Metadata)

	return s.Interface.CreateActor(ctx, dbActor)
}

func validateActor(ctx context.Context, actor *ateapipb.Actor) field.ErrorList {
	// Call the generated validation.
	op := operation.Operation{Type: operation.Create}
	return Validate_Actor(ctx, op, nil, actor, nil)
}

/*
func (s *ServiceImpl) CreateActorSnapshot(ctx context.Context, snapshot *ateapipb.ActorSnapshot) (*ateapipb.ActorSnapshot, error) {
	dbKey := actorSnapshotDBKey(snapshot.GetMetadata().GetAtespace(), snapshot.GetMetadata().GetName())
	dbSnapshot := proto.Clone(snapshot).(*ateapipb.ActorSnapshot)
	dbSnapshot.Metadata = newCreateMetadata(snapshot.GetMetadata().GetAtespace(), snapshot.GetMetadata().GetName())
	b, err := protojson.Marshal(dbSnapshot)
	if err != nil {
		return nil, fmt.Errorf("while marshaling actor snapshot: %w", err)
	}
	ok, err := s.rdb.SetNX(ctx, dbKey, b, 0).Result()
	if err != nil {
		return nil, fmt.Errorf("while creating actor snapshot: %w", err)
	}
	if !ok {
		return nil, store.ErrAlreadyExists
	}
	return dbSnapshot, nil
}

func (s *ServiceImpl) GetActorSnapshot(ctx context.Context, atespace, name string) (*ateapipb.ActorSnapshot, error) {
	dbKey := actorSnapshotDBKey(atespace, name)
	b, err := s.rdb.Get(ctx, dbKey).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("while getting actor snapshot key %q: %w", dbKey, err)
	}
	snapshot := &ateapipb.ActorSnapshot{}
	if err := protojson.Unmarshal(b, snapshot); err != nil {
		return nil, fmt.Errorf("while unmarshaling actor snapshot: %w", err)
	}
	return snapshot, nil
}

func (s *ServiceImpl) GetActorSnapshotByTag(ctx context.Context, atespace, name string) (*ateapipb.ActorSnapshot, *ateapipb.ActorSnapshotTag, error) {
	b, err := s.rdb.Get(ctx, actorSnapshotTagDBKey(atespace, name)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil, store.ErrNotFound
		}
		return nil, nil, fmt.Errorf("while resolving actor snapshot tag %s/%s: %w", atespace, name, err)
	}
	tag := &ateapipb.ActorSnapshotTag{}
	if err := protojson.Unmarshal(b, tag); err != nil {
		return nil, nil, fmt.Errorf("while unmarshaling actor snapshot tag %s/%s: %w", atespace, name, err)
	}
	snapshot, err := s.GetActorSnapshot(ctx, tag.GetSnapshot().GetAtespace(), tag.GetSnapshot().GetName())
	return snapshot, tag, err
}

func (s *ServiceImpl) ListActorSnapshots(ctx context.Context, atespace string, pageSize int32, pageTokenStr string) ([]*ateapipb.ActorSnapshot, string, error) {
	var result []*ateapipb.ActorSnapshot
	nextToken, err := s.listPage(ctx, actorSnapshotScanPattern(atespace), pageSize, pageTokenStr, func(ctx context.Context, master *redis.Client, keys []string) (int, error) {
		cmds, err := master.Pipelined(ctx, func(pipe redis.Pipeliner) error {
			for _, key := range keys {
				pipe.Get(ctx, key)
			}
			return nil
		})
		if err != nil && !errors.Is(err, redis.Nil) {
			return 0, fmt.Errorf("while fetching actor snapshots in shard %s: %w", master.Options().Addr, err)
		}
		collected := 0
		for _, cmd := range cmds {
			getCmd, ok := cmd.(*redis.StringCmd)
			if !ok || errors.Is(getCmd.Err(), redis.Nil) {
				continue
			}
			if getCmd.Err() != nil {
				return 0, fmt.Errorf("while getting actor snapshot: %w", getCmd.Err())
			}
			snapshot := &ateapipb.ActorSnapshot{}
			if err := protojson.Unmarshal([]byte(getCmd.Val()), snapshot); err != nil {
				return 0, fmt.Errorf("while unmarshaling actor snapshot: %w", err)
			}
			result = append(result, snapshot)
			collected++
		}
		return collected, nil
	})
	if err != nil {
		return nil, "", err
	}
	return result, nextToken, nil
}

func (s *ServiceImpl) TagActorSnapshot(ctx context.Context, atespace, name string, tag *ateapipb.ActorSnapshotTag) (*ateapipb.ActorSnapshotTag, error) {
	if _, err := s.GetActorSnapshot(ctx, atespace, name); err != nil {
		return nil, err
	}
	dbTag := proto.Clone(tag).(*ateapipb.ActorSnapshotTag)
	dbTag.Metadata = newCreateMetadata(tag.GetMetadata().GetAtespace(), tag.GetMetadata().GetName())
	dbTag.Snapshot = &ateapipb.ObjectRef{Atespace: atespace, Name: name}
	b, err := protojson.Marshal(dbTag)
	if err != nil {
		return nil, fmt.Errorf("while marshaling actor snapshot tag: %w", err)
	}
	tagKey := actorSnapshotTagDBKey(dbTag.GetMetadata().GetAtespace(), dbTag.GetMetadata().GetName())
	created, err := s.rdb.SetNX(ctx, tagKey, b, 0).Result()
	if err != nil {
		return nil, fmt.Errorf("while creating actor snapshot tag: %w", err)
	}
	if !created {
		existing, err := s.rdb.Get(ctx, tagKey).Bytes()
		if err != nil {
			return nil, fmt.Errorf("while getting actor snapshot tag: %w", err)
		}
		existingTag := &ateapipb.ActorSnapshotTag{}
		if err := protojson.Unmarshal(existing, existingTag); err != nil {
			return nil, fmt.Errorf("while unmarshaling actor snapshot tag: %w", err)
		}
		if existingTag.GetSnapshot().GetAtespace() != atespace || existingTag.GetSnapshot().GetName() != name || existingTag.GetScope() != tag.GetScope() {
			return nil, store.ErrAlreadyExists
		}
		return existingTag, nil
	}
	return dbTag, nil
}

func (s *ServiceImpl) UpdateActorSnapshotTag(ctx context.Context, atespace, name string, scope ateapipb.ActorSnapshotTagScope, expectedVersion int64) (*ateapipb.ActorSnapshotTag, error) {
	tagKey := actorSnapshotTagDBKey(atespace, name)
	var updated *ateapipb.ActorSnapshotTag
	err := s.rdb.Watch(ctx, func(tx *redis.Tx) error {
		b, err := tx.Get(ctx, tagKey).Bytes()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				return store.ErrNotFound
			}
			return err
		}
		tag := &ateapipb.ActorSnapshotTag{}
		if err := protojson.Unmarshal(b, tag); err != nil {
			return fmt.Errorf("while unmarshaling actor snapshot tag %s/%s: %w", atespace, name, err)
		}
		if tag.GetMetadata().GetVersion() != expectedVersion {
			return store.ErrVersionConflict
		}
		if tag.GetScope() == scope {
			updated = tag
			return nil
		}
		tag.Scope = scope
		tag.Metadata = newUpdateMetadata(tag.GetMetadata())
		b, err = protojson.Marshal(tag)
		if err != nil {
			return fmt.Errorf("while marshaling actor snapshot tag: %w", err)
		}
		if _, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, tagKey, b, 0)
			return nil
		}); err != nil {
			return err
		}
		updated = tag
		return nil
	}, tagKey)
	if errors.Is(err, redis.TxFailedErr) {
		return nil, store.ErrVersionConflict
	}
	if err != nil {
		return nil, fmt.Errorf("while updating actor snapshot tag: %w", err)
	}
	return updated, nil
}

func (s *ServiceImpl) DeleteActorSnapshotTag(ctx context.Context, atespace, name string) (*ateapipb.ActorSnapshotTag, error) {
	_, tag, err := s.GetActorSnapshotByTag(ctx, atespace, name)
	if err != nil {
		return nil, err
	}
	tagKey := actorSnapshotTagDBKey(atespace, name)
	if n, err := s.rdb.Del(ctx, tagKey).Result(); err != nil {
		return nil, fmt.Errorf("while deleting actor snapshot tag: %w", err)
	} else if n == 0 {
		return nil, store.ErrNotFound
	}
	return tag, nil
}

func (s *ServiceImpl) CreateWorker(ctx context.Context, worker *ateapipb.Worker) error {
	dbKey := workerDBKey(worker.GetWorkerNamespace(), worker.GetWorkerPool(), worker.GetWorkerPod())

	// Clone because we will update the version field, and we don't want to
	// stomp the caller's copy.
	dbWorker := proto.Clone(worker).(*ateapipb.Worker)
	dbWorker.Version = 1

	dbWorkerBytes, err := protojson.Marshal(dbWorker)
	if err != nil {
		return fmt.Errorf("in protojson.Marshal: %w", err)
	}

	ok, err := s.rdb.SetNX(ctx, dbKey, dbWorkerBytes, 0).Result()
	if err != nil {
		return fmt.Errorf("while executing redis set: %w", err)
	}
	if !ok {
		return store.ErrAlreadyExists
	}

	s.publishWorkerEvent(ctx, store.WorkerEventCreated, dbWorker)
	return nil
}

func (s *ServiceImpl) GetWorker(ctx context.Context, namespace, pool, pod string) (*ateapipb.Worker, error) {
	dbKey := workerDBKey(namespace, pool, pod)

	dbWorkerBytes, err := s.rdb.Get(ctx, dbKey).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("while getting worker key %q: %w", dbKey, err)
	}

	worker := &ateapipb.Worker{}
	if err := protojson.Unmarshal(dbWorkerBytes, worker); err != nil {
		return nil, fmt.Errorf("in protojson.Unmarshal: %w", err)
	}

	if worker.GetWorkerNamespace() != namespace || worker.GetWorkerPool() != pool || worker.GetWorkerPod() != pod {
		return nil, fmt.Errorf("(impossible) mismatch between stored namespace/pool/pod and key")
	}

	return worker, nil
}

func (s *ServiceImpl) UpdateWorker(ctx context.Context, worker *ateapipb.Worker, expectedVersion int64) error {
	dbKey := workerDBKey(worker.GetWorkerNamespace(), worker.GetWorkerPool(), worker.GetWorkerPod())

	// Clone because we will update the version field, and we don't want to
	// stomp the caller's copy.
	dbWorker := proto.Clone(worker).(*ateapipb.Worker)

	err := s.rdb.Watch(ctx, func(tx *redis.Tx) error {
		currentVal, err := tx.Get(ctx, dbKey).Bytes()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				return store.ErrNotFound
			}
			return fmt.Errorf("while getting worker: %w", err)
		}

		currentWorker := &ateapipb.Worker{}
		if err := protojson.Unmarshal(currentVal, currentWorker); err != nil {
			return fmt.Errorf("in protojson.Unmarshal: %w", err)
		}

		if currentWorker.GetVersion() != expectedVersion {
			return store.ErrVersionConflict
		}
		dbWorker.Version = currentWorker.GetVersion() + 1
		if currentWorker.GetWorkerNamespace() != dbWorker.GetWorkerNamespace() {
			return fmt.Errorf("worker_namespace is immutable")
		}
		if currentWorker.GetWorkerPool() != dbWorker.GetWorkerPool() {
			return fmt.Errorf("worker_pool is immutable")
		}
		if currentWorker.GetWorkerPod() != dbWorker.GetWorkerPod() {
			return fmt.Errorf("worker_pod is immutable")
		}
		if currentWorker.GetIp() != dbWorker.GetIp() {
			return fmt.Errorf("ip is immutable")
		}

		newVal, err := protojson.Marshal(dbWorker)
		if err != nil {
			return fmt.Errorf("in protojson.Marshal: %w", err)
		}

		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, dbKey, newVal, 0)
			return nil
		})
		return err
	}, dbKey)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.ErrNotFound
		}
		if errors.Is(err, store.ErrVersionConflict) || errors.Is(err, redis.TxFailedErr) {
			return store.ErrVersionConflict
		}
		return fmt.Errorf("while executing update worker transaction: %w", err)
	}

	s.publishWorkerEvent(ctx, store.WorkerEventUpdated, dbWorker)
	return nil
}

func (s *ServiceImpl) DeleteWorker(ctx context.Context, namespace, pool, pod string) error {
	dbKey := workerDBKey(namespace, pool, pod)
	err := s.rdb.Del(ctx, dbKey).Err()
	if err != nil {
		return fmt.Errorf("while deleting worker key %q: %w", dbKey, err)
	}
	s.publishWorkerEvent(ctx, store.WorkerEventDeleted, &ateapipb.Worker{
		WorkerNamespace: namespace,
		WorkerPod:       pod,
	})
	return nil
}

func (s *ServiceImpl) DeleteActor(ctx context.Context, actorRef resources.ActorRef) (*ateapipb.Actor, error) {
	dbKey := actorDBKey(actorRef)
	var deleted *ateapipb.Actor
	err := s.rdb.Watch(ctx, func(tx *redis.Tx) error {
		currentVal, err := tx.Get(ctx, dbKey).Bytes()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				return store.ErrNotFound
			}
			return fmt.Errorf("while getting actor: %w", err)
		}

		currentActor := &ateapipb.Actor{}
		if err := protojson.Unmarshal(currentVal, currentActor); err != nil {
			return fmt.Errorf("in protojson.Unmarshal: %w", err)
		}

		if currentActor.GetStatus() != ateapipb.Actor_STATUS_DELETING {
			return store.ErrFailedPrecondition
		}

		if _, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Del(ctx, dbKey)
			return nil
		}); err != nil {
			return err
		}
		deleted = currentActor
		return nil
	}, dbKey)

	if err != nil {
		if errors.Is(err, redis.TxFailedErr) {
			return nil, store.ErrVersionConflict
		}
		return nil, err
	}

	return deleted, nil
}

// validateUpdateActorMutation reports whether an actor mutation left the fields it does
// not own alone.
func validateUpdateActorMutation(storedActor, mutatedActor *ateapipb.Actor) error {
	if stored, mutated := storedActor.GetMetadata().GetAtespace(), mutatedActor.GetMetadata().GetAtespace(); stored != mutated {
		return fmt.Errorf("metadata.atespace is immutable: mutation changed it from %q to %q", stored, mutated)
	}
	if stored, mutated := storedActor.GetMetadata().GetName(), mutatedActor.GetMetadata().GetName(); stored != mutated {
		return fmt.Errorf("metadata.name is immutable: mutation changed it from %q to %q", stored, mutated)
	}
	if stored, mutated := storedActor.GetActorTemplateNamespace(), mutatedActor.GetActorTemplateNamespace(); stored != mutated {
		return fmt.Errorf("actor_template_namespace is immutable: mutation changed it from %q to %q", stored, mutated)
	}
	if stored, mutated := storedActor.GetActorTemplateName(), mutatedActor.GetActorTemplateName(); stored != mutated {
		return fmt.Errorf("actor_template_name is immutable: mutation changed it from %q to %q", stored, mutated)
	}
	return nil
}

// updateActorMaxAttempts bounds how many times UpdateActor re-runs its
// read-modify-write after a concurrent writer invalidates the transaction.
const updateActorMaxAttempts = 5

func (s *ServiceImpl) UpdateActor(ctx context.Context, actorRef resources.ActorRef, mutate func(*ateapipb.Actor) error) (*ateapipb.Actor, error) {
	dbKey := actorDBKey(actorRef)
	for range updateActorMaxAttempts {
		var dbActor *ateapipb.Actor
		var abortErr error

		err := s.rdb.Watch(ctx, func(tx *redis.Tx) error {
			currentVal, err := tx.Get(ctx, dbKey).Bytes()
			if err != nil {
				if errors.Is(err, redis.Nil) {
					return store.ErrNotFound
				}
				return fmt.Errorf("while getting actor: %w", err)
			}

			currentActor := &ateapipb.Actor{}
			if err := protojson.Unmarshal(currentVal, currentActor); err != nil {
				return fmt.Errorf("in protojson.Unmarshal: %w", err)
			}

			// Snapshot the stored state before handing the actor to mutate.
			// mutate is free to edit anything it is given.
			actorBeforeMutation := proto.Clone(currentActor).(*ateapipb.Actor)
			if err := mutate(currentActor); err != nil {
				abortErr = err
				return err
			}
			if err := validateUpdateActorMutation(actorBeforeMutation, currentActor); err != nil {
				abortErr = err
				return err
			}
			// The stored metadata is authoritative; derive the next metadata
			// from it, discarding whatever mutate made of it.
			currentActor.Metadata = newUpdateMetadata(actorBeforeMutation.GetMetadata())

			newVal, err := protojson.Marshal(currentActor)
			if err != nil {
				return fmt.Errorf("in protojson.Marshal: %w", err)
			}

			if _, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, dbKey, newVal, 0)
				return nil
			}); err != nil {
				return err
			}
			dbActor = currentActor
			return nil
		}, dbKey)

		switch {
		case err == nil:
			return dbActor, nil
		case abortErr != nil:
			return nil, abortErr
		case errors.Is(err, store.ErrNotFound):
			return nil, store.ErrNotFound
		case errors.Is(err, redis.TxFailedErr):
			// A concurrent write landed between WATCH and EXEC, so mutate never
			// saw it. Re-read and run it against the newer state.
			continue
		default:
			return nil, fmt.Errorf("while executing update actor transaction: %w", err)
		}
	}

	// Only the TxFailedErr branch continues the loop, so getting here means every
	// attempt lost the race.
	return nil, store.ErrVersionConflict
}

func (s *ServiceImpl) ListWorkers(ctx context.Context, pageSize int32, pageTokenStr string) ([]*ateapipb.Worker, string, error) {
	var result []*ateapipb.Worker
	nextToken, err := s.listPage(ctx, "worker:*", pageSize, pageTokenStr, func(ctx context.Context, master *redis.Client, keys []string) (int, error) {
		workers, err := fetchProtos(ctx, master, keys, func() *ateapipb.Worker { return &ateapipb.Worker{} })
		if err != nil {
			return 0, err
		}
		result = append(result, workers...)
		return len(workers), nil
	})
	if err != nil {
		return nil, "", err
	}
	return result, nextToken, nil
}

type pageToken struct {
	ShardHash string `json:"shard_hash"`
	Cursor    uint64 `json:"cursor"`
}

func encodePageToken(token pageToken) string {
	b, _ := json.Marshal(token)
	return base64.StdEncoding.EncodeToString(b)
}

func decodePageToken(tokenStr string) (pageToken, error) {
	var token pageToken
	if tokenStr == "" {
		return token, nil
	}
	b, err := base64.StdEncoding.DecodeString(tokenStr)
	if err != nil {
		return token, err
	}
	err = json.Unmarshal(b, &token)
	return token, err
}

func hashShardAddr(addr string) string {
	h := sha256.Sum256([]byte(addr))
	return hex.EncodeToString(h[:])
}

// ListActors lists actors, scoped to the given atespace. An empty atespace lists
// across all atespaces (SCAN actor:*); a non-empty atespace restricts the scan to
// that atespace (SCAN actor:<atespace>:*).
func (s *ServiceImpl) ListActors(ctx context.Context, atespace string, pageSize int32, pageTokenStr string) ([]*ateapipb.Actor, string, error) {
	var result []*ateapipb.Actor
	nextToken, err := s.listPage(ctx, actorScanPattern(atespace), pageSize, pageTokenStr, func(ctx context.Context, master *redis.Client, keys []string) (int, error) {
		actors, err := fetchProtos(ctx, master, keys, func() *ateapipb.Actor { return &ateapipb.Actor{} })
		if err != nil {
			return 0, err
		}
		result = append(result, actors...)
		return len(actors), nil
	})
	if err != nil {
		return nil, "", err
	}
	return result, nextToken, nil
}

// listPage SCANs pattern across the redis masters from the page token, feeding key batches to collect and returns the next-page token.
func (s *ServiceImpl) listPage(ctx context.Context, pattern string, pageSize int32, pageTokenStr string, collect func(ctx context.Context, master *redis.Client, keys []string) (int, error)) (string, error) {
	token, err := decodePageToken(pageTokenStr)
	if err != nil {
		return "", fmt.Errorf("invalid page token: %w", err)
	}

	masters, err := s.getSortedMasters(ctx)
	if err != nil {
		return "", err
	}

	startIndex, err := findStartingShard(masters, token.ShardHash)
	if err != nil {
		return "", err
	}

	i := startIndex
	cursor := token.Cursor
	collected := 0

	for i < len(masters) && collected < int(pageSize) {
		master := masters[i]
		remaining := int(pageSize) - collected

		var keys []string
		keys, cursor, err = master.Scan(ctx, cursor, pattern, int64(remaining)).Result()
		if err != nil {
			return "", fmt.Errorf("while scanning shard %s: %w", master.Options().Addr, err)
		}

		if len(keys) > 0 {
			n, err := collect(ctx, master, keys)
			if err != nil {
				return "", err
			}
			collected += n
		}

		if cursor == 0 {
			i++
		}
	}

	var nextToken string
	if i < len(masters) {
		nextToken = encodePageToken(pageToken{
			ShardHash: hashShardAddr(masters[i].Options().Addr),
			Cursor:    cursor,
		})
	}

	return nextToken, nil
}

func (s *ServiceImpl) getSortedMasters(ctx context.Context) ([]*redis.Client, error) {
	var mu sync.Mutex
	var masters []*redis.Client
	// ForEachMaster invokes the callback concurrently, one goroutine per master.
	err := s.rdb.ForEachMaster(ctx, func(ctx context.Context, master *redis.Client) error {
		mu.Lock()
		defer mu.Unlock()
		masters = append(masters, master)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("while listing redis masters: %w", err)
	}

	sort.Slice(masters, func(i, j int) bool {
		return masters[i].Options().Addr < masters[j].Options().Addr
	})
	return masters, nil
}

func findStartingShard(masters []*redis.Client, shardHash string) (int, error) {
	if shardHash == "" {
		return 0, nil
	}
	for i, m := range masters {
		if hashShardAddr(m.Options().Addr) == shardHash {
			return i, nil
		}
	}
	return 0, fmt.Errorf("topology changed: shard with hash %s not found (aborted)", shardHash)
}

// fetchProtos fetches keys into newMsg-created messages.
func fetchProtos[M proto.Message](ctx context.Context, master *redis.Client, keys []string, newMsg func() M) ([]M, error) {
	cmds, err := master.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, key := range keys {
			pipe.Get(ctx, key)
		}
		return nil
	})
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("while fetching keys in shard %s: %w", master.Options().Addr, err)
	}

	var out []M
	for _, cmd := range cmds {
		getCmd, ok := cmd.(*redis.StringCmd)
		if !ok {
			continue
		}
		if getCmd.Err() != nil {
			if errors.Is(getCmd.Err(), redis.Nil) {
				continue
			}
			return nil, fmt.Errorf("while getting key: %w", getCmd.Err())
		}

		msg := newMsg()
		if err := protojson.Unmarshal([]byte(getCmd.Val()), msg); err != nil {
			return nil, fmt.Errorf("in protojson.Unmarshal: %w", err)
		}
		out = append(out, msg)
	}
	return out, nil
}

// lockRenewScript extends key's TTL only if it is still owned by ARGV[1],
// atomically. Returns 1 if renewed, 0 if the lock was lost (expired and
// possibly reacquired by someone else, or otherwise deleted).
var lockRenewScript = redis.NewScript(`
	if redis.call("get", KEYS[1]) == ARGV[1] then
		return redis.call("pexpire", KEYS[1], ARGV[2])
	else
		return 0
	end
`)

// lockReleaseScript deletes key only if it is still owned by ARGV[1],
// atomically, so a caller can never release a lock it no longer holds.
var lockReleaseScript = redis.NewScript(`
	if redis.call("get", KEYS[1]) == ARGV[1] then
		return redis.call("del", KEYS[1])
	else
		return 0
	end
`)

// defaultLockTTL is how long a lock may go unrenewed before another client
// can reclaim it.
const defaultLockTTL = 30 * time.Second

func (s *ServiceImpl) AcquireLock(ctx context.Context, key string) (*store.Lock, error) {
	ttl := s.lockTTL
	value := uuid.New().String()

	ok, err := s.rdb.SetNX(ctx, key, value, ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("while acquiring lock for %q: %w", key, err)
	}
	if !ok {
		return nil, store.ErrLockConflict
	}

	// leaseCtx is cancelled either by Close, or by the renewal loop below if it
	// ever stops without Close having been called (i.e. the lease was lost).
	leaseCtx, cancel := context.WithCancel(ctx)
	renewalDone := make(chan struct{})

	go func() {
		defer close(renewalDone)
		defer cancel()
		s.renewLockLoop(leaseCtx, key, value, ttl)
	}()

	closeFn := func() {
		cancel()
		<-renewalDone // wait for the renewal loop to stop before releasing.

		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer releaseCancel()
		if err := s.releaseLock(releaseCtx, key, value); err != nil {
			slog.WarnContext(releaseCtx, "failed to release lock, relying on TTL to reclaim it", "key", key, "error", err)
		}
	}

	return store.NewLock(leaseCtx, closeFn), nil
}

const (
	// renewIntervalDivisor and renewRetryPeriodDivisor set the renewal loop's
	// steady-state cadence and in-failure retry spacing as fractions of ttl:
	// interval = ttl/renewIntervalDivisor, retryPeriod = ttl/renewRetryPeriodDivisor.
	renewIntervalDivisor    = 3
	renewRetryPeriodDivisor = 10
	// renewDeadlineFraction bounds how much of the lock's TTL the renewal loop
	// may spend retrying after its last successful renewal before conceding the
	// lease as lost.
	renewDeadlineFraction = 2.0 / 3.0
)

func (s *ServiceImpl) renewLockLoop(ctx context.Context, key, value string, ttl time.Duration) {
	interval := ttl / renewIntervalDivisor
	renewDeadline := time.Duration(float64(ttl) * renewDeadlineFraction)

	lastRenewed := time.Now()
	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			renewCtx, cancel := context.WithDeadline(ctx, lastRenewed.Add(renewDeadline))
			renewed := s.tryRenewLock(renewCtx, key, value, ttl)
			cancel()
			if !renewed {
				return
			}
			lastRenewed = time.Now()
			timer.Reset(interval)
		}
	}
}

func (s *ServiceImpl) tryRenewLock(ctx context.Context, key, value string, ttl time.Duration) bool {
	retryPeriod := ttl / renewRetryPeriodDivisor

	retry := time.NewTimer(0) // first attempt fires immediately.
	defer retry.Stop()

	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				slog.WarnContext(ctx, "failed to renew lock and its renew deadline has elapsed, treating lease as lost", "key", key)
			}
			return false

		case <-retry.C:
			renewed, err := s.renewLock(ctx, key, value, ttl)

			if ctx.Err() != nil {
				return false // deadline elapsed or Close raced with this attempt.
			}

			switch {
			case err == nil && renewed:
				return true

			case err == nil && !renewed:
				slog.WarnContext(ctx, "lock renewal found lease no longer owned", "key", key)
				return false

			default:
				slog.WarnContext(ctx, "failed to renew lock, retrying before its renew deadline elapses", "key", key, "error", err)
				retry.Reset(retryPeriod)
			}
		}
	}
}

func (s *ServiceImpl) renewLock(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	res, err := lockRenewScript.Run(ctx, s.rdb, []string{key}, value, ttl.Milliseconds()).Result()
	if err != nil {
		return false, fmt.Errorf("while renewing lock for %q: %w", key, err)
	}
	renewed, _ := res.(int64)
	return renewed == 1, nil
}

func (s *ServiceImpl) releaseLock(ctx context.Context, key, value string) error {
	_, err := lockReleaseScript.Run(ctx, s.rdb, []string{key}, value).Result()
	if err != nil {
		return fmt.Errorf("while releasing lock for %q with value %q: %w", key, value, err)
	}
	return nil
}
*/

func setCreateMetadata(meta *ateapipb.ResourceMetadata) {
	now := timestamppb.Now()
	meta.Uid = uuid.NewString()
	meta.Version = 1
	meta.CreateTime = now
	meta.UpdateTime = now
}

/*
func newUpdateMetadata(current *ateapipb.ResourceMetadata) *ateapipb.ResourceMetadata {
	next := proto.Clone(current).(*ateapipb.ResourceMetadata)
	next.Version = current.GetVersion() + 1
	next.UpdateTime = timestamppb.Now()
	return next
}
*/
