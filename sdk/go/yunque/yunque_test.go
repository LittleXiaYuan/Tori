package yunque

import "testing"

func TestAPIErrorMessageParsesNestedGatewayError(t *testing.T) {
	body := []byte(`{"error":{"code":"BAD_REQUEST","message":"unsupported recovery action"}}`)
	got := apiErrorMessage(body)
	want := "BAD_REQUEST: unsupported recovery action"
	if got != want {
		t.Fatalf("apiErrorMessage() = %q, want %q", got, want)
	}
}

func TestAPIErrorMessageFallsBackToText(t *testing.T) {
	got := apiErrorMessage([]byte("plain failure"))
	if got != "plain failure" {
		t.Fatalf("apiErrorMessage() = %q, want plain failure", got)
	}
}
