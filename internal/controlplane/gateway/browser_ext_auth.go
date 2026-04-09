package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func fetchBrowserExtensionIdentity(ctx context.Context, baseURL, token string) (browserExtensionIdentity, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return browserExtensionIdentity{}, fmt.Errorf("empty TORI_API_BASE_URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/extension/introspect", nil)
	if err != nil {
		return browserExtensionIdentity{}, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return browserExtensionIdentity{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return browserExtensionIdentity{}, fmt.Errorf("extension introspection failed: %s", resp.Status)
	}
	var payload struct {
		Success bool                     `json:"success"`
		Data    browserExtensionIdentity `json:"data"`
		Message string                   `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return browserExtensionIdentity{}, err
	}
	if !payload.Success {
		if payload.Message == "" {
			payload.Message = "extension introspection was rejected"
		}
		return browserExtensionIdentity{}, errors.New(payload.Message)
	}
	return payload.Data, nil
}
