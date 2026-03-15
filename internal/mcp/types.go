package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Tool describes an MCP tool with its name, description, and input schema.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema"`
}

// CallRequest represents a tool invocation.
type CallRequest struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// CallResult is the outcome of a tool execution.
type CallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents one piece of tool output.
type ContentBlock struct {
	Type string `json:"type"` // "text", "image", "resource"
	Text string `json:"text,omitempty"`
}

// Provider supplies tools and handles their execution.
type Provider interface {
	ListTools(ctx context.Context) ([]Tool, error)
	CallTool(ctx context.Context, name string, args map[string]any) (*CallResult, error)
}

// SuccessResult creates a text-based success result.
func SuccessResult(data any) *CallResult {
	text := "ok"
	if data != nil {
		switch v := data.(type) {
		case string:
			text = v
		default:
			b, err := json.Marshal(v)
			if err == nil {
				text = string(b)
			}
		}
	}
	return &CallResult{
		Content: []ContentBlock{{Type: "text", Text: text}},
	}
}

// ErrorResult creates an error result.
func ErrorResult(msg string) *CallResult {
	if strings.TrimSpace(msg) == "" {
		msg = "tool execution failed"
	}
	return &CallResult{
		IsError: true,
		Content: []ContentBlock{{Type: "text", Text: msg}},
	}
}

// ErrToolNotFound indicates the tool was not found in any provider.
var ErrToolNotFound = fmt.Errorf("tool not found")

// StringArg extracts a string argument safely.
func StringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	v, ok := args[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprintf("%v", v))
}

// IntArg extracts an integer argument safely.
func IntArg(args map[string]any, key string, defaultVal int) int {
	if args == nil {
		return defaultVal
	}
	v, ok := args[key]
	if !ok {
		return defaultVal
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, err := n.Int64()
		if err == nil {
			return int(i)
		}
	}
	return defaultVal
}
