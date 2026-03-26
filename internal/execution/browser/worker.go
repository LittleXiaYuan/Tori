package browser

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/opp"
)

// WorkerState tracks the sub-agent's lifecycle.
type WorkerState string

const (
	WorkerIdle    WorkerState = "idle"
	WorkerBusy    WorkerState = "busy"
	WorkerPaused  WorkerState = "paused" // waiting for human decision (OPP PROBLEM)
	WorkerStopped WorkerState = "stopped"
)

// WorkerConfig configures the browser sub-agent.
type WorkerConfig struct {
	Engine      *Engine                  // browser engine instance
	LLM         *llm.Client              // dedicated LLM for browser reasoning (ideally vision-capable)
	MaxSteps    int                      // max autonomous steps (default 20)
	DataDir     string                   // screenshot save directory
	OnEvent     func(observe.AgentEvent) // SSE callback
	Notifier    *Notifier                // optional: multi-channel event/problem notifier
	Recognizer  *Recognizer              // optional: 4-tier OCR fallback (DOM→Tesseract→Vision LLM→Human)
}

// Worker is a browser sub-agent that operates independently.
// It receives natural-language tasks, plans browser actions via LLM,
// executes them, and communicates status via OPP messages.
type Worker struct {
	mu       sync.Mutex
	cfg      WorkerConfig
	state    WorkerState
	taskID   string
	history  []llm.Message // sub-agent's own conversation history
	cancel   context.CancelFunc

	// OPP state
	oppState opp.TaskState

	// currentProblemID holds the active OPP problem ID while waiting for human decision.
	// Set by requestHumanDecision, cleared on resolution. Used by SubmitDecision.
	currentProblemID string

	// decideCh is the fallback channel when no Notifier is configured.
	decideCh chan string
}

func NewWorker(cfg WorkerConfig) *Worker {
	if cfg.MaxSteps <= 0 {
		cfg.MaxSteps = 20
	}
	return &Worker{
		cfg:      cfg,
		state:    WorkerIdle,
		oppState: opp.StatePending,
		decideCh: make(chan string, 1),
	}
}

// RunTask executes a browser task described in natural language.
// It returns the final result text and any error.
func (w *Worker) RunTask(ctx context.Context, taskID, description string) (string, error) {
	w.mu.Lock()
	if w.state == WorkerBusy {
		w.mu.Unlock()
		return "", fmt.Errorf("browser worker: already busy")
	}
	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.state = WorkerBusy
	w.taskID = taskID
	w.oppState = opp.StateRunning
	w.history = []llm.Message{
		{Role: "system", Content: browserSystemPrompt},
		{Role: "user", Content: description},
	}
	w.mu.Unlock()

	defer func() {
		// Clear screenshot hook so it doesn't outlive the task
		if w.cfg.Notifier != nil {
			w.cfg.Engine.SetScreenshotHook(nil)
		}
		w.mu.Lock()
		w.state = WorkerIdle
		w.cancel = nil
		w.mu.Unlock()
		cancel()
	}()

	// Wire screenshot hook → Notifier so every screenshot is broadcast in real time
	if w.cfg.Notifier != nil {
		n := w.cfg.Notifier
		w.cfg.Engine.SetScreenshotHook(func(data []byte) {
			b64 := base64.StdEncoding.EncodeToString(data)
			n.BroadcastScreenshot(context.Background(), b64)
		})
	}

	w.emitEvent(observe.EventToolStart, "🌐 浏览器子Agent开始执行: "+truncateStr(description, 100))

	// OPP ACCEPT
	w.emitOPP(opp.MsgAccept, map[string]any{"task_id": taskID})

	var finalResult string

	for step := 0; step < w.cfg.MaxSteps; step++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		// Ask LLM what to do next
		reply, toolCalls, err := w.cfg.LLM.ChatWithTools(ctx, w.history, w.toolDefs(), 0.3)
		if err != nil {
			return "", fmt.Errorf("browser worker step %d: %w", step, err)
		}

		w.history = append(w.history, llm.Message{Role: "assistant", Content: reply})

		// No tool calls = LLM is done, reply is the final answer
		if len(toolCalls) == 0 {
			finalResult = reply
			break
		}

		// Execute each tool call
		for _, tc := range toolCalls {
			var args map[string]any
			json.Unmarshal([]byte(tc.Function.Arguments), &args)

			toolName := tc.Function.Name
			slog.Info("browser worker: tool call", "tool", toolName, "step", step)

			// OPP PROGRESS
			w.emitOPP(opp.MsgProgress, map[string]any{
				"step":    step,
				"action":  toolName,
				"task_id": taskID,
			})
			w.emitEvent(observe.EventToolStart, fmt.Sprintf("🔧 [step %d] %s", step, toolName))

			// Special: if LLM decides it needs human help
			if toolName == "request_human_help" {
				reason, _ := args["reason"].(string)
				options, _ := args["options"].([]any)
				decision, err := w.requestHumanDecision(ctx, reason, options)
				if err != nil {
					w.history = append(w.history, llm.ToolResultMessage(tc.ID,
						fmt.Sprintf("人工协助超时或取消: %v", err)))
					continue
				}
				w.history = append(w.history, llm.ToolResultMessage(tc.ID,
					fmt.Sprintf("用户选择: %s", decision)))
				continue
			}

			// Regular browser tool dispatch — use OCR-aware dispatcher if Recognizer is configured
			var dispatcher *Dispatcher
			if w.cfg.Recognizer != nil {
				dispatcher = NewDispatcherWithOCR(w.cfg.Engine, w.cfg.DataDir, w.cfg.Recognizer)
			} else {
				dispatcher = NewDispatcher(w.cfg.Engine, w.cfg.DataDir)
			}
			result := dispatcher.Dispatch(toolName, args)
			resultJSON, _ := json.Marshal(result)

			w.emitEvent(observe.EventToolResult, fmt.Sprintf("✅ [%s] 完成", toolName))
			w.history = append(w.history, llm.ToolResultMessage(tc.ID, string(resultJSON)))
		}
	}

	// OPP RESULT
	w.oppState = opp.StateCompleted
	w.emitOPP(opp.MsgResult, map[string]any{
		"task_id": taskID,
		"status":  "success",
		"output":  truncateStr(finalResult, 2000),
	})
	w.emitEvent(observe.EventToolResult, "🏁 浏览器任务完成")

	return finalResult, nil
}

// requestHumanDecision sends an OPP PROBLEM and waits for a human DECIDE response.
// When a Notifier is configured, it routes through Notifier.RaiseProblem so all
// connected channels (WebUI SSE, IM, etc.) receive the problem with a live screenshot.
// Falls back to the internal decideCh when no Notifier is available.
func (w *Worker) requestHumanDecision(ctx context.Context, reason string, options []any) (string, error) {
	w.mu.Lock()
	w.state = WorkerPaused
	w.oppState = opp.StateWaitingInput
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.state = WorkerBusy
		w.oppState = opp.StateRunning
		w.currentProblemID = ""
		w.mu.Unlock()
	}()

	w.emitEvent(observe.EventToolResult, "⚠️ 需要人工决策: "+reason)

	// Build problem options (shared between both paths)
	probOptions := make([]ProblemOption, 0, len(options))
	for _, opt := range options {
		if m, ok := opt.(map[string]any); ok {
			probOptions = append(probOptions, ProblemOption{
				Key:   fmt.Sprintf("%v", m["value"]),
				Label: fmt.Sprintf("%v", m["label"]),
			})
		}
	}

	// ── Path A: Notifier available — attach screenshot and route through Notifier ──
	if w.cfg.Notifier != nil {
		problemID := fmt.Sprintf("prob-%d", time.Now().UnixMilli())
		w.mu.Lock()
		w.currentProblemID = problemID
		w.mu.Unlock()

		problem := ProblemData{
			ID:          problemID,
			Description: reason,
			URL:         w.cfg.Engine.CurrentURL(),
			Options:     probOptions,
		}
		// Attach a live screenshot so the human can see the current page state
		if imgData, err := w.cfg.Engine.ScreenshotBytes(); err == nil {
			problem.Screenshot = base64.StdEncoding.EncodeToString(imgData)
		}

		// 5-minute timeout via context
		decideCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		decision, err := w.cfg.Notifier.RaiseProblem(decideCtx, problem)
		if err != nil {
			return "", fmt.Errorf("decision timeout or cancelled: %w", err)
		}
		w.emitEvent(observe.EventToolResult, "✅ 人工决策收到: "+decision)
		return decision, nil
	}

	// ── Path B: Fallback — log OPP event and wait on internal channel ──
	w.emitOPP(opp.MsgProblem, map[string]any{
		"task_id":     w.taskID,
		"severity":    "warning",
		"description": reason,
	})
	select {
	case decision := <-w.decideCh:
		w.emitOPP(opp.MsgDecide, map[string]any{
			"task_id": w.taskID,
			"choice":  decision,
		})
		return decision, nil
	case <-time.After(5 * time.Minute):
		return "", fmt.Errorf("decision timeout")
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// SubmitDecision sends a human decision to unblock a paused worker.
// When a Notifier is configured, it delegates to Notifier.ResolveProblem using the
// current problem ID so the decision flows through all notification channels.
func (w *Worker) SubmitDecision(choice string) {
	w.mu.Lock()
	probID := w.currentProblemID
	w.mu.Unlock()

	if probID != "" && w.cfg.Notifier != nil {
		w.cfg.Notifier.ResolveProblem(probID, choice)
		return
	}
	// Fallback: send directly to internal channel
	select {
	case w.decideCh <- choice:
	default:
	}
}

// Stop cancels the current task.
func (w *Worker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		w.cancel()
	}
	w.state = WorkerStopped
	w.oppState = opp.StateCancelled
}

// State returns the current worker state.
func (w *Worker) State() WorkerState {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.state
}

func (w *Worker) emitEvent(typ string, msg string) {
	if w.cfg.OnEvent == nil {
		return
	}
	evt := observe.NewEvent("", observe.DomainPlanner, typ, msg)
	evt.Meta.Skill = "browser"
	w.cfg.OnEvent(evt)
}

func (w *Worker) emitOPP(msgType opp.MessageType, data map[string]any) {
	slog.Info("browser worker: opp", "type", msgType, "task", w.taskID)
	if w.cfg.Notifier == nil {
		return
	}
	// Route OPP events to Notifier channels so WebUI SSE and IM stay in sync.
	switch msgType {
	case opp.MsgProgress:
		action, _ := data["action"].(string)
		step := fmt.Sprintf("%v", data["step"])
		w.cfg.Notifier.BroadcastAction(context.Background(), string(msgType),
			fmt.Sprintf("[step %s] %s", step, action))
	case opp.MsgResult:
		w.cfg.Notifier.Broadcast(context.Background(), BrowserEvent{
			Type:   EventResult,
			TaskID: w.taskID,
			Data:   data,
		})
	case opp.MsgAccept:
		w.cfg.Notifier.BroadcastAction(context.Background(), string(msgType),
			fmt.Sprintf("task %s accepted", w.taskID))
	}
}

// toolDefs returns the tools available to the browser sub-agent LLM.
func (w *Worker) toolDefs() []llm.FunctionDef {
	defs := make([]llm.FunctionDef, 0)

	// All standard browser tools
	for _, td := range ToolDefinitions() {
		fn, _ := td["function"].(map[string]any)
		if fn == nil {
			continue
		}
		name, _ := fn["name"].(string)
		desc, _ := fn["description"].(string)
		params, _ := fn["parameters"].(map[string]any)
		defs = append(defs, llm.FunctionDef{
			Name:        name,
			Description: desc,
			Parameters:  params,
		})
	}

	// Special: request human help (triggers OPP PROBLEM/DECIDE)
	defs = append(defs, llm.FunctionDef{
		Name:        "request_human_help",
		Description: "当遇到验证码、登录弹窗、或其他需要人工介入的情况时调用。描述问题并提供选项。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"reason": map[string]any{
					"type":        "string",
					"description": "需要人工帮助的原因",
				},
				"options": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"value": map[string]any{"type": "string"},
							"label": map[string]any{"type": "string"},
						},
					},
					"description": "提供给用户的选项",
				},
			},
			"required": []string{"reason"},
		},
	})

	return defs
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

var _ = strings.HasPrefix // keep import

const browserSystemPrompt = `你是一个浏览器操作助手。你通过调用浏览器工具来完成用户的任务。

你的工作流程：
1. 用 browser_navigate 打开目标页面
2. 用 browser_read 了解页面结构和内容
3. 根据任务需要，用 browser_click / browser_type 进行交互
4. 用 browser_screenshot 截图记录操作结果
5. 如果遇到验证码、登录弹窗等障碍，调用 request_human_help 寻求人工帮助

注意事项：
- 每一步操作后检查结果，确认操作成功
- 遇到页面加载慢的情况，先截图确认状态
- 不要在一步中做太多操作，保持可追溯性
- 最终返回任务执行结果的文字总结`
