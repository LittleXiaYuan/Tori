package imagegen

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/pkg/skills"
)

type fakeImageGenerator struct {
	lastReq llm.ImageGenerateRequest
	resp    *llm.ImageGenerateResponse
	err     error
}

func (f *fakeImageGenerator) GenerateImage(_ context.Context, req llm.ImageGenerateRequest) (*llm.ImageGenerateResponse, error) {
	f.lastReq = req
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func TestImageGenerateSkillTextToImage(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "output")
	gen := &fakeImageGenerator{resp: &llm.ImageGenerateResponse{
		Artifacts: []llm.ImageArtifact{{B64JSON: "Zm9v", MimeType: "image/png"}},
	}}
	skill := NewImageGenerateSkill(gen, outputDir)

	result, err := skill.Execute(context.Background(), map[string]any{"prompt": "a cat"}, &skills.Environment{})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(gen.lastReq.InputImages) != 0 {
		t.Fatalf("expected no input images for plain generation, got %#v", gen.lastReq.InputImages)
	}
	if !strings.Contains(result, "已生成 1 张图片") {
		t.Fatalf("unexpected result: %q", result)
	}
}

func TestImageGenerateSkillEditModePassesInputImage(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "output")
	uploadsDir := filepath.Join(dir, "uploads")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		t.Fatalf("mkdir uploads: %v", err)
	}
	srcPath := filepath.Join(uploadsDir, "cat.png")
	if err := os.WriteFile(srcPath, []byte("source-bytes"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	gen := &fakeImageGenerator{resp: &llm.ImageGenerateResponse{
		Artifacts: []llm.ImageArtifact{{B64JSON: "ZWRpdGVk", MimeType: "image/png"}},
	}}
	skill := NewImageGenerateSkill(gen, outputDir)

	_, err := skill.Execute(context.Background(), map[string]any{
		"prompt":           "把背景换成雪山",
		"input_image_path": srcPath,
	}, &skills.Environment{})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(gen.lastReq.InputImages) != 1 {
		t.Fatalf("expected 1 input image threaded through, got %#v", gen.lastReq.InputImages)
	}
	if string(gen.lastReq.InputImages[0].Data) != "source-bytes" {
		t.Fatalf("unexpected input image bytes: %q", gen.lastReq.InputImages[0].Data)
	}
	if gen.lastReq.InputImages[0].MimeType != "image/png" {
		t.Fatalf("unexpected mime type: %q", gen.lastReq.InputImages[0].MimeType)
	}
}

func TestImageGenerateSkillRejectsPathOutsideDataDir(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "output")
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "secret.png")
	if err := os.WriteFile(outsidePath, []byte("nope"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	gen := &fakeImageGenerator{resp: &llm.ImageGenerateResponse{Artifacts: []llm.ImageArtifact{{B64JSON: "eA=="}}}}
	skill := NewImageGenerateSkill(gen, outputDir)

	_, err := skill.Execute(context.Background(), map[string]any{
		"prompt":           "edit it",
		"input_image_path": outsidePath,
	}, &skills.Environment{})
	if err == nil {
		t.Fatal("expected an error for a path outside the app data directory")
	}
	if gen.lastReq.Prompt != "" {
		t.Fatalf("generator should not have been called, but received: %#v", gen.lastReq)
	}
}
