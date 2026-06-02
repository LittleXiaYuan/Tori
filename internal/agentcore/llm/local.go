package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// ──────────────────────────────────────────────
// LocalBackend — supported local LLM backends
// ──────────────────────────────────────────────

// LocalBackend identifies a local LLM serving backend.
type LocalBackend string

const (
	BackendOllama   LocalBackend = "ollama"
	BackendVLLM     LocalBackend = "vllm"
	BackendLMStudio LocalBackend = "lmstudio"
	BackendLlamaCpp LocalBackend = "llamacpp"
)

// LocalModelInfo holds metadata about a model discovered from a local backend.
type LocalModelInfo struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Size     int64        `json:"size,omitempty"`     // bytes
	Modified string       `json:"modified,omitempty"` // ISO time
	Backend  LocalBackend `json:"backend"`
	BaseURL  string       `json:"base_url"`
}

// LocalProbeResult holds the result of probing a local backend.
type LocalProbeResult struct {
	Backend   LocalBackend     `json:"backend"`
	BaseURL   string           `json:"base_url"`
	Available bool             `json:"available"`
	Models    []LocalModelInfo `json:"models,omitempty"`
	Error     string           `json:"error,omitempty"`
	Latency   time.Duration    `json:"latency"`
}

// ──────────────────────────────────────────────
// ProbeLocal — check if a local backend is running
// ──────────────────────────────────────────────

// ProbeLocal checks if a local LLM backend is reachable at the given URL.
// It auto-detects the backend type and discovers available models.
func ProbeLocal(ctx context.Context, baseURL string) *LocalProbeResult {
	start := time.Now()
	result := &LocalProbeResult{BaseURL: baseURL}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Try Ollama first (/api/tags)
	if models, err := discoverOllama(ctx, baseURL); err == nil {
		result.Backend = BackendOllama
		result.Available = true
		result.Models = models
		result.Latency = time.Since(start)
		return result
	}

	// Try OpenAI-compatible /v1/models (vLLM, LM Studio, llamacpp)
	if models, err := discoverOpenAICompat(ctx, baseURL); err == nil {
		result.Backend = detectBackend(ctx, baseURL)
		result.Available = true
		result.Models = models
		result.Latency = time.Since(start)
		return result
	}

	result.Available = false
	result.Error = "no local LLM backend detected at " + baseURL
	result.Latency = time.Since(start)
	return result
}

// DiscoverModels lists available models from a local backend.
func DiscoverModels(ctx context.Context, baseURL string, backend LocalBackend) ([]LocalModelInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	switch backend {
	case BackendOllama:
		return discoverOllama(ctx, baseURL)
	case BackendVLLM, BackendLMStudio, BackendLlamaCpp:
		return discoverOpenAICompat(ctx, baseURL)
	default:
		// Auto-detect
		if models, err := discoverOllama(ctx, baseURL); err == nil {
			return models, nil
		}
		return discoverOpenAICompat(ctx, baseURL)
	}
}

// ──────────────────────────────────────────────
// AutoRegisterLocal — register local backend as provider
// ──────────────────────────────────────────────

// LocalAutoConfig holds config for auto-registering a local backend.
type LocalAutoConfig struct {
	BaseURL string
	Model   string // specific model to use; empty = first discovered
	Tier    string // "fast", "smart", "expert"; default "fast"
	Backend LocalBackend
}

// AutoRegisterLocal probes the local backend and registers it as a provider.
// Returns the provider ID if successful, or error if not reachable.
func AutoRegisterLocal(ctx context.Context, registry *ProviderRegistry, cfg LocalAutoConfig) (string, error) {
	if cfg.BaseURL == "" {
		return "", fmt.Errorf("local: base_url is required")
	}
	if cfg.Tier == "" {
		cfg.Tier = "fast"
	}

	probe := ProbeLocal(ctx, cfg.BaseURL)
	if !probe.Available {
		return "", fmt.Errorf("local backend not available at %s: %s", cfg.BaseURL, probe.Error)
	}

	model := cfg.Model
	if model == "" && len(probe.Models) > 0 {
		model = probe.Models[0].ID
	}
	if model == "" {
		return "", fmt.Errorf("no models found at %s", cfg.BaseURL)
	}

	backend := probe.Backend
	if cfg.Backend != "" {
		backend = cfg.Backend
	}

	providerID := fmt.Sprintf("local-%s", backend)

	// Determine the chat-completions base URL
	chatBase := chatBaseURL(cfg.BaseURL, backend)

	err := registry.Register(ProviderConfig{
		ID:           providerID,
		DisplayName:  fmt.Sprintf("Local %s (%s)", backend, model),
		Type:         ProviderTypeChat,
		Source:       ProviderSourceLocal,
		BaseURL:      chatBase,
		APIKeys:      []string{"local"}, // local backends don't need real keys
		Model:        model,
		Enabled:      true,
		Tier:         cfg.Tier,
		Capabilities: []Capability{CapChat},
	})
	if err != nil {
		return "", fmt.Errorf("register local provider: %w", err)
	}

	slog.Info("local model auto-registered",
		"provider", providerID,
		"backend", backend,
		"model", model,
		"tier", cfg.Tier,
		"latency", probe.Latency,
	)
	return providerID, nil
}

// ──────────────────────────────────────────────
// Internal: discovery protocols
// ──────────────────────────────────────────────

// discoverOllama calls Ollama's /api/tags endpoint.
func discoverOllama(ctx context.Context, baseURL string) ([]LocalModelInfo, error) {
	url := strings.TrimRight(baseURL, "/") + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama /api/tags: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var tagsResp struct {
		Models []struct {
			Name       string `json:"name"`
			Model      string `json:"model"`
			Size       int64  `json:"size"`
			ModifiedAt string `json:"modified_at"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &tagsResp); err != nil {
		return nil, fmt.Errorf("ollama /api/tags decode: %w", err)
	}
	models := make([]LocalModelInfo, 0, len(tagsResp.Models))
	for _, m := range tagsResp.Models {
		id := m.Name
		if id == "" {
			id = m.Model
		}
		models = append(models, LocalModelInfo{
			ID:       id,
			Name:     m.Name,
			Size:     m.Size,
			Modified: m.ModifiedAt,
			Backend:  BackendOllama,
			BaseURL:  baseURL,
		})
	}
	return models, nil
}

// discoverOpenAICompat calls /v1/models (vLLM, LM Studio, llamacpp-server).
func discoverOpenAICompat(ctx context.Context, baseURL string) ([]LocalModelInfo, error) {
	url := strings.TrimRight(baseURL, "/") + "/v1/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai-compat /v1/models: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var modelsResp struct {
		Data []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("openai-compat /v1/models decode: %w", err)
	}
	backend := detectBackend(ctx, baseURL)
	models := make([]LocalModelInfo, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		models = append(models, LocalModelInfo{
			ID:      m.ID,
			Name:    m.ID,
			Backend: backend,
			BaseURL: baseURL,
		})
	}
	return models, nil
}

// detectBackend attempts to identify the specific backend behind an OpenAI-compat API.
func detectBackend(ctx context.Context, baseURL string) LocalBackend {
	// vLLM typically has /version or returns "vllm" in headers
	url := strings.TrimRight(baseURL, "/") + "/version"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if req != nil {
		if resp, err := http.DefaultClient.Do(req); err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
				if strings.Contains(strings.ToLower(string(body)), "vllm") {
					return BackendVLLM
				}
			}
		}
	}
	// Default to generic vLLM for /v1/models-only backends
	return BackendVLLM
}

// chatBaseURL returns the correct base URL for /chat/completions based on backend.
// Ollama exposes /v1/chat/completions under the same host.
// vLLM and others already use /v1/ prefix.
func chatBaseURL(baseURL string, backend LocalBackend) string {
	base := strings.TrimRight(baseURL, "/")
	switch backend {
	case BackendOllama:
		// Ollama's OpenAI-compatible endpoint lives at <host>/v1
		return base + "/v1"
	default:
		// vLLM, LM Studio already serve at <host>/v1
		if strings.HasSuffix(base, "/v1") {
			return base
		}
		return base + "/v1"
	}
}
