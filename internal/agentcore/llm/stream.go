package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"yunque-agent/pkg/safego"
)

// StreamDelta is a single token chunk from the LLM.
type StreamDelta struct {
	Content  string `json:"content"`
	Finished bool   `json:"finished"`
}

// ChatStream sends a streaming request and returns a channel of deltas.
func (c *Client) ChatStream(ctx context.Context, messages []Message, temperature float64) (<-chan StreamDelta, error) {
	req := ChatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   4096,
	}
	body := struct {
		ChatRequest
		Stream bool `json:"stream"`
	}{req, true}

	b, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm stream request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("llm stream api %d", resp.StatusCode)
	}

	ch := make(chan StreamDelta, 64)
	safego.Go("llm-stream-reader", func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				ch <- StreamDelta{Finished: true}
				return
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0]
				if delta.Delta.Content != "" {
					ch <- StreamDelta{Content: delta.Delta.Content}
				}
				if delta.FinishReason != nil {
					ch <- StreamDelta{Finished: true}
					return
				}
			}
		}
	})

	return ch, nil
}
