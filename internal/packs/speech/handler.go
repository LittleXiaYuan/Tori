package speechpack

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"yunque-agent/internal/agentcore/emotion"
	"yunque-agent/internal/agentcore/speech"
	"yunque-agent/internal/apperror"
	"yunque-agent/pkg/packruntime"

	"github.com/gorilla/websocket"
)

const PackID = "yunque.pack.speech"

type Gateway interface {
	SpeechRegistry() *speech.Registry
	EmotionAnalyzer() *emotion.Analyzer
	CheckWSOrigin(r *http.Request) bool
}

type Handler struct {
	speechOf  func() *speech.Registry
	emotionOf func() *emotion.Analyzer
	originOK  func(*http.Request) bool
	host      packruntime.Host
	started   atomic.Bool
}

func New(gateway Gateway) *Handler {
	if gateway == nil {
		return NewProvider(nil, nil, nil)
	}
	return NewProvider(gateway.SpeechRegistry, gateway.EmotionAnalyzer, gateway.CheckWSOrigin)
}

func NewProvider(
	speechRegistry func() *speech.Registry,
	emotionAnalyzer func() *emotion.Analyzer,
	checkWSOrigin func(*http.Request) bool,
) *Handler {
	if checkWSOrigin == nil {
		checkWSOrigin = func(*http.Request) bool { return true }
	}
	return &Handler{speechOf: speechRegistry, emotionOf: emotionAnalyzer, originOK: checkWSOrigin}
}

func (h *Handler) PackID() string { return PackID }

var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("speech pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodPost, Path: "/v1/speech/tts", Handler: h.TTS},
		{Method: http.MethodPost, Path: "/v1/speech/stt", Handler: h.STT},
		{Method: http.MethodGet, Path: "/v1/speech/stt/stream", Handler: h.STTStream},
		{Method: http.MethodGet, Path: "/v1/speech/voices", Handler: h.Voices},
	}
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodPost, Path: "/v1/speech/tts", Description: "Synthesize speech from text."},
		{Method: http.MethodPost, Path: "/v1/speech/stt", Description: "Transcribe uploaded audio to text."},
		{Method: http.MethodGet, Path: "/v1/speech/stt/stream", Description: "Transcribe WebSocket audio chunks in real time."},
		{Method: http.MethodGet, Path: "/v1/speech/voices", Description: "List available speech voices and TTS providers."},
	}
}

func Paths() []string {
	return []string{"/v1/speech/tts", "/v1/speech/stt", "/v1/speech/stt/stream", "/v1/speech/voices"}
}

func (h *Handler) speech() *speech.Registry {
	if h.speechOf == nil {
		return nil
	}
	return h.speechOf()
}

func (h *Handler) emotion() *emotion.Analyzer {
	if h.emotionOf == nil {
		return nil
	}
	return h.emotionOf()
}

func (h *Handler) TTS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	reg := h.speech()
	if reg == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "speech not configured")
		return
	}

	var req struct {
		Text    string `json:"text"`
		Voice   string `json:"voice,omitempty"`
		Format  string `json:"format,omitempty"`
		Emotion string `json:"emotion,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid json")
		return
	}
	if req.Text == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "text is required")
		return
	}
	if len(req.Text) > 5000 {
		apperror.WriteCode(w, apperror.CodeBadRequest, "text too long (max 5000 chars)")
		return
	}

	opts := speech.TTSOptions{Voice: req.Voice, Format: req.Format}
	if req.Emotion != "" {
		opts = opts.ApplyEmotion(req.Emotion)
	}
	audio, err := reg.TextToSpeech(r.Context(), req.Text, opts)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}

	w.Header().Set("Content-Type", contentTypeForFormat(req.Format))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(audio)
}

func (h *Handler) STT(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	reg := h.speech()
	if reg == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "speech not configured")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	audio, err := io.ReadAll(r.Body)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "failed to read audio")
		return
	}
	lang := r.URL.Query().Get("language")
	text, err := reg.SpeechToText(r.Context(), audio, speech.STTOptions{Language: lang})
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}

	resp := map[string]any{"text": text}
	if r.URL.Query().Get("detect_emotion") == "true" {
		if analyzer := h.emotion(); analyzer != nil && analyzer.Enabled() {
			if emotionResult, err := analyzer.AnalyzeText(r.Context(), text); err == nil && emotionResult != nil {
				emotionResult.Source = "audio"
				resp["emotion"] = emotionResult
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) Voices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	reg := h.speech()
	if reg == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "speech not configured")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"voices":    reg.Voices(),
		"providers": reg.ListTTS(),
	})
}

func (h *Handler) STTStream(w http.ResponseWriter, r *http.Request) {
	reg := h.speech()
	if reg == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "speech not configured")
		return
	}
	upgrader := websocket.Upgrader{
		CheckOrigin:     h.originOK,
		ReadBufferSize:  64 * 1024,
		WriteBufferSize: 4 * 1024,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("stt stream: upgrade failed", "err", err)
		return
	}
	defer conn.Close()

	lang := r.URL.Query().Get("language")
	if lang == "" {
		lang = "zh"
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		return nil
	})

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Warn("stt stream: read error", "err", err)
			}
			break
		}
		if msgType == websocket.TextMessage {
			var ctrl struct {
				Action string `json:"action"`
			}
			if json.Unmarshal(data, &ctrl) == nil && ctrl.Action == "stop" {
				break
			}
			continue
		}
		if msgType != websocket.BinaryMessage || len(data) == 0 {
			continue
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		text, sttErr := reg.SpeechToText(ctx, data, speech.STTOptions{Language: lang})
		cancel()

		resp := map[string]any{"text": text, "final": false}
		if sttErr != nil {
			resp["error"] = sttErr.Error()
			resp["text"] = ""
		}
		if text != "" {
			if analyzer := h.emotion(); analyzer != nil && analyzer.Enabled() {
				if emotionResult, emotErr := analyzer.AnalyzeText(r.Context(), text); emotErr == nil && emotionResult != nil {
					emotionResult.Source = "audio"
					resp["emotion"] = emotionResult
				}
			}
		}
		respBytes, _ := json.Marshal(resp)
		if err := conn.WriteMessage(websocket.TextMessage, respBytes); err != nil {
			break
		}
	}
	finalResp, _ := json.Marshal(map[string]any{"text": "", "final": true})
	_ = conn.WriteMessage(websocket.TextMessage, finalResp)
}

func contentTypeForFormat(format string) string {
	switch format {
	case "wav":
		return "audio/wav"
	case "opus":
		return "audio/opus"
	case "flac":
		return "audio/flac"
	default:
		return "audio/mpeg"
	}
}
