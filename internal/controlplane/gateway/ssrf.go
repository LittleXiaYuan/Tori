package gateway

// ssrf.go — Shared helpers for defending outbound HTTP calls against
// Server-Side Request Forgery. Used by knowledge-URL import, Tori OAuth
// bind, and any future feature that dials an operator-supplied URL.
//
// The key insight is that a single pre-flight DNS check is not sufficient:
//   1. An attacker can register a hostname whose A-record flips between a
//      public IP (first lookup) and a loopback IP (second lookup used by the
//      HTTP dialer). This is the classic DNS-rebinding SSRF.
//   2. A redirect from a public origin to `http://127.0.0.1/…` would sail
//      through the initial check.
//
// We defend against both by:
//   - re-validating every URL inside CheckRedirect, and
//   - re-validating the *resolved IP* inside Transport.DialContext, which is
//     what actually gets connected to.

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// validateSSRFTarget returns a non-nil error when the supplied URL points at
// an address the server must refuse to dial (loopback, private, link-local,
// cloud metadata, or unresolvable).
func validateSSRFTarget(u *url.URL) error {
	if u == nil {
		return fmt.Errorf("invalid url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported url scheme: %s", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("invalid url: missing host")
	}
	if isPrivateOrLoopback(host) {
		return fmt.Errorf("access to private/loopback addresses is not allowed")
	}
	ips, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("dns resolve failed: %w", err)
	}
	for _, ip := range ips {
		if isPrivateOrLoopback(ip) {
			return fmt.Errorf("access to private/loopback addresses is not allowed")
		}
	}
	return nil
}

// newSSRFSafeClient builds an http.Client that re-validates every redirect
// target and every TCP dial against the private/loopback deny-list. The
// caller's timeout budget is applied to both the request and the individual
// dial to guard against slow-read DoS.
func newSSRFSafeClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			// Resolve again at connect-time so a DNS rebinding between the
			// initial check and the connect is caught here.
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("dns resolve failed: %w", err)
			}
			for _, ip := range ips {
				if isPrivateIP(ip.IP) {
					return nil, fmt.Errorf("dial blocked: %s resolves to private address %s", host, ip.IP)
				}
			}
			return dialer.DialContext(ctx, network, addr)
		},
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		IdleConnTimeout:       30 * time.Second,
		MaxIdleConnsPerHost:   2,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return validateSSRFTarget(req.URL)
		},
	}
}

// isPrivateIP is the low-level variant used by DialContext, where we already
// have an IP in hand.
func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
