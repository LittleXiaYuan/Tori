package general

import (
	"context"
	"fmt"
	"strings"

	"yunque-agent/pkg/skills"
)

// TranslateSkill provides multi-language translation powered by LLM.
type TranslateSkill struct{}

func NewTranslateSkill() *TranslateSkill { return &TranslateSkill{} }

func (s *TranslateSkill) Name() string        { return "translate" }
func (s *TranslateSkill) Description() string  { return "多语言翻译：自动检测源语言，翻译为目标语言" }
func (s *TranslateSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "要翻译的文本",
			},
			"target_lang": map[string]any{
				"type":        "string",
				"description": "目标语言（如：中文、英语、日语、韩语、法语、德语、西班牙语、俄语等）",
			},
			"source_lang": map[string]any{
				"type":        "string",
				"description": "源语言（可选，默认自动检测）",
			},
			"style": map[string]any{
				"type":        "string",
				"description": "翻译风格（可选）：formal（正式）、casual（口语）、technical（技术）、literary（文学）",
			},
		},
		"required": []string{"text", "target_lang"},
	}
}

// supportedLanguages lists recognized language names for validation.
var supportedLanguages = map[string]bool{
	"中文": true, "chinese": true, "简体中文": true, "繁体中文": true,
	"英语": true, "english": true, "英文": true,
	"日语": true, "japanese": true, "日文": true,
	"韩语": true, "korean": true, "韩文": true,
	"法语": true, "french": true, "法文": true,
	"德语": true, "german": true, "德文": true,
	"西班牙语": true, "spanish": true,
	"俄语": true, "russian": true, "俄文": true,
	"葡萄牙语": true, "portuguese": true,
	"意大利语": true, "italian": true,
	"阿拉伯语": true, "arabic": true,
	"泰语": true, "thai": true,
	"越南语": true, "vietnamese": true,
	"印地语": true, "hindi": true,
	"马来语": true, "malay": true,
	"印尼语": true, "indonesian": true,
	"土耳其语": true, "turkish": true,
	"波兰语": true, "polish": true,
	"荷兰语": true, "dutch": true,
	"瑞典语": true, "swedish": true,
}

var styleLabels = map[string]string{
	"formal":    "正式、书面",
	"casual":    "口语化、日常",
	"technical": "技术性、精准术语",
	"literary":  "文学性、优美",
}

func (s *TranslateSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	text, _ := args["text"].(string)
	targetLang, _ := args["target_lang"].(string)
	sourceLang, _ := args["source_lang"].(string)
	style, _ := args["style"].(string)

	text = strings.TrimSpace(text)
	targetLang = strings.TrimSpace(targetLang)

	if text == "" {
		return "", fmt.Errorf("text is required")
	}
	if targetLang == "" {
		return "", fmt.Errorf("target_lang is required")
	}
	if len(text) > 10000 {
		return "", fmt.Errorf("text too long, max 10000 characters")
	}

	// Validate target language
	if !supportedLanguages[strings.ToLower(targetLang)] {
		return "", fmt.Errorf("unsupported target language: %s", targetLang)
	}

	if env.LLMCall == nil {
		return "", fmt.Errorf("LLM not available")
	}

	system := buildTranslatePrompt(targetLang, sourceLang, style)
	result, err := env.LLMCall(ctx, system, text)
	if err != nil {
		return "", fmt.Errorf("translation failed: %w", err)
	}

	result = strings.TrimSpace(result)
	// Remove any wrapping quotes the LLM might add
	if len(result) > 1 && result[0] == '"' && result[len(result)-1] == '"' {
		result = result[1 : len(result)-1]
	}

	return result, nil
}

func buildTranslatePrompt(targetLang, sourceLang, style string) string {
	var sb strings.Builder
	sb.WriteString("你是一个专业翻译引擎。你的任务是将用户提供的文本翻译为")
	sb.WriteString(targetLang)
	sb.WriteString("。\n\n")

	if sourceLang != "" {
		sb.WriteString("源语言：")
		sb.WriteString(sourceLang)
		sb.WriteString("\n")
	} else {
		sb.WriteString("请自动检测源语言。\n")
	}

	if style != "" {
		if label, ok := styleLabels[style]; ok {
			sb.WriteString("翻译风格：")
			sb.WriteString(label)
			sb.WriteString("\n")
		}
	}

	sb.WriteString(`
规则：
1. 只输出翻译结果，不要包含任何解释、注释或原文
2. 保持原文的格式（段落、列表、标点等）
3. 专有名词保持原文或使用通用译法
4. 翻译要自然流畅，符合目标语言的表达习惯
5. 如果原文已经是目标语言，直接原样返回`)

	return sb.String()
}
