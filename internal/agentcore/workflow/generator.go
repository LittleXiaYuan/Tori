package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// LLMCallFunc is the minimal LLM interface used by the natural-language
// workflow generator. It intentionally mirrors skills.LLMCallFunc without
// importing the skills package into the workflow core.
type LLMCallFunc func(ctx context.Context, system, user string) (string, error)

// GenerationSource identifies whether a workflow came from the LLM or from the
// deterministic template fallback.
type GenerationSource string

const (
	GenerationSourceLLM      GenerationSource = "llm"
	GenerationSourceTemplate GenerationSource = "template"
)

// GenerationResult is returned by GenerateDefinition.
type GenerationResult struct {
	Definition *Definition
	Source     GenerationSource
	Message    string
	RawOutput  string
}

// GeneratorOptions controls natural-language workflow generation.
type GeneratorOptions struct {
	TenantID string
	LLMCall  LLMCallFunc
	Now      func() time.Time
}

const nl2WorkflowPrompt = `你是云雀的 NL2Workflow 引擎，负责把自然语言需求转换为可执行、可编辑的 DAG 工作流定义 JSON。

只返回纯 JSON，不要 Markdown，不要解释。字段必须符合：
{
  "name": "简短中文名称",
  "description": "用户需求摘要",
  "nodes": [
    {"id":"start","name":"开始","type":"start","position":{"x":80,"y":180}},
    {"id":"...","name":"...","type":"knowledge|llm|skill|condition|transform|browser|code|end","config":{},"position":{"x":260,"y":180}}
  ],
  "edges": [{"id":"e_start_x","from_node":"start","to_node":"...","label":"..."}],
  "variables": [{"name":"input","type":"string","required":false,"description":"..."}]
}

节点类型优先级：
- 涉及知识库、资料、文档检索：knowledge，config: {"query":"...","top_k":5}
- 涉及总结、撰写、判断、生成内容：llm，config: {"system_prompt":"...","user_prompt":"..."}
- 涉及发邮件/消息/内部工具：skill，config: {"skill_name":"send_email|send_msg|...","args":{}}
- 涉及网页操作：browser，config: {"action":"navigate|click|type|read|screenshot","target":"..."}
- 需要格式转换：transform，config: {"template":"..."}
- 需要代码处理：code，config: {"language":"javascript|python","code":"..."}

要求：
1. 必须包含 start 与 end 节点。
2. 节点 ID 使用小写字母、数字、下划线。
3. 生成 4 到 8 个节点，保持能在演示中读懂。
4. 只使用上述结构，不要输出未知字段。`

// GenerateDefinition converts a natural-language requirement into a workflow
// definition. It tries the injected LLM first and falls back to a deterministic
// template so the desktop demo remains usable without a configured provider.
func GenerateDefinition(ctx context.Context, requirement string, opts GeneratorOptions) (*GenerationResult, error) {
	requirement = strings.TrimSpace(requirement)
	if requirement == "" {
		return nil, fmt.Errorf("requirement is required")
	}

	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	tenantID := strings.TrimSpace(opts.TenantID)
	if tenantID == "" {
		tenantID = "default"
	}

	if opts.LLMCall != nil {
		raw, err := opts.LLMCall(ctx, nl2WorkflowPrompt, requirement)
		if err == nil {
			def, parseErr := ParseGeneratedDefinition(raw)
			if parseErr == nil {
				NormalizeDefinition(def, requirement, tenantID, now())
				return &GenerationResult{
					Definition: def,
					Source:     GenerationSourceLLM,
					Message:    "已通过模型生成并保存工作流，可继续编辑或直接试运行。",
					RawOutput:  raw,
				}, nil
			}
		}
	}

	def := GenerateTemplateDefinition(requirement, tenantID, now())
	return &GenerationResult{
		Definition: def,
		Source:     GenerationSourceTemplate,
		Message:    "当前模型不可用或返回无法解析，已使用内置模板生成可演示工作流。",
	}, nil
}

// ParseGeneratedDefinition extracts and parses a workflow definition from an
// LLM reply. It accepts fenced JSON because many providers add Markdown even
// when instructed not to.
func ParseGeneratedDefinition(raw string) (*Definition, error) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)
	if start := strings.Index(cleaned, "{"); start > 0 {
		cleaned = cleaned[start:]
	}
	if end := strings.LastIndex(cleaned, "}"); end >= 0 && end < len(cleaned)-1 {
		cleaned = cleaned[:end+1]
	}

	var def Definition
	if err := json.Unmarshal([]byte(cleaned), &def); err != nil {
		return nil, fmt.Errorf("parse generated workflow JSON: %w", err)
	}
	if strings.TrimSpace(def.Name) == "" {
		return nil, fmt.Errorf("generated workflow missing name")
	}
	if len(def.Nodes) == 0 {
		return nil, fmt.Errorf("generated workflow missing nodes")
	}
	return &def, nil
}

// NormalizeDefinition fills defaults, sanitizes node/edge fields, and ensures
// the generated graph is easy to open in the visual editor.
func NormalizeDefinition(def *Definition, requirement, tenantID string, now time.Time) {
	if def == nil {
		return
	}
	if def.ID == "" {
		def.ID = fmt.Sprintf("wf_%d", now.UnixNano())
	}
	if def.Version <= 0 {
		def.Version = 1
	}
	def.TenantID = tenantID
	if def.Description == "" {
		def.Description = summarizeRequirement(requirement, 120)
	}
	if def.CreatedAt.IsZero() {
		def.CreatedAt = now
	}
	def.UpdatedAt = now

	seenNodes := map[string]bool{}
	for i := range def.Nodes {
		n := &def.Nodes[i]
		if n.ID == "" {
			n.ID = fmt.Sprintf("node_%d", i+1)
		}
		n.ID = sanitizeID(n.ID, fmt.Sprintf("node_%d", i+1))
		for seenNodes[n.ID] {
			n.ID = fmt.Sprintf("%s_%d", n.ID, i+1)
		}
		seenNodes[n.ID] = true
		if n.Name == "" {
			n.Name = defaultNodeName(n.Type, i)
		}
		if n.Type == "" {
			n.Type = NodeLLM
		}
		if n.Config == nil {
			n.Config = map[string]any{}
		}
		if n.Position.X == 0 && n.Position.Y == 0 {
			n.Position = Position{X: float64(80 + i*220), Y: float64(160 + (i%2)*110)}
		}
	}

	if len(def.Nodes) > 0 && !hasNodeType(def.Nodes, NodeStart) {
		start := Node{ID: uniqueNodeID(seenNodes, "start"), Name: "开始", Type: NodeStart, Position: Position{X: 80, Y: 180}, Config: map[string]any{}}
		def.Nodes = append([]Node{start}, def.Nodes...)
		seenNodes[start.ID] = true
	}
	if len(def.Nodes) > 0 && !hasNodeType(def.Nodes, NodeEnd) {
		end := Node{ID: uniqueNodeID(seenNodes, "end"), Name: "结束", Type: NodeEnd, Position: Position{X: float64(80 + len(def.Nodes)*220), Y: 180}, Config: map[string]any{}}
		def.Nodes = append(def.Nodes, end)
		seenNodes[end.ID] = true
	}

	validEdges := make([]Edge, 0, len(def.Edges))
	seenEdges := map[string]bool{}
	for i, e := range def.Edges {
		if !seenNodes[e.FromNode] || !seenNodes[e.ToNode] || e.FromNode == e.ToNode {
			continue
		}
		if e.ID == "" {
			e.ID = fmt.Sprintf("edge_%s_%s", e.FromNode, e.ToNode)
		}
		e.ID = sanitizeID(e.ID, fmt.Sprintf("edge_%d", i+1))
		if seenEdges[e.ID] {
			e.ID = fmt.Sprintf("%s_%d", e.ID, i+1)
		}
		seenEdges[e.ID] = true
		validEdges = append(validEdges, e)
	}
	def.Edges = validEdges
	ensureLinearConnectivity(def)
}

// GenerateTemplateDefinition creates a deterministic, executable definition for
// demos when no provider is configured.
func GenerateTemplateDefinition(requirement, tenantID string, now time.Time) *Definition {
	if looksLikeSocialPublishWorkflow(requirement) {
		return GenerateSocialPublishTemplateDefinition(requirement, tenantID, now)
	}

	short := summarizeRequirement(requirement, 60)
	name := "智能工作流：" + short
	if utf8.RuneCountInString(name) > 34 {
		name = string([]rune(name)[:34])
	}
	def := &Definition{
		ID:          fmt.Sprintf("wf_%d", now.UnixNano()),
		Name:        name,
		Description: summarizeRequirement(requirement, 160),
		Version:     1,
		TenantID:    tenantID,
		CreatedAt:   now,
		UpdatedAt:   now,
		Variables: []Variable{{
			Name:        "input",
			Type:        "string",
			Required:    false,
			Description: "运行时补充输入，例如日期、目标渠道或文件路径。",
		}},
		Nodes: []Node{
			{ID: "start", Name: "开始", Type: NodeStart, Position: Position{X: 80, Y: 180}, Config: map[string]any{}},
			{ID: "understand", Name: "理解需求", Type: NodeLLM, Position: Position{X: 300, Y: 180}, Config: map[string]any{
				"system_prompt": "你是云雀工作流中的需求分析节点，请提取目标、输入、输出与约束。",
				"user_prompt":   requirement,
			}},
			{ID: "collect_context", Name: "检索相关资料", Type: NodeKnowledge, Position: Position{X: 520, Y: 180}, Config: map[string]any{
				"query": requirement,
				"top_k": 5,
			}},
			{ID: "draft_result", Name: "生成执行结果", Type: NodeLLM, Position: Position{X: 740, Y: 180}, Config: map[string]any{
				"system_prompt": "你是云雀的任务执行节点，请结合前序分析和资料生成结构化结果。",
				"user_prompt":   "需求：" + requirement + "\n请输出可交付结果、风险点与下一步动作。",
			}},
			{ID: "format_output", Name: "整理输出", Type: NodeTransform, Position: Position{X: 960, Y: 180}, Config: map[string]any{
				"template": "已根据需求生成工作流结果：{_node_draft_result}",
			}},
			{ID: "end", Name: "结束", Type: NodeEnd, Position: Position{X: 1180, Y: 180}, Config: map[string]any{}},
		},
		Edges: []Edge{
			{ID: "edge_start_understand", FromNode: "start", ToNode: "understand", Label: "开始分析"},
			{ID: "edge_understand_collect", FromNode: "understand", ToNode: "collect_context", Label: "补充上下文"},
			{ID: "edge_collect_draft", FromNode: "collect_context", ToNode: "draft_result", Label: "生成"},
			{ID: "edge_draft_format", FromNode: "draft_result", ToNode: "format_output", Label: "整理"},
			{ID: "edge_format_end", FromNode: "format_output", ToNode: "end", Label: "完成"},
		},
	}
	return def
}

// GenerateSocialPublishTemplateDefinition creates an executable browser workflow
// for social-content publishing demos. It intentionally models direct publish
// as a real browser action instead of a draft-only placeholder so operators can
// demonstrate efficiency gains on an already logged-in browser session.
func GenerateSocialPublishTemplateDefinition(requirement, tenantID string, now time.Time) *Definition {
	lower := strings.ToLower(requirement)
	platformName := "内容平台"
	publishURL := "https://creator.xiaohongshu.com/publish/publish"
	titleSelector := `input[placeholder*="标题"], textarea[placeholder*="标题"]`
	bodySelector := `[contenteditable="true"], textarea[placeholder*="正文"], .ql-editor, .ProseMirror`
	publishButtonText := "发布"
	defaultTitle := "云雀自动化效率演示"
	defaultBody := "今天用云雀 Agent 演示内容运营自动化：自动打开创作中心、填写内容、截图留痕，并在满足平台条件后直接点击发布。"

	switch {
	case strings.Contains(requirement, "小红书") || strings.Contains(lower, "xiaohongshu") || strings.Contains(lower, "rednote"):
		platformName = "小红书"
	case strings.Contains(requirement, "微博") || strings.Contains(lower, "weibo"):
		platformName = "微博"
		publishURL = "https://weibo.com"
		titleSelector = ""
		bodySelector = `textarea, [contenteditable="true"]`
		publishButtonText = "发布"
	case strings.Contains(requirement, "twitter") || strings.Contains(requirement, "推特") || strings.Contains(lower, "x.com"):
		platformName = "X/Twitter"
		publishURL = "https://x.com/home"
		titleSelector = ""
		bodySelector = `[data-testid="tweetTextarea_0"]`
		publishButtonText = "Post"
		defaultBody = "Yunque Agent 正在演示浏览器自动化：打开页面、填写内容并直接发布，减少重复运营操作。"
	}

	nodes := []Node{
		{ID: "start", Name: "开始", Type: NodeStart, Position: Position{X: 80, Y: 180}, Config: map[string]any{}},
		{ID: "open_publish_page", Name: "打开" + platformName + "发布页", Type: NodeBrowser, Position: Position{X: 300, Y: 180}, Config: map[string]any{
			"action": "navigate",
			"target": publishURL,
		}},
		{ID: "write_content", Name: "生成发布文案", Type: NodeLLM, Position: Position{X: 520, Y: 180}, Config: map[string]any{
			"system_prompt": "你是云雀内容运营节点，请根据需求输出适合直接发布的标题和正文，避免冗长口号。",
			"user_prompt":   "平台：" + platformName + "\n需求：" + requirement + "\n请输出标题与正文。",
		}},
	}
	edges := []Edge{
		{ID: "edge_start_open", FromNode: "start", ToNode: "open_publish_page", Label: "打开平台"},
		{ID: "edge_open_write", FromNode: "open_publish_page", ToNode: "write_content", Label: "准备内容"},
	}
	lastID := "write_content"

	if titleSelector != "" {
		nodes = append(nodes, Node{ID: "fill_title", Name: "填写标题", Type: NodeBrowser, Position: Position{X: 740, Y: 120}, Config: map[string]any{
			"action":   "input",
			"selector": titleSelector,
			"text":     defaultTitle,
		}})
		edges = append(edges, Edge{ID: "edge_write_title", FromNode: "write_content", ToNode: "fill_title", Label: "标题"})
		lastID = "fill_title"
	}

	nodes = append(nodes,
		Node{ID: "fill_body", Name: "填写正文", Type: NodeBrowser, Position: Position{X: 740, Y: 240}, Config: map[string]any{
			"action":   "input",
			"selector": bodySelector,
			"text":     defaultBody,
		}},
		Node{ID: "capture_before_publish", Name: "发布前截图", Type: NodeBrowser, Position: Position{X: 960, Y: 180}, Config: map[string]any{
			"action": "screenshot",
		}},
		Node{ID: "publish", Name: "点击发布", Type: NodeBrowser, Position: Position{X: 1180, Y: 180}, Config: map[string]any{
			"action":      "click",
			"text_target": publishButtonText,
		}},
		Node{ID: "capture_after_publish", Name: "发布后截图", Type: NodeBrowser, Position: Position{X: 1400, Y: 180}, Config: map[string]any{
			"action": "screenshot",
		}},
		Node{ID: "end", Name: "结束", Type: NodeEnd, Position: Position{X: 1620, Y: 180}, Config: map[string]any{}},
	)
	edges = append(edges,
		Edge{ID: "edge_" + lastID + "_body", FromNode: lastID, ToNode: "fill_body", Label: "正文"},
		Edge{ID: "edge_body_capture", FromNode: "fill_body", ToNode: "capture_before_publish", Label: "留痕"},
		Edge{ID: "edge_capture_publish", FromNode: "capture_before_publish", ToNode: "publish", Label: "直发"},
		Edge{ID: "edge_publish_after", FromNode: "publish", ToNode: "capture_after_publish", Label: "确认"},
		Edge{ID: "edge_after_end", FromNode: "capture_after_publish", ToNode: "end", Label: "完成"},
	)

	return &Definition{
		ID:          fmt.Sprintf("wf_%d", now.UnixNano()),
		Name:        platformName + "直发自动化",
		Description: summarizeRequirement(requirement, 160),
		Version:     1,
		TenantID:    tenantID,
		CreatedAt:   now,
		UpdatedAt:   now,
		Variables: []Variable{
			{Name: "content_goal", Type: "string", Required: false, Description: "发布目标或活动主题，可覆盖默认演示文案。"},
			{Name: "asset_path", Type: "string", Required: false, Description: "待上传素材路径；当前浏览器扩展直发演示先覆盖文字发布链路。"},
		},
		Nodes: nodes,
		Edges: edges,
	}
}

func looksLikeSocialPublishWorkflow(requirement string) bool {
	text := strings.ToLower(requirement)
	social := []string{"小红书", "xiaohongshu", "rednote", "微博", "weibo", "twitter", "x.com", "推特", "抖音", "douyin", "bilibili"}
	publish := []string{"发帖", "发布", "直发", "post", "publish", "投稿", "笔记", "内容运营"}
	hasSocial := false
	for _, token := range social {
		if strings.Contains(text, strings.ToLower(token)) {
			hasSocial = true
			break
		}
	}
	if !hasSocial {
		return false
	}
	for _, token := range publish {
		if strings.Contains(text, strings.ToLower(token)) {
			return true
		}
	}
	return false
}

func ensureLinearConnectivity(def *Definition) {
	if len(def.Nodes) < 2 || len(def.Edges) > 0 {
		return
	}
	for i := 0; i < len(def.Nodes)-1; i++ {
		from := def.Nodes[i].ID
		to := def.Nodes[i+1].ID
		def.Edges = append(def.Edges, Edge{
			ID:       fmt.Sprintf("edge_%s_%s", from, to),
			FromNode: from,
			ToNode:   to,
		})
	}
}

func hasNodeType(nodes []Node, typ NodeType) bool {
	for _, n := range nodes {
		if n.Type == typ {
			return true
		}
	}
	return false
}

func uniqueNodeID(seen map[string]bool, base string) string {
	id := sanitizeID(base, "node")
	if !seen[id] {
		return id
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s_%d", id, i)
		if !seen[candidate] {
			return candidate
		}
	}
}

var invalidIDChars = regexp.MustCompile(`[^a-z0-9_]+`)

func sanitizeID(id, fallback string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	id = strings.ReplaceAll(id, "-", "_")
	id = invalidIDChars.ReplaceAllString(id, "_")
	id = strings.Trim(id, "_")
	if id == "" {
		return fallback
	}
	return id
}

func summarizeRequirement(text string, maxRunes int) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if text == "" {
		return "自动化任务"
	}
	r := []rune(text)
	if len(r) <= maxRunes {
		return text
	}
	return string(r[:maxRunes]) + "…"
}

func defaultNodeName(typ NodeType, idx int) string {
	switch typ {
	case NodeStart:
		return "开始"
	case NodeEnd:
		return "结束"
	case NodeKnowledge:
		return "检索资料"
	case NodeLLM:
		return "模型处理"
	case NodeSkill:
		return "调用能力"
	case NodeCondition:
		return "条件判断"
	case NodeTransform:
		return "整理数据"
	case NodeBrowser:
		return "浏览器操作"
	case NodeCode:
		return "代码处理"
	default:
		return fmt.Sprintf("节点 %d", idx+1)
	}
}
