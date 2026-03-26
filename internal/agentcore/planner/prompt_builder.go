package planner

import (
	"context"

	ctxwindow "yunque-agent/internal/agentcore/context"
	"yunque-agent/internal/agentcore/emotion"
)

// CognitiveContextFunc collects dynamic context from all CognitivePlugins.
type CognitiveContextFunc func(ctx context.Context, userMessage string) string

// PromptBuilder assembles the 5 dynamic context layers (P1–P5)
// used by BuildMessages. Extracting this logic makes it testable
// and pluggable independently of the Planner's execution loop.
type PromptBuilder struct {
	// Data sources (injected from Planner's callbacks)
	memory           func(ctx context.Context, tenantID, query string) string
	graphContext     func(query string) string
	codeContext      func(query string) string
	stateContext     func() string
	strategyContext  func() string
	reverie          *Reverie
	skillOptimizer   *SkillOptimizer
	cognitiveContext CognitiveContextFunc // CognitivePlugin dynamic context
	dynBudget        int
}

// NewPromptBuilder creates a PromptBuilder from Planner's callbacks.
func NewPromptBuilder(p *Planner) *PromptBuilder {
	return &PromptBuilder{
		memory:           p.memory,
		graphContext:     p.graphContext,
		codeContext:      p.codeContext,
		stateContext:     p.stateContext,
		strategyContext:  p.strategyContext,
		reverie:          p.reverie,
		skillOptimizer:   p.skillOptimizer,
		cognitiveContext: p.cognitiveContext,
		dynBudget:        p.dynContextBudget,
	}
}

// DynamicContextRequest carries per-request info needed for layer assembly.
type DynamicContextRequest struct {
	LastMessage string
	TenantID    string
	TaskContext string
	EmotionHint *emotion.Result
}

// BuildDynamicContext assembles the 5-priority dynamic context layers and
// returns the assembled text ready to be injected as a system message.
// Returns "" if no layers have content.
func (pb *PromptBuilder) BuildDynamicContext(ctx context.Context, req DynamicContextRequest) string {
	var layers []ctxwindow.Layer

	// P1: Task working memory (highest dynamic priority)
	if req.TaskContext != "" {
		layers = append(layers, ctxwindow.Layer{
			Name:     "task",
			Priority: ctxwindow.LayerPriorityTask,
			Content:  req.TaskContext,
		})
	}

	// P2+P3: Memory, graph, and code retrieval run in parallel
	// These are independent I/O calls — parallelizing saves ~200-500ms per request
	type layerResult struct {
		name     string
		priority ctxwindow.LayerPriority
		prefix   string
		content  string
	}
	results := make(chan layerResult, 3)
	pending := 0

	if pb.memory != nil {
		pending++
		go func() {
			if memCtx := pb.memory(ctx, req.TenantID, req.LastMessage); memCtx != "" {
				results <- layerResult{"memory", ctxwindow.LayerPriorityMemory, "## 记忆上下文\n", memCtx}
			} else {
				results <- layerResult{}
			}
		}()
	}
	if pb.graphContext != nil {
		pending++
		go func() {
			if graphCtx := pb.graphContext(req.LastMessage); graphCtx != "" {
				results <- layerResult{"graph", ctxwindow.LayerPriorityRetrieval, "## 知识图谱\n", graphCtx}
			} else {
				results <- layerResult{}
			}
		}()
	}
	if pb.codeContext != nil {
		pending++
		go func() {
			if codeCtx := pb.codeContext(req.LastMessage); codeCtx != "" {
				results <- layerResult{"code", ctxwindow.LayerPriorityRetrieval, "", codeCtx}
			} else {
				results <- layerResult{}
			}
		}()
	}

	// Collect parallel results
	for i := 0; i < pending; i++ {
		r := <-results
		if r.content != "" {
			layers = append(layers, ctxwindow.Layer{
				Name:     r.name,
				Priority: r.priority,
				Content:  r.prefix + r.content,
			})
		}
	}

	// P3.5: CognitivePlugin dynamic context — domain-specific knowledge
	// injected by plugins that implement CognitivePlugin.DynamicContext()
	if pb.cognitiveContext != nil {
		if cogCtx := pb.cognitiveContext(ctx, req.LastMessage); cogCtx != "" {
			layers = append(layers, ctxwindow.Layer{
				Name:     "cognitive_plugins",
				Priority: ctxwindow.LayerPriorityRetrieval,
				Content:  cogCtx,
			})
		}
	}

	// P4: Cognition — only inject what's genuinely relevant to the current query.
	// Emotion and strategy have higher priority than reverie (which is speculative).
	if req.EmotionHint != nil {
		if snippet := req.EmotionHint.ContextSnippet(); snippet != "" {
			layers = append(layers, ctxwindow.Layer{
				Name:     "emotion",
				Priority: ctxwindow.LayerPriorityCognition,
				Content:  "## 情绪感知\n" + snippet,
			})
		}
	}
	if pb.strategyContext != nil {
		if strCtx := pb.strategyContext(); strCtx != "" {
			layers = append(layers, ctxwindow.Layer{
				Name:     "strategy",
				Priority: ctxwindow.LayerPriorityCognition,
				Content:  "## 行动指南（基于过去经验）\n" + strCtx,
			})
		}
	}
	if pb.stateContext != nil {
		if sCtx := pb.stateContext(); sCtx != "" {
			layers = append(layers, ctxwindow.Layer{
				Name:     "state",
				Priority: ctxwindow.LayerPriorityCognition,
				Content:  sCtx,
			})
		}
	}
	// Reverie is lowest-priority cognition — only inject when genuinely relevant
	if pb.reverie != nil {
		if jctx := pb.reverie.JournalContext(2, req.LastMessage); jctx != "" {
			layers = append(layers, ctxwindow.Layer{
				Name:     "reverie",
				Priority: ctxwindow.LayerPriorityHints, // demoted: hints level, not cognition
				Content:  jctx,
			})
		}
	}

	// P5: Hints — skill optimization
	if pb.skillOptimizer != nil {
		if hints := pb.skillOptimizer.OptimizationHints(); hints != "" {
			layers = append(layers, ctxwindow.Layer{
				Name:     "skill_hints",
				Priority: ctxwindow.LayerPriorityHints,
				Content:  "## 技能优化\n" + hints,
			})
		}
	}

	// Assemble layers within token budget with anti-pollution features:
	// - Per-layer limit prevents any single source from dominating
	// - Dedup removes repeated sentences across layers
	// - Partial truncation instead of full drop preserves partial context
	budget := pb.dynBudget
	if budget <= 0 {
		budget = 4000
	}
	perLayer := budget / 3 // no single layer gets more than ~33% of budget
	if perLayer < 500 {
		perLayer = 500
	}
	assembler := ctxwindow.NewLayerAssembler(budget).
		WithPerLayerLimit(perLayer).
		WithDedup()
	assembled, _ := assembler.Assemble(layers)
	return assembled
}
