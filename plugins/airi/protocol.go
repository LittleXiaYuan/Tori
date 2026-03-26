package airi

import (
	"encoding/json"
	"fmt"
	"time"
)

// ─── Airi Protocol Types ───
// These mirror the TypeScript types in @proj-airi/plugin-protocol/types/events.ts
// but only include what the Yunque bridge needs.

// AiriEvent is the top-level WebSocket message envelope used by Airi's server-runtime.
type AiriEvent struct {
	Type     string           `json:"type"`
	Data     json.RawMessage  `json:"data"`
	Metadata *EventMetadata   `json:"metadata,omitempty"`
	Route    *RouteConfig     `json:"route,omitempty"`
}

// RouteConfig controls event routing on the server-runtime.
type RouteConfig struct {
	Bypass       bool     `json:"bypass,omitempty"`
	Destinations []string `json:"destinations,omitempty"`
}

// EventMetadata carries source identity and event correlation IDs.
type EventMetadata struct {
	Source *ModuleIdentity `json:"source,omitempty"`
	Event  *EventID        `json:"event,omitempty"`
}

// EventID is used for event correlation / tracing.
type EventID struct {
	ID       string `json:"id"`
	ParentID string `json:"parentId,omitempty"`
}

// ModuleIdentity identifies a module instance in Airi's plugin system.
type ModuleIdentity struct {
	ID     string          `json:"id"`
	Kind   string          `json:"kind"` // always "plugin"
	Plugin *PluginIdentity `json:"plugin"`
	Labels map[string]string `json:"labels,omitempty"`
}

// PluginIdentity is the stable plugin identifier.
type PluginIdentity struct {
	ID      string            `json:"id"`
	Version string            `json:"version,omitempty"`
	Labels  map[string]string `json:"labels,omitempty"`
}

// ─── Event Data Types ───

// AuthenticateData is the payload for "module:authenticate".
type AuthenticateData struct {
	Token string `json:"token"`
}

// AuthenticatedData is the payload for "module:authenticated".
type AuthenticatedData struct {
	Authenticated bool `json:"authenticated"`
}

// AnnounceData is the payload for "module:announce".
type AnnounceData struct {
	Name           string          `json:"name"`
	Identity       ModuleIdentity  `json:"identity"`
	PossibleEvents []string        `json:"possibleEvents"`
}

// AnnouncedData is the payload for "module:announced".
type AnnouncedData struct {
	Name     string          `json:"name"`
	Index    *int            `json:"index,omitempty"`
	Identity ModuleIdentity  `json:"identity"`
}

// HeartbeatData is the payload for "transport:connection:heartbeat".
type HeartbeatData struct {
	Kind    string `json:"kind"`    // "ping" or "pong"
	Message string `json:"message"` // e.g. "🩵" or "💛"
	At      int64  `json:"at,omitempty"`
}

// InputTextData is the payload for "input:text".
type InputTextData struct {
	Text     string `json:"text"`
	TextRaw  string `json:"textRaw,omitempty"`
}

// ErrorData is the payload for "error" events.
type ErrorData struct {
	Message string `json:"message"`
}

// RegistryModulesSyncData is the payload for "registry:modules:sync".
type RegistryModulesSyncData struct {
	Modules []struct {
		Name     string          `json:"name"`
		Index    *int            `json:"index,omitempty"`
		Identity ModuleIdentity  `json:"identity"`
	} `json:"modules"`
}

// ─── Constructors ───

var moduleVersion = "1.0.0"

// NewModuleIdentity creates a ModuleIdentity for the Yunque bridge module.
func NewModuleIdentity(instanceID string) ModuleIdentity {
	return ModuleIdentity{
		ID:   instanceID,
		Kind: "plugin",
		Plugin: &PluginIdentity{
			ID:      "yunque-agent",
			Version: moduleVersion,
		},
	}
}

// NewMetadata creates EventMetadata with a fresh event ID.
func NewMetadata(identity ModuleIdentity, parentID string) *EventMetadata {
	meta := &EventMetadata{
		Source: &identity,
		Event: &EventID{
			ID: fmt.Sprintf("yq-%d", time.Now().UnixNano()),
		},
	}
	if parentID != "" {
		meta.Event.ParentID = parentID
	}
	return meta
}

// ─── Event Builders ───

// NewAuthenticateEvent creates a "module:authenticate" event.
func NewAuthenticateEvent(token string, identity ModuleIdentity) AiriEvent {
	data, _ := json.Marshal(AuthenticateData{Token: token})
	return AiriEvent{
		Type:     "module:authenticate",
		Data:     data,
		Metadata: NewMetadata(identity, ""),
	}
}

// NewAnnounceEvent creates a "module:announce" event.
func NewAnnounceEvent(moduleName string, identity ModuleIdentity) AiriEvent {
	data, _ := json.Marshal(AnnounceData{
		Name:     moduleName,
		Identity: identity,
		PossibleEvents: []string{
			"input:text",
			"transport:connection:heartbeat",
		},
	})
	return AiriEvent{
		Type:     "module:announce",
		Data:     data,
		Metadata: NewMetadata(identity, ""),
	}
}

// NewHeartbeatPingEvent creates a heartbeat ping event.
func NewHeartbeatPingEvent(identity ModuleIdentity) AiriEvent {
	data, _ := json.Marshal(HeartbeatData{
		Kind:    "ping",
		Message: "🩵",
		At:      time.Now().UnixMilli(),
	})
	return AiriEvent{
		Type:     "transport:connection:heartbeat",
		Data:     data,
		Metadata: NewMetadata(identity, ""),
	}
}

// NewInputTextEvent creates an "input:text" event to send text to Airi.
func NewInputTextEvent(text string, identity ModuleIdentity) AiriEvent {
	data, _ := json.Marshal(InputTextData{
		Text: text,
	})
	return AiriEvent{
		Type:     "input:text",
		Data:     data,
		Metadata: NewMetadata(identity, ""),
	}
}

// ─── Parse Helpers ───

// ParseEventType extracts just the type field from a raw JSON message.
func ParseEventType(raw []byte) (string, error) {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", err
	}
	return envelope.Type, nil
}

// ParseEvent parses a full AiriEvent from raw JSON.
func ParseEvent(raw []byte) (*AiriEvent, error) {
	var event AiriEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// ParseData extracts the typed data from an AiriEvent.
func ParseData[T any](event *AiriEvent) (*T, error) {
	var data T
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return nil, err
	}
	return &data, nil
}
