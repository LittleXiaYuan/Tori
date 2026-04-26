package localbrain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// LoRAAdapter manages LoRA adapter lifecycle on a vLLM-compatible inference server.
// vLLM supports dynamic LoRA loading via its /v1/models endpoint and the
// `model` field in chat completion requests (e.g. "base-model:lora-name").
//
// For servers that don't support native LoRA management (e.g. relay station),
// this adapter degrades gracefully: Load/Unload become no-ops and List returns
// the statically configured adapters.
type LoRAAdapter struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string

	mu sync.RWMutex
	// Static fallback: if the server doesn't support dynamic LoRA management,
	// keep a local registry of known adapters.
	knownAdapters map[string]*AdapterInfo
}

// AdapterInfo describes a loaded LoRA adapter.
type AdapterInfo struct {
	Name      string    `json:"name"`
	BasePath  string    `json:"base_path,omitempty"`
	BaseModel string    `json:"base_model"`
	Status    string    `json:"status"` // "loaded" | "unloaded" | "loading" | "error"
	LoadedAt  time.Time `json:"loaded_at,omitempty"`
	Version   string    `json:"version,omitempty"`
	TenantID  string    `json:"tenant_id,omitempty"`
}

// LoRAAdapterConfig configures the vLLM LoRA adapter client.
type LoRAAdapterConfig struct {
	BaseURL string // vLLM server base URL (e.g. "http://localhost:8000")
	APIKey  string // optional API key
	Timeout time.Duration
}

func NewLoRAAdapter(cfg LoRAAdapterConfig) *LoRAAdapter {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &LoRAAdapter{
		baseURL: cfg.BaseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		apiKey:        cfg.APIKey,
		knownAdapters: make(map[string]*AdapterInfo),
	}
}

// Load requests the inference server to load a LoRA adapter.
// On vLLM, this is done via POST to the LoRA management endpoint.
// The adapter becomes available as "base-model:lora-name" in subsequent requests.
func (la *LoRAAdapter) Load(ctx context.Context, name, loraPath, baseModel string) error {
	la.mu.Lock()
	defer la.mu.Unlock()

	reqBody := map[string]string{
		"lora_name": name,
		"lora_path": loraPath,
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", la.baseURL+"/v1/lora/load", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("lora load: create request: %w", err)
	}
	la.setHeaders(req)

	resp, err := la.httpClient.Do(req)
	if err != nil {
		slog.Warn("lora adapter: server unreachable, registering locally",
			"name", name, "err", err)
		la.knownAdapters[name] = &AdapterInfo{
			Name: name, BaseModel: baseModel, Status: "local_only",
			BasePath: loraPath, LoadedAt: time.Now(),
		}
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		slog.Warn("lora adapter: server does not support LoRA management, registering locally",
			"name", name, "status", resp.StatusCode)
		la.knownAdapters[name] = &AdapterInfo{
			Name: name, BaseModel: baseModel, Status: "local_only",
			BasePath: loraPath, LoadedAt: time.Now(),
		}
		return nil
	}
	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("lora load: server returned %d: %s", resp.StatusCode, string(errBody))
	}

	la.knownAdapters[name] = &AdapterInfo{
		Name: name, BaseModel: baseModel, Status: "loaded",
		BasePath: loraPath, LoadedAt: time.Now(),
	}
	return nil
}

// Unload requests the inference server to unload a LoRA adapter.
func (la *LoRAAdapter) Unload(ctx context.Context, name string) error {
	la.mu.Lock()
	defer la.mu.Unlock()

	reqBody := map[string]string{"lora_name": name}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "DELETE", la.baseURL+"/v1/lora/"+name, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("lora unload: create request: %w", err)
	}
	la.setHeaders(req)

	resp, err := la.httpClient.Do(req)
	if err != nil {
		delete(la.knownAdapters, name)
		return nil
	}
	defer resp.Body.Close()

	delete(la.knownAdapters, name)
	return nil
}

// List returns all known LoRA adapters (from server or local registry).
func (la *LoRAAdapter) List(ctx context.Context) ([]*AdapterInfo, error) {
	la.mu.Lock()
	defer la.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, "GET", la.baseURL+"/v1/lora/list", nil)
	if err != nil {
		return la.localList(), nil
	}
	la.setHeaders(req)

	resp, err := la.httpClient.Do(req)
	if err != nil {
		return la.localList(), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return la.localList(), nil
	}

	var result struct {
		Adapters []*AdapterInfo `json:"adapters"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return la.localList(), nil
	}

	for _, a := range result.Adapters {
		la.knownAdapters[a.Name] = a
	}
	return result.Adapters, nil
}

// ModelName returns the model name to use in chat completion requests
// when a specific LoRA adapter should be active.
// Format: "base-model:lora-name" (vLLM convention).
func (la *LoRAAdapter) ModelName(baseModel, loraName string) string {
	if loraName == "" {
		return baseModel
	}
	return baseModel + ":" + loraName
}

// IsLoaded checks if a specific adapter is currently loaded.
func (la *LoRAAdapter) IsLoaded(name string) bool {
	la.mu.RLock()
	defer la.mu.RUnlock()
	info, ok := la.knownAdapters[name]
	return ok && info.Status == "loaded"
}

func (la *LoRAAdapter) localList() []*AdapterInfo {
	var list []*AdapterInfo
	for _, info := range la.knownAdapters {
		list = append(list, info)
	}
	return list
}

func (la *LoRAAdapter) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if la.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+la.apiKey)
	}
}
