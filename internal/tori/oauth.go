package tori

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

type UserInfo struct {
	UserID           string `json:"user_id"`
	Username         string `json:"username"`
	Email            string `json:"email"`
	APIKey           string `json:"api_key"`
	SandboxAvailable bool   `json:"sandbox_available"`
}

type BindingStatus struct {
	Bound            bool   `json:"bound"`
	Username         string `json:"username,omitempty"`
	ToriURL          string `json:"tori_url,omitempty"`
	ExpiresAt        string `json:"expires_at,omitempty"`
	SandboxAvailable bool   `json:"sandbox_available"`
}

func GetBindingStatus(ts *TokenStore) BindingStatus {
	t := ts.Get()
	if t == nil {
		return BindingStatus{Bound: false}
	}
	sandboxOK := os.Getenv("SANDBOX_CLOUD_API_KEY") != "" || t.APIKey != ""
	return BindingStatus{
		Bound:            true,
		Username:         t.Username,
		ToriURL:          t.ToriBaseURL,
		ExpiresAt:        t.ExpiresAt.Format(time.RFC3339),
		SandboxAvailable: sandboxOK,
	}
}

type BindResult struct {
	Token    *TokenResponse
	UserInfo *UserInfo
	Err      error
}

func StartBindFlow(ctx context.Context, cfg OAuthConfig) (authorizeURL string, resultCh chan BindResult, err error) {
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return "", nil, fmt.Errorf("generate PKCE: %w", err)
	}

	resultCh = make(chan BindResult, 1)
	callbackPath := "/callback"

	listenAddr := fmt.Sprintf("127.0.0.1:%d", cfg.CallbackPort)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return "", nil, fmt.Errorf("listen callback: %w", err)
		}
	}
	actualPort := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d%s", actualPort, callbackPath)

	state := randomString(16)

	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {cfg.ClientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {cfg.Scopes},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {cfg.CodeChallengeMethod},
	}
	authorizeURL = strings.TrimRight(cfg.ToriBaseURL, "/") + "/oauth/authorize?" + params.Encode()

	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 30 * time.Second}

	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			go func() {
				time.Sleep(500 * time.Millisecond)
				srv.Close()
			}()
		}()

		if r.URL.Query().Get("state") != state {
			resultCh <- BindResult{Err: fmt.Errorf("state mismatch")}
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			errMsg := r.URL.Query().Get("error")
			resultCh <- BindResult{Err: fmt.Errorf("auth error: %s", errMsg)}
			http.Error(w, "no code received", http.StatusBadRequest)
			return
		}

		tok, err := exchangeCode(cfg, code, redirectURI, verifier)
		if err != nil {
			resultCh <- BindResult{Err: err}
			http.Error(w, "token exchange failed", http.StatusInternalServerError)
			return
		}

		info, err := fetchUserInfo(cfg, tok.AccessToken)
		if err != nil {
			slog.Warn("tori: fetch userinfo failed, continuing without it", "err", err)
		}

		if info != nil {
			result, err := fetchAgentToken(cfg, tok.AccessToken)
			if err != nil {
				slog.Error("tori: fetchAgentToken failed", "err", err)
			} else if result.Token == "" {
				slog.Warn("tori: fetchAgentToken returned empty token")
			} else {
				info.APIKey = strings.TrimPrefix(result.Token, "sk-")
				info.SandboxAvailable = result.SandboxAvailable
				slog.Info("tori: agent token obtained", "len", len(info.APIKey), "sandbox", result.SandboxAvailable)
			}
		} else {
			slog.Warn("tori: user info is nil, skipping agent token fetch")
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<html><body><h2>Authorization successful!</h2><p>You may close this window.</p></body></html>`)

		resultCh <- BindResult{Token: tok, UserInfo: info}
	})

	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			slog.Error("tori: callback server error", "err", err)
		}
		close(resultCh)
	}()

	go func() {
		timer := time.NewTimer(5 * time.Minute)
		defer timer.Stop()
		select {
		case <-timer.C:
			slog.Warn("tori: auth flow timed out after 5 minutes")
			srv.Close()
		case <-ctx.Done():
			srv.Close()
		}
	}()

	return authorizeURL, resultCh, nil
}

func RefreshAccessToken(cfg OAuthConfig, refreshToken string) (*TokenResponse, error) {
	tokenURL := strings.TrimRight(cfg.ToriBaseURL, "/") + "/oauth/token"
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {cfg.ClientID},
		"refresh_token": {refreshToken},
	}

	resp, err := http.PostForm(tokenURL, form)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", tokenURL, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh token: status %d: %s", resp.StatusCode, body)
	}

	var tok TokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("parse refresh response: %w", err)
	}
	return &tok, nil
}

func exchangeCode(cfg OAuthConfig, code, redirectURI, verifier string) (*TokenResponse, error) {
	tokenURL := strings.TrimRight(cfg.ToriBaseURL, "/") + "/oauth/token"
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {cfg.ClientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}

	resp, err := http.PostForm(tokenURL, form)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", tokenURL, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, body)
	}

	var tok TokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}
	return &tok, nil
}

func fetchUserInfo(cfg OAuthConfig, accessToken string) (*UserInfo, error) {
	userinfoURL := strings.TrimRight(cfg.ToriBaseURL, "/") + "/oauth/userinfo"
	req, _ := http.NewRequest("GET", userinfoURL, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo returned %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var info UserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

type agentTokenResult struct {
	Token            string `json:"token"`
	SandboxAvailable bool   `json:"sandbox_available"`
}

func fetchAgentToken(cfg OAuthConfig, accessToken string) (*agentTokenResult, error) {
	agentTokenURL := strings.TrimRight(cfg.ToriBaseURL, "/") + "/api/oauth/agent-token"
	slog.Debug("tori: fetching agent token", "url", agentTokenURL)

	req, err := http.NewRequest("GET", agentTokenURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	slog.Debug("tori: agent-token response", "status", resp.StatusCode, "body_len", len(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent-token returned %d: %s", resp.StatusCode, string(body))
	}

	var result agentTokenResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal: %w (body: %s)", err, string(body))
	}
	if result.Token == "" {
		slog.Warn("tori: agent-token response had empty token field", "body", string(body))
	}
	return &result, nil
}

func generatePKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return verifier, challenge, nil
}

func randomString(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:n]
}
