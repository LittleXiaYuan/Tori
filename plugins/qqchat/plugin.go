package qqchat

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"yunque-agent/pkg/plugin"
	"yunque-agent/pkg/skills"
)

// LLMFunc is a function that calls the LLM.
type LLMFunc func(ctx context.Context, system, user string) (string, error)

// ChatRecord is a single message from a QQ chat export.
type ChatRecord struct {
	Timestamp time.Time `json:"timestamp"`
	Sender    string    `json:"sender"`
	Content   string    `json:"content"`
}

// AnalysisResult holds the LLM analysis of a chat history.
type AnalysisResult struct {
	ID             string            `json:"id"`
	FileName       string            `json:"file_name"`
	TotalMessages  int               `json:"total_messages"`
	Participants   []string          `json:"participants"`
	TimeRange      string            `json:"time_range"`
	Summary        string            `json:"summary"`
	PersonaProfile map[string]string `json:"persona_profiles"` // participant -> personality summary
	TopTopics      []string          `json:"top_topics"`
	Sentiment      string            `json:"sentiment"`
	AnalyzedAt     time.Time         `json:"analyzed_at"`
}

// Plugin is the QQ Chat Analyzer plugin — a full-stack UIPlugin.
type Plugin struct {
	dataDir  string
	llmCall  LLMFunc
	mu       sync.RWMutex
	analyses map[string]*AnalysisResult // id -> result
	records  map[string][]ChatRecord    // id -> parsed messages
}

// New creates a new QQ Chat Analyzer plugin.
func New(dataDir string, llmCall LLMFunc) *Plugin {
	dir := filepath.Join(dataDir, "qqchat")
	os.MkdirAll(dir, 0o755)
	return &Plugin{
		dataDir:  dir,
		llmCall:  llmCall,
		analyses: make(map[string]*AnalysisResult),
		records:  make(map[string][]ChatRecord),
	}
}

func (p *Plugin) Name() string        { return "qqchat" }
func (p *Plugin) Description() string { return "QQ聊天记录分析与角色扮演插件" }
func (p *Plugin) SystemPrompt() string {
	return `你具备QQ聊天记录分析能力：可以解析QQ导出的聊天记录，分析对话模式、情感倾向、话题分布，并能模拟参与者的说话风格进行角色扮演。`
}

func (p *Plugin) Skills() []skills.Skill {
	return []skills.Skill{
		&analyzeSkill{plugin: p},
		&roleplaySkill{plugin: p},
	}
}

// UITabs implements plugin.UIPlugin.
func (p *Plugin) UITabs() []plugin.UITab {
	return []plugin.UITab{
		{
			Key:         "qqchat",
			Label:       "QQ聊天分析",
			LabelEn:     "QQ Chat Analyzer",
			Icon:        "MessageSquareText",
			Description: "导入QQ聊天记录，AI分析对话模式并角色扮演",
		},
	}
}

// HTTPHandlers implements plugin.UIPlugin.
func (p *Plugin) HTTPHandlers() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"/upload":   p.handleUpload,
		"/analyses": p.handleAnalyses,
		"/analysis": p.handleAnalysis,
		"/roleplay": p.handleRoleplay,
		"/delete":   p.handleDelete,
	}
}

// ── HTTP Handlers ──

// handleUpload accepts a QQ chat export file (text), parses it, and runs LLM analysis.
// POST /v1/ext/qqchat/upload   body: multipart/form-data with "file" field, or JSON {"content":"...","filename":"..."}
func (p *Plugin) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var content string
	var filename string

	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "multipart/form-data") {
		// Multipart file upload
		r.ParseMultipartForm(50 * 1024 * 1024) // 50MB max
		file, header, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no file provided"})
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, 50*1024*1024))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "read file: " + err.Error()})
			return
		}
		content = string(data)
		filename = header.Filename
	} else {
		// JSON body
		var req struct {
			Content  string `json:"content"`
			Filename string `json:"filename"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 50*1024*1024)).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
			return
		}
		content = req.Content
		filename = req.Filename
	}

	if content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "empty content"})
		return
	}
	if filename == "" {
		filename = "chat.txt"
	}

	// Parse QQ chat records
	records := ParseQQChat(content)
	if len(records) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no chat records found — ensure this is a QQ chat export"})
		return
	}

	// Generate ID
	id := fmt.Sprintf("qq_%d", time.Now().UnixMilli())

	// Store parsed records
	p.mu.Lock()
	p.records[id] = records
	p.mu.Unlock()

	// Run LLM analysis
	analysis, err := p.analyze(r.Context(), id, filename, records)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "analysis failed: " + err.Error()})
		return
	}

	// Persist
	p.mu.Lock()
	p.analyses[id] = analysis
	p.mu.Unlock()
	p.save(id)

	writeJSON(w, http.StatusOK, analysis)
}

// handleAnalyses returns all analysis results.
// GET /v1/ext/qqchat/analyses
func (p *Plugin) handleAnalyses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}

	p.mu.RLock()
	results := make([]*AnalysisResult, 0, len(p.analyses))
	for _, a := range p.analyses {
		results = append(results, a)
	}
	p.mu.RUnlock()

	// Sort by analyzed_at desc
	sort.Slice(results, func(i, j int) bool {
		return results[i].AnalyzedAt.After(results[j].AnalyzedAt)
	})

	writeJSON(w, http.StatusOK, map[string]any{"analyses": results})
}

// handleAnalysis returns a single analysis.
// GET /v1/ext/qqchat/analysis?id=xxx
func (p *Plugin) handleAnalysis(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	p.mu.RLock()
	a, ok := p.analyses[id]
	p.mu.RUnlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	writeJSON(w, http.StatusOK, a)
}

// handleRoleplay sends a roleplay request — ask the LLM to impersonate someone from the chat.
// POST /v1/ext/qqchat/roleplay  body: {"id":"qq_xxx","persona":"小明","message":"你好啊"}
func (p *Plugin) handleRoleplay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID      string `json:"id"`
		Persona string `json:"persona"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	p.mu.RLock()
	analysis, aOK := p.analyses[req.ID]
	records, rOK := p.records[req.ID]
	p.mu.RUnlock()

	if !aOK || !rOK {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "analysis not found — upload a chat first"})
		return
	}

	// Build persona context from chat records
	profile := analysis.PersonaProfile[req.Persona]
	if profile == "" {
		profile = "未知角色"
	}

	// Get sample messages from this person (max 30)
	var samples []string
	for _, rec := range records {
		if rec.Sender == req.Persona {
			samples = append(samples, rec.Content)
			if len(samples) >= 30 {
				break
			}
		}
	}

	systemPrompt := fmt.Sprintf(`你现在要扮演一个叫"%s"的人。以下是关于这个人的性格分析：
%s

以下是这个人的真实聊天记录样本（学习其说话风格）：
%s

请完全模仿这个人的说话方式、用词习惯、语气和性格来回复。不要提到你是AI。`, req.Persona, profile, strings.Join(samples, "\n"))

	reply, err := p.llmCall(r.Context(), systemPrompt, req.Message)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "LLM error: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"persona": req.Persona,
		"reply":   strings.TrimSpace(reply),
	})
}

// handleDelete removes an analysis.
// DELETE /v1/ext/qqchat/delete?id=xxx
func (p *Plugin) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "DELETE required", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	p.mu.Lock()
	delete(p.analyses, id)
	delete(p.records, id)
	p.mu.Unlock()

	// Remove persisted file
	os.Remove(filepath.Join(p.dataDir, id+".json"))

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ── Analysis Logic ──

func (p *Plugin) analyze(ctx context.Context, id, filename string, records []ChatRecord) (*AnalysisResult, error) {
	// Gather participants
	participantSet := map[string]int{}
	for _, r := range records {
		participantSet[r.Sender]++
	}
	participants := make([]string, 0, len(participantSet))
	for name := range participantSet {
		participants = append(participants, name)
	}
	sort.Strings(participants)

	// Time range
	var timeRange string
	if len(records) > 0 {
		first := records[0].Timestamp.Format("2006-01-02")
		last := records[len(records)-1].Timestamp.Format("2006-01-02")
		timeRange = first + " ~ " + last
	}

	// Build chat excerpt for LLM (max 200 messages to avoid context overflow)
	maxMsg := 200
	if len(records) < maxMsg {
		maxMsg = len(records)
	}
	var excerpt strings.Builder
	for i := 0; i < maxMsg; i++ {
		r := records[i]
		excerpt.WriteString(fmt.Sprintf("[%s] %s: %s\n", r.Timestamp.Format("01-02 15:04"), r.Sender, r.Content))
	}

	participantList := strings.Join(participants, "、")
	systemPrompt := `你是一个聊天记录分析专家。请分析以下QQ聊天记录，返回JSON格式的分析结果。

返回格式（仅JSON，无代码块包装）：
{
  "summary": "对话整体总结（200字以内）",
  "persona_profiles": {"参与者名": "性格特征、说话风格、兴趣爱好分析（100字以内）"},
  "top_topics": ["话题1", "话题2", "话题3"],
  "sentiment": "整体情感基调（如：轻松愉快/严肃认真/混合等）"
}`

	userPrompt := fmt.Sprintf("聊天参与者：%s\n共 %d 条消息，时间范围：%s\n\n聊天记录：\n%s",
		participantList, len(records), timeRange, excerpt.String())

	resp, err := p.llmCall(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	// Parse LLM response
	resp = strings.TrimSpace(resp)
	resp = trimCodeFences(resp)

	var parsed struct {
		Summary        string            `json:"summary"`
		PersonaProfile map[string]string `json:"persona_profiles"`
		TopTopics      []string          `json:"top_topics"`
		Sentiment      string            `json:"sentiment"`
	}
	if err := json.Unmarshal([]byte(resp), &parsed); err != nil {
		slog.Warn("qqchat: LLM response not valid JSON, using raw", "err", err)
		parsed.Summary = resp
	}

	return &AnalysisResult{
		ID:             id,
		FileName:       filename,
		TotalMessages:  len(records),
		Participants:   participants,
		TimeRange:      timeRange,
		Summary:        parsed.Summary,
		PersonaProfile: parsed.PersonaProfile,
		TopTopics:      parsed.TopTopics,
		Sentiment:      parsed.Sentiment,
		AnalyzedAt:     time.Now(),
	}, nil
}

// ── QQ Chat Parser ──

// QQ chat export format (common patterns):
//
//	2024-01-15 14:32:05 用户名
//	消息内容
//
//	2024-01-15 14:32:05 用户名(12345678)
//	消息内容
var qqHeaderRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}\s+\d{1,2}:\d{2}:\d{2})\s+(.+?)(?:\(\d+\))?\s*$`)

// ParseQQChat parses a QQ chat export text into structured records.
func ParseQQChat(content string) []ChatRecord {
	scanner := bufio.NewScanner(strings.NewReader(content))
	var records []ChatRecord
	var current *ChatRecord
	var contentBuf strings.Builder

	flush := func() {
		if current != nil && contentBuf.Len() > 0 {
			current.Content = strings.TrimSpace(contentBuf.String())
			if current.Content != "" {
				records = append(records, *current)
			}
		}
		current = nil
		contentBuf.Reset()
	}

	for scanner.Scan() {
		line := scanner.Text()

		if m := qqHeaderRe.FindStringSubmatch(line); m != nil {
			flush()
			ts, err := time.Parse("2006-01-02 15:04:05", m[1])
			if err != nil {
				continue
			}
			current = &ChatRecord{
				Timestamp: ts,
				Sender:    strings.TrimSpace(m[2]),
			}
			continue
		}

		if current != nil {
			if contentBuf.Len() > 0 {
				contentBuf.WriteString("\n")
			}
			contentBuf.WriteString(line)
		}
	}
	flush()

	return records
}

// ── Skills ──

type analyzeSkill struct{ plugin *Plugin }

func (s *analyzeSkill) Name() string { return "qq_chat_analyze" }
func (s *analyzeSkill) Description() string {
	return "分析QQ聊天记录文件，提取对话模式、参与者性格、话题和情感"
}
func (s *analyzeSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "QQ聊天记录文件路径",
			},
		},
		"required": []string{"file_path"},
	}
}
func (s *analyzeSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	filePath, _ := args["file_path"].(string)
	if filePath == "" {
		return "", fmt.Errorf("file_path is required")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	records := ParseQQChat(string(data))
	if len(records) == 0 {
		return "未找到有效的QQ聊天记录", nil
	}

	id := fmt.Sprintf("qq_%d", time.Now().UnixMilli())
	s.plugin.mu.Lock()
	s.plugin.records[id] = records
	s.plugin.mu.Unlock()

	analysis, err := s.plugin.analyze(ctx, id, filepath.Base(filePath), records)
	if err != nil {
		return "", err
	}

	s.plugin.mu.Lock()
	s.plugin.analyses[id] = analysis
	s.plugin.mu.Unlock()
	s.plugin.save(id)

	result, _ := json.MarshalIndent(analysis, "", "  ")
	return string(result), nil
}

type roleplaySkill struct{ plugin *Plugin }

func (s *roleplaySkill) Name() string { return "qq_chat_roleplay" }
func (s *roleplaySkill) Description() string {
	return "根据QQ聊天记录中的角色风格进行角色扮演对话"
}
func (s *roleplaySkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"analysis_id": map[string]any{
				"type":        "string",
				"description": "分析记录ID（来自qq_chat_analyze的结果）",
			},
			"persona": map[string]any{
				"type":        "string",
				"description": "要扮演的角色名称（聊天参与者之一）",
			},
			"message": map[string]any{
				"type":        "string",
				"description": "要发送给该角色的消息",
			},
		},
		"required": []string{"analysis_id", "persona", "message"},
	}
}
func (s *roleplaySkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	id, _ := args["analysis_id"].(string)
	persona, _ := args["persona"].(string)
	message, _ := args["message"].(string)

	s.plugin.mu.RLock()
	analysis, aOK := s.plugin.analyses[id]
	records, rOK := s.plugin.records[id]
	s.plugin.mu.RUnlock()

	if !aOK || !rOK {
		return "", fmt.Errorf("analysis %s not found", id)
	}

	profile := analysis.PersonaProfile[persona]
	if profile == "" {
		profile = "未知角色"
	}

	var samples []string
	for _, rec := range records {
		if rec.Sender == persona {
			samples = append(samples, rec.Content)
			if len(samples) >= 30 {
				break
			}
		}
	}

	systemPrompt := fmt.Sprintf(`你现在要扮演一个叫"%s"的人。性格分析：%s

真实聊天样本：
%s

完全模仿这个人的说话方式回复。不要提到你是AI。`, persona, profile, strings.Join(samples, "\n"))

	reply, err := s.plugin.llmCall(ctx, systemPrompt, message)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(reply), nil
}

// ── Persistence ──

func (p *Plugin) save(id string) {
	p.mu.RLock()
	a, ok := p.analyses[id]
	p.mu.RUnlock()
	if !ok {
		return
	}

	data, _ := json.MarshalIndent(a, "", "  ")
	path := filepath.Join(p.dataDir, id+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		slog.Error("qqchat: save analysis", "id", id, "err", err)
	}
}

// LoadPersisted loads previously saved analyses from disk.
func (p *Plugin) LoadPersisted() {
	entries, err := os.ReadDir(p.dataDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(p.dataDir, e.Name()))
		if err != nil {
			continue
		}
		var a AnalysisResult
		if err := json.Unmarshal(data, &a); err != nil {
			continue
		}
		p.mu.Lock()
		p.analyses[a.ID] = &a
		p.mu.Unlock()
	}
	slog.Info("qqchat: loaded persisted analyses", "count", len(p.analyses))
}

// ── Helpers ──

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func trimCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}
	if strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}
