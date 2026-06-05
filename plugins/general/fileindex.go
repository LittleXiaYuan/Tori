package general

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"yunque-agent/pkg/skills"
)

// FileIndex is Yunque's built-in "Everything-like" filename index: an in-process
// cache of file paths under a fixed set of read roots. The first query builds it
// (bounded by an entry cap + a wall-clock budget so it never runs away), and
// every subsequent query is instant from memory until the TTL lapses or the root
// set changes. No external dependency (no Everything install, no ripgrep) — it's
// native to the agent and works the same on every machine.
type FileIndex struct {
	buildMu sync.Mutex // serializes (single-flights) rebuilds

	mu        sync.RWMutex // guards the fields below
	entries   []fileIndexEntry
	rootsKey  string
	builtAt   time.Time
	truncated bool
}

type fileIndexEntry struct {
	lower string // lower-cased base name for matching
	path  string // absolute path
}

const (
	fileIndexMaxEntries  = 400_000
	fileIndexTTL         = 5 * time.Minute
	fileIndexBuildBudget = 25 * time.Second
)

// fileIndexSkipDirs are pruned while indexing: huge, machine-generated, or
// privacy-sensitive trees the agent almost never wants to find files in by name.
var fileIndexSkipDirs = map[string]bool{
	"node_modules": true, ".git": true, ".svn": true, ".hg": true,
	"$recycle.bin": true, "system volume information": true,
	"$windows.~bt": true, "$windows.~ws": true,
	"appdata": true, "windows": true,
	".cache": true, "__pycache__": true, ".venv": true, "venv": true,
	".gradle": true, ".m2": true, ".npm": true, ".pnpm-store": true,
}

func newFileIndex() *FileIndex { return &FileIndex{} }

func rootsKeyOf(roots []string) string {
	cp := append([]string{}, roots...)
	sort.Strings(cp)
	return strings.Join(cp, "|")
}

// ensure (re)builds the index if it is empty, stale (past TTL), or built for a
// different root set. Safe for concurrent callers — the actual build is
// single-flighted under buildMu.
func (ix *FileIndex) ensure(roots []string) {
	key := rootsKeyOf(roots)
	if ix.fresh(key) {
		return
	}
	ix.buildMu.Lock()
	defer ix.buildMu.Unlock()
	if ix.fresh(key) { // re-check: another goroutine may have built it
		return
	}
	entries, truncated := buildFileIndex(roots)
	ix.mu.Lock()
	ix.entries = entries
	ix.rootsKey = key
	ix.builtAt = time.Now()
	ix.truncated = truncated
	ix.mu.Unlock()
}

func (ix *FileIndex) fresh(key string) bool {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	return ix.rootsKey == key && len(ix.entries) > 0 && time.Since(ix.builtAt) < fileIndexTTL
}

func buildFileIndex(roots []string) ([]fileIndexEntry, bool) {
	deadline := time.Now().Add(fileIndexBuildBudget)
	out := make([]fileIndexEntry, 0, 8192)
	truncated := false
	seen := map[string]bool{}
	for _, root := range roots {
		abs, err := filepath.Abs(root)
		if err != nil || seen[abs] {
			continue
		}
		seen[abs] = true
		_ = filepath.WalkDir(abs, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if len(out) >= fileIndexMaxEntries || time.Now().After(deadline) {
				truncated = true
				return filepath.SkipAll
			}
			if d.IsDir() {
				if p != abs && fileIndexSkipDirs[strings.ToLower(d.Name())] {
					return filepath.SkipDir
				}
				return nil
			}
			out = append(out, fileIndexEntry{lower: strings.ToLower(d.Name()), path: p})
			return nil
		})
		if truncated {
			break
		}
	}
	return out, truncated
}

// find returns up to limit paths whose base name contains query (case-insensitive).
func (ix *FileIndex) find(query string, limit int) []string {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	if limit <= 0 {
		limit = 50
	}
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	var res []string
	for _, e := range ix.entries {
		if strings.Contains(e.lower, q) {
			res = append(res, e.path)
			if len(res) >= limit {
				break
			}
		}
	}
	return res
}

func (ix *FileIndex) stats() (int, bool) {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	return len(ix.entries), ix.truncated
}

// ── file_find skill ─────────────────────────────────────────────────────────

// FileFindSkill is the instant filename search exposed to the planner.
type FileFindSkill struct {
	hostReadPaths []string
	ix            *FileIndex
}

func NewFileFindSkill(hostReadPaths []string) *FileFindSkill {
	return &FileFindSkill{hostReadPaths: hostReadPaths, ix: newFileIndex()}
}

func (s *FileFindSkill) Name() string { return "file_find" }
func (s *FileFindSkill) Description() string {
	return "按文件名【秒级】查找(内置索引,首次构建后即时,无需 Everything)。给文件名关键词即返回匹配的完整路径,比 file_search 的遍历快得多——'叫 xxx 的文件在哪'优先用它。仅在允许的读目录内。"
}

func (s *FileFindSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string", "description": "文件名关键词(子串,大小写不敏感)"},
			"limit": map[string]any{"type": "integer", "description": "最多返回数(默认 50)"},
		},
		"required": []string{"query"},
	}
}

func (s *FileFindSkill) Execute(_ context.Context, args map[string]any, env *skills.Environment) (string, error) {
	query, _ := args["query"].(string)
	if strings.TrimSpace(query) == "" {
		return "", fmt.Errorf("query is required")
	}
	roots := mergeWorkspacePaths(s.hostReadPaths, env)
	if len(roots) == 0 {
		return "", fmt.Errorf("no readable directories are configured (set HOST_READ_PATHS or open a folder)")
	}
	s.ix.ensure(roots)
	matches := s.ix.find(query, asInt(args["limit"]))
	indexed, truncated := s.ix.stats()
	out, _ := json.Marshal(map[string]any{
		"matches":   matches,
		"count":     len(matches),
		"indexed":   indexed,
		"truncated": truncated,
	})
	return string(out), nil
}
