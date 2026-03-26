package gateway

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"yunque-agent/internal/apperror"
	"yunque-agent/internal/execution/browser"
)

// handleBrowserScreenshotLatest returns the latest screenshot as base64 JPEG.
func (g *Gateway) handleBrowserScreenshotLatest(w http.ResponseWriter, r *http.Request) {
	if g.browserEngine == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "browser not running")
		return
	}

	data, err := g.browserEngine.ScreenshotBytes()
	if err != nil {
		apperror.Write(w, apperror.Wrap(apperror.CodeInternal, "screenshot failed", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"image": base64.StdEncoding.EncodeToString(data),
		"url":   g.browserEngine.CurrentURL(),
	})
}

// handleOPPPending returns all pending OPP problems awaiting human decision.
func (g *Gateway) handleOPPPending(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var problems []browser.ProblemData
	if g.browserNotifier != nil {
		problems = g.browserNotifier.PendingProblems()
	}
	if problems == nil {
		problems = []browser.ProblemData{}
	}
	json.NewEncoder(w).Encode(map[string]any{
		"problems": problems,
		"count":    len(problems),
	})
}

// handleOPPDecide processes a human decision for a pending OPP problem.
func (g *Gateway) handleOPPDecide(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}

	var req browser.DecideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ProblemID == "" {
		apperror.WriteCode(w, apperror.CodeMissingField, "problem_id is required")
		return
	}

	if g.browserNotifier == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "notifier not available")
		return
	}

	ok := g.browserNotifier.ResolveProblem(req.ProblemID, req.Decision)
	if !ok {
		apperror.WriteCode(w, apperror.CodeNotFound, "problem not found or already resolved")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":     "resolved",
		"problem_id": req.ProblemID,
	})
}
