// Package yunque is the official Go SDK for writing Yunque Agent plugins.
//
// It provides access to all agent capabilities through a simple API,
// enabling plugins to consume AND extend the agent's functionality.
//
// # Quick Start
//
//	package main
//
//	import "yunque-agent/sdk/go/yunque"
//
//	func main() {
//	    yunque.RegisterSkill("my_tool", "Does something", myHandler)
//	    yunque.Run()
//	}
//
//	func myHandler(ctx context.Context, args map[string]any) (string, error) {
//	    reply, _ := yunque.LLM(ctx, "You are helpful", args["input"].(string))
//	    return reply, nil
//	}
//
// # Environment Variables
//
// The agent runtime injects these when launching plugin processes:
//
//	YUNQUE_API_BASE      - Agent API base URL (default: http://localhost:9090)
//	YUNQUE_PLUGIN_TOKEN  - Plugin-scoped API token (permissions limited by manifest)
//	YUNQUE_PLUGIN_NAME   - Plugin identifier
//	YUNQUE_PLUGIN_DIR    - Plugin directory path
package yunque

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	apiBase    = envOr("YUNQUE_API_BASE", "http://localhost:9090")
	pluginToken = os.Getenv("YUNQUE_PLUGIN_TOKEN")
	pluginName  = envOr("YUNQUE_PLUGIN_NAME", os.Getenv("PLUGIN_NAME"))
	pluginDir   = envOr("YUNQUE_PLUGIN_DIR", os.Getenv("PLUGIN_DIR"))
	httpClient  = &http.Client{Timeout: 30 * time.Second}
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// PluginName returns the plugin identifier.
func PluginName() string { return pluginName }

// PluginDir returns the plugin directory path.
func PluginDir() string { return pluginDir }

// ── API Call ──

func apiCall(ctx context.Context, method, path string, body any) (map[string]any, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, apiBase+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if pluginToken != "" {
		req.Header.Set("Authorization", "Bearer "+pluginToken)
	}
	req.Header.Set("X-Plugin-Name", pluginName)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api call %s: %w", path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("api %s HTTP %d: %s", path, resp.StatusCode, string(respBody))
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result, nil
}

// ── LLM ──

// LLM calls the agent's language model with a system prompt and user input.
func LLM(ctx context.Context, systemPrompt, userInput string) (string, error) {
	resp, err := apiCall(ctx, "POST", "/v1/plugin-api/llm", map[string]any{
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userInput},
		},
		"temperature": 0.7,
	})
	if err != nil {
		return "", err
	}
	reply, _ := resp["reply"].(string)
	return reply, nil
}

// Chat sends a multi-turn conversation to the LLM.
func Chat(ctx context.Context, messages []Message, temperature float64) (string, error) {
	msgList := make([]map[string]string, len(messages))
	for i, m := range messages {
		msgList[i] = map[string]string{"role": m.Role, "content": m.Content}
	}
	resp, err := apiCall(ctx, "POST", "/v1/plugin-api/llm", map[string]any{
		"messages":    msgList,
		"temperature": temperature,
	})
	if err != nil {
		return "", err
	}
	reply, _ := resp["reply"].(string)
	return reply, nil
}

// Message is a chat message.
type Message struct {
	Role    string
	Content string
}

// ── Web Search ──

// SearchResult is a single search result.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// Search queries the web using the agent's search providers.
func Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	resp, err := apiCall(ctx, "POST", "/v1/plugin-api/search", map[string]any{
		"query": query, "limit": limit,
	})
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(resp["results"])
	var results []SearchResult
	json.Unmarshal(raw, &results)
	return results, nil
}

// ── Channel Messaging ──

// Send sends a message through a channel (Telegram, Feishu, Discord, etc.).
func Send(ctx context.Context, channelType, target, content string) error {
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/send", map[string]any{
		"channel": channelType,
		"target":  target,
		"content": content,
		"format":  "markdown",
	})
	return err
}

// ── HTTP Request ──

// HTTP makes an arbitrary HTTP request (requires "network" permission).
func HTTP(ctx context.Context, method, url string, body any, headers map[string]string) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reqBody = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
	return data, resp.StatusCode, nil
}

// ── Plugin Memory (private namespace) ──

// Memory provides access to the plugin's private key-value store.
var Memory = &memoryNamespace{}

type memoryNamespace struct{}

func (m *memoryNamespace) Get(ctx context.Context, key string) (string, error) {
	resp, err := apiCall(ctx, "POST", "/v1/plugin-api/memory/get", map[string]any{"key": key})
	if err != nil {
		return "", err
	}
	v, _ := resp["value"].(string)
	return v, nil
}

func (m *memoryNamespace) Set(ctx context.Context, key, value string) error {
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/memory/set", map[string]any{
		"key": key, "value": value,
	})
	return err
}

func (m *memoryNamespace) Delete(ctx context.Context, key string) error {
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/memory/delete", map[string]any{"key": key})
	return err
}

func (m *memoryNamespace) List(ctx context.Context, prefix string) (map[string]string, error) {
	resp, err := apiCall(ctx, "POST", "/v1/plugin-api/memory/list", map[string]any{"prefix": prefix})
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(resp["entries"])
	var entries map[string]string
	json.Unmarshal(raw, &entries)
	return entries, nil
}

func (m *memoryNamespace) Search(ctx context.Context, query string, limit int) ([]string, error) {
	resp, err := apiCall(ctx, "POST", "/v1/plugin-api/memory/search", map[string]any{
		"query": query, "limit": limit,
	})
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(resp["results"])
	var results []string
	json.Unmarshal(raw, &results)
	return results, nil
}

// ── Agent Memory (shared) ──

// AgentMemory provides access to the agent's shared memory system.
var AgentMemory = &agentMemoryNamespace{}

type agentMemoryNamespace struct{}

// Search queries the agent's combined memory (short+mid+long+graph+editable).
func (m *agentMemoryNamespace) Search(ctx context.Context, query string) (string, error) {
	resp, err := apiCall(ctx, "POST", "/v1/plugin-api/agent-memory/search", map[string]any{
		"query": query, "top_k": 5,
	})
	if err != nil {
		return "", err
	}
	context, _ := resp["context"].(string)
	return context, nil
}

// Add writes a fact to the agent's mid-term memory.
func (m *agentMemoryNamespace) Add(ctx context.Context, fact string) error {
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/agent-memory/add", map[string]any{
		"fact": fact, "source": pluginName,
	})
	return err
}

// ── Knowledge Base ──

// Knowledge provides access to the agent's RAG knowledge base.
var Knowledge = &knowledgeNamespace{}

type knowledgeNamespace struct{}

func (k *knowledgeNamespace) Search(ctx context.Context, query string, limit int) ([]map[string]any, error) {
	resp, err := apiCall(ctx, "POST", "/v1/plugin-api/knowledge/search", map[string]any{
		"query": query, "limit": limit,
	})
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(resp["results"])
	var results []map[string]any
	json.Unmarshal(raw, &results)
	return results, nil
}

func (k *knowledgeNamespace) Ingest(ctx context.Context, content, filename string) error {
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/knowledge/ingest", map[string]any{
		"content": content, "source": pluginName, "filename": filename,
	})
	return err
}

// ── Cron ──

// Cron provides access to the scheduled task system.
var Cron = &cronNamespace{}

type cronNamespace struct{}

func (c *cronNamespace) Add(ctx context.Context, expr, name, message string) (string, error) {
	resp, err := apiCall(ctx, "POST", "/v1/plugin-api/cron/add", map[string]any{
		"expression": expr,
		"name":       pluginName + ":" + name,
		"message":    message,
	})
	if err != nil {
		return "", err
	}
	id, _ := resp["id"].(string)
	return id, nil
}

func (c *cronNamespace) Remove(ctx context.Context, id string) error {
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/cron/remove", map[string]any{"id": id})
	return err
}

// ── Skill Registration ──

// SkillHandler is the function signature for plugin-provided skills.
type SkillHandler func(ctx context.Context, args map[string]any) (string, error)

// HookHandler is the function signature for lifecycle hooks.
type HookHandler func(ctx context.Context, data map[string]any)

var (
	registeredSkills = make(map[string]registeredSkill)
	registeredHooks  = make(map[string][]HookHandler)
	hookMu           sync.RWMutex
)

type registeredSkill struct {
	Name        string
	Description string
	Handler     SkillHandler
}

// RegisterSkill registers a new skill that the Planner can call.
// The skill will be available as a function call tool in the LLM.
func RegisterSkill(name, description string, handler SkillHandler) {
	registeredSkills[name] = registeredSkill{
		Name:        name,
		Description: description,
		Handler:     handler,
	}
}

// OnChatBefore registers a hook called before each chat message is processed.
func OnChatBefore(handler HookHandler) {
	hookMu.Lock()
	registeredHooks["chat.before"] = append(registeredHooks["chat.before"], handler)
	hookMu.Unlock()
}

// OnChatAfter registers a hook called after each chat reply is generated.
func OnChatAfter(handler HookHandler) {
	hookMu.Lock()
	registeredHooks["chat.after"] = append(registeredHooks["chat.after"], handler)
	hookMu.Unlock()
}

// OnMemoryExtract registers a hook called when facts are extracted from conversations.
func OnMemoryExtract(handler HookHandler) {
	hookMu.Lock()
	registeredHooks["memory.extract"] = append(registeredHooks["memory.extract"], handler)
	hookMu.Unlock()
}

// ── System Extension Registration ──

// RegisterProvider adds a new LLM provider to the agent.
// The provider must serve an OpenAI-compatible API.
func RegisterProvider(ctx context.Context, id, baseURL, model string, opts ...ProviderOpt) error {
	cfg := map[string]any{
		"id": id, "base_url": baseURL, "model": model, "type": "chat",
	}
	for _, opt := range opts {
		opt(cfg)
	}
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/register/provider", cfg)
	return err
}

// ProviderOpt configures optional provider fields.
type ProviderOpt func(map[string]any)

func WithAPIKeys(keys ...string) ProviderOpt {
	return func(m map[string]any) { m["api_keys"] = keys }
}

func WithTier(tier string) ProviderOpt {
	return func(m map[string]any) { m["tier"] = tier }
}

func WithProviderType(t string) ProviderOpt {
	return func(m map[string]any) { m["type"] = t }
}

// RegisterChannel adds a new messaging channel adapter.
func RegisterChannel(ctx context.Context, name, webhookURL, sendEndpoint string) error {
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/register/channel", map[string]any{
		"name": name, "webhook_url": webhookURL, "send_endpoint": sendEndpoint,
	})
	return err
}

// RegisterSearchEngine adds a new web search provider.
func RegisterSearchEngine(ctx context.Context, name, baseURL, apiKey string) error {
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/register/search", map[string]any{
		"name": name, "base_url": baseURL, "api_key": apiKey,
	})
	return err
}

// RegisterGuardrail adds a new safety rule.
func RegisterGuardrail(ctx context.Context, name, description, phase string, keywords []string) error {
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/register/guardrail", map[string]any{
		"name": name, "description": description, "phase": phase, "keywords": keywords,
	})
	return err
}

// RegisterEmbedding adds a new vector embedding model.
func RegisterEmbedding(ctx context.Context, name, baseURL, model string, dimensions int) error {
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/register/embedding", map[string]any{
		"name": name, "base_url": baseURL, "model": model, "dimensions": dimensions,
	})
	return err
}

// RegisterSpeech adds a new TTS or STT engine.
func RegisterSpeech(ctx context.Context, name, speechType, baseURL, model string) error {
	_, err := apiCall(ctx, "POST", "/v1/plugin-api/register/speech", map[string]any{
		"name": name, "type": speechType, "base_url": baseURL, "model": model,
	})
	return err
}

// ── Plugin Lifecycle ──

// Run starts the plugin and blocks until the agent shuts it down.
// It starts an HTTP server for the agent to call registered skills and hooks,
// then registers itself with the agent via the plugin API.
func Run() {
	port := envOr("YUNQUE_PLUGIN_PORT", "0")

	mux := http.NewServeMux()

	// Skill execution endpoint
	mux.HandleFunc("/skill/", func(w http.ResponseWriter, r *http.Request) {
		skillName := strings.TrimPrefix(r.URL.Path, "/skill/")
		skill, ok := registeredSkills[skillName]
		if !ok {
			http.Error(w, `{"error":"skill not found"}`, http.StatusNotFound)
			return
		}
		var args map[string]any
		json.NewDecoder(r.Body).Decode(&args)

		result, err := skill.Handler(r.Context(), args)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"result": result})
	})

	// Hook callback endpoint
	mux.HandleFunc("/hook/", func(w http.ResponseWriter, r *http.Request) {
		hookName := strings.TrimPrefix(r.URL.Path, "/hook/")
		hookMu.RLock()
		handlers := registeredHooks[hookName]
		hookMu.RUnlock()

		var data map[string]any
		json.NewDecoder(r.Body).Decode(&data)

		for _, h := range handlers {
			h(r.Context(), data)
		}
		w.WriteHeader(http.StatusOK)
	})

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Start server
	srv := &http.Server{Addr: ":" + port, Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("plugin server failed", "err", err)
		}
	}()

	slog.Info("yunque plugin started",
		"name", pluginName,
		"skills", len(registeredSkills),
		"hooks", len(registeredHooks),
	)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	slog.Info("yunque plugin stopped", "name", pluginName)
}
