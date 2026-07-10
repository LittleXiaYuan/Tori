package gateway

import (
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/llm"
)

// A manually-added model (no preset_id) has no way to declare itself
// image-generation capable other than this explicit flag — preset-based
// registration gets Capabilities from the preset's per-model definition, but
// the simplified "add model" UI skips presets entirely. See
// providers-panel.tsx's "支持生图" checkbox.
func TestProviderRegisterImageGenFlagSetsCapability(t *testing.T) {
	gw, _ := newTestGateway()
	gw.providerReg = llm.NewProviderRegistry(nil)

	body := `{"id":"my-gemini","base_url":"https://generativelanguage.googleapis.com/v1beta","model":"gemini-2.5-flash-image","image_gen":true}`
	req := httptest.NewRequest("POST", "/api/providers/register", strings.NewReader(body))
	w := httptest.NewRecorder()
	gw.handleProviderRegister(w, req)
	if w.Code != 200 {
		t.Fatalf("register status=%d body=%s", w.Code, w.Body.String())
	}

	p := gw.providerReg.Get("my-gemini")
	if p == nil {
		t.Fatal("provider not registered")
	}
	if !hasCapability(p.Config.Capabilities, llm.CapImageGen) {
		t.Fatalf("expected CapImageGen on manually-added provider, got %v", p.Config.Capabilities)
	}
}

func TestProviderRegisterWithoutImageGenFlagStaysUncapable(t *testing.T) {
	gw, _ := newTestGateway()
	gw.providerReg = llm.NewProviderRegistry(nil)

	body := `{"id":"plain-chat","base_url":"https://api.openai.com/v1","model":"gpt-5.5"}`
	req := httptest.NewRequest("POST", "/api/providers/register", strings.NewReader(body))
	w := httptest.NewRecorder()
	gw.handleProviderRegister(w, req)
	if w.Code != 200 {
		t.Fatalf("register status=%d body=%s", w.Code, w.Body.String())
	}

	p := gw.providerReg.Get("plain-chat")
	if p == nil {
		t.Fatal("provider not registered")
	}
	if hasCapability(p.Config.Capabilities, llm.CapImageGen) {
		t.Fatalf("expected no CapImageGen without opt-in, got %v", p.Config.Capabilities)
	}
}
