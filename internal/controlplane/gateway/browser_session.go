package gateway

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

const (
	browserSessionTicketTTL = 2 * time.Minute
)

type browserSessionTicket struct {
	Ticket    string
	TenantID  string
	Nonce     string
	ExpiresAt time.Time
	UsedAt    time.Time
}

type BrowserSessionStore struct {
	mu      sync.Mutex
	tickets map[string]browserSessionTicket
}

func NewBrowserSessionStore() *BrowserSessionStore {
	return &BrowserSessionStore{tickets: make(map[string]browserSessionTicket)}
}

func (s *BrowserSessionStore) Issue(tenantID string) (browserSessionTicket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.pruneLocked(now)

	ticket, err := randomHex(32)
	if err != nil {
		return browserSessionTicket{}, err
	}
	nonce, err := randomHex(16)
	if err != nil {
		return browserSessionTicket{}, err
	}

	record := browserSessionTicket{
		Ticket:    ticket,
		TenantID:  tenantID,
		Nonce:     nonce,
		ExpiresAt: now.Add(browserSessionTicketTTL),
	}
	s.tickets[ticket] = record
	return record, nil
}

func (s *BrowserSessionStore) Consume(ticket, nonce string) (browserSessionTicket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.pruneLocked(now)

	record, ok := s.tickets[ticket]
	if !ok {
		return browserSessionTicket{}, fmt.Errorf("invalid browser session ticket")
	}
	if record.Nonce != nonce {
		delete(s.tickets, ticket)
		return browserSessionTicket{}, fmt.Errorf("browser session nonce mismatch")
	}
	if !record.UsedAt.IsZero() {
		delete(s.tickets, ticket)
		return browserSessionTicket{}, fmt.Errorf("browser session ticket already used")
	}
	record.UsedAt = now
	s.tickets[ticket] = record
	return record, nil
}

func (s *BrowserSessionStore) Invalidate(ticket string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tickets, ticket)
}

func (s *BrowserSessionStore) pruneLocked(now time.Time) {
	for key, ticket := range s.tickets {
		if now.After(ticket.ExpiresAt) {
			delete(s.tickets, key)
		}
	}
}

func randomHex(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func browserChallengeProof(ticket, clientNonce, serverChallenge string) string {
	sum := sha256.Sum256([]byte(ticket + ":" + clientNonce + ":" + serverChallenge))
	return hex.EncodeToString(sum[:])
}

type ctxBrowserTicketKey struct{}

func contextWithBrowserTicket(ctx context.Context, ticket browserSessionTicket) context.Context {
	return context.WithValue(ctx, ctxBrowserTicketKey{}, ticket)
}

func browserTicketFromCtx(ctx context.Context) (browserSessionTicket, bool) {
	ticket, ok := ctx.Value(ctxBrowserTicketKey{}).(browserSessionTicket)
	return ticket, ok
}
