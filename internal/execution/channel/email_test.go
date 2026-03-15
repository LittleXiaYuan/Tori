package channel

import (
	"testing"
)

func TestEmailType(t *testing.T) {
	e := NewEmail(EmailConfig{Host: "smtp.example.com", Username: "test@example.com", Password: "pass"})
	if e.Type() != "email" {
		t.Fatalf("expected 'email', got %s", e.Type())
	}
}

func TestEmailDefaultPort(t *testing.T) {
	e := NewEmail(EmailConfig{Host: "smtp.example.com"})
	if e.cfg.Port != 587 {
		t.Fatalf("expected default port 587, got %d", e.cfg.Port)
	}
}

func TestEmailDefaultFrom(t *testing.T) {
	e := NewEmail(EmailConfig{Host: "smtp.example.com", Username: "user@test.com"})
	if e.cfg.From != "user@test.com" {
		t.Fatalf("expected from=username, got %s", e.cfg.From)
	}
}

func TestBuildEmailBody(t *testing.T) {
	body := buildEmailBody("from@test.com", "to@test.com", "Test Subject", "Hello world")
	if !contains(body, "From: from@test.com") {
		t.Fatal("missing From header")
	}
	if !contains(body, "To: to@test.com") {
		t.Fatal("missing To header")
	}
	if !contains(body, "Subject: Test Subject") {
		t.Fatal("missing Subject header")
	}
	if !contains(body, "Hello world") {
		t.Fatal("missing body content")
	}
}

func TestExtractSubject(t *testing.T) {
	s, ok := extractSubject("This is the subject\nAnd the body follows")
	if !ok || s != "This is the subject" {
		t.Fatalf("expected subject extraction, got %q ok=%v", s, ok)
	}

	_, ok = extractSubject("ab")
	if ok {
		t.Fatal("too short should not extract")
	}

	_, ok = extractSubject("single line without newline")
	if ok {
		t.Fatal("no newline should not extract")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
