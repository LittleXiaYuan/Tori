package speech

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegistryTTS(t *testing.T) {
	reg := NewRegistry()
	if list := reg.ListTTS(); len(list) != 0 {
		t.Fatalf("expected 0 TTS providers, got %d", len(list))
	}

	_, err := reg.TextToSpeech(context.Background(), "hello", TTSOptions{})
	if err == nil {
		t.Fatal("expected error with no provider")
	}

	reg.RegisterTTS(&mockTTS{name: "mock1"})
	reg.RegisterTTS(&mockTTS{name: "mock2"})

	if list := reg.ListTTS(); len(list) != 2 {
		t.Fatalf("expected 2 TTS providers, got %d", len(list))
	}

	// Default should be first registered
	audio, err := reg.TextToSpeech(context.Background(), "test", TTSOptions{})
	if err != nil {
		t.Fatalf("TTS failed: %v", err)
	}
	if string(audio) != "mock1:test" {
		t.Fatalf("expected mock1:test, got %q", string(audio))
	}
}

func TestRegistrySTT(t *testing.T) {
	reg := NewRegistry()
	if list := reg.ListSTT(); len(list) != 0 {
		t.Fatalf("expected 0 STT providers, got %d", len(list))
	}

	_, err := reg.SpeechToText(context.Background(), []byte("audio"), STTOptions{})
	if err == nil {
		t.Fatal("expected error with no provider")
	}

	reg.RegisterSTT(&mockSTT{name: "mock_stt"})

	text, err := reg.SpeechToText(context.Background(), []byte("audio_data"), STTOptions{Language: "zh"})
	if err != nil {
		t.Fatalf("STT failed: %v", err)
	}
	if text != "mock_stt:audio_data" {
		t.Fatalf("expected mock_stt:audio_data, got %q", text)
	}
}

func TestOpenAITTSName(t *testing.T) {
	tts := NewOpenAITTS("", "key", "")
	if tts.Name() != "openai_tts" {
		t.Fatalf("expected openai_tts, got %s", tts.Name())
	}
}

func TestOpenAITTSVoices(t *testing.T) {
	tts := NewOpenAITTS("", "key", "")
	voices := tts.Voices()
	if len(voices) == 0 {
		t.Fatal("expected voices")
	}
	found := false
	for _, v := range voices {
		if v.ID == "alloy" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected alloy voice")
	}
}

func TestOpenAITTSWithMockServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audio/speech" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("unexpected auth: %s", auth)
		}

		var payload map[string]any
		json.NewDecoder(r.Body).Decode(&payload)
		if payload["model"] != "tts-1" {
			t.Errorf("unexpected model: %v", payload["model"])
		}
		if payload["voice"] != "nova" {
			t.Errorf("unexpected voice: %v", payload["voice"])
		}

		w.WriteHeader(200)
		w.Write([]byte("fake-audio-data"))
	}))
	defer srv.Close()

	tts := NewOpenAITTS(srv.URL, "test-key", "tts-1")
	audio, err := tts.TextToSpeech(context.Background(), "Hello world", TTSOptions{
		Voice:  "nova",
		Speed:  1.0,
		Format: "mp3",
	})
	if err != nil {
		t.Fatalf("TTS failed: %v", err)
	}
	if string(audio) != "fake-audio-data" {
		t.Fatalf("expected fake-audio-data, got %q", string(audio))
	}
}

func TestOpenAITTSDefaultOptions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		json.NewDecoder(r.Body).Decode(&payload)
		if payload["voice"] != "alloy" {
			t.Errorf("expected default voice alloy, got %v", payload["voice"])
		}
		if payload["response_format"] != "mp3" {
			t.Errorf("expected default format mp3, got %v", payload["response_format"])
		}
		w.Write([]byte("audio"))
	}))
	defer srv.Close()

	tts := NewOpenAITTS(srv.URL, "key", "")
	_, err := tts.TextToSpeech(context.Background(), "test", TTSOptions{})
	if err != nil {
		t.Fatalf("TTS failed: %v", err)
	}
}

func TestOpenAITTSErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		fmt.Fprint(w, `{"error": "bad request"}`)
	}))
	defer srv.Close()

	tts := NewOpenAITTS(srv.URL, "key", "")
	_, err := tts.TextToSpeech(context.Background(), "test", TTSOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Fatalf("expected 400 in error, got: %v", err)
	}
}

func TestOpenAISTTName(t *testing.T) {
	stt := NewOpenAISTT("", "key", "")
	if stt.Name() != "openai_stt" {
		t.Fatalf("expected openai_stt, got %s", stt.Name())
	}
}

func TestOpenAISTTWithMockServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audio/transcriptions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Verify multipart
		if !strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("expected multipart, got %s", r.Header.Get("Content-Type"))
		}

		// Parse multipart to verify fields
		r.ParseMultipartForm(10 << 20)
		if r.FormValue("model") != "whisper-1" {
			t.Errorf("unexpected model: %s", r.FormValue("model"))
		}
		if r.FormValue("language") != "zh" {
			t.Errorf("unexpected language: %s", r.FormValue("language"))
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Errorf("no file: %v", err)
		} else {
			data, _ := io.ReadAll(file)
			if string(data) != "fake-audio" {
				t.Errorf("unexpected file content: %q", string(data))
			}
		}

		json.NewEncoder(w).Encode(map[string]string{"text": "你好世界"})
	}))
	defer srv.Close()

	stt := NewOpenAISTT(srv.URL, "test-key", "whisper-1")
	text, err := stt.SpeechToText(context.Background(), []byte("fake-audio"), STTOptions{Language: "zh"})
	if err != nil {
		t.Fatalf("STT failed: %v", err)
	}
	if text != "你好世界" {
		t.Fatalf("expected 你好世界, got %q", text)
	}
}

func TestOpenAISTTErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, `{"error": "internal"}`)
	}))
	defer srv.Close()

	stt := NewOpenAISTT(srv.URL, "key", "")
	_, err := stt.SpeechToText(context.Background(), []byte("audio"), STTOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected 500 in error, got: %v", err)
	}
}

func TestEdgeTTSName(t *testing.T) {
	tts := NewEdgeTTS()
	if tts.Name() != "edge_tts" {
		t.Fatalf("expected edge_tts, got %s", tts.Name())
	}
}

func TestEdgeTTSVoices(t *testing.T) {
	tts := NewEdgeTTS()
	voices := tts.Voices()
	if len(voices) == 0 {
		t.Fatal("expected voices")
	}
	// Should have Chinese voices
	found := false
	for _, v := range voices {
		if strings.HasPrefix(v.Language, "zh-CN") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected zh-CN voices")
	}
}

func TestEdgeTTSPlaceholder(t *testing.T) {
	tts := NewEdgeTTS()
	_, err := tts.TextToSpeech(context.Background(), "hello", TTSOptions{})
	if err == nil {
		t.Fatal("expected error from placeholder")
	}
}

func TestTTSOptions(t *testing.T) {
	opts := TTSOptions{
		Voice:  "nova",
		Speed:  1.5,
		Format: "wav",
	}
	if opts.Voice != "nova" || opts.Speed != 1.5 || opts.Format != "wav" {
		t.Fatal("options mismatch")
	}
}

func TestSTTOptions(t *testing.T) {
	opts := STTOptions{
		Language: "en",
		Format:   "wav",
	}
	if opts.Language != "en" || opts.Format != "wav" {
		t.Fatal("options mismatch")
	}
}

func TestApplyEmotion(t *testing.T) {
	base := TTSOptions{Speed: 1.0, Voice: "alloy", Format: "mp3"}

	// Happy → speed up
	happy := base.ApplyEmotion("happy")
	if happy.Speed <= 1.0 {
		t.Errorf("happy speed should increase, got %v", happy.Speed)
	}
	if happy.Voice != "alloy" {
		t.Errorf("happy should keep voice alloy, got %s", happy.Voice)
	}

	// Sad → slow down + voice override when no voice set
	noVoice := TTSOptions{Speed: 1.0, Format: "mp3"}
	sad := noVoice.ApplyEmotion("sad")
	if sad.Speed >= 1.0 {
		t.Errorf("sad speed should decrease, got %v", sad.Speed)
	}
	if sad.Voice != "echo" {
		t.Errorf("sad should set voice to echo, got %s", sad.Voice)
	}

	// Sad with explicit voice → keep user voice
	sadExplicit := base.ApplyEmotion("sad")
	if sadExplicit.Voice != "alloy" {
		t.Errorf("should keep explicit voice, got %s", sadExplicit.Voice)
	}

	// Unknown emotion → unchanged
	unknown := base.ApplyEmotion("rage")
	if unknown.Speed != base.Speed || unknown.Voice != base.Voice {
		t.Errorf("unknown emotion should not change opts")
	}

	// Empty → unchanged
	empty := base.ApplyEmotion("")
	if empty.Speed != base.Speed {
		t.Errorf("empty emotion should not change opts")
	}

	// Clamp test (extreme low)
	veryLow := TTSOptions{Speed: 0.5}
	clamped := veryLow.ApplyEmotion("sad") // 0.5 * 0.85 = 0.425 → clamped to 0.5
	if clamped.Speed < 0.5 {
		t.Errorf("speed should be clamped to 0.5, got %v", clamped.Speed)
	}
}

// ── Mock providers for testing ──

type mockTTS struct {
	name string
}

func (m *mockTTS) Name() string { return m.name }
func (m *mockTTS) Voices() []Voice {
	return []Voice{{ID: "default", Name: "Default"}}
}
func (m *mockTTS) TextToSpeech(_ context.Context, text string, _ TTSOptions) ([]byte, error) {
	return []byte(m.name + ":" + text), nil
}

type mockSTT struct {
	name string
}

func (m *mockSTT) Name() string { return m.name }
func (m *mockSTT) SpeechToText(_ context.Context, audio []byte, _ STTOptions) (string, error) {
	return m.name + ":" + string(audio), nil
}
