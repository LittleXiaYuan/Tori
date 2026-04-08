package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/task"
	"yunque-agent/internal/observe"
)

type slashCommandResponse struct {
	Result *planner.PlanResult
	Raw    map[string]any
}

func (g *Gateway) tryHandleSlashCommand(ctx context.Context, req planner.PlanRequest) (*slashCommandResponse, bool, error) {
	if len(req.Messages) == 0 {
		return nil, false, nil
	}
	last := req.Messages[len(req.Messages)-1]
	if last.Role != "user" {
		return nil, false, nil
	}

	cmd, args := parseGatewaySlashCommand(last.Content)
	if cmd == "" {
		return nil, false, nil
	}

	switch cmd {
	case "/navigate":
		return g.handleSlashBrowserAction(ctx, req, "browser_navigate", map[string]any{
			"url": normalizeNavigateTarget(args),
		})
	case "/screenshot":
		return g.handleSlashBrowserAction(ctx, req, "browser_screenshot", map[string]any{})
	case "/content":
		return g.handleSlashBrowserAction(ctx, req, "browser_get_content", map[string]any{})
	case "/mark":
		return g.handleSlashBrowserAction(ctx, req, "browser_mark_elements", map[string]any{})
	case "/unmark":
		return g.handleSlashBrowserAction(ctx, req, "browser_unmark_elements", map[string]any{})
	case "/scroll":
		direction := strings.TrimSpace(args)
		if direction == "" {
			direction = "down"
		}
		return g.handleSlashBrowserAction(ctx, req, "browser_scroll", map[string]any{"direction": direction})
	case "/click":
		actionArgs := map[string]any{}
		arg := strings.TrimSpace(args)
		if arg == "" {
			return slashError("请在 /click 后提供元素编号或选择器"), true, nil
		}
		if idx, err := strconv.Atoi(arg); err == nil {
			actionArgs["index"] = idx
		} else {
			actionArgs["selector"] = arg
		}
		return g.handleSlashBrowserAction(ctx, req, "browser_click", actionArgs)
	case "/type":
		text := strings.TrimSpace(args)
		if text == "" {
			return slashError("请在 /type 后提供要输入的文本"), true, nil
		}
		return g.handleSlashBrowserAction(ctx, req, "browser_input", map[string]any{"text": text})
	case "/github_repos":
		return g.handleSlashConnectorAction(ctx, req, "github", "list_repos", map[string]any{})
	case "/github_issues":
		owner, repo, ok := parseOwnerRepo(args)
		if !ok {
			return slashError("请使用 /github_issues owner/repo"), true, nil
		}
		return g.handleSlashConnectorAction(ctx, req, "github", "list_issues", map[string]any{"owner": owner, "repo": repo, "state": "open"})
	case "/gmail_inbox":
		params := map[string]any{"max_results": 10}
		if q := strings.TrimSpace(args); q != "" {
			params["query"] = q
		}
		return g.handleSlashConnectorAction(ctx, req, "gmail", "list_messages", params)
	case "/calendar_events":
		now := time.Now()
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		end := start.Add(24 * time.Hour)
		return g.handleSlashConnectorAction(ctx, req, "google_calendar", "list_events", map[string]any{
			"time_min":    start.Format(time.RFC3339),
			"time_max":    end.Format(time.RFC3339),
			"max_results": 10,
		})
	case "/notion_search":
		q := strings.TrimSpace(args)
		if q == "" {
			return slashError("请使用 /notion_search 关键词"), true, nil
		}
		return g.handleSlashConnectorAction(ctx, req, "notion", "search", map[string]any{"query": q})
	case "/slack_send":
		channel, text, ok := parseChannelAndText(args)
		if !ok {
			return slashError("请使用 /slack_send #channel 消息内容"), true, nil
		}
		return g.handleSlashConnectorAction(ctx, req, "slack", "send_message", map[string]any{"channel": channel, "text": text})
	case "/new_task":
		title := strings.TrimSpace(args)
		if title == "" {
			return slashError("请使用 /new_task 任务标题"), true, nil
		}
		return g.handleSlashNewTask(ctx, req, title)
	default:
		return nil, false, nil
	}
}

func (g *Gateway) handleSlashBrowserAction(ctx context.Context, req planner.PlanRequest, skill string, args map[string]any) (*slashCommandResponse, bool, error) {
	hub := g.browserHub
	if hub == nil || !hub.Connected() {
		return slashError("浏览器扩展未连接。请先安装并连接 Yunque Browser Connector。"), true, nil
	}

	action, err := browserActionForSlash(skill, args)
	if err != nil {
		return slashError(err.Error()), true, nil
	}

	emitSlashToolEvent(req, observe.EventToolStart, skill, fmt.Sprintf("执行 %s", skill), args, "", req.StepCallback)
	result, err := hub.SendAction(ctx, action)
	if err != nil {
		emitSlashToolEvent(req, observe.EventToolResult, skill, fmt.Sprintf("%s 执行失败", skill), args, err.Error(), req.StepCallback)
		return nil, true, err
	}

	payload, _ := json.Marshal(result)
	payloadText := string(payload)
	emitSlashToolEvent(req, observe.EventToolResult, skill, fmt.Sprintf("%s 执行完成", skill), args, payloadText, req.StepCallback)

	reply := summarizeBrowserSlashReply(skill, args, result)
	browserSummary := summarizeBrowserSlashArtifact(skill, args, result)
	return &slashCommandResponse{
		Result: &planner.PlanResult{
			Reply:      reply,
			SkillsUsed: []string{skill},
			Steps:      1,
		},
		Raw: map[string]any{
			"reply":           reply,
			"skills_used":     []string{skill},
			"steps":           1,
			"browser":         result,
			"browser_summary": browserSummary,
		},
	}, true, nil
}

func (g *Gateway) handleSlashConnectorAction(ctx context.Context, req planner.PlanRequest, connectorID, actionID string, params map[string]any) (*slashCommandResponse, bool, error) {
	if g.connectorReg == nil {
		return slashError("连接器系统尚未初始化。"), true, nil
	}
	def := g.connectorReg.GetDef(connectorID)
	if def == nil {
		return slashError("未找到对应连接器。"), true, nil
	}
	inst := g.connectorReg.GetInstance(connectorID)
	if inst.Status != "connected" {
		return slashError(fmt.Sprintf("%s 尚未连接，请先到设置页完成连接。", def.Name)), true, nil
	}

	skill := fmt.Sprintf("connector_%s_%s", connectorID, actionID)
	emitSlashToolEvent(req, observe.EventToolStart, skill, fmt.Sprintf("执行 %s", skill), params, "", req.StepCallback)
	result, err := g.connectorReg.Execute(ctx, connectorID, actionID, params)
	if err != nil {
		emitSlashToolEvent(req, observe.EventToolResult, skill, fmt.Sprintf("%s 执行失败", skill), params, err.Error(), req.StepCallback)
		return nil, true, err
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	text := string(data)
	if len(text) > 4000 {
		text = text[:4000] + "\n... (truncated)"
	}
	emitSlashToolEvent(req, observe.EventToolResult, skill, fmt.Sprintf("%s 执行完成", skill), params, text, req.StepCallback)

	reply := fmt.Sprintf("已执行 %s / %s。\n\n```json\n%s\n```", def.Name, actionID, text)
	return &slashCommandResponse{
		Result: &planner.PlanResult{
			Reply:      reply,
			SkillsUsed: []string{skill},
			Steps:      1,
		},
		Raw: map[string]any{
			"reply":       reply,
			"skills_used": []string{skill},
			"steps":       1,
		},
	}, true, nil
}

func (g *Gateway) handleSlashNewTask(_ context.Context, _ planner.PlanRequest, title string) (*slashCommandResponse, bool, error) {
	if g.taskStore == nil {
		return slashError("任务系统尚未启用。"), true, nil
	}
	created, err := g.taskStore.Create(task.CreateRequest{
		Title:       title,
		Description: title,
		TenantID:    "default",
	})
	if err != nil {
		return nil, true, err
	}
	reply := fmt.Sprintf("已创建任务：%s\n任务 ID：%s", created.Title, created.ID)
	return &slashCommandResponse{
		Result: &planner.PlanResult{
			Reply:      reply,
			SkillsUsed: []string{"task_create"},
			Steps:      1,
		},
		Raw: map[string]any{
			"reply":       reply,
			"skills_used": []string{"task_create"},
			"steps":       1,
		},
	}, true, nil
}

func slashError(msg string) *slashCommandResponse {
	return &slashCommandResponse{
		Result: &planner.PlanResult{Reply: msg, Steps: 1},
		Raw:    map[string]any{"reply": msg, "skills_used": []string{}, "steps": 1},
	}
}

func parseGatewaySlashCommand(text string) (string, string) {
	text = strings.TrimSpace(text)
	if text == "" || text[0] != '/' {
		return "", ""
	}
	parts := strings.SplitN(text, " ", 2)
	cmd := strings.ToLower(parts[0])
	if at := strings.IndexByte(cmd, '@'); at > 0 {
		cmd = cmd[:at]
	}
	args := ""
	if len(parts) == 2 {
		args = strings.TrimSpace(parts[1])
	}
	return cmd, args
}

func normalizeNavigateTarget(arg string) string {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "https://www.google.com"
	}
	lower := strings.ToLower(arg)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return arg
	}
	if strings.Contains(arg, ".") && !strings.Contains(arg, " ") && !strings.ContainsAny(arg, "中文，。！？；：") {
		return "https://" + arg
	}
	return "https://www.bing.com/search?q=" + url.QueryEscape(arg)
}

func parseOwnerRepo(arg string) (string, string, bool) {
	parts := strings.Split(strings.TrimSpace(arg), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func parseChannelAndText(arg string) (string, string, bool) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", "", false
	}
	parts := strings.SplitN(arg, " ", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", false
	}
	return parts[0], strings.TrimSpace(parts[1]), true
}

func browserActionForSlash(skill string, args map[string]any) (BrowserAction, error) {
	switch skill {
	case "browser_navigate":
		target, _ := args["url"].(string)
		if target == "" {
			return BrowserAction{}, fmt.Errorf("缺少导航目标")
		}
		return BrowserAction{Type: "browser_navigate", URL: target}, nil
	case "browser_screenshot":
		return BrowserAction{Type: "browser_screenshot"}, nil
	case "browser_get_content":
		return BrowserAction{Type: "browser_get_content"}, nil
	case "browser_mark_elements":
		return BrowserAction{Type: "browser_mark_elements"}, nil
	case "browser_unmark_elements":
		return BrowserAction{Type: "browser_unmark_elements"}, nil
	case "browser_scroll":
		direction, _ := args["direction"].(string)
		if direction == "" {
			direction = "down"
		}
		return BrowserAction{Type: "browser_scroll", Direction: direction}, nil
	case "browser_click":
		if idx, ok := args["index"].(int); ok {
			return BrowserAction{Type: "browser_click", Target: &ActionTarget{Strategy: "byIndex", Index: idx}}, nil
		}
		if sel, ok := args["selector"].(string); ok && sel != "" {
			return BrowserAction{Type: "browser_click", Target: &ActionTarget{Strategy: "bySelector", Selector: sel}}, nil
		}
		return BrowserAction{}, fmt.Errorf("click 需要元素编号或选择器")
	case "browser_input":
		text, _ := args["text"].(string)
		if text == "" {
			return BrowserAction{}, fmt.Errorf("缺少输入内容")
		}
		return BrowserAction{Type: "browser_input", Text: text}, nil
	default:
		return BrowserAction{}, fmt.Errorf("不支持的浏览器命令：%s", skill)
	}
}

func summarizeBrowserSlashReply(skill string, args map[string]any, result BrowserResult) string {
	if !result.OK {
		if result.Error != "" {
			return "??????????" + result.Error
		}
		return "??????????"
	}

	switch skill {
	case "browser_navigate":
		target, _ := args["url"].(string)
		parts := []string{fmt.Sprintf("?????????%s", firstNonEmpty(result.URL, target))}
		if result.Title != "" {
			parts = append(parts, fmt.Sprintf("?????%s", result.Title))
		}
		if result.Screenshot != "" {
			parts = append(parts, "???????????????????")
		}
		return strings.Join(parts, "\n")
	case "browser_screenshot":
		return "????????????????????"
	case "browser_get_content":
		if result.Content == "" {
			return "????????????????????????"
		}
		text := result.Content
		if len(text) > 1200 {
			text = text[:1200] + "\n... (truncated)"
		}
		prefix := "?????????"
		if result.Title != "" {
			prefix = fmt.Sprintf("????????%s??", result.Title)
		}
		return prefix + "\n\n" + text
	case "browser_mark_elements":
		if result.Total > 0 {
			return fmt.Sprintf("??????? %d ??????????????? /click ?? ?????", result.Total)
		}
		return "???????????????????? /click ?? ?????"
	case "browser_unmark_elements":
		return "????????????"
	case "browser_scroll":
		return "????????????????????"
	case "browser_click":
		if result.Title != "" || result.URL != "" {
			return fmt.Sprintf("?????????????%s", firstNonEmpty(result.Title, result.URL, "???"))
		}
		return "????????"
	case "browser_input":
		return "??????????"
	default:
		return "?????????"
	}
}

func isBrowserSkill(skill string) bool {
	return strings.HasPrefix(skill, "browser_")
}

func parseBrowserResultText(raw string) BrowserResult {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return BrowserResult{}
	}
	var result BrowserResult
	if err := json.Unmarshal([]byte(raw), &result); err == nil {
		return result
	}
	return BrowserResult{OK: true, Content: raw}
}

func summarizeBrowserPlanArtifact(plan []planner.PlanStep) map[string]any {
	for i := len(plan) - 1; i >= 0; i-- {
		step := plan[i]
		if !isBrowserSkill(step.Skill) {
			continue
		}
		result := parseBrowserResultText(step.Result)
		if step.Error != "" && result.Error == "" {
			result.OK = false
			result.Error = step.Error
		}
		if step.Status == planner.StepFailed && result.Error == "" {
			result.OK = false
			result.Error = "browser action failed"
		}
		return summarizeBrowserSlashArtifact(step.Skill, step.Args, result)
	}
	return nil
}

func suggestedBrowserNextStep(skill string, result BrowserResult) (string, string) {
	switch skill {
	case "browser_mark_elements":
		if result.Total > 0 {
			return "/click ", "Click a marked element"
		}
	case "browser_navigate":
		return "/content", "Read this page"
	case "browser_click", "browser_input", "browser_scroll":
		return "/content", "Inspect the updated page"
	case "browser_get_content":
		return "/mark", "Mark interactive elements"
	}
	return "", ""
}

func summarizeBrowserSlashArtifact(skill string, args map[string]any, result BrowserResult) map[string]any {
	target, _ := args["url"].(string)
	summary := map[string]any{
		"skill":          skill,
		"ok":             result.OK,
		"url":            firstNonEmpty(result.URL, target),
		"title":          result.Title,
		"tab_id":         result.TabID,
		"has_screenshot": result.Screenshot != "",
		"text_length":    len(result.Content),
		"element_count":  result.Total,
	}
	if result.Error != "" {
		summary["error"] = result.Error
	}
	if result.Content != "" {
		preview := result.Content
		if len(preview) > 240 {
			preview = preview[:240] + "..."
		}
		summary["preview"] = preview
	}
	if nextCommand, nextLabel := suggestedBrowserNextStep(skill, result); nextCommand != "" {
		summary["next_command"] = nextCommand
		summary["next_label"] = nextLabel
	}
	return summary
}

func emitSlashToolEvent(req planner.PlanRequest, eventType, skill, summary string, args map[string]any, result string, cb planner.StepCallback) {
	if cb == nil {
		return
	}
	evt := observe.NewEvent(req.TraceID, observe.DomainPlanner, eventType, summary)
	evt.Meta = observe.EventMeta{
		TenantID: req.TenantID,
		TaskID:   req.TaskID,
		Skill:    skill,
	}
	if eventType == observe.EventToolStart {
		evt.Detail = observe.ToolStartDetail{Skill: skill, Args: args}
	} else {
		evt.Detail = observe.ToolResultDetail{Skill: skill, Result: result}
	}
	cb(evt)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
