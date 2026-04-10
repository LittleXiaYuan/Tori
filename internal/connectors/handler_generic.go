package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GenericRESTHandler implements ConnectorHandler for simple REST APIs
// that authenticate via a Bearer token or API key header.
type GenericRESTHandler struct {
	BaseURL        string
	AuthHeader     string // e.g., "Authorization", "X-API-Key"
	AuthPrefix     string // e.g., "Bearer ", "Bot ", ""
	ValidateURL    string // GET endpoint to validate token (must return 200)
	ValidateMethod string // default GET
	UserField      string // JSON field for user display name in validate response
	SuccessField   string // optional boolean response field that must be true
	Headers        map[string]string
	httpClient     *http.Client
	actionMap      map[string]ActionConfig
}

// ActionConfig maps an action ID to a REST call.
type ActionConfig struct {
	Method   string
	PathTmpl string // e.g., "/repos/{owner}/{repo}/issues"
	BodyKeys []string
}

func NewGenericRESTHandler(baseURL, authHeader, authPrefix, validateURL, userField string) *GenericRESTHandler {
	return &GenericRESTHandler{
		BaseURL:        baseURL,
		AuthHeader:     authHeader,
		AuthPrefix:     authPrefix,
		ValidateURL:    validateURL,
		ValidateMethod: http.MethodGet,
		UserField:      userField,
		Headers:        make(map[string]string),
		httpClient:     &http.Client{},
		actionMap:      make(map[string]ActionConfig),
	}
}

func (h *GenericRESTHandler) RegisterAction(id string, cfg ActionConfig) {
	h.actionMap[id] = cfg
}

func (h *GenericRESTHandler) Connect(ctx context.Context, creds *Credentials) (*Credentials, error) {
	return creds, nil
}

func (h *GenericRESTHandler) Disconnect(ctx context.Context, creds *Credentials) error {
	return nil
}

func (h *GenericRESTHandler) Refresh(ctx context.Context, creds *Credentials) (*Credentials, error) {
	return creds, nil
}

func (h *GenericRESTHandler) Validate(ctx context.Context, creds *Credentials) (bool, string, error) {
	if h.ValidateURL == "" {
		return true, "connected", nil
	}

	method := h.ValidateMethod
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, h.ValidateURL, nil)
	if err != nil {
		return false, "", err
	}
	h.setAuth(req, creds)
	h.setHeaders(req)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false, "", fmt.Errorf("validation returned %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	userName := "connected"
	if h.SuccessField != "" || h.UserField != "" {
		var data map[string]any
		if json.Unmarshal(body, &data) == nil {
			if h.SuccessField != "" {
				v, ok := data[h.SuccessField].(bool)
				if !ok || !v {
					if msg, ok := data["error"].(string); ok && msg != "" {
						return false, "", errors.New(msg)
					}
					return false, "", fmt.Errorf("validation rejected by upstream API")
				}
			}
			if v, ok := data[h.UserField].(string); ok && v != "" {
				userName = v
			}
		}
	}
	return true, userName, nil
}

func (h *GenericRESTHandler) Execute(ctx context.Context, creds *Credentials, actionID string, params map[string]any) (any, error) {
	cfg, ok := h.actionMap[actionID]
	if !ok {
		return nil, fmt.Errorf("unknown action: %s", actionID)
	}

	path := cfg.PathTmpl
	for k, v := range params {
		if s, ok := v.(string); ok {
			path = strings.ReplaceAll(path, "{"+k+"}", s)
		}
	}

	var bodyReader io.Reader
	if cfg.Method == "POST" || cfg.Method == "PUT" || cfg.Method == "PATCH" {
		bodyMap := make(map[string]any)
		if len(cfg.BodyKeys) > 0 {
			for _, k := range cfg.BodyKeys {
				if v, ok := params[k]; ok {
					bodyMap[k] = v
				}
			}
		} else {
			bodyMap = params
		}
		data, _ := json.Marshal(bodyMap)
		bodyReader = strings.NewReader(string(data))
	}

	url := h.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, cfg.Method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	h.setAuth(req, creds)
	h.setHeaders(req)
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result any
	if json.Unmarshal(respBody, &result) != nil {
		return string(respBody), nil
	}
	if h.SuccessField != "" {
		if data, ok := result.(map[string]any); ok {
			v, ok := data[h.SuccessField].(bool)
			if !ok || !v {
				if msg, ok := data["error"].(string); ok && msg != "" {
					return nil, errors.New(msg)
				}
				return nil, fmt.Errorf("upstream API rejected request")
			}
		}
	}
	return result, nil
}

func (h *GenericRESTHandler) setAuth(req *http.Request, creds *Credentials) {
	token := creds.AccessToken
	if token == "" {
		token = creds.APIKey
	}
	req.Header.Set(h.AuthHeader, h.AuthPrefix+token)
}

func (h *GenericRESTHandler) setHeaders(req *http.Request) {
	for k, v := range h.Headers {
		req.Header.Set(k, v)
	}
}

// ─── Preset generic handlers ─────────────────────────

func NewSlackHandler() *GenericRESTHandler {
	h := NewGenericRESTHandler(
		"https://slack.com/api",
		"Authorization", "Bearer ",
		"https://slack.com/api/auth.test",
		"user",
	)
	h.SuccessField = "ok"
	h.RegisterAction("send_message", ActionConfig{Method: "POST", PathTmpl: "/chat.postMessage", BodyKeys: []string{"channel", "text"}})
	h.RegisterAction("list_channels", ActionConfig{Method: "GET", PathTmpl: "/conversations.list?limit=100"})
	return h
}

func NewJiraHandler(baseURL string) *GenericRESTHandler {
	h := NewGenericRESTHandler(
		baseURL+"/rest/api/3",
		"Authorization", "Basic ",
		baseURL+"/rest/api/3/myself",
		"displayName",
	)
	h.RegisterAction("search_issues", ActionConfig{Method: "GET", PathTmpl: "/search?jql={jql}"})
	h.RegisterAction("create_issue", ActionConfig{Method: "POST", PathTmpl: "/issue", BodyKeys: []string{"project", "summary", "issue_type", "description"}})
	return h
}
