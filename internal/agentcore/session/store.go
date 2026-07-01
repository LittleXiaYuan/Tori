package session

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/pkg/safego"
)

// Session holds a conversation's message history.
type Session struct {
	ID         string        `json:"id"`
	TenantID   string        `json:"tenant_id"`
	Name       string        `json:"name,omitempty"`
	Summary    string        `json:"summary,omitempty"`
	Pinned     bool          `json:"pinned,omitempty"`
	ArchivedAt *time.Time    `json:"archived_at,omitempty"`
	Messages   []llm.Message `json:"messages"`
	Files      []SessionFile `json:"files,omitempty"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

// SessionFile is a file the user uploaded or a skill generated earlier in
// this conversation. Tracking it here — in memory, alongside the message
// history — lets later turns reference it (e.g. "edit the doc you just made")
// without the user re-uploading or re-typing the path.
type SessionFile struct {
	Path    string    `json:"path"`
	Name    string    `json:"name"`
	Kind    string    `json:"kind"` // "uploaded" | "generated"
	Skill   string    `json:"skill,omitempty"`
	Size    int64     `json:"size,omitempty"`
	AddedAt time.Time `json:"added_at"`
}

// maxSessionFiles caps how many distinct files are remembered per session,
// evicting the oldest once the limit is exceeded. Prevents unbounded growth
// in long-running conversations that generate many files.
const maxSessionFiles = 40

// Repo is the optional persistence backend for sessions.
type Repo interface {
	GetOrCreate(ctx context.Context, sessionID, tenantID string) error
	Append(ctx context.Context, sessionID, role, content string) error
	GetMessages(ctx context.Context, sessionID string, limit int) ([]llm.Message, error)
	Delete(ctx context.Context, sessionID string) error
	ListByTenant(ctx context.Context, tenantID string) ([]Session, error)
}

// Store manages conversation sessions in memory with optional DB persistence.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session // sessionID -> Session
	maxMsgs  int
	ttl      time.Duration
	repo     Repo // optional DB backend
	stopGC   chan struct{}
}

// NewStore creates a conversation store with max messages per session.
// Sessions idle for more than 2 hours are automatically cleaned up.
func NewStore(maxMessages int) *Store {
	if maxMessages <= 0 {
		maxMessages = 50
	}
	s := &Store{
		sessions: make(map[string]*Session),
		maxMsgs:  maxMessages,
		ttl:      24 * time.Hour,
		stopGC:   make(chan struct{}),
	}
	go s.gcLoop()
	return s
}

func (s *Store) gcLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.evictExpired()
		case <-s.stopGC:
			return
		}
	}
}

func (s *Store) evictExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-s.ttl)
	for id, sess := range s.sessions {
		if sess.Pinned {
			continue
		}
		if sess.UpdatedAt.Before(cutoff) && len(sess.Messages) == 0 {
			delete(s.sessions, id)
		}
	}
}

// StopGC stops the background session cleanup goroutine.
func (s *Store) StopGC() {
	select {
	case s.stopGC <- struct{}{}:
	default:
	}
}

// SetRepo attaches a persistence backend.
func (s *Store) SetRepo(repo Repo) {
	s.repo = repo
}

// LoadFromRepo restores sessions from the persistence backend into memory.
// Should be called once after SetRepo during startup.
func (s *Store) LoadFromRepo(tenantID string) int {
	if s.repo == nil {
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sessions, err := s.repo.ListByTenant(ctx, tenantID)
	if err != nil {
		slog.Error("session store: failed to load from repo", "err", err)
		return 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	loaded := 0
	for _, sess := range sessions {
		if _, exists := s.sessions[sess.ID]; exists {
			continue
		}
		restored := &Session{
			ID:         sess.ID,
			TenantID:   sess.TenantID,
			Name:       sess.Name,
			Summary:    sess.Summary,
			Pinned:     sess.Pinned,
			ArchivedAt: sess.ArchivedAt,
			CreatedAt:  sess.CreatedAt,
			UpdatedAt:  sess.UpdatedAt,
		}
		if msgs, err := s.repo.GetMessages(ctx, sess.ID, s.maxMsgs); err == nil {
			restored.Messages = msgs
		}
		s.sessions[sess.ID] = restored
		loaded++
	}
	return loaded
}

// GetOrCreate returns an existing session or creates a new one.
func (s *Store) GetOrCreate(sessionID, tenantID string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[sessionID]; ok {
		return sess
	}
	sess := &Session{
		ID:        sessionID,
		TenantID:  tenantID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.sessions[sessionID] = sess

	// Persist + load history from DB
	if s.repo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.repo.GetOrCreate(ctx, sessionID, tenantID); err != nil {
			slog.Error("session repo GetOrCreate", "err", err)
		}
		if msgs, err := s.repo.GetMessages(ctx, sessionID, s.maxMsgs); err == nil && len(msgs) > 0 {
			sess.Messages = msgs
		}
	}
	return sess
}

// Append adds messages to a session, trimming old ones if needed.
func (s *Store) Append(sessionID string, msgs ...llm.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return
	}
	sess.Messages = append(sess.Messages, msgs...)
	sess.UpdatedAt = time.Now()

	// Persist each message to DB (async — don't block the request path)
	if s.repo != nil {
		persistMsgs := make([]llm.Message, len(msgs))
		copy(persistMsgs, msgs)
		sid := sessionID
		safego.Go("session-persist", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			for _, m := range persistMsgs {
				if err := s.repo.Append(ctx, sid, m.Role, m.Content); err != nil {
					slog.Error("session repo Append", "err", err)
				}
			}
		})
	}

	// Trim: keep system message + last N messages
	if len(sess.Messages) > s.maxMsgs {
		start := 0
		if len(sess.Messages) > 0 && sess.Messages[0].Role == "system" {
			start = 1
		}
		excess := len(sess.Messages) - s.maxMsgs
		if excess > 0 && excess < len(sess.Messages)-start {
			if start == 1 {
				sess.Messages = append(sess.Messages[:1], sess.Messages[1+excess:]...)
			} else {
				sess.Messages = sess.Messages[excess:]
			}
		}
	}
}

// AddFiles registers files uploaded or generated during a session, keyed by
// path (re-adding an existing path refreshes its entry — e.g. a doc that was
// edited again — rather than duplicating it). Does nothing for an unknown
// session (mirrors Append's no-op-if-missing behavior).
func (s *Store) AddFiles(sessionID string, files ...SessionFile) {
	if len(files) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return
	}
	for _, f := range files {
		if f.Path == "" {
			continue
		}
		replaced := false
		for i, existing := range sess.Files {
			if existing.Path == f.Path {
				sess.Files[i] = f
				replaced = true
				break
			}
		}
		if !replaced {
			sess.Files = append(sess.Files, f)
		}
	}
	if excess := len(sess.Files) - maxSessionFiles; excess > 0 {
		sess.Files = sess.Files[excess:]
	}
	sess.UpdatedAt = time.Now()
}

// Files returns a copy of the files registered against a session.
func (s *Store) Files(sessionID string) []SessionFile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil
	}
	out := make([]SessionFile, len(sess.Files))
	copy(out, sess.Files)
	return out
}

// Get returns a session's messages, rehydrating from the file repo if needed.
func (s *Store) Get(sessionID string) []llm.Message {
	s.mu.RLock()
	sess, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	if ok {
		s.mu.RLock()
		defer s.mu.RUnlock()
		out := make([]llm.Message, len(sess.Messages))
		copy(out, sess.Messages)
		return out
	}
	if s.repo == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	msgs, err := s.repo.GetMessages(ctx, sessionID, s.maxMsgs)
	if err != nil || len(msgs) == 0 {
		return nil
	}
	s.mu.Lock()
	if existing, ok := s.sessions[sessionID]; ok {
		s.mu.Unlock()
		return existing.Messages
	}
	restored := &Session{
		ID:        sessionID,
		Messages:  msgs,
		UpdatedAt: time.Now(),
	}
	s.sessions[sessionID] = restored
	s.mu.Unlock()
	return msgs
}

// Delete removes a session from memory and the persistence backend.
func (s *Store) Delete(sessionID string) {
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.mu.Unlock()
	if s.repo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.repo.Delete(ctx, sessionID)
	}
}

// ListByTenant returns all sessions for a tenant, merging in-memory and
// persisted sessions so that evicted (but saved) sessions still appear.
func (s *Store) ListByTenant(tenantID string) []Session {
	s.mu.RLock()
	seen := make(map[string]bool, len(s.sessions))
	var out []Session
	for _, sess := range s.sessions {
		if sess.TenantID == tenantID {
			seen[sess.ID] = true
			out = append(out, Session{
				ID:         sess.ID,
				TenantID:   sess.TenantID,
				Name:       sess.Name,
				Summary:    sess.Summary,
				Pinned:     sess.Pinned,
				ArchivedAt: sess.ArchivedAt,
				CreatedAt:  sess.CreatedAt,
				UpdatedAt:  sess.UpdatedAt,
			})
		}
	}
	s.mu.RUnlock()

	if s.repo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		repoSessions, err := s.repo.ListByTenant(ctx, tenantID)
		if err == nil {
			for _, rs := range repoSessions {
				if seen[rs.ID] {
					continue
				}
				out = append(out, rs)
			}
		}
	}
	return out
}

// Count returns total session count.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

// Rename sets a display name for a session.
func (s *Store) Rename(sessionID, name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return false
	}
	sess.Name = name
	sess.UpdatedAt = time.Now()
	s.persistMeta(sess)
	return true
}

// SetSummary updates the summary in a session (typically auto-generated).
func (s *Store) SetSummary(sessionID, summary string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return
	}
	sess.Summary = summary
	sess.UpdatedAt = time.Now()
	s.persistMeta(sess)
}

// Pin toggles the pinned state of a session.
func (s *Store) Pin(sessionID string, pinned bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return false
	}
	sess.Pinned = pinned
	sess.UpdatedAt = time.Now()
	s.persistMeta(sess)
	return true
}

// Archive soft-deletes a session (removes from active list, keeps data).
func (s *Store) Archive(sessionID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return false
	}
	now := time.Now()
	sess.ArchivedAt = &now
	sess.UpdatedAt = now
	s.persistMeta(sess)
	return true
}

// Unarchive restores an archived session.
func (s *Store) Unarchive(sessionID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return false
	}
	sess.ArchivedAt = nil
	sess.UpdatedAt = time.Now()
	s.persistMeta(sess)
	return true
}

// GetSession returns session info (without messages).
func (s *Store) GetSession(sessionID string) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil
	}
	return &Session{
		ID:         sess.ID,
		TenantID:   sess.TenantID,
		Name:       sess.Name,
		Summary:    sess.Summary,
		Pinned:     sess.Pinned,
		ArchivedAt: sess.ArchivedAt,
		CreatedAt:  sess.CreatedAt,
		UpdatedAt:  sess.UpdatedAt,
	}
}

// persistMeta saves session metadata via repo if available (non-blocking).
func (s *Store) persistMeta(sess *Session) {
	if s.repo == nil {
		return
	}
	if mr, ok := s.repo.(MetaRepo); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := mr.SaveMeta(ctx, sess); err != nil {
			slog.Error("session persist meta", "id", sess.ID, "err", err)
		}
	}
}

// MetaRepo extends Repo with metadata persistence.
type MetaRepo interface {
	SaveMeta(ctx context.Context, sess *Session) error
}
