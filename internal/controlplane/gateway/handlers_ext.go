package gateway

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/session"
	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/safego"
)

// Persona base/preset HTTP handlers were de-shelled into the persona pack
// (internal/packs/persona). The gateway keeps only narrow persona accessors.

// --- Conversation API ---

func (g *Gateway) handleConversations(w http.ResponseWriter, r *http.Request) {
	tid := tenantFromCtx(r.Context())
	w.Header().Set("Content-Type", "application/json")
	sessions := g.convStore.ListByTenant(tid)

	// Filter: exclude archived unless ?archived=true
	showArchived := r.URL.Query().Get("archived") == "true"
	var filtered []session.Session
	for _, s := range sessions {
		if s.ArchivedAt != nil && !showArchived {
			continue
		}
		filtered = append(filtered, s)
	}
	json.NewEncoder(w).Encode(map[string]any{"sessions": filtered, "count": len(filtered)})
}

func (g *Gateway) handleConversationMessages(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "session_id query param required")
		return
	}
	switch r.Method {
	case http.MethodGet:
		msgs := g.convStore.Get(sessionID)
		msgs = visibleConversationMessages(msgs)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"messages": msgs, "count": len(msgs)})
	case http.MethodDelete:
		g.convStore.Delete(sessionID)
		if g.taskStore != nil {
			tenantID := tenantFromCtx(r.Context())
			tasks := g.taskStore.List(tenantID, 0)
			for _, t := range tasks {
				if t.Constraints != nil && t.Constraints.Extra != nil {
					if sid, ok := t.Constraints.Extra["session_id"].(string); ok && sid == sessionID {
						g.taskStore.Delete(t.ID)
					}
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
	default:
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or DELETE only")
	}
}

func visibleConversationMessages(msgs []llm.Message) []llm.Message {
	if len(msgs) == 0 {
		return nil
	}
	out := make([]llm.Message, 0, len(msgs))
	for _, msg := range msgs {
		if isHiddenAttachmentContextMessage(msg) {
			continue
		}
		out = append(out, msg)
	}
	return out
}

// handleConversationManage handles rename, pin, archive operations on a conversation.
func (g *Gateway) handleConversationManage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "PUT only")
		return
	}
	var req struct {
		SessionID string  `json:"session_id"`
		Name      *string `json:"name,omitempty"`
		Pinned    *bool   `json:"pinned,omitempty"`
		Archive   *bool   `json:"archive,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid body")
		return
	}
	if req.SessionID == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "session_id required")
		return
	}

	if req.Name != nil {
		g.convStore.Rename(req.SessionID, *req.Name)
	}
	if req.Pinned != nil {
		g.convStore.Pin(req.SessionID, *req.Pinned)
	}
	if req.Archive != nil {
		if *req.Archive {
			g.convStore.Archive(req.SessionID)
		} else {
			g.convStore.Unarchive(req.SessionID)
		}
	}

	sess := g.convStore.GetSession(req.SessionID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": "updated", "session": sess})
}

// --- Feishu Webhook ---

// feishuWebhookMaxBody caps the request body at 1 MiB. Feishu events are well
// below this threshold; anything larger is almost certainly abuse.
const feishuWebhookMaxBody = 1 << 20

// handleFeishuWebhook is the public callback endpoint invoked by Feishu for
// URL verification and incoming events. It is deliberately *not* wrapped in
// requireAuth because Feishu itself authenticates via signature/token, but we
// enforce that here before any downstream state (planner.Run, LLM calls) is
// touched. Without this, any network-reachable host could forge events and
// burn through the agent's LLM quota.
func (g *Gateway) handleFeishuWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	// Buffer the body once so we can verify the signature (computed over the
	// raw bytes) and then decode the JSON.
	raw, err := io.ReadAll(io.LimitReader(r.Body, feishuWebhookMaxBody+1))
	if err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "read body failed")
		return
	}
	if len(raw) > feishuWebhookMaxBody {
		apperror.WriteCode(w, apperror.CodeBadRequest, "request body too large")
		return
	}

	if err := verifyFeishuRequest(r.Header, raw); err != nil {
		slog.Warn("feishu webhook rejected", "err", err, "remote", r.RemoteAddr)
		writeJSONStatus(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	var body struct {
		Challenge string `json:"challenge"` // URL verification
		Token     string `json:"token"`     // Legacy URL-verification token
		Type      string `json:"type"`
		Event     struct {
			Message struct {
				ChatID  string `json:"chat_id"`
				Content string `json:"content"`
			} `json:"message"`
		} `json:"event"`
	}
	if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&body); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid request body")
		return
	}
	// Feishu URL verification (happens once during app setup). The token in
	// the body must match the configured verification token; we already
	// compare it in constant time inside verifyFeishuRequest.
	if body.Challenge != "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"challenge": body.Challenge})
		return
	}
	// Process message async and reply via Feishu API
	incomingText := feishuMessageText(body.Event.Message.Content)
	if incomingText != "" {
		safego.Go("feishu-webhook-reply", func() {
			ctx := context.Background()
			if code, content, ok := parseCollabCommand(incomingText); ok {
				reply := g.handleCollabInbound(ctx, code, content, "feishu", body.Event.Message.ChatID)
				if g.feishuAPI != nil && body.Event.Message.ChatID != "" {
					if err := g.feishuAPI.SendMessage(body.Event.Message.ChatID, reply); err != nil {
						slog.Error("feishu collab reply error", "err", err)
					}
				}
				return
			}
			result, err := g.planner.Run(ctx, planner.PlanRequest{
				Messages: []llm.Message{{Role: "user", Content: incomingText}},
				TenantID: "default",
			})
			if err != nil {
				slog.Error("feishu webhook planner error", "err", err)
				return
			}
			// Send reply back to Feishu chat
			if g.feishuAPI != nil && body.Event.Message.ChatID != "" {
				if err := g.feishuAPI.SendMessage(body.Event.Message.ChatID, result.Reply); err != nil {
					slog.Error("feishu reply error", "err", err)
				}
			}
			slog.Info("feishu webhook reply", "chat_id", body.Event.Message.ChatID, "len", len(result.Reply))
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// verifyFeishuRequest authenticates an incoming Feishu webhook using whichever
// credentials the operator has configured:
//   - X-Lark-Signature (HMAC over timestamp + nonce + encrypt_key + body) when
//     FEISHU_ENCRYPT_KEY is set — this is the documented signed-event scheme.
//   - A constant-time comparison of the body's `token` field against
//     FEISHU_VERIFICATION_TOKEN when that env is set.
//
// We fail-closed: if neither env is configured the webhook is disabled so a
// pre-production agent does not silently accept unauthenticated events.
func verifyFeishuRequest(h http.Header, body []byte) error {
	encryptKey := strings.TrimSpace(os.Getenv("FEISHU_ENCRYPT_KEY"))
	verifyToken := strings.TrimSpace(os.Getenv("FEISHU_VERIFICATION_TOKEN"))

	if encryptKey == "" && verifyToken == "" {
		return fmt.Errorf("feishu webhook not configured (set FEISHU_ENCRYPT_KEY or FEISHU_VERIFICATION_TOKEN)")
	}

	if encryptKey != "" {
		sig := h.Get("X-Lark-Signature")
		ts := h.Get("X-Lark-Request-Timestamp")
		nonce := h.Get("X-Lark-Request-Nonce")
		if sig == "" || ts == "" {
			// If the signature headers are missing we fall through to token
			// auth below; that mirrors Feishu's own optional signing.
		} else {
			tsInt, err := strconv.ParseInt(ts, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid timestamp header")
			}
			if delta := time.Since(time.Unix(tsInt, 0)); delta < -5*time.Minute || delta > 5*time.Minute {
				return fmt.Errorf("timestamp outside allowed window")
			}
			// Feishu signs sha256(timestamp + nonce + encrypt_key + body) and
			// then base16-encodes the digest. Sorting the three string inputs
			// is not required; we preserve the documented order.
			h := sha256.New()
			_, _ = io.WriteString(h, ts)
			_, _ = io.WriteString(h, nonce)
			_, _ = io.WriteString(h, encryptKey)
			_, _ = h.Write(body)
			expected := hex.EncodeToString(h.Sum(nil))
			if subtle.ConstantTimeCompare([]byte(expected), []byte(sig)) != 1 {
				return fmt.Errorf("signature mismatch")
			}
			return nil
		}
	}

	if verifyToken == "" {
		return fmt.Errorf("missing signature headers and no verification token configured")
	}
	// Fall back to comparing the body's `token` claim in constant time.
	var peek struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &peek); err != nil {
		return fmt.Errorf("invalid json body")
	}
	if subtle.ConstantTimeCompare([]byte(peek.Token), []byte(verifyToken)) != 1 {
		return fmt.Errorf("token mismatch")
	}
	return nil
}
