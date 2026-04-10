package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// ── Anthropic Messages API types ──

type claudeRequest struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens"`
	System      string          `json:"system,omitempty"`
	Messages    []claudeMessage `json:"messages"`
	Stream      bool            `json:"stream"`
	Temperature float64         `json:"temperature,omitempty"`
	Tools       []claudeTool    `json:"tools,omitempty"`
}

type claudeMessage struct {
	Role    string        `json:"role"`
	Content any           `json:"content"` // string or []claudeContentBlock
}

type claudeContentBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	Source    *claudeImageSource `json:"source,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Input     any    `json:"input,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

type claudeImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type claudeTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"input_schema"`
}

type claudeResponse struct {
	ID      string                `json:"id"`
	Content []claudeContentBlock  `json:"content"`
	StopReason string             `json:"stop_reason"`
}

type claudeSSEEvent struct {
	Type  string          `json:"type"`
	Index int             `json:"index,omitempty"`
	Delta json.RawMessage `json:"delta,omitempty"`
	ContentBlock *claudeContentBlock `json:"content_block,omitempty"`
}

type claudeDelta struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// ── Conversion: OpenAI → Claude ──

func openAIToClaudeMessages(messages []Message) (system string, claudeMsgs []claudeMessage) {
	for _, m := range messages {
		if m.Role == "system" {
			system += m.Content + "\n"
			continue
		}

		role := m.Role
		if role == "tool" {
			role = "user"
		}

		if len(m.ContentParts) > 0 {
			var blocks []claudeContentBlock
			for _, p := range m.ContentParts {
				switch p.Type {
				case "text":
					blocks = append(blocks, claudeContentBlock{Type: "text", Text: p.Text})
				case "image_url":
					if p.ImageURL != nil {
						mediaType, data := parseDataURL(p.ImageURL.URL)
						if data != "" {
							blocks = append(blocks, claudeContentBlock{
								Type: "image",
								Source: &claudeImageSource{
									Type:      "base64",
									MediaType: mediaType,
									Data:      data,
								},
							})
						}
					}
				}
			}
			if len(blocks) > 0 {
				claudeMsgs = append(claudeMsgs, claudeMessage{Role: role, Content: blocks})
				continue
			}
		}

		if m.Role == "tool" && m.ToolCallID != "" {
			claudeMsgs = append(claudeMsgs, claudeMessage{
				Role: "user",
				Content: []claudeContentBlock{{
					Type:      "tool_result",
					ToolUseID: m.ToolCallID,
					Content:   m.Content,
				}},
			})
			continue
		}

		if len(m.ToolCalls) > 0 {
			var blocks []claudeContentBlock
			if m.Content != "" {
				blocks = append(blocks, claudeContentBlock{Type: "text", Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				var input any
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
				blocks = append(blocks, claudeContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				})
			}
			claudeMsgs = append(claudeMsgs, claudeMessage{Role: "assistant", Content: blocks})
			continue
		}

		claudeMsgs = append(claudeMsgs, claudeMessage{Role: role, Content: m.Content})
	}
	system = strings.TrimSpace(system)
	return
}

func parseDataURL(url string) (mediaType, data string) {
	if !strings.HasPrefix(url, "data:") {
		return "", ""
	}
	parts := strings.SplitN(url, ",", 2)
	if len(parts) != 2 {
		return "", ""
	}
	meta := strings.TrimPrefix(parts[0], "data:")
	meta = strings.TrimSuffix(meta, ";base64")
	return meta, parts[1]
}

func functionDefsToClaudeTools(defs []FunctionDef) []claudeTool {
	out := make([]claudeTool, len(defs))
	for i, d := range defs {
		out[i] = claudeTool{
			Name:        d.Name,
			Description: d.Description,
			InputSchema: d.Parameters,
		}
	}
	return out
}

// ── Client methods for Anthropic dialect ──

func (c *Client) chatOnceAnthropic(ctx context.Context, messages []Message, temperature float64) (string, error) {
	r, err := c.chatOnceAnthropicFull(ctx, messages, temperature)
	return r.Content, err
}

func (c *Client) chatOnceAnthropicFull(ctx context.Context, messages []Message, temperature float64, onDelta ...StreamDeltaFunc) (ChatResult, error) {
	system, cMsgs := openAIToClaudeMessages(messages)

	req := claudeRequest{
		Model:       c.model,
		MaxTokens:   4096,
		System:      system,
		Messages:    cMsgs,
		Stream:      true,
		Temperature: temperature,
	}
	b, _ := json.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/messages", bytes.NewReader(b))
	if err != nil {
		return ChatResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return ChatResult{}, fmt.Errorf("claude request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return ChatResult{}, fmt.Errorf("claude api %d: %.500s", resp.StatusCode, string(body))
	}

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		return c.readClaudeSSE(resp.Body, onDelta...)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ChatResult{}, fmt.Errorf("read claude response: %w", err)
	}
	var cResp claudeResponse
	if err := json.Unmarshal(body, &cResp); err != nil {
		return ChatResult{}, fmt.Errorf("decode claude response: %w", err)
	}

	var content, thinking string
	for _, block := range cResp.Content {
		switch block.Type {
		case "text":
			content += block.Text
		case "thinking":
			thinking += block.Text
		}
	}
	if content == "" && thinking != "" {
		content = thinking
	}
	return ChatResult{Content: content, ReasoningContent: thinking}, nil
}

func (c *Client) readClaudeSSE(body io.Reader, onDelta ...StreamDeltaFunc) (ChatResult, error) {
	var cb StreamDeltaFunc
	if len(onDelta) > 0 {
		cb = onDelta[0]
	}
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var content, thinking strings.Builder
	var currentBlockType string

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			eventType := strings.TrimPrefix(line, "event: ")
			switch eventType {
			case "content_block_start":
			case "content_block_delta":
			case "content_block_stop":
				currentBlockType = ""
			case "message_stop":
				break
			}
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var evt struct {
			Type         string `json:"type"`
			ContentBlock *struct {
				Type string `json:"type"`
			} `json:"content_block,omitempty"`
			Delta *struct {
				Type string `json:"type"`
				Text string `json:"text,omitempty"`
			} `json:"delta,omitempty"`
		}
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			continue
		}

		switch evt.Type {
		case "content_block_start":
			if evt.ContentBlock != nil {
				currentBlockType = evt.ContentBlock.Type
			}
		case "content_block_delta":
			if evt.Delta != nil {
				isThinking := currentBlockType == "thinking" || evt.Delta.Type == "thinking_delta"
				if isThinking {
					thinking.WriteString(evt.Delta.Text)
					if cb != nil && evt.Delta.Text != "" {
						cb("", evt.Delta.Text)
					}
				} else {
					content.WriteString(evt.Delta.Text)
					if cb != nil && evt.Delta.Text != "" {
						cb(evt.Delta.Text, "")
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return ChatResult{Content: content.String(), ReasoningContent: thinking.String()}, fmt.Errorf("claude sse read: %w", err)
	}

	result := ChatResult{Content: content.String(), ReasoningContent: thinking.String()}
	if result.Content == "" && result.ReasoningContent != "" {
		result.Content = result.ReasoningContent
	}
	if result.Content == "" {
		return result, fmt.Errorf("claude: empty response")
	}
	return result, nil
}

func (c *Client) chatWithToolsAnthropic(ctx context.Context, messages []Message, tools []FunctionDef, temperature float64) (string, []ToolCall, error) {
	system, cMsgs := openAIToClaudeMessages(messages)

	req := claudeRequest{
		Model:       c.model,
		MaxTokens:   4096,
		System:      system,
		Messages:    cMsgs,
		Stream:      false,
		Temperature: temperature,
		Tools:       functionDefsToClaudeTools(tools),
	}
	b, _ := json.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/messages", bytes.NewReader(b))
	if err != nil {
		return "", nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", nil, fmt.Errorf("claude fc request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("claude fc api %d: %.500s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("read claude fc response: %w", err)
	}

	var cResp claudeResponse
	if err := json.Unmarshal(body, &cResp); err != nil {
		return "", nil, fmt.Errorf("decode claude fc response: %w", err)
	}

	var textContent string
	var toolCalls []ToolCall
	for _, block := range cResp.Content {
		switch block.Type {
		case "text":
			textContent += block.Text
		case "tool_use":
			argsJSON, _ := json.Marshal(block.Input)
			tc := ToolCall{
				ID:   block.ID,
				Type: "function",
			}
			tc.Function.Name = block.Name
			tc.Function.Arguments = string(argsJSON)
			toolCalls = append(toolCalls, tc)
		}
	}

	slog.Info("claude fc response", "text_len", len(textContent), "tool_calls", len(toolCalls), "stop_reason", cResp.StopReason)
	return textContent, toolCalls, nil
}
