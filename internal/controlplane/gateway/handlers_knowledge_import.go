package gateway

import (
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/knowledge"
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
	seen := map[string]struct{}{rootURL: {}}
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
