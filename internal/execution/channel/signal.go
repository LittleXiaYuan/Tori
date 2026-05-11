package channel

// ─── Channel: Signal ────────────────────────────────────────
// Type:     "signal"
// Protocol: CLI守护进程 (signal-cli daemon --json)
// Inbound:  text (仅文本消息)
// Outbound: text (仅纯文本，无附件)
// Env vars: SIGNAL_PHONE_NUMBER, SIGNAL_CONFIG_DIR
// Status:   Stub — 依赖本机 signal-cli 守护进程，运维成本高
//
// TODO: [P2] 支持发送附件 (signal-cli send -a <file>)
// TODO: [P2] 支持接收附件消息 (attachments in DataMessage)
// TODO: [P3] 支持 Signal Reactions (sendReaction)
// TODO: [P3] 处理消息自毁 (disappearing messages)
// TODO: [P3] 实现 Reactor 接口 (消息表情回应)
// ─────────────────────────────────────────────────────────────

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
)

// Signal implements the Channel interface using signal-cli (JSON-RPC mode).
// Requires signal-cli to be installed and registered with a phone number.
type Signal struct {
	number    string // registered phone number (e.g. "+1234567890")
	configDir string // signal-cli config directory (optional)
	mu        sync.Mutex
}

// SignalConfig holds configuration for the Signal channel.
type SignalConfig struct {
	PhoneNumber string `json:"phone_number"` // e.g. "+1234567890"
	ConfigDir   string `json:"config_dir,omitempty"`
}

// NewSignal creates a Signal channel.
func NewSignal(cfg SignalConfig) *Signal {
	return &Signal{
		number:    cfg.PhoneNumber,
		configDir: cfg.ConfigDir,
	}
}

func (s *Signal) Type() string { return "signal" }

// Start listens for incoming Signal messages via signal-cli daemon (blocking).
func (s *Signal) Start(ctx context.Context, handler func(Message) Reply) error {
	args := []string{"--output=json", "-u", s.number, "daemon", "--json"}
	if s.configDir != "" {
		args = append([]string{"--config", s.configDir}, args...)
	}

	cmd := exec.CommandContext(ctx, "signal-cli", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("signal: stdout pipe: %w", err)
	}
	cmd.Stderr = nil // suppress stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("signal: start signal-cli: %w", err)
	}

	slog.Info("signal: daemon started", "number", s.number)

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var envelope signalEnvelope
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			continue
		}

		if envelope.Envelope.DataMessage == nil {
			continue
		}
		dm := envelope.Envelope.DataMessage
		if dm.Message == "" {
			continue
		}

		sender := envelope.Envelope.Source
		groupID := ""
		channelID := sender
		if dm.GroupInfo != nil {
			groupID = dm.GroupInfo.GroupID
			channelID = groupID
		}

		msg := Message{
			ChannelType: "signal",
			ChannelID:   channelID,
			UserID:      sender,
			UserName:    envelope.Envelope.SourceName,
			Content:     dm.Message,
			Extra: map[string]string{
				"timestamp": fmt.Sprintf("%d", envelope.Envelope.Timestamp),
			},
		}
		if groupID != "" {
			msg.Extra["group_id"] = groupID
		}

		reply := handler(msg)
		if !IsEmptyReply(reply) {
			target := sender
			if groupID != "" {
				target = groupID
			}
			if err := s.Send(ctx, target, reply); err != nil {
				slog.Warn("signal: reply failed", "to", target, "err", err)
			}
		}
	}

	return cmd.Wait()
}

// Send sends a message via signal-cli.
func (s *Signal) Send(ctx context.Context, target string, reply Reply) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	args := []string{"-u", s.number}
	if s.configDir != "" {
		args = append([]string{"--config", s.configDir}, args...)
	}

	text := ContentWithButtonFallback(reply)
	// Determine if target is a group or individual
	if strings.HasPrefix(target, "+") {
		args = append(args, "send", "-m", text, target)
	} else {
		args = append(args, "send", "-m", text, "-g", target)
	}

	cmd := exec.CommandContext(ctx, "signal-cli", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("signal: send failed: %w: %s", err, string(output))
	}
	return nil
}

// ──────────────────────────────────────────────
// signal-cli JSON types
// ──────────────────────────────────────────────

type signalEnvelope struct {
	Envelope struct {
		Source      string             `json:"source"`
		SourceName  string             `json:"sourceName"`
		Timestamp   int64              `json:"timestamp"`
		DataMessage *signalDataMessage `json:"dataMessage"`
	} `json:"envelope"`
}

type signalDataMessage struct {
	Timestamp int64            `json:"timestamp"`
	Message   string           `json:"message"`
	GroupInfo *signalGroupInfo `json:"groupInfo"`
}

type signalGroupInfo struct {
	GroupID string `json:"groupId"`
	Type    string `json:"type"`
}

// ListGroups returns all Signal groups the bot is a member of via signal-cli.
func (s *Signal) ListGroups(_ context.Context) ([]GroupInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	args := []string{"-u", s.number, "--output=json", "listGroups"}
	if s.configDir != "" {
		args = append([]string{"--config", s.configDir}, args...)
	}

	cmd := exec.Command("signal-cli", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("signal listGroups: %w", err)
	}

	var groups []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(output, &groups); err != nil {
		return nil, fmt.Errorf("signal parse groups: %w", err)
	}

	out := make([]GroupInfo, 0, len(groups))
	for _, g := range groups {
		out = append(out, GroupInfo{
			ID:          g.ID,
			Name:        g.Name,
			ChannelType: "signal",
			ChatType:    "group",
		})
	}
	return out, nil
}
