package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ──────────────────────────────────────────────
// File system tool results
// ──────────────────────────────────────────────

type ReadResult struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Lines   int    `json:"lines"`
	Size    int64  `json:"size"`
}

type WriteResult struct {
	Path    string `json:"path"`
	Written int    `json:"written"` // bytes written
	Created bool   `json:"created"` // true if file was new
}

type EditResult struct {
	Path     string `json:"path"`
	Replaced int    `json:"replaced"` // number of replacements
}

type GrepMatch struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

type GrepResult struct {
	Pattern string      `json:"pattern"`
	Matches []GrepMatch `json:"matches"`
	Total   int         `json:"total"`
}

type FindResult struct {
	Pattern string   `json:"pattern"`
	Files   []string `json:"files"`
	Total   int      `json:"total"`
}

type LsEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

type LsResult struct {
	Path    string    `json:"path"`
	Entries []LsEntry `json:"entries"`
}

// ──────────────────────────────────────────────
// FileSystem tool — sandboxable file operations
// ──────────────────────────────────────────────

// FileSystem provides file operations scoped to a root directory.
type FileSystem struct {
	root string // workspace root
}

// NewFileSystem creates a file system tool scoped to root.
func NewFileSystem(root string) *FileSystem {
	return &FileSystem{root: root}
}

// resolve ensures the path is within the workspace root.
func (fs *FileSystem) resolve(path string) (string, error) {
	if filepath.IsAbs(path) {
		// Check if it's under root
		rel, err := filepath.Rel(fs.root, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", fmt.Errorf("path %q is outside workspace root", path)
		}
		return filepath.Clean(path), nil
	}
	abs := filepath.Join(fs.root, path)
	rel, err := filepath.Rel(fs.root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path %q escapes workspace root", path)
	}
	return filepath.Clean(abs), nil
}

// Read reads a file's contents. Supports optional line offset/limit.
func (fs *FileSystem) Read(path string, offset, limit int) (*ReadResult, error) {
	abs, err := fs.resolve(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	if offset > 0 || limit > 0 {
		if offset < 1 {
			offset = 1
		}
		start := offset - 1
		if start >= len(lines) {
			return &ReadResult{Path: path, Content: "", Lines: 0, Size: int64(len(data))}, nil
		}
		end := len(lines)
		if limit > 0 && start+limit < end {
			end = start + limit
		}
		lines = lines[start:end]
		content = strings.Join(lines, "\n")
	}

	info, _ := os.Stat(abs)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}

	return &ReadResult{
		Path:    path,
		Content: content,
		Lines:   len(lines),
		Size:    size,
	}, nil
}

// Write creates or overwrites a file.
func (fs *FileSystem) Write(path, content string) (*WriteResult, error) {
	abs, err := fs.resolve(path)
	if err != nil {
		return nil, err
	}

	_, statErr := os.Stat(abs)
	created := os.IsNotExist(statErr)

	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("write: mkdir: %w", err)
	}

	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	return &WriteResult{
		Path:    path,
		Written: len(content),
		Created: created,
	}, nil
}

// Edit performs find-and-replace in a file.
func (fs *FileSystem) Edit(path, oldStr, newStr string, replaceAll bool) (*EditResult, error) {
	abs, err := fs.resolve(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("edit: %w", err)
	}

	content := string(data)
	count := 0

	if replaceAll {
		count = strings.Count(content, oldStr)
		content = strings.ReplaceAll(content, oldStr, newStr)
	} else {
		if strings.Contains(content, oldStr) {
			content = strings.Replace(content, oldStr, newStr, 1)
			count = 1
		}
	}

	if count == 0 {
		return nil, fmt.Errorf("edit: old_string not found in %s", path)
	}

	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("edit: write: %w", err)
	}

	return &EditResult{Path: path, Replaced: count}, nil
}

// Grep searches file contents for a pattern (regex or fixed string).
func (fs *FileSystem) Grep(pattern, searchPath string, fixedString bool, maxResults int) (*GrepResult, error) {
	abs, err := fs.resolve(searchPath)
	if err != nil {
		return nil, err
	}

	if maxResults <= 0 {
		maxResults = 100
	}

	var re *regexp.Regexp
	if !fixedString {
		re, err = regexp.Compile("(?i)" + pattern)
		if err != nil {
			return nil, fmt.Errorf("grep: invalid pattern: %w", err)
		}
	}

	result := &GrepResult{Pattern: pattern}

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if info.Size() > 10*1024*1024 { // skip files > 10MB
			return nil
		}
		if isBinary(path) {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(fs.root, path)
		lines := strings.Split(string(data), "\n")

		for i, line := range lines {
			if len(result.Matches) >= maxResults {
				return filepath.SkipAll
			}

			matched := false
			if fixedString {
				matched = strings.Contains(strings.ToLower(line), strings.ToLower(pattern))
			} else {
				matched = re.MatchString(line)
			}

			if matched {
				result.Matches = append(result.Matches, GrepMatch{
					File:    relPath,
					Line:    i + 1,
					Content: truncate(strings.TrimSpace(line), 200),
				})
			}
		}
		return nil
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("grep: %w", err)
	}

	if info.IsDir() {
		filepath.Walk(abs, walkFn)
	} else {
		walkFn(abs, info, nil)
	}

	result.Total = len(result.Matches)
	return result, nil
}

// Find searches for files by glob pattern.
func (fs *FileSystem) Find(pattern, searchPath string, maxResults int) (*FindResult, error) {
	abs, err := fs.resolve(searchPath)
	if err != nil {
		return nil, err
	}
	if maxResults <= 0 {
		maxResults = 50
	}

	result := &FindResult{Pattern: pattern}

	filepath.Walk(abs, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if len(result.Files) >= maxResults {
			return filepath.SkipAll
		}

		name := info.Name()
		matched, _ := filepath.Match(pattern, name)
		if matched {
			relPath, _ := filepath.Rel(fs.root, path)
			result.Files = append(result.Files, relPath)
		}
		return nil
	})

	result.Total = len(result.Files)
	return result, nil
}

// Ls lists directory contents.
func (fs *FileSystem) Ls(path string) (*LsResult, error) {
	abs, err := fs.resolve(path)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, fmt.Errorf("ls: %w", err)
	}

	result := &LsResult{Path: path}
	for _, e := range entries {
		info, _ := e.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		result.Entries = append(result.Entries, LsEntry{
			Name:  e.Name(),
			IsDir: e.IsDir(),
			Size:  size,
		})
	}
	return result, nil
}

// isBinary does a simple check if a file is likely binary.
func isBinary(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	binaryExts := map[string]bool{
		".exe": true, ".dll": true, ".so": true, ".dylib": true,
		".zip": true, ".tar": true, ".gz": true, ".bz2": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
		".ico": true, ".bmp": true, ".webp": true,
		".pdf": true, ".doc": true, ".xls": true,
		".wasm": true, ".o": true, ".a": true,
		".db": true, ".sqlite": true,
	}
	return binaryExts[ext]
}

// ──────────────────────────────────────────────
// Extended file operations
// ──────────────────────────────────────────────

// StatResult provides detailed file information.
type StatResult struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"is_dir"`
	ModTime string `json:"mod_time"`
	Mode    string `json:"mode"`
	Exists  bool   `json:"exists"`
}

// Stat returns file information.
func (fs *FileSystem) Stat(path string) (*StatResult, error) {
	abs, err := fs.resolve(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if os.IsNotExist(err) {
		return &StatResult{Path: path, Exists: false}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("stat: %w", err)
	}
	return &StatResult{
		Path:    path,
		Size:    info.Size(),
		IsDir:   info.IsDir(),
		ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
		Mode:    info.Mode().String(),
		Exists:  true,
	}, nil
}

// Mkdir creates a directory (including parents).
func (fs *FileSystem) Mkdir(path string) error {
	abs, err := fs.resolve(path)
	if err != nil {
		return err
	}
	return os.MkdirAll(abs, 0o755)
}

// Delete removes a file or empty directory. Does NOT do recursive delete for safety.
func (fs *FileSystem) Delete(path string) error {
	abs, err := fs.resolve(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	if info.IsDir() {
		entries, _ := os.ReadDir(abs)
		if len(entries) > 0 {
			return fmt.Errorf("delete: directory %q is not empty (use recursive=true if intended)", path)
		}
		return os.Remove(abs)
	}
	return os.Remove(abs)
}

// MoveResult represents the result of a move/rename operation.
type MoveResult struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Move renames or moves a file/directory within the workspace.
func (fs *FileSystem) Move(src, dst string) (*MoveResult, error) {
	absSrc, err := fs.resolve(src)
	if err != nil {
		return nil, fmt.Errorf("move: source: %w", err)
	}
	absDst, err := fs.resolve(dst)
	if err != nil {
		return nil, fmt.Errorf("move: dest: %w", err)
	}
	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(absDst), 0o755); err != nil {
		return nil, fmt.Errorf("move: mkdir: %w", err)
	}
	if err := os.Rename(absSrc, absDst); err != nil {
		return nil, fmt.Errorf("move: %w", err)
	}
	return &MoveResult{From: src, To: dst}, nil
}

// Copy copies a file within the workspace.
func (fs *FileSystem) Copy(src, dst string) (*WriteResult, error) {
	absSrc, err := fs.resolve(src)
	if err != nil {
		return nil, fmt.Errorf("copy: source: %w", err)
	}
	absDst, err := fs.resolve(dst)
	if err != nil {
		return nil, fmt.Errorf("copy: dest: %w", err)
	}

	info, err := os.Stat(absSrc)
	if err != nil {
		return nil, fmt.Errorf("copy: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("copy: cannot copy directory (use move or shell)")
	}

	data, err := os.ReadFile(absSrc)
	if err != nil {
		return nil, fmt.Errorf("copy: read: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(absDst), 0o755); err != nil {
		return nil, fmt.Errorf("copy: mkdir: %w", err)
	}

	_, statErr := os.Stat(absDst)
	if err := os.WriteFile(absDst, data, 0o644); err != nil {
		return nil, fmt.Errorf("copy: write: %w", err)
	}

	return &WriteResult{
		Path:    dst,
		Written: len(data),
		Created: os.IsNotExist(statErr),
	}, nil
}

// TreeEntry represents a node in a directory tree.
type TreeEntry struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	IsDir    bool        `json:"is_dir"`
	Size     int64       `json:"size,omitempty"`
	Children []TreeEntry `json:"children,omitempty"`
}

// Tree returns a recursive directory listing (max depth limited).
func (fs *FileSystem) Tree(path string, maxDepth int) (*TreeEntry, error) {
	abs, err := fs.resolve(path)
	if err != nil {
		return nil, err
	}
	if maxDepth <= 0 {
		maxDepth = 3
	}
	return fs.buildTree(abs, path, maxDepth, 0)
}

func (fs *FileSystem) buildTree(absPath, relPath string, maxDepth, depth int) (*TreeEntry, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}

	entry := &TreeEntry{
		Name:  info.Name(),
		Path:  relPath,
		IsDir: info.IsDir(),
		Size:  info.Size(),
	}

	if !info.IsDir() || depth >= maxDepth {
		return entry, nil
	}

	children, err := os.ReadDir(absPath)
	if err != nil {
		return entry, nil
	}

	for _, child := range children {
		childAbs := filepath.Join(absPath, child.Name())
		childRel := filepath.Join(relPath, child.Name())
		childEntry, _ := fs.buildTree(childAbs, childRel, maxDepth, depth+1)
		if childEntry != nil {
			entry.Children = append(entry.Children, *childEntry)
		}
	}

	return entry, nil
}

// Ensure json import is used
var _ = json.Marshal
