package imagegen

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/pkg/skills"
)

// ImageGenerateSkill is a function-calling skill that generates images.
type ImageGenerateSkill struct {
	gen       llm.ImageGenerator
	outputDir string
}

func NewImageGenerateSkill(gen llm.ImageGenerator, outputDir string) *ImageGenerateSkill {
	return &ImageGenerateSkill{gen: gen, outputDir: outputDir}
}

func (s *ImageGenerateSkill) SetGenerator(gen llm.ImageGenerator) {
	s.gen = gen
}

func (s *ImageGenerateSkill) Name() string { return "image_generate" }
func (s *ImageGenerateSkill) Description() string {
	return "根据文字描述生成图片。输入 prompt (生图提示词，英文效果更好)，可选 size (如 1024x1024)、quality (standard/hd)。返回生成的图片文件路径。"
}

func (s *ImageGenerateSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prompt": map[string]any{
				"type":        "string",
				"description": "Image generation prompt (English recommended for best results)",
			},
			"size": map[string]any{
				"type":        "string",
				"description": "Image size, e.g. 1024x1024, 1792x1024, 1024x1792",
				"default":     "1024x1024",
			},
			"quality": map[string]any{
				"type":        "string",
				"description": "Image quality: standard or hd",
				"default":     "standard",
			},
			"negative_prompt": map[string]any{
				"type":        "string",
				"description": "Negative prompt (what to avoid in the image)",
			},
		},
		"required": []string{"prompt"},
	}
}

func (s *ImageGenerateSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	if s.gen == nil {
		return "", fmt.Errorf("图像生成未配置。请在设置中添加支持生图的模型（如 OpenAI gpt-image-1 或 Qwen Image）")
	}

	prompt, _ := args["prompt"].(string)
	if prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}

	req := llm.ImageGenerateRequest{
		Prompt:      prompt,
		Size:        stringOr(args, "size", "1024x1024"),
		Quality:     stringOr(args, "quality", "standard"),
		Negative:    stringOr(args, "negative_prompt", ""),
		N:           1,
		ResponseFmt: "b64_json",
	}

	slog.Info("image_generate: calling provider", "prompt_len", len(prompt), "size", req.Size)
	resp, err := s.gen.GenerateImage(ctx, req)
	if err != nil {
		return "", fmt.Errorf("生图失败: %w", err)
	}

	if len(resp.Artifacts) == 0 {
		return "", fmt.Errorf("生图失败: 未返回图片")
	}

	_ = os.MkdirAll(s.outputDir, 0755)
	var paths []string
	for i, art := range resp.Artifacts {
		data, err := art.ImageData(ctx)
		if err != nil {
			slog.Warn("image_generate: download failed", "idx", i, "err", err)
			continue
		}

		ext := ".png"
		if art.MimeType == "image/jpeg" {
			ext = ".jpg"
		} else if art.MimeType == "image/webp" {
			ext = ".webp"
		}

		filename := fmt.Sprintf("generated_%s_%d%s", time.Now().Format("20060102_150405"), i, ext)
		fpath := filepath.Join(s.outputDir, filename)
		if err := os.WriteFile(fpath, data, 0644); err != nil {
			slog.Warn("image_generate: save failed", "path", fpath, "err", err)
			continue
		}
		paths = append(paths, fpath)
		slog.Info("image_generate: saved", "path", fpath, "size", len(data))
	}

	if len(paths) == 0 {
		return "", fmt.Errorf("生图失败: 无法保存图片")
	}

	result := fmt.Sprintf("已生成 %d 张图片:\n", len(paths))
	for _, p := range paths {
		result += fmt.Sprintf("- %s\n", p)
	}
	if resp.Artifacts[0].RevisedPrompt != "" {
		result += fmt.Sprintf("\n优化后的提示词: %s", resp.Artifacts[0].RevisedPrompt)
	}

	// Return base64 preview for first image in the result
	if resp.Artifacts[0].B64JSON != "" {
		preview := resp.Artifacts[0].B64JSON
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		_ = preview
	}

	return result, nil
}

func stringOr(args map[string]any, key, def string) string {
	if v, ok := args[key].(string); ok && v != "" {
		return v
	}
	return def
}

// Base64Preview returns base64 data URL for first artifact (for frontend display).
func Base64Preview(art llm.ImageArtifact) string {
	if art.B64JSON != "" {
		return "data:" + art.MimeType + ";base64," + art.B64JSON
	}
	if art.URL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		data, err := art.ImageData(ctx)
		if err != nil {
			return ""
		}
		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
	}
	return ""
}
