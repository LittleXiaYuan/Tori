package planner

import (
	"strings"

	"yunque-agent/internal/observe"
)

// CognitiveLoadLevel describes how much orchestration pressure a request puts
// on the planner. It is deliberately heuristic and cheap: the goal is not to
// replace model reasoning, but to choose a safer execution engine before the
// first expensive model call.
type CognitiveLoadLevel string

const (
	CognitiveLoadLow      CognitiveLoadLevel = "low"
	CognitiveLoadMedium   CognitiveLoadLevel = "medium"
	CognitiveLoadHigh     CognitiveLoadLevel = "high"
	CognitiveLoadCritical CognitiveLoadLevel = "critical"
)

// CognitiveLoadAssessment is the route-time summary for a request.
type CognitiveLoadAssessment struct {
	Level     CognitiveLoadLevel `json:"level"`
	Score     int                `json:"score"`
	GoalChars int                `json:"goal_chars"`
	Signals   []string           `json:"signals,omitempty"`
	Domains   []string           `json:"domains,omitempty"`
}

func (a CognitiveLoadAssessment) NeedsLongHorizon() bool {
	return a.Level == CognitiveLoadHigh || a.Level == CognitiveLoadCritical
}

func assessCognitiveLoad(req PlanRequest) CognitiveLoadAssessment {
	goal := strings.TrimSpace(extractGoal(req))
	lower := strings.ToLower(goal)
	chars := len([]rune(goal))
	a := CognitiveLoadAssessment{Level: CognitiveLoadLow, GoalChars: chars}

	add := func(score int, signal string) {
		a.Score += score
		for _, s := range a.Signals {
			if s == signal {
				return
			}
		}
		a.Signals = append(a.Signals, signal)
	}

	switch {
	case chars > 600:
		add(4, "very_long_query")
	case chars > 260:
		add(3, "long_query")
	case chars > 160:
		add(2, "medium_long_query")
	}

	actionKeywords := []string{
		"继续", "推进", "完善", "拆解", "实现", "修复", "测试", "验证", "部署", "重构", "扫描", "清理",
		"路线图", "蓝图", "一直执行", "持续", "全部", "都",
		"continue", "implement", "fix", "test", "verify", "deploy", "refactor", "roadmap", "all",
	}
	actionHits := countKeywordHits(lower, actionKeywords)
	switch {
	case actionHits >= 7:
		add(4, "many_requested_actions")
	case actionHits >= 4:
		add(3, "multi_action_request")
	case actionHits >= 2:
		add(1, "some_actions")
	}

	domainKeywords := map[string][]string{
		"planner":  {"planner", "规划器", "计划器", "reason", "react", "long horizon"},
		"docs":     {"doc", "docs", "文档", "技术蓝图", "路线图", "blueprint"},
		"code":     {"代码", "code", "repo", "仓库", "实现"},
		"tests":    {"测试", "test", "typecheck", "验证"},
		"frontend": {"前端", "frontend", "ui", "页面", "聊天框"},
		"backend":  {"后端", "backend", "gateway", "api", "provider"},
		"subagent": {"子代理", "subagent", "handoff", "委派"},
		"files":    {"word", "ppt", "excel", "xlsx", "docx", "pptx", "附件", "文件"},
	}
	for domain, kws := range domainKeywords {
		if countKeywordHits(lower, kws) > 0 {
			a.Domains = append(a.Domains, domain)
		}
	}
	switch {
	case len(a.Domains) >= 5:
		add(4, "many_domains")
	case len(a.Domains) >= 3:
		add(2, "multi_domain")
	}

	failureKeywords := []string{"问题", "失败", "崩溃", "超时", "中断", "fallback", "failed", "timeout", "crash", "error"}
	if countKeywordHits(lower, failureKeywords) > 0 {
		add(1, "failure_context")
	}

	if len(req.AllowedSkills) > 0 {
		add(1, "restricted_tool_surface")
	}

	switch {
	case a.Score >= 8:
		a.Level = CognitiveLoadCritical
	case a.Score >= 5:
		a.Level = CognitiveLoadHigh
	case a.Score >= 3:
		a.Level = CognitiveLoadMedium
	default:
		a.Level = CognitiveLoadLow
	}
	return a
}

func countKeywordHits(text string, keywords []string) int {
	count := 0
	for _, kw := range keywords {
		if kw == "" {
			continue
		}
		if strings.Contains(text, strings.ToLower(kw)) {
			count++
		}
	}
	return count
}

func (p *Planner) shouldUseLongHorizon(req PlanRequest) bool {
	return p.executionMode(req).Mode == PlanExecutionLongHorizon
}

func (p *Planner) emitCognitiveLoadEvent(req PlanRequest, a CognitiveLoadAssessment) {
	if req.StepCallback == nil {
		return
	}
	evt := observe.NewEvent(req.TraceID, observe.DomainPlanner, observe.EventPlan, "认知负荷较高，切换为长程规划执行")
	evt.Meta.TenantID = req.TenantID
	evt.Meta.SessionID = req.SessionID
	evt.Meta.TaskID = req.TaskID
	evt.Detail = a
	req.StepCallback(evt)
}
