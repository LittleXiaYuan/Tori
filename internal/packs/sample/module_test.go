package sample

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/pkg/packruntime"
)

// fakeHost is a test double for packruntime.Host. Its existence proves the
// sample pack depends only on the kernel contract: it can be driven with no
// Gateway, no real subsystems — just the narrow ports.
type fakeHost struct {
	kv     map[string]any
	events []string
}

func newFakeHost() *fakeHost { return &fakeHost{kv: map[string]any{}} }

var _ packruntime.Host = (*fakeHost)(nil)

func (f *fakeHost) Handle(string, http.HandlerFunc)                   {}
func (f *fakeHost) RequireAuth(h http.HandlerFunc) http.HandlerFunc   { return h }
func (f *fakeHost) Logger() *slog.Logger                              { return slog.Default() }
func (f *fakeHost) LLM() packruntime.LLMPort                          { return nil }
func (f *fakeHost) KV() packruntime.KVPort                            { return f }
func (f *fakeHost) Events() packruntime.EventBus                      { return f }
func (f *fakeHost) Service(string) (any, bool)                        { return nil, false }

// KVPort
func (f *fakeHost) Get(_ context.Context, key string, _ any) (bool, error) {
	_, ok := f.kv[key]
	return ok, nil
}
func (f *fakeHost) Put(_ context.Context, key string, val any) error {
	f.kv[key] = val
	return nil
}

// EventBus
func (f *fakeHost) Emit(kind string, _ map[string]any) { f.events = append(f.events, kind) }

func TestSampleModuleRunsOnHostOnly(t *testing.T) {
	m := New()
	fh := newFakeHost()

	if err := m.Init(fh); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(m.Routes()) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(m.Routes()))
	}

	// ping
	rec := httptest.NewRecorder()
	m.handlePing(rec, httptest.NewRequest(http.MethodGet, "/v1/packs/sample/ping", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("ping status = %d", rec.Code)
	}
	var ping map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &ping); err != nil {
		t.Fatalf("ping body: %v", err)
	}
	if ping["pong"] != true || ping["started"] != true {
		t.Fatalf("unexpected ping body: %v", ping)
	}

	// echo persists through the kernel KV port
	rec2 := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/packs/sample/echo",
		strings.NewReader(`{"key":"k1","value":"hello"}`))
	m.handleEcho(rec2, req)
	if rec2.Code != http.StatusOK {
		t.Fatalf("echo status = %d", rec2.Code)
	}
	if fh.kv["k1"] != "hello" {
		t.Fatalf("expected host KV to store hello, got %v", fh.kv["k1"])
	}

	if err := m.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// AsModule on a v2 module returns it unchanged.
	if packruntime.AsModule(m).PackID() != PackID {
		t.Fatalf("AsModule should preserve PackID")
	}
}

// TestSampleProvidesContextAndSkill proves the reference pack flows into the
// agent: it contributes prompt context (when enabled) and a callable tool — the
// Tier 0 microkernel "context line" + "tool line".
func TestSampleProvidesContextAndSkill(t *testing.T) {
	var _ packruntime.ContextProvider = (*Module)(nil)
	var _ packruntime.SkillProvider = (*Module)(nil)

	m := New()
	if got := m.BuildContext(context.Background(), "hi", "t"); got != "" {
		t.Fatalf("expected empty context before Start, got %q", got)
	}
	_ = m.Init(newFakeHost())
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if got := m.BuildContext(context.Background(), "hi", "t"); got == "" {
		t.Fatalf("expected context after Start")
	}

	sk := m.Skills()
	if len(sk) != 1 || sk[0].Name() != "kernel_sample_ping" {
		t.Fatalf("expected kernel_sample_ping skill, got %#v", sk)
	}
	out, err := sk[0].Execute(context.Background(), nil, nil)
	if err != nil || out != "pong" {
		t.Fatalf("skill Execute = %q, %v; want pong,nil", out, err)
	}
}
