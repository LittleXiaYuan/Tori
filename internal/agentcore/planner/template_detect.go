package planner

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// UploadAnalysis is the LLM interpretation of an uploaded file (template vs data, etc.).
type UploadAnalysis struct {
	FileKind     string   `json:"file_kind"`
	IsTemplate   bool     `json:"is_template"`
	Summary      string   `json:"summary"`
	Suggestions  []string `json:"suggestions,omitempty"`
	Placeholders []string `json:"placeholders,omitempty"` // detected {{key}} field names
}

// AnalyzeUploadedFile runs a small structured JSON task on a text snippet of the file.
func (p *Planner) AnalyzeUploadedFile(ctx context.Context, filename, textSnippet string) (*UploadAnalysis, error) {
	if p == nil {
		return nil, fmt.Errorf("planner or llm not configured")
	}
	modelRuntime := p.ensureModelRuntime()
	if modelRuntime == nil || modelRuntime.DefaultClient() == nil {
		return nil, fmt.Errorf("planner or llm not configured")
	}
	return modelRuntime.AnalyzeUploadedFile(ctx, filename, textSnippet)
}

// AnalysisToActions builds interactive actions for the upload response flow.
func AnalysisToActions(filePath string, a *UploadAnalysis) []AgentAction {
	if a == nil {
		return nil
	}
	base := filepath.Base(filePath)
	ext := strings.ToLower(filepath.Ext(filePath))

	// Build the question text
	q := fmt.Sprintf("我收到了「%s」（类型：%s）。%s", base, a.FileKind, strings.TrimSpace(a.Summary))
	if len(a.Placeholders) > 0 {
		q += fmt.Sprintf("\n\n检测到 %d 个模板占位符：%s", len(a.Placeholders), strings.Join(a.Placeholders, ", "))
	}
	q += "\n\n你希望怎么处理这个文件？"

	var opts []AskOption

	// If template with placeholders detected, offer fill as primary action
	if len(a.Placeholders) > 0 {
		fieldList := strings.Join(a.Placeholders, "、")
		fillSkill := "docx_fill"
		switch ext {
		case ".xlsx":
			fillSkill = "xlsx_fill"
		case ".pptx":
			fillSkill = "pptx_fill"
		}
		opts = append(opts, AskOption{
			Label: "📝 填充此模板",
			Value: fmt.Sprintf("请用 %s 填充模板 %s，需要填写的字段有：%s。请逐个问我每个字段的值，然后生成文件。", fillSkill, filePath, fieldList),
		})
	} else if a.IsTemplate {
		opts = append(opts, AskOption{
			Label: "📝 按此模板生成内容",
			Value: fmt.Sprintf("请根据文件 %s 的结构，指导我填写并生成一版示例内容。", filePath),
		})
	}

	// Edit option for editable formats
	if ext == ".docx" || ext == ".xlsx" || ext == ".pptx" {
		editSkill := "docx_edit"
		switch ext {
		case ".xlsx":
			editSkill = "xlsx_edit"
		case ".pptx":
			editSkill = "pptx_edit"
		}
		opts = append(opts, AskOption{
			Label: "✏️ 编辑此文件",
			Value: fmt.Sprintf("我想编辑这个文件（%s），请告诉我你能做哪些修改，然后用 %s 执行。", filePath, editSkill),
		})
	}

	// Reference / summary option (always available)
	opts = append(opts, AskOption{
		Label: "📖 作为参考资料",
		Value: fmt.Sprintf("请阅读我上传的文件 %s 并给出结构化摘要与可执行建议。后续对话中可以引用其内容。", base),
	})

	// Archive option
	opts = append(opts, AskOption{
		Label: "📁 仅存档",
		Value: "好的，我已收到文件，暂不需要处理。",
	})

	// LLM-suggested actions
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

var placeholderRe = regexp.MustCompile(`\{\{\s*(\w+)\s*\}\}`)

// detectPlaceholders scans text for {{key}} patterns and returns unique field names.
func detectPlaceholders(text string) []string {
	matches := placeholderRe.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range matches {
		key := m[1]
		if !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	return out
}
