package connectors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const gcalAPIBase = "https://www.googleapis.com/calendar/v3"

// GoogleCalendarHandler implements ConnectorHandler for Google Calendar REST API.
type GoogleCalendarHandler struct {
	httpClient *http.Client
}

func NewGoogleCalendarFullHandler() *GoogleCalendarHandler {
	return &GoogleCalendarHandler{httpClient: &http.Client{}}
}

func (h *GoogleCalendarHandler) Connect(_ context.Context, creds *Credentials) (*Credentials, error) {
	return creds, nil
}

func (h *GoogleCalendarHandler) Disconnect(_ context.Context, _ *Credentials) error { return nil }

func (h *GoogleCalendarHandler) Refresh(_ context.Context, creds *Credentials) (*Credentials, error) {
	return creds, nil
}

func (h *GoogleCalendarHandler) Validate(ctx context.Context, creds *Credentials) (bool, string, error) {
	result, err := h.doAPI(ctx, creds, http.MethodGet, "/calendars/primary", nil)
	if err != nil {
		return false, "", err
	}
	data, _ := result.(map[string]any)
	summary, _ := data["summary"].(string)
	if summary == "" {
		summary = "connected"
	}
	return true, summary, nil
}

func (h *GoogleCalendarHandler) Execute(ctx context.Context, creds *Credentials, actionID string, params map[string]any) (any, error) {
	switch actionID {
	case "list_events":
		timeMin := paramStr(params, "time_min", time.Now().Format(time.RFC3339))
		timeMax := paramStr(params, "time_max", time.Now().Add(7*24*time.Hour).Format(time.RFC3339))
		maxResults := paramStr(params, "max_results", "20")
		path := fmt.Sprintf("/calendars/primary/events?timeMin=%s&timeMax=%s&maxResults=%s&singleEvents=true&orderBy=startTime",
			url.QueryEscape(timeMin), url.QueryEscape(timeMax), maxResults)
		return h.doAPI(ctx, creds, http.MethodGet, path, nil)

	case "create_event":
		summary := paramStr(params, "summary", "")
		start := paramStr(params, "start", "")
		end := paramStr(params, "end", "")
		if summary == "" || start == "" || end == "" {
			return nil, fmt.Errorf("summary, start, and end are required")
		}
		body := map[string]any{
			"summary": summary,
			"start":   map[string]any{"dateTime": start},
			"end":     map[string]any{"dateTime": end},
		}
		if desc := paramStr(params, "description", ""); desc != "" {
			body["description"] = desc
		}
		return h.doAPI(ctx, creds, http.MethodPost, "/calendars/primary/events", body)

	case "delete_event":
		eventID := paramStr(params, "event_id", "")
		if eventID == "" {
			return nil, fmt.Errorf("event_id is required")
		}
		return h.doAPI(ctx, creds, http.MethodDelete, "/calendars/primary/events/"+eventID, nil)

	default:
		return nil, fmt.Errorf("unknown action: %s", actionID)
	}
}

func (h *GoogleCalendarHandler) doAPI(ctx context.Context, creds *Credentials, method, path string, body any) (any, error) {
	var reader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, gcalAPIBase+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return map[string]any{"ok": true}, nil
	}

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Google Calendar API error %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var result any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}
	return result, nil
}
