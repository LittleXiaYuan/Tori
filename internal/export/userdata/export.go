package userdata

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	reflectpkg "yunque-agent/internal/experimental/reflect"
	yledger "yunque-agent/internal/ledger"
	ledger "yunque-agent/internal/ledgercore"
)

// Options configures a readable user-data export.
type Options struct {
	DataDir string
	OutDir  string
	Now     func() time.Time
}

// Report describes the generated export artifact.
type Report struct {
	ExportDir     string    `json:"export_dir"`
	GeneratedAt   time.Time `json:"generated_at"`
	DataDir       string    `json:"data_dir"`
	MemoryCount   int       `json:"memory_count"`
	SessionCount  int       `json:"session_count"`
	FeedbackCount int       `json:"feedback_count"`
	RawFiles      []RawFile `json:"raw_files"`
	Warnings      []string  `json:"warnings,omitempty"`
}

// RawFile records a copied raw source file.
type RawFile struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type memoryRecord struct {
	ID        string    `json:"id,omitempty"`
	TenantID  string    `json:"tenant_id,omitempty"`
	Kind      string    `json:"kind,omitempty"`
	Key       string    `json:"key,omitempty"`
	Content   string    `json:"content,omitempty"`
	Source    string    `json:"source,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type sessionRecord struct {
	ID         string           `json:"id"`
	TenantID   string           `json:"tenant_id,omitempty"`
	Name       string           `json:"name,omitempty"`
	Summary    string           `json:"summary,omitempty"`
	Pinned     bool             `json:"pinned,omitempty"`
	ArchivedAt *time.Time       `json:"archived_at,omitempty"`
	Messages   []sessionMessage `json:"messages"`
}

type sessionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Export creates a readable export folder with Markdown summaries plus raw copies.
func Export(ctx context.Context, opts Options) (*Report, error) {
	dataDir := strings.TrimSpace(opts.DataDir)
	if dataDir == "" {
		return nil, fmt.Errorf("data dir is required")
	}
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, err
	}
	outRoot := strings.TrimSpace(opts.OutDir)
	if outRoot == "" {
		outRoot = filepath.Join(absDataDir, "exports")
	}
	absOutRoot, err := filepath.Abs(outRoot)
	if err != nil {
		return nil, err
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	generatedAt := now().UTC()
	exportDir := filepath.Join(absOutRoot, "yunque-export-"+generatedAt.Format("20060102-150405"))
	if err := os.MkdirAll(filepath.Join(exportDir, "raw"), 0o755); err != nil {
		return nil, err
	}

	report := &Report{ExportDir: exportDir, GeneratedAt: generatedAt, DataDir: absDataDir}

	memories := readMemories(ctx, absDataDir, report)
	sessions := readSessions(absDataDir, report)
	feedback := readFeedback(ctx, absDataDir, report)

	report.MemoryCount = len(memories)
	report.SessionCount = len(sessions)
	report.FeedbackCount = len(feedback)

	if err := writeFile(exportDir, "README.md", []byte(renderREADME(report))); err != nil {
		return nil, err
	}
	if err := writeFile(exportDir, "memory.md", []byte(renderMemories(memories))); err != nil {
		return nil, err
	}
	if err := writeFile(exportDir, "conversations.md", []byte(renderSessions(sessions))); err != nil {
		return nil, err
	}
	if err := writeFile(exportDir, "feedback.md", []byte(renderFeedback(feedback))); err != nil {
		return nil, err
	}

	copyRawSources(absDataDir, exportDir, report)
	manifest, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := writeFile(exportDir, "manifest.json", append(manifest, '\n')); err != nil {
		return nil, err
	}
	return report, nil
}

func readMemories(ctx context.Context, dataDir string, report *Report) []memoryRecord {
	var out []memoryRecord
	if path := filepath.Join(dataDir, "memory.json"); fileExists(path) {
		data, err := os.ReadFile(path)
		if err != nil {
			report.Warnings = append(report.Warnings, "memory.json read failed: "+err.Error())
		} else {
			out = append(out, parseMemoryJSON(data, "memory.json")...)
		}
	}
	dbPath := filepath.Join(dataDir, "ledger", "ledger.db")
	if fileExists(dbPath) {
		ldg, err := yledger.InitLedgerAt(dbPath)
		if err != nil {
			report.Warnings = append(report.Warnings, "ledger memory export skipped: "+err.Error())
		} else {
			defer ldg.Close()
			seen := make(map[string]struct{})
			for _, tenantID := range []string{"", "default", "personal"} {
				entries, err := ldg.Memory.Search(ctx, ledger.MemoryQuery{TenantID: tenantID, Limit: 5000})
				if err != nil {
					report.Warnings = append(report.Warnings, "ledger memory search failed: "+err.Error())
					break
				}
				for _, m := range entries {
					if _, ok := seen[m.ID]; ok {
						continue
					}
					seen[m.ID] = struct{}{}
					out = append(out, memoryRecord{ID: m.ID, TenantID: m.TenantID, Kind: string(m.Kind), Key: m.Key, Content: m.Content, Source: m.Source, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt})
				}
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func parseMemoryJSON(data []byte, source string) []memoryRecord {
	var arr []map[string]any
	if json.Unmarshal(data, &arr) == nil {
		out := make([]memoryRecord, 0, len(arr))
		for _, item := range arr {
			out = append(out, memoryFromMap(item, source))
		}
		return out
	}
	var obj map[string]any
	if json.Unmarshal(data, &obj) == nil {
		var out []memoryRecord
		flattenMemoryMap(&out, "", obj, source)
		return out
	}
	return nil
}

func flattenMemoryMap(out *[]memoryRecord, prefix string, v any, source string) {
	switch t := v.(type) {
	case map[string]any:
		if hasAny(t, "content", "value", "text", "summary") {
			rec := memoryFromMap(t, source)
			if rec.Key == "" {
				rec.Key = strings.Trim(prefix, "/")
			}
			*out = append(*out, rec)
			return
		}
		for k, child := range t {
			next := k
			if prefix != "" {
				next = prefix + "/" + k
			}
			flattenMemoryMap(out, next, child, source)
		}
	case []any:
		for i, child := range t {
			flattenMemoryMap(out, fmt.Sprintf("%s/%d", prefix, i), child, source)
		}
	case string:
		if strings.TrimSpace(t) != "" {
			*out = append(*out, memoryRecord{Key: strings.Trim(prefix, "/"), Content: t, Source: source})
		}
	}
}

func memoryFromMap(m map[string]any, fallbackSource string) memoryRecord {
	return memoryRecord{
		ID:        strAny(m["id"]),
		TenantID:  strAny(m["tenant_id"]),
		Kind:      strAny(m["kind"]),
		Key:       firstStr(m, "key", "title", "name"),
		Content:   firstStr(m, "content", "value", "text", "summary"),
		Source:    firstNonEmpty(strAny(m["source"]), fallbackSource),
		CreatedAt: parseTimeAny(m["created_at"]),
		UpdatedAt: parseTimeAny(m["updated_at"]),
	}
}

func readSessions(dataDir string, report *Report) []sessionRecord {
	dir := filepath.Join(dataDir, "sessions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			report.Warnings = append(report.Warnings, "sessions read failed: "+err.Error())
		}
		return nil
	}
	var out []sessionRecord
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var s sessionRecord
		if json.Unmarshal(data, &s) != nil {
			report.Warnings = append(report.Warnings, "session parse skipped: "+e.Name())
			continue
		}
		if s.ID == "" {
			s.ID = strings.TrimSuffix(e.Name(), ".json")
		}
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func readFeedback(ctx context.Context, dataDir string, report *Report) []reflectpkg.Experience {
	byID := map[string]reflectpkg.Experience{}
	appendExp := func(items []reflectpkg.Experience) {
		for _, e := range items {
			key := e.ID
			if key == "" {
				key = fmt.Sprintf("%s/%s/%s/%s", e.Source, e.SourceID, e.Lesson, e.CreatedAt)
			}
			byID[key] = e
		}
	}
	if path := filepath.Join(dataDir, "experience.json"); fileExists(path) {
		store := reflectpkg.NewExperienceStore(path)
		appendExp(store.All())
	}
	dbPath := filepath.Join(dataDir, "ledger", "ledger.db")
	if fileExists(dbPath) {
		ldg, err := yledger.InitLedgerAt(dbPath)
		if err != nil {
			report.Warnings = append(report.Warnings, "ledger feedback export skipped: "+err.Error())
		} else {
			defer ldg.Close()
			var exps []reflectpkg.Experience
			found, err := ldg.KV.Get(ctx, "workload_feedback", "data", &exps)
			if err != nil {
				report.Warnings = append(report.Warnings, "ledger workload feedback read failed: "+err.Error())
			} else if found {
				appendExp(exps)
			}
		}
	}
	out := make([]reflectpkg.Experience, 0, len(byID))
	for _, e := range byID {
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func copyRawSources(dataDir, exportDir string, report *Report) {
	rawRoot := filepath.Join(exportDir, "raw")
	for _, rel := range []string{"memory.json", "memory.json.migrated", "experience.json", "experience.json.migrated", "working_memory.json", "adaptive.json", "graph.json", "editable.json", "audit.jsonl", filepath.Join("ledger", "ledger.db")} {
		copyOneRaw(dataDir, rawRoot, rel, report)
	}
	copyRawDir(dataDir, rawRoot, "sessions", report)
}

func copyRawDir(dataDir, rawRoot, relDir string, report *Report) {
	absDir := filepath.Join(dataDir, relDir)
	if !dirExists(absDir) {
		return
	}
	filepath.WalkDir(absDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dataDir, path)
		copyOneRaw(dataDir, rawRoot, rel, report)
		return nil
	})
}

func copyOneRaw(dataDir, rawRoot, rel string, report *Report) {
	src := filepath.Join(dataDir, rel)
	info, err := os.Stat(src)
	if err != nil || info.IsDir() {
		return
	}
	dst := filepath.Join(rawRoot, rel)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		report.Warnings = append(report.Warnings, "raw mkdir failed: "+err.Error())
		return
	}
	in, err := os.Open(src)
	if err != nil {
		report.Warnings = append(report.Warnings, "raw open failed: "+rel)
		return
	}
	defer in.Close()
	var buf bytes.Buffer
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(&buf, h), in)
	if err != nil {
		report.Warnings = append(report.Warnings, "raw read failed: "+rel)
		return
	}
	if err := os.WriteFile(dst, buf.Bytes(), 0o644); err != nil {
		report.Warnings = append(report.Warnings, "raw write failed: "+rel)
		return
	}
	report.RawFiles = append(report.RawFiles, RawFile{Path: filepath.ToSlash(filepath.Join("raw", rel)), Bytes: n, SHA256: hex.EncodeToString(h.Sum(nil))})
}

func renderREADME(r *Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# 云雀用户数据导出\n\n")
	fmt.Fprintf(&b, "- 生成时间：%s\n- 数据目录：`%s`\n- 记忆：%d 条\n- 对话：%d 个会话\n- Feedback：%d 条\n\n", r.GeneratedAt.Format(time.RFC3339), r.DataDir, r.MemoryCount, r.SessionCount, r.FeedbackCount)
	b.WriteString("## 文件说明\n\n- `memory.md`：可读记忆导出。\n- `conversations.md`：可读对话导出。\n- `feedback.md`：可读 workload feedback / 反思经验。\n- `manifest.json`：机器可读清单、计数、原始文件哈希。\n- `raw/`：原始 JSON / SQLite / 会话文件副本，用于迁移或审计。\n\n")
	if len(r.Warnings) > 0 {
		b.WriteString("## 注意\n\n")
		for _, w := range r.Warnings {
			fmt.Fprintf(&b, "- %s\n", w)
		}
	}
	return b.String()
}

func renderMemories(items []memoryRecord) string {
	var b strings.Builder
	b.WriteString("# 记忆导出\n\n")
	if len(items) == 0 {
		b.WriteString("未发现可导出的记忆。\n")
		return b.String()
	}
	for _, m := range items {
		title := firstNonEmpty(m.Key, m.ID, "未命名记忆")
		fmt.Fprintf(&b, "## %s\n\n", mdEscape(title))
		meta := compactMeta(map[string]string{"id": m.ID, "tenant": m.TenantID, "kind": m.Kind, "source": m.Source, "created": formatTime(m.CreatedAt), "updated": formatTime(m.UpdatedAt)})
		if meta != "" {
			fmt.Fprintf(&b, "_%s_\n\n", meta)
		}
		fmt.Fprintf(&b, "%s\n\n", strings.TrimSpace(m.Content))
	}
	return b.String()
}

func renderSessions(items []sessionRecord) string {
	var b strings.Builder
	b.WriteString("# 对话导出\n\n")
	if len(items) == 0 {
		b.WriteString("未发现可导出的对话。\n")
		return b.String()
	}
	for _, s := range items {
		title := firstNonEmpty(s.Name, s.ID)
		fmt.Fprintf(&b, "## %s\n\n", mdEscape(title))
		if s.Summary != "" {
			fmt.Fprintf(&b, "> %s\n\n", strings.ReplaceAll(s.Summary, "\n", "\n> "))
		}
		for i, msg := range s.Messages {
			fmt.Fprintf(&b, "### %d. %s\n\n%s\n\n", i+1, mdEscape(msg.Role), strings.TrimSpace(msg.Content))
		}
	}
	return b.String()
}

func renderFeedback(items []reflectpkg.Experience) string {
	var b strings.Builder
	b.WriteString("# Feedback / 反思经验导出\n\n")
	if len(items) == 0 {
		b.WriteString("未发现可导出的 feedback。\n")
		return b.String()
	}
	for _, e := range items {
		title := firstNonEmpty(e.SourceID, e.ID, e.Category, "feedback")
		fmt.Fprintf(&b, "## %s\n\n", mdEscape(title))
		meta := compactMeta(map[string]string{"id": e.ID, "source": e.Source, "category": e.Category, "outcome": e.Outcome, "created": formatTime(e.CreatedAt)})
		if meta != "" {
			fmt.Fprintf(&b, "_%s_\n\n", meta)
		}
		if len(e.Tags) > 0 {
			fmt.Fprintf(&b, "标签：`%s`\n\n", strings.Join(e.Tags, "`, `"))
		}
		if strings.TrimSpace(e.Lesson) != "" {
			fmt.Fprintf(&b, "**经验**\n\n%s\n\n", strings.TrimSpace(e.Lesson))
		}
		if strings.TrimSpace(e.Context) != "" {
			fmt.Fprintf(&b, "**上下文**\n\n%s\n\n", strings.TrimSpace(e.Context))
		}
	}
	return b.String()
}

func writeFile(root, rel string, data []byte) error {
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
func fileExists(path string) bool { info, err := os.Stat(path); return err == nil && !info.IsDir() }
func dirExists(path string) bool  { info, err := os.Stat(path); return err == nil && info.IsDir() }
func strAny(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}
func firstStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := strings.TrimSpace(strAny(m[k])); s != "" {
			return s
		}
	}
	return ""
}
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
func hasAny(m map[string]any, keys ...string) bool {
	for _, k := range keys {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}
func parseTimeAny(v any) time.Time {
	s := strAny(v)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
func compactMeta(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k, v := range m {
		if strings.TrimSpace(v) != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+": "+m[k])
	}
	return strings.Join(parts, " · ")
}
func mdEscape(s string) string { return strings.ReplaceAll(s, "\n", " ") }
