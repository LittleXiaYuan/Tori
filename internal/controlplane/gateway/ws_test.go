package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWSChatDocumentIntentAddsRoutingHint(t *testing.T) {
	var mu sync.Mutex
	var capturedBodies []string
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []map[string]string `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		raw, _ := json.Marshal(payload.Messages)
		mu.Lock()
		capturedBodies = append(capturedBodies, string(raw))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": "ok"}}},
		})
	}))
	defer mock.Close()

	gw, tm := newE2EGateway(mock.URL)
	tenant := tm.Register("ws-document-routing")
	conn := &bufferedWSConn{}

	gw.handleWSChat(context.Background(), &wsConn{conn: conn}, tenant.ID, wsMessage{
		Type:    "chat",
		Content: "请读取这个 xls 文件，并提取表格字段",
	})

	mu.Lock()
	defer mu.Unlock()
	if len(capturedBodies) == 0 {
		t.Fatalf("expected websocket chat to reach mock LLM")
	}
	firstPrompt := capturedBodies[0]
	if !strings.Contains(firstPrompt, "[Document routing]") || !strings.Contains(firstPrompt, "document_parse") {
		t.Fatalf("expected document routing hint in websocket planner prompt, got %s", firstPrompt)
	}
	if conn.Len() == 0 {
		t.Fatalf("expected websocket reply frame to be written")
	}
}

func TestWSChatPlannerErrorIsFriendly(t *testing.T) {
	gw, _ := newTestGateway()
	conn := &bufferedWSConn{}

	gw.handleWSChat(context.Background(), &wsConn{conn: conn}, "t1", wsMessage{
		Type:    "chat",
		Content: "你好",
	})

	reply := conn.LastReply(t)
	if reply.Type != "error" {
		t.Fatalf("expected error reply, got %+v", reply)
	}
	for _, raw := range []string{"all fallback LLM clients failed", "context deadline exceeded", "execution failed", "handoff agent", "EOF"} {
		if strings.Contains(reply.Content, raw) {
			t.Fatalf("websocket error response should be friendly, found raw %q in %+v", raw, reply)
		}
	}
	if !strings.Contains(reply.Content, "现场") {
		t.Fatalf("expected friendly recovery wording, got %+v", reply)
	}
}

type bufferedWSConn struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (c *bufferedWSConn) Read(_ []byte) (int, error) {
	return 0, net.ErrClosed
}

func (c *bufferedWSConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Write(p)
}

func (c *bufferedWSConn) Close() error {
	return nil
}

func (c *bufferedWSConn) LocalAddr() net.Addr {
	return dummyWSAddr("local")
}

func (c *bufferedWSConn) RemoteAddr() net.Addr {
	return dummyWSAddr("remote")
}

func (c *bufferedWSConn) SetDeadline(time.Time) error {
	return nil
}

func (c *bufferedWSConn) SetReadDeadline(time.Time) error {
	return nil
}

func (c *bufferedWSConn) SetWriteDeadline(time.Time) error {
	return nil
}

func (c *bufferedWSConn) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Len()
}

func (c *bufferedWSConn) LastReply(t *testing.T) wsReply {
	t.Helper()
	c.mu.Lock()
	data := append([]byte(nil), c.buf.Bytes()...)
	c.mu.Unlock()
	if len(data) < 2 {
		t.Fatalf("no websocket frame captured")
	}
	offset := 2
	payloadLen := int(data[1] & 0x7F)
	switch payloadLen {
	case 126:
		if len(data) < 4 {
			t.Fatalf("short websocket frame")
		}
		payloadLen = int(data[2])<<8 | int(data[3])
		offset = 4
	case 127:
		t.Fatalf("test helper does not support 64-bit websocket frame length")
	}
	if len(data) < offset+payloadLen {
		t.Fatalf("short websocket payload: have=%d want=%d", len(data)-offset, payloadLen)
	}
	var reply wsReply
	if err := json.Unmarshal(data[offset:offset+payloadLen], &reply); err != nil {
		t.Fatalf("decode websocket reply: %v payload=%q", err, string(data[offset:offset+payloadLen]))
	}
	return reply
}

type dummyWSAddr string

func (a dummyWSAddr) Network() string {
	return "test"
}

func (a dummyWSAddr) String() string {
	return string(a)
}
