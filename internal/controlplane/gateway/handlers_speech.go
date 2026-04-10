package gateway

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"yunque-agent/internal/agentcore/speech"
	"yunque-agent/internal/apperror"

	"github.com/gorilla/websocket"
)

// handleTTS handles POST /v1/speech/tts — synthesize speech from text.
func (g *Gateway) handleTTS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	if g.speechReg == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "speech not configured")
		return
	}

	var req struct {
		Text    string `json:"text"`
		Voice   string `json:"voice,omitempty"`
		Format  string `json:"format,omitempty"`
		Emotion string `json:"emotion,omitempty"` // apply emotion-aware voice modulation
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

	opts := speech.TTSOptions{
		Voice:  req.Voice,
		Format: req.Format,
	}
	if req.Emotion != "" {
		opts = opts.ApplyEmotion(req.Emotion)
	}

	audio, err := g.speechReg.TextToSpeech(r.Context(), req.Text, opts)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}

	ct := "audio/mpeg"
	switch req.Format {
	case "wav":
		ct = "audio/wav"
	case "opus":
		ct = "audio/opus"
	case "flac":
		ct = "audio/flac"
	}

	w.Header().Set("Content-Type", ct)
	w.WriteHeader(http.StatusOK)
	w.Write(audio)
}

// handleSTT handles POST /v1/speech/stt — transcribe audio to text.
func (g *Gateway) handleSTT(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	if g.speechReg == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "speech not configured")
		return
	}

	// Max 10MB audio
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	audio, err := io.ReadAll(r.Body)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "failed to read audio")
		return
	}

	lang := r.URL.Query().Get("language")
	text, err := g.speechReg.SpeechToText(r.Context(), audio, speech.STTOptions{
		Language: lang,
	})
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, err.Error())
		return
	}

	resp := map[string]any{"text": text}

	// Optional emotion detection on transcribed text
	if r.URL.Query().Get("detect_emotion") == "true" && g.emotionAnalyzer != nil && g.emotionAnalyzer.Enabled() {
		if emotionResult, err := g.emotionAnalyzer.AnalyzeText(r.Context(), text); err == nil && emotionResult != nil {
			emotionResult.Source = "audio"
			resp["emotion"] = emotionResult
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleVoices handles GET /v1/speech/voices — list available voices.
func (g *Gateway) handleVoices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "method not allowed")
		return
	}
	if g.speechReg == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "speech not configured")
		return
	}

	voices := g.speechReg.Voices()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"voices":    voices,
		"providers": g.speechReg.ListTTS(),
	})
}

// handleSTTStream handles WebSocket /v1/speech/stt/stream — real-time audio transcription.
// Client sends binary audio chunks via WebSocket; server transcribes each chunk and sends
// back JSON text results in real-time.
func (g *Gateway) handleSTTStream(w http.ResponseWriter, r *http.Request) {
	if g.speechReg == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "speech not configured")
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin:     g.checkWSOrigin,
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
			// Control message: {"action": "stop"} to end the stream
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

		// Transcribe the audio chunk
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		text, sttErr := g.speechReg.SpeechToText(ctx, data, speech.STTOptions{Language: lang})
		cancel()

		resp := map[string]any{"text": text, "final": false}
		if sttErr != nil {
			resp["error"] = sttErr.Error()
			resp["text"] = ""
		}

		// Optional emotion detection
		if text != "" && g.emotionAnalyzer != nil && g.emotionAnalyzer.Enabled() {
			if emotionResult, emotErr := g.emotionAnalyzer.AnalyzeText(r.Context(), text); emotErr == nil && emotionResult != nil {
				emotionResult.Source = "audio"
				resp["emotion"] = emotionResult
			}
		}

		respBytes, _ := json.Marshal(resp)
		if err := conn.WriteMessage(websocket.TextMessage, respBytes); err != nil {
			break
		}
	}

	// Send final message
	finalResp, _ := json.Marshal(map[string]any{"text": "", "final": true})
	conn.WriteMessage(websocket.TextMessage, finalResp)
}
