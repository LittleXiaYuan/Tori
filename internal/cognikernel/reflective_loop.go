package cognikernel

import (
	"context"
	"log/slog"
	"time"
)

// ReflectiveLoop automates the post-conversation reflection pipeline:
//
//	conversation ends → Reflect.Evaluate() → ExperienceStore.Add()
//	→ Strategy.Compile() → Distiller.Distill() → SelfDistillSink
//
// This loop runs asynchronously after each conversation, ensuring every
// interaction contributes to the agent's learning spiral.
type ReflectiveLoop struct {
	reflectFn    ReflectEvalFunc
	experienceFn ExperienceRecordFunc
	distillFn    DistillFunc
	selfDistill  SelfDistillSink
	memoryUpdate MemoryUpdateFunc
}

// ReflectEvalFunc evaluates conversation quality.
// Returns quality score (1-10), satisfaction, issues, and suggested memory updates.
type ReflectEvalFunc func(ctx context.Context, intent, reply string, skillResults []string) (*ReflectEvalResult, error)

// ReflectEvalResult is the output of a reflection evaluation.
type ReflectEvalResult struct {
	Satisfied     bool           `json:"satisfied"`
	Quality       int            `json:"quality"`
	Issues        []string       `json:"issues"`
	Suggestions   []string       `json:"suggestions"`
	MemoryUpdates []MemUpdateReq `json:"memory_updates,omitempty"`
}

// MemUpdateReq is a memory update requested by reflection.
type MemUpdateReq struct {
	Action string `json:"action"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}

// ExperienceRecordFunc records a structured experience entry.
type ExperienceRecordFunc func(source, category, outcome, lesson, ctx string, tags []string)

// DistillFunc distills an expert-tier response into a reusable rule.
type DistillFunc func(ctx context.Context, question, expertReply string)

// SelfDistillSink pushes high-quality samples to the LoRA training pipeline.
type SelfDistillSink interface {
	PushSample(ctx context.Context, sample TrainingSample) error
}

// TrainingSample is a training data point for the self-distillation pipeline.
type TrainingSample struct {
	Input      string   `json:"input"`
	Output     string   `json:"output"`
	Score      float64  `json:"score"`
	ModelTier  string   `json:"model_tier"`
	SkillsUsed []string `json:"skills_used"`
	TenantID   string   `json:"tenant_id"`
}

// MemoryUpdateFunc applies a memory update from reflection.
type MemoryUpdateFunc func(ctx context.Context, action, key, value string) error

// NewReflectiveLoop creates the reflective loop pipeline.
func NewReflectiveLoop() *ReflectiveLoop {
	return &ReflectiveLoop{}
}

func (rl *ReflectiveLoop) SetReflectEval(fn ReflectEvalFunc)       { rl.reflectFn = fn }
func (rl *ReflectiveLoop) SetExperienceRecord(fn ExperienceRecordFunc) { rl.experienceFn = fn }
func (rl *ReflectiveLoop) SetDistill(fn DistillFunc)               { rl.distillFn = fn }
func (rl *ReflectiveLoop) SetSelfDistillSink(sink SelfDistillSink) { rl.selfDistill = sink }
func (rl *ReflectiveLoop) SetMemoryUpdate(fn MemoryUpdateFunc)     { rl.memoryUpdate = fn }

// Run executes the full reflective pipeline for one conversation.
func (rl *ReflectiveLoop) Run(ctx context.Context, data ConversationEndData) (*ReflectResult, error) {
	result := &ReflectResult{}

	// Step 1: Evaluate conversation quality
	if rl.reflectFn != nil {
		eval, err := rl.reflectFn(ctx, data.UserIntent, data.AgentReply, data.SkillsUsed)
		if err != nil {
			slog.Warn("reflective_loop: evaluation failed", "err", err)
		} else {
			result.Satisfied = eval.Satisfied
			result.Quality = eval.Quality
			result.Score = float64(eval.Quality) / 10.0

			// Step 1b: Apply memory updates suggested by reflection
			if rl.memoryUpdate != nil {
				for _, mu := range eval.MemoryUpdates {
					if err := rl.memoryUpdate(ctx, mu.Action, mu.Key, mu.Value); err != nil {
						slog.Warn("reflective_loop: memory update failed",
							"action", mu.Action, "key", mu.Key, "err", err)
					} else {
						result.MemoryUpdates++
					}
				}
			}
		}
	}

	// Step 2: Record structured experience
	if rl.experienceFn != nil {
		outcome := "success"
		if result.Quality < 5 {
			outcome = "failure"
		} else if result.Quality < 7 {
			outcome = "partial"
		}

		lesson := buildLesson(data, result)
		rl.experienceFn(
			"interaction",
			categorizeInteraction(data.SkillsUsed),
			outcome,
			lesson,
			data.UserIntent,
			data.SkillsUsed,
		)
		result.ExperiencesAdded++
	}

	// Step 3: Knowledge distillation (Expert-tier responses only)
	if rl.distillFn != nil && data.ModelTier == "expert" && len(data.AgentReply) > 200 {
		rl.distillFn(ctx, data.UserIntent, data.AgentReply)
		result.DistilledRules++
	}

	// Step 4: Push to self-distillation sink (high-quality samples)
	if rl.selfDistill != nil && result.Quality >= 8 {
		sample := TrainingSample{
			Input:      data.UserIntent,
			Output:     data.AgentReply,
			Score:      float64(result.Quality),
			ModelTier:  data.ModelTier,
			SkillsUsed: data.SkillsUsed,
			TenantID:   data.TenantID,
		}
		if err := rl.selfDistill.PushSample(ctx, sample); err != nil {
			slog.Warn("reflective_loop: self-distill push failed", "err", err)
		}
	}

	return result, nil
}

func buildLesson(data ConversationEndData, result *ReflectResult) string {
	if result.Quality >= 8 {
		skills := "direct response"
		if len(data.SkillsUsed) > 0 {
			skills = joinStrings(data.SkillsUsed, ", ")
		}
		return "高质量回答（" + skills + "），用户意图被充分满足"
	}
	if result.Quality < 5 {
		return "回复质量不足，可能未正确理解用户意图或工具使用不当"
	}
	return "回复基本满足但有改进空间"
}

func categorizeInteraction(skillsUsed []string) string {
	if len(skillsUsed) == 0 {
		return "direct_response"
	}
	return "skill_usage"
}

func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for _, s := range ss[1:] {
		result += sep + s
	}
	return result
}

// ── SelfDistillAdapter bridges the existing DataCollector to our interface ──

// DataCollectorAdapter wraps an existing DataCollector into SelfDistillSink.
type DataCollectorAdapter struct {
	collectFn func(ctx context.Context, input, output string, score float64, tier string, skills []string) error
}

func NewDataCollectorAdapter(fn func(ctx context.Context, input, output string, score float64, tier string, skills []string) error) *DataCollectorAdapter {
	return &DataCollectorAdapter{collectFn: fn}
}

func (a *DataCollectorAdapter) PushSample(ctx context.Context, sample TrainingSample) error {
	if a.collectFn == nil {
		return nil
	}
	return a.collectFn(ctx, sample.Input, sample.Output, sample.Score, sample.ModelTier, sample.SkillsUsed)
}

// ── Noop sink for when no training pipeline is configured ──

type noopSink struct{}

func (noopSink) PushSample(context.Context, TrainingSample) error { return nil }

// NoopSelfDistillSink returns a no-op sink.
func NoopSelfDistillSink() SelfDistillSink { return noopSink{} }

// ── Conversation-level experience extraction helpers ──

// ReflectAndLearn is a convenience method that wraps the full reflective pipeline
// for use as a simple callback. It returns true if the conversation was satisfactory.
func (rl *ReflectiveLoop) ReflectAndLearn(ctx context.Context, tenantID, sessionID, intent, reply string, skillsUsed []string, modelTier string) bool {
	data := ConversationEndData{
		TenantID:   tenantID,
		SessionID:  sessionID,
		UserIntent: intent,
		AgentReply: reply,
		SkillsUsed: skillsUsed,
		ModelTier:  modelTier,
		Duration:   time.Duration(0),
	}

	result, err := rl.Run(ctx, data)
	if err != nil {
		return true
	}
	return result.Satisfied
}
