package ledger

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	baseledger "yunque-agent/internal/ledgercore"
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

// NativeKVHistoryRowPreview is a deterministic, non-destructive preview of one
// future native kv_history row derived from the current reserved Ledger KV
// history adapter.
type NativeKVHistoryRowPreview struct {
	ID            string    `json:"id"`
	Namespace     string    `json:"namespace"`
	Key           string    `json:"key"`
	Version       int       `json:"version"`
	Value         []byte    `json:"value_base64"`
	ValueSHA256   string    `json:"value_sha256"`
	UpdatedAt     time.Time `json:"updated_at"`
	ArchivedAt    time.Time `json:"archived_at,omitempty"`
	Current       bool      `json:"current"`
	AuditSeq      uint64    `json:"audit_seq,omitempty"`
	AuditHash     string    `json:"audit_hash,omitempty"`
	SourceAdapter string    `json:"source_adapter"`
}

// NativeKVHistoryMigrationPreview describes how many rows the current reserved
// __kv_history__ adapter would expand into if a native kv_history table existed.
// It intentionally never creates tables, migrates data, writes native rows, or
// removes reserved adapter documents.
type NativeKVHistoryMigrationPreview struct {
	Namespace               string                      `json:"namespace"`
	GeneratedAt             time.Time                   `json:"generated_at"`
	SourceNamespace         string                      `json:"source_namespace"`
	NativeTable             string                      `json:"native_table"`
	ScannedDocumentCount    int                         `json:"scanned_document_count"`
	PreviewRowCount         int                         `json:"preview_row_count"`
	ReturnedRowCount        int                         `json:"returned_row_count"`
	Limit                   int                         `json:"limit,omitempty"`
	WritesNativeKVHistory   bool                        `json:"writes_native_kv_history"`
	MigratesKVHistory       bool                        `json:"migrates_kv_history"`
	UsesReservedKVNamespace bool                        `json:"uses_reserved_kv_namespace"`
	Rows                    []NativeKVHistoryRowPreview `json:"rows"`
	Notes                   []string                    `json:"notes,omitempty"`
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
	if current != nil && bytes.Equal(current.Value, value) {
		return nil
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

// PreviewNativeKVHistoryRows expands the reserved __kv_history__ documents for
// one namespace into deterministic future kv_history rows. The preview includes
// historical versions plus current rows from Ledger KV, but it is deliberately
// read-only: no native table is created, no migration is executed, and no
// reserved history documents are changed or deleted.
func (s *TemporalKVStore) PreviewNativeKVHistoryRows(ctx context.Context, namespace string, limit int) (NativeKVHistoryMigrationPreview, error) {
	if s == nil || s.ldg == nil {
		return NativeKVHistoryMigrationPreview{}, fmt.Errorf("temporal kv: ledger is required")
	}
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		namespace = "memory_snapshot"
	}

	preview := NativeKVHistoryMigrationPreview{
		Namespace:               namespace,
		GeneratedAt:             s.now().UTC(),
		SourceNamespace:         temporalKVHistoryNamespace,
		NativeTable:             "kv_history",
		WritesNativeKVHistory:   false,
		MigratesKVHistory:       false,
		UsesReservedKVNamespace: true,
		Notes: []string{
			"Preview only: no native kv_history table is created, no rows are migrated, and no reserved __kv_history__ documents are changed.",
			"Rows are expanded from reserved TemporalKV history documents plus current Ledger KV entries for deterministic migration review.",
		},
	}

	docsByKey := map[string]temporalKVHistoryDocument{}
	prefix := encodedHistoryPrefix(namespace)
	historyDocs, err := s.ldg.KV.List(ctx, temporalKVHistoryNamespace)
	if err != nil && !errors.Is(err, baseledger.ErrKVNotFound) {
		return preview, err
	}
	for _, entry := range historyDocs {
		if !strings.HasPrefix(entry.Key, prefix) {
			continue
		}
		var doc temporalKVHistoryDocument
		if err := json.Unmarshal(entry.Value, &doc); err != nil {
			return preview, fmt.Errorf("temporal kv history decode %s: %w", entry.Key, err)
		}
		if doc.Namespace == "" {
			doc.Namespace = namespace
		}
		if doc.Key == "" {
			if key, ok := decodeHistoryKey(entry.Key); ok {
				doc.Key = key
			}
		}
		if doc.Namespace != namespace || doc.Key == "" {
			continue
		}
		preview.ScannedDocumentCount++
		docsByKey[doc.Key] = doc
	}

	keys := make(map[string]bool, len(docsByKey))
	for key := range docsByKey {
		keys[key] = true
	}
	currentEntries, err := s.ldg.KV.List(ctx, namespace)
	if err != nil && !errors.Is(err, baseledger.ErrKVNotFound) {
		return preview, err
	}
	currentByKey := make(map[string]*baseledger.KVEntry, len(currentEntries))
	for _, entry := range currentEntries {
		if entry == nil {
			continue
		}
		keys[entry.Key] = true
		copied := *entry
		copied.Value = append([]byte(nil), entry.Value...)
		copied.UpdatedAt = entry.UpdatedAt.UTC()
		currentByKey[entry.Key] = &copied
	}

	sortedKeys := make([]string, 0, len(keys))
	for key := range keys {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

	rows := make([]NativeKVHistoryRowPreview, 0)
	for _, key := range sortedKeys {
		doc := docsByKey[key]
		versions := append([]TemporalKVVersion{}, doc.Versions...)
		sort.SliceStable(versions, func(i, j int) bool {
			if versions[i].Version == versions[j].Version {
				return versions[i].UpdatedAt.Before(versions[j].UpdatedAt)
			}
			return versions[i].Version < versions[j].Version
		})
		for _, version := range versions {
			if version.Namespace == "" {
				version.Namespace = namespace
			}
			if version.Key == "" {
				version.Key = key
			}
			rows = append(rows, nativeKVHistoryRowPreviewFromVersion(version, false))
		}
		if current := currentByKey[key]; current != nil {
			version := TemporalKVVersion{
				Namespace: namespace,
				Key:       key,
				Version:   len(doc.Versions) + 1,
				Value:     append([]byte(nil), current.Value...),
				UpdatedAt: current.UpdatedAt.UTC(),
				Current:   true,
			}
			rows = append(rows, nativeKVHistoryRowPreviewFromVersion(version, true))
		}
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		if rows[i].Key != rows[j].Key {
			return rows[i].Key < rows[j].Key
		}
		if rows[i].Version != rows[j].Version {
			return rows[i].Version < rows[j].Version
		}
		return rows[i].UpdatedAt.Before(rows[j].UpdatedAt)
	})
	preview.PreviewRowCount = len(rows)
	if limit > 0 && len(rows) > limit {
		preview.Rows = rows[:limit]
		preview.Limit = limit
	} else {
		preview.Rows = rows
	}
	preview.ReturnedRowCount = len(preview.Rows)
	return preview, nil
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

func nativeKVHistoryRowPreviewFromVersion(version TemporalKVVersion, current bool) NativeKVHistoryRowPreview {
	version.Namespace = strings.TrimSpace(version.Namespace)
	version.Key = strings.TrimSpace(version.Key)
	version.UpdatedAt = version.UpdatedAt.UTC()
	version.ArchivedAt = version.ArchivedAt.UTC()
	value := append([]byte(nil), version.Value...)
	valueSHA := sha256Hex(value)
	idSeed := fmt.Sprintf("%s/%s/%d/%s/%s", version.Namespace, version.Key, version.Version, version.UpdatedAt.Format(time.RFC3339Nano), valueSHA)
	return NativeKVHistoryRowPreview{
		ID:            "kvh-" + sha256Hex([]byte(idSeed))[:16],
		Namespace:     version.Namespace,
		Key:           version.Key,
		Version:       version.Version,
		Value:         value,
		ValueSHA256:   valueSHA,
		UpdatedAt:     version.UpdatedAt,
		ArchivedAt:    version.ArchivedAt,
		Current:       current || version.Current,
		AuditSeq:      0,
		AuditHash:     "",
		SourceAdapter: "reserved-ledger-kv-namespace",
	}
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
