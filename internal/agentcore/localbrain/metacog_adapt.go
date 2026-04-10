package localbrain

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// MetaCogAdapter 将元认知监控器的告警反馈到 AgenticThinking。
//
// 林俊旸文章核心论点：
//   "agent 需要知道自己不知道什么" — 这就是元认知。
//
// 实现：
//   - MetaCog 检测异常（循环、置信度下降、停滞）→ 发出 Alert
//   - MetaCogAdapter 接收 Alert → 动态调整 AgenticThinking 的参数
//   - 循环检测 → 强制 ThinkDeep + 回溯
//   - 置信度下降 → 升级模型层级
//   - 停滞 → 增加思考步骤上限
type MetaCogAdapter struct {
	mu       sync.Mutex
	thinking *AgenticThinking
	state    map[string]*TaskCogState // taskID → state
}

// TaskCogState 记录单个任务的元认知状态。
type TaskCogState struct {
	TaskID          string
	AlertCount      int
	LoopDetected    bool
	ConfidenceDrop  bool
	Stalled         bool
	ForceDeepThink  bool
	ForceExpert     bool
	AdjustedAt      time.Time
}

// NewMetaCogAdapter 创建元认知适配器。
func NewMetaCogAdapter(thinking *AgenticThinking) *MetaCogAdapter {
	return &MetaCogAdapter{
		thinking: thinking,
		state:    make(map[string]*TaskCogState),
	}
}

// AlertKind 告警类型（与 metacog 包的 AlertKind 对应）。
type AlertKind string

const (
	MCAlertLoop         AlertKind = "loop_detected"
	MCAlertConfDrop     AlertKind = "confidence_drop"
	MCAlertBacktrack    AlertKind = "excessive_backtrack"
	MCAlertStall        AlertKind = "stall"
	MCAlertNoProgress   AlertKind = "no_progress"
)

// OnAlert 接收元认知告警并调整思考策略。
// 这个方法应该被注册为 MetaCogMonitor.SetAlertFunc 的回调。
func (mca *MetaCogAdapter) OnAlert(taskID string, kind AlertKind, severity string) {
	mca.mu.Lock()
	defer mca.mu.Unlock()

	state, ok := mca.state[taskID]
	if !ok {
		state = &TaskCogState{TaskID: taskID}
		mca.state[taskID] = state
	}

	state.AlertCount++
	state.AdjustedAt = time.Now()

	switch kind {
	case MCAlertLoop:
		// 检测到循环 → 强制深度思考 + 考虑回溯
		state.LoopDetected = true
		state.ForceDeepThink = true
		slog.Warn("metacog-adapt: loop detected, forcing deep think", "task", taskID)

	case MCAlertConfDrop:
		// 置信度下降 → 升级到专家模型
		state.ConfidenceDrop = true
		state.ForceExpert = true
		slog.Warn("metacog-adapt: confidence drop, forcing expert model", "task", taskID)

	case MCAlertStall, MCAlertNoProgress:
		// 停滞 → 强制深度思考
		state.Stalled = true
		state.ForceDeepThink = true
		slog.Warn("metacog-adapt: stall detected, forcing deep think", "task", taskID)

	case MCAlertBacktrack:
		// 过度回溯 → 升级到专家模型 + 深度思考
		state.ForceDeepThink = true
		state.ForceExpert = true
		slog.Warn("metacog-adapt: excessive backtrack, escalating", "task", taskID)
	}

	// 动态调整 AgenticThinking 配置
	if mca.thinking != nil {
		mca.applyAdjustments(state)
	}
}

// GetOverrides 返回当前任务的思考覆盖设置。
// AgenticThinking.Think() 应该在执行前检查这些覆盖。
func (mca *MetaCogAdapter) GetOverrides(taskID string) *ThinkOverrides {
	mca.mu.Lock()
	defer mca.mu.Unlock()

	state, ok := mca.state[taskID]
	if !ok {
		return nil
	}

	// 超过 5 分钟的调整过期
	if time.Since(state.AdjustedAt) > 5*time.Minute {
		delete(mca.state, taskID)
		return nil
	}

	return &ThinkOverrides{
		ForceLevel:  state.ForceDeepThink,
		ForceExpert: state.ForceExpert,
	}
}

// ThinkOverrides 思考行为覆盖。
type ThinkOverrides struct {
	ForceLevel  bool // 强制 ThinkDeep
	ForceExpert bool // 强制使用 expert 模型
}

// ClearTask 清除指定任务的元认知状态。
func (mca *MetaCogAdapter) ClearTask(taskID string) {
	mca.mu.Lock()
	defer mca.mu.Unlock()
	delete(mca.state, taskID)
}

// applyAdjustments 将元认知调整应用到 AgenticThinking。
func (mca *MetaCogAdapter) applyAdjustments(state *TaskCogState) {
	if state.AlertCount > 5 {
		// 告警过多 → 提高默认思考级别
		mca.thinking.config.DefaultThinkLevel = int(ThinkDeep)
	}
}

// ThinkWithMetaCog 对 AgenticThinking.Think 的包装，融入元认知覆盖。
func (mca *MetaCogAdapter) ThinkWithMetaCog(ctx context.Context, thinking *AgenticThinking, req ThinkRequest) (*ThinkResult, error) {
	// 检查元认知覆盖
	overrides := mca.GetOverrides(req.TaskID)
	if overrides != nil {
		if overrides.ForceLevel {
			// 强制深度思考
			return thinking.deepThink(ctx, req)
		}
	}

	// 正常流程
	return thinking.Think(ctx, req)
}
