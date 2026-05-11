package localbrain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"yunque-agent/pkg/cogni"
)

// IntentTrainingSample is the unified training format for the intent
// recognition small model. It supports both the LocalBrain routing
// classifier (chat/code/search/tool/complex) and the NL config
// translator (35 specific intents).
//
// Training data format (ChatML JSONL for Qwen2.5 / LLaMA-Factory):
//
//	{"messages":[
//	  {"role":"system","content":"<system_prompt>"},
//	  {"role":"user","content":"<user_query>"},
//	  {"role":"assistant","content":"<structured_json_output>"}
//	]}
type IntentTrainingSample struct {
	// Input fields
	UserQuery string `json:"user_query"`
	TenantID  string `json:"tenant_id,omitempty"`

	// Classification output — routing tier
	RouteIntent    Intent `json:"route_intent"`
	RouteTier      string `json:"route_tier"`
	RouteUpgraded  bool   `json:"route_upgraded"`
	RouteSatisfied bool   `json:"route_satisfied"`

	// Classification output — NL config intent (if applicable)
	NLIntent     cogni.IntentType     `json:"nl_intent,omitempty"`
	NLCategory   cogni.IntentCategory `json:"nl_category,omitempty"`
	NLConfidence float64              `json:"nl_confidence,omitempty"`
	NLParams     map[string]any       `json:"nl_params,omitempty"`

	// Quality metadata
	Score     float64   `json:"score"`
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`
}

// IntentClassifierOutput is the structured JSON that the fine-tuned
// model should produce. It combines routing + NL config classification
// in a single forward pass.
type IntentClassifierOutput struct {
	// Routing classification (always present)
	Category   string  `json:"category"`
	Complexity string  `json:"complexity"`
	Confidence float64 `json:"confidence"`
	NeedTools  bool    `json:"need_tools"`

	// NL config classification (present when category == "config")
	NLIntent   string         `json:"nl_intent,omitempty"`
	NLCategory string         `json:"nl_category,omitempty"`
	NLParams   map[string]any `json:"nl_params,omitempty"`
}

const intentClassifierSystemPrompt = `You are a unified intent classifier for the Yunque AI assistant. Given a user query, output ONLY valid JSON with two levels of classification:

Level 1 — Routing: Determine the general category and complexity.
Level 2 — NL Config: If the user is making a configuration request, identify the specific intent.

Output format:
{"category":"chat|code|search|tool|config|complex","complexity":"simple|medium|hard","confidence":0.0-1.0,"need_tools":true|false,"nl_intent":"<specific_intent_or_empty>","nl_category":"<intent_category_or_empty>","nl_params":{}}

Rules:
- "chat": greetings, chitchat, simple Q&A
- "code": coding, debugging, code review
- "search": web search, document lookup
- "tool": file ops, shell commands, API calls
- "config": user wants to change settings, manage knowledge, schedule tasks, adjust UI/persona
- "complex": multi-step reasoning, planning, analysis
- For "config" category, always populate nl_intent and nl_category
- nl_intent values: scheduler_create|scheduler_list|scheduler_remove|channel_send|browser_task|kb_add|kb_add_url|kb_remove|kb_list|kb_search|kb_stats|kb_ingest|model_switch|model_tier|provider_add|output_lang|output_style|search_toggle|memory_add|memory_forget|memory_recall|ui_mode|ui_theme|ui_font|ui_zen|system_info|usage_stats|data_backup|skill_install|audit_log|persona_name|persona_style|persona_role|persona_reset|cogni_create`

// ExportIntentTrainingData generates ChatML-format JSONL training data
// from the LocalBrain's accumulated feedback history plus synthetic
// NL config examples. This is the primary data source for fine-tuning
// the intent recognition small model.
func ExportIntentTrainingData(brain *LocalBrain, tenantID, outDir string) (string, int, error) {
	if err := os.MkdirAll(outDir, 0750); err != nil {
		return "", 0, fmt.Errorf("create output dir: %w", err)
	}

	filename := fmt.Sprintf("intent_train_%s_%s.jsonl", tenantID, time.Now().Format("20060102_150405"))
	outPath := filepath.Join(outDir, filename)

	f, err := os.Create(outPath)
	if err != nil {
		return "", 0, fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	count := 0

	// Source 1: Brain's routing feedback history
	for _, ts := range brain.ExportTrainingData(tenantID) {
		output := IntentClassifierOutput{
			Category:   ts.Intent.Category,
			Complexity: ts.Intent.Complexity,
			Confidence: ts.Intent.Confidence,
			NeedTools:  ts.Intent.NeedTools,
		}
		outputJSON, _ := json.Marshal(output)

		record := chatMLRecord(intentClassifierSystemPrompt, ts.Input, string(outputJSON))
		if err := enc.Encode(record); err != nil {
			continue
		}
		count++
	}

	// Source 2: Synthetic NL config examples from IntentRegistry
	for intentType, meta := range cogni.IntentRegistry {
		for _, example := range meta.Examples {
			output := IntentClassifierOutput{
				Category:   "config",
				Complexity: "simple",
				Confidence: 0.95,
				NeedTools:  meta.RequiresLLM,
				NLIntent:   string(intentType),
				NLCategory: string(meta.Category),
			}
			outputJSON, _ := json.Marshal(output)

			record := chatMLRecord(intentClassifierSystemPrompt, example, string(outputJSON))
			if err := enc.Encode(record); err != nil {
				continue
			}
			count++
		}
	}

	return outPath, count, nil
}

func chatMLRecord(system, user, assistant string) map[string]any {
	return map[string]any{
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
			{"role": "assistant", "content": assistant},
		},
	}
}

// SelfDistillSink is the interface that the CogniKernel's ReflectiveLoop
// uses to feed conversation outcomes back into the intent classifier's
// training pipeline. This is the bridge between the cognitive core and
// the self-distillation system.
type SelfDistillSink interface {
	// IngestConversation receives a completed conversation for scoring
	// and potential inclusion in the next training batch. The feedback
	// includes the original query, the chosen route, and the outcome.
	IngestConversation(sample IntentTrainingSample) error

	// FlushBatch writes accumulated samples to disk in ChatML JSONL
	// format, ready for the LoRA training pipeline to pick up.
	FlushBatch(tenantID string) (path string, count int, err error)

	// Stats returns the current accumulation counts.
	Stats() SinkStats
}

// SinkStats reports the state of the distillation sink.
type SinkStats struct {
	Pending   int       `json:"pending"`
	Flushed   int       `json:"flushed"`
	LastFlush time.Time `json:"last_flush"`
}

// InMemorySink is a simple in-process implementation of SelfDistillSink.
// Production deployments would use a persistent queue or database.
type InMemorySink struct {
	samples   []IntentTrainingSample
	dataDir   string
	flushed   int
	lastFlush time.Time
}

// NewInMemorySink creates a sink that accumulates samples in memory.
func NewInMemorySink(dataDir string) *InMemorySink {
	return &InMemorySink{dataDir: dataDir}
}

func (s *InMemorySink) IngestConversation(sample IntentTrainingSample) error {
	if sample.UserQuery == "" {
		return fmt.Errorf("empty user query")
	}
	sample.Timestamp = time.Now()
	s.samples = append(s.samples, sample)
	return nil
}

func (s *InMemorySink) FlushBatch(tenantID string) (string, int, error) {
	if len(s.samples) == 0 {
		return "", 0, nil
	}

	if err := os.MkdirAll(s.dataDir, 0750); err != nil {
		return "", 0, fmt.Errorf("create data dir: %w", err)
	}

	filename := fmt.Sprintf("distill_sink_%s_%s.jsonl", tenantID, time.Now().Format("20060102_150405"))
	outPath := filepath.Join(s.dataDir, filename)

	f, err := os.Create(outPath)
	if err != nil {
		return "", 0, fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	written := 0
	for _, sample := range s.samples {
		if sample.Score < 0.5 {
			continue
		}
		output := IntentClassifierOutput{
			Category:   sample.RouteIntent.Category,
			Complexity: sample.RouteIntent.Complexity,
			Confidence: sample.RouteIntent.Confidence,
			NeedTools:  sample.RouteIntent.NeedTools,
		}
		if sample.NLIntent != "" {
			output.NLIntent = string(sample.NLIntent)
			output.NLCategory = string(sample.NLCategory)
			output.NLParams = sample.NLParams
			output.Category = "config"
		}
		outputJSON, _ := json.Marshal(output)

		record := chatMLRecord(intentClassifierSystemPrompt, sample.UserQuery, string(outputJSON))
		if err := enc.Encode(record); err != nil {
			continue
		}
		written++
	}

	s.flushed += written
	s.lastFlush = time.Now()
	s.samples = nil

	return outPath, written, nil
}

func (s *InMemorySink) Stats() SinkStats {
	return SinkStats{
		Pending:   len(s.samples),
		Flushed:   s.flushed,
		LastFlush: s.lastFlush,
	}
}
