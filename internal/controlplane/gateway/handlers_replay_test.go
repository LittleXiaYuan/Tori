package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/observe"
)

func TestConversationReplaySanitizesPipelineEventsByDefault(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("replay-sanitized")
	trail := observe.NewAuditTrail(20)
	gw.SetEventTrail(trail)
	sessionID := "session-replay-safe"
	gw.convStore.GetOrCreate(sessionID, tenant.ID)
	gw.convStore.Append(sessionID, llm.Message{Role: "user", Content: "你好呀"})
	gw.convStore.Append(sessionID, llm.Message{Role: "assistant", Content: "我在。"})

	rawSummary := `handoff agent "file_exec" execution failed: context deadline exceeded`
	rawDetail := `all fallback LLM clients failed (FC): EOF`
	event := observe.NewEvent("trace-replay-safe", observe.DomainPlanner, observe.EventHandoffDone, rawSummary)
	event.Detail = observe.HandoffDetail{Agent: "file_exec", Error: rawDetail}
	event.Meta.SessionID = sessionID
	trail.Record(event)

	req := authedRequest(http.MethodGet, "/v1/conversations/replay?session_id="+sessionID, "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	body := strings.ToLower(w.Body.String())
	for _, banned := range []string{"handoff agent", "execution failed", "context deadline exceeded", "all fallback", "eof"} {
		if strings.Contains(body, banned) {
			t.Fatalf("replay response should be friendly by default; found %q in %s", banned, w.Body.String())
		}
	}
	if !strings.Contains(w.Body.String(), `"raw":false`) {
		t.Fatalf("expected raw=false marker, got %s", w.Body.String())
	}
	if !(strings.Contains(w.Body.String(), "现场已保留") || strings.Contains(w.Body.String(), "已保留现场")) {
		t.Fatalf("expected friendly recovery wording, got %s", w.Body.String())
	}
}

func TestConversationReplayRawModePreservesPipelineEvents(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("replay-raw")
	trail := observe.NewAuditTrail(20)
	gw.SetEventTrail(trail)
	sessionID := "session-replay-raw"
	gw.convStore.GetOrCreate(sessionID, tenant.ID)
	gw.convStore.Append(sessionID, llm.Message{Role: "user", Content: "你好呀"})
	gw.convStore.Append(sessionID, llm.Message{Role: "assistant", Content: "我在。"})

	rawSummary := `handoff agent "file_exec" execution failed: context deadline exceeded`
	event := observe.NewEvent("trace-replay-raw", observe.DomainPlanner, observe.EventHandoffDone, rawSummary)
	event.Meta.SessionID = sessionID
	trail.Record(event)

	req := authedRequest(http.MethodGet, "/v1/conversations/replay?session_id="+sessionID+"&raw=1", "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	body := strings.ToLower(w.Body.String())
	for _, want := range []string{"handoff agent", "context deadline exceeded", `"raw":true`} {
		if !strings.Contains(body, want) {
			t.Fatalf("raw replay response should preserve %q, got %s", want, w.Body.String())
		}
	}
}

func TestConversationReplaySkipsHiddenAttachmentContextWhenPairingTurns(t *testing.T) {
	gw, tm := newTestGateway()
	tenant := tm.Register("replay-attachment-hidden")
	sessionID := "session-replay-attachment-hidden"
	gw.convStore.GetOrCreate(sessionID, tenant.ID)
	gw.convStore.Append(sessionID, llm.Message{Role: "user", Content: "请读取附件"})
	gw.convStore.Append(sessionID, llm.Message{
		Role:    "system",
		Content: buildHiddenAttachmentContextMessage("[Attached file: 入驻申请表.docx]\n公司名称\t云鸢科技\n"),
	})
	gw.convStore.Append(sessionID, llm.Message{Role: "assistant", Content: "已读取附件。"})

	req := authedRequest(http.MethodGet, "/v1/conversations/replay?session_id="+sessionID, "", tenant.APIKey)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `"total_turns":1`) {
		t.Fatalf("expected one replay turn, got %s", body)
	}
	if !strings.Contains(body, `"assistant_reply":"已读取附件。"`) {
		t.Fatalf("expected assistant reply to be paired after hidden attachment context, got %s", body)
	}
	for _, banned := range []string{hiddenAttachmentContextMarker, "公司名称\\t云鸢科技", "公司名称\t云鸢科技", "Attached file"} {
		if strings.Contains(body, banned) {
			t.Fatalf("replay response should not expose hidden attachment context %q, got %s", banned, body)
		}
	}
}
