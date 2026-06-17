package knowledge

// webimport.go holds the URL/web import domain logic shared by the gateway
// (its SSRF-safe fetch wrapper + natural-language config) and the knowledge
// pack's native import-url handler.
//
// These are pure functions: no HTTP, no SSRF, no gateway dependency. The
// SSRF-safe fetch deliberately stays in the gateway (it is shared with other
// outbound-fetch features); callers fetch the body there and hand it to
// BuildPage for transport-free cleaning.

import (
	"fmt"
	"html"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
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

// ImportPage is a fetched + cleaned web page staged for knowledge ingestion.
type ImportPage struct {
	URL     string
	Name    string
	Content string
	RawHTML string
}

// ImportTreeNode is a node in the import preview tree (e.g. DeepWiki chapters).
type ImportTreeNode struct {
	Title    string            `json:"title"`
	URL      string            `json:"url,omitempty"`
	Path     string            `json:"path,omitempty"`
	Children []*ImportTreeNode `json:"children,omitempty"`
}

// BuildPage turns an already-fetched HTTP body into a cleaned ImportPage.
// contentType is the response Content-Type header, used to choose HTML vs
// plain-text extraction. fallbackName wins over the derived title when set.
func BuildPage(rawURL, fallbackName, rawBody, contentType string) (*ImportPage, error) {
	raw := rawBody
	var content string
	if strings.Contains(strings.ToLower(contentType), "html") || looksLikeHTML(raw) {
		content = ExtractHTML(raw)
	} else {
		content = normalizeImportedText(raw)
	}
	if content == "" {
		return nil, fmt.Errorf("no readable content extracted")
	}

	name := fallbackName
	if name == "" {
		name = deriveName(rawURL, raw)
	}
	if name == "" {
		name = rawURL
	}

	final := fmt.Sprintf("# %s\n\nSource: %s\n\n%s", name, rawURL, content)
	return &ImportPage{URL: rawURL, Name: name, Content: final, RawHTML: raw}, nil
}

// ExtractChildLinks returns same-repo child links discovered in a DeepWiki
// page's HTML, capped at limit and sorted for deterministic crawling.
func ExtractChildLinks(rootURL, rawHTML string, limit int) []string {
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

// BuildImportTree builds a chapter tree for the imported DeepWiki pages.
func BuildImportTree(rootPage *ImportPage, imported []*Source) *ImportTreeNode {
	rootNode := &ImportTreeNode{Title: rootPage.Name, URL: rootPage.URL, Path: "/"}
	if len(imported) <= 1 {
		return rootNode
	}

	nodes := map[string]*ImportTreeNode{"": rootNode}
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

		parent := ensureImportTreeNode(nodes, rootNode, parentKey)
		node := ensureImportTreeNode(nodes, rootNode, sectionKey)
		node.Title = src.Name
		node.URL = src.Path
		node.Path = relPath
		attachImportTreeNode(parent, node)
	}

	sortImportTree(rootNode)
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

func ensureImportTreeNode(nodes map[string]*ImportTreeNode, root *ImportTreeNode, key string) *ImportTreeNode {
	if key == "" {
		return root
	}
	if node, ok := nodes[key]; ok {
		return node
	}
	node := &ImportTreeNode{Title: key, Path: key}
	nodes[key] = node
	parentKey := ""
	if strings.Contains(key, ".") {
		parentKey = key[:strings.LastIndex(key, ".")]
	}
	parent := ensureImportTreeNode(nodes, root, parentKey)
	attachImportTreeNode(parent, node)
	return node
}

func attachImportTreeNode(parent, child *ImportTreeNode) {
	for _, existing := range parent.Children {
		if existing == child {
			return
		}
	}
	parent.Children = append(parent.Children, child)
}

func sortImportTree(node *ImportTreeNode) {
	for _, child := range node.Children {
		sortImportTree(child)
	}
	sort.Slice(node.Children, func(i, j int) bool {
		return node.Children[i].Path < node.Children[j].Path
	})
}

func deriveName(rawURL, raw string) string {
	if matches := kbTitleRE.FindStringSubmatch(raw); len(matches) > 1 {
		title := normalizeImportedText(matches[1])
		if title != "" {
			return title
		}
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
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

// ExtractHTML strips scripts/styles/chrome and markup, returning readable text.
func ExtractHTML(raw string) string {
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
		"\u2022", "- ",
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
