package planner

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParseSkillPackage_SingleFile(t *testing.T) {
	reply := `SKILL_NAME: weather-api
SKILL_DISPLAY: Weather API
SKILL_DESC: Query weather data from OpenWeatherMap
---SKILL_CONTENT---
# Weather API Skill

Use the OpenWeatherMap API to get current weather.

## Steps
1. Call GET https://api.openweathermap.org/data/2.5/weather?q={city}&appid={key}
2. Parse the JSON response
3. Return temperature and conditions
---END_SKILL---`

	pkg, err := parseSkillPackage(reply)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Slug != "weather-api" {
		t.Errorf("slug = %q, want %q", pkg.Slug, "weather-api")
	}
	if pkg.Name != "Weather API" {
		t.Errorf("name = %q, want %q", pkg.Name, "Weather API")
	}
	if pkg.Desc != "Query weather data from OpenWeatherMap" {
		t.Errorf("desc = %q, want %q", pkg.Desc, "Query weather data from OpenWeatherMap")
	}
	if len(pkg.Files) != 1 {
		t.Fatalf("files = %d, want 1", len(pkg.Files))
	}
	if pkg.Files[0].Path != "SKILL.md" {
		t.Errorf("file path = %q, want %q", pkg.Files[0].Path, "SKILL.md")
	}
	if !strings.Contains(pkg.Files[0].Content, "OpenWeatherMap API") {
		t.Errorf("content missing expected text")
	}
}

func TestParseSkillPackage_MultiFile(t *testing.T) {
	reply := `SKILL_NAME: thesis-gen
SKILL_DISPLAY: Thesis Generator
SKILL_DESC: Generate graduation thesis from template
---FILE: SKILL.md---
# Thesis Generator

## Steps
1. Parse template
2. Generate content
3. Apply formatting
---END_FILE---
---FILE: scripts/generate.py---
#!/usr/bin/env python3
from docx import Document

def generate(template_path, content):
    doc = Document(template_path)
    return doc
---END_FILE---
---FILE: templates/meta.json---
{
  "title": "",
  "author": "",
  "advisor": ""
}
---END_FILE---`

	pkg, err := parseSkillPackage(reply)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Slug != "thesis-gen" {
		t.Errorf("slug = %q, want %q", pkg.Slug, "thesis-gen")
	}
	if len(pkg.Files) != 3 {
		t.Fatalf("files = %d, want 3", len(pkg.Files))
	}
	paths := []string{"SKILL.md", "scripts/generate.py", "templates/meta.json"}
	for i, want := range paths {
		if pkg.Files[i].Path != want {
			t.Errorf("file[%d] path = %q, want %q", i, pkg.Files[i].Path, want)
		}
	}
	if !strings.Contains(pkg.Files[1].Content, "from docx import") {
		t.Error("script content should contain python imports")
	}
}

func TestParseSkillPackage_MissingSlug(t *testing.T) {
	reply := `SKILL_DISPLAY: Test
SKILL_DESC: A test skill
---SKILL_CONTENT---
some content
---END_SKILL---`
	_, err := parseSkillPackage(reply)
	if err == nil {
		t.Fatal("expected error for missing slug")
	}
	if !strings.Contains(err.Error(), "incomplete") {
		t.Errorf("error = %v, want 'incomplete'", err)
	}
}

func TestParseSkillPackage_MissingContent(t *testing.T) {
	reply := `SKILL_NAME: test
SKILL_DISPLAY: Test
SKILL_DESC: A test`
	_, err := parseSkillPackage(reply)
	if err == nil {
		t.Fatal("expected error for missing content markers")
	}
}

func TestParseSkillPackage_DefaultDesc(t *testing.T) {
	reply := `SKILL_NAME: test-skill
SKILL_DISPLAY: My Skill
---SKILL_CONTENT---
content here
---END_SKILL---`
	pkg, err := parseSkillPackage(reply)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Desc != "My Skill" {
		t.Errorf("desc should default to name, got %q", pkg.Desc)
	}
}

func TestSkillGenerator_Generate_Success(t *testing.T) {
	gen := NewSkillGenerator(10 * time.Second)

	gen.SetWebSearch(func(ctx context.Context, query string, limit int) ([]WebSearchResult, error) {
		return []WebSearchResult{
			{Title: "Official Docs", URL: "https://example.com/docs", Snippet: "API reference for XYZ service"},
		}, nil
	})

	gen.SetLLMCall(func(ctx context.Context, system, user string) (string, error) {
		if !strings.Contains(system, "skill generator") {
			t.Error("system prompt should contain 'skill generator'")
		}
		if !strings.Contains(user, "Official Docs") {
			t.Error("user prompt should include search results")
		}
		return `SKILL_NAME: xyz-api
SKILL_DISPLAY: XYZ Service
SKILL_DESC: Interact with XYZ service
---FILE: SKILL.md---
# XYZ Service Skill
Call the XYZ API endpoint.
---END_FILE---`, nil
	})

	var registered bool
	gen.SetRegister(func(slug, name, description, content string) (string, error) {
		registered = true
		if slug != "xyz-api" {
			t.Errorf("slug = %q, want %q", slug, "xyz-api")
		}
		if name != "XYZ Service" {
			t.Errorf("name = %q, want %q", name, "XYZ Service")
		}
		return name, nil
	})

	name, err := gen.Generate(context.Background(), "xyz service integration", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "XYZ Service" {
		t.Errorf("returned name = %q, want %q", name, "XYZ Service")
	}
	if !registered {
		t.Error("register callback was not called")
	}
}

func TestSkillGenerator_Generate_PackageRegister(t *testing.T) {
	gen := NewSkillGenerator(10 * time.Second)

	gen.SetWebSearch(func(ctx context.Context, query string, limit int) ([]WebSearchResult, error) {
		return []WebSearchResult{{Title: "Doc", URL: "https://example.com", Snippet: "api"}}, nil
	})
	gen.SetLLMCall(func(ctx context.Context, system, user string) (string, error) {
		return `SKILL_NAME: multi
SKILL_DISPLAY: Multi File
SKILL_DESC: A multi-file skill
---FILE: SKILL.md---
# Main
---END_FILE---
---FILE: scripts/run.sh---
#!/bin/bash
echo "hello"
---END_FILE---`, nil
	})

	var pkgFiles []SkillFile
	gen.SetRegisterPackage(func(slug, name, description string, files []SkillFile) (string, error) {
		pkgFiles = files
		return name, nil
	})
	gen.SetRegister(func(slug, name, description, content string) (string, error) {
		t.Fatal("single register should not be called when package register is set")
		return "", nil
	})

	name, err := gen.Generate(context.Background(), "multi-file skill", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Multi File" {
		t.Errorf("name = %q, want %q", name, "Multi File")
	}
	if len(pkgFiles) != 2 {
		t.Errorf("files = %d, want 2", len(pkgFiles))
	}
}

func TestSkillGenerator_Generate_InsufficientSources(t *testing.T) {
	gen := NewSkillGenerator(10 * time.Second)

	gen.SetWebSearch(func(ctx context.Context, query string, limit int) ([]WebSearchResult, error) {
		return []WebSearchResult{{Title: "Irrelevant", URL: "https://example.com", Snippet: "nothing useful"}}, nil
	})
	gen.SetLLMCall(func(ctx context.Context, system, user string) (string, error) {
		return "INSUFFICIENT_SOURCES", nil
	})
	gen.SetRegister(func(slug, name, description, content string) (string, error) {
		t.Fatal("register should not be called for insufficient sources")
		return "", nil
	})

	_, err := gen.Generate(context.Background(), "obscure-tool", "")
	if err == nil {
		t.Fatal("expected error for insufficient sources")
	}
	if !strings.Contains(err.Error(), "insufficient") {
		t.Errorf("error = %v, want 'insufficient'", err)
	}
}

func TestSkillGenerator_Generate_NoSearchResults(t *testing.T) {
	gen := NewSkillGenerator(10 * time.Second)

	gen.SetWebSearch(func(ctx context.Context, query string, limit int) ([]WebSearchResult, error) {
		return nil, nil
	})
	gen.SetLLMCall(func(ctx context.Context, system, user string) (string, error) {
		t.Fatal("LLM should not be called when no search results")
		return "", nil
	})
	gen.SetRegister(func(slug, name, description, content string) (string, error) {
		t.Fatal("register should not be called")
		return "", nil
	})

	_, err := gen.Generate(context.Background(), "nonexistent", "")
	if err == nil {
		t.Fatal("expected error for no search results")
	}
	if !strings.Contains(err.Error(), "no web results") {
		t.Errorf("error = %v, want 'no web results'", err)
	}
}

func TestSkillGenerator_Generate_WebSearchError(t *testing.T) {
	gen := NewSkillGenerator(10 * time.Second)

	gen.SetWebSearch(func(ctx context.Context, query string, limit int) ([]WebSearchResult, error) {
		return nil, fmt.Errorf("network timeout")
	})
	gen.SetLLMCall(func(ctx context.Context, system, user string) (string, error) {
		t.Fatal("LLM should not be called on search error")
		return "", nil
	})
	gen.SetRegister(func(slug, name, description, content string) (string, error) { return "", nil })

	_, err := gen.Generate(context.Background(), "test", "")
	if err == nil || !strings.Contains(err.Error(), "web search failed") {
		t.Fatalf("error = %v, want 'web search failed'", err)
	}
}

func TestSkillGenerator_Generate_LLMError(t *testing.T) {
	gen := NewSkillGenerator(10 * time.Second)

	gen.SetWebSearch(func(ctx context.Context, query string, limit int) ([]WebSearchResult, error) {
		return []WebSearchResult{{Title: "Doc", URL: "https://example.com", Snippet: "content"}}, nil
	})
	gen.SetLLMCall(func(ctx context.Context, system, user string) (string, error) {
		return "", fmt.Errorf("rate limited")
	})
	gen.SetRegister(func(slug, name, description, content string) (string, error) { return "", nil })

	_, err := gen.Generate(context.Background(), "test", "")
	if err == nil || !strings.Contains(err.Error(), "LLM generation failed") {
		t.Fatalf("error = %v, want 'LLM generation failed'", err)
	}
}

func TestSkillGenerator_Generate_NotConfigured(t *testing.T) {
	gen := NewSkillGenerator(10 * time.Second)
	_, err := gen.Generate(context.Background(), "test", "")
	if err == nil || !strings.Contains(err.Error(), "not fully configured") {
		t.Fatalf("error = %v, want 'not fully configured'", err)
	}
}

func TestSkillGenerator_Ready(t *testing.T) {
	gen := NewSkillGenerator(10 * time.Second)
	if gen.Ready() {
		t.Fatal("should not be ready with no functions set")
	}
	gen.SetWebSearch(func(ctx context.Context, query string, limit int) ([]WebSearchResult, error) { return nil, nil })
	if gen.Ready() {
		t.Fatal("should not be ready with only webSearch")
	}
	gen.SetLLMCall(func(ctx context.Context, system, user string) (string, error) { return "", nil })
	if gen.Ready() {
		t.Fatal("should not be ready without register")
	}
	gen.SetRegister(func(slug, name, description, content string) (string, error) { return "", nil })
	if !gen.Ready() {
		t.Fatal("should be ready with all functions set")
	}
}

func TestSkillGenerator_Ready_PackageOnly(t *testing.T) {
	gen := NewSkillGenerator(10 * time.Second)
	gen.SetWebSearch(func(ctx context.Context, query string, limit int) ([]WebSearchResult, error) { return nil, nil })
	gen.SetLLMCall(func(ctx context.Context, system, user string) (string, error) { return "", nil })
	gen.SetRegisterPackage(func(slug, name, desc string, files []SkillFile) (string, error) { return "", nil })
	if !gen.Ready() {
		t.Fatal("should be ready with webSearch + llmCall + registerPackage")
	}
}

func TestSkillGenerator_Generate_MalformedLLMOutput(t *testing.T) {
	gen := NewSkillGenerator(10 * time.Second)

	gen.SetWebSearch(func(ctx context.Context, query string, limit int) ([]WebSearchResult, error) {
		return []WebSearchResult{{Title: "Doc", URL: "https://example.com", Snippet: "api docs"}}, nil
	})
	gen.SetLLMCall(func(ctx context.Context, system, user string) (string, error) {
		return "Here is some random text without proper markers.", nil
	})
	gen.SetRegister(func(slug, name, description, content string) (string, error) { return name, nil })

	_, err := gen.Generate(context.Background(), "random-capability", "")
	if err == nil {
		t.Fatal("expected error for malformed LLM output")
	}
}

func TestSkillGenerator_Generate_RegisterError(t *testing.T) {
	gen := NewSkillGenerator(10 * time.Second)

	gen.SetWebSearch(func(ctx context.Context, query string, limit int) ([]WebSearchResult, error) {
		return []WebSearchResult{{Title: "Doc", URL: "https://example.com", Snippet: "content"}}, nil
	})
	gen.SetLLMCall(func(ctx context.Context, system, user string) (string, error) {
		return `SKILL_NAME: test-skill
SKILL_DISPLAY: Test
SKILL_DESC: A test
---SKILL_CONTENT---
content
---END_SKILL---`, nil
	})
	gen.SetRegister(func(slug, name, description, content string) (string, error) {
		return "", fmt.Errorf("registry full")
	})

	_, err := gen.Generate(context.Background(), "test", "")
	if err == nil || !strings.Contains(err.Error(), "register") {
		t.Fatalf("error = %v, want register error", err)
	}
}

func TestSkillGenerator_Generate_FailureContextPassedToLLM(t *testing.T) {
	gen := NewSkillGenerator(10 * time.Second)

	gen.SetWebSearch(func(ctx context.Context, query string, limit int) ([]WebSearchResult, error) {
		return []WebSearchResult{{Title: "Doc", URL: "https://example.com", Snippet: "content"}}, nil
	})

	var capturedUserPrompt string
	gen.SetLLMCall(func(ctx context.Context, system, user string) (string, error) {
		capturedUserPrompt = user
		return "INSUFFICIENT_SOURCES", nil
	})
	gen.SetRegister(func(slug, name, description, content string) (string, error) { return name, nil })

	gen.Generate(context.Background(), "data-transform", "tool 'jq' not found in PATH")

	if !strings.Contains(capturedUserPrompt, "jq") {
		t.Error("failure context should be passed to LLM prompt")
	}
	if !strings.Contains(capturedUserPrompt, "data-transform") {
		t.Error("capability description should be in LLM prompt")
	}
}
