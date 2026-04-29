package general

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"yunque-agent/pkg/skills"
)

// ImageGenSkill generates images via OpenAI DALL-E compatible API.
// Configure via environment variables:
//   - IMAGEGEN_API_URL   (default: https://api.openai.com/v1/images/generations)
//   - IMAGEGEN_API_KEY   (required, falls back to OPENAI_API_KEY)
//   - IMAGEGEN_MODEL     (default: dall-e-3)
type ImageGenSkill struct{}

func NewImageGenSkill() *ImageGenSkill { return &ImageGenSkill{} }

func (s *ImageGenSkill) Ready() (bool, string) {
	if os.Getenv("IMAGEGEN_API_KEY") != "" || os.Getenv("OPENAI_API_KEY") != "" || os.Getenv("LLM_API_KEY") != "" {
		return true, ""
	}
	return false, "需要配置 IMAGEGEN_API_KEY 或 OPENAI_API_KEY"
}

func (s *ImageGenSkill) Name() string        { return "image_gen" }
func (s *ImageGenSkill) Description() string { return "AI图片生成：根据文字描述生成图片（DALL-E兼容）" }
func (s *ImageGenSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prompt": map[string]any{
				"type":        "string",
				"description": "图片描述（英文效果最佳，中文会自动翻译）",
			},
			"size": map[string]any{
				"type":        "string",
				"description": "图片尺寸（可选）：1024x1024（默认）、1024x1792、1792x1024",
			},
			"quality": map[string]any{
				"type":        "string",
				"description": "质量（可选）：standard（默认）、hd",
			},
			"style": map[string]any{
				"type":        "string",
				"description": "风格（可选）：vivid（生动，默认）、natural（自然）",
			},
			"n": map[string]any{
				"type":        "integer",
				"description": "生成数量（可选，默认1，最多4）",
			},
		},
		"required": []string{"prompt"},
	}
}

var validSizes = map[string]bool{
	"1024x1024": true,
	"1024x1792": true,
	"1792x1024": true,
	"256x256":   true,
	"512x512":   true,
}

var validQualities = map[string]bool{
	"standard": true,
	"hd":       true,
}

var validStyles = map[string]bool{
	"vivid":   true,
	"natural": true,
}

func (s *ImageGenSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	prompt, _ := args["prompt"].(string)
	size, _ := args["size"].(string)
	quality, _ := args["quality"].(string)
	style, _ := args["style"].(string)
	nFloat, _ := args["n"].(float64)
	n := int(nFloat)

	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}
	if len(prompt) > 4000 {
		return "", fmt.Errorf("prompt too long, max 4000 characters")
	}

	// Defaults
	if size == "" {
		size = "1024x1024"
	}
	if quality == "" {
		quality = "standard"
	}
	if style == "" {
		style = "vivid"
	}
	if n <= 0 {
		n = 1
	}
	if n > 4 {
		return "", fmt.Errorf("n must be between 1 and 4")
	}

	// Validate
	if !validSizes[size] {
		return "", fmt.Errorf("invalid size %q, must be one of: 1024x1024, 1024x1792, 1792x1024, 256x256, 512x512", size)
	}
	if !validQualities[quality] {
		return "", fmt.Errorf("invalid quality %q, must be standard or hd", quality)
	}
	if !validStyles[style] {
		return "", fmt.Errorf("invalid style %q, must be vivid or natural", style)
	}

	// Resolve API config
	apiURL := os.Getenv("IMAGEGEN_API_URL")
	if apiURL == "" {
		if baseURL := os.Getenv("LLM_BASE_URL"); baseURL != "" {
			apiURL = strings.TrimRight(baseURL, "/") + "/images/generations"
		} else {
			apiURL = "https://api.openai.com/v1/images/generations"
		}
	}
	apiKey := os.Getenv("IMAGEGEN_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("LLM_API_KEY")
	}
	if apiKey == "" {
		return "", fmt.Errorf("需要配置 IMAGEGEN_API_KEY、OPENAI_API_KEY 或 LLM_API_KEY")
	}
	model := os.Getenv("IMAGEGEN_MODEL")
	if model == "" {
		model = "dall-e-3"
	}

	// If prompt is Chinese, translate to English for better results
	if containsChinese(prompt) && env.LLMCall != nil {
		translated, err := env.LLMCall(ctx,
			"Translate the following image description to English. Output ONLY the English translation, nothing else.",
			prompt,
		)
		if err == nil && strings.TrimSpace(translated) != "" {
			prompt = strings.TrimSpace(translated)
			// Strip quotes
			if len(prompt) > 1 && prompt[0] == '"' && prompt[len(prompt)-1] == '"' {
				prompt = prompt[1 : len(prompt)-1]
			}
		}
	}

	// Build request
	reqBody := map[string]any{
		"model":   model,
		"prompt":  prompt,
		"n":       n,
		"size":    size,
		"quality": quality,
		"style":   style,
	}

	result, err := callImageAPI(ctx, apiURL, apiKey, reqBody)
	if err != nil {
		return "", err
	}

	return result, nil
}

type imageAPIResponse struct {
	Created int64 `json:"created"`
	Data    []struct {
		URL           string `json:"url"`
		RevisedPrompt string `json:"revised_prompt"`
		B64JSON       string `json:"b64_json"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func callImageAPI(ctx context.Context, apiURL, apiKey string, reqBody map[string]any) (string, error) {
	b, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(b))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("image API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var apiResp imageAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("decode response: %w (status %d)", err, resp.StatusCode)
	}

	if apiResp.Error != nil {
		return "", fmt.Errorf("image API error: %s (%s)", apiResp.Error.Message, apiResp.Error.Code)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("image API returned status %d: %s", resp.StatusCode, string(body))
	}

	if len(apiResp.Data) == 0 {
		return "", fmt.Errorf("image API returned no images")
	}

	// Format results
	var sb strings.Builder
	for i, img := range apiResp.Data {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		if len(apiResp.Data) > 1 {
			sb.WriteString(fmt.Sprintf("### 图片 %d\n", i+1))
		}
		if img.URL != "" {
			sb.WriteString(fmt.Sprintf("![生成的图片](%s)\n", img.URL))
		}
		if img.RevisedPrompt != "" {
			sb.WriteString(fmt.Sprintf("优化后的描述：%s", img.RevisedPrompt))
		}
	}

	return sb.String(), nil
}

// containsChinese checks if string contains CJK characters.
func containsChinese(s string) bool {
	for _, r := range s {
		if r >= 0x4E00 && r <= 0x9FFF {
			return true
		}
	}
	return false
}
