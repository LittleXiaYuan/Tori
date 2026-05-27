package tori

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const defaultHTTPTimeout = 10 * time.Second

var blockedOutboundPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
}

// validateToriTarget rejects operator-controlled Tori endpoints that point at
// local, private, link-local, or cloud-metadata style addresses. The Gateway
// validates user-supplied bind URLs before starting OAuth, but this package is
// also used by refresh, discovery, and sync code paths. Keeping the guard here
// prevents future internal callers from accidentally using http.DefaultClient
// against a persisted or config-sourced unsafe Tori URL.
func validateToriTarget(u *url.URL) error {
	if u == nil {
		return fmt.Errorf("invalid tori url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported tori url scheme: %s", u.Scheme)
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return fmt.Errorf("invalid tori url: missing host")
	}
	ips, err := resolveToriHost(context.Background(), host)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		if isBlockedOutboundIP(ip) {
			return fmt.Errorf("tori url host %q resolves to blocked address %s", host, ip)
		}
	}
	return nil
}

func validateToriURLString(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid tori url: %w", err)
	}
	if err := validateToriTarget(u); err != nil {
		return nil, err
	}
	return u, nil
}

func newSafeHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		// Do not honor proxy env vars for operator-supplied Tori URLs. A proxy
		// could otherwise become the real dial target and bypass host/IP checks
		// in this client.
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := resolveToriHost(ctx, host)
			if err != nil {
				return nil, err
			}
			var lastErr error
			for _, ip := range ips {
				if isBlockedOutboundIP(ip) {
					return nil, fmt.Errorf("dial blocked: %s resolves to blocked address %s", host, ip)
				}
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, fmt.Errorf("no resolved addresses for %s", host)
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
			return validateToriTarget(req.URL)
		},
	}
}

func doSafeRequest(req *http.Request, timeout time.Duration) (*http.Response, error) {
	if req == nil || req.URL == nil {
		return nil, fmt.Errorf("invalid request")
	}
	if err := validateToriTarget(req.URL); err != nil {
		return nil, err
	}
	return newSafeHTTPClient(timeout).Do(req)
}

func postSafeForm(ctx context.Context, rawURL string, values url.Values, timeout time.Duration) (*http.Response, error) {
	u, err := validateToriURLString(rawURL)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return doSafeRequest(req, timeout)
}

func jsonSafeRequest(ctx context.Context, method, rawURL string, body []byte, timeout time.Duration) (*http.Request, error) {
	u, err := validateToriURLString(rawURL)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	return http.NewRequestWithContext(ctx, method, u.String(), reader)
}

func resolveToriHost(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("dns resolve failed for %q: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("dns resolve failed for %q: no addresses", host)
	}
	result := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		result = append(result, ip.IP)
	}
	return result, nil
}

func isBlockedOutboundIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	addr = addr.Unmap()
	for _, prefix := range blockedOutboundPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
