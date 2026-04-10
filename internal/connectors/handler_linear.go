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

// LinearHandler implements ConnectorHandler for Linear's GraphQL API.
type LinearHandler struct {
	httpClient *http.Client
}

func NewLinearHandler() *LinearHandler {
	return &LinearHandler{httpClient: &http.Client{}}
}

func (h *LinearHandler) Connect(ctx context.Context, creds *Credentials) (*Credentials, error) {
	return creds, nil
}

func (h *LinearHandler) Disconnect(ctx context.Context, creds *Credentials) error {
	return nil
}

func (h *LinearHandler) Refresh(ctx context.Context, creds *Credentials) (*Credentials, error) {
	return creds, nil
}

func (h *LinearHandler) Validate(ctx context.Context, creds *Credentials) (bool, string, error) {
	result, err := h.graphQL(ctx, creds, `query { viewer { id name email } }`, nil)
	if err != nil {
		return false, "", err
	}

	name := nestedString(result, "data", "viewer", "name")
	if name == "" {
		name = nestedString(result, "data", "viewer", "email")
	}
	if name == "" {
		name = "connected"
	}
	return true, name, nil
}

func (h *LinearHandler) Execute(ctx context.Context, creds *Credentials, actionID string, params map[string]any) (any, error) {
	switch actionID {
	case "list_issues":
		query := `query($first:Int!) { issues(first:$first) { nodes { id identifier title description url state { name } team { id key name } } } }`
		return h.graphQL(ctx, creds, query, map[string]any{"first": 25})
	case "create_issue":
		title := paramStr(params, "title", "")
		teamID := paramStr(params, "team_id", "")
		if title == "" || teamID == "" {
			return nil, fmt.Errorf("title and team_id are required")
		}

		mutation := `mutation($title:String!,$description:String,$teamId:String!) {
			issueCreate(input:{title:$title, description:$description, teamId:$teamId}) {
				success
				issue { id identifier title url }
			}
		}`
		vars := map[string]any{
			"title":       title,
			"description": paramStr(params, "description", ""),
			"teamId":      teamID,
		}

		result, err := h.graphQL(ctx, creds, mutation, vars)
		if err != nil {
			return nil, err
		}
		if ok := nestedBool(result, "data", "issueCreate", "success"); !ok {
			return nil, fmt.Errorf("linear rejected issue creation")
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unknown action: %s", actionID)
	}
}

func (h *LinearHandler) graphQL(ctx context.Context, creds *Credentials, query string, variables map[string]any) (map[string]any, error) {
	payload := map[string]any{"query": query}
	if variables != nil {
		payload["variables"] = variables
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.linear.app/graphql", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", creds.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Linear API error %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	if errorsVal, ok := result["errors"]; ok {
		return nil, fmt.Errorf("linear graphql error: %v", errorsVal)
	}
	return result, nil
}

func nestedBool(data map[string]any, path ...string) bool {
	var cur any = data
	for _, key := range path {
		next, ok := cur.(map[string]any)
		if !ok {
			return false
		}
		cur, ok = next[key]
		if !ok {
			return false
		}
	}
	v, _ := cur.(bool)
	return v
}
