package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/llm"
)

// FileRepo persists sessions as JSON files on disk.
// Each session is stored as data/sessions/<sessionID>.json.
type FileRepo struct {
	mu  sync.Mutex
	dir string
}

// NewFileRepo creates a file-based session repository.
func NewFileRepo(dir string) (*FileRepo, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("session file repo: mkdir %s: %w", dir, err)
	}
	return &FileRepo{dir: dir}, nil
}

type sessionFile struct {
	ID         string        `json:"id"`
	TenantID   string        `json:"tenant_id"`
	Name       string        `json:"name,omitempty"`
	Summary    string        `json:"summary,omitempty"`
	Pinned     bool          `json:"pinned,omitempty"`
	ArchivedAt *time.Time    `json:"archived_at,omitempty"`
	Messages   []llm.Message `json:"messages"`
}

func (r *FileRepo) path(sessionID string) string {
	return filepath.Join(r.dir, sessionID+".json")
}

func (r *FileRepo) load(sessionID string) (*sessionFile, error) {
	data, err := os.ReadFile(r.path(sessionID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var sf sessionFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, err
	}
	return &sf, nil
}

func (r *FileRepo) save(sf *sessionFile) error {
	data, err := json.Marshal(sf)
	if err != nil {
		return err
	}
	return os.WriteFile(r.path(sf.ID), data, 0644)
}

// GetOrCreate ensures a session file exists.
func (r *FileRepo) GetOrCreate(_ context.Context, sessionID, tenantID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	sf, err := r.load(sessionID)
	if err != nil {
		return err
	}
	if sf == nil {
		return r.save(&sessionFile{ID: sessionID, TenantID: tenantID})
	}
	return nil
}

// Append adds a message to the session file.
func (r *FileRepo) Append(_ context.Context, sessionID, role, content string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	sf, err := r.load(sessionID)
	if err != nil {
		return err
	}
	if sf == nil {
		sf = &sessionFile{ID: sessionID}
	}
	sf.Messages = append(sf.Messages, llm.Message{Role: role, Content: content})
	return r.save(sf)
}

// GetMessages returns the last N messages from the session file.
func (r *FileRepo) GetMessages(_ context.Context, sessionID string, limit int) ([]llm.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sf, err := r.load(sessionID)
	if err != nil {
		return nil, err
	}
	if sf == nil {
		return nil, nil
	}
	msgs := sf.Messages
	if limit > 0 && len(msgs) > limit {
		msgs = msgs[len(msgs)-limit:]
	}
	return msgs, nil
}

// Delete removes a session file.
func (r *FileRepo) Delete(_ context.Context, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	err := os.Remove(r.path(sessionID))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// ListByTenant returns all sessions for a tenant.
func (r *FileRepo) ListByTenant(_ context.Context, tenantID string) ([]Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}
	var out []Session
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		sid := e.Name()[:len(e.Name())-5]
		sf, err := r.load(sid)
		if err != nil || sf == nil {
			continue
		}
		if tenantID != "" && sf.TenantID != tenantID {
			continue
		}
		out = append(out, Session{ID: sf.ID, TenantID: sf.TenantID, Name: sf.Name, Summary: sf.Summary, Pinned: sf.Pinned, ArchivedAt: sf.ArchivedAt})
	}
	return out, nil
}

// SaveMeta updates session metadata in the file without changing messages.
func (r *FileRepo) SaveMeta(_ context.Context, sess *Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	sf, err := r.load(sess.ID)
	if err != nil {
		return err
	}
	if sf == nil {
		sf = &sessionFile{ID: sess.ID, TenantID: sess.TenantID}
	}
	sf.Name = sess.Name
	sf.Summary = sess.Summary
	sf.Pinned = sess.Pinned
	sf.ArchivedAt = sess.ArchivedAt
	return r.save(sf)
}
