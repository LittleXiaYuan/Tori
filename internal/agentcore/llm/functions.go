package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
		Role      string     `json:"role"`
		Content   string     `json:"content"`
		ToolCalls []ToolCall `json:"tool_calls,omitempty"`
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
func (c *Client) ChatWithTools(ctx context.Context, messages []Message, tools []FunctionDef, temperature float64, toolChoice ...string) (string, []ToolCall, error) {
	if err := c.breaker.Allow(); err != nil {
		return "", nil, err
	}
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
	}
	if len(toolChoice) > 0 && toolChoice[0] != "" && toolChoice[0] != ToolChoiceAuto {
		reqBody["tool_choice"] = toolChoice[0]
	}
	body, _ := json.Marshal(reqBody)

	url := c.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		c.breaker.RecordFailure()
		return "", nil, fmt.Errorf("chat with tools: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		c.breaker.RecordFailure()
		return "", nil, fmt.Errorf("chat API status %d", resp.StatusCode)
	}

	var result ChatResponseFC
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.breaker.RecordFailure()
		return "", nil, fmt.Errorf("decode: %w", err)
	}

	if len(result.Choices) == 0 {
		c.breaker.RecordFailure()
		return "", nil, fmt.Errorf("no choices returned")
	}

	c.breaker.RecordSuccess()
	choice := result.Choices[0]
	return choice.Message.Content, choice.Message.ToolCalls, nil
}

// ToolResultMessage creates a tool result message to feed back to the LLM.
func ToolResultMessage(toolCallID, content string) Message {
	return Message{
		Role:       "tool",
		Content:    content,
		ToolCallID: toolCallID,
	}
}
