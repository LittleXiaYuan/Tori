package emotionpack

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/pkg/packruntime"
)

type fakeGW struct {
	hist *emotion.History
	sm   *emotion.StickerMap
}

func (f fakeGW) EmotionHistory() *emotion.History { return f.hist }
func (f fakeGW) StickerMap() *emotion.StickerMap  { return f.sm }

// TestEmotionPackV2 verifies the emotion pack is a v2 Module with the expected
// route surface and degrades to 404 when the subsystems are not configured
// (native handlers, de-shelled from the gateway).
func TestEmotionPackV2(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := New(fakeGW{}) // nil history + sticker map
	if h.PackID() != PackID {
		t.Fatalf("PackID = %q, want %q", h.PackID(), PackID)
	}
	if got := len(h.Routes()); got != 2 {
		t.Fatalf("Routes len = %d, want 2", got)
	}
	if err := h.Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	histRec := httptest.NewRecorder()
	h.handleHistory(histRec, httptest.NewRequest(http.MethodGet, "/v1/emotion/history", nil))
	if histRec.Code != http.StatusNotFound {
		t.Fatalf("nil history handleHistory = %d, want 404", histRec.Code)
	}

	stickerRec := httptest.NewRecorder()
	h.handleStickers(stickerRec, httptest.NewRequest(http.MethodGet, "/v1/emotion/stickers", nil))
	if stickerRec.Code != http.StatusNotFound {
		t.Fatalf("nil sticker map handleStickers = %d, want 404", stickerRec.Code)
	}

	if err := h.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

// TestStickerPutThenGet exercises the native PUT→GET path end to end through the
// pack handler, proving the logic moved into the pack (not a shell).
func TestStickerPutThenGet(t *testing.T) {
	h := New(fakeGW{sm: emotion.NewStickerMap()})

	body := []byte(`{"platform":"feishu","emotion":"happy","stickers":[{"package_id":"p1","sticker_id":"s1"}]}`)
	putRec := httptest.NewRecorder()
	h.handleStickers(putRec, httptest.NewRequest(http.MethodPut, "/v1/emotion/stickers", bytes.NewReader(body)))
	if putRec.Code != http.StatusOK {
		t.Fatalf("put = %d, want 200: %s", putRec.Code, putRec.Body.String())
	}

	getRec := httptest.NewRecorder()
	h.handleStickers(getRec, httptest.NewRequest(http.MethodGet, "/v1/emotion/stickers", nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("get = %d, want 200", getRec.Code)
	}
	var export map[string]map[string][]map[string]any
	if err := json.Unmarshal(getRec.Body.Bytes(), &export); err != nil {
		t.Fatal(err)
	}
	if len(export["feishu"]["happy"]) == 0 {
		t.Fatalf("expected registered sticker under feishu/happy, got %s", getRec.Body.String())
	}
}

// TestHistoryQueryOK confirms a configured history returns 200 with a summary.
func TestHistoryQueryOK(t *testing.T) {
	h := New(fakeGW{hist: emotion.NewHistory(10)})
	rec := httptest.NewRecorder()
	h.handleHistory(rec, httptest.NewRequest(http.MethodGet, "/v1/emotion/history?session_id=s1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("history = %d, want 200", rec.Code)
	}
	var resp struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 0 {
		t.Fatalf("empty history should have total 0, got %d", resp.Total)
	}
}
