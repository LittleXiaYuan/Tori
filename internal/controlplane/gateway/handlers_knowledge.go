package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/knowledge"
	"yunque-agent/pkg/safego"
)

var (
	kbStripScriptRE   = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	kbStripStyleRE    = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	kbStripSVGRE      = regexp.MustCompile(`(?is)<svg[^>]*>.*?</svg>`)
	kbStripNoscriptRE = regexp.MustCompile(`(?is)<noscript[^>]*>.*?</noscript>`)
	kbStripHeaderRE   = regexp.MustCompile(`(?is)<header[^>]*>.*?</header>`)
	kbStripFooterRE   = regexp.MustCompile(`(?is)<footer[^>]*>.*?</footer>`)
	kbTagRE           = regexp.MustCompile(`(?s)<[^>]+>`)
	kbTitleRE         = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	kbHrefRE          = regexp.MustCompile(`(?is)href=["']([^"'#]+)["']`)
)

func (g *Gateway) handleKBSearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.knowledgeStore == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	query := r.URL.Query().Get("q")
	if query == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "query parameter 'q' required"})
		return
	}
	limit := 10
	if n := r.URL.Query().Get("n"); n != "" {
		if v, err := strconv.Atoi(n); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > 50 {
		limit = 50
	}
	fileFilter := r.URL.Query().Get("file")
	langFilter := r.URL.Query().Get("lang")
	chunks := g.knowledgeStore.SearchFiltered(query, limit, fileFilter, langFilter)
	json.NewEncoder(w).Encode(map[string]any{"chunks": chunks, "count": len(chunks)})
}

func (g *Gateway) handleKBSources(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.knowledgeStore == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"sources": g.knowledgeStore.Sources()})
}

func (g *Gateway) handleKBStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.knowledgeStore == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	json.NewEncoder(w).Encode(g.knowledgeStore.Stats())
}

// handleKBUpload handles file upload to knowledge base.
// POST /v1/knowledge/upload  (multipart/form-data, field: "file")
func (g *Gateway) handleKBUpload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.knowledgeStore == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}

	// Max 10MB file
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	file, header, err := r.FormFile("file")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "file field required (max 10MB)"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "read file failed"})
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	var (
		src       any
		parseMeta map[string]any
	)
	switch {
	case ext == ".txt" || ext == ".md":
		src, err = g.knowledgeStore.IngestText(header.Filename, string(data))
	case isMinerUSupportedExt(ext):
		if g.documentParser == nil || !g.documentParser.Enabled() {
			json.NewEncoder(w).Encode(map[string]string{"error": "unsupported format: " + ext + " (enable MinerU to parse this file type)"})
			return
		}
		parseResult, parseErr := g.ingestKnowledgeWithMinerU(r.Context(), header.Filename, data)
		if parseErr != nil {
			json.NewEncoder(w).Encode(map[string]string{"error": parseErr.Error()})
			return
		}
		src = parseResult.Source
		parseMeta = parseResult.Response()
	default:
		json.NewEncoder(w).Encode(map[string]string{"error": "unsupported format: " + ext + " (use .txt, .md or enable MinerU for PDF/Office/image files)"})
		return
	}
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Rebuild semantic index in background
	safego.Go("knowledge-reindex", func() {
		if err := g.knowledgeStore.BuildIndex(context.Background()); err != nil {
			slog.Warn("knowledge: reindex after upload failed", "err", err)
		}
	})

	resp := map[string]any{"source": src, "stats": g.knowledgeStore.Stats()}
	if parseMeta != nil {
		resp["parse"] = parseMeta
	}
	json.NewEncoder(w).Encode(resp)
}

type mineruKnowledgeIngestResult struct {
	Source *knowledge.Source
	Parse  map[string]any
}

func (r *mineruKnowledgeIngestResult) Response() map[string]any {
	if r == nil {
		return nil
	}
	return r.Parse
}

func (g *Gateway) ingestKnowledgeWithMinerU(ctx context.Context, filename string, data []byte) (*mineruKnowledgeIngestResult, error) {
	if g.documentParser == nil || !g.documentParser.Enabled() {
		return nil, fmt.Errorf("MinerU is not enabled")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("uploaded file is empty")
	}

	ext := strings.ToLower(filepath.Ext(filename))
	tmpFile, err := os.CreateTemp("", "yunque-mineru-*"+ext)
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("close temp file: %w", err)
	}

	result, err := g.documentParser.ParseFile(ctx, tmpPath)
	if err != nil {
		return nil, err
	}
	markdown := strings.TrimSpace(result.Markdown)
	if markdown == "" {
		return nil, fmt.Errorf("MinerU did not return markdown content")
	}

	name := filename
	if ext != "" {
		name = strings.TrimSuffix(filename, ext) + ".md"
	}
	src, err := g.knowledgeStore.IngestText(name, markdown)
	if err != nil {
		return nil, err
	}
	if src != nil {
		src.Type = knowledge.SourceFile
		src.Path = filename
	}

	return &mineruKnowledgeIngestResult{
		Source: src,
		Parse: map[string]any{
			"parser":          "mineru",
			"backend":         result.Backend,
			"markdown_chars":  len(markdown),
			"has_layout_json": strings.TrimSpace(result.JSON) != "",
		},
	}, nil
}

func isMinerUSupportedExt(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".pdf", ".doc", ".docx", ".ppt", ".pptx", ".png", ".jpg", ".jpeg", ".webp", ".bmp", ".tif", ".tiff":
		return true
	default:
		return false
	}
}

// handleKBIngest handles direct text ingestion.
// POST /v1/knowledge/ingest  {"name": "...", "content": "..."}
func (g *Gateway) handleKBIngest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.knowledgeStore == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	var req struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Content == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "name and content required"})
		return
	}
	if req.Name == "" {
		req.Name = "inline-text"
	}

	src, err := g.knowledgeStore.IngestText(req.Name, req.Content)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	safego.Go("knowledge-reindex", func() {
		if err := g.knowledgeStore.BuildIndex(context.Background()); err != nil {
			slog.Warn("knowledge: reindex after ingest failed", "err", err)
		}
	})

	json.NewEncoder(w).Encode(map[string]any{"source": src, "stats": g.knowledgeStore.Stats()})
}

// handleKBImportURL handles importing text content from a URL.
// POST /v1/knowledge/import-url {"url": "...", "name": "..."}
func (g *Gateway) handleKBImportURL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.knowledgeStore == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	var req struct {
		URL           string `json:"url"`
		Name          string `json:"name"`
		CrawlChildren bool   `json:"crawl_children"`
		MaxPages      int    `json:"max_pages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.URL) == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "url required"})
		return
	}

	if req.MaxPages <= 0 {
		req.MaxPages = 5
	}
	if req.MaxPages > 20 {
		req.MaxPages = 20
	}

	page, err := fetchKnowledgeURLPage(strings.TrimSpace(req.URL), strings.TrimSpace(req.Name))
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	imported := make([]*knowledge.Source, 0, req.MaxPages)
	src, err := g.knowledgeStore.IngestURL(page.Name, page.URL, page.Content)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	imported = append(imported, src)

	if req.CrawlChildren {
		for _, childURL := range extractDeepWikiChildLinks(page.URL, page.RawHTML, req.MaxPages-1) {
			childPage, childErr := fetchKnowledgeURLPage(childURL, "")
			if childErr != nil {
				slog.Warn("knowledge: import child url failed", "url", childURL, "err", childErr)
				continue
			}
			childSrc, childErr := g.knowledgeStore.IngestURL(childPage.Name, childPage.URL, childPage.Content)
			if childErr != nil {
				slog.Warn("knowledge: ingest child url failed", "url", childURL, "err", childErr)
				continue
			}
			imported = append(imported, childSrc)
		}
	}

	safego.Go("knowledge-reindex", func() {
		if err := g.knowledgeStore.BuildIndex(context.Background()); err != nil {
			slog.Warn("knowledge: reindex after import-url failed", "err", err)
		}
	})

	json.NewEncoder(w).Encode(map[string]any{"source": src, "sources": imported, "imported": len(imported), "tree": buildKnowledgeImportTree(page, imported), "stats": g.knowledgeStore.Stats()})
}

// handleKBImportRepo handles importing a local repository or code directory.
// POST /v1/knowledge/import-repo {"path": "...", "max_files": 200}
func (g *Gateway) handleKBImportRepo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.knowledgeStore == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	var req struct {
		Path     string `json:"path"`
		MaxFiles int    `json:"max_files"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Path) == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "path required"})
		return
	}
	src, err := g.knowledgeStore.IngestDirectory(strings.TrimSpace(req.Path), req.MaxFiles)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	safego.Go("knowledge-reindex", func() {
		if err := g.knowledgeStore.BuildIndex(context.Background()); err != nil {
			slog.Warn("knowledge: reindex after import-repo failed", "err", err)
		}
	})
	json.NewEncoder(w).Encode(map[string]any{"source": src, "stats": g.knowledgeStore.Stats()})
}

// handleKBDelete removes a knowledge source by ID.
// DELETE /v1/knowledge/source?id=xxx
func (g *Gateway) handleKBDelete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if g.knowledgeStore == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "knowledge base not configured"})
		return
	}
	sourceID := r.URL.Query().Get("id")
	if sourceID == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "source id required"})
		return
	}
	ok := g.knowledgeStore.RemoveSource(sourceID)
	if !ok {
		json.NewEncoder(w).Encode(map[string]string{"error": "source not found"})
		return
	}

	safego.Go("knowledge-reindex", func() {
		if err := g.knowledgeStore.BuildIndex(context.Background()); err != nil {
			slog.Warn("knowledge: reindex after delete failed", "err", err)
		}
	})

	json.NewEncoder(w).Encode(map[string]any{"deleted": sourceID, "stats": g.knowledgeStore.Stats()})
}

type knowledgeImportPage struct {
	URL     string
	Name    string
	Content string
	RawHTML string
}

type knowledgeImportTreeNode struct {
	Title    string                     `json:"title"`
	URL      string                     `json:"url,omitempty"`
	Path     string                     `json:"path,omitempty"`
	Children []*knowledgeImportTreeNode `json:"children,omitempty"`
}

func fetchKnowledgeURLPage(rawURL, fallbackName string) (*knowledgeImportPage, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported url scheme: %s", parsed.Scheme)
	}

	// SSRF protection: resolve hostname and reject private/loopback IPs
	hostname := parsed.Hostname()
	if isPrivateOrLoopback(hostname) {
		return nil, fmt.Errorf("access to private/loopback addresses is not allowed")
	}
	ips, err := net.LookupHost(hostname)
	if err != nil {
		return nil, fmt.Errorf("dns resolve failed: %w", err)
	}
	for _, ipStr := range ips {
		if isPrivateOrLoopback(ipStr) {
			return nil, fmt.Errorf("access to private/loopback addresses is not allowed")
		}
	}

	client := &http.Client{Timeout: 20 * time.Second}
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
	raw := string(body)
	content := raw
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "html") || looksLikeHTML(raw) {
		content = extractKnowledgeHTML(raw)
	} else {
		content = normalizeImportedText(raw)
	}
	if content == "" {
		return nil, fmt.Errorf("no readable content extracted")
	}

	name := fallbackName
	if name == "" {
		name = deriveKnowledgeName(parsed, raw)
	}
	if name == "" {
		name = rawURL
	}

	final := fmt.Sprintf("# %s\n\nSource: %s\n\n%s", name, rawURL, content)
	return &knowledgeImportPage{URL: rawURL, Name: name, Content: final, RawHTML: raw}, nil
}

// isPrivateOrLoopback checks if an IP or hostname belongs to private, loopback,
// link-local, or other non-routable address ranges (SSRF protection).
func isPrivateOrLoopback(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		// Not a valid IP — check common hostnames
		lower := strings.ToLower(host)
		return lower == "localhost" || strings.HasSuffix(lower, ".local") ||
			lower == "metadata.google.internal" || lower == "169.254.169.254"
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

func extractDeepWikiChildLinks(rootURL, rawHTML string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	root, err := url.Parse(rootURL)
	if err != nil || !strings.Contains(strings.ToLower(root.Host), "deepwiki.com") {
		return nil
	}
	segments := strings.Split(strings.Trim(root.Path, "/"), "/")
	if len(segments) < 2 {
		return nil
	}
	repoPrefix := "/" + segments[0] + "/" + segments[1]
	seen := map[string]struct{}{rootURL: struct{}{}}
	links := make([]string, 0, limit)

	for _, match := range kbHrefRE.FindAllStringSubmatch(rawHTML, -1) {
		candidate := strings.TrimSpace(match[1])
		if candidate == "" {
			continue
		}
		parsed, parseErr := url.Parse(candidate)
		if parseErr != nil {
			continue
		}
		resolved := root.ResolveReference(parsed)
		resolved.RawQuery = ""
		resolved.Fragment = ""
		if !strings.EqualFold(resolved.Host, root.Host) {
			continue
		}
		if !strings.HasPrefix(resolved.Path, repoPrefix+"/") {
			continue
		}
		if resolved.Path == root.Path {
			continue
		}
		finalURL := resolved.String()
		if _, ok := seen[finalURL]; ok {
			continue
		}
		seen[finalURL] = struct{}{}
		links = append(links, finalURL)
		if len(links) >= limit {
			break
		}
	}

	sort.Strings(links)
	if len(links) > limit {
		links = links[:limit]
	}
	return links
}

func buildKnowledgeImportTree(rootPage *knowledgeImportPage, imported []*knowledge.Source) *knowledgeImportTreeNode {
	rootNode := &knowledgeImportTreeNode{Title: rootPage.Name, URL: rootPage.URL, Path: "/"}
	if len(imported) <= 1 {
		return rootNode
	}

	nodes := map[string]*knowledgeImportTreeNode{"": rootNode}
	parsedRoot, err := url.Parse(rootPage.URL)
	if err != nil {
		return rootNode
	}
	segments := strings.Split(strings.Trim(parsedRoot.Path, "/"), "/")
	if len(segments) < 2 {
		return rootNode
	}
	repoBase := "/" + segments[0] + "/" + segments[1]

	for _, src := range imported[1:] {
		parsed, parseErr := url.Parse(src.Path)
		if parseErr != nil {
			continue
		}
		relPath := strings.TrimPrefix(parsed.Path, repoBase)
		relPath = strings.TrimPrefix(relPath, "/")
		if relPath == "" {
			continue
		}
		slug := path.Base(parsed.Path)
		sectionKey := deepWikiSectionKey(slug)
		parentKey := ""
		if sectionKey != "" && strings.Contains(sectionKey, ".") {
			parentKey = sectionKey[:strings.LastIndex(sectionKey, ".")]
		}
		if sectionKey == "" {
			sectionKey = relPath
		}

		parent := ensureKnowledgeTreeNode(nodes, rootNode, parentKey)
		node := ensureKnowledgeTreeNode(nodes, rootNode, sectionKey)
		node.Title = src.Name
		node.URL = src.Path
		node.Path = relPath
		attachKnowledgeTreeNode(parent, node)
	}

	sortKnowledgeTree(rootNode)
	return rootNode
}

func deepWikiSectionKey(slug string) string {
	prefix := slug
	if idx := strings.Index(prefix, "-"); idx >= 0 {
		prefix = prefix[:idx]
	}
	for _, r := range prefix {
		if (r < '0' || r > '9') && r != '.' {
			return ""
		}
	}
	return prefix
}

func ensureKnowledgeTreeNode(nodes map[string]*knowledgeImportTreeNode, root *knowledgeImportTreeNode, key string) *knowledgeImportTreeNode {
	if key == "" {
		return root
	}
	if node, ok := nodes[key]; ok {
		return node
	}
	node := &knowledgeImportTreeNode{Title: key, Path: key}
	nodes[key] = node
	parentKey := ""
	if strings.Contains(key, ".") {
		parentKey = key[:strings.LastIndex(key, ".")]
	}
	parent := ensureKnowledgeTreeNode(nodes, root, parentKey)
	attachKnowledgeTreeNode(parent, node)
	return node
}

func attachKnowledgeTreeNode(parent, child *knowledgeImportTreeNode) {
	for _, existing := range parent.Children {
		if existing == child {
			return
		}
	}
	parent.Children = append(parent.Children, child)
}

func sortKnowledgeTree(node *knowledgeImportTreeNode) {
	for _, child := range node.Children {
		sortKnowledgeTree(child)
	}
	sort.Slice(node.Children, func(i, j int) bool {
		return node.Children[i].Path < node.Children[j].Path
	})
}

func deriveKnowledgeName(parsed *url.URL, raw string) string {
	if matches := kbTitleRE.FindStringSubmatch(raw); len(matches) > 1 {
		title := normalizeImportedText(matches[1])
		if title != "" {
			return title
		}
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) > 0 && segments[0] != "" {
		return segments[len(segments)-1]
	}
	return parsed.Host
}

func looksLikeHTML(raw string) bool {
	s := strings.ToLower(raw)
	return strings.Contains(s, "<html") || strings.Contains(s, "<body") || strings.Contains(s, "<main")
}

func extractKnowledgeHTML(raw string) string {
	cleaned := raw
	for _, pattern := range []*regexp.Regexp{kbStripScriptRE, kbStripStyleRE, kbStripSVGRE, kbStripNoscriptRE, kbStripHeaderRE, kbStripFooterRE} {
		cleaned = pattern.ReplaceAllString(cleaned, " ")
	}
	cleaned = kbTagRE.ReplaceAllString(cleaned, "\n")
	return normalizeImportedText(cleaned)
}

func normalizeImportedText(raw string) string {
	replacer := strings.NewReplacer(
		"\r", "\n",
		"\t", " ",
		"[Image: Image]", " ",
		"•", "- ",
	)
	raw = html.UnescapeString(replacer.Replace(raw))

	lines := strings.Split(raw, "\n")
	filtered := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		line = strings.Join(strings.Fields(strings.TrimSpace(line)), " ")
		if line == "" {
			if !blank && len(filtered) > 0 {
				filtered = append(filtered, "")
			}
			blank = true
			continue
		}
		if strings.EqualFold(line, "DeepWiki") || strings.EqualFold(line, "Edit Wiki") || strings.EqualFold(line, "Share") {
			continue
		}
		filtered = append(filtered, line)
		blank = false
	}

	return strings.TrimSpace(strings.Join(filtered, "\n"))
}
