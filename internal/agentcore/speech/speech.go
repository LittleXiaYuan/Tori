package speech

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"sync"
	"time"
)

// TTSProvider converts text to speech audio.
type TTSProvider interface {
	Name() string
	TextToSpeech(ctx context.Context, text string, opts TTSOptions) ([]byte, error)
	Voices() []Voice
}

// STTProvider converts speech audio to text.
type STTProvider interface {
	Name() string
	SpeechToText(ctx context.Context, audio []byte, opts STTOptions) (string, error)
}

// TTSOptions for text-to-speech conversion.
type TTSOptions struct {
	Voice  string  `json:"voice,omitempty"`  // voice ID
	Speed  float64 `json:"speed,omitempty"`  // playback speed (0.5-2.0)
	Format string  `json:"format,omitempty"` // output format: mp3, wav, opus, flac
}

// EmotionTTSProfile maps an emotion to voice parameter adjustments.
type EmotionTTSProfile struct {
	Voice string  `json:"voice,omitempty"` // preferred voice override
	Speed float64 `json:"speed,omitempty"` // speed multiplier (applied * base speed)
}

// DefaultEmotionProfiles provides sensible TTS adjustments per emotion.
var DefaultEmotionProfiles = map[string]EmotionTTSProfile{
	"happy":     {Speed: 1.1},
	"sad":       {Speed: 0.85, Voice: "echo"},
	"angry":     {Speed: 1.15},
	"fearful":   {Speed: 0.9},
	"disgusted": {Speed: 0.95},
	"surprised": {Speed: 1.1},
	"neutral":   {},
}

// ApplyEmotion adjusts TTS options based on the detected emotion.
// If emotion is empty or unknown, options are returned unchanged.
func (o TTSOptions) ApplyEmotion(emotion string) TTSOptions {
	p, ok := DefaultEmotionProfiles[emotion]
	if !ok {
		return o
	}
	out := o
	if p.Voice != "" && out.Voice == "" {
		out.Voice = p.Voice
	}
	if p.Speed > 0 {
		base := out.Speed
		if base <= 0 {
			base = 1.0
		}
		s := base * p.Speed
		if s < 0.5 {
			s = 0.5
		} else if s > 2.0 {
			s = 2.0
		}
		out.Speed = s
	}
	return out
}

// STTOptions for speech-to-text conversion.
type STTOptions struct {
	Language string `json:"language,omitempty"` // language hint (e.g., "zh", "en")
	Format   string `json:"format,omitempty"`   // input audio format
}

// Voice represents an available TTS voice.
type Voice struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Language    string `json:"language"`
	Gender      string `json:"gender,omitempty"`
	Description string `json:"description,omitempty"`
}

// Registry manages TTS and STT providers.
type Registry struct {
	mu     sync.RWMutex
	tts    map[string]TTSProvider
	stt    map[string]STTProvider
	defTTS string // default TTS provider name
	defSTT string // default STT provider name
}

// NewRegistry creates a speech provider registry.
func NewRegistry() *Registry {
	return &Registry{
		tts: make(map[string]TTSProvider),
		stt: make(map[string]STTProvider),
	}
}

// RegisterTTS adds a TTS provider.
func (r *Registry) RegisterTTS(p TTSProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tts[p.Name()] = p
	if r.defTTS == "" {
		r.defTTS = p.Name()
	}
}

// RegisterSTT adds an STT provider.
func (r *Registry) RegisterSTT(p STTProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stt[p.Name()] = p
	if r.defSTT == "" {
		r.defSTT = p.Name()
	}
}

// TextToSpeech converts text using the default or specified TTS provider.
func (r *Registry) TextToSpeech(ctx context.Context, text string, opts TTSOptions) ([]byte, error) {
	r.mu.RLock()
	p, ok := r.tts[r.defTTS]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no TTS provider available")
	}
	return p.TextToSpeech(ctx, text, opts)
}

// SpeechToText converts audio using the default or specified STT provider.
func (r *Registry) SpeechToText(ctx context.Context, audio []byte, opts STTOptions) (string, error) {
	r.mu.RLock()
	p, ok := r.stt[r.defSTT]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("no STT provider available")
	}
	return p.SpeechToText(ctx, audio, opts)
}

// ListTTS returns all registered TTS provider names.
func (r *Registry) ListTTS() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tts))
	for k := range r.tts {
		names = append(names, k)
	}
	return names
}

// ListSTT returns all registered STT provider names.
func (r *Registry) ListSTT() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.stt))
	for k := range r.stt {
		names = append(names, k)
	}
	return names
}

// Voices returns all voices from all registered TTS providers.
func (r *Registry) Voices() []Voice {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var voices []Voice
	for _, p := range r.tts {
		voices = append(voices, p.Voices()...)
	}
	return voices
}

// ──────────────────────────────────────────
// OpenAI TTS/STT Provider
// ──────────────────────────────────────────

// OpenAITTS implements TTSProvider using OpenAI's TTS API.
type OpenAITTS struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

// NewOpenAITTS creates an OpenAI TTS provider.
func NewOpenAITTS(baseURL, apiKey, model string) *OpenAITTS {
	if model == "" {
		model = "tts-1"
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAITTS{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (o *OpenAITTS) Name() string { return "openai_tts" }

func (o *OpenAITTS) Voices() []Voice {
	return []Voice{
		{ID: "alloy", Name: "Alloy", Language: "multi", Gender: "neutral"},
		{ID: "echo", Name: "Echo", Language: "multi", Gender: "male"},
		{ID: "fable", Name: "Fable", Language: "multi", Gender: "female"},
		{ID: "onyx", Name: "Onyx", Language: "multi", Gender: "male"},
		{ID: "nova", Name: "Nova", Language: "multi", Gender: "female"},
		{ID: "shimmer", Name: "Shimmer", Language: "multi", Gender: "female"},
	}
}

func (o *OpenAITTS) TextToSpeech(ctx context.Context, text string, opts TTSOptions) ([]byte, error) {
	voice := opts.Voice
	if voice == "" {
		voice = "alloy"
	}
	format := opts.Format
	if format == "" {
		format = "mp3"
	}
	speed := opts.Speed
	if speed <= 0 {
		speed = 1.0
	}

	payload := map[string]any{
		"model":           o.model,
		"input":           text,
		"voice":           voice,
		"response_format": format,
		"speed":           speed,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := o.baseURL + "/audio/speech"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai tts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai tts: HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	// Limit read to 50MB
	audioData, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
	if err != nil {
		return nil, fmt.Errorf("openai tts: read response: %w", err)
	}

	slog.Debug("openai tts: generated audio", "text_len", len(text), "audio_bytes", len(audioData))
	return audioData, nil
}

// OpenAISTT implements STTProvider using OpenAI's Whisper API.
type OpenAISTT struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

// NewOpenAISTT creates an OpenAI STT (Whisper) provider.
func NewOpenAISTT(baseURL, apiKey, model string) *OpenAISTT {
	if model == "" {
		model = "whisper-1"
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &OpenAISTT{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

func (o *OpenAISTT) Name() string { return "openai_stt" }

func (o *OpenAISTT) SpeechToText(ctx context.Context, audio []byte, opts STTOptions) (string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add audio file
	part, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		return "", err
	}
	if _, err := part.Write(audio); err != nil {
		return "", err
	}

	// Add model
	writer.WriteField("model", o.model)

	// Add language hint
	if opts.Language != "" {
		writer.WriteField("language", opts.Language)
	}

	writer.WriteField("response_format", "json")
	writer.Close()

	url := o.baseURL + "/audio/transcriptions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai stt: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai stt: HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("openai stt: decode: %w", err)
	}

	slog.Debug("openai stt: transcribed", "audio_bytes", len(audio), "text_len", len(result.Text))
	return result.Text, nil
}

// ──────────────────────────────────────────
// Edge TTS Provider (Microsoft Edge Read Aloud)
// ──────────────────────────────────────────

// EdgeTTS implements TTSProvider using Edge TTS (free, no API key needed).
// Uses the Edge Read Aloud WebSocket API for speech synthesis.
type EdgeTTS struct {
	client *http.Client
}

// NewEdgeTTS creates an Edge TTS provider.
func NewEdgeTTS() *EdgeTTS {
	return &EdgeTTS{
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (e *EdgeTTS) Name() string { return "edge_tts" }

func (e *EdgeTTS) Voices() []Voice {
	return []Voice{
		{ID: "zh-CN-XiaoxiaoNeural", Name: "晓晓", Language: "zh-CN", Gender: "female", Description: "温暖亲切"},
		{ID: "zh-CN-YunxiNeural", Name: "云希", Language: "zh-CN", Gender: "male", Description: "阳光开朗"},
		{ID: "zh-CN-YunjianNeural", Name: "云健", Language: "zh-CN", Gender: "male", Description: "沉稳大气"},
		{ID: "zh-CN-XiaoyiNeural", Name: "晓伊", Language: "zh-CN", Gender: "female", Description: "温柔知性"},
		{ID: "en-US-JennyNeural", Name: "Jenny", Language: "en-US", Gender: "female"},
		{ID: "en-US-GuyNeural", Name: "Guy", Language: "en-US", Gender: "male"},
		{ID: "ja-JP-NanamiNeural", Name: "Nanami", Language: "ja-JP", Gender: "female"},
		{ID: "ko-KR-SunHiNeural", Name: "SunHi", Language: "ko-KR", Gender: "female"},
	}
}

// TextToSpeech uses the Edge TTS WebSocket API (free, no API key).
func (e *EdgeTTS) TextToSpeech(ctx context.Context, text string, opts TTSOptions) ([]byte, error) {
	voice := opts.Voice
	if voice == "" {
		voice = "zh-CN-XiaoxiaoNeural"
	}
	format := opts.Format
	if format == "" || format == "mp3" {
		format = "audio-24khz-48kbitrate-mono-mp3"
	}

	return edgeTTSSynthesize(ctx, text, voice, format)
}
