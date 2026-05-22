package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"yunque-agent/internal/agentcore/llm"
	ldg "yunque-agent/internal/ledgercore"
	"yunque-agent/pkg/safego"
)

// DataCollector captures successful conversation pairs as training-ready
// experience memories in Ledger. It runs as a non-blocking post-execution
// hook attached to Planner.Run().
//
// Collection criteria:
//   - Only completed (non-error) exchanges
//   - User message ≥ minQueryLen (skip greetings/noise)
//   - Assistant reply ≥ minReplyLen
//   - If reflection is available, only keep score ≥ minScore
//
// Data is stored as MemoryExperience entries tagged with source="training_data",
// then batch-exported by the nighttime scheduler via Ledger.ExportTrainingData().
var dataCollectorSeq atomic.Int64

type DataCollector struct {
	ledger *ldg.Ledger
	mu     sync.Mutex

	minQueryLen int
	minReplyLen int
	minScore    float64
	enabled     bool
}

// DataCollectorConfig configures the training data collector.
type DataCollectorConfig struct {
	MinQueryLen int     // minimum user message length (runes), default 10
	MinReplyLen int     // minimum assistant reply length (runes), default 20
	MinScore    float64 // minimum reflection score to keep, default 0.5
	Enabled     bool
}

// NewDataCollector creates a training data collector attached to a Ledger instance.
func NewDataCollector(l *ldg.Ledger, cfg DataCollectorConfig) *DataCollector {
	if cfg.MinQueryLen <= 0 {
		cfg.MinQueryLen = 10
	}
	if cfg.MinReplyLen <= 0 {
		cfg.MinReplyLen = 20
	}
	if cfg.MinScore <= 0 {
		cfg.MinScore = 0.5
	}
	return &DataCollector{
		ledger:      l,
		minQueryLen: cfg.MinQueryLen,
		minReplyLen: cfg.MinReplyLen,
		minScore:    cfg.MinScore,
		enabled:     cfg.Enabled,
	}
}

// conversationPair is the structured content stored in MemoryExperience.
type conversationPair struct {
	UserMessage string   `json:"user_message"`
	AssistReply string   `json:"assist_reply"`
	SkillsUsed  []string `json:"skills_used,omitempty"`
	Steps       int      `json:"steps"`
	TaskType    string   `json:"task_type"`
	ModelUsed   string   `json:"model_used,omitempty"`
}

// Collect records a successful conversation exchange.
// Called asynchronously from Planner.Run() — must not block.
func (dc *DataCollector) Collect(ctx context.Context, req PlanRequest, result *PlanResult, reflectScore float64) {
	if !dc.enabled || dc.ledger == nil {
		return
	}

	userMsg := extractGoal(req)
	if len([]rune(userMsg)) < dc.minQueryLen {
		return
	}
	if result == nil || len([]rune(result.Reply)) < dc.minReplyLen {
		return
	}
	if reflectScore > 0 && reflectScore < dc.minScore {
		return
	}

	pair := conversationPair{
		UserMessage: userMsg,
		AssistReply: result.Reply,
		SkillsUsed:  result.SkillsUsed,
		Steps:       result.Steps,
		TaskType:    classifyTaskType(req, result),
		ModelUsed:   req.ModelOverride,
	}

	content, err := json.Marshal(pair)
	if err != nil {
		slog.Warn("data_collector: marshal failed", "err", err)
		return
	}

	taskID := req.TaskID
	var taskPtr *string
	if taskID != "" {
		taskPtr = &taskID
	}

	key := fmt.Sprintf("train:%s-%d", time.Now().Format("20060102T150405"), dataCollectorSeq.Add(1))

	safego.Go("data-collector-store", func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := dc.ledger.Memory.Put(bgCtx, &ldg.MemoryEntry{
			TenantID:   req.TenantID,
			TaskID:     taskPtr,
			Kind:       ldg.MemoryExperience,
			Key:        key,
			Content:    string(content),
			Source:     "training_data",
			Confidence: clampScore(reflectScore),
			Metadata:   ldg.JSON(`{"collector":"auto","version":"1"}`),
		})
		if err != nil {
			slog.Warn("data_collector: store failed", "err", err, "tenant", req.TenantID)
		} else {
			slog.Debug("data_collector: stored training pair", "tenant", req.TenantID, "key", key)
		}
	})
}

func classifyTaskType(req PlanRequest, result *PlanResult) string {
	if len(result.SkillsUsed) > 0 {
		return "tool_use"
	}
	if result.Steps > 1 {
		return "multi_step"
	}
	msgs := req.Messages
	if len(msgs) > 2 {
		return "multi_turn"
	}
	return "single_turn"
}

func clampScore(score float64) float64 {
	if score <= 0 {
		return 0.6
	}
	if score > 1 {
		return 1
	}
	return score
}

// CollectFromMessages records training data from raw message pairs.
// Used by non-Planner paths (e.g. direct chat gateway).
func (dc *DataCollector) CollectFromMessages(ctx context.Context, tenantID string, messages []llm.Message) {
	if !dc.enabled || dc.ledger == nil || len(messages) < 2 {
		return
	}

	var lastUser, lastAssist string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" && lastAssist == "" {
			lastAssist = messages[i].Content
		}
		if messages[i].Role == "user" && lastUser == "" {
			lastUser = messages[i].Content
		}
		if lastUser != "" && lastAssist != "" {
			break
		}
	}

	if len([]rune(lastUser)) < dc.minQueryLen || len([]rune(lastAssist)) < dc.minReplyLen {
		return
	}

	pair := conversationPair{
		UserMessage: lastUser,
		AssistReply: lastAssist,
		TaskType:    "direct_chat",
	}

	content, _ := json.Marshal(pair)
	key := fmt.Sprintf("train:%s-%d", time.Now().Format("20060102T150405"), dataCollectorSeq.Add(1))

	safego.Go("data-collector-night", func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		dc.ledger.Memory.Put(bgCtx, &ldg.MemoryEntry{
			TenantID:   tenantID,
			Kind:       ldg.MemoryExperience,
			Key:        key,
			Content:    string(content),
			Source:     "training_data",
			Confidence: 0.6,
			Metadata:   ldg.JSON(`{"collector":"auto","version":"1"}`),
		})
	})
}
