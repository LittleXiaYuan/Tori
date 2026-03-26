package gateway

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"yunque-agent/internal/apperror"
	"yunque-agent/internal/execution/browser"
)

// handleBrowserStatus returns the current browser engine state and capabilities.
func (g *Gateway) handleBrowserStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	status := map[string]any{
		"enabled":  g.browserEngine != nil,
		"headless": true,
		"state":    "disabled",
	}

	if g.browserEngine != nil {
		status["state"] = "running"
		status["headless"] = g.browserHeadless
		status["data_dir"] = g.browserDataDir
	}

	// OCR capabilities
	ocrCaps := []string{"dom"}
	if g.browserRecognizer != nil {
		caps := g.browserRecognizer.Capabilities()
		ocrCaps = make([]string, len(caps))
		for i, c := range caps {
			ocrCaps[i] = string(c)
		}
	}
	status["ocr_capabilities"] = ocrCaps

	// Worker state
	if g.browserWorker != nil {
		status["worker_state"] = string(g.browserWorker.State())
	}

	json.NewEncoder(w).Encode(status)
}

// handleBrowserConfig handles POST requests to change browser configuration at runtime.
func (g *Gateway) handleBrowserConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}

	var req struct {
		Enabled  *bool  `json:"enabled"`
		Headless *bool  `json:"headless"`
		DataDir  string `json:"data_dir,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid json")
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Handle disable
	if req.Enabled != nil && !*req.Enabled {
		if g.browserEngine != nil {
			g.browserEngine.Close()
			g.browserEngine = nil
			g.planner.SetBrowser(nil)
			slog.Info("browser: disabled via API")
		}
		json.NewEncoder(w).Encode(map[string]any{"status": "disabled"})
		return
	}

	// Handle enable or reconfigure
	headless := true
	if req.Headless != nil {
		headless = *req.Headless
	} else if g.browserEngine != nil {
		headless = g.browserHeadless
	}

	dataDir := g.browserDataDir
	if req.DataDir != "" {
		dataDir = req.DataDir
	}
	if dataDir == "" {
		dataDir = "data/browser"
	}

	// If engine is running with different settings, restart it
	if g.browserEngine != nil && (headless != g.browserHeadless) {
		slog.Info("browser: restarting with new config", "headless", headless)
		g.browserEngine.Close()
		g.browserEngine = nil
	}

	// Start engine if not running
	if g.browserEngine == nil {
		timeout := 30 * time.Second
		if t := os.Getenv("BROWSER_TIMEOUT"); t != "" {
			if d, err := time.ParseDuration(t); err == nil {
				timeout = d
			}
		}

		engine, err := browser.New(browser.Config{
			Headless: headless,
			Timeout:  timeout,
			DataDir:  dataDir,
		})
		if err != nil {
			apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "browser start failed", err))
			return
		}

		g.browserEngine = engine
		g.browserHeadless = headless
		g.browserDataDir = dataDir

		dispatcher := browser.NewDispatcher(engine, dataDir)
		g.planner.SetBrowser(dispatcher)

		// Init OCR recognizer
		recognizer := browser.NewRecognizer(browser.RecognizerConfig{
			Engine:        engine,
			TesseractBin:  os.Getenv("TESSERACT_BIN"),
			VisionBaseURL: os.Getenv("VISION_BASE_URL"),
			VisionAPIKey:  os.Getenv("VISION_API_KEY"),
			VisionModel:   os.Getenv("VISION_MODEL"),
		})
		g.browserRecognizer = recognizer

		slog.Info("browser: started via API", "headless", headless, "data_dir", dataDir)
	}

	json.NewEncoder(w).Encode(map[string]any{
		"status":   "running",
		"headless": g.browserHeadless,
		"data_dir": g.browserDataDir,
	})
}
