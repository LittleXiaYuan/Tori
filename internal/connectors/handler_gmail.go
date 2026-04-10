package connectors

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const gmailAPIBase = "https://gmail.googleapis.com/gmail/v1/users/me"

// GmailHandler implements ConnectorHandler for the Gmail REST API.
// Authentication: user pastes an OAuth2 Access Token (from Google OAuth playground or similar).
type GmailHandler struct {
	httpClient *http.Client
}

func NewGmailFullHandler() *GmailHandler {
	return &GmailHandler{httpClient: &http.Client{}}
}

func (h *GmailHandler) Connect(_ context.Context, creds *Credentials) (*Credentials, error) {
	return creds, nil
}

func (h *GmailHandler) Disconnect(_ context.Context, _ *Credentials) error { return nil }

func (h *GmailHandler) Refresh(_ context.Context, creds *Credentials) (*Credentials, error) {
	return creds, nil
}

func (h *GmailHandler) Validate(ctx context.Context, creds *Credentials) (bool, string, error) {
	result, err := h.doAPI(ctx, creds, http.MethodGet, "/profile", nil)
	if err != nil {
		return false, "", err
	}
	data, _ := result.(map[string]any)
	email, _ := data["emailAddress"].(string)
	if email == "" {
		email = "connected"
	}
	return true, email, nil
}

func (h *GmailHandler) Execute(ctx context.Context, creds *Credentials, actionID string, params map[string]any) (any, error) {
	switch actionID {
	case "list_messages":
		query := paramStr(params, "query", "")
		maxResults := paramStr(params, "max_results", "10")
		path := "/messages?maxResults=" + maxResults
		if query != "" {
			path += "&q=" + url.QueryEscape(query)
		}
		listResult, err := h.doAPI(ctx, creds, http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}
		data, _ := listResult.(map[string]any)
		msgs, _ := data["messages"].([]any)
		if len(msgs) == 0 {
			return map[string]any{"messages": []any{}, "count": 0}, nil
		}
		limit := 10
		if len(msgs) < limit {
			limit = len(msgs)
		}
		enriched := make([]any, 0, limit)
		for _, m := range msgs[:limit] {
			mObj, _ := m.(map[string]any)
			id, _ := mObj["id"].(string)
			if id == "" {
				continue
			}
			detail, err := h.doAPI(ctx, creds, http.MethodGet, "/messages/"+id+"?format=metadata&metadataHeaders=Subject&metadataHeaders=From&metadataHeaders=Date", nil)
			if err != nil {
				continue
			}
			enriched = append(enriched, detail)
		}
		return map[string]any{"messages": enriched, "count": len(enriched)}, nil

	case "get_message":
		msgID := paramStr(params, "message_id", "")
		if msgID == "" {
			return nil, fmt.Errorf("message_id is required")
		}
		return h.doAPI(ctx, creds, http.MethodGet, "/messages/"+msgID+"?format=full", nil)

	case "send_message":
		to := paramStr(params, "to", "")
		subject := paramStr(params, "subject", "")
		body := paramStr(params, "body", "")
		if to == "" || subject == "" || body == "" {
			return nil, fmt.Errorf("to, subject, and body are required")
		}
		raw := fmt.Sprintf("To: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s", to, subject, body)
		encoded := base64.RawURLEncoding.EncodeToString([]byte(raw))
		return h.doAPI(ctx, creds, http.MethodPost, "/messages/send", map[string]any{"raw": encoded})

	case "list_labels":
		return h.doAPI(ctx, creds, http.MethodGet, "/labels", nil)

	default:
		return nil, fmt.Errorf("unknown action: %s", actionID)
	}
}

func (h *GmailHandler) doAPI(ctx context.Context, creds *Credentials, method, path string, body any) (any, error) {
	var reader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, gmailAPIBase+path, reader)
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

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Gmail API error %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var result any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}
	return result, nil
}
