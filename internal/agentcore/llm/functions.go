package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// FunctionDef defines a function/tool for the LLM to call.
type FunctionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolCall represents a tool call returned by the LLM.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ChatChoice with tool calls support.
type ChatChoiceFC struct {
	Message struct {
		Role             string     `json:"role"`
		Content          string     `json:"content"`
		ReasoningContent string     `json:"reasoning_content,omitempty"`
		ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
}

// ChatResponseFC is the response with function calling support.
type ChatResponseFC struct {
	Choices []ChatChoiceFC `json:"choices"`
}

// ToolChoiceAuto lets the model decide whether to call tools or respond with text.
const ToolChoiceAuto = "auto"

// ToolChoiceRequired forces the model to call at least one tool (no plain text allowed).
const ToolChoiceRequired = "required"

// ChatWithTools sends a chat request with function/tool definitions.
// Returns the response message and any tool calls.
// ChatWithToolsOpts holds optional parameters for ChatWithTools.
type ChatWithToolsOpts struct {
	ThinkingEnabled    *bool
	OnReasoning        func(reasoning string) // called when reasoning_content is received
	LastReasoningOut   *string                // if set, receives the reasoning_content from the response
}

func (c *Client) ChatWithTools(ctx context.Context, messages []Message, tools []FunctionDef, temperature float64, toolChoice ...string) (string, []ToolCall, error) {
	return c.ChatWithToolsEx(ctx, messages, tools, temperature, nil, toolChoice...)
}

func (c *Client) ChatWithToolsEx(ctx context.Context, messages []Message, tools []FunctionDef, temperature float64, opts *ChatWithToolsOpts, toolChoice ...string) (string, []ToolCall, error) {
	if err := c.breaker.Allow(); err != nil {
		return "", nil, err
	}

	if c.dialect == DialectAnthropic {
		reply, calls, err := c.chatWithToolsAnthropic(ctx, messages, tools, temperature)
		if err == nil {
			c.breaker.RecordSuccess()
		} else {
			c.breaker.RecordFailure()
		}
		return reply, calls, err
	}

	// Build tool definitions once
	toolDefs := make([]map[string]any, len(tools))
	for i, t := range tools {
		toolDefs[i] = map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Parameters,
			},
		}
	}

	reqBody := map[string]any{
		"model":       c.model,
		"messages":    messages,
		"temperature": temperature,
		"tools":       toolDefs,
		"stream":      false,
	}
	if len(toolChoice) > 0 && toolChoice[0] != "" && toolChoice[0] != ToolChoiceAuto {
		reqBody["tool_choice"] = toolChoice[0]
	}
	if adj := SanitizeRequestBody(reqBody, c.model); len(adj) > 0 {
		slog.Info("llm: sanitized request for model", "model", c.model, "adjusted", adj)
	}
	if opts != nil {
		InjectThinking(reqBody, c.model, opts.ThinkingEnabled)
	}
	body, _ := json.Marshal(reqBody)

	// Retry loop (same as Chat) — tool call payloads are large, network can be flaky
	const maxRetries = 3
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * time.Second
			slog.Info("llm: retrying ChatWithTools", "attempt", attempt+1, "backoff", backoff)
			select {
			case <-ctx.Done():
				return "", nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		url := c.baseURL + "/chat/completions"
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
		if err != nil {
			return "", nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("chat with tools: %w", err)
			if ctx.Err() != nil {
				return "", nil, ctx.Err()
			}
			continue
		}

		if resp.StatusCode != 200 {
			respBody := make([]byte, 2048)
			n, _ := resp.Body.Read(respBody)
			resp.Body.Close()
			errDetail := string(respBody[:n])
			lastErr = fmt.Errorf("chat API status %d: %.500s", resp.StatusCode, errDetail)
			slog.Warn("llm: ChatWithTools non-200", "status", resp.StatusCode, "model", c.model, "body", errDetail)

			if resp.StatusCode == 400 {
				if messagesHaveImages(messages) {
					slog.Warn("llm: ChatWithTools 400 with images, falling back to text-only", "model", c.model)
					messages = stripImages(messages)
					reqBody["messages"] = messages
					body, _ = json.Marshal(reqBody)
					continue
				}
				// Auto-fix: try stripping temperature/response_format and retry once
				if attempt == 0 {
					slog.Info("llm: ChatWithTools 400, auto-sanitizing request params", "model", c.model)
					delete(reqBody, "temperature")
					delete(reqBody, "response_format")
					delete(reqBody, "top_p")
					delete(reqBody, "frequency_penalty")
					delete(reqBody, "presence_penalty")
					body, _ = json.Marshal(reqBody)
					continue
				}
			}
			if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
				c.breaker.RecordFailure()
				return "", nil, lastErr
			}
			continue
		}

		var result ChatResponseFC
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			lastErr = fmt.Errorf("decode: %w", err)
			continue
		}
		resp.Body.Close()

		if len(result.Choices) == 0 {
			lastErr = fmt.Errorf("no choices returned")
			continue
		}

		c.breaker.RecordSuccess()
		choice := result.Choices[0]

		if choice.Message.ReasoningContent != "" && opts != nil {
			if opts.OnReasoning != nil {
				opts.OnReasoning(choice.Message.ReasoningContent)
			}
			if opts.LastReasoningOut != nil {
				*opts.LastReasoningOut = choice.Message.ReasoningContent
			}
		}

		if len(choice.Message.ToolCalls) > 0 {
			names := make([]string, len(choice.Message.ToolCalls))
			for i, tc := range choice.Message.ToolCalls {
				names[i] = tc.Function.Name
			}
			slog.Info("llm: tool_calls returned", "count", len(choice.Message.ToolCalls), "tools", names, "finish_reason", choice.FinishReason)
		} else {
			slog.Info("llm: no tool_calls, text reply", "finish_reason", choice.FinishReason, "content_len", len(choice.Message.Content), "content_head", truncateStr(choice.Message.Content, 120))
		}

		return choice.Message.Content, choice.Message.ToolCalls, nil
	}

	c.breaker.RecordFailure()
	return "", nil, lastErr
}

// ToolResultMessage creates a tool result message to feed back to the LLM.
func ToolResultMessage(toolCallID, content string) Message {
	return Message{
		Role:       "tool",
		Content:    content,
		ToolCallID: toolCallID,
	}
}

func truncateStr(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "..."
}
