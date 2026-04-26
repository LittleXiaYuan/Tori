package localbrain

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	ldg "github.com/LittleXiaYuan/ledger"
)

// TrainingPipeline 从 Ledger 推理轨迹中提取 LoRA 微调训练数据。
//
// 工作原理（对应林俊旸文章中的 "Agentic RL" 基础设施需求）：
//   1. Ledger 的 ReasoningTracer 记录每一步 Think/Observe/Decide/Backtrack
//   2. 这些 trace 天然就是 RL 的 trajectory 数据
//   3. RewardModel 对轨迹打分（成功/失败/效率）
//   4. 打分后的数据导出为 JSONL，用于 LoRA 微调小模型
//   5. 微调后的小模型替换 LocalBrain.client → 形成正循环
type TrainingPipeline struct {
	ledger   *ldg.Ledger
	brain    *LocalBrain
	dataDir  string
	rewarder RewardModel
}

// RewardModel 对一条推理轨迹打分。
type RewardModel interface {
	Score(ctx context.Context, trace *ldg.ReasoningTrace, taskSuccess bool) float64
}

// NewTrainingPipeline 创建训练管线。
func NewTrainingPipeline(ledger *ldg.Ledger, brain *LocalBrain, dataDir string) *TrainingPipeline {
	return &TrainingPipeline{
		ledger:   ledger,
		brain:    brain,
		dataDir:  dataDir,
		rewarder: &DefaultRewardModel{},
	}
}

// SetRewardModel 替换默认奖励模型。
func (tp *TrainingPipeline) SetRewardModel(rm RewardModel) {
	tp.rewarder = rm
}

// TrajectoryRecord 是一条完整的训练轨迹记录。
type TrajectoryRecord struct {
	TaskID      string          `json:"task_id"`
	Trajectory  []TrajectoryStep `json:"trajectory"`
	Reward      float64         `json:"reward"`
	TaskSuccess bool            `json:"task_success"`
	ExportedAt  time.Time       `json:"exported_at"`
}

// TrajectoryStep 是轨迹中的单步。
type TrajectoryStep struct {
	StepType   string  `json:"step_type"`   // think/observe/decide/backtrack
	Content    string  `json:"content"`
	Confidence float64 `json:"confidence,omitempty"`
	Decision   string  `json:"decision,omitempty"`
	Reason     string  `json:"reason,omitempty"`
}

// ExtractTrajectory 从 Ledger 中提取指定任务的推理轨迹。
func (tp *TrainingPipeline) ExtractTrajectory(ctx context.Context, taskID string, taskSuccess bool) (*TrajectoryRecord, error) {
	trace, err := tp.ledger.Events.GetReasoningTrace(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get trace: %w", err)
	}
	if trace == nil || len(trace.Events) == 0 {
		return nil, fmt.Errorf("no reasoning trace for task %s", taskID)
	}

	record := &TrajectoryRecord{
		TaskID:      taskID,
		TaskSuccess: taskSuccess,
		ExportedAt:  time.Now(),
	}

	for _, evt := range trace.Events {
		step := TrajectoryStep{}
		var payload map[string]interface{}
		if json.Unmarshal(evt.Payload, &payload) == nil {
			if thought, ok := payload["thought"].(string); ok {
				step.Content = thought
			}
			if observation, ok := payload["observation"].(string); ok {
				step.Content = observation
			}
			if conf, ok := payload["confidence"].(float64); ok {
				step.Confidence = conf
			}
			if decision, ok := payload["decision"].(string); ok {
				step.Decision = decision
			}
			if reason, ok := payload["reason"].(string); ok {
				step.Reason = reason
			}
		}

		switch {
		case string(evt.Kind) == "reasoning.thought":
			step.StepType = "think"
		case string(evt.Kind) == "reasoning.observe":
			step.StepType = "observe"
		case string(evt.Kind) == "reasoning.decision":
			step.StepType = "decide"
		case string(evt.Kind) == "reasoning.backtrack":
			step.StepType = "backtrack"
		case string(evt.Kind) == "reasoning.hypothesis":
			step.StepType = "hypothesize"
		case string(evt.Kind) == "reasoning.reflect":
			step.StepType = "reflect"
		default:
			step.StepType = string(evt.Kind)
		}

		record.Trajectory = append(record.Trajectory, step)
	}

	// 使用奖励模型打分
	record.Reward = tp.rewarder.Score(ctx, trace, taskSuccess)

	return record, nil
}

// ExportBatch 批量导出多个任务的轨迹为 JSONL 文件。
// 格式兼容 DPO (Direct Preference Optimization) 和 SFT (Supervised Fine-Tuning)。
func (tp *TrainingPipeline) ExportBatch(ctx context.Context, taskIDs []string, taskResults map[string]bool) (string, int, error) {
	if err := os.MkdirAll(tp.dataDir, 0750); err != nil {
		return "", 0, fmt.Errorf("create data dir: %w", err)
	}

	filename := fmt.Sprintf("trajectory_%s.jsonl", time.Now().Format("20060102_150405"))
	outPath := filepath.Join(tp.dataDir, filename)

	f, err := os.Create(outPath)
	if err != nil {
		return "", 0, fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	exported := 0

	for _, taskID := range taskIDs {
		success := taskResults[taskID]
		record, err := tp.ExtractTrajectory(ctx, taskID, success)
		if err != nil {
			slog.Debug("training: skip task", "task", taskID, "err", err)
			continue
		}

		// 转换为 SFT 格式：每个决策点 → 一条训练样本
		for _, step := range record.Trajectory {
			if step.StepType != "decide" {
				continue
			}
			decisionJSON, _ := json.Marshal(map[string]interface{}{
				"decision":   step.Decision,
				"reason":     step.Reason,
				"confidence": step.Confidence,
			})
			sample := map[string]interface{}{
				"instruction": "You are an agentic decision maker. Given the context, decide whether to use a tool, think deeper, or answer directly.",
				"input":       step.Content,
				"output":      string(decisionJSON),
				"reward":      record.Reward,
				"task_success": record.TaskSuccess,
			}
			if err := enc.Encode(sample); err != nil {
				continue
			}
			exported++
		}

		// 同时导出完整轨迹用于 DPO
		if err := enc.Encode(record); err != nil {
			continue
		}
		exported++
	}

	slog.Info("training: exported", "path", outPath, "samples", exported)
	return outPath, exported, nil
}

// ── 默认奖励模型 ──

// DefaultRewardModel 基于简单启发式的奖励模型。
type DefaultRewardModel struct{}

func (d *DefaultRewardModel) Score(_ context.Context, trace *ldg.ReasoningTrace, taskSuccess bool) float64 {
	if trace == nil || trace.Summary == nil {
		return 0.0
	}

	score := 0.0
	s := trace.Summary

	// 基础分：任务是否成功
	if taskSuccess {
		score += 0.5
	}

	// 效率奖励：步骤越少越好
	if s.TotalSteps <= 3 {
		score += 0.2
	} else if s.TotalSteps <= 6 {
		score += 0.1
	}

	// 回溯惩罚：回溯越多越差
	if s.Backtracks == 0 {
		score += 0.15
	} else if s.Backtracks > 3 {
		score -= 0.1
	}

	// 平均置信度奖励
	if s.AvgConfidence > 0.7 {
		score += 0.1
	} else if s.AvgConfidence < 0.3 {
		score -= 0.1
	}

	// 有反思的正向信号
	if s.Reflections > 0 {
		score += 0.05
	}

	// 边界值
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}
