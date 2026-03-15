package gateway

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/planner"
)

// wsConn is a minimal WebSocket connection using raw TCP upgrade.
// For production, use gorilla/websocket. This implementation covers the core protocol.
type wsConn struct {
	conn   net.Conn
	mu     sync.Mutex
	closed bool
}

// wsMessage represents a WebSocket text frame payload.
type wsMessage struct {
	Type    string          `json:"type"` // "chat", "ping", "subscribe"
	Content string          `json:"content,omitempty"`
	Session string          `json:"session_id,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type wsReply struct {
	Type    string `json:"type"` // "reply", "chunk", "error", "pong"
	Content string `json:"content,omitempty"`
	Done    bool   `json:"done,omitempty"`
}

func (g *Gateway) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check for WebSocket upgrade
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		http.Error(w, "websocket upgrade required", http.StatusBadRequest)
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "websocket not supported", http.StatusInternalServerError)
		return
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Send WebSocket handshake response
	acceptKey := computeAcceptKey(r.Header.Get("Sec-WebSocket-Key"))
	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey + "\r\n\r\n"
	bufrw.WriteString(resp)
	bufrw.Flush()

	ws := &wsConn{conn: conn}
	defer ws.close()

	tid := tenantFromCtx(r.Context())
	slog.Info("websocket connected", "tenant", tid)

	// Read loop
	reader := bufio.NewReader(conn)
	for {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		msg, err := readWSFrame(reader)
		if err != nil {
			slog.Debug("websocket read error", "err", err)
			break
		}

		var wsMsg wsMessage
		if err := json.Unmarshal(msg, &wsMsg); err != nil {
			ws.sendJSON(wsReply{Type: "error", Content: "invalid json"})
			continue
		}

		switch wsMsg.Type {
		case "ping":
			ws.sendJSON(wsReply{Type: "pong"})

		case "chat":
			go g.handleWSChat(r.Context(), ws, tid, wsMsg)

		default:
			ws.sendJSON(wsReply{Type: "error", Content: "unknown type: " + wsMsg.Type})
		}
	}
}

func (g *Gateway) handleWSChat(ctx context.Context, ws *wsConn, tenantID string, msg wsMessage) {
	msgs := []llm.Message{{Role: "user", Content: msg.Content}}

	// Load session history
	if msg.Session != "" {
		history := g.convStore.Get(msg.Session)
		if len(history) > 0 {
			msgs = append(history, msgs...)
		}
		g.convStore.Append(msg.Session, llm.Message{Role: "user", Content: msg.Content})
	}

	result, err := g.planner.Run(ctx, planner.PlanRequest{
		Messages: msgs,
		TenantID: tenantID,
	})
	if err != nil {
		ws.sendJSON(wsReply{Type: "error", Content: err.Error()})
		return
	}

	if msg.Session != "" {
		g.convStore.Append(msg.Session, llm.Message{Role: "assistant", Content: result.Reply})
	}

	// Send chunked reply for streaming feel
	chunks := splitIntoChunks(result.Reply, 50)
	for i, chunk := range chunks {
		ws.sendJSON(wsReply{
			Type:    "chunk",
			Content: chunk,
			Done:    i == len(chunks)-1,
		})
	}
}

func (ws *wsConn) sendJSON(v any) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if ws.closed {
		return nil
	}
	data, _ := json.Marshal(v)
	return writeWSFrame(ws.conn, data)
}

func (ws *wsConn) close() {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if !ws.closed {
		ws.closed = true
		ws.conn.Close()
	}
}

func splitIntoChunks(s string, chunkSize int) []string {
	runes := []rune(s)
	if len(runes) == 0 {
		return []string{""}
	}
	var chunks []string
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}

// --- WebSocket frame helpers ---

func computeAcceptKey(key string) string {
	const magic = "258EAFA5-E914-47DA-95CA-5AB5DC85B11B"
	h := sha1.New()
	h.Write([]byte(key + magic))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func readWSFrame(r *bufio.Reader) ([]byte, error) {
	// Read first 2 bytes
	b1, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	b2, err := r.ReadByte()
	if err != nil {
		return nil, err
	}

	masked := (b2 & 0x80) != 0
	payloadLen := int(b2 & 0x7F)

	if payloadLen == 126 {
		buf := make([]byte, 2)
		if _, err := r.Read(buf); err != nil {
			return nil, err
		}
		payloadLen = int(binary.BigEndian.Uint16(buf))
	} else if payloadLen == 127 {
		buf := make([]byte, 8)
		if _, err := r.Read(buf); err != nil {
			return nil, err
		}
		payloadLen = int(binary.BigEndian.Uint64(buf))
	}

	var mask [4]byte
	if masked {
		if _, err := r.Read(mask[:]); err != nil {
			return nil, err
		}
	}

	payload := make([]byte, payloadLen)
	if _, err := r.Read(payload); err != nil {
		return nil, err
	}

	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}

	_ = b1 // opcode in b1, we handle text frames
	return payload, nil
}

func writeWSFrame(conn net.Conn, data []byte) error {
	// Text frame, no mask
	frame := []byte{0x81} // FIN + text opcode
	l := len(data)
	if l < 126 {
		frame = append(frame, byte(l))
	} else if l < 65536 {
		frame = append(frame, 126)
		buf := make([]byte, 2)
		binary.BigEndian.PutUint16(buf, uint16(l))
		frame = append(frame, buf...)
	} else {
		frame = append(frame, 127)
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(l))
		frame = append(frame, buf...)
	}
	frame = append(frame, data...)
	_, err := conn.Write(frame)
	return err
}
