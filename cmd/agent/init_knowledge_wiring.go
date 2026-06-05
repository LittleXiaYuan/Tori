package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"yunque-agent/internal/agentcore/embeddings"
	"yunque-agent/internal/agentcore/knowledge"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/planner"
	agentrt "yunque-agent/internal/agentcore/runtime"
	"yunque-agent/internal/controlplane/gateway"
	iledger "yunque-agent/internal/ledger"
	"yunque-agent/pkg/safego"

	"yunque-agent/internal/ledgercore"
)

func initKnowledgeWiring(app *agentrt.App, gw *gateway.Gateway, embedRes *embeddings.Resolver) *knowledge.Store {
	cfg := app.Config

	knowledgeStore := knowledge.NewStore(DefaultKnowledgeChunkSize)
	knowledgeStore.SetMetricsHooks(app.Metrics.RecordKnowledgeSearch, app.Metrics.RecordRerank)
	kbDir := cfg.DataPath("knowledge")
	if _, err := os.Stat(kbDir); err == nil {
		entries, _ := os.ReadDir(kbDir)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			path := filepath.Join(kbDir, e.Name())
			ext := strings.ToLower(filepath.Ext(e.Name()))
			switch ext {
			case ".txt", ".md":
				if _, err := knowledgeStore.IngestFile(path); err != nil {
					slog.Warn("knowledge ingest failed", "file", e.Name(), "err", err)
				}
			case ".csv":
				if _, err := knowledgeStore.IngestCSV(path); err != nil {
					slog.Warn("knowledge ingest failed", "file", e.Name(), "err", err)
				}
			case ".json":
				if _, err := knowledgeStore.IngestJSON(path); err != nil {
					slog.Warn("knowledge ingest failed", "file", e.Name(), "err", err)
				}
			case ".pdf":
				if _, err := knowledgeStore.IngestPDF(path); err != nil {
					slog.Warn("knowledge ingest failed", "file", e.Name(), "err", err)
				}
			case ".docx":
				if data, readErr := os.ReadFile(path); readErr == nil {
					if _, err := knowledgeStore.IngestDocxBytes(e.Name(), data); err != nil {
						slog.Warn("knowledge ingest failed", "file", e.Name(), "err", err)
					}
				}
			case ".xlsx":
				if data, readErr := os.ReadFile(path); readErr == nil {
					if _, err := knowledgeStore.IngestXlsxBytes(e.Name(), data); err != nil {
						slog.Warn("knowledge ingest failed", "file", e.Name(), "err", err)
					}
				}
			}
		}
	}
	kbStats := knowledgeStore.Stats()
	slog.Info("knowledge base loaded", "sources", kbStats.Sources, "chunks", kbStats.Chunks)
	if ldgRaw, ok := app.Get(agentrt.CompLedger); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			knowledgeStore.SetKVStore(iledger.NewKVConfigStore(ldg, "knowledge"))
			slog.Info("knowledge store wired to Ledger KV")
		}
	}
	gw.SetKnowledgeStore(knowledgeStore)
	gw.SetKnowledgeDir(kbDir)
	app.Set(agentrt.CompKnowledgeStore, knowledgeStore)

	// Wire LLM Wiki store
	wikiDigester := knowledge.NewLLMWikiDigester(func(ctx context.Context, prompt string) (string, error) {
		pool := app.LLMPool
		if pool == nil {
			return "", fmt.Errorf("no LLM pool available")
		}
		client := pool.Get("smart")
		if client == nil {
			client = pool.Primary()
		}
		if client == nil {
			return "", fmt.Errorf("no LLM client available")
		}
		return client.Chat(ctx, []llm.Message{{Role: "user", Content: prompt}}, 0.3)
	})
	wikiStore := knowledge.NewWikiStore(wikiDigester)
	if ldgRaw, ok := app.Get(agentrt.CompLedger); ok {
		if ldg, ok := ldgRaw.(*ledger.Ledger); ok {
			wikiStore.SetKVStore(iledger.NewKVConfigStore(ldg, "wiki"))
		}
	}
	gw.SetWikiStore(wikiStore)
	app.Set("wiki_store", wikiStore)

	// Wire embedder → knowledge store. Building the vector index embeds every KB chunk
	// in batches (~1-2.4s, scales with corpus size + embedder latency) and used to run
	// synchronously here — on the boot critical path before /healthz binds, so the desktop
	// loader waited on it. Build it in the background instead: BuildIndex is concurrency-
	// safe and SemanticSearch transparently falls back to keyword search until the index
	// reports ready, so it is a search enhancement, not a prerequisite for serving.
	if emb, ok := embedRes.Primary(); ok {
		knowledgeStore.SetEmbedder(emb)
		if kbStats.Chunks > 0 {
			safego.Go("knowledge-build-index", func() {
				if err := knowledgeStore.BuildIndex(context.Background()); err != nil {
					slog.Warn("knowledge: semantic index build failed", "err", err)
				}
			})
		}
	}

	// Wire reranker (Jina / Cohere)
	if jinaKey := os.Getenv("JINA_API_KEY"); jinaKey != "" {
		knowledgeStore.SetReranker(knowledge.NewJinaReranker(knowledge.JinaRerankerConfig{
			APIKey: jinaKey,
			Model:  os.Getenv("JINA_RERANK_MODEL"),
		}))
	} else if cohereKey := os.Getenv("COHERE_API_KEY"); cohereKey != "" {
		knowledgeStore.SetReranker(knowledge.NewCohereReranker(knowledge.CohereRerankerConfig{
			APIKey: cohereKey,
			Model:  os.Getenv("COHERE_RERANK_MODEL"),
		}))
	}

	wireKnowledgeToPlanner(app.Planner, knowledgeStore)

	return knowledgeStore
}

func wireKnowledgeToPlanner(p *planner.Planner, ks *knowledge.Store) {
	p.AppendGraphContext(func(query string) string {
		var parts []string

		scored := ks.HybridSearchReranked(context.Background(), query, DefaultKnowledgeTopK)
		if len(scored) > 0 {
			var sb strings.Builder
			sb.WriteString("## 知识库检索结果\n")
			sb.WriteString("**重要：以下内容来自用户上传的知识库，是最权威的信息来源。回答用户问题时必须优先使用这些内容，而不是调用 web_search 或 file_search 等工具。**\n")
			sb.WriteString("请在回答中引用来源（使用【来源: 文件名】标注）。\n\n")
			n := 0
			for _, sc := range scored {
				if sc.Chunk.Metadata != nil && sc.Chunk.Metadata["lang"] != "" {
					continue
				}
				n++
				sourceName := ""
				if sc.Chunk.Metadata != nil {
					if f := sc.Chunk.Metadata["file"]; f != "" {
						sourceName = f
					} else if u := sc.Chunk.Metadata["url"]; u != "" {
						sourceName = u
					}
				}
				if sourceName == "" {
					if src := ks.GetSource(sc.Chunk.SourceID); src != nil {
						sourceName = src.Name
					}
				}
				if sourceName != "" {
					sb.WriteString(fmt.Sprintf("[来源%d: %s]\n%s\n\n", n, sourceName, sc.Chunk.Content))
				} else {
					sb.WriteString(fmt.Sprintf("[来源%d]\n%s\n\n", n, sc.Chunk.Content))
				}
				if n >= MaxKnowledgeResults {
					break
				}
			}
			if n > 0 {
				parts = append(parts, sb.String())
			}
		}

		return strings.Join(parts, "\n")
	})

	p.SetCodeContext(func(query string) string {
		if !ks.HasCodeSources() {
			return ""
		}
		scored := ks.HybridSearchReranked(context.Background(), query, DefaultCodeTopK)
		var codeResults []knowledge.ScoredChunk
		for _, sc := range scored {
			if sc.Chunk.Metadata != nil && sc.Chunk.Metadata["lang"] != "" {
				codeResults = append(codeResults, sc)
				if len(codeResults) >= MaxKnowledgeResults {
					break
				}
			}
		}
		if len(codeResults) == 0 {
			return ""
		}
		var sb strings.Builder
		sb.WriteString("## 代码上下文\n以下是从代码仓库中检索到的相关代码片段：\n")
		for i, sc := range codeResults {
			filePath := sc.Chunk.Metadata["file"]
			lang := sc.Chunk.Metadata["lang"]
			content := sc.Chunk.Content
			if strings.HasPrefix(content, "FILE:") {
				if idx := strings.Index(content, "\n\n"); idx > 0 && idx < 80 {
					content = strings.TrimSpace(content[idx+2:])
				}
			}
			sb.WriteString(fmt.Sprintf("\n### %d. %s (%s)\n```%s\n%s\n```\n", i+1, filePath, lang, lang, content))
		}
		return sb.String()
	})
}
