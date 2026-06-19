// Package ssrf provides shared guards for outbound HTTP calls that accept
// operator-supplied URLs.
package ssrf

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ValidateTarget returns a non-nil error when the supplied URL points at an
// address the server must refuse to dial (loopback, private, link-local, cloud
// metadata, or unresolvable).
func ValidateTarget(u *url.URL) error {
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
	if IsPrivateOrLoopback(host) {
		return fmt.Errorf("access to private/loopback addresses is not allowed")
	}
	ips, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("dns resolve failed: %w", err)
	}
	for _, ip := range ips {
		if IsPrivateOrLoopback(ip) {
			return fmt.Errorf("access to private/loopback addresses is not allowed")
		}
	}
	return nil
}

// NewSafeClient builds an http.Client that re-validates every redirect target
// and every TCP dial against the private/loopback deny-list.
func NewSafeClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("dns resolve failed: %w", err)
			}
			for _, ip := range ips {
				if IsPrivateIP(ip.IP) {
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
			return ValidateTarget(req.URL)
		},
	}
}

// IsPrivateIP is the low-level variant used by DialContext, where an IP has
// already been resolved.
func IsPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

// IsPrivateOrLoopback checks if an IP or hostname belongs to private, loopback,
// link-local, or other non-routable address ranges.
func IsPrivateOrLoopback(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		lower := strings.ToLower(host)
		return lower == "localhost" || strings.HasSuffix(lower, ".local") ||
			lower == "metadata.google.internal" || lower == "169.254.169.254"
	}
	return IsPrivateIP(ip)
}
