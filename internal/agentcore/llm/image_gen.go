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
	Size        string `json:"size,omitempty"`        // e.g. "1024x1024"
	N           int    `json:"n,omitempty"`            // number of images (default 1)
	ResponseFmt string `json:"response_format,omitempty"` // "url" or "b64_json"
	Quality     string `json:"quality,omitempty"`      // "standard", "hd"
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

// ── Provider Registry extension for image generation ──

// GetImageGenerator returns an ImageGenerator for a provider that has CapImageGen.
func (r *ProviderRegistry) GetImageGenerator() ImageGenerator {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.providers {
		if !p.Enabled() {
			continue
		}
		for _, cap := range p.Config.Capabilities {
			if cap == CapImageGen {
				return createImageGen(p.Config)
			}
		}
	}
	return nil
}

func createImageGen(cfg ProviderConfig) ImageGenerator {
	key := ""
	if len(cfg.APIKeys) > 0 {
		key = cfg.APIKeys[0]
	}
	switch {
	case strings.Contains(cfg.BaseURL, "dashscope"):
		return NewQwenImageGen(cfg.BaseURL, key, cfg.Model)
	default:
		return NewOpenAIImageGen(cfg.BaseURL, key, cfg.Model)
	}
}
