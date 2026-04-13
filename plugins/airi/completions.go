package airi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/planner"
)

var actTagRe = regexp.MustCompile(`<\|ACT\s*\{[^}]*"name"\s*:\s*"(\w+)"[^}]*\}\|>`)

// OpenAIRequest represents the structure of an incoming OpenAI API call from Airi.
type OpenAIRequest struct {
	Model    string          `json:"model"`
	Messages []OpenAIMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// handleChatCompletions processes Airi messages using Yunque's Planner.
func (p *Plugin) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var req OpenAIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	// Map Airi messages to Yunque LLM messages
	var msgs []llm.Message
	for _, m := range req.Messages {
		msgs = append(msgs, llm.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// Inject Airi-specific system prompt so the LLM knows it's driving a Live2D character
	airiSystemPrompt := llm.Message{
		Role:    "system",
		Content: p.airiSystemPrompt(),
	}
	msgs = append([]llm.Message{airiSystemPrompt}, msgs...)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	planReq := planner.PlanRequest{
		Messages:          msgs,
		TenantID:          "default",
		DisableTools:      true,
		DisableDelegation: true,
	}

	result, err := p.app.Planner.Run(ctx, planReq)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "planner error: " + err.Error()})
		return
	}

	reply := result.Reply
	var finalText string
	if m := actTagRe.FindString(reply); m != "" {
		finalText = reply
	} else {
		finalText = fmt.Sprintf("<|ACT {\"emotion\":{\"name\":\"neutral\",\"intensity\":1}}|>\n%s", reply)
	}

	if req.Stream {
		p.streamResponse(w, finalText, req.Model)
	} else {
		p.batchResponse(w, finalText, req.Model)
	}
}

// streamResponse chunks the final text and sends it as SSE (Server Sent Events).
func (p *Plugin) streamResponse(w http.ResponseWriter, content string, model string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	chunkID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())

	sendChunk := func(text string) {
		chunk := map[string]any{
			"id":      chunkID,
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   model,
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]string{
						"content": text,
					},
					"finish_reason": nil,
				},
			},
		}
		b, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", string(b))
		flusher.Flush()
	}

	sendChunk(content)

	// End of stream
	final := map[string]any{
		"id":      chunkID,
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]any{
			{
				"index":         0,
				"delta":         map[string]string{},
				"finish_reason": "stop",
			},
		},
	}
	b, _ := json.Marshal(final)
	fmt.Fprintf(w, "data: %s\n\n", string(b))
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// batchResponse returns the text in a single JSON payload.
func (p *Plugin) batchResponse(w http.ResponseWriter, content string, model string) {
	resp := map[string]any{
		"id":      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []map[string]any{
			{
				"index":         0,
				"message":       map[string]string{"role": "assistant", "content": content},
				"finish_reason": "stop",
			},
		},
	}
	writeJSON(w, http.StatusOK, resp)
}
