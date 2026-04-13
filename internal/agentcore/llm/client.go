package llm

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"yunque-agent/internal/observe"
)

// Dialect determines the API format used by the client.
type Dialect string

const (
	DialectOpenAI    Dialect = ""          // OpenAI-compatible (default)
	DialectAnthropic Dialect = "anthropic" // Anthropic Messages API
)

// Client is an LLM caller supporting OpenAI-compatible and Anthropic APIs.
type Client struct {
	baseURL       string
	apiKey        string
	model         string
	dialect       Dialect
	http          *http.Client
	breaker       *CircuitBreaker
	cache         *ResponseCache
	contextWindow int // in K tokens (e.g. 128 = 128K), 0 = use default
}

// newOptimizedTransport creates an HTTP transport tuned for LLM API calls:
// - Connection pooling: reuse TCP connections across requests
// - Keep-alive: prevent TCP re-handshake overhead
// - TLS session caching: avoid full TLS negotiation on reconnect
// - Response header timeout: fail fast if server accepts but doesn't respond
func newOptimizedTransport() *http.Transport {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second, // TCP connect timeout (generous for CN proxies)
			KeepAlive: 30 * time.Second, // TCP keep-alive probe interval
		}).DialContext,
		MaxIdleConns:          100,              // total idle connections across all hosts
		MaxIdleConnsPerHost:   10,               // idle connections per LLM API host
		MaxConnsPerHost:       20,               // max concurrent connections per host
		IdleConnTimeout:       90 * time.Second, // kill idle connections after 90s
		TLSHandshakeTimeout:   15 * time.Second, // TLS handshake limit
		ExpectContinueTimeout: 1 * time.Second,  // 100-continue wait
		ResponseHeaderTimeout: 60 * time.Second, // time to first response byte (tool calls need more)
		ForceAttemptHTTP2:     false,            // disable HTTP/2 — causes h2 timeout with some CN proxies
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
}

// NewClient creates a new LLM client with optimized HTTP transport.
// No global timeout — each request uses context deadline instead,
// allowing streaming calls to run longer than non-streaming ones.
func NewClient(baseURL, apiKey, model string) *Client {
	return newClientWithDialect(baseURL, apiKey, model, DialectOpenAI)
}

// NewClaudeClient creates a client configured for the Anthropic Messages API.
func NewClaudeClient(baseURL, apiKey, model string) *Client {
	return newClientWithDialect(baseURL, apiKey, model, DialectAnthropic)
}

func newClientWithDialect(baseURL, apiKey, model string, dialect Dialect) *Client {
	return &Client{
		baseURL: baseURL,
		dialect: dialect,
		apiKey:  apiKey,
		model:   model,
		http:    &http.Client{Transport: newOptimizedTransport()},
		breaker: NewCircuitBreaker(8, 15*time.Second),
		cache:   NewResponseCache(60*time.Second, 256),
	}
}

// Breaker returns the circuit breaker for status inspection.
func (c *Client) Breaker() *CircuitBreaker { return c.breaker }

// Model returns the current default model ID.
func (c *Client) Model() string { return c.model }

// SetContextWindow sets the model's context window in K tokens (e.g. 128 = 128K).
func (c *Client) SetContextWindow(k int) { c.contextWindow = k }

// ContextWindowTokens returns the model's context window in tokens.
// Falls back to 128K if not explicitly set.
func (c *Client) ContextWindowTokens() int {
	if c.contextWindow > 0 {
		return c.contextWindow * 1024
	}
	return 128 * 1024
}

// ContentPart represents one segment of a multimodal message.
// Supports text, image_url (vision), and video_url (Kimi K2.5 / Qwen VL).
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *MediaURL `json:"image_url,omitempty"`
	VideoURL *MediaURL `json:"video_url,omitempty"`
}

// MediaURL carries base64 or URL for vision / video models.
type MediaURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// ImageURL is an alias kept for backward compatibility.
type ImageURL = MediaURL

// Message is a chat message. For multimodal, populate ContentParts instead of Content.
type Message struct {
	Role             string        `json:"role"`
	Content          string        `json:"content"`
	ContentParts     []ContentPart `json:"content_parts,omitempty"`
	ToolCallID       string        `json:"tool_call_id,omitempty"`
	ToolCalls        []ToolCall    `json:"tool_calls,omitempty"`
	ReasoningContent string        `json:"-"` // preserved for multi-turn thinking (Kimi K2.5 etc.)
}

// MarshalJSON produces OpenAI-compatible JSON: if ContentParts is set, "content" becomes an array.
func (m Message) MarshalJSON() ([]byte, error) {
	type plain struct {
		Role             string     `json:"role"`
		Content          any        `json:"content"`
		ToolCallID       string     `json:"tool_call_id,omitempty"`
		ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
		ReasoningContent string     `json:"reasoning_content,omitempty"`
	}
	p := plain{Role: m.Role, ToolCallID: m.ToolCallID, ToolCalls: m.ToolCalls, ReasoningContent: m.ReasoningContent}
	if len(m.ContentParts) > 0 {
		p.Content = m.ContentParts
	} else {
		p.Content = m.Content
	}
	return json.Marshal(p)
}

// UnmarshalJSON handles both string and array "content" from API responses.
func (m *Message) UnmarshalJSON(data []byte) error {
	type plain struct {
		Role             string          `json:"role"`
		Content          json.RawMessage `json:"content"`
		ToolCallID       string          `json:"tool_call_id,omitempty"`
		ToolCalls        []ToolCall      `json:"tool_calls,omitempty"`
		ReasoningContent string          `json:"reasoning_content,omitempty"`
	}
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	m.Role = p.Role
	m.ToolCallID = p.ToolCallID
	m.ToolCalls = p.ToolCalls
	m.ReasoningContent = p.ReasoningContent
	if len(p.Content) > 0 && p.Content[0] == '"' {
		return json.Unmarshal(p.Content, &m.Content)
	}
	if len(p.Content) > 0 && p.Content[0] == '[' {
		if err := json.Unmarshal(p.Content, &m.ContentParts); err == nil {
			for _, part := range m.ContentParts {
				if part.Type == "text" {
					m.Content += part.Text
				}
			}
			return nil
		}
	}
	m.Content = string(p.Content)
	return nil
}

// ChatRequest is the request to the LLM.
type ChatRequest struct {
	Model       string         `json:"model"`
	Messages    []Message      `json:"messages"`
	Temperature float64        `json:"temperature,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Stream      bool           `json:"stream"`
	Extra       map[string]any `json:"-"` // provider-specific params merged at marshal time
}

// MarshalJSON merges Extra fields into the top-level JSON object.
func (r ChatRequest) MarshalJSON() ([]byte, error) {
	type plain ChatRequest
	b, err := json.Marshal(plain(r))
	if err != nil || len(r.Extra) == 0 {
		return b, err
	}
	var m map[string]json.RawMessage
	_ = json.Unmarshal(b, &m)
	for k, v := range r.Extra {
		raw, _ := json.Marshal(v)
		m[k] = raw
	}
	return json.Marshal(m)
}

// ChatResponse is the non-streaming LLM response.
type ChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

// streamChunk is a single SSE chunk from the streaming API.
// Supports reasoning_content (DeepSeek R1, Kimi K2.5, etc.)
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Role             string `json:"role"`
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

// ChatResult contains the full response including optional reasoning content.
type ChatResult struct {
	Content          string
	ReasoningContent string
}

// StreamDeltaFunc is called for each token during streaming.
// contentDelta is normal reply text, reasoningDelta is thinking/reasoning text.
type StreamDeltaFunc func(contentDelta, reasoningDelta string)

// Chat sends messages to the LLM and returns the assistant reply.
// Retries up to 2 times on transient errors (5xx, timeout).
// Cache returns the response cache for stats/inspection.
func (c *Client) Cache() *ResponseCache { return c.cache }

// Close releases resources held by the client (stops cache eviction loop).
func (c *Client) Close() {
	if c.cache != nil {
		c.cache.Stop()
	}
}

func (c *Client) Chat(ctx context.Context, messages []Message, temperature float64) (string, error) {
	// Streaming responses need generous timeout (LLM generation can take time)
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
		defer cancel()
	}
	ctx, span := observe.StartSpan(ctx, "llm.Chat")
	span.Attrs["model"] = c.model
	if err := c.breaker.Allow(); err != nil {
		observe.EndSpan(span, err)
		return "", err
	}
	// Check cache for identical request
	if cached, ok := c.cache.Get(messages, temperature); ok {
		span.Attrs["cache"] = "hit"
		observe.EndSpan(span, nil)
		return cached, nil
	}
	const maxRetries = 3
	var lastErr error
	hasImages := messagesHaveImages(messages)
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * 500 * time.Millisecond
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
		}
		result, err := c.chatOnce(ctx, messages, temperature)
		if err == nil {
			c.breaker.RecordSuccess()
			c.cache.Put(messages, temperature, result)
			observe.EndSpan(span, nil)
			return result, nil
		}
		lastErr = err
		// Vision fallback: if model returns 400 and messages contain images, strip images and retry
		if hasImages && strings.Contains(err.Error(), "status 400") {
			slog.Warn("llm: 400 with images, falling back to text-only", "model", c.model)
			messages = stripImages(messages)
			hasImages = false
		}
		if ctx.Err() != nil {
			observe.EndSpan(span, ctx.Err())
			return "", ctx.Err()
		}
	}
	c.breaker.RecordFailure()
	finalErr := fmt.Errorf("llm: all %d attempts failed: %w", maxRetries, lastErr)
	observe.EndSpan(span, finalErr)
	return "", finalErr
}

// ChatFull is like Chat but returns both content and reasoning_content.
// Optional onDelta streams each token in real-time.
func (c *Client) ChatFull(ctx context.Context, messages []Message, temperature float64, onDelta ...StreamDeltaFunc) (ChatResult, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
		defer cancel()
	}
	if err := c.breaker.Allow(); err != nil {
		return ChatResult{}, err
	}
	result, err := c.chatOnceFull(ctx, messages, temperature, onDelta...)
	if err == nil {
		c.breaker.RecordSuccess()
	} else {
		c.breaker.RecordFailure()
	}
	return result, err
}

func (c *Client) chatOnceFull(ctx context.Context, messages []Message, temperature float64, onDelta ...StreamDeltaFunc) (ChatResult, error) {
	if c.dialect == DialectAnthropic {
		return c.chatOnceAnthropicFull(ctx, messages, temperature, onDelta...)
	}
	messages = normalizeSystemMessages(messages)
	temp := temperature
	if GetConstraints(c.model).FixedTemperature {
		temp = 0
	}
	req := ChatRequest{Model: c.model, Messages: messages, Temperature: temp, MaxTokens: 4096, Stream: true}
	b, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return ChatResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return ChatResult{}, fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return ChatResult{}, fmt.Errorf("llm api %d: %.500s", resp.StatusCode, string(body))
	}
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		return c.readSSEFull(resp.Body, onDelta...)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChatResult{}, fmt.Errorf("read llm response: %w", err)
	}
	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return ChatResult{}, fmt.Errorf("decode llm response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return ChatResult{}, fmt.Errorf("llm: no choices")
	}
	return ChatResult{Content: chatResp.Choices[0].Message.Content}, nil
}

func (c *Client) chatOnce(ctx context.Context, messages []Message, temperature float64) (string, error) {
	if c.dialect == DialectAnthropic {
		return c.chatOnceAnthropic(ctx, messages, temperature)
	}
	messages = normalizeSystemMessages(messages)
	temp := temperature
	if GetConstraints(c.model).FixedTemperature {
		temp = 0
	}
	req := ChatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: temp,
		MaxTokens:   4096,
		Stream:      true,
	}
	b, _ := json.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("llm api %d: %.500s", resp.StatusCode, string(body))
	}

	// Check if response is SSE stream or regular JSON
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		return c.readSSE(resp.Body)
	}

	// Fallback: regular JSON response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read llm response: %w", err)
	}
	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("decode llm response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("llm: no choices")
	}
	return chatResp.Choices[0].Message.Content, nil
}

// readSSE parses a Server-Sent Events stream and concatenates delta content.
// Limits total accumulated response to 10MB to prevent OOM from malicious streams.
func (c *Client) readSSE(body io.Reader) (string, error) {
	r, err := c.readSSEFull(body)
	if err != nil {
		return r.Content, err
	}
	return r.Content, nil
}

// readSSEFull parses SSE and returns both content and reasoning_content.
// If onDelta is non-nil, each chunk is streamed to it in real-time.
func (c *Client) readSSEFull(body io.Reader, onDelta ...StreamDeltaFunc) (ChatResult, error) {
	var cb StreamDeltaFunc
	if len(onDelta) > 0 {
		cb = onDelta[0]
	}
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	const maxResponseBytes = 10 * 1024 * 1024
	var content, reasoning strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimPrefix(line, "data:")
		data = strings.TrimSpace(data)
		if data == "[DONE]" {
			break
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		for _, choice := range chunk.Choices {
			content.WriteString(choice.Delta.Content)
			reasoning.WriteString(choice.Delta.ReasoningContent)
			if cb != nil && (choice.Delta.Content != "" || choice.Delta.ReasoningContent != "") {
				cb(choice.Delta.Content, choice.Delta.ReasoningContent)
			}
		}
		if content.Len()+reasoning.Len() > maxResponseBytes {
			return ChatResult{Content: content.String(), ReasoningContent: reasoning.String()},
				fmt.Errorf("llm: response exceeded %d bytes limit", maxResponseBytes)
		}
	}
	if err := scanner.Err(); err != nil {
		return ChatResult{Content: content.String(), ReasoningContent: reasoning.String()},
			fmt.Errorf("sse read: %w", err)
	}
	result := ChatResult{Content: content.String(), ReasoningContent: reasoning.String()}
	if result.Content == "" && result.ReasoningContent == "" {
		return result, fmt.Errorf("llm: empty response from stream")
	}
	if result.Content == "" && result.ReasoningContent != "" {
		result.Content = result.ReasoningContent
	}
	return result, nil
}

// ChatJSON sends a request with response_format=json_object for structured output.
// Retries up to 2 times on transient errors (5xx, timeout).
func (c *Client) ChatJSON(ctx context.Context, messages []Message) (string, error) {
	// JSON responses are typically shorter, use tighter timeout
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
	}
	if err := c.breaker.Allow(); err != nil {
		return "", err
	}
	const maxRetries = 3
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * 500 * time.Millisecond
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
		}
		result, err := c.chatJSONOnce(ctx, messages)
		if err == nil {
			c.breaker.RecordSuccess()
			return result, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
	}
	c.breaker.RecordFailure()
	return "", fmt.Errorf("chat json: all %d attempts failed: %w", maxRetries, lastErr)
}

func (c *Client) chatJSONOnce(ctx context.Context, messages []Message) (string, error) {
	reqBody := map[string]any{
		"model":           c.model,
		"messages":        messages,
		"temperature":     0,
		"stream":          true,
		"response_format": map[string]string{"type": "json_object"},
	}
	if adj := SanitizeRequestBody(reqBody, c.model); len(adj) > 0 {
		slog.Info("llm: sanitized JSON request", "model", c.model, "adjusted", adj)
	}
	b, _ := json.Marshal(reqBody)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("chat json: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("llm api %d: %.500s", resp.StatusCode, string(body))
	}
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		return c.readSSE(resp.Body)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("llm: no choices")
	}
	return chatResp.Choices[0].Message.Content, nil
}

// ChatWithModel calls the LLM using a specific model without mutating the client's default model.
// This is safe for concurrent use — no shared state is modified.
func (c *Client) ChatWithModel(ctx context.Context, model string, messages []Message, temperature float64) (string, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
		defer cancel()
	}
	ctx, span := observe.StartSpan(ctx, "llm.ChatWithModel")
	span.Attrs["model"] = model
	if err := c.breaker.Allow(); err != nil {
		observe.EndSpan(span, err)
		return "", err
	}
	if cached, ok := c.cache.Get(messages, temperature); ok {
		span.Attrs["cache"] = "hit"
		observe.EndSpan(span, nil)
		return cached, nil
	}
	const maxRetries = 3
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * 500 * time.Millisecond
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
			}
		}
		result, err := c.chatOnceWithModel(ctx, model, messages, temperature)
		if err == nil {
			c.breaker.RecordSuccess()
			c.cache.Put(messages, temperature, result)
			observe.EndSpan(span, nil)
			return result, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			observe.EndSpan(span, ctx.Err())
			return "", ctx.Err()
		}
	}
	c.breaker.RecordFailure()
	finalErr := fmt.Errorf("llm: all %d attempts failed: %w", maxRetries, lastErr)
	observe.EndSpan(span, finalErr)
	return "", finalErr
}

// chatOnceWithModel is like chatOnce but uses the specified model instead of c.model.
func (c *Client) chatOnceWithModel(ctx context.Context, model string, messages []Message, temperature float64) (string, error) {
	temp := temperature
	if GetConstraints(model).FixedTemperature {
		temp = 0
	}
	req := ChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temp,
		MaxTokens:   4096,
		Stream:      true,
	}
	b, _ := json.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("llm api %d: %.500s", resp.StatusCode, string(body))
	}

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		return c.readSSE(resp.Body)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read llm response: %w", err)
	}
	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("decode llm response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("llm: no choices")
	}
	return chatResp.Choices[0].Message.Content, nil
}

func messagesHaveMedia(msgs []Message) bool {
	for _, m := range msgs {
		for _, p := range m.ContentParts {
			if p.Type == "image_url" || p.Type == "video_url" {
				return true
			}
		}
	}
	return false
}

// messagesHaveImages kept for backward compat (also checks video).
func messagesHaveImages(msgs []Message) bool { return messagesHaveMedia(msgs) }

func stripImages(msgs []Message) []Message {
	out := make([]Message, len(msgs))
	for i, m := range msgs {
		out[i] = m
		if len(m.ContentParts) > 0 {
			var textOnly []ContentPart
			for _, p := range m.ContentParts {
				if p.Type == "text" {
					textOnly = append(textOnly, p)
				}
			}
			out[i].ContentParts = nil
			out[i].Content = ""
			for _, p := range textOnly {
				out[i].Content += p.Text
			}
		}
	}
	return out
}

// normalizeSystemMessages merges all system-role messages into a single
// leading system message to avoid validation errors on models (e.g. Qwen3.5)
// that reject mid-conversation system messages.
func normalizeSystemMessages(msgs []Message) []Message {
	if len(msgs) == 0 {
		return msgs
	}

	var sysBuilder strings.Builder
	out := make([]Message, 0, len(msgs))
	firstSysDone := false

	for _, m := range msgs {
		if m.Role == "system" {
			if sysBuilder.Len() > 0 {
				sysBuilder.WriteString("\n\n")
			}
			sysBuilder.WriteString(m.Content)
			continue
		}
		if !firstSysDone && sysBuilder.Len() > 0 {
			out = append(out, Message{Role: "system", Content: sysBuilder.String()})
			firstSysDone = true
			sysBuilder.Reset()
		}
		out = append(out, m)
		if m.Role == "system" {
			// should not happen due to outer if, but guard
			continue
		}
	}

	if sysBuilder.Len() > 0 && !firstSysDone {
		out = append([]Message{{Role: "system", Content: sysBuilder.String()}}, out...)
	} else if sysBuilder.Len() > 0 {
		// trailing system messages after the last user/assistant — merge into the first system
		if len(out) > 0 && out[0].Role == "system" {
			out[0].Content += "\n\n" + sysBuilder.String()
		} else {
			out = append([]Message{{Role: "system", Content: sysBuilder.String()}}, out...)
		}
	}

	return out
}
