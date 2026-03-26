package llm

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"yunque-agent/internal/observe"
)

// Client is an OpenAI-compatible LLM caller.
type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
	breaker *CircuitBreaker
	cache   *ResponseCache
}

// newOptimizedTransport creates an HTTP transport tuned for LLM API calls:
// - Connection pooling: reuse TCP connections across requests
// - Keep-alive: prevent TCP re-handshake overhead
// - TLS session caching: avoid full TLS negotiation on reconnect
// - Response header timeout: fail fast if server accepts but doesn't respond
func newOptimizedTransport() *http.Transport {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,  // TCP connect timeout
			KeepAlive: 30 * time.Second, // TCP keep-alive probe interval
		}).DialContext,
		MaxIdleConns:          100,              // total idle connections across all hosts
		MaxIdleConnsPerHost:   10,               // idle connections per LLM API host
		MaxConnsPerHost:       20,               // max concurrent connections per host
		IdleConnTimeout:       90 * time.Second, // kill idle connections after 90s
		TLSHandshakeTimeout:   10 * time.Second, // TLS handshake limit
		ExpectContinueTimeout: 1 * time.Second,  // 100-continue wait
		ResponseHeaderTimeout: 30 * time.Second, // time to first response byte
		ForceAttemptHTTP2:     true,              // prefer HTTP/2 for multiplexing
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
}

// NewClient creates a new LLM client with optimized HTTP transport.
// No global timeout — each request uses context deadline instead,
// allowing streaming calls to run longer than non-streaming ones.
func NewClient(baseURL, apiKey, model string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		http:    &http.Client{Transport: newOptimizedTransport()},
		breaker: NewCircuitBreaker(5, 30*time.Second),
		cache:   NewResponseCache(60*time.Second, 256),
	}
}

// Breaker returns the circuit breaker for status inspection.
func (c *Client) Breaker() *CircuitBreaker { return c.breaker }

// Model returns the current default model ID.
func (c *Client) Model() string { return c.model }

// Message is a chat message.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// ChatRequest is the request to the LLM.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream"`
}

// ChatResponse is the non-streaming LLM response.
type ChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

// streamChunk is a single SSE chunk from the streaming API.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

// Chat sends messages to the LLM and returns the assistant reply.
// Retries up to 2 times on transient errors (5xx, timeout).
// Cache returns the response cache for stats/inspection.
func (c *Client) Cache() *ResponseCache { return c.cache }

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

func (c *Client) chatOnce(ctx context.Context, messages []Message, temperature float64) (string, error) {
	req := ChatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: temperature,
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
	scanner := bufio.NewScanner(body)
	// Set max token size to 1MB per line to handle large SSE chunks
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	const maxResponseBytes = 10 * 1024 * 1024 // 10MB
	var sb strings.Builder
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
			continue // skip malformed chunks
		}
		for _, choice := range chunk.Choices {
			sb.WriteString(choice.Delta.Content)
		}
		if sb.Len() > maxResponseBytes {
			return sb.String(), fmt.Errorf("llm: response exceeded %d bytes limit", maxResponseBytes)
		}
	}
	if err := scanner.Err(); err != nil {
		return sb.String(), fmt.Errorf("sse read: %w", err)
	}
	result := sb.String()
	if result == "" {
		return "", fmt.Errorf("llm: empty response from stream")
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
	req := ChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temperature,
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
