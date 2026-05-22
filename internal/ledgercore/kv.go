package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// KVStore provides namespaced key-value storage backed by the Ledger Backend.
// It replaces scattered JSON files (trust_scores.json, emotion_history.json, etc.)
// with a single, transactional SQLite-backed store.
//
// Namespaces prevent key collisions across subsystems:
//
//	kv.Put(ctx, "trust", "scores", trustData)
//	kv.Put(ctx, "emotion", "history", emotionData)
//	kv.Put(ctx, "config", "providers", providerData)
type KVStore struct {
	backend Backend
}

// Put stores a JSON-serializable value under the given namespace and key.
func (s *KVStore) Put(ctx context.Context, namespace, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("kv: marshal %s/%s: %w", namespace, key, err)
	}
	return s.backend.KVPut(ctx, &KVEntry{
		Namespace: namespace,
		Key:       key,
		Value:     data,
		UpdatedAt: time.Now(),
	})
}

// Get retrieves a value and unmarshals it into dest.
// Returns false if the key does not exist.
func (s *KVStore) Get(ctx context.Context, namespace, key string, dest any) (bool, error) {
	entry, err := s.backend.KVGet(ctx, namespace, key)
	if err != nil {
		if errors.Is(err, ErrKVNotFound) {
			return false, nil
		}
		return false, err
	}
	if err := json.Unmarshal(entry.Value, dest); err != nil {
		return true, fmt.Errorf("kv: unmarshal %s/%s: %w", namespace, key, err)
	}
	return true, nil
}

// GetRaw retrieves the raw KVEntry without unmarshaling.
func (s *KVStore) GetRaw(ctx context.Context, namespace, key string) (*KVEntry, error) {
	return s.backend.KVGet(ctx, namespace, key)
}

// PutRaw stores a raw byte value.
func (s *KVStore) PutRaw(ctx context.Context, namespace, key string, value []byte) error {
	return s.backend.KVPut(ctx, &KVEntry{
		Namespace: namespace,
		Key:       key,
		Value:     value,
		UpdatedAt: time.Now(),
	})
}

// Delete removes a key from the store.
func (s *KVStore) Delete(ctx context.Context, namespace, key string) error {
	return s.backend.KVDelete(ctx, namespace, key)
}

// List returns all entries in a namespace.
func (s *KVStore) List(ctx context.Context, namespace string) ([]*KVEntry, error) {
	return s.backend.KVList(ctx, namespace)
}

// ListKeys returns just the keys in a namespace.
func (s *KVStore) ListKeys(ctx context.Context, namespace string) ([]string, error) {
	entries, err := s.backend.KVList(ctx, namespace)
	if err != nil {
		return nil, err
	}
	keys := make([]string, len(entries))
	for i, e := range entries {
		keys[i] = e.Key
	}
	return keys, nil
}
