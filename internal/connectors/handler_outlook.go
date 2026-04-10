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

const msgraphBase = "https://graph.microsoft.com/v1.0/me"

// OutlookMailHandler implements ConnectorHandler for Microsoft Graph Mail API.
type OutlookMailHandler struct {
	httpClient *http.Client
}

func NewOutlookMailFullHandler() *OutlookMailHandler {
	return &OutlookMailHandler{httpClient: &http.Client{}}
}

func (h *OutlookMailHandler) Connect(_ context.Context, creds *Credentials) (*Credentials, error) {
	return creds, nil
}
func (h *OutlookMailHandler) Disconnect(_ context.Context, _ *Credentials) error { return nil }
func (h *OutlookMailHandler) Refresh(_ context.Context, creds *Credentials) (*Credentials, error) {
	return creds, nil
}

func (h *OutlookMailHandler) Validate(ctx context.Context, creds *Credentials) (bool, string, error) {
	result, err := graphGet(ctx, h.httpClient, creds, "")
	if err != nil {
		return false, "", err
	}
	data, _ := result.(map[string]any)
	name, _ := data["displayName"].(string)
	if name == "" {
		name, _ = data["userPrincipalName"].(string)
	}
	if name == "" {
		name = "connected"
	}
	return true, name, nil
}

func (h *OutlookMailHandler) Execute(ctx context.Context, creds *Credentials, actionID string, params map[string]any) (any, error) {
	switch actionID {
	case "list_messages":
		return graphGet(ctx, h.httpClient, creds, "/messages?$top=20&$orderby=receivedDateTime+desc&$select=id,subject,from,receivedDateTime,isRead")
	case "get_message":
		id := paramStr(params, "message_id", "")
		if id == "" {
			return nil, fmt.Errorf("message_id is required")
		}
		return graphGet(ctx, h.httpClient, creds, "/messages/"+id)
	case "send_message":
		to := paramStr(params, "to", "")
		subject := paramStr(params, "subject", "")
		body := paramStr(params, "body", "")
		if to == "" || subject == "" || body == "" {
			return nil, fmt.Errorf("to, subject, and body are required")
		}
		payload := map[string]any{
			"message": map[string]any{
				"subject": subject,
				"body":    map[string]any{"contentType": "Text", "content": body},
				"toRecipients": []map[string]any{
					{"emailAddress": map[string]any{"address": to}},
				},
			},
		}
		return graphPost(ctx, h.httpClient, creds, "/sendMail", payload)
	default:
		return nil, fmt.Errorf("unknown action: %s", actionID)
	}
}

// OutlookCalendarHandler implements ConnectorHandler for Microsoft Graph Calendar API.
type OutlookCalendarHandler struct {
	httpClient *http.Client
}

func NewOutlookCalendarFullHandler() *OutlookCalendarHandler {
	return &OutlookCalendarHandler{httpClient: &http.Client{}}
}

func (h *OutlookCalendarHandler) Connect(_ context.Context, creds *Credentials) (*Credentials, error) {
	return creds, nil
}
func (h *OutlookCalendarHandler) Disconnect(_ context.Context, _ *Credentials) error { return nil }
func (h *OutlookCalendarHandler) Refresh(_ context.Context, creds *Credentials) (*Credentials, error) {
	return creds, nil
}

func (h *OutlookCalendarHandler) Validate(ctx context.Context, creds *Credentials) (bool, string, error) {
	result, err := graphGet(ctx, h.httpClient, creds, "")
	if err != nil {
		return false, "", err
	}
	data, _ := result.(map[string]any)
	name, _ := data["displayName"].(string)
	if name == "" {
		name = "connected"
	}
	return true, name, nil
}

func (h *OutlookCalendarHandler) Execute(ctx context.Context, creds *Credentials, actionID string, params map[string]any) (any, error) {
	switch actionID {
	case "list_events":
		start := paramStr(params, "start", time.Now().Format(time.RFC3339))
		end := paramStr(params, "end", time.Now().Add(7*24*time.Hour).Format(time.RFC3339))
		path := fmt.Sprintf("/calendarView?startDateTime=%s&endDateTime=%s&$top=25&$orderby=start/dateTime", url.QueryEscape(start), url.QueryEscape(end))
		return graphGet(ctx, h.httpClient, creds, path)
	case "create_event":
		subject := paramStr(params, "subject", "")
		startTime := paramStr(params, "start", "")
		endTime := paramStr(params, "end", "")
		if subject == "" || startTime == "" || endTime == "" {
			return nil, fmt.Errorf("subject, start, and end are required")
		}
		body := map[string]any{
			"subject": subject,
			"start":   map[string]any{"dateTime": startTime, "timeZone": "UTC"},
			"end":     map[string]any{"dateTime": endTime, "timeZone": "UTC"},
		}
		return graphPost(ctx, h.httpClient, creds, "/events", body)
	default:
		return nil, fmt.Errorf("unknown action: %s", actionID)
	}
}

func graphGet(ctx context.Context, client *http.Client, creds *Credentials, path string) (any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, msgraphBase+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Microsoft Graph error %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var result any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}
	return result, nil
}

func graphPost(ctx context.Context, client *http.Client, creds *Credentials, path string, body any) (any, error) {
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, msgraphBase+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusNoContent {
		return map[string]any{"ok": true}, nil
	}
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Microsoft Graph error %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var result any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}
	return result, nil
}
