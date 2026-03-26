package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"yunque-agent/internal/agentcore/llm"
)

// UploadAnalysis is the LLM interpretation of an uploaded file (template vs data, etc.).
type UploadAnalysis struct {
	FileKind    string   `json:"file_kind"`
	IsTemplate  bool     `json:"is_template"`
	Summary     string   `json:"summary"`
	Suggestions []string `json:"suggestions,omitempty"`
}

// AnalyzeUploadedFile runs a small structured JSON task on a text snippet of the file.
func (p *Planner) AnalyzeUploadedFile(ctx context.Context, filename, textSnippet string) (*UploadAnalysis, error) {
	if p == nil || p.llm == nil {
		return nil, fmt.Errorf("planner or llm not configured")
	}
	snippet := textSnippet
	if len([]rune(snippet)) > 8000 {
		snippet = string([]rune(snippet)[:8000])
	}
	system := `你是文件分析助手。只输出一段合法 JSON，不要 markdown 代码块。格式：
{"file_kind":"xlsx|docx|csv|pdf|txt|other","is_template":true/false,"summary":"一句话说明文件用途与结构","suggestions":["可选的后续动作短句"]}
is_template：是否为表单/模板/需用户填写的范式文件。`
	user := fmt.Sprintf("文件名: %s\n\n内容预览:\n%s", filename, snippet)
	msgs := []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: user}}
	raw, err := p.llm.Chat(ctx, msgs, 0.2)
	if err != nil {
		return nil, err
	}
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "{"); i >= 0 {
		raw = raw[i:]
	}
	if j := strings.LastIndex(raw, "}"); j >= 0 {
		raw = raw[:j+1]
	}
	var a UploadAnalysis
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		return &UploadAnalysis{
			FileKind:   "unknown",
			Summary:    "无法自动解析元数据，可直接描述你的目标。",
			IsTemplate: strings.Contains(strings.ToLower(filename), "模板"),
		}, nil
	}
	return &a, nil
}

// AnalysisToActions builds interactive actions for the upload response flow.
func AnalysisToActions(filePath string, a *UploadAnalysis) []AgentAction {
	if a == nil {
		return nil
	}
	base := filepath.Base(filePath)
	q := fmt.Sprintf("我收到了「%s」（类型：%s）。%s 你希望我怎么帮你？", base, a.FileKind, strings.TrimSpace(a.Summary))
	opts := []AskOption{
		{Label: "摘要与要点", Value: fmt.Sprintf("请阅读我上传的文件 %s 并给出结构化摘要与可执行建议。", base)},
		{Label: "按模板/表格处理", Value: fmt.Sprintf("这是模板或表格文件（路径：%s），请说明如何填写或如何拆分/汇总。", filePath)},
		{Label: "仅存档", Value: "好的，我已收到文件，暂不需要处理。"},
	}
	if a.IsTemplate {
		opts = append([]AskOption{
			{Label: "按此模板生成内容", Value: fmt.Sprintf("请根据文件 %s 的结构，指导我填写并生成一版示例内容。", filePath)},
		}, opts...)
	}
	for _, s := range a.Suggestions {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		opts = append(opts, AskOption{Label: truncateRunes(s, 24), Value: s})
	}
	actions := []AgentAction{
		AskAction(q, opts...),
		FileAction(filePath, base, "", 0),
	}
	return actions
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
