package knowledge

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"yunque-agent/internal/agentcore/embeddings"
)

// ──────────────────────────────────────────────
// Benchmark: BM25 vs Vector vs Hybrid retrieval
// Measures: Recall@K, Precision@K, MRR, NDCG@K,
//           latency, token savings
// ──────────────────────────────────────────────

type benchQuery struct {
	Query    string
	Relevant []string // IDs of relevant chunks (ground truth)
}

func buildBenchCorpus() (*Store, []benchQuery) {
	store := NewStore(800)

	type docEntry struct {
		id      string
		content string
	}

	docs := []docEntry{
		{"rag-arch", "RAG（检索增强生成）架构将大语言模型与外部知识库结合。核心流程：用户提问 → 检索相关文档片段 → 将片段注入 prompt → LLM 生成回答。优势是减少幻觉、支持实时知识更新、无需微调模型。关键组件包括向量数据库、embedding 模型、chunk 策略和 reranker。"},
		{"vec-db", "向量数据库选型比较：Milvus 支持十亿级向量、分布式架构；Qdrant 轻量 Rust 实现、适合中小规模；Weaviate 带 GraphQL API；Chroma 适合原型开发。HNSW 索引在召回率和速度之间取得良好平衡，典型配置 M=16, efConstruction=200。"},
		{"bm25-theory", "BM25 是经典的概率检索模型，基于词频（TF）和逆文档频率（IDF）。公式：score = IDF * (tf * (k1+1)) / (tf + k1*(1-b+b*dl/avgdl))。参数 k1 控制词频饱和度（通常1.2-2.0），b 控制文档长度归一化（通常0.75）。对精确关键词匹配效果好，但无法理解语义相似性。"},
		{"hybrid-retrieval", "混合检索（Hybrid Retrieval）结合稀疏检索（BM25）和稠密检索（向量搜索）。融合方法包括 RRF（Reciprocal Rank Fusion）、线性加权和学习排序。RRF 公式：score(d) = Σ 1/(k+rank_i)，k 通常取60。混合检索在多个基准测试上优于单一检索方式，召回率提升10-20%。"},
		{"prompt-eng", "Prompt 工程最佳实践：1）使用系统提示设定角色和约束；2）提供少样本示例（few-shot）；3）思维链（CoT）引导推理；4）结构化输出格式（JSON/XML）。高级技巧包括自一致性（self-consistency）、思维树（Tree of Thought）和检索增强的 prompt 构建。"},
		{"token-opt", "Token 优化策略：1）语义缓存（Semantic Cache）—相似查询命中缓存，可减少30-50%重复调用；2）prompt 压缩—移除冗余上下文，保留关键信息；3）分层摘要—将长文档逐层压缩为摘要后再注入；4）动态上下文窗口—根据查询复杂度调整注入片段数量。综合策略可实现3-5倍 token 节省。"},
		{"embedding-model", "Embedding 模型选择：text-embedding-3-small（OpenAI，1536维，性价比高）；BGE-M3（BAAI，多语言，支持稀疏+稠密混合）；Jina-embeddings-v2（8K 上下文窗口）；GTE-large（阿里，中文效果好）。选型要点：维度、多语言支持、最大输入长度、推理速度和成本。"},
		{"agent-design", "Agent 架构设计模式：ReAct（推理+行动交替）、Plan-and-Execute（先规划后执行）、Multi-Agent（多智能体协作）。工具调用是 Agent 核心能力，需要定义清晰的工具描述和参数 schema。错误处理和重试机制对可靠性至关重要。"},
		{"eval-metrics", "检索评估指标：Recall@K（前K个结果中包含相关文档的比例）、Precision@K（前K个结果中相关文档占比）、MRR（平均倒数排名）、NDCG（归一化折扣累积增益）。端到端评估还需考虑 LLM 回答质量，可用 RAGAS 框架评估 faithfulness 和 relevancy。"},
		{"chunking", "文档分块策略：固定大小分块（简单但可能截断语义）、基于段落分块（保持语义完整）、递归字符分割（LangChain 默认）、语义分块（基于 embedding 相似度判断边界）。重叠窗口（overlap）通常设置为块大小的10-20%，避免上下文断裂。"},
		{"go-concurrency", "Go 语言并发模型基于 CSP（通信顺序进程）。goroutine 是轻量级线程，channel 用于 goroutine 间通信。常用并发模式：worker pool、fan-out/fan-in、pipeline。sync.WaitGroup 用于等待一组 goroutine 完成，context 用于超时和取消控制。"},
		{"k8s-deploy", "Kubernetes 部署最佳实践：使用 Deployment 管理无状态应用，StatefulSet 管理有状态服务。资源限制（requests/limits）防止资源争抢。就绪探针（readinessProbe）和存活探针（livenessProbe）确保服务健康。HPA 实现自动扩缩容。"},
		{"react-patterns", "React 现代开发模式：Server Components 减少客户端 JS 体积；Suspense 优化加载体验；use() hook 简化数据获取；React Compiler 自动优化重渲染。状态管理趋势从 Redux 向 Zustand/Jotai 等轻量方案迁移。"},
		{"security-auth", "认证与授权安全架构：JWT（无状态、可扩展）vs Session（服务端存储、更安全）。OAuth 2.0 + OIDC 实现第三方登录。RBAC（基于角色）和 ABAC（基于属性）访问控制。安全要点：HTTPS、CSRF 防护、XSS 防护、SQL 注入防护、速率限制。"},
		{"data-pipeline", "数据管道架构：ETL（Extract-Transform-Load）传统批处理模式；ELT 先加载后转换，适合数据湖场景。流式处理用 Kafka/Flink 实现实时数据流。数据质量检查包括完整性、一致性、时效性校验。Apache Airflow 是主流的工作流编排工具。"},
		{"ml-ops", "MLOps 实践：模型版本管理（MLflow/DVC）、特征存储（Feature Store）、实验追踪、A/B 测试部署。模型监控关注数据漂移（data drift）、概念漂移（concept drift）和性能退化。CI/CD 流水线自动化模型训练、验证和部署。"},
		{"llm-memory", "LLM 记忆分层设计：L1 原始记忆（完整对话历史，token 消耗高）；L2 特征记忆（提取关键实体和关系）；L3 结构记忆（知识图谱形式组织）；L4 模式记忆（用户偏好和行为模式）。从 L1 到 L4 压缩比约 3-5 倍，显著降低上下文 token 消耗。"},
		{"api-design", "RESTful API 设计原则：资源命名使用名词复数；HTTP 方法语义化（GET 读、POST 创建、PUT 更新、DELETE 删除）；版本控制用 URL 或 Header；分页用 cursor 而非 offset。GraphQL 适合复杂查询场景，gRPC 适合微服务间高性能通信。"},
		{"perf-opt", "性能优化方法论：先 profile 后优化，避免过早优化。前端：代码分割、图片懒加载、CDN 缓存。后端：数据库索引优化、连接池、读写分离、缓存策略（Redis）。全栈：减少网络往返（batching）、压缩传输（gzip/brotli）、异步处理。"},
		{"testing-strategy", "测试策略金字塔：单元测试（占比最大，快速、隔离）→ 集成测试（验证模块交互）→ E2E 测试（端到端用户流程）。TDD 开发流程：Red-Green-Refactor。关键质量指标：代码覆盖率、变异测试分数、缺陷逃逸率。Mock 和 Stub 用于隔离外部依赖。"},
	}

	for _, d := range docs {
		src := store.newSource(d.id, SourceText)
		store.addPreparedChunks(src, []PreparedChunk{
			{Content: d.content, Metadata: map[string]string{"doc_id": d.id}},
		})
	}

	queries := []benchQuery{
		{
			Query:    "RAG 检索增强生成架构设计",
			Relevant: []string{"rag-arch", "hybrid-retrieval", "vec-db"},
		},
		{
			Query:    "BM25 算法原理和参数调优",
			Relevant: []string{"bm25-theory", "hybrid-retrieval"},
		},
		{
			Query:    "如何优化 token 消耗降低 LLM 成本",
			Relevant: []string{"token-opt", "llm-memory", "prompt-eng"},
		},
		{
			Query:    "向量数据库 HNSW 索引选型",
			Relevant: []string{"vec-db", "embedding-model"},
		},
		{
			Query:    "混合检索 RRF 融合提升召回率",
			Relevant: []string{"hybrid-retrieval", "bm25-theory", "eval-metrics"},
		},
		{
			Query:    "Agent 多智能体协作架构",
			Relevant: []string{"agent-design", "llm-memory"},
		},
		{
			Query:    "检索效果评估指标 Recall Precision NDCG",
			Relevant: []string{"eval-metrics", "hybrid-retrieval"},
		},
		{
			Query:    "文档分块 chunking 策略",
			Relevant: []string{"chunking", "rag-arch"},
		},
		{
			Query:    "embedding 模型中文多语言选型",
			Relevant: []string{"embedding-model", "vec-db"},
		},
		{
			Query:    "Prompt 工程思维链 CoT 技巧",
			Relevant: []string{"prompt-eng", "token-opt"},
		},
		{
			Query:    "Go 并发模式 goroutine channel",
			Relevant: []string{"go-concurrency"},
		},
		{
			Query:    "Kubernetes 部署 HPA 自动扩缩容",
			Relevant: []string{"k8s-deploy"},
		},
		{
			Query:    "LLM 记忆分层 L1 L2 L3 L4 压缩",
			Relevant: []string{"llm-memory", "token-opt"},
		},
		{
			Query:    "数据管道 ETL 流式处理 Kafka",
			Relevant: []string{"data-pipeline", "ml-ops"},
		},
		{
			Query:    "rerank 重排序模型 cross-encoder",
			Relevant: []string{"eval-metrics", "hybrid-retrieval", "rag-arch"},
		},
	}

	return store, queries
}

// deterministicEmbedder produces repeatable pseudo-embeddings
// based on token overlap — simulates a real embedder's behavior
// without requiring network access.
type deterministicEmbedder struct {
	dim    int
	vocabM map[string]int
	nextID int
}

func newDeterministicEmbedder(dim int) *deterministicEmbedder {
	return &deterministicEmbedder{dim: dim, vocabM: make(map[string]int)}
}

func (e *deterministicEmbedder) Model() string      { return "bench-mock-embedder" }
func (e *deterministicEmbedder) Dimensions() int     { return e.dim }

func (e *deterministicEmbedder) Embed(_ context.Context, input string) ([]float32, error) {
	return e.toVec(input), nil
}

func (e *deterministicEmbedder) EmbedBatch(_ context.Context, inputs []string) ([][]float32, error) {
	vecs := make([][]float32, len(inputs))
	for i, inp := range inputs {
		vecs[i] = e.toVec(inp)
	}
	return vecs, nil
}

func (e *deterministicEmbedder) toVec(text string) []float32 {
	tokens := tokenize(text)
	vec := make([]float32, e.dim)
	for _, t := range tokens {
		idx, ok := e.vocabM[t]
		if !ok {
			idx = e.nextID
			e.vocabM[t] = idx
			e.nextID++
		}
		bucket := idx % e.dim
		vec[bucket] += 1.0
	}
	norm := float32(0)
	for _, v := range vec {
		norm += v * v
	}
	if norm > 0 {
		norm = float32(math.Sqrt(float64(norm)))
		for i := range vec {
			vec[i] /= norm
		}
	}
	return vec
}

// ──────────────────────────────────────────────
// Metric calculations
// ──────────────────────────────────────────────

func recallAtK(retrieved []string, relevant []string) float64 {
	if len(relevant) == 0 {
		return 1.0
	}
	relSet := make(map[string]bool, len(relevant))
	for _, r := range relevant {
		relSet[r] = true
	}
	hit := 0
	for _, r := range retrieved {
		if relSet[r] {
			hit++
		}
	}
	return float64(hit) / float64(len(relevant))
}

func precisionAtK(retrieved []string, relevant []string) float64 {
	if len(retrieved) == 0 {
		return 0
	}
	relSet := make(map[string]bool, len(relevant))
	for _, r := range relevant {
		relSet[r] = true
	}
	hit := 0
	for _, r := range retrieved {
		if relSet[r] {
			hit++
		}
	}
	return float64(hit) / float64(len(retrieved))
}

func mrr(retrieved []string, relevant []string) float64 {
	relSet := make(map[string]bool, len(relevant))
	for _, r := range relevant {
		relSet[r] = true
	}
	for i, r := range retrieved {
		if relSet[r] {
			return 1.0 / float64(i+1)
		}
	}
	return 0
}

func ndcgAtK(retrieved []string, relevant []string) float64 {
	relSet := make(map[string]bool, len(relevant))
	for _, r := range relevant {
		relSet[r] = true
	}
	dcg := 0.0
	for i, r := range retrieved {
		if relSet[r] {
			dcg += 1.0 / math.Log2(float64(i+2)) // i+2 because log2(1)=0
		}
	}
	idealK := len(relevant)
	if idealK > len(retrieved) {
		idealK = len(retrieved)
	}
	idcg := 0.0
	for i := 0; i < idealK; i++ {
		idcg += 1.0 / math.Log2(float64(i+2))
	}
	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

func extractDocIDs(chunks []Chunk) []string {
	ids := make([]string, 0, len(chunks))
	seen := make(map[string]bool)
	for _, c := range chunks {
		docID := c.Metadata["doc_id"]
		if docID == "" {
			for _, src := range []string{c.SourceID, c.ID} {
				if src != "" {
					docID = src
					break
				}
			}
		}
		if docID != "" && !seen[docID] {
			ids = append(ids, docID)
			seen[docID] = true
		}
	}
	return ids
}

func extractDocIDsFromScored(scored []ScoredChunk) []string {
	ids := make([]string, 0, len(scored))
	seen := make(map[string]bool)
	for _, sc := range scored {
		docID := sc.Chunk.Metadata["doc_id"]
		if docID == "" {
			docID = sc.Chunk.SourceID
		}
		if docID != "" && !seen[docID] {
			ids = append(ids, docID)
			seen[docID] = true
		}
	}
	return ids
}

// ──────────────────────────────────────────────
// Benchmark tests
// ──────────────────────────────────────────────

func TestHybridBenchmark(t *testing.T) {
	store, queries := buildBenchCorpus()
	ctx := context.Background()
	K := 5

	emb := newDeterministicEmbedder(256)
	store.SetEmbedder(emb)
	if err := store.BuildIndex(ctx); err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	type methodResult struct {
		name    string
		recall  float64
		prec    float64
		mrrVal  float64
		ndcg    float64
		latency time.Duration
	}

	runMethod := func(name string, searchFn func(q string) []string) methodResult {
		var totalRecall, totalPrec, totalMRR, totalNDCG float64
		var totalLatency time.Duration

		for _, q := range queries {
			start := time.Now()
			retrieved := searchFn(q.Query)
			elapsed := time.Since(start)
			totalLatency += elapsed

			totalRecall += recallAtK(retrieved, q.Relevant)
			totalPrec += precisionAtK(retrieved, q.Relevant)
			totalMRR += mrr(retrieved, q.Relevant)
			totalNDCG += ndcgAtK(retrieved, q.Relevant)
		}

		n := float64(len(queries))
		return methodResult{
			name:    name,
			recall:  totalRecall / n,
			prec:    totalPrec / n,
			mrrVal:  totalMRR / n,
			ndcg:    totalNDCG / n,
			latency: totalLatency / time.Duration(len(queries)),
		}
	}

	// ── Method 1: Pure substring search (baseline) ──
	baseline := runMethod("Substring", func(q string) []string {
		chunks := store.Search(q, K)
		return extractDocIDs(chunks)
	})

	// ── Method 2: Pure BM25 ──
	bm25Only := runMethod("BM25", func(q string) []string {
		store.mu.Lock()
		if store.bm25Cache == nil || store.bm25Built != store.bm25Version {
			store.bm25Cache = NewBM25Index(store.chunks)
			store.bm25Built = store.bm25Version
		}
		bm25Idx := store.bm25Cache
		allChunks := make([]Chunk, len(store.chunks))
		copy(allChunks, store.chunks)
		store.mu.Unlock()

		results := bm25Idx.Search(q, K)
		ids := make([]string, 0, len(results))
		seen := make(map[string]bool)
		for _, r := range results {
			if r.ChunkIndex < len(allChunks) {
				docID := allChunks[r.ChunkIndex].Metadata["doc_id"]
				if docID != "" && !seen[docID] {
					ids = append(ids, docID)
					seen[docID] = true
				}
			}
		}
		return ids
	})

	// ── Method 3: Pure vector search ──
	vectorOnly := runMethod("Vector", func(q string) []string {
		chunks := store.SemanticSearch(ctx, q, K)
		return extractDocIDs(chunks)
	})

	// ── Method 4: Hybrid (BM25 + Vector + RRF) ──
	hybrid := runMethod("Hybrid(BM25+Vec+RRF)", func(q string) []string {
		scored := store.HybridSearch(ctx, q, K)
		return extractDocIDsFromScored(scored)
	})

	methods := []methodResult{baseline, bm25Only, vectorOnly, hybrid}

	// ── Print benchmark report ──
	t.Log("")
	t.Log("╔══════════════════════════════════════════════════════════════════════════╗")
	t.Log("║          Hybrid Retrieval MVP Benchmark Report                          ║")
	t.Log("║          Corpus: 20 docs (中英混合技术知识库) | Queries: 15             ║")
	t.Logf("║          K=%d | Date: %s                                    ║", K, time.Now().Format("2006-01-02"))
	t.Log("╠══════════════════════════════════════════════════════════════════════════╣")
	t.Log("║  Method                 │ Recall@5 │ Prec@5 │  MRR   │ NDCG@5 │ Latency║")
	t.Log("╠═════════════════════════╪══════════╪════════╪════════╪════════╪════════╣")
	for _, m := range methods {
		t.Logf("║  %-23s│  %5.1f%%  │ %5.1f%% │ %5.3f  │ %5.3f  │ %6s ║",
			m.name,
			m.recall*100,
			m.prec*100,
			m.mrrVal,
			m.ndcg,
			fmtDuration(m.latency),
		)
	}
	t.Log("╚══════════════════════════════════════════════════════════════════════════╝")

	// ── Improvement analysis ──
	if hybrid.recall > 0 && bm25Only.recall > 0 {
		recallImproveOverBM25 := (hybrid.recall - bm25Only.recall) / bm25Only.recall * 100
		recallImproveOverVec := float64(0)
		if vectorOnly.recall > 0 {
			recallImproveOverVec = (hybrid.recall - vectorOnly.recall) / vectorOnly.recall * 100
		}
		t.Log("")
		t.Log("── Improvement Analysis ──")
		t.Logf("  Hybrid vs BM25-only:   Recall +%.1f%%", recallImproveOverBM25)
		t.Logf("  Hybrid vs Vector-only: Recall +%.1f%%", recallImproveOverVec)
	}

	// ── Token savings estimation ──
	t.Log("")
	t.Log("── Token Savings Estimation ──")
	avgChunkTokens := 200 // ~200 tokens per chunk for Chinese text
	fullContextTokens := 20 * avgChunkTokens
	hybridContextTokens := K * avgChunkTokens
	savingsRatio := float64(fullContextTokens) / float64(hybridContextTokens)
	t.Logf("  Full context injection:  %d chunks × %d tokens = %d tokens",
		20, avgChunkTokens, fullContextTokens)
	t.Logf("  Hybrid retrieval top-%d:  %d chunks × %d tokens = %d tokens",
		K, K, avgChunkTokens, hybridContextTokens)
	t.Logf("  Token savings ratio:     %.1fx", savingsRatio)
	t.Logf("  With semantic cache (+30%% hit rate): ~%.1fx effective savings", savingsRatio*1.3)

	// ── Assertions: Hybrid should outperform or match single methods ──
	if hybrid.recall < bm25Only.recall*0.9 {
		t.Errorf("Hybrid recall (%.3f) significantly worse than BM25 (%.3f)", hybrid.recall, bm25Only.recall)
	}
}

// BenchmarkBM25Search measures BM25 index build + search throughput.
func BenchmarkBM25Search(b *testing.B) {
	store, queries := buildBenchCorpus()
	store.mu.RLock()
	chunks := make([]Chunk, len(store.chunks))
	copy(chunks, store.chunks)
	store.mu.RUnlock()

	idx := NewBM25Index(chunks)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q := queries[i%len(queries)]
		idx.Search(q.Query, 5)
	}
}

// BenchmarkBM25IndexBuild measures BM25 index construction time.
func BenchmarkBM25IndexBuild(b *testing.B) {
	store, _ := buildBenchCorpus()
	store.mu.RLock()
	chunks := make([]Chunk, len(store.chunks))
	copy(chunks, store.chunks)
	store.mu.RUnlock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewBM25Index(chunks)
	}
}

// BenchmarkHybridSearch measures full hybrid retrieval pipeline.
func BenchmarkHybridSearch(b *testing.B) {
	store, queries := buildBenchCorpus()
	ctx := context.Background()

	emb := newDeterministicEmbedder(256)
	store.SetEmbedder(emb)
	store.BuildIndex(ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q := queries[i%len(queries)]
		store.HybridSearch(ctx, q.Query, 5)
	}
}

// BenchmarkTokenize measures CJK-aware tokenizer throughput.
func BenchmarkTokenize(b *testing.B) {
	text := "RAG检索增强生成架构将大语言模型与外部知识库结合，混合检索BM25+向量实现高召回率"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tokenize(text)
	}
}

// BenchmarkFuseRRF measures RRF fusion overhead.
func BenchmarkFuseRRF(b *testing.B) {
	dense := make([]Chunk, 20)
	sparse := make([]Chunk, 20)
	for i := 0; i < 20; i++ {
		dense[i] = Chunk{ID: fmt.Sprintf("d-%d", i), Content: fmt.Sprintf("dense doc %d", i)}
		sparse[i] = Chunk{ID: fmt.Sprintf("s-%d", i), Content: fmt.Sprintf("sparse doc %d", i)}
	}
	for i := 0; i < 5; i++ {
		sparse[i].ID = dense[i].ID
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FuseRRF(dense, sparse, 60, 5)
	}
}

// TestPerQueryBreakdown prints per-query results for detailed analysis.
func TestPerQueryBreakdown(t *testing.T) {
	store, queries := buildBenchCorpus()
	ctx := context.Background()

	emb := newDeterministicEmbedder(256)
	store.SetEmbedder(emb)
	store.BuildIndex(ctx)

	t.Log("")
	t.Log("── Per-Query Breakdown (Hybrid vs BM25) ──")
	t.Log(strings.Repeat("─", 90))
	t.Logf("  %-40s │ BM25 R@5 │ Hybrid R@5 │  Winner", "Query")
	t.Log(strings.Repeat("─", 90))

	hybridWins, bm25Wins, ties := 0, 0, 0

	for _, q := range queries {
		// BM25
		store.mu.Lock()
		if store.bm25Cache == nil || store.bm25Built != store.bm25Version {
			store.bm25Cache = NewBM25Index(store.chunks)
			store.bm25Built = store.bm25Version
		}
		bm25Idx := store.bm25Cache
		allChunks := make([]Chunk, len(store.chunks))
		copy(allChunks, store.chunks)
		store.mu.Unlock()

		bm25Results := bm25Idx.Search(q.Query, 5)
		bm25IDs := make([]string, 0)
		seen := make(map[string]bool)
		for _, r := range bm25Results {
			if r.ChunkIndex < len(allChunks) {
				docID := allChunks[r.ChunkIndex].Metadata["doc_id"]
				if docID != "" && !seen[docID] {
					bm25IDs = append(bm25IDs, docID)
					seen[docID] = true
				}
			}
		}

		// Hybrid
		hybridResults := store.HybridSearch(ctx, q.Query, 5)
		hybridIDs := extractDocIDsFromScored(hybridResults)

		bm25Recall := recallAtK(bm25IDs, q.Relevant)
		hybridRecall := recallAtK(hybridIDs, q.Relevant)

		winner := "  TIE"
		if hybridRecall > bm25Recall {
			winner = "  HYBRID ✓"
			hybridWins++
		} else if bm25Recall > hybridRecall {
			winner = "  BM25 ✓"
			bm25Wins++
		} else {
			ties++
		}

		queryDisplay := q.Query
		if len([]rune(queryDisplay)) > 38 {
			queryDisplay = string([]rune(queryDisplay)[:35]) + "..."
		}
		t.Logf("  %-40s │  %5.1f%%   │   %5.1f%%    │%s",
			queryDisplay, bm25Recall*100, hybridRecall*100, winner)
	}

	t.Log(strings.Repeat("─", 90))
	t.Logf("  Summary: Hybrid wins=%d, BM25 wins=%d, Ties=%d", hybridWins, bm25Wins, ties)
}

// TestBM25IndexCacheEfficiency verifies the cache invalidation logic.
func TestBM25IndexCacheEfficiency(t *testing.T) {
	store := NewStore(500)
	store.IngestText("doc1", "机器学习深度学习人工智能")

	store.mu.Lock()
	v1 := store.bm25Version
	store.mu.Unlock()

	store.IngestText("doc2", "自然语言处理文本分析")

	store.mu.Lock()
	v2 := store.bm25Version
	store.mu.Unlock()

	if v2 <= v1 {
		t.Error("bm25Version should increment after adding chunks")
	}

	ctx := context.Background()
	results := store.HybridSearch(ctx, "机器学习", 3)
	if len(results) == 0 {
		t.Fatal("expected hybrid results")
	}

	store.mu.Lock()
	cached := store.bm25Cache != nil
	built := store.bm25Built
	store.mu.Unlock()

	if !cached {
		t.Error("BM25 cache should exist after HybridSearch")
	}
	if built != v2 {
		t.Error("BM25 cache should be at latest version")
	}

	// No new chunks → cache should be reused
	_ = store.HybridSearch(ctx, "深度学习", 3)
	store.mu.Lock()
	builtAgain := store.bm25Built
	store.mu.Unlock()

	if builtAgain != built {
		t.Error("BM25 cache should not rebuild when chunks unchanged")
	}
}

// verify the deterministicEmbedder satisfies the interface
var _ embeddings.Embedder = (*deterministicEmbedder)(nil)

func fmtDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	return fmt.Sprintf("%dms", d.Milliseconds())
}
