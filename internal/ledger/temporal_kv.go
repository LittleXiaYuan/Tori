package ledger

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	baseledger "github.com/LittleXiaYuan/ledger"
)

const temporalKVHistoryNamespace = "__kv_history__"

// TemporalKVVersion is one materialized historical KV value.
type TemporalKVVersion struct {
	Namespace  string    `json:"namespace"`
	Key        string    `json:"key"`
	Version    int       `json:"version"`
	Value      []byte    `json:"value"`
	UpdatedAt  time.Time `json:"updated_at"`
	ArchivedAt time.Time `json:"archived_at,omitempty"`
	Current    bool      `json:"current,omitempty"`
}

type temporalKVHistoryDocument struct {
	Namespace string              `json:"namespace"`
	Key       string              `json:"key"`
	Versions  []TemporalKVVersion `json:"versions"`
}

// TemporalKVStore adds versioned-write and point-in-time read helpers on top of
// Ledger KV without forcing every existing KV caller to change at once.
//
// The first slice stores the kv_history stream in a reserved Ledger KV namespace
// so yunque-agent can wire Memory Time Travel immediately while the lower-level
// Ledger module can still evolve toward a native kv_history table later.
type TemporalKVStore struct {
	ldg *baseledger.Ledger
	now func() time.Time
}

// TemporalKVOption configures a TemporalKVStore.
type TemporalKVOption func(*TemporalKVStore)

// WithTemporalKVNow overrides the clock, primarily for deterministic tests.
func WithTemporalKVNow(now func() time.Time) TemporalKVOption {
	return func(s *TemporalKVStore) {
		if now != nil {
			s.now = now
		}
	}
}

// NewTemporalKVStore creates a versioned KV facade for a Ledger instance.
func NewTemporalKVStore(ldg *baseledger.Ledger, opts ...TemporalKVOption) *TemporalKVStore {
	s := &TemporalKVStore{ldg: ldg, now: func() time.Time { return time.Now().UTC() }}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// PutVersioned stores a JSON value and archives the previous value if one exists.
func (s *TemporalKVStore) PutVersioned(ctx context.Context, namespace, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("temporal kv marshal %s/%s: %w", namespace, key, err)
	}
	return s.PutRawVersioned(ctx, namespace, key, data)
}

// PutRawVersioned stores a raw value using the store clock.
func (s *TemporalKVStore) PutRawVersioned(ctx context.Context, namespace, key string, value []byte) error {
	return s.PutRawVersionedAt(ctx, namespace, key, value, s.now())
}

// PutRawVersionedAt stores a raw value at a deterministic logical time and
// archives the previous current value into kv_history first.
func (s *TemporalKVStore) PutRawVersionedAt(ctx context.Context, namespace, key string, value []byte, updatedAt time.Time) error {
	if s == nil || s.ldg == nil {
		return fmt.Errorf("temporal kv: ledger is required")
	}
	namespace = strings.TrimSpace(namespace)
	key = strings.TrimSpace(key)
	if namespace == "" || key == "" {
		return fmt.Errorf("temporal kv: namespace and key are required")
	}
	if updatedAt.IsZero() {
		updatedAt = s.now()
	}
	updatedAt = updatedAt.UTC()

	current, err := s.ldg.KV.GetRaw(ctx, namespace, key)
	if err != nil && !errors.Is(err, baseledger.ErrKVNotFound) {
		return err
	}
	if current != nil && !bytes.Equal(current.Value, value) {
		doc, err := s.loadHistory(ctx, namespace, key)
		if err != nil {
			return err
		}
		doc.Versions = append(doc.Versions, TemporalKVVersion{
			Namespace:  namespace,
			Key:        key,
			Version:    len(doc.Versions) + 1,
			Value:      append([]byte(nil), current.Value...),
			UpdatedAt:  current.UpdatedAt.UTC(),
			ArchivedAt: updatedAt,
		})
		if err := s.saveHistory(ctx, doc); err != nil {
			return err
		}
	}

	return s.ldg.Backend().KVPut(ctx, &baseledger.KVEntry{
		Namespace: namespace,
		Key:       key,
		Value:     append([]byte(nil), value...),
		UpdatedAt: updatedAt,
	})
}

// GetRawAt returns the value that was current at or before the requested time.
func (s *TemporalKVStore) GetRawAt(ctx context.Context, namespace, key string, at time.Time) (*baseledger.KVEntry, bool, error) {
	if s == nil || s.ldg == nil {
		return nil, false, fmt.Errorf("temporal kv: ledger is required")
	}
	if at.IsZero() {
		at = s.now()
	}
	at = at.UTC()

	current, err := s.ldg.KV.GetRaw(ctx, namespace, key)
	if err != nil && !errors.Is(err, baseledger.ErrKVNotFound) {
		return nil, false, err
	}
	if current != nil && !current.UpdatedAt.After(at) {
		return current, true, nil
	}

	versions, err := s.ListVersions(ctx, namespace, key, 0)
	if err != nil {
		return nil, false, err
	}
	for _, version := range versions {
		if version.Current || version.UpdatedAt.After(at) {
			continue
		}
		return &baseledger.KVEntry{
			Namespace: version.Namespace,
			Key:       version.Key,
			Value:     append([]byte(nil), version.Value...),
			UpdatedAt: version.UpdatedAt,
		}, true, nil
	}
	return nil, false, nil
}

// ListVersions returns current + historical versions, newest first.
func (s *TemporalKVStore) ListVersions(ctx context.Context, namespace, key string, limit int) ([]TemporalKVVersion, error) {
	if s == nil || s.ldg == nil {
		return nil, fmt.Errorf("temporal kv: ledger is required")
	}
	doc, err := s.loadHistory(ctx, namespace, key)
	if err != nil {
		return nil, err
	}
	versions := append([]TemporalKVVersion{}, doc.Versions...)
	current, err := s.ldg.KV.GetRaw(ctx, namespace, key)
	if err != nil && !errors.Is(err, baseledger.ErrKVNotFound) {
		return nil, err
	}
	if current != nil {
		versions = append(versions, TemporalKVVersion{
			Namespace: namespace,
			Key:       key,
			Version:   len(doc.Versions) + 1,
			Value:     append([]byte(nil), current.Value...),
			UpdatedAt: current.UpdatedAt.UTC(),
			Current:   true,
		})
	}
	sort.Slice(versions, func(i, j int) bool {
		if versions[i].Version == versions[j].Version {
			return versions[i].UpdatedAt.After(versions[j].UpdatedAt)
		}
		return versions[i].Version > versions[j].Version
	})
	if limit > 0 && len(versions) > limit {
		return versions[:limit], nil
	}
	return versions, nil
}

// SnapshotRawAt reconstructs all keys in a namespace at a point in time.
func (s *TemporalKVStore) SnapshotRawAt(ctx context.Context, namespace string, at time.Time) (map[string][]byte, error) {
	if s == nil || s.ldg == nil {
		return nil, fmt.Errorf("temporal kv: ledger is required")
	}
	keys := map[string]bool{}
	current, err := s.ldg.KV.List(ctx, namespace)
	if err != nil {
		return nil, err
	}
	for _, entry := range current {
		keys[entry.Key] = true
	}

	prefix := encodedHistoryPrefix(namespace)
	historyDocs, err := s.ldg.KV.List(ctx, temporalKVHistoryNamespace)
	if err != nil && !errors.Is(err, baseledger.ErrKVNotFound) {
		return nil, err
	}
	for _, entry := range historyDocs {
		if strings.HasPrefix(entry.Key, prefix) {
			if key, ok := decodeHistoryKey(entry.Key); ok {
				keys[key] = true
			}
		}
	}

	out := make(map[string][]byte, len(keys))
	for key := range keys {
		entry, found, err := s.GetRawAt(ctx, namespace, key, at)
		if err != nil {
			return nil, err
		}
		if found {
			out[key] = append([]byte(nil), entry.Value...)
		}
	}
	return out, nil
}

func (s *TemporalKVStore) loadHistory(ctx context.Context, namespace, key string) (temporalKVHistoryDocument, error) {
	doc := temporalKVHistoryDocument{Namespace: namespace, Key: key}
	entry, err := s.ldg.KV.GetRaw(ctx, temporalKVHistoryNamespace, historyKey(namespace, key))
	if err != nil {
		if errors.Is(err, baseledger.ErrKVNotFound) {
			return doc, nil
		}
		return doc, err
	}
	if err := json.Unmarshal(entry.Value, &doc); err != nil {
		return doc, fmt.Errorf("temporal kv history decode %s/%s: %w", namespace, key, err)
	}
	return doc, nil
}

func (s *TemporalKVStore) saveHistory(ctx context.Context, doc temporalKVHistoryDocument) error {
	data, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	return s.ldg.KV.PutRaw(ctx, temporalKVHistoryNamespace, historyKey(doc.Namespace, doc.Key), data)
}

func historyKey(namespace, key string) string {
	return encodedHistoryPrefix(namespace) + encodeHistoryPart(key)
}

func encodedHistoryPrefix(namespace string) string {
	return encodeHistoryPart(namespace) + "/"
}

func encodeHistoryPart(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func decodeHistoryKey(value string) (string, bool) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return "", false
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false
	}
	return string(data), true
}
