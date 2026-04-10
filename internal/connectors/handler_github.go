package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GitHubHandler implements ConnectorHandler for GitHub.
type GitHubHandler struct {
	httpClient *http.Client
}

func NewGitHubHandler() *GitHubHandler {
	return &GitHubHandler{httpClient: &http.Client{}}
}

func (h *GitHubHandler) Connect(ctx context.Context, creds *Credentials) (*Credentials, error) {
	return creds, nil
}

func (h *GitHubHandler) Disconnect(ctx context.Context, creds *Credentials) error {
	return nil
}

func (h *GitHubHandler) Refresh(ctx context.Context, creds *Credentials) (*Credentials, error) {
	return creds, nil // PAT tokens don't expire
}

func (h *GitHubHandler) Validate(ctx context.Context, creds *Credentials) (bool, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return false, "", err
	}
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false, "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var user struct {
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return false, "", err
	}

	name := user.Name
	if name == "" {
		name = user.Login
	}
	return true, name, nil
}

func (h *GitHubHandler) Execute(ctx context.Context, creds *Credentials, actionID string, params map[string]any) (any, error) {
	switch actionID {
	case "list_repos":
		return h.doAPI(ctx, creds, "GET", "/user/repos?per_page=30&sort="+paramStr(params, "sort", "updated"), nil)
	case "get_repo":
		owner := paramStr(params, "owner", "")
		repo := paramStr(params, "repo", "")
		if owner == "" || repo == "" {
			return nil, fmt.Errorf("owner and repo are required")
		}
		return h.doAPI(ctx, creds, "GET", fmt.Sprintf("/repos/%s/%s", owner, repo), nil)
	case "list_issues":
		owner := paramStr(params, "owner", "")
		repo := paramStr(params, "repo", "")
		state := paramStr(params, "state", "open")
		return h.doAPI(ctx, creds, "GET", fmt.Sprintf("/repos/%s/%s/issues?state=%s&per_page=30", owner, repo, state), nil)
	case "create_issue":
		owner := paramStr(params, "owner", "")
		repo := paramStr(params, "repo", "")
		body := map[string]any{"title": params["title"]}
		if b, ok := params["body"].(string); ok {
			body["body"] = b
		}
		return h.doAPI(ctx, creds, "POST", fmt.Sprintf("/repos/%s/%s/issues", owner, repo), body)
	case "list_prs":
		owner := paramStr(params, "owner", "")
		repo := paramStr(params, "repo", "")
		state := paramStr(params, "state", "open")
		return h.doAPI(ctx, creds, "GET", fmt.Sprintf("/repos/%s/%s/pulls?state=%s&per_page=30", owner, repo, state), nil)
	case "search_code":
		query := paramStr(params, "query", "")
		return h.doAPI(ctx, creds, "GET", "/search/code?q="+query, nil)
	case "get_file":
		owner := paramStr(params, "owner", "")
		repo := paramStr(params, "repo", "")
		path := paramStr(params, "path", "")
		ref := paramStr(params, "ref", "")
		url := fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, path)
		if ref != "" {
			url += "?ref=" + ref
		}
		return h.doAPI(ctx, creds, "GET", url, nil)
	default:
		return nil, fmt.Errorf("unknown action: %s", actionID)
	}
}

func (h *GitHubHandler) doAPI(ctx context.Context, creds *Credentials, method, path string, body any) (any, error) {
	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = strings.NewReader(string(data))
	}

	url := "https://api.github.com" + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
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
		return nil, fmt.Errorf("GitHub API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil
	}
	return result, nil
}

func paramStr(params map[string]any, key, fallback string) string {
	if v, ok := params[key].(string); ok && v != "" {
		return v
	}
	return fallback
}
