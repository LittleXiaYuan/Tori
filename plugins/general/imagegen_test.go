package general

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"yunque-agent/pkg/skills"
)

func TestImageGenSkill_Metadata(t *testing.T) {
	s := NewImageGenSkill()
	if s.Name() != "image_gen" {
		t.Fatalf("expected name 'image_gen', got %q", s.Name())
	}
	params := s.Parameters()
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties map")
	}
	for _, key := range []string{"prompt", "size", "quality", "style", "n"} {
		if _, ok := props[key]; !ok {
			t.Fatalf("missing parameter %q", key)
		}
	}
}

func TestImageGenSkill_EmptyPrompt(t *testing.T) {
	s := NewImageGenSkill()
	env := &skills.Environment{}
	_, err := s.Execute(context.Background(), map[string]any{
		"prompt": "",
	}, env)
	if err == nil || !strings.Contains(err.Error(), "prompt is required") {
		t.Fatalf("expected prompt required error, got %v", err)
	}
}

func TestImageGenSkill_PromptTooLong(t *testing.T) {
	s := NewImageGenSkill()
	env := &skills.Environment{}
	_, err := s.Execute(context.Background(), map[string]any{
		"prompt": strings.Repeat("a", 4001),
	}, env)
	if err == nil || !strings.Contains(err.Error(), "too long") {
		t.Fatalf("expected too long error, got %v", err)
	}
}

func TestImageGenSkill_InvalidSize(t *testing.T) {
	s := NewImageGenSkill()
	os.Setenv("IMAGEGEN_API_KEY", "test-key")
	defer os.Unsetenv("IMAGEGEN_API_KEY")
	env := &skills.Environment{}
	_, err := s.Execute(context.Background(), map[string]any{
		"prompt": "a cat",
		"size":   "999x999",
	}, env)
	if err == nil || !strings.Contains(err.Error(), "invalid size") {
		t.Fatalf("expected invalid size error, got %v", err)
	}
}

func TestImageGenSkill_InvalidQuality(t *testing.T) {
	s := NewImageGenSkill()
	os.Setenv("IMAGEGEN_API_KEY", "test-key")
	defer os.Unsetenv("IMAGEGEN_API_KEY")
	env := &skills.Environment{}
	_, err := s.Execute(context.Background(), map[string]any{
		"prompt":  "a cat",
		"quality": "ultra",
	}, env)
	if err == nil || !strings.Contains(err.Error(), "invalid quality") {
		t.Fatalf("expected invalid quality error, got %v", err)
	}
}

func TestImageGenSkill_InvalidStyle(t *testing.T) {
	s := NewImageGenSkill()
	os.Setenv("IMAGEGEN_API_KEY", "test-key")
	defer os.Unsetenv("IMAGEGEN_API_KEY")
	env := &skills.Environment{}
	_, err := s.Execute(context.Background(), map[string]any{
		"prompt": "a cat",
		"style":  "abstract",
	}, env)
	if err == nil || !strings.Contains(err.Error(), "invalid style") {
		t.Fatalf("expected invalid style error, got %v", err)
	}
}

func TestImageGenSkill_NTooLarge(t *testing.T) {
	s := NewImageGenSkill()
	os.Setenv("IMAGEGEN_API_KEY", "test-key")
	defer os.Unsetenv("IMAGEGEN_API_KEY")
	env := &skills.Environment{}
	_, err := s.Execute(context.Background(), map[string]any{
		"prompt": "a cat",
		"n":      float64(5),
	}, env)
	if err == nil || !strings.Contains(err.Error(), "between 1 and 4") {
		t.Fatalf("expected n error, got %v", err)
	}
}

func TestImageGenSkill_NoAPIKey(t *testing.T) {
	s := NewImageGenSkill()
	os.Unsetenv("IMAGEGEN_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	env := &skills.Environment{}
	_, err := s.Execute(context.Background(), map[string]any{
		"prompt": "a cat",
	}, env)
	if err == nil || !strings.Contains(err.Error(), "environment variable") {
		t.Fatalf("expected API key error, got %v", err)
	}
}

func TestImageGenSkill_SuccessfulCall(t *testing.T) {
	// Mock API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key-123" {
			t.Error("expected auth header")
		}
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		if req["prompt"] != "a cute cat" {
			t.Errorf("unexpected prompt: %v", req["prompt"])
		}

		resp := imageAPIResponse{
			Created: 1234567890,
			Data: []struct {
				URL           string `json:"url"`
				RevisedPrompt string `json:"revised_prompt"`
				B64JSON       string `json:"b64_json"`
			}{
				{URL: "https://example.com/image.png", RevisedPrompt: "A cute fluffy cat"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	os.Setenv("IMAGEGEN_API_URL", server.URL)
	os.Setenv("IMAGEGEN_API_KEY", "test-key-123")
	defer os.Unsetenv("IMAGEGEN_API_URL")
	defer os.Unsetenv("IMAGEGEN_API_KEY")

	s := NewImageGenSkill()
	env := &skills.Environment{}
	result, err := s.Execute(context.Background(), map[string]any{
		"prompt": "a cute cat",
	}, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "https://example.com/image.png") {
		t.Fatalf("expected image URL in result, got %q", result)
	}
	if !strings.Contains(result, "A cute fluffy cat") {
		t.Fatalf("expected revised prompt in result, got %q", result)
	}
}

func TestImageGenSkill_MultipleImages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := imageAPIResponse{
			Created: 1234567890,
			Data: []struct {
				URL           string `json:"url"`
				RevisedPrompt string `json:"revised_prompt"`
				B64JSON       string `json:"b64_json"`
			}{
				{URL: "https://example.com/img1.png"},
				{URL: "https://example.com/img2.png"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	os.Setenv("IMAGEGEN_API_URL", server.URL)
	os.Setenv("IMAGEGEN_API_KEY", "key")
	defer os.Unsetenv("IMAGEGEN_API_URL")
	defer os.Unsetenv("IMAGEGEN_API_KEY")

	s := NewImageGenSkill()
	env := &skills.Environment{}
	result, err := s.Execute(context.Background(), map[string]any{
		"prompt": "cats",
		"n":      float64(2),
	}, env)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "图片 1") || !strings.Contains(result, "图片 2") {
		t.Fatalf("expected numbered images, got %q", result)
	}
}

func TestImageGenSkill_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "billing limit reached",
				"code":    "billing_hard_limit_reached",
			},
		})
	}))
	defer server.Close()

	os.Setenv("IMAGEGEN_API_URL", server.URL)
	os.Setenv("IMAGEGEN_API_KEY", "key")
	defer os.Unsetenv("IMAGEGEN_API_URL")
	defer os.Unsetenv("IMAGEGEN_API_KEY")

	s := NewImageGenSkill()
	env := &skills.Environment{}
	_, err := s.Execute(context.Background(), map[string]any{
		"prompt": "cats",
	}, env)
	if err == nil || !strings.Contains(err.Error(), "billing limit") {
		t.Fatalf("expected API error, got %v", err)
	}
}

func TestImageGenSkill_ChineseAutoTranslate(t *testing.T) {
	var capturedPrompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		capturedPrompt = req["prompt"].(string)
		resp := imageAPIResponse{
			Data: []struct {
				URL           string `json:"url"`
				RevisedPrompt string `json:"revised_prompt"`
				B64JSON       string `json:"b64_json"`
			}{
				{URL: "https://example.com/img.png"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	os.Setenv("IMAGEGEN_API_URL", server.URL)
	os.Setenv("IMAGEGEN_API_KEY", "key")
	defer os.Unsetenv("IMAGEGEN_API_URL")
	defer os.Unsetenv("IMAGEGEN_API_KEY")

	s := NewImageGenSkill()
	env := &skills.Environment{
		LLMCall: func(ctx context.Context, system, user string) (string, error) {
			return "a cute cat sitting on a sofa", nil
		},
	}
	_, err := s.Execute(context.Background(), map[string]any{
		"prompt": "一只可爱的猫坐在沙发上",
	}, env)
	if err != nil {
		t.Fatal(err)
	}
	if capturedPrompt != "a cute cat sitting on a sofa" {
		t.Fatalf("expected translated prompt, got %q", capturedPrompt)
	}
}

func TestContainsChinese(t *testing.T) {
	if !containsChinese("你好") {
		t.Fatal("expected true for Chinese")
	}
	if containsChinese("hello world") {
		t.Fatal("expected false for English")
	}
	if !containsChinese("hello 世界") {
		t.Fatal("expected true for mixed")
	}
}
