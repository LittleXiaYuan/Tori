package general

import (
	"archive/zip"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"yunque-agent/pkg/skills"
)

func TestHtmlExportSkill(t *testing.T) {
	dir := t.TempDir()
	s := NewHtmlExportSkill([]string{dir})

	t.Run("basic markdown to html", func(t *testing.T) {
		p := filepath.Join(dir, "test.html")
		result, err := s.Execute(context.Background(), map[string]any{
			"path":    p,
			"title":   "测试报告",
			"content": "# 标题\n\n这是正文\n\n## 二级标题\n\n- 列表项1\n- 列表项2\n\n**加粗文本**",
		}, &skills.Environment{})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result, ".html") {
			t.Fatalf("unexpected result: %s", result)
		}

		data, _ := os.ReadFile(p)
		html := string(data)
		if !strings.Contains(html, "<h1>标题</h1>") {
			t.Error("missing h1")
		}
		if !strings.Contains(html, "<h2>二级标题</h2>") {
			t.Error("missing h2")
		}
		if !strings.Contains(html, "<li>列表项1</li>") {
			t.Error("missing list item")
		}
		if !strings.Contains(html, "<strong>加粗文本</strong>") {
			t.Error("missing bold")
		}
		if !strings.Contains(html, "<title>测试报告</title>") {
			t.Error("missing title")
		}
	})

	t.Run("code block", func(t *testing.T) {
		p := filepath.Join(dir, "code.html")
		_, err := s.Execute(context.Background(), map[string]any{
			"path":    p,
			"content": "# Code\n\n```\nfunc main() {}\n```",
		}, &skills.Environment{})
		if err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(p)
		if !strings.Contains(string(data), "<pre><code>") {
			t.Error("missing code block")
		}
	})

	t.Run("access denied", func(t *testing.T) {
		_, err := s.Execute(context.Background(), map[string]any{
			"path":    "/etc/test.html",
			"content": "test",
		}, &skills.Environment{})
		if err == nil {
			t.Fatal("expected access denied error")
		}
	})
}

func TestPptxCreateSkill(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		if _, err2 := exec.LookPath("python"); err2 != nil {
			t.Skip("skipping: python not found in PATH")
		}
	}
	dir := t.TempDir()
	s := NewPptxCreateSkill([]string{dir})

	t.Run("basic slides", func(t *testing.T) {
		p := filepath.Join(dir, "test.pptx")
		result, err := s.Execute(context.Background(), map[string]any{
			"path": p,
			"content": `# 封面
欢迎使用云雀 Agent
---
# 功能介绍
多步规划
插件架构
五层记忆
---
总结
感谢使用`,
		}, &skills.Environment{})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result, "3 张幻灯片") {
			t.Fatalf("unexpected result: %s", result)
		}

		// Verify it's a valid zip with slide XMLs
		r, err := zip.OpenReader(p)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()

		files := make(map[string]bool)
		for _, f := range r.File {
			files[f.Name] = true
		}
		expected := []string{
			"[Content_Types].xml",
			"_rels/.rels",
			"ppt/presentation.xml",
			"ppt/_rels/presentation.xml.rels",
			"ppt/slides/slide1.xml",
			"ppt/slides/slide2.xml",
			"ppt/slides/slide3.xml",
		}
		for _, name := range expected {
			if !files[name] {
				t.Errorf("missing file in pptx: %s", name)
			}
		}
	})

	t.Run("no slides", func(t *testing.T) {
		_, err := s.Execute(context.Background(), map[string]any{
			"path":    filepath.Join(dir, "empty.pptx"),
			"content": "",
		}, &skills.Environment{})
		if err == nil {
			t.Fatal("expected error for empty content")
		}
	})

	t.Run("access denied", func(t *testing.T) {
		_, err := s.Execute(context.Background(), map[string]any{
			"path":    "/etc/test.pptx",
			"content": "# Slide\ncontent",
		}, &skills.Environment{})
		if err == nil {
			t.Fatal("expected access denied error")
		}
	})
}

func TestParseSlides(t *testing.T) {
	slides := parseSlides("# Title\nBody text\n---\nSlide 2\nMore content\n---\n# Final\nEnd")
	if len(slides) != 3 {
		t.Fatalf("expected 3 slides, got %d", len(slides))
	}
	if slides[0].Title != "Title" {
		t.Errorf("expected title 'Title', got '%s'", slides[0].Title)
	}
	if slides[0].Body != "Body text" {
		t.Errorf("expected body 'Body text', got '%s'", slides[0].Body)
	}
	if slides[1].Title != "Slide 2" {
		t.Errorf("expected title 'Slide 2', got '%s'", slides[1].Title)
	}
}

func TestRenderMarkdownToHTML(t *testing.T) {
	html := renderMarkdownToHTML("Test", "# Hello\n\nWorld\n\n- A\n- B")
	if !strings.Contains(html, "<h1>Hello</h1>") {
		t.Error("missing h1")
	}
	if !strings.Contains(html, "<p>World</p>") {
		t.Error("missing paragraph")
	}
	if !strings.Contains(html, "<li>A</li>") {
		t.Error("missing list item")
	}
}
