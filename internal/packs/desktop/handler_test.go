package desktoppack

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/pkg/packruntime"
)

type fakeController struct {
	consoleHidden bool
	autoStart     bool
	setErr        error
}

func (f *fakeController) ToggleConsole() bool {
	f.consoleHidden = !f.consoleHidden
	return f.consoleHidden
}

func (f *fakeController) IsConsoleHidden() bool { return f.consoleHidden }

func (f *fakeController) SetAutoStart(enabled bool) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.autoStart = enabled
	return nil
}

func (f *fakeController) IsAutoStartEnabled() bool { return f.autoStart }

func TestDesktopPackV2AndRouteSpecs(t *testing.T) {
	var _ packruntime.Module = (*Handler)(nil)

	h := NewWithController(nil)
	if h.PackID() != PackID {
		t.Fatalf("PackID=%q, want %q", h.PackID(), PackID)
	}
	if got := len(h.Routes()); got != 2 {
		t.Fatalf("Routes len=%d, want 2", got)
	}
	if got := len(RouteSpecs()); got != 4 {
		t.Fatalf("RouteSpecs len=%d, want 4", got)
	}
	paths := map[string]bool{}
	for _, route := range h.Routes() {
		paths[route.Path] = true
	}
	for _, spec := range RouteSpecs() {
		if !paths[spec.Path] {
			t.Fatalf("route spec path %s has no mounted route", spec.Path)
		}
	}
	if err := h.Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := h.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestConsoleReadAndToggle(t *testing.T) {
	ctrl := &fakeController{}
	h := NewWithController(ctrl)

	read := httptest.NewRecorder()
	h.Console(read, httptest.NewRequest(http.MethodGet, "/v1/desktop/console", nil))
	if !strings.Contains(read.Body.String(), `"console_hidden":false`) {
		t.Fatalf("unexpected read body: %s", read.Body.String())
	}

	toggle := httptest.NewRecorder()
	h.Console(toggle, httptest.NewRequest(http.MethodPost, "/v1/desktop/console", nil))
	if !strings.Contains(toggle.Body.String(), `"console_hidden":true`) {
		t.Fatalf("unexpected toggle body: %s", toggle.Body.String())
	}
}

func TestAutoStartReadToggleAndError(t *testing.T) {
	ctrl := &fakeController{}
	h := NewWithController(ctrl)

	read := httptest.NewRecorder()
	h.AutoStart(read, httptest.NewRequest(http.MethodGet, "/v1/desktop/autostart", nil))
	if !strings.Contains(read.Body.String(), `"autostart_enabled":false`) {
		t.Fatalf("unexpected read body: %s", read.Body.String())
	}

	toggle := httptest.NewRecorder()
	h.AutoStart(toggle, httptest.NewRequest(http.MethodPost, "/v1/desktop/autostart", nil))
	if !strings.Contains(toggle.Body.String(), `"autostart_enabled":true`) {
		t.Fatalf("unexpected toggle body: %s", toggle.Body.String())
	}

	ctrl.setErr = errors.New("registry unavailable")
	errRec := httptest.NewRecorder()
	h.AutoStart(errRec, httptest.NewRequest(http.MethodPost, "/v1/desktop/autostart", nil))
	if !strings.Contains(errRec.Body.String(), `"error":"registry unavailable"`) {
		t.Fatalf("unexpected error body: %s", errRec.Body.String())
	}
}
