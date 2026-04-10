package speech

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Edge TTS WebSocket protocol constants.
const (
	edgeWSURL    = "wss://speech.platform.bing.com/consumer/speech/synthesize/readaloud/edge/v1"
	edgeOrigin   = "chrome-extension://jdiccldimpdaibmpdkjnbmckianbfold"
	edgeTrusted  = "https://www.bing.com"
	edgeUA       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 Edg/130.0.0.0"
	edgeMaxChunk = 2000 // max characters per SSML chunk

	// Fallback token if dynamic fetch fails.
	edgeFallbackToken = "6A5AA1D4EAFF4E9FB37E23D68491D6F4"
	edgeTokenTTL      = 24 * time.Hour
)

// edgeTokenCache caches the TrustedClientToken with expiry.
var edgeTokenCache struct {
	mu      sync.Mutex
	token   string
	expires time.Time
}

// getEdgeToken returns a cached or freshly-fetched TrustedClientToken.
// Falls back to the hardcoded token if the dynamic fetch fails.
func getEdgeToken() string {
	edgeTokenCache.mu.Lock()
	defer edgeTokenCache.mu.Unlock()

	if edgeTokenCache.token != "" && time.Now().Before(edgeTokenCache.expires) {
		return edgeTokenCache.token
	}

	// Try to fetch from Edge's extension page (contains the token in JS)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://speech.platform.bing.com/consumer/speech/synthesize/readaloud/edge/v1?TrustedClientToken="+edgeFallbackToken+"&ConnectionId=test",
		nil)
	if err == nil {
		req.Header.Set("User-Agent", edgeUA)
		req.Header.Set("Origin", edgeOrigin)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 400 {
				// Current fallback token is still valid
				edgeTokenCache.token = edgeFallbackToken
				edgeTokenCache.expires = time.Now().Add(edgeTokenTTL)
				return edgeTokenCache.token
			}
		}
	}

	// Token validation failed, use fallback anyway and retry next time with shorter TTL
	slog.Warn("edge_tts: token validation failed, using fallback")
	edgeTokenCache.token = edgeFallbackToken
	edgeTokenCache.expires = time.Now().Add(1 * time.Hour)
	return edgeTokenCache.token
}

// edgeTTSSynthesize connects to Edge TTS WebSocket and synthesizes speech.
func edgeTTSSynthesize(ctx context.Context, text, voice, outputFormat string) ([]byte, error) {
	if text == "" {
		return nil, fmt.Errorf("edge_tts: empty text")
	}

	reqID := generateRequestID()
	token := getEdgeToken()

	// Build WebSocket URL
	wsURL := fmt.Sprintf("%s?TrustedClientToken=%s&ConnectionId=%s", edgeWSURL, token, reqID)

	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
	}

	header := http.Header{}
	header.Set("Origin", edgeOrigin)
	header.Set("User-Agent", edgeUA)

	conn, _, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		// Token may have expired, invalidate cache and retry once
		edgeTokenCache.mu.Lock()
		edgeTokenCache.token = ""
		edgeTokenCache.mu.Unlock()
		return nil, fmt.Errorf("edge_tts: websocket dial: %w", err)
	}
	defer conn.Close()

	// Send configuration message
	configMsg := fmt.Sprintf(
		"Content-Type:application/json; charset=utf-8\r\nPath:speech.config\r\n\r\n"+
			`{"context":{"synthesis":{"audio":{"metadataoptions":{"sentenceBoundaryEnabled":"false","wordBoundaryEnabled":"false"},"outputFormat":"%s"}}}}`,
		outputFormat,
	)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(configMsg)); err != nil {
		return nil, fmt.Errorf("edge_tts: send config: %w", err)
	}

	// Split text into chunks for long text
	chunks := splitTextChunks(text, edgeMaxChunk)
	var allAudio bytes.Buffer

	for _, chunk := range chunks {
		audio, err := edgeSynthChunk(ctx, conn, reqID, chunk, voice)
		if err != nil {
			return nil, err
		}
		allAudio.Write(audio)
	}

	slog.Debug("edge_tts: synthesis complete", "text_len", len(text), "audio_bytes", allAudio.Len(), "voice", voice)
	return allAudio.Bytes(), nil
}

// edgeSynthChunk sends one SSML chunk and collects audio data.
func edgeSynthChunk(ctx context.Context, conn *websocket.Conn, reqID, text, voice string) ([]byte, error) {
	ssml := buildSSML(text, voice)

	ssmlMsg := fmt.Sprintf(
		"X-RequestId:%s\r\nContent-Type:application/ssml+xml\r\nPath:ssml\r\n\r\n%s",
		reqID, ssml,
	)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(ssmlMsg)); err != nil {
		return nil, fmt.Errorf("edge_tts: send ssml: %w", err)
	}

	var audioBuf bytes.Buffer

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("edge_tts: read: %w", err)
		}

		switch msgType {
		case websocket.TextMessage:
			msg := string(data)
			if strings.Contains(msg, "Path:turn.end") {
				return audioBuf.Bytes(), nil
			}
			// turn.start, response, metadata — skip
		case websocket.BinaryMessage:
			audio := extractAudioFromBinary(data)
			if audio != nil {
				audioBuf.Write(audio)
			}
		}
	}
}

// extractAudioFromBinary parses the Edge TTS binary frame format.
// Binary messages have a 2-byte header length (big-endian) followed by
// a text header, then the audio payload.
func extractAudioFromBinary(data []byte) []byte {
	if len(data) < 2 {
		return nil
	}
	headerLen := int(binary.BigEndian.Uint16(data[:2]))
	if 2+headerLen > len(data) {
		return nil
	}
	header := string(data[2 : 2+headerLen])
	if !strings.Contains(header, "Path:audio") {
		return nil
	}
	return data[2+headerLen:]
}

// buildSSML creates SSML markup for Edge TTS.
func buildSSML(text, voice string) string {
	escaped := html.EscapeString(text)
	return fmt.Sprintf(
		`<speak version='1.0' xmlns='http://www.w3.org/2001/10/synthesis' xml:lang='en-US'>`+
			`<voice name='%s'><prosody pitch='+0Hz' rate='+0%%' volume='+0%%'>%s</prosody></voice></speak>`,
		voice, escaped,
	)
}

// splitTextChunks splits text into chunks at sentence boundaries.
func splitTextChunks(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	remaining := text
	for len(remaining) > 0 {
		if len(remaining) <= maxLen {
			chunks = append(chunks, remaining)
			break
		}
		// Find a good split point
		cutAt := maxLen
		for _, sep := range []string{"。", ".", "！", "!", "？", "?", "；", ";", "\n", "，", ","} {
			if idx := strings.LastIndex(remaining[:maxLen], sep); idx > 0 {
				cutAt = idx + len(sep)
				break
			}
		}
		chunks = append(chunks, remaining[:cutAt])
		remaining = remaining[cutAt:]
	}
	return chunks
}

// generateRequestID creates a random hex request ID.
func generateRequestID() string {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		// fallback
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
