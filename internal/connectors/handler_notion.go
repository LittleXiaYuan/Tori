package connectors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const notionVersion = "2022-06-28"

// NotionHandler implements ConnectorHandler for the Notion REST API.
type NotionHandler struct {
	httpClient *http.Client
}

func NewNotionHandler() *NotionHandler {
	return &NotionHandler{httpClient: &http.Client{}}
}

func (h *NotionHandler) Connect(ctx context.Context, creds *Credentials) (*Credentials, error) {
	return creds, nil
}

func (h *NotionHandler) Disconnect(ctx context.Context, creds *Credentials) error {
	return nil
}

func (h *NotionHandler) Refresh(ctx context.Context, creds *Credentials) (*Credentials, error) {
	return creds, nil
}

func (h *NotionHandler) Validate(ctx context.Context, creds *Credentials) (bool, string, error) {
	result, err := h.doRequest(ctx, creds, http.MethodGet, "/users/me", nil)
	if err != nil {
		return false, "", err
	}

	userName := "connected"
	if data, ok := result.(map[string]any); ok {
		if name := nestedString(data, "name"); name != "" {
			userName = name
		} else if name := nestedString(data, "bot", "owner", "user", "name"); name != "" {
			userName = name
		} else if email := nestedString(data, "bot", "owner", "user", "person", "email"); email != "" {
			userName = email
		}
	}
	return true, userName, nil
}

func (h *NotionHandler) Execute(ctx context.Context, creds *Credentials, actionID string, params map[string]any) (any, error) {
	switch actionID {
	case "search":
		body := map[string]any{}
		if q := paramStr(params, "query", ""); q != "" {
			body["query"] = q
		}
		return h.doRequest(ctx, creds, http.MethodPost, "/search", body)
	case "get_page":
		pageID := paramStr(params, "page_id", "")
		if pageID == "" {
			return nil, fmt.Errorf("page_id is required")
		}
		return h.doRequest(ctx, creds, http.MethodGet, "/pages/"+pageID, nil)
	case "create_page":
		parentID := paramStr(params, "parent_id", "")
		title := paramStr(params, "title", "")
		if parentID == "" || title == "" {
			return nil, fmt.Errorf("parent_id and title are required")
		}

		body := map[string]any{
			"parent": map[string]any{
				"page_id": parentID,
			},
			"properties": map[string]any{
				"title": map[string]any{
					"title": []map[string]any{
						{
							"text": map[string]any{
								"content": title,
							},
						},
					},
				},
			},
		}

		if content := strings.TrimSpace(paramStr(params, "content", "")); content != "" {
			body["children"] = []map[string]any{
				{
					"object": "block",
					"type":   "paragraph",
					"paragraph": map[string]any{
						"rich_text": []map[string]any{
							{
								"type": "text",
								"text": map[string]any{
									"content": content,
								},
							},
						},
					},
				},
			}
		}

		return h.doRequest(ctx, creds, http.MethodPost, "/pages", body)
	default:
		return nil, fmt.Errorf("unknown action: %s", actionID)
	}
}

func (h *NotionHandler) doRequest(ctx context.Context, creds *Credentials, method, path string, body any) (any, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, "https://api.notion.com/v1"+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("Notion-Version", notionVersion)
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
		return nil, fmt.Errorf("Notion API error %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var result any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}
	return result, nil
}

func nestedString(data map[string]any, path ...string) string {
	var cur any = data
	for _, key := range path {
		next, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur, ok = next[key]
		if !ok {
			return ""
		}
	}
	s, _ := cur.(string)
	return s
}
