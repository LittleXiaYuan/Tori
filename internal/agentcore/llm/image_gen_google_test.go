package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeGoogleBaseURL(t *testing.T) {
	cases := map[string]string{
		"https://generativelanguage.googleapis.com/v1beta/openai":  "https://generativelanguage.googleapis.com/v1beta",
		"https://generativelanguage.googleapis.com/v1beta/openai/": "https://generativelanguage.googleapis.com/v1beta",
		"https://generativelanguage.googleapis.com/v1beta":         "https://generativelanguage.googleapis.com/v1beta",
		"":                                                         "https://generativelanguage.googleapis.com/v1beta",
	}
	for in, want := range cases {
		if got := normalizeGoogleBaseURL(in); got != want {
			t.Errorf("normalizeGoogleBaseURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGeminiImagenGenGenerateImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "imagen-4.0-generate-001:predict") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		instances, _ := body["instances"].([]any)
		if len(instances) != 1 {
			t.Fatalf("expected 1 instance, got %d", len(instances))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"predictions": []map[string]any{
				{"bytesBase64Encoded": "Zm9v", "mimeType": "image/png"},
			},
		})
	}))
	defer srv.Close()

	gen := NewGeminiImagenGen(srv.URL, "test-key", "")
	resp, err := gen.GenerateImage(context.Background(), ImageGenerateRequest{Prompt: "a cat"})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if len(resp.Artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(resp.Artifacts))
	}
	if resp.Artifacts[0].B64JSON != "Zm9v" {
		t.Errorf("unexpected b64 data: %s", resp.Artifacts[0].B64JSON)
	}
	if resp.Artifacts[0].Provider != "google" {
		t.Errorf("expected provider google, got %s", resp.Artifacts[0].Provider)
	}
}

func TestGeminiFlashImageGenGenerateImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "gemini-2.5-flash-image:generateContent") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{"text": "here is your cat"},
							{"inlineData": map[string]any{"mimeType": "image/png", "data": "Ym9vgA=="}},
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	gen := NewGeminiFlashImageGen(srv.URL, "test-key", "")
	resp, err := gen.GenerateImage(context.Background(), ImageGenerateRequest{Prompt: "a cat wearing sunglasses"})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if len(resp.Artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(resp.Artifacts))
	}
	if resp.Artifacts[0].RevisedPrompt != "here is your cat" {
		t.Errorf("expected text part attached as revised prompt, got %q", resp.Artifacts[0].RevisedPrompt)
	}
}

func TestGeminiFlashImageGenEditModeSendsInputImageBeforeText(t *testing.T) {
	var capturedParts []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Contents []struct {
				Parts []map[string]any `json:"parts"`
			} `json:"contents"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if len(body.Contents) != 1 {
			t.Fatalf("expected 1 content block, got %d", len(body.Contents))
		}
		capturedParts = body.Contents[0].Parts
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{
				{"content": map[string]any{"parts": []map[string]any{
					{"inlineData": map[string]any{"mimeType": "image/png", "data": "ZWRpdGVk"}},
				}}},
			},
		})
	}))
	defer srv.Close()

	gen := NewGeminiFlashImageGen(srv.URL, "test-key", "")
	_, err := gen.GenerateImage(context.Background(), ImageGenerateRequest{
		Prompt:      "把背景换成雪山",
		InputImages: []ImageInput{{Data: []byte("fake-source-png"), MimeType: "image/png"}},
	})
	if err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if len(capturedParts) != 2 {
		t.Fatalf("expected 2 parts (image then text), got %d: %#v", len(capturedParts), capturedParts)
	}
	if _, ok := capturedParts[0]["inlineData"]; !ok {
		t.Fatalf("expected first part to be the input image, got %#v", capturedParts[0])
	}
	if _, ok := capturedParts[1]["text"]; !ok {
		t.Fatalf("expected second part to be the text instruction, got %#v", capturedParts[1])
	}
}

func TestGeminiFlashImageGenNoImageReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{
				{"content": map[string]any{"parts": []map[string]any{{"text": "sorry, I can't generate that"}}}},
			},
		})
	}))
	defer srv.Close()

	gen := NewGeminiFlashImageGen(srv.URL, "test-key", "")
	_, err := gen.GenerateImage(context.Background(), ImageGenerateRequest{Prompt: "something refused"})
	if err == nil {
		t.Fatal("expected error when no image is returned")
	}
}

func TestCreateImageGenDispatch(t *testing.T) {
	cases := []struct {
		name  string
		cfg   ProviderConfig
		check func(t *testing.T, gen ImageGenerator)
	}{
		{
			name: "dashscope goes to qwen",
			cfg:  ProviderConfig{BaseURL: "https://dashscope.aliyuncs.com", Model: "qwen-image"},
			check: func(t *testing.T, gen ImageGenerator) {
				if _, ok := gen.(*QwenImageGen); !ok {
					t.Errorf("expected *QwenImageGen, got %T", gen)
				}
			},
		},
		{
			name: "google imagen model goes to imagen adapter",
			cfg:  ProviderConfig{BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai", Model: "imagen-4.0-generate-001"},
			check: func(t *testing.T, gen ImageGenerator) {
				if _, ok := gen.(*GeminiImagenGen); !ok {
					t.Errorf("expected *GeminiImagenGen, got %T", gen)
				}
			},
		},
		{
			name: "google gemini image model goes to flash image adapter",
			cfg:  ProviderConfig{BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai", Model: "gemini-2.5-flash-image"},
			check: func(t *testing.T, gen ImageGenerator) {
				if _, ok := gen.(*GeminiFlashImageGen); !ok {
					t.Errorf("expected *GeminiFlashImageGen, got %T", gen)
				}
			},
		},
		{
			name: "default goes to openai",
			cfg:  ProviderConfig{BaseURL: "https://api.openai.com/v1", Model: "gpt-image-1"},
			check: func(t *testing.T, gen ImageGenerator) {
				if _, ok := gen.(*OpenAIImageGen); !ok {
					t.Errorf("expected *OpenAIImageGen, got %T", gen)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, createImageGen(tc.cfg))
		})
	}
}

func TestGetImageGeneratorRespectsPin(t *testing.T) {
	reg := NewProviderRegistry(nil)
	if err := reg.Register(ProviderConfig{
		ID: "openai-img", Type: ProviderTypeChat, BaseURL: "https://api.openai.com/v1",
		Model: "gpt-image-1", Enabled: true, Capabilities: []Capability{CapImageGen},
	}); err != nil {
		t.Fatalf("register openai-img: %v", err)
	}
	if err := reg.Register(ProviderConfig{
		ID: "gemini-img", Type: ProviderTypeChat, BaseURL: "https://generativelanguage.googleapis.com/v1beta",
		Model: "gemini-2.5-flash-image", Enabled: true, Capabilities: []Capability{CapImageGen},
	}); err != nil {
		t.Fatalf("register gemini-img: %v", err)
	}

	reg.SetImageGenProvider("gemini-img")
	if _, ok := reg.GetImageGenerator().(*GeminiFlashImageGen); !ok {
		t.Fatalf("expected pinned gemini-img to win")
	}

	// Pinning a disabled/unknown provider falls back to auto-select rather
	// than returning nil — a stale pin (e.g. provider deleted) must not
	// silently break image generation.
	reg.SetImageGenProvider("does-not-exist")
	if reg.GetImageGenerator() == nil {
		t.Fatalf("expected fallback to auto-select when pinned provider is unavailable")
	}

	reg.SetImageGenProvider("")
	if reg.GetImageGenerator() == nil {
		t.Fatalf("expected auto-select to find a CapImageGen provider when unpinned")
	}

	providers := reg.ImageGenCapableProviders()
	if len(providers) != 2 {
		t.Fatalf("expected 2 image-gen-capable providers, got %d", len(providers))
	}
}
