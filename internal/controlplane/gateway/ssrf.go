package gateway

// ssrf.go — Gateway compatibility wrappers for the shared SSRF guard.

import (
	"net/http"
	"net/url"
	"time"

	"yunque-agent/internal/security/ssrf"
)

func validateSSRFTarget(u *url.URL) error {
	return ssrf.ValidateTarget(u)
}

func newSSRFSafeClient(timeout time.Duration) *http.Client {
	return ssrf.NewSafeClient(timeout)
}
