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
