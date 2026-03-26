package channel

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// EngagementProfile controls how the agent engages in group conversations.
type EngagementProfile struct {
	Mode              string        `json:"mode"`               // "active", "passive", "silent"
	HeartbeatEnabled  bool          `json:"heartbeat_enabled"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	ResponseRate      float64       `json:"response_rate"`      // 0.0 to 1.0
	GroupSystemPrompt string        `json:"group_system_prompt"` // extra system prompt for group context
	YAMLHeaders       bool          `json:"yaml_headers"`       // use YAML-formatted message headers
}

// LoadEngagementMode loads engagement settings from environment variables.
func LoadEngagementMode() EngagementProfile {
	mode := strings.ToLower(os.Getenv("ENGAGEMENT_MODE"))
	if mode == "" {
		mode = "active"
	}
	hbEnabled, _ := strconv.ParseBool(os.Getenv("GROUP_HEARTBEAT_ENABLED"))
	hbIntervalMin := 30
	if v := os.Getenv("GROUP_HEARTBEAT_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			hbIntervalMin = n
		}
	}
	rate := 0.3
	if v := os.Getenv("ENGAGEMENT_RESPONSE_RATE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			rate = f
		}
	}
	slog.Info("engagement profile loaded", "mode", mode, "heartbeat", hbEnabled, "interval_min", hbIntervalMin)
	return EngagementProfile{
		Mode:              mode,
		HeartbeatEnabled:  hbEnabled,
		HeartbeatInterval: time.Duration(hbIntervalMin) * time.Minute,
		ResponseRate:      rate,
	}
}
