package planner

// token_ab_test.go — a real A/B measurement of the Cogni token thesis.
//
// Falsifiable claim (from the design discussion): on a focused multi-turn
// session, an AUTHORITATIVE Cogni surface (cogni-on) produces a deterministic,
// prompt-cache-stable tool block, whereas the ambient per-message intent filter
// (cogni-off) churns the tool block every turn and defeats prompt caching.
//
// This test drives the REAL buildFunctionDefs path with a realistic skill
// registry + a coherent session of user messages, serializes the actual tool
// schema the model would receive, and reports:
//   - per-turn tool-block size (approx tokens),
//   - distinct tool blocks across the session (= prompt-cache misses),
//   - effective tool tokens billed across the session WITH provider prompt
//     caching (cache read modeled at 10% of input, the typical Anthropic/OpenAI
//     cached-token price).
//
// Run: go test ./internal/agentcore/planner -run TestTokenABCogniOnVsOff -v

import (
	"context"
	"encoding/json"
	"testing"

	"yunque-agent/pkg/skills"
)

// abSkill is a skill with a realistic JSON schema so serialized tool sizes
// approximate production tool blocks.
type abSkill struct {
	name string
	desc string
}

func (s *abSkill) Name() string        { return s.name }
func (s *abSkill) Description() string { return s.desc }
func (s *abSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query":  map[string]any{"type": "string", "description": "primary input or target for " + s.name},
			"limit":  map[string]any{"type": "integer", "description": "maximum number of results to return"},
			"format": map[string]any{"type": "string", "enum": []string{"json", "text", "markdown"}, "description": "output format"},
		},
		"required": []string{"query"},
	}
}
func (s *abSkill) Execute(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
	return "", nil
}

// pinnedSurfaceRuntime is an authoritative CogniRuntime whose surface is a fixed
// set of skill names, independent of the user message — the per-session focused
// surface a real Cogni declaration would pin.
type pinnedSurfaceRuntime struct {
	pinned map[string]bool
}

func (p pinnedSurfaceRuntime) BuildContext(context.Context, string, string, string, string) string {
	return ""
}
func (p pinnedSurfaceRuntime) FilterSkills(_ string, _ string, _ string, in []skills.Skill) []skills.Skill {
	out := make([]skills.Skill, 0, len(in))
	for _, s := range in {
		if p.pinned[s.Name()] {
			out = append(out, s)
		}
	}
	return out
}
func (p pinnedSurfaceRuntime) Trace(string, string, string) (CogniTraceDetail, bool) {
	return CogniTraceDetail{}, false
}
func (p pinnedSurfaceRuntime) Tools(context.Context, string, string, string) []CogniTool {
	return nil
}
func (p pinnedSurfaceRuntime) SurfaceAuthoritative(string, string, string) bool { return true }
func (p pinnedSurfaceRuntime) RecordToolOutcome(string, string, string, string, bool) {
}

func TestTokenABCogniOnVsOff(t *testing.T) {
	// 6 categories (matching the planner's hardcoded intentKeywords) × 6 skills.
	cats := map[string][]string{
		"browser":   {"browser_open", "browser_click", "browser_search", "browser_screenshot", "browser_scroll", "browser_input"},
		"connector": {"connector_github_issue", "connector_github_pr", "connector_gmail", "connector_calendar", "connector_notion", "connector_linear"},
		"research":  {"research_report", "research_analyze", "research_survey", "research_summarize", "research_cite", "research_collect"},
		"file":      {"file_save", "file_export", "file_write_markdown", "file_write_csv", "file_write_pdf", "file_read"},
		"image":     {"image_generate", "image_edit", "image_caption", "image_upscale", "image_variations", "image_describe"},
		"workflow":  {"workflow_create", "workflow_run", "workflow_schedule", "workflow_list", "workflow_cancel", "workflow_status"},
	}

	reg := skills.NewRegistry()
	for catID, names := range cats {
		for _, n := range names {
			reg.Register(&abSkill{name: n, desc: "tool " + n + " in category " + catID})
		}
		reg.DefineCategory(skills.SkillCategory{ID: catID, Name: catID, SkillNames: names})
	}

	// A coherent github-+-files working session. Each message triggers a slightly
	// different set of categories, so the ambient (cogni-off) intent filter churns
	// the tool block turn to turn.
	session := []string{
		"打开 github 仓库看看 issue",
		"把这个 issue 导出成 markdown 文件",
		"搜索相关的 pr 并打开",
		"生成一份分析报告保存到 csv",
		"research the linear tickets and summarize",
		"draw an illustration for the report",
		"automate this workflow",
		"把 notion 里的内容导出为 pdf",
	}

	// cogni-off: ambient planner, no cogni runtime → per-message intent filter.
	pOff := NewPlanner(nil, reg, 8)

	// cogni-on: an authoritative Cogni pinning the session's core working surface
	// (connectors + files) for every turn.
	pinned := map[string]bool{}
	for _, n := range append(append([]string{}, cats["connector"]...), cats["file"]...) {
		pinned[n] = true
	}
	pOn := NewPlanner(nil, reg, 8)
	pOn.SetCogniRuntime(pinnedSurfaceRuntime{pinned: pinned})

	const cacheReadFraction = 0.10 // typical provider cached-input price ≈ 10%

	type agg struct {
		rawTokens       int            // billed every turn if NO cache
		effectiveTokens float64        // billed WITH prompt cache (first block full, repeats at 10%)
		distinct        map[string]int // tool_set_hash → first-seen turn size
	}
	measure := func(p *Planner, label string) agg {
		a := agg{distinct: map[string]int{}}
		for i, msg := range session {
			defs := p.buildFunctionDefs(msg, "tenant", "web", false, nil,
				p.ensureContextAssembly(), p.ensureDelegationRuntime(), p.ensureSkillRuntime())
			b, _ := json.Marshal(defs)
			tok := len(b) / 4 // rough but consistent across both arms
			hash := toolSetHash(defs)
			a.rawTokens += tok
			if _, seen := a.distinct[hash]; seen {
				a.effectiveTokens += float64(tok) * cacheReadFraction
			} else {
				a.distinct[hash] = tok
				a.effectiveTokens += float64(tok)
			}
			t.Logf("[%s] turn %d: tools=%d approx_tokens=%d hash=%s msg=%q", label, i+1, len(defs), tok, hash, msg)
		}
		return a
	}

	off := measure(pOff, "cogni-off")
	on := measure(pOn, "cogni-on")

	savedRaw := off.rawTokens - on.rawTokens
	savedEff := off.effectiveTokens - on.effectiveTokens
	pct := func(num, den float64) float64 {
		if den == 0 {
			return 0
		}
		return num / den * 100
	}

	t.Logf("================ TOKEN A/B (8-turn focused session) ================")
	t.Logf("cogni-off: distinct tool blocks=%d/8 (cache misses), raw tokens(no cache)=%d, effective tokens(with cache)=%.0f",
		len(off.distinct), off.rawTokens, off.effectiveTokens)
	t.Logf("cogni-on : distinct tool blocks=%d/8 (cache misses), raw tokens(no cache)=%d, effective tokens(with cache)=%.0f",
		len(on.distinct), on.rawTokens, on.effectiveTokens)
	t.Logf("saved (no-cache view):  %d tokens (%.1f%%)", savedRaw, pct(float64(savedRaw), float64(off.rawTokens)))
	t.Logf("saved (with-cache view): %.0f tokens (%.1f%%)  <- the architectural win", savedEff, pct(savedEff, off.effectiveTokens))
	t.Logf("===================================================================")

	// Falsifiable assertions: the authoritative surface must be more cache-stable
	// (fewer distinct blocks) and cheaper under prompt caching. If these ever
	// fail, the Cogni token thesis is wrong for this scenario and we must stop.
	if len(on.distinct) >= len(off.distinct) {
		t.Fatalf("cogni-on should produce fewer distinct tool blocks than cogni-off; on=%d off=%d", len(on.distinct), len(off.distinct))
	}
	if len(on.distinct) != 1 {
		t.Fatalf("authoritative cogni surface should yield exactly 1 distinct tool block across the session, got %d", len(on.distinct))
	}
	if on.effectiveTokens >= off.effectiveTokens {
		t.Fatalf("cogni-on effective (cached) tokens should be lower; on=%.0f off=%.0f", on.effectiveTokens, off.effectiveTokens)
	}
}
