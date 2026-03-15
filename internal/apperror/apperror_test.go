package apperror

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNew(t *testing.T) {
	e := New(CodeBadRequest, "bad input")
	if e.Code != CodeBadRequest {
		t.Fatalf("expected BAD_REQUEST, got %s", e.Code)
	}
	if e.Message != "bad input" {
		t.Fatalf("expected 'bad input', got %s", e.Message)
	}
	if e.Detail != "" {
		t.Fatal("expected empty detail")
	}
}

func TestWrap(t *testing.T) {
	inner := errors.New("connection refused")
	e := Wrap(CodeStorageError, "db failed", inner)
	if e.Detail != "connection refused" {
		t.Fatalf("expected detail from inner error, got %s", e.Detail)
	}
	if e.Error() != "STORAGE_ERROR: db failed (connection refused)" {
		t.Fatalf("unexpected Error(): %s", e.Error())
	}
}

func TestHTTPStatus_KnownCodes(t *testing.T) {
	cases := []struct {
		code   Code
		status int
	}{
		{CodeBadRequest, 400},
		{CodeUnauthorized, 401},
		{CodeForbidden, 403},
		{CodeNotFound, 404},
		{CodeMethodNotAllow, 405},
		{CodeTooManyReqs, 429},
		{CodeQuotaExceeded, 429},
		{CodePayloadTooLrg, 413},
		{CodeMissingField, 400},
		{CodeInternal, 500},
		{CodeLLMError, 502},
		{CodeLLMUnavailable, 503},
		{CodeSandboxError, 500},
		{CodeStorageError, 500},
	}
	for _, tc := range cases {
		e := New(tc.code, "test")
		if e.HTTPStatus() != tc.status {
			t.Errorf("code %s: expected %d, got %d", tc.code, tc.status, e.HTTPStatus())
		}
	}
}

func TestHTTPStatus_UnknownCode(t *testing.T) {
	e := &Error{Code: "UNKNOWN_CODE", Message: "test"}
	if e.HTTPStatus() != 500 {
		t.Fatalf("unknown code should default to 500, got %d", e.HTTPStatus())
	}
}

func TestWrite(t *testing.T) {
	w := httptest.NewRecorder()
	Write(w, Wrap(CodeLLMError, "llm down", errors.New("timeout")))

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}

	var body map[string]map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body["error"]["code"] != "LLM_ERROR" {
		t.Fatalf("expected LLM_ERROR, got %s", body["error"]["code"])
	}
	if body["error"]["detail"] != "timeout" {
		t.Fatalf("expected detail 'timeout', got %s", body["error"]["detail"])
	}
}

func TestWriteCode(t *testing.T) {
	w := httptest.NewRecorder()
	WriteCode(w, CodeUnauthorized, "no token")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	var body map[string]map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["error"]["code"] != "UNAUTHORIZED" {
		t.Fatalf("expected UNAUTHORIZED, got %s", body["error"]["code"])
	}
}

func TestErrorString(t *testing.T) {
	e1 := New(CodeBadRequest, "bad")
	if e1.Error() != "BAD_REQUEST: bad" {
		t.Fatalf("unexpected: %s", e1.Error())
	}
	e2 := Wrap(CodeInternal, "fail", errors.New("oom"))
	if e2.Error() != "INTERNAL_ERROR: fail (oom)" {
		t.Fatalf("unexpected: %s", e2.Error())
	}
}
