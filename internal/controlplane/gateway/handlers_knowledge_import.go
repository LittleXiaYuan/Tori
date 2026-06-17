package gateway

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"yunque-agent/internal/agentcore/knowledge"
)

// fetchKnowledgeURLPage performs an SSRF-safe GET of rawURL and returns a
// cleaned ImportPage.
//
// The SSRF guard (validateSSRFTarget + newSSRFSafeClient) stays in the gateway
// because it is shared with other outbound-fetch features (Tori bind, wasm host
// funcs). The transport-free content extraction / crawl / tree logic now lives
// in the knowledge domain layer (knowledge.BuildPage / ExtractChildLinks /
// BuildImportTree). This wrapper is used by the NL-config importer and, via
// Gateway.FetchImportPage, by the knowledge pack's native import-url handler.
func fetchKnowledgeURLPage(rawURL, fallbackName string) (*knowledge.ImportPage, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported url scheme: %s", parsed.Scheme)
	}
	if err := validateSSRFTarget(parsed); err != nil {
		return nil, err
	}

	client := newSSRFSafeClient(20 * time.Second)
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Yunque-Agent/1.0 (+knowledge-import)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("fetch failed: %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return nil, err
	}
	return knowledge.BuildPage(rawURL, fallbackName, string(body), resp.Header.Get("Content-Type"))
}
