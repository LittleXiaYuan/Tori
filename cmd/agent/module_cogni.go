package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/embeddings"
	agentcogni "yunque-agent/internal/agentcore/cogni"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/planner"
	agentrt "yunque-agent/internal/agentcore/runtime"
	builtinCogni "yunque-agent/internal/cognikernel/builtin"
	"yunque-agent/internal/controlplane/gateway"
	mcpkg "yunque-agent/internal/mcp"
	cognikernelpack "yunque-agent/internal/packs/cognikernel"
	"yunque-agent/pkg/cogni"
	"yunque-agent/pkg/cognisdk"
	"yunque-agent/pkg/packruntime"
	"yunque-agent/pkg/safego"
	"yunque-agent/pkg/skills"
)

// cogniHookEnabled gates whether activated Cogni declarations actually
// influence the planner's system prompt. We default to ON so dropping a
// .json into data/cognis/ has visible effect, but the wiring stays
// per-module so it can be turned off without disabling the API.
const cogniHookEnabled = true

type plannerCogniRuntime struct {
	enabled func() bool
	hook    *cogni.Hook
	mcp     *cogni.MCPManager
	// beliefAdapter merges cognisdk Pack perception/belief into BuildContext
	// output so the planner has ONE cogni layer instead of two parallel ones
	// (cogni + belief). nil = cogni Declaration only (backward compatible).
	// scope is derived upstream by prompt_builder.intentToScope and passed in
	// through BuildContext's scope param, which drives the belief scope gate (#34).
	beliefAdapter *cognisdk.HostAdapter
}

func (r plannerCogniRuntime) active() bool {
	return r.hook != nil && (r.enabled == nil || r.enabled())
}

func (r plannerCogniRuntime) request(message, tenantID, channel string) cogni.ContextRequest {
	req := cogni.ContextRequest{Message: message, TenantID: tenantID, Channel: channel}
	// Step 3 of cogni consolidation: let cognisdk 先感知消息意图/风险，
	// 把结果作为 hint 注入 cogni 的激活判定，消除两边各自猜意图的冗余。
	// 各自激活逻辑保持独立，只是 cogni 现在能看到 cognisdk 的感知结果。
	if r.beliefAdapter != nil {
		res := r.beliefAdapter.Evaluate(context.Background(), cognisdk.Input{Message: message})
		if res.Perception.Intent != "" || res.Perception.Risk != "" {
			req.PerceptionHint = &cogni.PerceptionHint{
				Intent:   res.Perception.Intent,
				Risk:     string(res.Perception.Risk),
				Category: res.Perception.Category,
			}
		}
	}
	return req
}

// requestWithForce builds the per-turn ContextRequest and carries the user's
// forced-cogni pick (chat `/智能体`) read from the planner-supplied context, so
// BuildContext / Tools force-activate that Cogni regardless of keyword score.
func (r plannerCogniRuntime) requestWithForce(ctx context.Context, message, tenantID, channel string) cogni.ContextRequest {
	req := r.request(message, tenantID, channel)
	req.ForceIDs = planner.ForcedCognisFromCtx(ctx)
	return req
}

func (r plannerCogniRuntime) BuildContext(ctx context.Context, message, tenantID, channel, scope string) string {
	if !r.active() && r.beliefAdapter == nil {
		return ""
	}
	// Step 1 of cogni consolidation: merge cogni Declaration context + cognisdk
	// Pack perception/belief into ONE output so the planner injects a single
	// cogni layer instead of two parallel ones. Either side may be empty.
	var parts []string
	if r.active() {
		if c := r.hook.BuildContext(r.requestWithForce(ctx, message, tenantID, channel)); c != "" {
			parts = append(parts, c)
		}
	}
	if r.beliefAdapter != nil {
		// scope is derived upstream by prompt_builder.intentToScope (from
		// IntentHint) and drives the belief scope gate (#34).
		if b := r.beliefAdapter.BuildContext(ctx, message, tenantID, channel, scope); b != "" {
			parts = append(parts, b)
		}
	}
	return injectModeInstructions(strings.Join(parts, "\n\n"))
}

// Decide implements the v2 Cogni interface by calling all active Cognis' Analyze
// methods and merging their decisions. This is the v2 entry point for intelligent
// context routing and resource allocation.
func (r plannerCogniRuntime) Decide(ctx context.Context, message, tenantID, channel, intentHint string) agentcogni.CogniFinalDecision {
	// Build the CogniRequest for v2 Analyze calls.
	// intentHint from LocalBrain (pre-computed by applyRuntimeClassification) is
	// threaded in so IntentCogni can defer to it instead of re-running keyword
	// detection — it's the same signal, but LocalBrain's version has access to
	// ScorerWithRecent (recent-usage bias) and multi-turn context.
	req := agentcogni.CogniRequest{
		Message:  message,
		TenantID: tenantID,
		Channel:  channel,
		Metadata: map[string]any{
			"intent_hint": intentHint, // "" = no upstream hint; IntentCogni falls back to detectIntent
		},
		// Force-route this turn to the user's pinned Cogni (chat `/智能体`); the
		// v1 compat adapter forwards this into the Hook so its behavior engages.
		ForceCogniIDs: planner.ForcedCognisFromCtx(ctx),
	}

	var cognis []agentcogni.CogniWithPriority

	// V2 Cognis: Intent, Emotion, Risk (highest priority, participates in resource allocation)

	// IntentCogni: task classification -> tools/skills mapping
	intentCogni := agentcogni.NewIntentCogni()
	intentDecision := intentCogni.Analyze(ctx, req)
	cognis = append(cognis, agentcogni.CogniWithPriority{
		Decision: intentDecision,
		Priority: intentCogni.Priority(), // 100
	})

	// RiskCogni: safety guardrails -> dangerous tool filtering
	riskCogni := agentcogni.NewRiskCogni()
	riskDecision := riskCogni.Analyze(ctx, req)
	cognis = append(cognis, agentcogni.CogniWithPriority{
		Decision: riskDecision,
		Priority: riskCogni.Priority(), // 80
	})

	// EmotionCogni: emotional support mode -> disable tools for empathy
	emotionCogni := agentcogni.NewEmotionCogni(nil) // nil = use heuristic detector
	emotionDecision := emotionCogni.Analyze(ctx, req)
	cognis = append(cognis, agentcogni.CogniWithPriority{
		Decision: emotionDecision,
		Priority: emotionCogni.Priority(), // 50
	})

	// V1 Cognis: wrapped via compat adapter (lowest priority, only contributes BehaviorText)

	// Wrap v1 hook (if active) in compat adapter
	if r.active() {
		v1Adapter := agentcogni.NewV1CompatAdapter(r.hook)
		decision := v1Adapter.Analyze(ctx, req)
		cognis = append(cognis, agentcogni.CogniWithPriority{
			Decision: decision,
			Priority: v1Adapter.Priority(), // 0
		})
	}

	// Wrap beliefAdapter (if active) — it also produces behavioral text via BuildContext
	if r.beliefAdapter != nil {
		// beliefAdapter.BuildContext returns text, wrap it in a CogniDecision
		beliefText := r.beliefAdapter.BuildContext(ctx, message, tenantID, channel, "")
		if beliefText != "" {
			cognis = append(cognis, agentcogni.CogniWithPriority{
				Decision: agentcogni.CogniDecision{
					BehaviorText: beliefText,
				},
				Priority: 0, // Same as v1 compat
			})
		}
	}

	// Merge all decisions (priority order: Intent=100, Risk=80, Emotion=50, v1=0)
	final := agentcogni.MergeDecisions(cognis)

	// Telemetry: log activated Cognis and decision summary
	slog.Info("cogni.Decide",
		"tenant_id", tenantID,
		"channel", channel,
		"intent", func() string {
			if final.Intent != nil {
				return final.Intent.Type
			}
			return "unknown"
		}(),
		"intent_confidence", func() float64 {
			if final.Intent != nil {
				return final.Intent.Confidence
			}
			return 0.0
		}(),
		"tools_count", len(final.ToolsNeeded),
		"skills_count", len(final.SkillsNeeded),
		"memory_limit", final.MemoryScope.Limit,
		"memory_categories", len(final.MemoryScope.Categories),
		"behavior_text_len", len(final.BehaviorText),
		"state_keys", len(final.State),
		"cognis_active", len(cognis),
	)

	return final
}

// injectModeInstructions scans the assembled cogni/belief context for static
// Inner State declarations (mode/risk/tone) and appends concrete behavioral
// guidance so these values actually change model output instead of being
// decorative text. Mirrors what buildToneGuide does for user emotion.
func injectModeInstructions(ctx string) string {
	if ctx == "" {
		return ctx
	}
	lower := strings.ToLower(ctx)
	var instructions []string
	// mode
	switch {
	case strings.Contains(lower, "mode: analytical") || strings.Contains(lower, "mode:analytical"):
		instructions = append(instructions, "【行为模式：分析】优先结构化分析，数据驱动，减少情感化表述。")
	case strings.Contains(lower, "mode: companion") || strings.Contains(lower, "mode:companion"):
		instructions = append(instructions, "【行为模式：陪伴】保持温暖、耐心、有情感共鸣的表达，任务也可以轻松一点。")
	case strings.Contains(lower, "mode: focused") || strings.Contains(lower, "mode:focused"):
		instructions = append(instructions, "【行为模式：专注】回答简洁直接，省略铺垫，聚焦目标输出。")
	}
	// risk
	switch {
	case strings.Contains(lower, "risk: high") || strings.Contains(lower, "risk:high"):
		instructions = append(instructions, "【风险：高】操作前主动确认，优先只读验证，遇到不确定立即暂停报告。")
	case strings.Contains(lower, "risk: medium") || strings.Contains(lower, "risk:medium"):
		instructions = append(instructions, "【风险：中】遇到不确定或破坏性操作前先确认。")
	}
	// tone
	switch {
	case strings.Contains(lower, "tone: precise") || strings.Contains(lower, "tone:precise"):
		instructions = append(instructions, "【语调：精确】使用技术性精确语言，减少模糊词。")
	case strings.Contains(lower, "tone: warm") || strings.Contains(lower, "tone:warm"):
		instructions = append(instructions, "【语调：温暖】用温和、有温度的语言回应。")
	}
	if len(instructions) == 0 {
		return ctx
	}
	return ctx + "\n\n" + strings.Join(instructions, "\n")
}

// cogniScopeFromChannel removed in Step 2: scope now derived upstream by
// prompt_builder.intentToScope and piped through CogniRuntime.BuildContext.

func (r plannerCogniRuntime) FilterSkills(message, tenantID, channel string, in []skills.Skill) []skills.Skill {
	if !r.active() {
		return in
	}
	return r.hook.FilterSkills(r.request(message, tenantID, channel), in)
}

func (r plannerCogniRuntime) Trace(message, tenantID, channel string) (planner.CogniTraceDetail, bool) {
	if !r.active() {
		return planner.CogniTraceDetail{}, false
	}
	trace, ok := r.hook.TraceSnapshot(r.request(message, tenantID, channel))
	if !ok {
		return planner.CogniTraceDetail{}, false
	}
	return plannerCogniTraceDetail(trace), true
}

// Tools surfaces MCP tools from every Cogni activated this turn. Skills are
// narrowed by FilterSkills + ToolSurface; MCP tools follow the same surface
// rules (only/include/exclude/max_tools) after mcp.tool_filter, so skill and
// MCP exposure share one declarative contract per Cogni.
func (r plannerCogniRuntime) Tools(ctx context.Context, message, tenantID, channel string) []planner.CogniTool {
	if !r.active() || r.mcp == nil {
		return nil
	}
	acts := r.hook.Activate(r.requestWithForce(ctx, message, tenantID, channel))
	if len(acts) == 0 {
		return nil
	}
	var out []planner.CogniTool
	seen := make(map[string]bool)
	for _, a := range acts {
		if a.Declaration == nil || len(a.Declaration.MCP.Servers) == 0 {
			continue
		}
		cogniID := a.Declaration.ID
		connectCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		err := r.mcp.EnsureConnected(connectCtx, cogniID)
		cancel()
		if err != nil {
			slog.Warn("cogni: mcp connect failed; tools unavailable this turn", "cogni", cogniID, "err", err)
			continue
		}
		raw := r.mcp.Tools(cogniID)
		filtered := cogni.SurfaceMCPTools(raw, a.Declaration.Surface)
		if before, after := len(raw), len(filtered); before != after {
			slog.Info("cogni: mcp tools narrowed by surface", "cogni", cogniID, "before", before, "after", after)
		}
		for _, t := range filtered {
			if seen[t.Name] {
				continue
			}
			seen[t.Name] = true
			id, name := cogniID, t.Name
			out = append(out, planner.CogniTool{
				Name:        name,
				Description: t.Description,
				Parameters:  t.InputSchema,
				Invoke: func(ctx context.Context, args map[string]any) (string, error) {
					res, callErr := r.mcp.CallTool(ctx, id, name, args)
					if callErr != nil {
						return "", callErr
					}
					return stringifyMCPResult(res), nil
				},
			})
		}
	}
	return out
}

// SurfaceAuthoritative reports whether an activated cogni declared a non-identity
// ToolSurface this turn, so the planner can treat the cogni's capability set
// (skills ∪ MCP tools) as the definitive, cache-stable tool block.
func (r plannerCogniRuntime) SurfaceAuthoritative(message, tenantID, channel string) bool {
	if !r.active() {
		return false
	}
	return r.hook.SurfaceAuthoritative(r.request(message, tenantID, channel))
}

// RecordToolOutcome feeds a tool result back into the experience self-tuning
// loop. The hook attributes it to the owning cogni(s) and records via the
// per-cogni ExperienceStore (debounced persist), so this is cheap and a no-op
// unless a cogni with experience enabled surfaces the tool.
func (r plannerCogniRuntime) RecordToolOutcome(message, tenantID, channel, tool string, success bool) {
	if !r.active() {
		return
	}
	r.hook.RecordToolOutcome(r.request(message, tenantID, channel), tool, success)
}

func stringifyMCPResult(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	default:
		if b, err := json.Marshal(t); err == nil {
			return string(b)
		}
		return fmt.Sprintf("%v", t)
	}
}

func plannerCogniTraceDetail(trace cogni.Trace) planner.CogniTraceDetail {
	detail := planner.CogniTraceDetail{
		ContextBytes:      trace.Context.Bytes,
		TemplateFallbacks: trace.Context.TemplateFallbacks,
		MessageHash:       trace.MessageHash,
		DurationMs:        trace.DurationMs,
		DroppedForBudget:  append([]string(nil), trace.Context.DroppedForBudget...),
	}
	for _, activation := range trace.Activations {
		if !activation.Activated {
			continue
		}
		if activation.DisplayName != "" {
			detail.Activated = append(detail.Activated, activation.DisplayName)
		} else {
			detail.Activated = append(detail.Activated, activation.ID)
		}
	}
	if filter := trace.ToolFilter; filter != nil {
		detail.ToolBefore = filter.Before
		detail.ToolAfter = filter.After
		detail.Removed = append([]string(nil), filter.Removed...)
		detail.FellBackToInput = filter.FellBackToInput
	}
	return detail
}

// cogniModule wires the hot-pluggable Cogni Registry into the runtime.
//
// Boot behaviour:
//  1. Init creates an empty cogni.Registry and attaches it to the App
//     (component key "cogni_registry") so other subsystems can look it up.
//  2. Init also pushes the registry into the Gateway via SetCogniRegistry
//     so the /v1/cognis/* admin endpoints become live.
//  3. Init builds a cogni.Hook backed by an in-memory TraceStore so every
//     turn produces a structured Trace consumable via /v1/cognis/traces.
//  4. Start performs a one-shot reload from `${DataDir}/cognis/` so any
//     declarations the user has dropped in are picked up.
//
// Hot-plug behaviour after boot:
//   - Drop a *.json file into data/cognis/ → POST /v1/cognis/reload picks it up.
//   - DELETE /v1/cognis/{id} or POST /v1/cognis/{id}/disable to remove/disable.
//   - POST /v1/cognis with a JSON body to add a declaration without a file.
//   - GET /v1/cognis/traces → recent turn traces (activations / context bytes / tool diff).
//
// The module is profile=lite because Cogni declarations themselves are cheap;
// gating happens at activation time (Evaluator) which is opt-in per turn.
type cogniModule struct {
	registry       *cogni.Registry
	dir            string
	store          cogni.TraceStore
	fileLog        *FileTraceStore
	sentinel       *cogni.Sentinel
	workflowEngine *cogni.WorkflowEngine
	experiences    map[string]*cogni.ExperienceStore // keyed by cogni ID
	hook           *cogni.Hook
	mcpMgr         *cogni.MCPManager
	bus            *cogni.CogniBus
	scheduler      *cogni.PerceptionScheduler
	costTracker    *cogni.CostTracker
	autoOrganizer  *cogni.AutoOrganizer
	packRegistry   *packruntime.Registry
	runtimeMu      sync.Mutex
	runtimeActive  bool
}

func (m *cogniModule) Name() string { return "cogni" }
func (m *cogniModule) Description() string {
	return "声明式智体注册中心，支持热插拔加载/启停"
}
func (m *cogniModule) Profile() string { return "lite" }

func (m *cogniModule) Init(ctx context.Context, app *agentrt.App) error {
	m.registry = cogni.NewRegistry()
	m.dir = filepath.Join(app.Config.DataDir, "cognis")

	// Prefer the file-backed trace store so decision history survives
	// restarts; fall back to pure in-memory if the file cannot be opened
	// (e.g. read-only data dir) — traces are diagnostic, never blocking.
	tracePath := filepath.Join(m.dir, "traces.jsonl")
	if fs, err := NewFileTraceStore(tracePath, 512, 10*1024*1024); err == nil {
		m.fileLog = fs
		m.store = fs
		slog.Info("cogni: trace log ready", "path", tracePath)
	} else {
		slog.Warn("cogni: using in-memory trace store (file log unavailable)", "err", err)
		m.store = cogni.NewInMemoryTraceStore(512)
	}

	app.Set(agentrt.CompCogniRegistry, m.registry)
	app.Set(agentrt.CompCogniTraces, m.store)
	if raw, ok := app.Get(agentrt.CompPackRuntimeRegistry); ok {
		if registry, ok := raw.(*packruntime.Registry); ok {
			m.packRegistry = registry
		}
	}

	m.sentinel = cogni.NewSentinel(m.store, m.registry, cogni.SentinelPolicy{
		Interval:              5 * time.Minute,
		AutoDisableOnCritical: cogniAutoDisableFromEnv(),
	})
	app.Set(agentrt.CompCogniSentinel, m.sentinel)

	// Evolution engine with LLM-powered bench & analyze
	evolutionEngine := cogni.NewEvolutionEngine(cogni.DefaultEvolutionConfig(), m.dir)
	if app.LLMPool != nil {
		if cl := app.LLMPool.GetOrFallback("smart"); cl != nil {
			evolutionEngine.SetBenchFunc(func(ctx context.Context, cogniID string) (*cogni.BenchResult, error) {
				decl, ok := m.registry.Get(cogniID)
				if !ok {
					return nil, fmt.Errorf("cogni %q not found", cogniID)
				}
				prompt := fmt.Sprintf("Evaluate this AI agent declaration and score its quality (0-100). Consider: activation rules clarity, context relevance, skill coverage, edge cases.\n\nDeclaration ID: %s\nDisplay Name: %s\nDescription: %s\n\nReturn ONLY a JSON: {\"score\": <0-100>, \"passed\": <count>, \"failed\": <count>, \"total\": <count>, \"failures\": [{\"task_id\": \"...\", \"expected\": \"...\", \"actual\": \"...\"}]}",
					decl.ID, decl.DisplayName, decl.Description)
				reply, err := cl.Chat(ctx, []llm.Message{{Role: "user", Content: prompt}}, 0.3)
				if err != nil {
					return nil, err
				}
				var br cogni.BenchResult
				if idx := strings.Index(reply, "{"); idx >= 0 {
					reply = reply[idx:]
				}
				if idx := strings.LastIndex(reply, "}"); idx >= 0 {
					reply = reply[:idx+1]
				}
				if err := json.Unmarshal([]byte(reply), &br); err != nil {
					return &cogni.BenchResult{Score: 50, Total: 1, Passed: 1}, nil
				}
				return &br, nil
			})
			evolutionEngine.SetAnalyzeFunc(func(ctx context.Context, failures []cogni.TaskFailure) ([]cogni.SkillMutation, error) {
				failJSON, _ := json.Marshal(failures)
				prompt := fmt.Sprintf("Analyze these AI agent benchmark failures and propose specific mutations to fix them.\n\nFailures:\n%s\n\nReturn ONLY a JSON array of mutations: [{\"skill_name\": \"...\", \"mutation_type\": \"prompt|parameter|timeout\", \"after\": {\"key\": \"value\"}, \"rationale\": \"...\"}]", string(failJSON))
				reply, err := cl.Chat(ctx, []llm.Message{{Role: "user", Content: prompt}}, 0.3)
				if err != nil {
					return nil, err
				}
				if idx := strings.Index(reply, "["); idx >= 0 {
					reply = reply[idx:]
				}
				if idx := strings.LastIndex(reply, "]"); idx >= 0 {
					reply = reply[:idx+1]
				}
				var mutations []cogni.SkillMutation
				if err := json.Unmarshal([]byte(reply), &mutations); err != nil {
					return nil, fmt.Errorf("parse mutations: %w", err)
				}
				return mutations, nil
			})
			slog.Info("cogni: evolution engine bench+analyze wired via LLM")
		}
	}

	// Self-evolution apply/revert — the missing link that lets the engine
	// actually mutate a Cogni declaration. SAFE BY DEFAULT: gated OFF unless
	// COGNI_EVOLUTION_APPLY_ENABLED=true. Even when enabled, mutations only run
	// via the manual evolve endpoint, are snapshot-reverted on regression by the
	// engine's own gate, are re-validated before landing, and never touch the
	// Cogni's identity. Effects are in-memory (registry) so a restart falls back
	// to the on-disk baseline — a deliberate safety floor for self-modification.
	if strings.EqualFold(strings.TrimSpace(os.Getenv("COGNI_EVOLUTION_APPLY_ENABLED")), "true") {
		var evoMu sync.Mutex
		evoBackups := make(map[string][]byte) // cogniID → pre-round declaration JSON
		evolutionEngine.SetApplyFunc(func(cogniID string, mutations []cogni.SkillMutation) error {
			decl, ok := m.registry.Get(cogniID)
			if !ok {
				return fmt.Errorf("cogni %q not found", cogniID)
			}
			original, err := json.Marshal(decl)
			if err != nil {
				return fmt.Errorf("snapshot marshal: %w", err)
			}
			// Deep-copy via JSON so we never mutate the live declaration in place.
			var next cogni.Declaration
			if err := json.Unmarshal(original, &next); err != nil {
				return fmt.Errorf("snapshot unmarshal: %w", err)
			}
			for _, mut := range mutations {
				applyMutationToDecl(&next, mut)
			}
			next.ID = cogniID // identity is immutable
			if err := next.Validate(); err != nil {
				return fmt.Errorf("mutated declaration invalid: %w", err)
			}
			// Snapshot the pre-round state each apply so a later revert restores
			// THIS round only (keeping any prior kept rounds).
			evoMu.Lock()
			evoBackups[cogniID] = original
			evoMu.Unlock()
			if err := m.registry.Add(&next, "evolution"); err != nil {
				return fmt.Errorf("registry update: %w", err)
			}
			slog.Info("cogni: evolution applied mutations", "cogni", cogniID, "count", len(mutations))
			return nil
		})
		evolutionEngine.SetRevertFunc(func(cogniID string, mutations []cogni.SkillMutation) error {
			evoMu.Lock()
			original, ok := evoBackups[cogniID]
			evoMu.Unlock()
			if !ok {
				return nil
			}
			var prev cogni.Declaration
			if err := json.Unmarshal(original, &prev); err != nil {
				return fmt.Errorf("revert unmarshal: %w", err)
			}
			prev.ID = cogniID
			if err := m.registry.Add(&prev, "evolution-revert"); err != nil {
				return fmt.Errorf("registry revert: %w", err)
			}
			slog.Info("cogni: evolution reverted to baseline", "cogni", cogniID)
			return nil
		})
		slog.Info("cogni: evolution apply+revert wired (snapshot-based, gate ON)")
	} else {
		slog.Info("cogni: evolution apply disabled (set COGNI_EVOLUTION_APPLY_ENABLED=true to allow self-modification)")
	}

	// Federation
	selfID := "local"
	selfURL := "http://localhost" + app.Config.Addr
	federation := cogni.NewCogniFederation(selfID, selfURL, m.registry)

	// Self-genesis engine
	var genesis *cogni.Genesis
	if app.LLMPool != nil {
		if cl := app.LLMPool.GetOrFallback("smart"); cl != nil {
			genesis = cogni.NewGenesis(func(ctx context.Context, system, user string) (string, error) {
				msgs := []llm.Message{
					{Role: "system", Content: system},
					{Role: "user", Content: user},
				}
				return cl.Chat(ctx, msgs, 0.7)
			})
		}
	}

	m.experiences = make(map[string]*cogni.ExperienceStore)

	// MCPManager: per-cogni MCP server connections with lazy initialization.
	connector := cogni.NewStdioMCPConnector(func(ctx context.Context, def cogni.MCPServerDef) (cogni.MCPConnection, error) {
		switch def.Transport {
		case "streamable_http", "sse":
			headers := def.Headers
			if headers == nil {
				headers = make(map[string]string)
			}
			timeout := 30 * time.Second
			if def.Timeout > 0 {
				timeout = time.Duration(def.Timeout) * time.Second
			}
			provider := mcpkg.NewStreamableHTTPProvider(def.URL, headers, timeout)
			if err := provider.Start(ctx); err != nil {
				return nil, err
			}
			return &mcpProviderBridge{provider: provider}, nil
		default:
			env := cogni.ResolveEnv(def.Env)
			provider := mcpkg.NewStdioProvider(def.Command, def.Args, env)
			if err := provider.Start(ctx); err != nil {
				return nil, err
			}
			return &mcpProviderBridge{provider: provider}, nil
		}
	})
	m.mcpMgr = cogni.NewMCPManager(connector)

	// CogniBus: intent broadcast + bidding router.
	m.bus = cogni.NewCogniBus(cogni.NewEvaluator(), cogni.DefaultBusConfig())

	// Cost tracker for economics layer.
	m.costTracker = cogni.NewCostTracker()

	// AutoOrganizer: automatically create cogni declarations from installed skills.
	if app.SkillRegistry != nil {
		reg := app.SkillRegistry
		m.autoOrganizer = cogni.NewAutoOrganizer(m.registry, func() []cogni.SkillInfo {
			all := reg.All()
			out := make([]cogni.SkillInfo, len(all))
			for i, s := range all {
				out[i] = cogni.SkillInfo{
					Name:        s.Name(),
					Description: s.Description(),
					Category:    reg.CategoryOf(s.Name()),
				}
			}
			return out
		})
		if app.LLMPool != nil {
			if cl := app.LLMPool.GetOrFallback("smart"); cl != nil {
				m.autoOrganizer.SetLLM(func(ctx context.Context, system, user string) (string, error) {
					msgs := []llm.Message{
						{Role: "system", Content: system},
						{Role: "user", Content: user},
					}
					return cl.Chat(ctx, msgs, 0.7)
				})
			}
		}
	}

	// Wire WorkflowEngine with a SkillExecutor that delegates to the skill registry.
	if app.SkillRegistry != nil {
		reg := app.SkillRegistry
		m.workflowEngine = cogni.NewWorkflowEngine(func(ctx context.Context, skillName string, args map[string]any) (any, error) {
			sk, ok := reg.Get(skillName)
			if !ok {
				return nil, fmt.Errorf("skill %q not found", skillName)
			}
			env := &skills.Environment{}
			if app.LLMPool != nil {
				if cl := app.LLMPool.GetOrFallback("smart"); cl != nil {
					env.LLMCall = func(ctx context.Context, system, user string) (string, error) {
						msgs := []llm.Message{
							{Role: "system", Content: system},
							{Role: "user", Content: user},
						}
						return cl.Chat(ctx, msgs, 0.7)
					}
				}
			}
			result, err := sk.Execute(ctx, args, env)
			return result, err
		})
	}

	// NL Config translator: natural language → structured configuration.
	// Reuses the same LLM client as Genesis.
	var nlTranslator *cogni.NLConfigTranslator
	if app.LLMPool != nil {
		if cl := app.LLMPool.GetOrFallback("smart"); cl != nil {
			nlTranslator = cogni.NewNLConfigTranslator(func(ctx context.Context, system, user string) (string, error) {
				msgs := []llm.Message{
					{Role: "system", Content: system},
					{Role: "user", Content: user},
				}
				return cl.Chat(ctx, msgs, 0.3)
			})
			slog.Info("cogni: NL config translator wired")
		}
	}

	// Gateway wiring — placed after all cogni subsystems are initialized
	// so the gateway receives valid (non-nil) references.
	if gwRaw, ok := app.Get(agentrt.CompGateway); ok {
		if gw, ok := gwRaw.(*gateway.Gateway); ok {
			gw.SetCogniRegistry(m.registry, m.dir)
			gw.SetCogniTraceStore(m.store)
			gw.SetCogniSentinel(m.sentinel)
			gw.SetCogniWorkflowEngine(m.workflowEngine)
			gw.SetCogniExperiences(m.experiences)
			gw.SetCogniGenesis(genesis)
			gw.SetCogniEvolution(evolutionEngine)
			gw.SetCogniFederation(federation)
			gw.SetCogniCostTracker(m.costTracker)
			gw.SetCogniBus(m.bus)
			gw.SetNLConfigTranslator(nlTranslator)
			cogniPackHandler := cognikernelpack.NewHandlerWithRuntimeState(gw, m)
			gw.SetCogniKernelRuntimeStateHandler(cogniPackHandler.HandleRuntimePackState)
			// Cogni Kernel migrated to the v2 Module lifecycle (Tier 0 microkernel).
			_ = gw.RegisterModule(cogniPackHandler)
		}
	}

	if cogniHookEnabled && app.Planner != nil {
		hook := cogni.NewHook(m.registry)
		m.hook = hook
		hook.SetTraceStore(m.store)
		if app.Orchestrator != nil {
			orch := app.Orchestrator
			hook.SetMemorySearch(func(ctx context.Context, tenantID, query string) string {
				return orch.CompileContext(ctx, tenantID, query)
			})
		}
		// Until a real pkg/capsule.Registry is wired into the runtime, use
		// skill category as the pseudo-capsule id so Declaration.Surface.
		// FromCapsules can actually narrow by "owner group" (e.g.
		// `from_capsules: ["browser"]`).
		if app.SkillRegistry != nil {
			reg := app.SkillRegistry
			hook.SetSkillOwner(func(name string) string {
				return reg.CategoryOf(name)
			})
		}
		hook.SetExperienceProvider(func(cogniID string) *cogni.ExperienceStore {
			return m.experiences[cogniID]
		})
		// Semantic Cogni activation: wire the embedder so Cognis declaring
		// `activation.semantic.examples` activate on meaning (paraphrase-robust),
		// not just literal keywords. No-op when the embed resolver is unavailable
		// or COGNI_SEMANTIC_ACTIVATION=false — keyword/regex scoring is unaffected.
		if os.Getenv("COGNI_SEMANTIC_ACTIVATION") != "false" {
			if raw, ok := app.Get("embed_resolver"); ok {
				if res, ok := raw.(*embeddings.Resolver); ok {
					if emb, ok := res.Primary(); ok {
						hook.SetEmbedder(func(text string) []float32 {
							cctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
							defer cancel()
							v, err := emb.Embed(cctx, text)
							if err != nil {
								slog.Debug("cogni: semantic embed failed", "err", err)
								return nil
							}
							return v
						})
						slog.Info("cogni: semantic activation enabled (embedder wired)")
					}
				}
			}
		}
		// Capability arbitration ("top-K experts win"): opt-in via env. Default
		// (unset) keeps legacy behavior where every activated cogni composes.
		if arbCfg := cogniArbitrationFromEnv(); !arbCfg.IsZero() {
			hook.SetArbitration(arbCfg)
			slog.Info("cogni: capability arbitration enabled",
				"max_active", arbCfg.MaxActive, "min_confidence", arbCfg.MinConfidence)
		}
		// Experience-driven surface self-tuning: opt-in via env. Default (unset)
		// keeps legacy behavior; recording still happens but pruning is inert.
		if tuneCfg := cogniExperienceTuningFromEnv(); !tuneCfg.IsZero() {
			hook.SetExperienceTuning(tuneCfg)
			slog.Info("cogni: experience surface tuning enabled",
				"min_observations", tuneCfg.MinObservations, "min_success_rate", tuneCfg.MinSuccessRate)
		}
		// Context byte budget: opt-in via env. Default (unset) keeps legacy
		// unbounded BuildContext concatenation.
		if budget := cogniContextByteBudgetFromEnv(); budget > 0 {
			hook.SetContextByteBudget(budget)
			slog.Info("cogni: context byte budget enabled", "budget_bytes", budget)
		}
		// Step 1 of cogni consolidation: pull the cognisdk belief adapter
		// (exposed by init_task_cognition.go) and merge it into the unified
		// plannerCogniRuntime so BuildContext emits ONE cogni layer containing
		// both Declaration context and Pack perception/belief, instead of two
		// parallel layers (cogni + belief) in the prompt.
		var beliefAdapter *cognisdk.HostAdapter
		if rawBA, ok := app.Get("cognisdk_belief_adapter"); ok {
			if ba, ok := rawBA.(*cognisdk.HostAdapter); ok {
				beliefAdapter = ba
			}
		}
		app.Planner.SetCogniRuntime(plannerCogniRuntime{
			enabled:       m.cogniKernelPackEnabled,
			hook:          hook,
			mcp:           m.mcpMgr,
			beliefAdapter: beliefAdapter,
		})
		// Wire cost tracking + bus routing on activation
		{
			tracker := m.costTracker
			bus := m.bus
			hook.SetOnActivation(func(cogniID string, score float64) {
				if tracker != nil && score > 0 {
					tracker.Record(cogni.CostEntry{
						CogniID:   cogniID,
						Cost:      score * 0.01,
						Operation: "activation",
					})
				}
				if bus != nil {
					slog.Debug("cogni_bus: activation", "cogni", cogniID, "score", score)
				}
			})
			// Economics enforcement: a cogni whose daily budget is exhausted is
			// dropped from the activation set until local midnight. estimatedCost=0
			// gates on accumulated daily spend only — per-run budgets stay advisory
			// (we can't estimate a turn's cost before running it).
			if tracker != nil {
				hook.SetBudgetGuard(func(cogniID string) error {
					return tracker.CheckBudget(cogniID, 0)
				})
			}
		}
		slog.Info("cogni: planner context injection + surface filter + mcp tools + bus + cost + trace wired")
	}

	// Adapt existing Plugins as Cogni declarations so they participate
	// in the unified evaluation pipeline.
	if app.PluginReg != nil {
		cogni.RegisterPlugins(m.registry, app.PluginReg.All())
		slog.Info("cogni: existing plugins adapted as declarations",
			"count", len(app.PluginReg.All()))
	}

	// Register built-in Cogni declarations (office/creative/data-analyst).
	builtinDecls, err := builtinCogni.LoadAll()
	if err != nil {
		slog.Warn("cogni: failed to load built-in declarations", "err", err)
	} else {
		for _, d := range builtinDecls {
			if addErr := m.registry.Add(d, "builtin"); addErr != nil {
				slog.Warn("cogni: failed to add built-in declaration", "id", d.ID, "err", addErr)
			}
		}
		if len(builtinDecls) > 0 {
			slog.Info("cogni: built-in declarations registered", "count", len(builtinDecls))
		}
	}

	slog.Info("cogni registry initialized", "dir", m.dir)
	return nil
}

func (m *cogniModule) Start(ctx context.Context) error {
	summary, err := m.registry.ReloadFromDir(m.dir)
	if err != nil {
		slog.Warn("cogni: initial reload failed", "dir", m.dir, "err", err)
		return nil
	}
	if summary.Added > 0 || summary.Updated > 0 {
		slog.Info("cogni: declarations loaded",
			"dir", m.dir,
			"added", summary.Added,
			"updated", summary.Updated,
			"errors", len(summary.Errors))
	}
	for _, e := range summary.Errors {
		slog.Warn("cogni: declaration load error", "path", e.Path, "err", e.Err)
	}

	// Re-initialize experience stores when cognis are added/updated/removed.
	m.registry.OnChange(func(event, id string) {
		// pkg/cogni.Registry invokes hooks while holding its mutation lock.
		// Rebuild runtime projections asynchronously so sync can call
		// Registry.Active() without self-deadlocking the add/update path.
		go m.syncCogniKernelPackRuntime(ctx)
	})

	m.watchCogniKernelPackState(ctx)

	// Auto-organizing skills into cognis calls the LLM (intelligent grouping +
	// activation-rule generation), and the initial runtime sync can be heavy.
	// Module Start runs on the boot critical path BEFORE the HTTP listener
	// binds, so doing this synchronously means a slow model stalls startup: the
	// desktop shell health-checks the backend and kills it after 60s, so the
	// listener never comes up ("本地服务不可用"). Run it in the background — these
	// are projections that converge shortly after boot, not prerequisites for
	// serving requests; OnChange re-syncs once auto-organize adds cognis.
	safego.Go("cogni-startup-organize", func() {
		if m.cogniKernelPackEnabled() && m.autoOrganizer != nil {
			result := m.autoOrganizer.Sync(context.Background())
			if result.Created > 0 || result.Updated > 0 {
				slog.Info("cogni: auto-organized skills into cognis",
					"created", result.Created,
					"updated", result.Updated,
					"removed", result.Removed)
			}
		}
		m.syncCogniKernelPackRuntime(ctx)
	})

	return nil
}

func (m *cogniModule) cogniKernelPackEnabled() bool {
	if m.packRegistry == nil {
		return true
	}
	pack, ok := m.packRegistry.Get(cognikernelpack.PackID)
	return ok && pack.Status == packruntime.PackStatusEnabled
}

func (m *cogniModule) watchCogniKernelPackState(ctx context.Context) {
	if m.packRegistry == nil {
		return
	}
	m.packRegistry.OnChange(func(event packruntime.ChangeEvent) {
		if event.Pack.Manifest.ID != cognikernelpack.PackID {
			return
		}
		m.syncCogniKernelPackRuntime(ctx)
	})
}

func (m *cogniModule) syncCogniKernelPackRuntime(ctx context.Context) {
	enabled := m.cogniKernelPackEnabled()

	m.runtimeMu.Lock()
	defer m.runtimeMu.Unlock()

	if !enabled {
		m.clearCogniRuntimeState()
		if m.runtimeActive {
			if m.scheduler != nil {
				m.scheduler.Stop()
			}
			if m.sentinel != nil {
				m.sentinel.Stop()
			}
			m.runtimeActive = false
			slog.Info("cogni: runtime loops stopped by pack state", "pack", cognikernelpack.PackID)
		}
		return
	}

	m.initExperienceStores()

	if m.sentinel != nil {
		m.sentinel.Start(ctx)
	}
	if m.hook != nil {
		if m.scheduler == nil {
			m.scheduler = cogni.NewPerceptionScheduler(m.registry, func(ctx context.Context, cogniID string, signal *cogni.PerceptionSignal) {
				slog.Info("cogni: perception event", "cogni", cogniID, "schedule", signal.ScheduleTriggered, "webhook", signal.WebhookTriggered)
			})
		}
		m.scheduler.Start()
		m.scheduler.Refresh()
		paths := m.scheduler.WebhookPaths()
		if len(paths) > 0 {
			slog.Info("cogni: perception webhook paths registered", "paths", paths)
		}
	}
	if !m.runtimeActive {
		slog.Info("cogni: runtime loops enabled by pack state", "pack", cognikernelpack.PackID)
	}
	m.runtimeActive = true
}

func (m *cogniModule) cogniRuntimeActive() bool {
	m.runtimeMu.Lock()
	defer m.runtimeMu.Unlock()
	return m.runtimeActive
}

func (m *cogniModule) CogniKernelRuntimeState() cognikernelpack.RuntimeStateReport {
	installed := false
	enabled := m.cogniKernelPackEnabled()
	status := "registry-unavailable"
	if m.packRegistry != nil {
		if pack, ok := m.packRegistry.Get(cognikernelpack.PackID); ok {
			installed = true
			status = string(pack.Status)
		} else {
			status = "not-installed"
		}
	}
	m.runtimeMu.Lock()
	runtimeRunning := m.runtimeActive
	experienceStoreCount := len(m.experiences)
	schedulerReady := m.scheduler != nil && runtimeRunning
	sentinelReady := m.sentinel != nil && runtimeRunning
	m.runtimeMu.Unlock()

	activeBusCognis := 0
	if m.bus != nil {
		activeBusCognis = m.bus.ActiveCognis()
	}
	report := cognikernelpack.RuntimeStateReport{
		PackID:                    cognikernelpack.PackID,
		Stage:                     "runtime-loop-pack-state-gate",
		PackInstalled:             installed,
		PackEnabled:               enabled,
		PackStatus:                status,
		RuntimeLoopPackStateReady: installed,
		RuntimeLoopRunning:        runtimeRunning,
		StopsRuntimeLoops:         installed && !enabled && !runtimeRunning,
		StartsRuntimeLoops:        installed && enabled && runtimeRunning,
		ClearsRuntimeState:        installed && !enabled && activeBusCognis == 0 && experienceStoreCount == 0,
		SentinelReady:             sentinelReady,
		SchedulerReady:            schedulerReady,
		BusReady:                  m.bus != nil && runtimeRunning,
		ExperienceStoreReady:      runtimeRunning,
		ActiveBusCognis:           activeBusCognis,
		ExperienceStoreCount:      experienceStoreCount,
		GeneratedAt:               time.Now().UTC(),
		Capabilities:              []string{"cognis.runtime.pack_state", "cognis.runtime.loop_gate"},
		Artifacts:                 []string{"cogni-runtime-pack-state.json"},
		Notes: []string{
			"runtime_loop_pack_state_ready=true means the live Cogni runtime loops are bound to yunque.pack.cogni-kernel enabled/disabled state.",
			"When the pack is disabled, planner Cogni context injection is suppressed and bus/experience runtime projections are cleared.",
			"This report is read-only; state changes still go through /v1/packs/enable and /v1/packs/disable.",
		},
	}
	if m.scheduler == nil && runtimeRunning {
		report.Notes = append(report.Notes, "perception scheduler is created lazily after the Cogni hook is wired.")
	}
	if m.sentinel != nil && runtimeRunning && !report.SentinelReady {
		report.Notes = append(report.Notes, "sentinel has no background interval configured; manual alert scans remain available.")
	}
	return report
}

// initExperienceStores creates/updates per-cogni ExperienceStore instances
// for every declaration that has Experience.Enabled == true.
func (m *cogniModule) initExperienceStores() {
	active := m.registry.Active()
	seen := make(map[string]bool, len(active))
	for _, d := range active {
		seen[d.ID] = true

		// Experience stores
		if d.Experience.Enabled {
			if _, exists := m.experiences[d.ID]; !exists {
				cfg := d.Experience
				if cfg.StoreDir == "" {
					cfg.StoreDir = filepath.Join(m.dir, "experience", d.ID)
				}
				m.experiences[d.ID] = cogni.NewExperienceStore(d.ID, cfg)
				slog.Info("cogni: experience store initialized", "cogni", d.ID)
			}
		} else if store, exists := m.experiences[d.ID]; exists {
			// Experience toggled off while the cogni stays active: flush pending
			// outcomes and drop the store so we stop injecting/recording stale
			// experience for it.
			store.Flush()
			delete(m.experiences, d.ID)
			slog.Info("cogni: experience store removed (disabled)", "cogni", d.ID)
		}

		// MCP connections (lazy — registered but not connected until first use)
		if len(d.MCP.Servers) > 0 && m.mcpMgr != nil {
			m.mcpMgr.Register(d.ID, d.MCP)
		}

		// Bus registration
		if m.bus != nil {
			m.bus.Register(d)
		}

		// Economics
		if m.costTracker != nil && (d.Economics.BudgetPerRun > 0 || d.Economics.DailyBudget > 0) {
			m.costTracker.SetConfig(d.ID, d.Economics)
		}
	}
	for id, store := range m.experiences {
		if !seen[id] {
			if store != nil {
				store.Flush() // persist debounced outcomes before dropping
			}
			delete(m.experiences, id)
		}
	}
}

func (m *cogniModule) clearCogniRuntimeState() {
	for id, store := range m.experiences {
		if store != nil {
			store.Flush() // persist debounced outcomes before dropping
		}
		delete(m.experiences, id)
	}
	if m.mcpMgr != nil {
		m.mcpMgr.CloseAll()
	}
	if m.bus != nil {
		m.bus.Clear()
	}
}

func (m *cogniModule) Stop() error {
	if m.scheduler != nil {
		m.scheduler.Stop()
	}
	if m.sentinel != nil {
		m.sentinel.Stop()
	}
	if m.mcpMgr != nil {
		m.mcpMgr.CloseAll()
	}
	if m.fileLog != nil {
		_ = m.fileLog.Close()
	}
	m.runtimeMu.Lock()
	m.runtimeActive = false
	m.runtimeMu.Unlock()
	return nil
}

// mcpProviderBridge adapts internal/mcp.Provider to cogni.MCPConnection.
type mcpProviderBridge struct {
	provider mcpkg.Provider
	stopper  interface{ Stop() }
}

func (b *mcpProviderBridge) ListTools(ctx context.Context) ([]cogni.MCPToolInfo, error) {
	tools, err := b.provider.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]cogni.MCPToolInfo, len(tools))
	for i, t := range tools {
		out[i] = cogni.MCPToolInfo{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}
	return out, nil
}

func (b *mcpProviderBridge) CallTool(ctx context.Context, name string, args map[string]any) (any, error) {
	result, err := b.provider.CallTool(ctx, name, args)
	if err != nil {
		return nil, err
	}
	if result.IsError && len(result.Content) > 0 {
		return nil, fmt.Errorf("%s", result.Content[0].Text)
	}
	if len(result.Content) > 0 {
		return result.Content[0].Text, nil
	}
	return "", nil
}

func (b *mcpProviderBridge) Close() error {
	if b.stopper != nil {
		b.stopper.Stop()
	}
	// The internal/mcp Provider interface has no Stop(), but the concrete
	// stdio/streamable-http providers do. Without this, Close() was a no-op and
	// every disconnected cogni MCP server leaked its child process / connection.
	if s, ok := b.provider.(interface{ Stop() }); ok {
		s.Stop()
	}
	return nil
}

// cogniAutoDisableFromEnv reads COGNI_AUTO_DISABLE (default false). When
// true, the sentinel disables cognis whose score stays critical.
func cogniAutoDisableFromEnv() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("COGNI_AUTO_DISABLE")))
	return v == "true" || v == "1" || v == "yes" || v == "on"
}

// cogniArbitrationFromEnv reads per-turn capability arbitration settings:
//   - COGNI_MAX_ACTIVE_COGNIS: cap how many cognis compose per turn (top-K).
//   - COGNI_MIN_CONFIDENCE: drop activations below this score floor.
//
// Both default to 0 (disabled) so the legacy "every activated cogni composes"
// behavior is preserved unless an operator opts in.
func cogniArbitrationFromEnv() cogni.ArbitrationConfig {
	cfg := cogni.ArbitrationConfig{}
	if v := strings.TrimSpace(os.Getenv("COGNI_MAX_ACTIVE_COGNIS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxActive = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("COGNI_MIN_CONFIDENCE")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			cfg.MinConfidence = f
		}
	}
	return cfg
}

// cogniExperienceTuningFromEnv reads experience-driven surface pruning settings:
//   - COGNI_EXP_MIN_OBSERVATIONS: minimum observations before a tool can be pruned.
//   - COGNI_EXP_MIN_SUCCESS: success-rate floor below which a tool is pruned.
//
// Both default to 0 (disabled) so recording accrues but pruning stays inert
// until an operator opts in.
func cogniExperienceTuningFromEnv() cogni.ExperienceTuningConfig {
	cfg := cogni.ExperienceTuningConfig{}
	if v := strings.TrimSpace(os.Getenv("COGNI_EXP_MIN_OBSERVATIONS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MinObservations = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("COGNI_EXP_MIN_SUCCESS")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			cfg.MinSuccessRate = f
		}
	}
	return cfg
}

// cogniContextByteBudgetFromEnv reads COGNI_CONTEXT_BYTE_BUDGET: the max
// total size (bytes) of the context block BuildContext assembles per turn.
// Unset or <= 0 (the default) disables enforcement, preserving today's
// unbounded concatenation until an operator opts in.
func cogniContextByteBudgetFromEnv() int {
	v := strings.TrimSpace(os.Getenv("COGNI_CONTEXT_BYTE_BUDGET"))
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func (m *cogniModule) Status() agentrt.ModuleStatus {
	enabled := m.registry != nil && m.cogniKernelPackEnabled()
	return agentrt.ModuleStatus{
		Name:        m.Name(),
		Description: m.Description(),
		Profile:     m.Profile(),
		Enabled:     enabled,
		Running:     enabled && m.cogniRuntimeActive(),
	}
}

// applyMutationToDecl applies one evolution mutation onto a (deep-copied)
// declaration using typed, guaranteed-valid field writes. It deliberately only
// touches the two safe, behavior-relevant surfaces — the injected prompt text
// (Context.Static) and activation sensitivity (Activation.*) — so a malformed
// LLM mutation can never produce an invalid declaration. Unknown shapes are
// no-ops; the engine's bench gate then reverts the empty change.
func applyMutationToDecl(d *cogni.Declaration, mut cogni.SkillMutation) {
	switch mut.MutationType {
	case "prompt":
		txt := afterString(mut.After, "static", "prompt", "text", "context", "instruction")
		if txt == "" {
			txt = strings.TrimSpace(mut.Rationale)
			if txt != "" {
				txt = "[evolution] " + txt
			}
		}
		if txt == "" {
			return
		}
		if strings.TrimSpace(d.Context.Static) != "" {
			d.Context.Static += "\n\n" + txt
		} else {
			d.Context.Static = txt
		}
	case "parameter":
		if v, ok := afterFloat(mut.After, "min_score"); ok {
			d.Activation.MinScore = clamp01(v)
		}
		if v, ok := afterFloat(mut.After, "keyword_weight"); ok {
			d.Activation.KeywordWeight = clamp01(v)
		}
		if v, ok := afterFloat(mut.After, "regex_weight"); ok {
			d.Activation.RegexWeight = clamp01(v)
		}
		for _, kw := range afterStrings(mut.After, "keywords") {
			if !containsString(d.Activation.Keywords, kw) {
				d.Activation.Keywords = append(d.Activation.Keywords, kw)
			}
		}
	default:
		// "timeout" / "new_skill" and any unknown type are intentionally not
		// auto-applied (resource-budget and capability-surface changes are out
		// of scope for safe auto-evolution).
	}
}

func afterString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func afterFloat(m map[string]any, key string) (float64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f, err == nil
	}
	return 0, false
}

func afterStrings(m map[string]any, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	switch arr := v.(type) {
	case []string:
		return arr
	case []any:
		out := make([]string, 0, len(arr))
		for _, it := range arr {
			if s, ok := it.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case string:
		if strings.TrimSpace(arr) != "" {
			return []string{strings.TrimSpace(arr)}
		}
	}
	return nil
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func containsString(list []string, s string) bool {
	for _, it := range list {
		if it == s {
			return true
		}
	}
	return false
}
