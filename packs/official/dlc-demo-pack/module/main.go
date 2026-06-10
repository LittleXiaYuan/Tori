//go:build wasip1

// Command module is the DLC Demo Pack's WASM backend, compiled to module.wasm:
//
//	GOOS=wasip1 GOARCH=wasm go build -o ../module.wasm ./module
//
// It implements the Pack WASM Route ABI (docs/spec/pack-wasm-abi.md): read a
// request envelope from stdin, write a response envelope to stdout.
package main

import (
	"encoding/json"
	"io"
	"os"
)

type requestEnvelope struct {
	Method string              `json:"method"`
	Path   string              `json:"path"`
	Query  map[string][]string `json:"query"`
	Header map[string][]string `json:"headers"`
	Body   string              `json:"body"`
}

type responseEnvelope struct {
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers,omitempty"`
	Body    string              `json:"body"`
}

func main() {
	in, _ := io.ReadAll(os.Stdin)
	var req requestEnvelope
	_ = json.Unmarshal(in, &req)

	// Echo the request body back inside a small JSON envelope.
	payload := map[string]any{
		"pong":   true,
		"method": req.Method,
		"echo":   json.RawMessage(orNull(req.Body)),
	}
	body, _ := json.Marshal(payload)

	resp := responseEnvelope{
		Status:  200,
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    string(body),
	}
	out, _ := json.Marshal(resp)
	_, _ = os.Stdout.Write(out)
}

func orNull(s string) string {
	if s == "" {
		return "null"
	}
	return s
}
