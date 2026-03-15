package channel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// FeishuAPI provides methods to send messages back via Feishu Open API.
type FeishuAPI struct {
	appID     string
	appSecret string

	mu          sync.Mutex
	token       string
	tokenExpiry time.Time
}

// NewFeishuAPI creates a Feishu API client.
func NewFeishuAPI(appID, appSecret string) *FeishuAPI {
	return &FeishuAPI{appID: appID, appSecret: appSecret}
}

// getToken fetches or reuses a tenant access token.
func (f *FeishuAPI) getToken() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.token != "" && time.Now().Before(f.tokenExpiry) {
		return f.token, nil
	}

	body, _ := json.Marshal(map[string]string{
		"app_id":     f.appID,
		"app_secret": f.appSecret,
	})
	resp, err := http.Post(
		"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
		"application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("feishu token: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Code != 0 {
		return "", fmt.Errorf("feishu token error: %s", result.Msg)
	}

	f.token = result.TenantAccessToken
	f.tokenExpiry = time.Now().Add(time.Duration(result.Expire-60) * time.Second)
	return f.token, nil
}

// SendMessage sends a text message to a chat.
func (f *FeishuAPI) SendMessage(chatID, text string) error {
	token, err := f.getToken()
	if err != nil {
		return err
	}

	content, _ := json.Marshal(map[string]string{"text": text})
	body, _ := json.Marshal(map[string]any{
		"receive_id": chatID,
		"msg_type":   "text",
		"content":    string(content),
	})

	req, err := http.NewRequest("POST",
		"https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id",
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("feishu send: %w", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return fmt.Errorf("feishu send status %d", resp.StatusCode)
	}
	return nil
}

// ReplyMessage replies to a specific message.
func (f *FeishuAPI) ReplyMessage(messageID, text string) error {
	token, err := f.getToken()
	if err != nil {
		return err
	}

	content, _ := json.Marshal(map[string]string{"text": text})
	body, _ := json.Marshal(map[string]any{
		"msg_type": "text",
		"content":  string(content),
	})

	url := fmt.Sprintf("https://open.feishu.cn/open-apis/im/v1/messages/%s/reply", messageID)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("feishu reply: %w", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)
	return nil
}
