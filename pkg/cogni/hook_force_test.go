package cogni

import "testing"

// TestForceActivationEngagesUnmatchedCogni verifies that ContextRequest.ForceIDs
// activates a Cogni whose keywords do NOT match the message (the chat `/智能体`
// force-route), while leaving normal score-driven activation and unknown ids
// untouched. Distinct tenants keep each sub-case in its own turn-cache entry.
func TestForceActivationEngagesUnmatchedCogni(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Add(&Declaration{
		ID:          "office-assistant",
		DisplayName: "办公助手",
		Activation:  ActivationRules{Keywords: []string{"做PPT", "写文档"}, MinScore: 0.5},
		Context:     ContextInjection{Static: "你是办公助手。"},
	}, "test"); err != nil {
		t.Fatalf("add declaration: %v", err)
	}
	h := NewHook(reg)

	// 1) A message with no matching keyword does not activate naturally.
	if acts := h.Activate(ContextRequest{Message: "你好啊", TenantID: "natural"}); len(acts) != 0 {
		t.Fatalf("expected no natural activation, got %d", len(acts))
	}

	// 2) Forcing the id activates it regardless of score.
	acts := h.Activate(ContextRequest{Message: "你好啊", TenantID: "forced", ForceIDs: []string{"office-assistant"}})
	found := false
	for _, a := range acts {
		if a.Declaration != nil && a.Declaration.ID == "office-assistant" && a.Activated {
			found = true
		}
	}
	if !found {
		t.Fatalf("forced cogni should activate, got %+v", acts)
	}

	// 3) Forcing an unknown id is a safe no-op.
	if acts := h.Activate(ContextRequest{Message: "你好啊", TenantID: "unknown", ForceIDs: []string{"does-not-exist"}}); len(acts) != 0 {
		t.Fatalf("unknown forced id should activate nothing, got %d", len(acts))
	}
}
