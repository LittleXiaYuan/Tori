package task

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"yunque-agent/pkg/risk"
)

// ──────────────────────────────────────────────
// Task — first-class work unit for the Agent Runtime
//
// A Task represents a concrete piece of work the agent accepts, plans,
// executes step-by-step, and delivers artifacts. Unlike a chat turn
// (message→reply→forget), a Task is persistent, trackable, resumable.
// ──────────────────────────────────────────────

// Status is the lifecycle state of a task.
type Status string

const (
	StatusPending     Status = "pending"     // created, not started
	StatusPlanning    Status = "planning"    // LLM generating execution plan
	StatusRunning     Status = "running"     // executing steps
	StatusPaused      Status = "paused"      // paused by user, can resume
	StatusCompleted   Status = "completed"   // all steps done, artifacts ready
	StatusFailed      Status = "failed"      // execution failed
	StatusCancelled   Status = "cancelled"   // cancelled by user
	StatusInterrupted Status = "interrupted" // process crashed while running, recoverable
)

// StepStatus is the state of a single step within a task.
type StepStatus string

const (
	StepPending  StepStatus = "pending"
	StepRunning  StepStatus = "running"
	StepDone     StepStatus = "done"
	StepFailed   StepStatus = "failed"
	StepSkipped  StepStatus = "skipped"
	StepRetrying StepStatus = "retrying"
)

const DefaultMaxRetries = 2

// Task is a persistent, trackable work unit.
type Task struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	Description  string        `json:"description"`
	Status       Status        `json:"status"`
	Steps        []Step        `json:"steps"`
	Artifacts    []Artifact    `json:"artifacts,omitempty"`
	Error        string        `json:"error,omitempty"` // failure reason
	RecoveryHint *RecoveryHint `json:"recovery_hint,omitempty"`
	TenantID     string        `json:"tenant_id"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	StartedAt    *time.Time    `json:"started_at,omitempty"`
	FinishedAt   *time.Time    `json:"finished_at,omitempty"`

	// ── Constraints (execution guardrails) ──
	Constraints *TaskConstraints `json:"constraints,omitempty"`
}

// RecoveryAction describes a concrete repair path for a failed task.
type RecoveryAction struct {
	ID       string         `json:"id"`
	Label    string         `json:"label"`
	Href     string         `json:"href,omitempty"`
	Method   string         `json:"method,omitempty"`
	Endpoint string         `json:"endpoint,omitempty"`
	Body     map[string]any `json:"body,omitempty"`
}

// RecoveryHint gives UIs and external clients structured repair guidance.
type RecoveryHint struct {
	Category         string           `json:"category"`
	Severity         string           `json:"severity"`
	Summary          string           `json:"summary"`
	Detail           string           `json:"detail,omitempty"`
	PrimaryAction    RecoveryAction   `json:"primary_action"`
	SecondaryActions []RecoveryAction `json:"secondary_actions,omitempty"`
	Source           string           `json:"source"`
	GroupKey         string           `json:"group_key,omitempty"`
}

// RiskLevel controls review behavior for a task.
type RiskLevel = risk.Level

const (
	RiskLow    RiskLevel = risk.Low    // async/sidecar review, don't block completion
	RiskMedium RiskLevel = risk.Medium // standard blocking review (default)
	RiskHigh   RiskLevel = risk.High   // blocking review + require human approval
)

// TaskConstraints defines execution guardrails for a task.
type TaskConstraints struct {
	MaxSteps        int            `json:"max_steps,omitempty"`        // 0 = use default (8)
	TimeoutSec      int            `json:"timeout_sec,omitempty"`      // 0 = no global timeout
	MaxCostUSD      float64        `json:"max_cost_usd,omitempty"`     // 0 = no cost limit
	SuccessCriteria string         `json:"success_criteria,omitempty"` // natural-language acceptance condition
	TestCommand     string         `json:"test_command,omitempty"`     // shell command to verify result (exit 0 = pass)
	Priority        string         `json:"priority,omitempty"`         // low / medium / high
	RiskLevel       RiskLevel      `json:"risk_level,omitempty"`       // low/medium/high — controls review mode
	AutoApprove     bool           `json:"auto_approve,omitempty"`     // skip human approval for medium-risk ops
	Tags            []string       `json:"tags,omitempty"`
	Extra           map[string]any `json:"extra,omitempty"` // extensible metadata
}

// Step is one unit of execution within a task.
type Step struct {
	ID         int            `json:"id"`
	Action     string         `json:"action"`     // human-readable description
	SkillName  string         `json:"skill_name"` // skill to execute (empty = LLM-only step)
	Args       map[string]any `json:"args,omitempty"`
	Status     StepStatus     `json:"status"`
	Result     string         `json:"result,omitempty"`
	Error      string         `json:"error,omitempty"`
	Input      string         `json:"input,omitempty"`       // chained from previous step result
	RetryCount int            `json:"retry_count,omitempty"` // how many retries attempted
	MaxRetries int            `json:"max_retries,omitempty"` // max allowed retries (default 2)
	GapType    string         `json:"gap_type,omitempty"`    // capability gap classification if failed
	Group      int            `json:"group,omitempty"`       // parallel group: steps with same group run concurrently
	DependsOn  []int          `json:"depends_on,omitempty"`  // task-local prerequisite step IDs
	Metadata   map[string]any `json:"metadata,omitempty"`    // extensible step provenance and planner state
	StartedAt  *time.Time     `json:"started_at,omitempty"`
	DoneAt     *time.Time     `json:"done_at,omitempty"`
}

// Artifact is a file or output produced by the task.
type Artifact struct {
	Name     string `json:"name"`
	Path     string `json:"path"` // relative path under data/tasks/{id}/
	Type     string `json:"type"` // "file", "text", "image", "code"
	Size     int64  `json:"size,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

// CreateRequest is the input for creating a new task.
type CreateRequest struct {
	Title       string           `json:"title"`
	Description string           `json:"description"`
	TenantID    string           `json:"-"` // injected from auth
	Constraints *TaskConstraints `json:"constraints,omitempty"`
}

// Validate checks the create request.
func (r *CreateRequest) Validate() error {
	if r.Description == "" {
		return fmt.Errorf("description is required")
	}
	return nil
}

// IsTerminal returns true if the task is in a final state.
func (t *Task) IsTerminal() bool {
	return t.Status == StatusCompleted || t.Status == StatusFailed || t.Status == StatusCancelled
}

// IsResumable returns true if the task can be resumed (paused, interrupted, or failed).
func (t *Task) IsResumable() bool {
	return t.Status == StatusPaused || t.Status == StatusInterrupted || t.Status == StatusFailed
}

const providerRecoveryHref = "/settings/providers?tab=providers"

var providerIDHintPattern = regexp.MustCompile(`\bprovider(?:[_\s-]?id)?\s*[:=]\s*["']?([a-z0-9][a-z0-9_.:-]{1,80})`)
var providerTokenPattern = regexp.MustCompile(`[a-z0-9][a-z0-9_.:-]{2,80}`)

var recoveryPatterns = []struct {
	category string
	severity string
	summary  string
	action   RecoveryAction
	pattern  *regexp.Regexp
}{
	{
		category: "provider",
		severity: "danger",
		summary:  "模型供应商认证失败，需要检查 API Key、Base URL 或账号权限",
		action:   RecoveryAction{ID: "open_providers", Label: "检查模型供应商", Href: providerRecoveryHref},
		pattern:  regexp.MustCompile(`(provider|model|llm|openai|qwen|moonshot|api key|apikey|模型|供应商|密钥).*(401|403|unauthori[sz]ed|forbidden|invalid|auth|认证|鉴权|无效)|(401|403|unauthori[sz]ed|forbidden|invalid|auth|认证|鉴权|无效).*(provider|model|llm|openai|qwen|moonshot|api key|apikey|模型|供应商|密钥)`),
	},
	{
		category: "provider",
		severity: "danger",
		summary:  "模型供应商额度或余额不足，需要充值或切换模型",
		action:   RecoveryAction{ID: "open_providers", Label: "检查模型供应商", Href: providerRecoveryHref},
		pattern:  regexp.MustCompile(`(provider|model|llm|openai|qwen|moonshot|模型|供应商).*(402|insufficient balance|quota exceeded|quota|billing|payment|余额|额度|欠费|充值)|(402|insufficient balance|quota exceeded|quota|billing|payment|余额|额度|欠费|充值).*(provider|model|llm|openai|qwen|moonshot|模型|供应商)`),
	},
	{
		category: "provider",
		severity: "warning",
		summary:  "模型供应商请求被限流，需要等待、降并发或切换模型",
		action:   RecoveryAction{ID: "open_providers", Label: "检查模型供应商", Href: providerRecoveryHref},
		pattern:  regexp.MustCompile(`(provider|model|llm|openai|qwen|moonshot|模型|供应商).*(429|rate limit|too many requests|throttle|限流|频率|请求过多)|(429|rate limit|too many requests|throttle|限流|频率|请求过多).*(provider|model|llm|openai|qwen|moonshot|模型|供应商)`),
	},
	{
		category: "connector",
		severity: "danger",
		summary:  "连接器动作超出 Allowlist，需要检查能力边界或改写任务动作",
		action:   RecoveryAction{ID: "open_connectors", Label: "修复连接器", Href: "/settings/connectors"},
		pattern:  regexp.MustCompile(`(connector|browser|github|gmail|slack|notion|jira|linear|action|tool|连接器|浏览器|动作|工具).*(allowlist|allow list|not allowed|unsupported action|denied by allowlist|未授权|不允许|能力边界)|(allowlist|allow list|not allowed|unsupported action|denied by allowlist|不在 allowlist|能力边界)`),
	},
	{
		category: "connector",
		severity: "danger",
		summary:  "连接器认证或凭据失效，需要重新授权",
		action:   RecoveryAction{ID: "open_connectors", Label: "修复连接器", Href: "/settings/connectors"},
		pattern:  regexp.MustCompile(`(connector|browser|github|gmail|slack|notion|jira|linear|oauth|credential|token|cookie|连接器|浏览器|凭证|令牌|授权).*(401|403|unauthori[sz]ed|forbidden|invalid|expired|auth|oauth|credential|token|认证|鉴权|授权|失效|过期|无效)|(401|403|unauthori[sz]ed|forbidden|invalid|expired|auth|oauth|credential|token|认证|鉴权|授权|失效|过期|无效).*(connector|browser|github|gmail|slack|notion|jira|linear|oauth|credential|token|cookie|连接器|浏览器|凭证|令牌|授权)`),
	},
	{
		category: "connector",
		severity: "warning",
		summary:  "连接器请求被限流，需要等待或降低调用频率",
		action:   RecoveryAction{ID: "open_connectors", Label: "修复连接器", Href: "/settings/connectors"},
		pattern:  regexp.MustCompile(`(connector|browser|github|gmail|slack|notion|jira|linear|连接器|浏览器).*(429|rate limit|too many requests|throttle|限流|频率|请求过多)|(429|rate limit|too many requests|throttle|限流|频率|请求过多).*(connector|browser|github|gmail|slack|notion|jira|linear|连接器|浏览器)`),
	},
	{
		category: "tool",
		severity: "warning",
		summary:  "工具不可用，需要启用工具或调整任务能力边界",
		action:   RecoveryAction{ID: "open_tools", Label: "检查工具", Href: "/tools"},
		pattern:  regexp.MustCompile(`unknown skill[:\s]+[a-z0-9_-]*_tool\b`),
	},
	{
		category: "skill",
		severity: "warning",
		summary:  "技能不可用，需要安装、启用或更换执行技能",
		action:   RecoveryAction{ID: "open_skills", Label: "检查技能", Href: "/skills"},
		pattern:  regexp.MustCompile(`unknown skill|missing skill|skill (not found|not installed|unavailable|disabled)|技能.*(不存在|未安装|不可用|已禁用)|未知技能`),
	},
	{
		category: "tool",
		severity: "warning",
		summary:  "工具不可用，需要启用工具或调整任务能力边界",
		action:   RecoveryAction{ID: "open_tools", Label: "检查工具", Href: "/tools"},
		pattern:  regexp.MustCompile(`tool (not found|not installed|unavailable|disabled)|unsupported tool|missing tool|allowed tool surface|工具.*(不存在|未安装|不可用|已禁用|未找到)|未知工具`),
	},
	{
		category: "dependency",
		severity: "warning",
		summary:  "任务依赖未满足，需要先查看执行链",
		action:   RecoveryAction{ID: "open_task_execution", Label: "查看执行链"},
		pattern:  regexp.MustCompile(`task dependency blocked|blocked by dependenc|dependency step|depends_on|depends on step|no ready steps|步骤\s*\d+\s*等待依赖步骤完成|依赖步骤.*尚未完成|前置步骤.*尚未完成`),
	},
	{
		category: "approval",
		severity: "danger",
		summary:  "需要先处理审批或权限，再恢复任务",
		action:   RecoveryAction{ID: "open_approvals", Label: "处理审批", Href: "/approvals"},
		pattern:  regexp.MustCompile(`approval|permission|unauthori[sz]ed|forbidden|denied|审批|权限|授权被拒`),
	},
	{
		category: "provider",
		severity: "warning",
		summary:  "模型供应商不可用，需要检查连接、密钥或额度",
		action:   RecoveryAction{ID: "open_providers", Label: "检查模型供应商", Href: providerRecoveryHref},
		pattern:  regexp.MustCompile(`provider|model|api key|apikey|quota|rate limit|llm|模型|供应商|密钥|余额|限流`),
	},
	{
		category: "connector",
		severity: "warning",
		summary:  "连接器不可用，需要重新连接或修复凭证",
		action:   RecoveryAction{ID: "open_connectors", Label: "修复连接器", Href: "/settings/connectors"},
		pattern:  regexp.MustCompile(`browser|connector|credential|oauth|cookie|pair|浏览器|连接器|凭证|配对|登录`),
	},
	{
		category: "sandbox",
		severity: "warning",
		summary:  "桌面沙箱不可用，需要检查 Computer Use 能力",
		action:   RecoveryAction{ID: "open_computer_use", Label: "检查桌面沙箱", Href: "/packs/computer-use"},
		pattern:  regexp.MustCompile(`sandbox|desktop|computer-use|computer use|tauri|桌面|沙箱`),
	},
}

// BuildRecoveryHint derives a structured recovery hint from task state. It is
// intentionally deterministic so API, UI, SDK, and notification cards agree.
func BuildRecoveryHint(t *Task) *RecoveryHint {
	if t == nil {
		return nil
	}
	if t.RecoveryHint != nil {
		return normalizeRecoveryHintGroupKey(cloneRecoveryHint(t.RecoveryHint))
	}
	return InferRecoveryHint(t, "backend-inferred")
}

// InferRecoveryHint computes a fresh hint even if the task has no stored hint.
// Runners use this at the failure site so the API can preserve a precise source.
func InferRecoveryHint(t *Task, source string) *RecoveryHint {
	if t == nil {
		return nil
	}
	if t.Status != StatusFailed && t.Status != StatusInterrupted {
		return nil
	}
	if strings.TrimSpace(source) == "" {
		source = "backend-inferred"
	}

	detail := firstNonEmpty(t.Error, failedStepError(t), t.Description, "查看任务详情并决定是否重试")
	text := strings.ToLower(strings.Join([]string{t.Error, t.Description, t.Title, failedStepText(t)}, " "))
	for _, item := range recoveryPatterns {
		if item.pattern.MatchString(text) {
			action := item.action
			if item.category == "provider" {
				action = providerRecoveryAction(text, action)
			}
			if item.category == "connector" {
				action = connectorRecoveryAction(text, action)
			}
			if item.category == "dependency" {
				action = taskExecutionAction(t.ID)
			}
			return normalizeRecoveryHintGroupKey(&RecoveryHint{
				Category:      item.category,
				Severity:      item.severity,
				Summary:       item.summary,
				Detail:        detail,
				PrimaryAction: action,
				SecondaryActions: []RecoveryAction{
					restartAction(t.ID),
					taskDetailAction(t.ID),
				},
				Source: source,
			})
		}
	}

	return normalizeRecoveryHintGroupKey(&RecoveryHint{
		Category:      "task",
		Severity:      "warning",
		Summary:       "任务失败，需要查看详情后重试或拆分",
		Detail:        detail,
		PrimaryAction: taskDetailAction(t.ID),
		SecondaryActions: []RecoveryAction{
			restartAction(t.ID),
		},
		Source: source,
	})
}

// WithRecoveryHint returns a clone that includes a computed recovery hint when
// one is available. It does not persist inferred hints back to existing tasks.
func WithRecoveryHint(t *Task) *Task {
	if t == nil {
		return nil
	}
	cp := t.clone()
	cp.RecoveryHint = BuildRecoveryHint(cp)
	return cp
}

// CurrentStep returns the first non-done step, or nil if all done.
func (t *Task) CurrentStep() *Step {
	for i := range t.Steps {
		if t.Steps[i].Status == StepPending || t.Steps[i].Status == StepRunning {
			return &t.Steps[i]
		}
	}
	return nil
}

// Progress returns (completed, total) step counts.
func (t *Task) Progress() (int, int) {
	done := 0
	for _, s := range t.Steps {
		if s.Status == StepDone || s.Status == StepSkipped {
			done++
		}
	}
	return done, len(t.Steps)
}

// clone returns a deep copy of the Task so callers cannot mutate internal store state.
func (t *Task) clone() *Task {
	cp := *t
	if len(t.Steps) > 0 {
		cp.Steps = make([]Step, len(t.Steps))
		for i, s := range t.Steps {
			cp.Steps[i] = s
			if s.Args != nil {
				cp.Steps[i].Args = make(map[string]any, len(s.Args))
				for k, v := range s.Args {
					cp.Steps[i].Args[k] = v
				}
			}
			if s.DependsOn != nil {
				cp.Steps[i].DependsOn = append([]int(nil), s.DependsOn...)
			}
			if s.Metadata != nil {
				cp.Steps[i].Metadata = make(map[string]any, len(s.Metadata))
				for k, v := range s.Metadata {
					cp.Steps[i].Metadata[k] = v
				}
			}
		}
	}
	if len(t.Artifacts) > 0 {
		cp.Artifacts = make([]Artifact, len(t.Artifacts))
		copy(cp.Artifacts, t.Artifacts)
	}
	if t.RecoveryHint != nil {
		cp.RecoveryHint = cloneRecoveryHint(t.RecoveryHint)
	}
	if t.StartedAt != nil {
		sa := *t.StartedAt
		cp.StartedAt = &sa
	}
	if t.FinishedAt != nil {
		fa := *t.FinishedAt
		cp.FinishedAt = &fa
	}
	return &cp
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func failedStepError(t *Task) string {
	if t == nil {
		return ""
	}
	for _, step := range t.Steps {
		if step.Status == StepFailed && step.Error != "" {
			return step.Error
		}
	}
	return ""
}

func failedStepText(t *Task) string {
	if t == nil {
		return ""
	}
	var parts []string
	for _, step := range t.Steps {
		if step.Status == StepFailed || step.Error != "" || step.GapType != "" {
			parts = append(parts, step.Action, step.SkillName, step.Error, step.GapType)
		}
	}
	return strings.Join(parts, " ")
}

func connectorRecoveryAction(text string, fallback RecoveryAction) RecoveryAction {
	if strings.Contains(text, "browser") || strings.Contains(text, "extension") || strings.Contains(text, "pair") || strings.Contains(text, "浏览器") || strings.Contains(text, "扩展") || strings.Contains(text, "配对") {
		return RecoveryAction{ID: "open_browser_pack", Label: "打开浏览器包", Href: "/packs/browser"}
	}

	for _, connectorID := range []string{"github", "gmail", "google_calendar", "google calendar", "slack", "notion", "linear", "jira"} {
		if strings.Contains(text, connectorID) {
			focusedID := strings.ReplaceAll(connectorID, " ", "_")
			action := fallback
			action.Href = "/settings/connectors?focus=" + url.QueryEscape(focusedID)
			return action
		}
	}

	return fallback
}

func providerRecoveryAction(text string, fallback RecoveryAction) RecoveryAction {
	providerID := providerIDFromRecoveryText(text)
	if providerID == "" {
		return fallback
	}
	action := fallback
	action.Href = "/settings/providers?focus=" + url.QueryEscape(providerID)
	return action
}

func providerIDFromRecoveryText(text string) string {
	if match := providerIDHintPattern.FindStringSubmatch(text); len(match) > 1 && isSpecificProviderID(match[1]) {
		return strings.ToLower(match[1])
	}
	for _, token := range providerTokenPattern.FindAllString(text, -1) {
		if isSpecificProviderID(token) {
			return strings.ToLower(token)
		}
	}
	return ""
}

func isSpecificProviderID(token string) bool {
	value := strings.ToLower(strings.TrimSpace(token))
	if value == "" || value == "provider" || value == "model" || value == "llm" {
		return false
	}
	if !strings.ContainsAny(value, "-_:") {
		return false
	}
	for _, family := range []string{"openai", "qwen", "moonshot", "kimi", "deepseek", "minimax", "gemini", "google", "anthropic", "claude", "ollama", "tori", "local"} {
		if strings.Contains(value, family) {
			return true
		}
	}
	return false
}

func restartAction(taskID string) RecoveryAction {
	return RecoveryAction{
		ID:       "restart_task",
		Label:    "重启任务",
		Method:   "POST",
		Endpoint: "/v1/tasks/restart",
		Body:     map[string]any{"id": taskID},
	}
}

func taskDetailAction(taskID string) RecoveryAction {
	return RecoveryAction{
		ID:    "open_task_detail",
		Label: "查看任务详情",
		Href:  "/task-detail?id=" + taskID,
	}
}

func taskExecutionAction(taskID string) RecoveryAction {
	return RecoveryAction{
		ID:    "open_task_execution",
		Label: "查看执行链",
		Href:  "/task-detail?id=" + url.QueryEscape(taskID) + "&tab=execution",
	}
}

func cloneRecoveryHint(h *RecoveryHint) *RecoveryHint {
	if h == nil {
		return nil
	}
	cp := *h
	cp.PrimaryAction = cloneRecoveryAction(h.PrimaryAction)
	if len(h.SecondaryActions) > 0 {
		cp.SecondaryActions = make([]RecoveryAction, len(h.SecondaryActions))
		for i, action := range h.SecondaryActions {
			cp.SecondaryActions[i] = cloneRecoveryAction(action)
		}
	}
	return &cp
}

func cloneRecoveryAction(action RecoveryAction) RecoveryAction {
	cp := action
	if action.Body != nil {
		cp.Body = make(map[string]any, len(action.Body))
		for k, v := range action.Body {
			cp.Body[k] = v
		}
	}
	return cp
}

func normalizeRecoveryHintGroupKey(h *RecoveryHint) *RecoveryHint {
	if h == nil {
		return nil
	}
	if strings.TrimSpace(h.GroupKey) == "" {
		h.GroupKey = recoveryGroupKey(h.Category, h.PrimaryAction)
	}
	return h
}

func recoveryGroupKey(category string, action RecoveryAction) string {
	normalizedCategory := strings.ToLower(strings.TrimSpace(category))
	if normalizedCategory == "" {
		normalizedCategory = "task"
	}
	target := firstNonEmpty(action.Href, action.Endpoint, action.ID, action.Label)
	target = strings.TrimSpace(target)
	if target == "" {
		return normalizedCategory
	}
	return normalizedCategory + "|" + target
}
