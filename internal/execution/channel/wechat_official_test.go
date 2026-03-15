package channel

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestWeChatOfficialType(t *testing.T) {
	ch := NewWeChatOfficial(WeChatOfficialConfig{AppID: "test"})
	if ch.Type() != "wechat_official" {
		t.Fatalf("expected wechat_official, got %s", ch.Type())
	}
}

func TestWeChatOfficialDefaults(t *testing.T) {
	ch := NewWeChatOfficial(WeChatOfficialConfig{AppID: "test"})
	if ch.port != "9882" {
		t.Fatalf("expected default port 9882, got %s", ch.port)
	}
	if ch.bindAddr != "0.0.0.0" {
		t.Fatalf("expected default bind 0.0.0.0, got %s", ch.bindAddr)
	}
	if ch.apiBaseURL != "https://api.weixin.qq.com/cgi-bin" {
		t.Fatalf("unexpected api base: %s", ch.apiBaseURL)
	}
}

func TestWeChatOfficialCustomConfig(t *testing.T) {
	ch := NewWeChatOfficial(WeChatOfficialConfig{
		AppID:      "myapp",
		AppSecret:  "mysecret",
		Token:      "mytoken",
		Port:       "8080",
		BindAddr:   "127.0.0.1",
		APIBaseURL: "https://custom.api.com",
	})
	if ch.appID != "myapp" {
		t.Fatalf("appID mismatch")
	}
	if ch.port != "8080" {
		t.Fatalf("port mismatch")
	}
	if ch.bindAddr != "127.0.0.1" {
		t.Fatalf("bindAddr mismatch")
	}
	if ch.apiBaseURL != "https://custom.api.com" {
		t.Fatalf("apiBaseURL mismatch")
	}
}

func computeWxSignature(token, timestamp, nonce string) string {
	strs := []string{token, timestamp, nonce}
	sort.Strings(strs)
	h := sha1.New()
	h.Write([]byte(strings.Join(strs, "")))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func TestWeChatOfficialVerifySignature(t *testing.T) {
	ch := NewWeChatOfficial(WeChatOfficialConfig{
		AppID: "test",
		Token: "mytoken",
	})

	// Correct signature
	sig := computeWxSignature("mytoken", "1234567890", "nonce123")
	if !ch.verifySignature(sig, "1234567890", "nonce123") {
		t.Fatal("expected valid signature to pass")
	}

	// Wrong signature
	if ch.verifySignature("invalid_sig", "1234567890", "nonce123") {
		t.Fatal("expected invalid signature to fail")
	}

	// Wrong timestamp
	if ch.verifySignature(sig, "9999999999", "nonce123") {
		t.Fatal("expected wrong timestamp to fail")
	}
}

func TestWeChatOfficialCallbackGETVerification(t *testing.T) {
	ch := NewWeChatOfficial(WeChatOfficialConfig{
		AppID: "test",
		Token: "mytoken",
	})

	sig := computeWxSignature("mytoken", "1234567890", "nonce123")
	url := fmt.Sprintf("/wechat/callback?signature=%s&timestamp=1234567890&nonce=nonce123&echostr=echo_test", sig)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()

	ch.handleCallback(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "echo_test" {
		t.Fatalf("expected echo_test, got %q", rec.Body.String())
	}
}

func TestWeChatOfficialCallbackGETBadSignature(t *testing.T) {
	ch := NewWeChatOfficial(WeChatOfficialConfig{
		AppID: "test",
		Token: "mytoken",
	})

	req := httptest.NewRequest(http.MethodGet, "/wechat/callback?signature=bad&timestamp=123&nonce=n&echostr=e", nil)
	rec := httptest.NewRecorder()

	ch.handleCallback(rec, req)

	if rec.Code != 403 {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestWeChatOfficialCallbackPOSTTextMessage(t *testing.T) {
	ch := NewWeChatOfficial(WeChatOfficialConfig{
		AppID: "test",
		Token: "mytoken",
	})

	xmlBody := `<xml>
		<ToUserName>gh_test</ToUserName>
		<FromUserName>user_openid</FromUserName>
		<CreateTime>1348831860</CreateTime>
		<MsgType>text</MsgType>
		<Content>你好世界</Content>
		<MsgId>1234567890</MsgId>
	</xml>`

	sig := computeWxSignature("mytoken", "1348831860", "nonce1")
	url := fmt.Sprintf("/wechat/callback?signature=%s&timestamp=1348831860&nonce=nonce1", sig)
	req := httptest.NewRequest(http.MethodPost, url, strings.NewReader(xmlBody))
	rec := httptest.NewRecorder()

	ch.handleCallback(rec, req)

	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "success" {
		t.Fatalf("expected success, got %q", rec.Body.String())
	}

	// Check message was queued
	select {
	case msg := <-ch.msgCh:
		if msg.Content != "你好世界" {
			t.Fatalf("expected 你好世界, got %q", msg.Content)
		}
		if msg.UserID != "user_openid" {
			t.Fatalf("expected user_openid, got %q", msg.UserID)
		}
		if msg.ChannelType != "wechat_official" {
			t.Fatalf("expected wechat_official, got %q", msg.ChannelType)
		}
	case <-time.After(time.Second):
		t.Fatal("no message received")
	}
}

func TestWeChatOfficialCallbackPOSTBadSignature(t *testing.T) {
	ch := NewWeChatOfficial(WeChatOfficialConfig{
		AppID: "test",
		Token: "mytoken",
	})

	req := httptest.NewRequest(http.MethodPost, "/wechat/callback?signature=bad&timestamp=123&nonce=n", strings.NewReader("<xml></xml>"))
	rec := httptest.NewRecorder()

	ch.handleCallback(rec, req)

	if rec.Code != 403 {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestWeChatOfficialExtractContent(t *testing.T) {
	ch := NewWeChatOfficial(WeChatOfficialConfig{AppID: "test"})

	tests := []struct {
		name    string
		msg     wxOfficialXMLMessage
		want    string
		wantPfx string
	}{
		{
			name: "text",
			msg:  wxOfficialXMLMessage{MsgType: "text", Content: "hello"},
			want: "hello",
		},
		{
			name:    "image",
			msg:     wxOfficialXMLMessage{MsgType: "image", PicURL: "https://img.com/1.jpg"},
			wantPfx: "[图片]",
		},
		{
			name: "voice_with_recognition",
			msg:  wxOfficialXMLMessage{MsgType: "voice", Recognition: "语音识别结果"},
			want: "语音识别结果",
		},
		{
			name: "voice_without_recognition",
			msg:  wxOfficialXMLMessage{MsgType: "voice"},
			want: "[语音消息]",
		},
		{
			name:    "location",
			msg:     wxOfficialXMLMessage{MsgType: "location", Label: "北京", LocationX: 39.9, LocationY: 116.4},
			wantPfx: "[位置]",
		},
		{
			name:    "link",
			msg:     wxOfficialXMLMessage{MsgType: "link", Title: "标题", URL: "https://example.com"},
			wantPfx: "[链接]",
		},
		{
			name: "subscribe_event",
			msg:  wxOfficialXMLMessage{MsgType: "event", Event: "subscribe"},
			want: "用户关注了公众号",
		},
		{
			name: "unknown",
			msg:  wxOfficialXMLMessage{MsgType: "unknown_type"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ch.extractContent(&tt.msg)
			if tt.want != "" && got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
			if tt.wantPfx != "" && !strings.HasPrefix(got, tt.wantPfx) {
				t.Fatalf("expected prefix %q, got %q", tt.wantPfx, got)
			}
		})
	}
}

func TestWeChatOfficialSendWithMockAPI(t *testing.T) {
	// Create a mock WeChat API server
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/token"):
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "test_token_123",
				"expires_in":   7200,
			})
		case strings.Contains(r.URL.Path, "/message/custom/send"):
			// Verify access_token
			if r.URL.Query().Get("access_token") != "test_token_123" {
				json.NewEncoder(w).Encode(map[string]any{"errcode": 40001, "errmsg": "invalid token"})
				return
			}
			var payload map[string]any
			json.NewDecoder(r.Body).Decode(&payload)
			if payload["touser"] != "user123" || payload["msgtype"] != "text" {
				t.Errorf("unexpected payload: %v", payload)
			}
			json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "errmsg": "ok"})
		}
	}))
	defer apiServer.Close()

	ch := NewWeChatOfficial(WeChatOfficialConfig{
		AppID:      "testapp",
		AppSecret:  "testsecret",
		Token:      "testtoken",
		APIBaseURL: apiServer.URL,
	})

	// Refresh token first
	ch.refreshAccessToken()
	if ch.accessToken != "test_token_123" {
		t.Fatalf("expected test_token_123, got %q", ch.accessToken)
	}

	// Send message
	err := ch.Send(context.Background(), "user123", Reply{Content: "你好"})
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
}

func TestWeChatOfficialSendLongMessage(t *testing.T) {
	sendCount := 0
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/token"):
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "tk",
				"expires_in":   7200,
			})
		case strings.Contains(r.URL.Path, "/message/custom/send"):
			sendCount++
			json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "errmsg": "ok"})
		}
	}))
	defer apiServer.Close()

	ch := NewWeChatOfficial(WeChatOfficialConfig{
		AppID:      "app",
		AppSecret:  "secret",
		APIBaseURL: apiServer.URL,
	})
	ch.refreshAccessToken()

	// Create a message longer than 600 chars
	longMsg := strings.Repeat("测试消息。", 200) // ~200*15 bytes, ~1000 chars
	err := ch.Send(context.Background(), "user1", Reply{Content: longMsg})
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if sendCount < 2 {
		t.Fatalf("expected multiple sends for long message, got %d", sendCount)
	}
}

func TestSplitWxOfficialMessage(t *testing.T) {
	// Short message — no split
	parts := splitWxOfficialMessage("hello", 600)
	if len(parts) != 1 || parts[0] != "hello" {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}

	// Long message — should split
	longMsg := strings.Repeat("a", 1200)
	parts = splitWxOfficialMessage(longMsg, 600)
	if len(parts) < 2 {
		t.Fatalf("expected at least 2 parts, got %d", len(parts))
	}

	// Message with sentence boundaries
	textWithBreaks := strings.Repeat("这是一个测试句子。", 100)
	parts = splitWxOfficialMessage(textWithBreaks, 600)
	for _, p := range parts {
		if len(p) > 600 {
			t.Fatalf("part exceeds max length: %d", len(p))
		}
	}
}

func TestWeChatOfficialXMLParsing(t *testing.T) {
	xmlData := `<xml>
		<ToUserName><![CDATA[gh_test]]></ToUserName>
		<FromUserName><![CDATA[oUser123]]></FromUserName>
		<CreateTime>1348831860</CreateTime>
		<MsgType><![CDATA[text]]></MsgType>
		<Content><![CDATA[Hello World]]></Content>
		<MsgId>1234567890123456</MsgId>
	</xml>`

	var msg wxOfficialXMLMessage
	err := xml.Unmarshal([]byte(xmlData), &msg)
	if err != nil {
		t.Fatalf("xml unmarshal failed: %v", err)
	}
	if msg.ToUserName != "gh_test" {
		t.Fatalf("expected gh_test, got %q", msg.ToUserName)
	}
	if msg.FromUserName != "oUser123" {
		t.Fatalf("expected oUser123, got %q", msg.FromUserName)
	}
	if msg.MsgType != "text" {
		t.Fatalf("expected text, got %q", msg.MsgType)
	}
	if msg.Content != "Hello World" {
		t.Fatalf("expected 'Hello World', got %q", msg.Content)
	}
	if msg.MsgID != 1234567890123456 {
		t.Fatalf("expected 1234567890123456, got %d", msg.MsgID)
	}
}
