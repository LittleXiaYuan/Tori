package planner

import (
	"context"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/pkg/skills"
)

// TestBuildMessages_IncludesPackContext proves the Tier 0 microkernel wire:
// context contributed by an enabled capability pack (via SetPackContext) is
// injected into the assembled prompt, so a Pack's enablement flows into the
// agent's reasoning — not just its HTTP routes.
func TestBuildMessages_IncludesPackContext(t *testing.T) {
	p := NewPlanner(nil, skills.NewRegistry(), 8)
	p.SetPackContext(func(ctx context.Context, tenantID, query string) string {
		return "PACK_CONTEXT_MARKER from enabled pack"
	})

	msgs, _ := p.BuildMessages(context.Background(), PlanRequest{
		Messages: []llm.Message{{Role: "user", Content: "请做 code review"}},
		TenantID: "tenant-a",
	})

	for _, m := range msgs {
		if strings.Contains(m.Content, "PACK_CONTEXT_MARKER") {
			return
		}
	}
	t.Fatalf("pack context not injected into prompt: %#v", msgs)
}
