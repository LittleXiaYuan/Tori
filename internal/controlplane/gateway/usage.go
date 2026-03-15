package gateway

import (
	"encoding/json"
	"net/http"
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
	MaxChatCalls   int64 `json:"max_chat_calls"`   // 0 = unlimited
	MaxTokensPerDay int64 `json:"max_tokens_per_day"` // 0 = unlimited
}

// UsageTracker tracks and enforces usage quotas.
type UsageTracker struct {
	mu     sync.RWMutex
	usage  map[string]*UsageRecord
	quotas map[string]*QuotaConfig
	daily  map[string]int64 // tenant -> tokens used today
	dayKey string           // "2006-01-02"
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

// SetQuota sets usage quota for a tenant.
func (u *UsageTracker) SetQuota(tenantID string, q QuotaConfig) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.quotas[tenantID] = &q
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

// --- Gateway handlers ---

func (g *Gateway) handleUsage(w http.ResponseWriter, r *http.Request) {
	tid := tenantFromCtx(r.Context())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(g.usage.GetUsage(tid))
}

func (g *Gateway) handleSetQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		TenantID string     `json:"tenant_id"`
		Quota    QuotaConfig `json:"quota"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.TenantID == "" {
		req.TenantID = tenantFromCtx(r.Context())
	}
	g.usage.SetQuota(req.TenantID, req.Quota)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
