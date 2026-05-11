package notify

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ShareBinding struct {
	Code        string    `json:"code"`
	ChannelID   string    `json:"channel_id"`
	ChannelType string    `json:"channel_type"`
	ChannelName string    `json:"channel_name"`
	SessionID   string    `json:"session_id"`
	TaskID      string    `json:"task_id,omitempty"`
	Title       string    `json:"title"`
	CreatedAt   time.Time `json:"created_at"`
	LastUsedAt  time.Time `json:"last_used_at,omitempty"`
}

func (n *Notifier) CreateShareBinding(ch *Channel, sessionID, taskID, title string) (*ShareBinding, error) {
	if ch == nil {
		return nil, fmt.Errorf("channel required")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(taskID)
	}
	if sessionID == "" {
		return nil, fmt.Errorf("session_id required")
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "云雀协作同步"
	}
	now := time.Now()
	for i := 0; i < 8; i++ {
		code, err := newShareCode()
		if err != nil {
			return nil, err
		}
		binding := &ShareBinding{
			Code:        code,
			ChannelID:   ch.ID,
			ChannelType: ch.Type,
			ChannelName: ch.Name,
			SessionID:   sessionID,
			TaskID:      strings.TrimSpace(taskID),
			Title:       title,
			CreatedAt:   now,
		}
		n.mu.Lock()
		if n.shares == nil {
			n.shares = make(map[string]*ShareBinding)
		}
		if _, exists := n.shares[code]; exists {
			n.mu.Unlock()
			continue
		}
		n.shares[code] = binding
		n.saveSharesLocked()
		n.mu.Unlock()
		copyBinding := *binding
		return &copyBinding, nil
	}
	return nil, fmt.Errorf("failed to allocate share code")
}

func (n *Notifier) GetShareBinding(code string) (*ShareBinding, bool) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, false
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	binding, ok := n.shares[code]
	if !ok || binding == nil {
		return nil, false
	}
	copyBinding := *binding
	return &copyBinding, true
}

func (n *Notifier) TouchShareBinding(code string) {
	code = strings.TrimSpace(code)
	if code == "" {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if binding, ok := n.shares[code]; ok && binding != nil {
		binding.LastUsedAt = time.Now()
		n.saveSharesLocked()
	}
}

func newShareCode() (string, error) {
	var b [5]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "yq_" + hex.EncodeToString(b[:]), nil
}

func (n *Notifier) loadShares() {
	data, err := os.ReadFile(n.shareStore)
	if err != nil {
		return
	}
	var shares []*ShareBinding
	if err := json.Unmarshal(data, &shares); err != nil {
		slog.Warn("notify: failed to load share bindings", "err", err)
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, binding := range shares {
		if binding != nil && binding.Code != "" {
			n.shares[binding.Code] = binding
		}
	}
}

func (n *Notifier) saveSharesLocked() {
	if n.shareStore == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(n.shareStore), 0o755); err != nil {
		slog.Warn("notify: failed to create share store dir", "err", err)
		return
	}
	shares := make([]*ShareBinding, 0, len(n.shares))
	for _, binding := range n.shares {
		copyBinding := *binding
		shares = append(shares, &copyBinding)
	}
	data, err := json.MarshalIndent(shares, "", "  ")
	if err != nil {
		slog.Warn("notify: failed to encode share bindings", "err", err)
		return
	}
	if err := os.WriteFile(n.shareStore, data, 0o644); err != nil {
		slog.Warn("notify: failed to save share bindings", "err", err)
	}
}
