package gateway

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// UsageRecord tracks API usage per tenant.
type UsageRecord struct {
	TenantID    string    `json:"tenant_id"`
	ChatCalls   int64     `json:"chat_calls"`
	StreamCalls int64     `json:"stream_calls"`
	SkillExecs  int64     `json:"skill_execs"`
	TokensUsed  int64     `json:"tokens_used"` // estimated
	LastCall    time.Time `json:"last_call"`
}

// QuotaConfig defines usage limits per tenant.
type QuotaConfig struct {
	MaxChatCalls    int64 `json:"max_chat_calls"`     // 0 = unlimited
	MaxTokensPerDay int64 `json:"max_tokens_per_day"` // 0 = unlimited
}

type usageKVStore interface {
	Put(ctx context.Context, key string, value any) error
	Get(ctx context.Context, key string, dest any) (bool, error)
}

// UsageTracker tracks and enforces usage quotas.
type UsageTracker struct {
	mu     sync.RWMutex
	usage  map[string]*UsageRecord
	quotas map[string]*QuotaConfig
	daily  map[string]int64 // tenant -> tokens used today
	dayKey string           // "2006-01-02"
	kvs    usageKVStore
	dirty  int
}

// NewUsageTracker creates a usage tracker.
func NewUsageTracker() *UsageTracker {
	return &UsageTracker{
		usage:  make(map[string]*UsageRecord),
		quotas: make(map[string]*QuotaConfig),
		daily:  make(map[string]int64),
		dayKey: time.Now().Format("2006-01-02"),
	}
}

// SetKVStore sets the KV store and loads persisted usage/quotas.
func (u *UsageTracker) SetKVStore(kvs usageKVStore) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.kvs = kvs

	var usage map[string]*UsageRecord
	if found, err := kvs.Get(context.Background(), "usage", &usage); err == nil && found {
		for id, rec := range usage {
			u.usage[id] = rec
		}
		slog.Info("usage: loaded records from KV", "tenants", len(usage))
	}
	var quotas map[string]*QuotaConfig
	if found, err := kvs.Get(context.Background(), "quotas", &quotas); err == nil && found {
		for id, q := range quotas {
			u.quotas[id] = q
		}
	}
}

// FlushToKV persists usage and quotas.
func (u *UsageTracker) FlushToKV() {
	u.mu.RLock()
	defer u.mu.RUnlock()
	if u.kvs == nil {
		return
	}
	_ = u.kvs.Put(context.Background(), "usage", u.usage)
	_ = u.kvs.Put(context.Background(), "quotas", u.quotas)
}

func (u *UsageTracker) persistKV() {
	if u.kvs == nil {
		return
	}
	u.dirty++
	if u.dirty%10 == 0 {
		_ = u.kvs.Put(context.Background(), "usage", u.usage)
	}
}

// SetQuota sets usage quota for a tenant.
func (u *UsageTracker) SetQuota(tenantID string, q QuotaConfig) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.quotas[tenantID] = &q
	if u.kvs != nil {
		_ = u.kvs.Put(context.Background(), "quotas", u.quotas)
	}
}

// RecordChat records a chat API call.
func (u *UsageTracker) RecordChat(tenantID string, tokens int64) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.resetDayIfNeeded()
	rec := u.getOrCreate(tenantID)
	rec.ChatCalls++
	rec.TokensUsed += tokens
	rec.LastCall = time.Now()
	u.daily[tenantID] += tokens
	u.persistKV()
}

// RecordStream records a stream API call.
func (u *UsageTracker) RecordStream(tenantID string, tokens int64) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.resetDayIfNeeded()
	rec := u.getOrCreate(tenantID)
	rec.StreamCalls++
	rec.TokensUsed += tokens
	rec.LastCall = time.Now()
	u.daily[tenantID] += tokens
	u.persistKV()
}

// CheckQuota returns true if the tenant is within quota.
func (u *UsageTracker) CheckQuota(tenantID string) bool {
	u.mu.RLock()
	defer u.mu.RUnlock()
	u.resetDayIfNeeded()
	q, ok := u.quotas[tenantID]
	if !ok {
		return true // no quota = unlimited
	}
	rec := u.usage[tenantID]
	if rec == nil {
		return true
	}
	if q.MaxChatCalls > 0 && rec.ChatCalls >= q.MaxChatCalls {
		return false
	}
	if q.MaxTokensPerDay > 0 && u.daily[tenantID] >= q.MaxTokensPerDay {
		return false
	}
	return true
}

// GetUsage returns usage for a tenant.
func (u *UsageTracker) GetUsage(tenantID string) *UsageRecord {
	u.mu.RLock()
	defer u.mu.RUnlock()
	if rec, ok := u.usage[tenantID]; ok {
		cp := *rec
		return &cp
	}
	return &UsageRecord{TenantID: tenantID}
}

// AllUsage returns usage for all tenants.
func (u *UsageTracker) AllUsage() []UsageRecord {
	u.mu.RLock()
	defer u.mu.RUnlock()
	var out []UsageRecord
	for _, rec := range u.usage {
		out = append(out, *rec)
	}
	return out
}

func (u *UsageTracker) getOrCreate(tenantID string) *UsageRecord {
	rec, ok := u.usage[tenantID]
	if !ok {
		rec = &UsageRecord{TenantID: tenantID}
		u.usage[tenantID] = rec
	}
	return rec
}

func (u *UsageTracker) resetDayIfNeeded() {
	today := time.Now().Format("2006-01-02")
	if today != u.dayKey {
		u.daily = make(map[string]int64)
		u.dayKey = today
	}
}
