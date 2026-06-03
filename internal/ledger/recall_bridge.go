package ledger

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"yunque-agent/internal/ledgercore"
)

// systemTenant is the shared namespace for background/global memories (reverie,
// internal writes) that should surface for every tenant during recall.
const systemTenant = "system"

// RecallBridge wraps Ledger Recall into a function signature compatible
// with Planner.SetGraphContext: func(query string) string.
//
// When the Planner plans a response, it calls graphContext(userMessage)
// to retrieve relevant historical context. RecallBridge queries Ledger's
// 7-factor scoring recall engine and formats the results as readable text.
type RecallBridge struct {
	ldg      *ledger.Ledger
	tenantID string // default tenant for recall queries
	rerank   Reranker
}

// Reranker reorders candidate documents by relevance to the query, returning
// the original indices best-first (length ≤ topK). Kept as a plain func type so
// the ledger adapter stays decoupled from the knowledge package's interface.
type Reranker func(ctx context.Context, query string, docs []string, topK int) ([]int, error)

// NewRecallBridge creates a bridge from Ledger Recall to Planner.
func NewRecallBridge(ldg *ledger.Ledger, defaultTenantID string) *RecallBridge {
	return &RecallBridge{ldg: ldg, tenantID: defaultTenantID}
}

// SetReranker attaches an optional cross-encoder reranker applied to recall
// results before they are formatted for the planner. When unset, recall keeps
// the Ledger engine's multi-signal ordering.
func (rb *RecallBridge) SetReranker(fn Reranker) { rb.rerank = fn }

// Query performs a Ledger recall against the bridge's default tenant.
// Kept for backward compatibility; prefer QueryTenant for tenant-correct recall.
func (rb *RecallBridge) Query(query string) string {
	return rb.QueryTenant(context.Background(), rb.tenantID, query)
}

// QueryForTenant returns a tenant-specific query function.
func (rb *RecallBridge) QueryForTenant(tenantID string) func(string) string {
	return func(query string) string {
		return rb.QueryTenant(context.Background(), tenantID, query)
	}
}

// QueryTenant performs a recall scoped to tenantID, unioned with the shared
// "system" namespace so background/global memories surface for every tenant.
// This is the tenant-aware entry point wired into the planner graph context,
// fixing the prior mismatch where writes used the active tenant but recall was
// pinned to "system".
func (rb *RecallBridge) QueryTenant(ctx context.Context, tenantID, query string) string {
	if ctx == nil {
		ctx = context.Background()
	}
	if tenantID == "" {
		tenantID = rb.tenantID
	}

	merged := rb.recall(ctx, tenantID, query)
	if tenantID != systemTenant {
		merged = mergeRecall(merged, rb.recall(ctx, systemTenant, query))
	}
	if merged == nil || len(merged.Entries) == 0 {
		return ""
	}
	rb.applyRerank(ctx, query, merged)
	return formatRecallEntries(merged)
}

// applyRerank reorders merged recall entries with the cross-encoder reranker.
// On any failure it leaves the existing order untouched (best-effort).
func (rb *RecallBridge) applyRerank(ctx context.Context, query string, result *ledger.RecallResult) {
	if rb.rerank == nil || result == nil || len(result.Entries) < 2 {
		return
	}
	docs := make([]string, len(result.Entries))
	for i, e := range result.Entries {
		docs[i] = e.Entry.Content
	}
	order, err := rb.rerank(ctx, query, docs, len(docs))
	if err != nil || len(order) == 0 {
		return
	}
	reordered := make([]ledger.ScoredEntry, 0, len(order))
	seen := make(map[int]bool, len(order))
	for _, idx := range order {
		if idx < 0 || idx >= len(result.Entries) || seen[idx] {
			continue
		}
		seen[idx] = true
		reordered = append(reordered, result.Entries[idx])
	}
	if len(reordered) > 0 {
		result.Entries = reordered
		result.TotalFound = len(reordered)
	}
}

// recall runs the underlying Ledger recall for a single tenant.
func (rb *RecallBridge) recall(ctx context.Context, tenantID, query string) *ledger.RecallResult {
	result, err := rb.ldg.Recall.Recall(ctx, ledger.RecallQuery{
		TenantID: tenantID,
		Query:    query,
		TaskType: ledger.TaskTypeGoal,
		Limit:    5,
		MinScore: 0.2,
		// Only surface user-facing memory kinds. Exclude experience (raw
		// training-data conversation pairs collected for nightly export) and
		// artifact refs — they would pollute recall and waste prompt tokens.
		MemoryKinds: []ledger.MemoryKind{
			ledger.MemoryFact, ledger.MemoryRule,
			ledger.MemorySummary, ledger.MemoryPreference,
		},
	})
	if err != nil {
		return nil
	}
	return result
}

// mergeRecall combines two recall results, de-duplicating by entry ID,
// re-sorting by score, and capping to the per-query limit.
func mergeRecall(a, b *ledger.RecallResult) *ledger.RecallResult {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	all := make([]ledger.ScoredEntry, 0, len(a.Entries)+len(b.Entries))
	all = append(all, a.Entries...)
	all = append(all, b.Entries...)

	seen := make(map[string]bool, len(all))
	out := &ledger.RecallResult{QueryTimeMs: a.QueryTimeMs + b.QueryTimeMs}
	for _, e := range all {
		if e.Entry.ID != "" && seen[e.Entry.ID] {
			continue
		}
		seen[e.Entry.ID] = true
		out.Entries = append(out.Entries, e)
	}
	sort.Slice(out.Entries, func(i, j int) bool {
		return out.Entries[i].Score > out.Entries[j].Score
	})
	if len(out.Entries) > 5 {
		out.Entries = out.Entries[:5]
	}
	out.TotalFound = len(out.Entries)
	return out
}

func formatRecallEntries(result *ledger.RecallResult) string {
	var sb strings.Builder
	sb.WriteString("### 历史经验 (Ledger Recall)\n")

	for i, entry := range result.Entries {
		kindLabel := kindToLabel(entry.Entry.Kind)
		sb.WriteString(fmt.Sprintf("%d. [%s] %s (置信度: %.0f%%, 相关性: %.0f%%)\n",
			i+1,
			kindLabel,
			entry.Entry.Content,
			entry.Entry.Confidence*100,
			entry.Score*100,
		))
	}

	sb.WriteString(fmt.Sprintf("\n_共检索 %d 条记忆，耗时 %dms_\n",
		result.TotalFound, result.QueryTimeMs))
	return sb.String()
}

func kindToLabel(k ledger.MemoryKind) string {
	switch k {
	case ledger.MemoryExperience:
		return "经验"
	case ledger.MemoryFact:
		return "事实"
	case ledger.MemoryRule:
		return "规则"
	case ledger.MemoryPreference:
		return "偏好"
	case ledger.MemorySummary:
		return "摘要"
	default:
		return string(k)
	}
}
