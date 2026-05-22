package ledger

import (
	"context"
	"fmt"
	"strings"

	"yunque-agent/internal/ledgercore"
)

// RecallBridge wraps Ledger Recall into a function signature compatible
// with Planner.SetGraphContext: func(query string) string.
//
// When the Planner plans a response, it calls graphContext(userMessage)
// to retrieve relevant historical context. RecallBridge queries Ledger's
// 7-factor scoring recall engine and formats the results as readable text.
type RecallBridge struct {
	ldg      *ledger.Ledger
	tenantID string // default tenant for recall queries
}

// NewRecallBridge creates a bridge from Ledger Recall to Planner.
func NewRecallBridge(ldg *ledger.Ledger, defaultTenantID string) *RecallBridge {
	return &RecallBridge{ldg: ldg, tenantID: defaultTenantID}
}

// Query performs a Ledger recall and returns formatted context for the Planner.
// This is the function to pass to Planner.SetGraphContext().
func (rb *RecallBridge) Query(query string) string {
	ctx := context.Background()

	result, err := rb.ldg.Recall.Recall(ctx, ledger.RecallQuery{
		TenantID: rb.tenantID,
		Query:    query,
		TaskType: ledger.TaskTypeGoal,
		Limit:    5,
		MinScore: 0.2,
	})
	if err != nil || result == nil || len(result.Entries) == 0 {
		return ""
	}

	return formatRecallEntries(result)
}

// QueryForTenant returns a tenant-specific query function.
func (rb *RecallBridge) QueryForTenant(tenantID string) func(string) string {
	return func(query string) string {
		ctx := context.Background()
		result, err := rb.ldg.Recall.Recall(ctx, ledger.RecallQuery{
			TenantID: tenantID,
			Query:    query,
			TaskType: ledger.TaskTypeGoal,
			Limit:    5,
			MinScore: 0.2,
		})
		if err != nil || result == nil || len(result.Entries) == 0 {
			return ""
		}
		return formatRecallEntries(result)
	}
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
