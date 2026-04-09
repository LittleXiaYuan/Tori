package tori

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type OAuthConfig struct {
	ToriBaseURL string // e.g. "https://tori.example.com"
	ClientID    string // public client ID (no secret — PKCE)
	Scopes      []string
}

func DefaultOAuthConfig() OAuthConfig {
	return OAuthConfig{
		ClientID: "yunque-agent",
		Scopes:   []string{"openid", "profile", "api"},
	}
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

type UserInfo struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	APIKey   string `json:"api_key"`
	Avatar   string `json:"avatar,omitempty"`
}

type pkceParams struct {
	Verifier  string
	Challenge string
	State     string
}

// StartBindFlow starts the OAuth2 PKCE authorization flow. It opens a local
// HTTP server to receive the callback, constructs the authorization URL, and
// returns the URL the user should open in their browser along with a channel
// that will receive the token response.
func StartBindFlow(ctx context.Context, cfg OAuthConfig) (authorizeURL string, resultCh <-chan BindResult, err error) {
	if cfg.ToriBaseURL == "" {
		return "", nil, fmt.Errorf("tori base URL not configured")
	}

	pkce, err := generatePKCE()
	if err != nil {
		return "", nil, fmt.Errorf("generate PKCE: %w", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, fmt.Errorf("listen for callback: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	ch := make(chan BindResult, 1)
	srv := &http.Server{}

	var once sync.Once
	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() {
			defer func() {
				go func() {
					time.Sleep(500 * time.Millisecond)
					srv.Shutdown(context.Background())
				}()
			}()

			result := handleCallback(r, cfg, pkce, redirectURI)
			ch <- result

			if result.Err != nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				fmt.Fprintf(w, `<html><body><h2>绑定失败 / Bind Failed</h2><p>%s</p><p>请关闭此页面。</p></body></html>`, result.Err)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<html><body><h2>绑定成功！/ Bind Successful!</h2><p>已成功绑定 Tori 账号，请关闭此页面回到云雀。</p></body></html>`)
		})
	})

	go func() {
		srv.Serve(listener)
		close(ch)
	}()

	go func() {
		select {
		case <-ctx.Done():
			srv.Shutdown(context.Background())
		case <-time.After(5 * time.Minute):
			srv.Shutdown(context.Background())
		}
	}()

	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {cfg.ClientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {strings.Join(cfg.Scopes, " ")},
		"state":                 {pkce.State},
		"code_challenge":        {pkce.Challenge},
		"code_challenge_method": {"S256"},
	}
	authorizeURL = fmt.Sprintf("%s/oauth/authorize?%s",
		strings.TrimRight(cfg.ToriBaseURL, "/"), params.Encode())

	return authorizeURL, ch, nil
}

type BindResult struct {
	Token    *TokenResponse
	UserInfo *UserInfo
	Err      error
}

func handleCallback(r *http.Request, cfg OAuthConfig, pkce *pkceParams, redirectURI string) BindResult {
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		desc := r.URL.Query().Get("error_description")
		return BindResult{Err: fmt.Errorf("%s: %s", errMsg, desc)}
	}

	state := r.URL.Query().Get("state")
	if state != pkce.State {
		return BindResult{Err: fmt.Errorf("state mismatch")}
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		return BindResult{Err: fmt.Errorf("missing authorization code")}
	}

	token, err := exchangeCode(cfg, code, pkce.Verifier, redirectURI)
	if err != nil {
		return BindResult{Err: fmt.Errorf("token exchange: %w", err)}
	}

	userInfo, err := fetchUserInfo(cfg, token.AccessToken)
	if err != nil {
		return BindResult{Token: token, Err: fmt.Errorf("fetch userinfo: %w", err)}
	}

	agentToken, err := fetchAgentToken(cfg, token.AccessToken)
	if err == nil && userInfo != nil {
		userInfo.APIKey = agentToken
	}

	return BindResult{Token: token, UserInfo: userInfo}
}

func exchangeCode(cfg OAuthConfig, code, verifier, redirectURI string) (*TokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {cfg.ClientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}

	tokenURL := fmt.Sprintf("%s/oauth/token", strings.TrimRight(cfg.ToriBaseURL, "/"))
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.PostForm(tokenURL, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d", resp.StatusCode)
	}

	var token TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	return &token, nil
}

func fetchUserInfo(cfg OAuthConfig, accessToken string) (*UserInfo, error) {
	userinfoURL := fmt.Sprintf("%s/oauth/userinfo", strings.TrimRight(cfg.ToriBaseURL, "/"))

	req, err := http.NewRequest("GET", userinfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo endpoint returned %d", resp.StatusCode)
	}

	var info UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode userinfo: %w", err)
	}
	return &info, nil
}

type agentTokenResponse struct {
	Token string `json:"token"`
}

func fetchAgentToken(cfg OAuthConfig, accessToken string) (string, error) {
	tokenURL := fmt.Sprintf("%s/api/oauth/agent-token", strings.TrimRight(cfg.ToriBaseURL, "/"))

	req, err := http.NewRequest("GET", tokenURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("agent token endpoint returned %d", resp.StatusCode)
	}

	var payload agentTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode agent token: %w", err)
	}
	if strings.TrimSpace(payload.Token) == "" {
		return "", fmt.Errorf("empty agent token")
	}
	return strings.TrimSpace(payload.Token), nil
}

// RefreshAccessToken uses a refresh_token to obtain a new access_token.
func RefreshAccessToken(cfg OAuthConfig, refreshToken string) (*TokenResponse, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {cfg.ClientID},
		"refresh_token": {refreshToken},
	}

	tokenURL := fmt.Sprintf("%s/oauth/token", strings.TrimRight(cfg.ToriBaseURL, "/"))
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.PostForm(tokenURL, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh returned %d", resp.StatusCode)
	}

	var token TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}
	return &token, nil
}

func generatePKCE() (*pkceParams, error) {
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, err
	}
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)

	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])

	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, err
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	return &pkceParams{
		Verifier:  verifier,
		Challenge: challenge,
		State:     state,
	}, nil
}
