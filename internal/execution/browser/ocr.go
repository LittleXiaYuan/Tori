package browser

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// RecognizeMode indicates which OCR tier is available.
type RecognizeMode string

const (
	RecognizeDom    RecognizeMode = "dom"    // DOM text extraction (always available)
	RecognizeOCR    RecognizeMode = "ocr"    // Tesseract local OCR
	RecognizeVision RecognizeMode = "vision" // Vision LLM (GPT-4o, Claude, etc.)
	RecognizeHuman  RecognizeMode = "human"  // Fallback: ask user via OPP
)

// Recognizer provides a 4-tier text recognition fallback chain:
//
//	DOM → Tesseract → Vision LLM → Human (OPP)
type Recognizer struct {
	engine       *Engine // browser engine for DOM + screenshots
	tesseractBin string  // path to tesseract binary (empty = disabled)

	// Vision LLM (optional)
	visionBaseURL string
	visionAPIKey  string
	visionModel   string
}

// RecognizerConfig configures the text recognition chain.
type RecognizerConfig struct {
	Engine       *Engine
	TesseractBin string // e.g. "tesseract" or "C:/Program Files/Tesseract-OCR/tesseract.exe"

	// Vision LLM — all 3 must be set to enable
	VisionBaseURL string // e.g. "https://api.openai.com/v1"
	VisionAPIKey  string
	VisionModel   string // e.g. "gpt-4o"
}

func NewRecognizer(cfg RecognizerConfig) *Recognizer {
	bin := cfg.TesseractBin
	if bin == "" {
		// Try auto-detect
		if path, err := exec.LookPath("tesseract"); err == nil {
			bin = path
			slog.Info("browser/ocr: tesseract auto-detected", "path", bin)
		}
	}
	return &Recognizer{
		engine:        cfg.Engine,
		tesseractBin:  bin,
		visionBaseURL: cfg.VisionBaseURL,
		visionAPIKey:  cfg.VisionAPIKey,
		visionModel:   cfg.VisionModel,
	}
}

// Capabilities returns which recognition tiers are available.
func (r *Recognizer) Capabilities() []RecognizeMode {
	caps := []RecognizeMode{RecognizeDom} // always
	if r.tesseractBin != "" {
		caps = append(caps, RecognizeOCR)
	}
	if r.visionBaseURL != "" && r.visionAPIKey != "" && r.visionModel != "" {
		caps = append(caps, RecognizeVision)
	}
	caps = append(caps, RecognizeHuman) // always available as last resort
	return caps
}

// RecognizeResult holds text extraction output.
type RecognizeResult struct {
	Text       string        `json:"text"`
	Mode       RecognizeMode `json:"mode"`        // which tier succeeded
	Confidence float64       `json:"confidence"`   // 0-1, rough estimate
	NeedHuman  bool          `json:"need_human"`   // true = all auto tiers failed
}

// ReadTextWithFallback tries to extract text using the full fallback chain.
// selector: CSS selector for the target element (empty = full page).
// hint: describes what we're looking for (e.g. "验证码文字").
func (r *Recognizer) ReadTextWithFallback(ctx context.Context, selector, hint string) *RecognizeResult {
	// Tier 1: DOM
	text, err := r.engine.ReadText(selector)
	if err == nil && strings.TrimSpace(text) != "" {
		slog.Info("browser/ocr: DOM extraction succeeded", "len", len(text))
		return &RecognizeResult{Text: text, Mode: RecognizeDom, Confidence: 0.95}
	}

	// Tier 2: Tesseract OCR (screenshot → local OCR)
	if r.tesseractBin != "" {
		ocrText, err := r.runTesseract(ctx, selector)
		if err == nil && strings.TrimSpace(ocrText) != "" {
			slog.Info("browser/ocr: Tesseract succeeded", "len", len(ocrText))
			return &RecognizeResult{Text: ocrText, Mode: RecognizeOCR, Confidence: 0.7}
		}
		if err != nil {
			slog.Warn("browser/ocr: Tesseract failed", "err", err)
		}
	}

	// Tier 3: Vision LLM
	if r.visionBaseURL != "" && r.visionAPIKey != "" {
		visionText, err := r.runVisionLLM(ctx, hint)
		if err == nil && strings.TrimSpace(visionText) != "" {
			slog.Info("browser/ocr: Vision LLM succeeded", "len", len(visionText))
			return &RecognizeResult{Text: visionText, Mode: RecognizeVision, Confidence: 0.9}
		}
		if err != nil {
			slog.Warn("browser/ocr: Vision LLM failed", "err", err)
		}
	}

	// Tier 4: Need human
	slog.Info("browser/ocr: all auto tiers failed, need human")
	return &RecognizeResult{NeedHuman: true, Mode: RecognizeHuman}
}

// runTesseract screenshots the element/page and runs Tesseract OCR.
func (r *Recognizer) runTesseract(ctx context.Context, selector string) (string, error) {
	tmpFile, err := os.CreateTemp("", "yunque-ocr-*.png")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	if err := r.engine.Screenshot(tmpFile.Name()); err != nil {
		return "", fmt.Errorf("screenshot for OCR: %w", err)
	}

	// Run: tesseract input.png stdout -l chi_sim+eng
	cmd := exec.CommandContext(ctx, r.tesseractBin, tmpFile.Name(), "stdout", "-l", "chi_sim+eng")
	out, err := cmd.Output()
	if err != nil {
		// Fallback: try without chi_sim (English only)
		cmd = exec.CommandContext(ctx, r.tesseractBin, tmpFile.Name(), "stdout")
		out, err = cmd.Output()
		if err != nil {
			return "", fmt.Errorf("tesseract: %w", err)
		}
	}
	return strings.TrimSpace(string(out)), nil
}

// runVisionLLM screenshots and sends to a vision-capable LLM via OpenAI API.
func (r *Recognizer) runVisionLLM(ctx context.Context, hint string) (string, error) {
	tmpFile, err := os.CreateTemp("", "yunque-vision-*.png")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	if err := r.engine.Screenshot(tmpFile.Name()); err != nil {
		return "", fmt.Errorf("screenshot for vision: %w", err)
	}

	imgData, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", err
	}
	b64 := base64.StdEncoding.EncodeToString(imgData)

	prompt := "请识别这张截图中的文字内容。只返回识别出的文字。"
	if hint != "" {
		prompt = fmt.Sprintf("请识别这张截图中的%s。只返回识别出的文字，不要其他解释。", hint)
	}

	// OpenAI vision API format
	reqBody := map[string]any{
		"model": r.visionModel,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": prompt},
					{"type": "image_url", "image_url": map[string]string{
						"url": "data:image/png;base64," + b64,
					}},
				},
			},
		},
		"max_tokens": 500,
	}

	body, _ := json.Marshal(reqBody)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		r.visionBaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+r.visionAPIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("vision API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("vision API %d: %.300s", resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("vision decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("vision: no choices")
	}
	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}
