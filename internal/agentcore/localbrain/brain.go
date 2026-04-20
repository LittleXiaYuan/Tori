package localbrain

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/pkg/jsonutil"
)

// LocalBrain 是本地小模型决策层。
// 职责：意图分类、安全预判、简单推理——不消耗 API token。
// 复杂任务自动升级到云端大模型。
//
// 设计哲学（来自林俊旸 Agentic Thinking）：
//   - 小模型负责快速、低延迟的"直觉判断"
//   - 大模型负责深度推理和复杂工具编排
//   - Ledger 轨迹数据用于周期性 LoRA 微调小模型
type LocalBrain struct {
	mu sync.RWMutex

	client     *llm.Client // 本地小模型 client（Ollama/vLLM/etc）
	pool       *llm.Pool   // LLM Pool 引用（用于升级到大模型）
	thresholds Thresholds

	// 用户行为学习缓存（LoRA 微调前的在线适应）
	userPatterns map[string]*UserPattern // tenantID → pattern

	stats BrainStats
}

// Thresholds 控制小模型何时自行处理、何时升级到大模型。
type Thresholds struct {
	// 意图分类置信度低于此值时升级到大模型
	ClassifyConfidence float64 `json:"classify_confidence"`
	// 回答生成置信度低于此值时升级
	AnswerConfidence float64 `json:"answer_confidence"`
	// 查询长度超过此值时直接升级（小模型上下文窗口有限）
	MaxQueryLength int `json:"max_query_length"`
	// 工具调用超过此数量时升级（多步编排交给大模型）
	MaxToolSteps int `json:"max_tool_steps"`
}

// DefaultThresholds 返回经验默认值。
func DefaultThresholds() Thresholds {
	return Thresholds{
		ClassifyConfidence: 0.7,
		AnswerConfidence:   0.6,
		MaxQueryLength:     500,
		MaxToolSteps:       3,
	}
}

// UserPattern 记录单个用户的行为模式（用于个性化决策）。
type UserPattern struct {
	TenantID     string
	QueryHistory []QueryRecord // 最近 N 条查询（滑动窗口）
	Preferences  map[string]float64
	LastUpdated  time.Time
}

// QueryRecord 记录单次查询的分类结果和用户反馈。
type QueryRecord struct {
	Query     string    `json:"query"`
	Intent    Intent    `json:"intent"`
	Tier      string    `json:"tier"`       // fast/smart/expert
	Upgraded  bool      `json:"upgraded"`   // 是否被升级到大模型
	Satisfied bool      `json:"satisfied"`  // 用户是否满意（隐式反馈）
	Timestamp time.Time `json:"timestamp"`
}

// BrainStats 统计本地决策层表现。
type BrainStats struct {
	mu              sync.Mutex
	TotalClassify   int64   `json:"total_classify"`
	LocalHandled    int64   `json:"local_handled"`    // 小模型独立处理
	UpgradedToCloud int64   `json:"upgraded_to_cloud"` // 升级到大模型
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
}

// New 创建本地决策层。
func New(localClient *llm.Client, pool *llm.Pool, opts ...Option) *LocalBrain {
	b := &LocalBrain{
		client:       localClient,
		pool:         pool,
		thresholds:   DefaultThresholds(),
		userPatterns: make(map[string]*UserPattern),
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Option 配置选项。
type Option func(*LocalBrain)

func WithThresholds(t Thresholds) Option {
	return func(b *LocalBrain) { b.thresholds = t }
}

// Intent 意图分类结果。
type Intent struct {
	Category   string  `json:"category"`   // chat/code/search/tool/complex
	Complexity string  `json:"complexity"` // simple/medium/hard
	Confidence float64 `json:"confidence"` // 0.0 ~ 1.0
	NeedTools  bool    `json:"need_tools"`
}

// Decision 决策结果。
type Decision struct {
	// 由谁处理
	Handler string `json:"handler"` // "local" | "fast" | "smart" | "expert"
	// 意图分类
	Intent Intent `json:"intent"`
	// 理由（自然语言）
	Reason string `json:"reason"`
	// 如果 Handler == "local"，这里是小模型的直接回答
	LocalReply string `json:"local_reply,omitempty"`
}

// Classify 用本地小模型对用户查询做意图分类。
// 这是最核心的方法：决定一条查询应该走小模型还是大模型。
func (b *LocalBrain) Classify(ctx context.Context, query, tenantID string) (*Decision, error) {
	start := time.Now()
	defer func() {
		b.stats.mu.Lock()
		b.stats.TotalClassify++
		lat := float64(time.Since(start).Milliseconds())
		b.stats.AvgLatencyMs = (b.stats.AvgLatencyMs*float64(b.stats.TotalClassify-1) + lat) / float64(b.stats.TotalClassify)
		b.stats.mu.Unlock()
	}()

	// 快速路径：简短问候/闲聊，交给fast tier直接处理，不需要工具
	if isGreeting(query) {
		b.stats.mu.Lock()
		b.stats.LocalHandled++
		b.stats.mu.Unlock()
		return &Decision{
			Handler: "fast",
			Intent:  Intent{Category: "chat", Complexity: "simple", Confidence: 1.0, NeedTools: false},
			Reason:  "simple greeting, no tools needed",
		}, nil
	}

	// 快速路径：查询太长直接升级
	if len(query) > b.thresholds.MaxQueryLength {
		return &Decision{
			Handler: "smart",
			Intent:  Intent{Category: "complex", Complexity: "hard", Confidence: 0.9},
			Reason:  "query too long for local model",
		}, nil
	}

	// 用小模型分类
	intent, err := b.classifyWithLocal(ctx, query, tenantID)
	if err != nil {
		slog.Warn("localbrain: classify failed, upgrading", "err", err)
		return &Decision{
			Handler: "smart",
			Intent:  Intent{Category: "unknown", Complexity: "medium", Confidence: 0.0},
			Reason:  fmt.Sprintf("local classify failed: %v", err),
		}, nil
	}

	// 置信度不足 → 升级
	if intent.Confidence < b.thresholds.ClassifyConfidence {
		b.stats.mu.Lock()
		b.stats.UpgradedToCloud++
		b.stats.mu.Unlock()
		return &Decision{
			Handler: b.selectCloudTier(intent),
			Intent:  *intent,
			Reason:  "low confidence, upgrading to cloud",
		}, nil
	}

	// 需要工具 → 升级（小模型 function calling 不稳定）
	if intent.NeedTools {
		b.stats.mu.Lock()
		b.stats.UpgradedToCloud++
		b.stats.mu.Unlock()
		return &Decision{
			Handler: b.selectCloudTier(intent),
			Intent:  *intent,
			Reason:  "tools required, upgrading",
		}, nil
	}

	// 简单查询 → 小模型直接回答
	if intent.Complexity == "simple" {
		b.stats.mu.Lock()
		b.stats.LocalHandled++
		b.stats.mu.Unlock()
		return &Decision{
			Handler: "local",
			Intent:  *intent,
			Reason:  "simple query, handling locally",
		}, nil
	}

	// 中等复杂度 → 根据用户历史决定
	if pattern := b.getUserPattern(tenantID); pattern != nil {
		if b.userPrefersFast(pattern, intent) {
			b.stats.mu.Lock()
			b.stats.LocalHandled++
			b.stats.mu.Unlock()
			return &Decision{
				Handler: "local",
				Intent:  *intent,
				Reason:  "user pattern suggests local can handle",
			}, nil
		}
	}

	// 默认升级到 smart tier
	b.stats.mu.Lock()
	b.stats.UpgradedToCloud++
	b.stats.mu.Unlock()
	return &Decision{
		Handler: "smart",
		Intent:  *intent,
		Reason:  "medium complexity, routing to cloud",
	}, nil
}

// classifyWithLocal 调用本地小模型做意图分类。
func (b *LocalBrain) classifyWithLocal(ctx context.Context, query, tenantID string) (*Intent, error) {
	if b.client == nil {
		return nil, fmt.Errorf("no local model configured")
	}

	// 构建分类 prompt（极度精简，适合小模型）
	sysPrompt := `You are an intent classifier. Output ONLY valid JSON.
{"category":"chat|code|search|tool|complex","complexity":"simple|medium|hard","confidence":0.0-1.0,"need_tools":true|false}
Rules:
- "chat": greetings, chitchat, simple Q&A
- "code": coding, debugging, code review
- "search": web search, document lookup
- "tool": file ops, shell commands, API calls
- "complex": multi-step reasoning, planning, analysis`

	msgs := []llm.Message{
		{Role: "system", Content: sysPrompt},
		{Role: "user", Content: query},
	}

	// 低温度，确定性输出
	reply, err := b.client.Chat(ctx, msgs, 0.1)
	if err != nil {
		return nil, err
	}

	intent := &Intent{}
	// 提取 JSON（小模型可能输出额外文本）
	if err := jsonutil.Unmarshal(reply, intent); err != nil {
		return nil, fmt.Errorf("parse intent: %w (raw: %s)", err, truncate(reply, 200))
	}

	return intent, nil
}

// selectCloudTier 根据意图决定升级到哪个云端模型层级。
func (b *LocalBrain) selectCloudTier(intent *Intent) string {
	switch intent.Complexity {
	case "hard":
		return "expert"
	case "medium":
		return "smart"
	default:
		return "fast"
	}
}

// RecordFeedback 记录用户隐式反馈（用于未来 LoRA 微调数据）。
func (b *LocalBrain) RecordFeedback(tenantID, query string, intent Intent, tier string, upgraded, satisfied bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	pattern, ok := b.userPatterns[tenantID]
	if !ok {
		pattern = &UserPattern{
			TenantID:    tenantID,
			Preferences: make(map[string]float64),
		}
		b.userPatterns[tenantID] = pattern
	}

	record := QueryRecord{
		Query:     truncate(query, 200),
		Intent:    intent,
		Tier:      tier,
		Upgraded:  upgraded,
		Satisfied: satisfied,
		Timestamp: time.Now(),
	}
	pattern.QueryHistory = append(pattern.QueryHistory, record)
	pattern.LastUpdated = time.Now()

	// 滑动窗口：只保留最近 200 条
	if len(pattern.QueryHistory) > 200 {
		pattern.QueryHistory = pattern.QueryHistory[len(pattern.QueryHistory)-200:]
	}

	// 更新偏好权重
	key := intent.Category + ":" + intent.Complexity
	if satisfied && !upgraded {
		pattern.Preferences[key] += 0.1 // 小模型成功处理 → 增强信任
	} else if !satisfied && !upgraded {
		pattern.Preferences[key] -= 0.2 // 小模型处理失败 → 降低信任
	}
}

// getUserPattern 获取用户模式（只读）。
func (b *LocalBrain) getUserPattern(tenantID string) *UserPattern {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.userPatterns[tenantID]
}

// userPrefersFast 根据用户历史判断是否可以用小模型处理。
func (b *LocalBrain) userPrefersFast(pattern *UserPattern, intent *Intent) bool {
	key := intent.Category + ":" + intent.Complexity
	score, ok := pattern.Preferences[key]
	if !ok {
		return false
	}
	return score > 0.3 // 积累了足够正面反馈
}

// Stats 返回统计信息。
func (b *LocalBrain) Stats() BrainStats {
	b.stats.mu.Lock()
	defer b.stats.mu.Unlock()
	return BrainStats{
		TotalClassify:   b.stats.TotalClassify,
		LocalHandled:    b.stats.LocalHandled,
		UpgradedToCloud: b.stats.UpgradedToCloud,
		AvgLatencyMs:    b.stats.AvgLatencyMs,
	}
}

// ExportTrainingData 导出 LoRA 微调训练数据。
// 返回 (query, intent, feedback) 三元组，用于离线微调。
func (b *LocalBrain) ExportTrainingData(tenantID string) []TrainingSample {
	b.mu.RLock()
	defer b.mu.RUnlock()

	pattern, ok := b.userPatterns[tenantID]
	if !ok {
		return nil
	}

	var samples []TrainingSample
	for _, r := range pattern.QueryHistory {
		if r.Satisfied { // 只用正面反馈数据训练
			samples = append(samples, TrainingSample{
				Input:    r.Query,
				Intent:   r.Intent,
				Tier:     r.Tier,
				Upgraded: r.Upgraded,
			})
		}
	}
	return samples
}

// TrainingSample 是 LoRA 微调的单条训练样本。
type TrainingSample struct {
	Input    string `json:"input"`
	Intent   Intent `json:"intent"`
	Tier     string `json:"tier"`
	Upgraded bool   `json:"upgraded"`
}

// ContextItem 是上下文筛选的输入/输出单元。
type ContextItem struct {
	Source     string  `json:"source"`     // "memory", "graph", "knowledge", etc.
	Content    string  `json:"content"`
	Importance int     `json:"importance"` // 0=low, 1=medium, 2=high (from memory layer)
	Score      float64 `json:"score"`      // relevance score assigned by FilterContext
}

// FilterResult 是上下文筛选的输出。
type FilterResult struct {
	Items    []ContextItem `json:"items"`
	Summary  string        `json:"summary"`
	Filtered int           `json:"filtered"` // how many items were removed
	Elapsed  time.Duration `json:"-"`
}

// FilterContext 用小模型筛选和精炼上下文，减少注入大模型的信息噪声。
//
// 分级信任设计（参考认知分层蓝图）：
//   - L0: 相关度打分（0.0-1.0）—— 几乎无幻觉风险
//   - L1: 精炼摘要（保留核心语义，压缩表达）—— 低风险
//   - 高重要性条目走旁路，不经过筛选，避免遗漏关键信息
//
// 返回的 FilterResult.Items 按相关度降序排列，调用方只需取 top-K。
func (b *LocalBrain) FilterContext(ctx context.Context, query string, items []ContextItem, maxItems int) (*FilterResult, error) {
	start := time.Now()

	if b.client == nil {
		return b.filterByRules(items, maxItems), nil
	}

	if len(items) == 0 {
		return &FilterResult{Elapsed: time.Since(start)}, nil
	}

	if maxItems <= 0 {
		maxItems = 8
	}

	if len(items) <= maxItems {
		return &FilterResult{
			Items:   items,
			Elapsed: time.Since(start),
		}, nil
	}

	// 旁路：高重要性条目直接保留，不经过小模型判断
	var bypass []ContextItem
	var candidates []ContextItem
	for _, item := range items {
		if item.Importance >= 2 {
			bypass = append(bypass, item)
		} else {
			candidates = append(candidates, item)
		}
	}

	// 如果旁路已经满足 maxItems，直接返回
	if len(bypass) >= maxItems {
		bypass = bypass[:maxItems]
		return &FilterResult{
			Items:    bypass,
			Filtered: len(items) - len(bypass),
			Elapsed:  time.Since(start),
		}, nil
	}

	// 剩余配额给小模型筛选
	remaining := maxItems - len(bypass)
	if remaining > len(candidates) {
		remaining = len(candidates)
	}

	scored, err := b.scoreRelevance(ctx, query, candidates)
	if err != nil {
		slog.Warn("localbrain: filter scoring failed, falling back to rules", "err", err)
		return b.filterByRules(items, maxItems), nil
	}

	// 取 top-remaining 个
	if len(scored) > remaining {
		scored = scored[:remaining]
	}

	result := append(bypass, scored...)
	return &FilterResult{
		Items:    result,
		Filtered: len(items) - len(result),
		Elapsed:  time.Since(start),
	}, nil
}

// scoreRelevance 让小模型对每个候选条目打相关度分，返回按分数降序排列的结果。
func (b *LocalBrain) scoreRelevance(ctx context.Context, query string, items []ContextItem) ([]ContextItem, error) {
	if len(items) == 0 {
		return nil, nil
	}

	var sb strings.Builder
	for i, item := range items {
		content := truncate(item.Content, 200)
		sb.WriteString(fmt.Sprintf("[%d] (%s) %s\n", i, item.Source, content))
	}

	sysPrompt := `Score each item's relevance to the user query. Output ONLY a JSON array of numbers (0.0-1.0).
Example: [0.9, 0.2, 0.7]
Rules:
- 1.0 = directly answers the query
- 0.7 = provides useful context
- 0.3 = tangentially related
- 0.0 = completely irrelevant`

	userPrompt := fmt.Sprintf("Query: %s\n\nItems:\n%s", truncate(query, 300), sb.String())

	msgs := []llm.Message{
		{Role: "system", Content: sysPrompt},
		{Role: "user", Content: userPrompt},
	}

	scoreCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	reply, err := b.client.Chat(scoreCtx, msgs, 0.1)
	if err != nil {
		return nil, err
	}

	// 解析分数数组
	var scores []float64
	if err := jsonutil.Unmarshal(reply, &scores); err != nil {
		return nil, fmt.Errorf("parse scores: %w (raw: %s)", err, truncate(reply, 200))
	}

	// 校验层：分数数量必须匹配，分数必须在 [0, 1] 范围内
	if len(scores) != len(items) {
		return nil, fmt.Errorf("score count mismatch: got %d, want %d", len(scores), len(items))
	}
	for i := range scores {
		if scores[i] < 0 {
			scores[i] = 0
		}
		if scores[i] > 1 {
			scores[i] = 1
		}
	}

	// 赋分并排序
	result := make([]ContextItem, len(items))
	copy(result, items)
	for i := range result {
		result[i].Score = scores[i]
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})

	return result, nil
}

// isGreeting detects trivial greetings that don't need tool calls or heavy processing.
func isGreeting(query string) bool {
	q := strings.TrimSpace(query)
	if len([]rune(q)) > 20 {
		return false
	}
	lower := strings.ToLower(q)
	stripped := strings.NewReplacer(
		"~", "", "～", "", "！", "", "!", "",
		"。", "", ".", "", "？", "", "?", "",
		"呀", "", "啊", "", "哦", "", "呢", "",
		"哈", "", "嘛", "", "吖", "", "鸭", "",
	).Replace(lower)

	greetings := []string{
		"你好", "您好", "嗨", "hi", "hello", "hey", "早上好", "下午好",
		"晚上好", "早安", "晚安", "在吗", "在不在", "yo", "嘿",
	}
	for _, g := range greetings {
		if stripped == g || strings.HasPrefix(stripped, g) || strings.Contains(lower, g) {
			return true
		}
	}
	return false
}

// filterByRules 是无小模型时的纯规则降级方案（基于关键词匹配和重要性）。
func (b *LocalBrain) filterByRules(items []ContextItem, maxItems int) *FilterResult {
	if maxItems <= 0 {
		maxItems = 8
	}

	sorted := make([]ContextItem, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Importance > sorted[j].Importance
	})

	if len(sorted) > maxItems {
		sorted = sorted[:maxItems]
	}

	return &FilterResult{
		Items:    sorted,
		Filtered: len(items) - len(sorted),
	}
}
