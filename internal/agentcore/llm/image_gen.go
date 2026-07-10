package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// ImageGenerateRequest is the unified request for all image generation providers.
type ImageGenerateRequest struct {
	Prompt      string `json:"prompt"`
	Negative    string `json:"negative_prompt,omitempty"`
	Size        string `json:"size,omitempty"`            // e.g. "1024x1024"
	N           int    `json:"n,omitempty"`               // number of images (default 1)
	ResponseFmt string `json:"response_format,omitempty"` // "url" or "b64_json"
	Quality     string `json:"quality,omitempty"`         // "standard", "hd"

	// InputImages carries reference image(s) for edit-mode requests (e.g.
	// "put a hat on this cat", "change the background to snow"). Providers
	// without native image-editing support ignore this and fall back to
	// plain text-to-image.
	InputImages []ImageInput `json:"input_images,omitempty"`
}

// ImageInput is a reference image supplied alongside a prompt for editing.
type ImageInput struct {
	Data     []byte `json:"-"`                   // raw image bytes
	MimeType string `json:"mime_type,omitempty"` // e.g. "image/png"
}

// ImageArtifact is a single generated image.
type ImageArtifact struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"b64_json,omitempty"`
	MimeType      string `json:"mime_type,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
	Provider      string `json:"provider,omitempty"`
	Model         string `json:"model,omitempty"`
}

// ImageData returns the raw image bytes (from URL or base64).
func (a *ImageArtifact) ImageData(ctx context.Context) ([]byte, error) {
	if a.B64JSON != "" {
		return base64.StdEncoding.DecodeString(a.B64JSON)
	}
	if a.URL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		return io.ReadAll(resp.Body)
	}
	return nil, fmt.Errorf("no image data")
}

// ImageGenerateResponse is the unified response.
type ImageGenerateResponse struct {
	Artifacts []ImageArtifact `json:"artifacts"`
}

// ImageGenerator is the interface for image generation backends.
type ImageGenerator interface {
	GenerateImage(ctx context.Context, req ImageGenerateRequest) (*ImageGenerateResponse, error)
}

// ── OpenAI Images API adapter ──

type OpenAIImageGen struct {
	BaseURL string
	APIKey  string
	Model   string // default "gpt-image-1"
	HTTP    *http.Client
}

func NewOpenAIImageGen(baseURL, apiKey, model string) *OpenAIImageGen {
	if model == "" {
		model = "gpt-image-1"
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAIImageGen{
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		APIKey:  apiKey,
		Model:   model,
		HTTP:    &http.Client{Timeout: 120 * time.Second},
	}
}

func (g *OpenAIImageGen) GenerateImage(ctx context.Context, req ImageGenerateRequest) (*ImageGenerateResponse, error) {
	if req.Size == "" {
		req.Size = "1024x1024"
	}
	if req.N <= 0 {
		req.N = 1
	}
	if req.ResponseFmt == "" {
		req.ResponseFmt = "b64_json"
	}

	payload := map[string]any{
		"model":           g.Model,
		"prompt":          req.Prompt,
		"size":            req.Size,
		"n":               req.N,
		"response_format": req.ResponseFmt,
	}
	if req.Quality != "" {
		payload["quality"] = req.Quality
	}

	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.BaseURL+"/images/generations", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.APIKey)

	resp, err := g.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai image gen: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai image gen %d: %.500s", resp.StatusCode, string(respBody))
	}

	var apiResp struct {
		Data []struct {
			URL           string `json:"url"`
			B64JSON       string `json:"b64_json"`
			RevisedPrompt string `json:"revised_prompt"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("openai image gen decode: %w", err)
	}

	result := &ImageGenerateResponse{}
	for _, d := range apiResp.Data {
		result.Artifacts = append(result.Artifacts, ImageArtifact{
			URL:           d.URL,
			B64JSON:       d.B64JSON,
			MimeType:      "image/png",
			RevisedPrompt: d.RevisedPrompt,
			Provider:      "openai",
			Model:         g.Model,
		})
	}

	slog.Info("openai image gen", "model", g.Model, "images", len(result.Artifacts))
	return result, nil
}

// ── Qwen Image API adapter ──

type QwenImageGen struct {
	BaseURL string // e.g. "https://dashscope.aliyuncs.com"
	APIKey  string
	Model   string // default "qwen-image-2.0"
	HTTP    *http.Client
}

func NewQwenImageGen(baseURL, apiKey, model string) *QwenImageGen {
	if model == "" {
		model = "qwen-image-2.0"
	}
	if baseURL == "" {
		baseURL = "https://dashscope.aliyuncs.com"
	}
	return &QwenImageGen{
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		APIKey:  apiKey,
		Model:   model,
		HTTP:    &http.Client{Timeout: 120 * time.Second},
	}
}

func (g *QwenImageGen) GenerateImage(ctx context.Context, req ImageGenerateRequest) (*ImageGenerateResponse, error) {
	if req.Size == "" {
		req.Size = "1024*1024"
	}
	if req.N <= 0 {
		req.N = 1
	}

	payload := map[string]any{
		"model": g.Model,
		"input": map[string]any{
			"prompt": req.Prompt,
		},
		"parameters": map[string]any{
			"size": req.Size,
			"n":    req.N,
		},
	}
	if req.Negative != "" {
		payload["input"].(map[string]any)["negative_prompt"] = req.Negative
	}

	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		g.BaseURL+"/api/v1/services/aigc/multimodal-generation/generation",
		bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.APIKey)
	httpReq.Header.Set("X-DashScope-Async", "enable")

	resp, err := g.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("qwen image gen: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("qwen image gen %d: %.500s", resp.StatusCode, string(respBody))
	}

	var apiResp struct {
		Output struct {
			Results []struct {
				URL string `json:"url"`
			} `json:"results"`
		} `json:"output"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("qwen image gen decode: %w", err)
	}

	result := &ImageGenerateResponse{}
	for _, r := range apiResp.Output.Results {
		result.Artifacts = append(result.Artifacts, ImageArtifact{
			URL:      r.URL,
			MimeType: "image/png",
			Provider: "qwen",
			Model:    g.Model,
		})
	}

	slog.Info("qwen image gen", "model", g.Model, "images", len(result.Artifacts))
	return result, nil
}

// ── Google Imagen adapter (predict API, text-to-image only) ──

type GeminiImagenGen struct {
	BaseURL string
	APIKey  string
	Model   string // default "imagen-4.0-generate-001"
	HTTP    *http.Client
}

func NewGeminiImagenGen(baseURL, apiKey, model string) *GeminiImagenGen {
	if model == "" {
		model = "imagen-4.0-generate-001"
	}
	return &GeminiImagenGen{
		BaseURL: normalizeGoogleBaseURL(baseURL),
		APIKey:  apiKey,
		Model:   model,
		HTTP:    &http.Client{Timeout: 120 * time.Second},
	}
}

func (g *GeminiImagenGen) GenerateImage(ctx context.Context, req ImageGenerateRequest) (*ImageGenerateResponse, error) {
	if req.N <= 0 {
		req.N = 1
	}
	instance := map[string]any{"prompt": req.Prompt}
	parameters := map[string]any{"sampleCount": req.N}
	if req.Negative != "" {
		parameters["negativePrompt"] = req.Negative
	}
	payload := map[string]any{
		"instances":  []any{instance},
		"parameters": parameters,
	}

	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/models/%s:predict?key=%s", g.BaseURL, g.Model, g.APIKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("imagen gen: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("imagen gen %d: %.500s", resp.StatusCode, string(respBody))
	}

	var apiResp struct {
		Predictions []struct {
			BytesBase64Encoded string `json:"bytesBase64Encoded"`
			MimeType           string `json:"mimeType"`
		} `json:"predictions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("imagen gen decode: %w", err)
	}

	result := &ImageGenerateResponse{}
	for _, p := range apiResp.Predictions {
		mime := p.MimeType
		if mime == "" {
			mime = "image/png"
		}
		result.Artifacts = append(result.Artifacts, ImageArtifact{
			B64JSON:  p.BytesBase64Encoded,
			MimeType: mime,
			Provider: "google",
			Model:    g.Model,
		})
	}

	slog.Info("imagen gen", "model", g.Model, "images", len(result.Artifacts))
	return result, nil
}

// ── Gemini Flash Image adapter ("nano-banana", generateContent multimodal API) ──

type GeminiFlashImageGen struct {
	BaseURL string
	APIKey  string
	Model   string // default "gemini-2.5-flash-image"
	HTTP    *http.Client
}

func NewGeminiFlashImageGen(baseURL, apiKey, model string) *GeminiFlashImageGen {
	if model == "" {
		model = "gemini-2.5-flash-image"
	}
	return &GeminiFlashImageGen{
		BaseURL: normalizeGoogleBaseURL(baseURL),
		APIKey:  apiKey,
		Model:   model,
		HTTP:    &http.Client{Timeout: 120 * time.Second},
	}
}

func (g *GeminiFlashImageGen) GenerateImage(ctx context.Context, req ImageGenerateRequest) (*ImageGenerateResponse, error) {
	prompt := req.Prompt
	if req.Negative != "" {
		prompt = fmt.Sprintf("%s\n(avoid: %s)", prompt, req.Negative)
	}
	// Reference images go first, then the instruction — this is the edit-mode
	// shape Gemini expects ("here's the image, here's what to change").
	// Plain text-to-image requests simply carry no image parts.
	var parts []any
	for _, in := range req.InputImages {
		if len(in.Data) == 0 {
			continue
		}
		mime := in.MimeType
		if mime == "" {
			mime = "image/png"
		}
		parts = append(parts, map[string]any{
			"inlineData": map[string]any{
				"mimeType": mime,
				"data":     base64.StdEncoding.EncodeToString(in.Data),
			},
		})
	}
	parts = append(parts, map[string]any{"text": prompt})
	payload := map[string]any{
		"contents": []any{
			map[string]any{"parts": parts},
		},
		"generationConfig": map[string]any{
			"responseModalities": []string{"TEXT", "IMAGE"},
		},
	}

	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", g.BaseURL, g.Model, g.APIKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.HTTP.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini image gen: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini image gen %d: %.500s", resp.StatusCode, string(respBody))
	}

	var apiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text       string `json:"text"`
					InlineData *struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("gemini image gen decode: %w", err)
	}

	result := &ImageGenerateResponse{}
	var revisedPrompt string
	for _, cand := range apiResp.Candidates {
		for _, part := range cand.Content.Parts {
			if part.InlineData != nil && part.InlineData.Data != "" {
				result.Artifacts = append(result.Artifacts, ImageArtifact{
					B64JSON:  part.InlineData.Data,
					MimeType: part.InlineData.MimeType,
					Provider: "google",
					Model:    g.Model,
				})
			} else if part.Text != "" {
				revisedPrompt = part.Text
			}
		}
	}
	if revisedPrompt != "" {
		for i := range result.Artifacts {
			result.Artifacts[i].RevisedPrompt = revisedPrompt
		}
	}
	if len(result.Artifacts) == 0 {
		return nil, fmt.Errorf("gemini image gen: no image returned (model may have refused the prompt)")
	}

	slog.Info("gemini flash image gen", "model", g.Model, "images", len(result.Artifacts))
	return result, nil
}

func normalizeGoogleBaseURL(base string) string {
	base = strings.TrimSuffix(strings.TrimSpace(base), "/")
	base = strings.TrimSuffix(base, "/openai")
	if base == "" {
		base = "https://generativelanguage.googleapis.com/v1beta"
	}
	return base
}

// ── Provider Registry extension for image generation ──

// GetImageGenerator returns an ImageGenerator for a provider that has CapImageGen.
// If a provider has been pinned via SetImageGenProvider, it is used when still
// enabled and capable; otherwise this falls back to the first enabled
// CapImageGen provider found (registration order, not priority-ordered).
func (r *ProviderRegistry) GetImageGenerator() ImageGenerator {
	if pinned := r.ImageGenProvider(); pinned != "" {
		if p := r.Get(pinned); p != nil && p.Enabled() && hasAllCapsSlice(p.Config.Capabilities, []Capability{CapImageGen}) {
			return createImageGen(p.Config)
		}
		slog.Warn("image_gen: pinned provider unavailable, falling back to auto-select", "provider", pinned)
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.providers {
		if !p.Enabled() {
			continue
		}
		if hasAllCapsSlice(p.Config.Capabilities, []Capability{CapImageGen}) {
			return createImageGen(p.Config)
		}
	}
	return nil
}

// ImageGenCapableProviders lists enabled providers carrying CapImageGen, for
// surfacing a selectable list in settings.
func (r *ProviderRegistry) ImageGenCapableProviders() []ProviderStatus {
	var out []ProviderStatus
	for _, p := range r.List() {
		if p.Enabled && hasAllCapsSlice(p.Capabilities, []Capability{CapImageGen}) {
			out = append(out, p)
		}
	}
	return out
}

func createImageGen(cfg ProviderConfig) ImageGenerator {
	key := ""
	if len(cfg.APIKeys) > 0 {
		key = cfg.APIKeys[0]
	}
	switch {
	case strings.Contains(cfg.BaseURL, "dashscope"):
		return NewQwenImageGen(cfg.BaseURL, key, cfg.Model)
	case strings.Contains(cfg.BaseURL, "generativelanguage.googleapis.com"):
		if strings.HasPrefix(cfg.Model, "imagen-") {
			return NewGeminiImagenGen(cfg.BaseURL, key, cfg.Model)
		}
		return NewGeminiFlashImageGen(cfg.BaseURL, key, cfg.Model)
	default:
		return NewOpenAIImageGen(cfg.BaseURL, key, cfg.Model)
	}
}
