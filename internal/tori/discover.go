package tori

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// DiscoveredModel represents a model available on a Tori instance.
type DiscoveredModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by,omitempty"`
	Created int64  `json:"created,omitempty"`
}

// HealthStatus represents a Tori instance health check response.
type HealthStatus struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Uptime  int64  `json:"uptime"`
	DB      bool   `json:"db"`
}

// CheckHealth pings the Tori health endpoint.
func CheckHealth(toriBaseURL string) (*HealthStatus, error) {
	healthURL := fmt.Sprintf("%s/api/health", strings.TrimRight(toriBaseURL, "/"))
	req, err := jsonSafeRequest(context.Background(), http.MethodGet, healthURL, nil, 5*time.Second)
	if err != nil {
		return nil, err
	}
	resp, err := doSafeRequest(req, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("health check: %w", err)
	}
	defer resp.Body.Close()

	var status HealthStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("decode health: %w", err)
	}
	return &status, nil
}

// UsageSummary represents aggregated usage data from Tori.
type UsageSummary struct {
	UserID           int   `json:"user_id"`
	RemainQuota      int64 `json:"remain_quota"`
	UsedQuota        int64 `json:"used_quota"`
	RequestCount     int64 `json:"request_count"`
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

// FetchUsage retrieves usage summary from a Tori instance.
func FetchUsage(toriBaseURL, apiKey string) (*UsageSummary, error) {
	usageURL := fmt.Sprintf("%s/api/usage/summary", strings.TrimRight(toriBaseURL, "/"))
	req, err := jsonSafeRequest(context.Background(), http.MethodGet, usageURL, nil, 10*time.Second)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := doSafeRequest(req, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("fetch usage: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch usage: status %d", resp.StatusCode)
	}
	var summary UsageSummary
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return nil, fmt.Errorf("decode usage: %w", err)
	}
	return &summary, nil
}

// DiscoverModels fetches the list of available models from a Tori instance.
// Uses the standard OpenAI-compatible /v1/models endpoint.
func DiscoverModels(toriBaseURL, apiKey string) ([]DiscoveredModel, error) {
	modelsURL := fmt.Sprintf("%s/v1/models", strings.TrimRight(toriBaseURL, "/"))

	req, err := jsonSafeRequest(context.Background(), http.MethodGet, modelsURL, nil, 15*time.Second)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := doSafeRequest(req, 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("discover models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discover models: status %d", resp.StatusCode)
	}

	var result struct {
		Data []DiscoveredModel `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode models: %w", err)
	}
	return result.Data, nil
}
