package speechpack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"yunque-agent/internal/agentcore/speech"
)

func TestSpeechPackTTSNilRegistry(t *testing.T) {
	h := NewProvider(nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/speech/tts", strings.NewReader(`{"text":"hi"}`))
	rec := httptest.NewRecorder()

	h.TTS(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSpeechPackTTSAndVoices(t *testing.T) {
	reg := speech.NewRegistry()
	reg.RegisterTTS(mockTTS{name: "mock"})
	h := NewProvider(func() *speech.Registry { return reg }, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/speech/tts", strings.NewReader(`{"text":"hi","format":"wav"}`))
	rec := httptest.NewRecorder()
	h.TTS(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "audio/wav" {
		t.Fatalf("content-type = %s", got)
	}
	if rec.Body.String() != "mock:hi" {
		t.Fatalf("audio = %q", rec.Body.String())
	}

	voicesReq := httptest.NewRequest(http.MethodGet, "/v1/speech/voices", nil)
	voicesRec := httptest.NewRecorder()
	h.Voices(voicesRec, voicesReq)
	if voicesRec.Code != http.StatusOK {
		t.Fatalf("voices status = %d body=%s", voicesRec.Code, voicesRec.Body.String())
	}
	var body struct {
		Providers []string       `json:"providers"`
		Voices    []speech.Voice `json:"voices"`
	}
	if err := json.Unmarshal(voicesRec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Providers) != 1 || body.Providers[0] != "mock" || len(body.Voices) != 1 {
		t.Fatalf("unexpected voices body: %#v", body)
	}
}

func TestSpeechPackSTT(t *testing.T) {
	reg := speech.NewRegistry()
	reg.RegisterSTT(mockSTT{name: "stt"})
	h := NewProvider(func() *speech.Registry { return reg }, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/speech/stt?language=zh", strings.NewReader("audio"))
	rec := httptest.NewRecorder()
	h.STT(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["text"] != "stt:audio:zh" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestSpeechPackRoutesAndSpecsStayAligned(t *testing.T) {
	routes := map[string]map[string]bool{}
	for _, route := range (&Handler{}).Routes() {
		if routes[route.Path] == nil {
			routes[route.Path] = map[string]bool{}
		}
		if route.Method != "" {
			routes[route.Path][route.Method] = true
		}
		for _, method := range route.Methods {
			routes[route.Path][method] = true
		}
	}
	for _, spec := range RouteSpecs() {
		if !routes[spec.Path][spec.Method] {
			t.Fatalf("routeSpec %s %s not mounted by Routes()", spec.Method, spec.Path)
		}
	}
}

type mockTTS struct {
	name string
}

func (m mockTTS) Name() string { return m.name }

func (m mockTTS) TextToSpeech(_ context.Context, text string, _ speech.TTSOptions) ([]byte, error) {
	return []byte(m.name + ":" + text), nil
}

func (m mockTTS) Voices() []speech.Voice {
	return []speech.Voice{{ID: "mock", Name: "Mock", Language: "zh-CN"}}
}

type mockSTT struct {
	name string
}

func (m mockSTT) Name() string { return m.name }

func (m mockSTT) SpeechToText(_ context.Context, audio []byte, opts speech.STTOptions) (string, error) {
	return m.name + ":" + string(audio) + ":" + opts.Language, nil
}
