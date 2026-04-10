package browserskill

import (
	"context"
	"encoding/json"
)

// HubAdapter wraps a BrowserHub to satisfy BrowserController.
type HubAdapter struct {
	hub interface {
		Connected() bool
		SendActionRaw(ctx context.Context, action json.RawMessage) (json.RawMessage, error)
	}
}

// NewHubAdapter creates a HubAdapter from an object implementing the required methods.
func NewHubAdapter(hub interface {
	Connected() bool
	SendActionRaw(ctx context.Context, action json.RawMessage) (json.RawMessage, error)
}) BrowserController {
	return &HubAdapter{hub: hub}
}

func (a *HubAdapter) Connected() bool {
	return a.hub.Connected()
}

func (a *HubAdapter) SendAction(ctx context.Context, action any) (any, error) {
	data, err := json.Marshal(action)
	if err != nil {
		return nil, err
	}
	resultData, err := a.hub.SendActionRaw(ctx, data)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	_ = json.Unmarshal(resultData, &result)
	return result, nil
}
