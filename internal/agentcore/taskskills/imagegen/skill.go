package imagegen

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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
	return "根据文字描述生成图片，或基于一张已有图片进行编辑/改图（如换背景、加元素）。输入 prompt (生图/改图提示词，英文效果更好)，可选 size (如 1024x1024)、quality (standard/hd)。若要编辑已有图片（比如用户刚上传的图，或你之前生成的图），传入 input_image_path（对话上下文里“本次对话已有文件”会给出可用的 path）。返回生成的图片文件路径。"
}

func (s *ImageGenerateSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prompt": map[string]any{
				"type":        "string",
				"description": "Image generation/edit prompt (English recommended for best results)",
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
			"input_image_path": map[string]any{
				"type":        "string",
				"description": "Path to an existing image to edit (from an earlier upload or generation in this conversation). Only supported by image-edit-capable providers (e.g. Gemini nano-banana); ignored otherwise.",
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

	if rawPath := stringOr(args, "input_image_path", ""); rawPath != "" {
		data, mime, err := s.resolveInputImage(rawPath)
		if err != nil {
			return "", err
		}
		req.InputImages = []llm.ImageInput{{Data: data, MimeType: mime}}
	}

	slog.Info("image_generate: calling provider", "prompt_len", len(prompt), "size", req.Size, "editing", len(req.InputImages) > 0)
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

// resolveInputImage reads a reference image for edit-mode requests. Reads
// are confined to the app's own data directory (outputDir's parent, e.g.
// "data/" covering data/output, data/uploads, data/tasks) so the model can
// only edit images this app already produced or received — not arbitrary
// paths on the host filesystem.
func (s *ImageGenerateSkill) resolveInputImage(rawPath string) (data []byte, mimeType string, err error) {
	rawPath = strings.TrimSpace(rawPath)
	abs, err := filepath.Abs(rawPath)
	if err != nil {
		return nil, "", fmt.Errorf("invalid input_image_path: %w", err)
	}
	dataRoot, err := filepath.Abs(filepath.Dir(s.outputDir))
	if err != nil {
		return nil, "", fmt.Errorf("invalid output directory: %w", err)
	}
	rel, err := filepath.Rel(dataRoot, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, "", fmt.Errorf("input_image_path 必须在应用数据目录内（如 data/output、data/uploads）")
	}
	data, err = os.ReadFile(abs)
	if err != nil {
		return nil, "", fmt.Errorf("读取参考图片失败: %w", err)
	}
	return data, mimeTypeFromExt(abs), nil
}

func mimeTypeFromExt(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "image/png"
	}
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
