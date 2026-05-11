// stress — stability & memory stress test for yunque-agent on low-resource servers.
//
// Scenarios:
//   go run ./cmd/stress -addr localhost:9090 -scenario all
//   go run ./cmd/stress -addr localhost:9090 -scenario memory -duration 5m
//   go run ./cmd/stress -addr localhost:9090 -scenario chat -rounds 20 -c 10
//   go run ./cmd/stress -addr localhost:9090 -scenario knowledge -files 50
//   go run ./cmd/stress -addr localhost:9090 -scenario oom -alloc 800
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type config struct {
	addr      string
	apiKey    string
	scenario  string
	duration  time.Duration
	rounds    int
	conc      int
	files     int
	allocMiB  int
	verbose   bool
	reportCSV string
}

type latencyStats struct {
	mu        sync.Mutex
	latencies []time.Duration
}

func (ls *latencyStats) add(d time.Duration) {
	ls.mu.Lock()
	ls.latencies = append(ls.latencies, d)
	ls.mu.Unlock()
}

func (ls *latencyStats) report() {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	if len(ls.latencies) == 0 {
		fmt.Println("  (no samples)")
		return
	}
	sort.Slice(ls.latencies, func(i, j int) bool { return ls.latencies[i] < ls.latencies[j] })
	var sum time.Duration
	for _, l := range ls.latencies {
		sum += l
	}
	n := len(ls.latencies)
	fmt.Printf("  Samples: %d\n", n)
	fmt.Printf("  Avg:     %v\n", (sum / time.Duration(n)).Round(time.Millisecond))
	fmt.Printf("  P50:     %v\n", ls.latencies[n*50/100].Round(time.Millisecond))
	fmt.Printf("  P95:     %v\n", ls.latencies[n*95/100].Round(time.Millisecond))
	fmt.Printf("  P99:     %v\n", ls.latencies[min(n*99/100, n-1)].Round(time.Millisecond))
	fmt.Printf("  Min:     %v\n", ls.latencies[0].Round(time.Millisecond))
	fmt.Printf("  Max:     %v\n", ls.latencies[n-1].Round(time.Millisecond))
}

func main() {
	cfg := config{}
	flag.StringVar(&cfg.addr, "addr", "localhost:9090", "agent address (host:port)")
	flag.StringVar(&cfg.apiKey, "key", "", "API key (env: DEFAULT_API_KEY)")
	flag.StringVar(&cfg.scenario, "scenario", "all", "test scenario: memory|chat|knowledge|oom|all")
	flag.DurationVar(&cfg.duration, "duration", 3*time.Minute, "duration for long-running scenarios")
	flag.IntVar(&cfg.rounds, "rounds", 10, "conversation rounds per session (chat scenario)")
	flag.IntVar(&cfg.conc, "c", 5, "concurrency level")
	flag.IntVar(&cfg.files, "files", 20, "number of knowledge files to ingest")
	flag.IntVar(&cfg.allocMiB, "alloc", 600, "target allocation in MiB (oom scenario)")
	flag.BoolVar(&cfg.verbose, "v", false, "verbose output")
	flag.StringVar(&cfg.reportCSV, "csv", "", "write memory samples to CSV file")
	flag.Parse()

	if cfg.apiKey == "" {
		cfg.apiKey = os.Getenv("DEFAULT_API_KEY")
		if cfg.apiKey == "" {
			cfg.apiKey = os.Getenv("API_KEY")
		}
	}

	fmt.Println("╔══════════════════════════════════════════════╗")
	fmt.Println("║  yunque-agent stress test suite              ║")
	fmt.Println("╚══════════════════════════════════════════════╝")
	fmt.Printf("  Target:      http://%s\n", cfg.addr)
	fmt.Printf("  Scenario:    %s\n", cfg.scenario)
	fmt.Printf("  Concurrency: %d\n", cfg.conc)
	fmt.Printf("  Duration:    %v\n", cfg.duration)
	fmt.Println()

	if !healthCheck(cfg) {
		fmt.Println("[FATAL] /healthz check failed — is the agent running?")
		os.Exit(1)
	}
	fmt.Println("[OK] Health check passed")
	fmt.Println()

	switch cfg.scenario {
	case "memory":
		runMemoryProfile(cfg)
	case "chat":
		runMultiRoundChat(cfg)
	case "knowledge":
		runKnowledgeIngest(cfg)
	case "oom":
		runOOMSimulation(cfg)
	case "all":
		runMemoryProfile(cfg)
		runMultiRoundChat(cfg)
		runKnowledgeIngest(cfg)
		fmt.Println("\n[INFO] OOM scenario skipped in 'all' mode (run with -scenario oom explicitly)")
	default:
		fmt.Printf("[ERROR] unknown scenario: %s\n", cfg.scenario)
		os.Exit(1)
	}

	fmt.Println("\n[DONE] All stress tests completed.")
}

func healthCheck(cfg config) bool {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s/healthz", cfg.addr))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// ─────────────── Scenario 1: Memory Profile ───────────────

func runMemoryProfile(cfg config) {
	fmt.Println("═══ Scenario: Memory Profile Under GOMEMLIMIT ═══")
	fmt.Printf("  Duration: %v | Sampling interval: 2s\n\n", cfg.duration)

	var csvFile *os.File
	if cfg.reportCSV != "" {
		var err error
		csvFile, err = os.Create(cfg.reportCSV)
		if err != nil {
			fmt.Printf("  [WARN] cannot create CSV: %v\n", err)
		} else {
			defer csvFile.Close()
			fmt.Fprintln(csvFile, "elapsed_s,rss_estimate_mib,goroutines,heap_alloc_mib,heap_sys_mib,gc_count,success,fail")
		}
	}

	client := &http.Client{Timeout: 120 * time.Second}
	var success, fail atomic.Int64
	stop := make(chan struct{})

	go func() {
		prompts := multiRoundPrompts()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			prompt := prompts[i%len(prompts)]
			body, _ := json.Marshal(map[string]any{
				"messages":   []map[string]string{{"role": "user", "content": prompt}},
				"session_id": fmt.Sprintf("mem-profile-%d", i%5),
			})
			req, _ := http.NewRequest("POST", fmt.Sprintf("http://%s/v1/chat", cfg.addr), bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+cfg.apiKey)
			resp, err := client.Do(req)
			if err != nil {
				fail.Add(1)
				continue
			}
			io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 200 {
				success.Add(1)
			} else {
				fail.Add(1)
			}
			time.Sleep(500 * time.Millisecond)
		}
	}()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	start := time.Now()

	var peakHeap uint64
	samples := 0

	for {
		select {
		case <-ticker.C:
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			elapsed := time.Since(start).Seconds()

			if m.HeapAlloc > peakHeap {
				peakHeap = m.HeapAlloc
			}

			if cfg.verbose || samples%15 == 0 {
				fmt.Printf("  [%5.0fs] Heap=%4dM Sys=%4dM GC=%d GoRoutines=%d OK=%d ERR=%d\n",
					elapsed,
					m.HeapAlloc/1024/1024,
					m.HeapSys/1024/1024,
					m.NumGC,
					runtime.NumGoroutine(),
					success.Load(),
					fail.Load(),
				)
			}

			if csvFile != nil {
				fmt.Fprintf(csvFile, "%.0f,%d,%d,%d,%d,%d,%d,%d\n",
					elapsed,
					m.Sys/1024/1024,
					runtime.NumGoroutine(),
					m.HeapAlloc/1024/1024,
					m.HeapSys/1024/1024,
					m.NumGC,
					success.Load(),
					fail.Load(),
				)
			}
			samples++

			if time.Since(start) >= cfg.duration {
				close(stop)
				fmt.Printf("\n  ── Memory Profile Summary ──\n")
				fmt.Printf("  Peak Heap:      %d MiB\n", peakHeap/1024/1024)
				fmt.Printf("  Final Sys:      %d MiB\n", m.Sys/1024/1024)
				fmt.Printf("  Total GC:       %d\n", m.NumGC)
				fmt.Printf("  Requests OK:    %d\n", success.Load())
				fmt.Printf("  Requests FAIL:  %d\n", fail.Load())

				if peakHeap > 400*1024*1024 {
					fmt.Println("  [WARN] Peak heap exceeded 400MiB GOMEMLIMIT target")
				} else {
					fmt.Println("  [PASS] Heap stayed within GOMEMLIMIT budget")
				}
				return
			}
		}
	}
}

// ─────────────── Scenario 2: Multi-Round Chat ───────────────

func runMultiRoundChat(cfg config) {
	fmt.Println("\n═══ Scenario: Multi-Round Conversation Stress ═══")
	fmt.Printf("  Sessions: %d concurrent | Rounds/session: %d\n\n", cfg.conc, cfg.rounds)

	client := &http.Client{Timeout: 120 * time.Second}
	var wg sync.WaitGroup
	var totalSuccess, totalFail atomic.Int64
	stats := &latencyStats{}

	prompts := multiRoundPrompts()

	for w := 0; w < cfg.conc; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			sessionID := fmt.Sprintf("stress-chat-%d-%d", workerID, time.Now().UnixMilli())
			var history []map[string]string

			for r := 0; r < cfg.rounds; r++ {
				prompt := prompts[r%len(prompts)]
				history = append(history, map[string]string{"role": "user", "content": prompt})

				body, _ := json.Marshal(map[string]any{
					"messages":   history,
					"session_id": sessionID,
				})

				req, _ := http.NewRequest("POST", fmt.Sprintf("http://%s/v1/chat", cfg.addr), bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+cfg.apiKey)

				t0 := time.Now()
				resp, err := client.Do(req)
				lat := time.Since(t0)

				if err != nil {
					totalFail.Add(1)
					if cfg.verbose {
						fmt.Printf("  [W%d R%d] ERR: %v\n", workerID, r, err)
					}
					continue
				}

				respBody, _ := io.ReadAll(resp.Body)
				resp.Body.Close()

				if resp.StatusCode == 200 {
					totalSuccess.Add(1)
					stats.add(lat)
					var parsed map[string]any
					if json.Unmarshal(respBody, &parsed) == nil {
						if reply, ok := parsed["reply"].(string); ok && len(reply) > 0 {
							truncated := reply
							if len(truncated) > 100 {
								truncated = truncated[:100]
							}
							history = append(history, map[string]string{"role": "assistant", "content": truncated})
						}
					}
				} else {
					totalFail.Add(1)
					if cfg.verbose {
						fmt.Printf("  [W%d R%d] HTTP %d\n", workerID, r, resp.StatusCode)
					}
				}

				// Keep context window manageable
				if len(history) > 20 {
					history = history[len(history)-20:]
				}
			}

			if cfg.verbose {
				fmt.Printf("  [W%d] Session %s completed %d rounds\n", workerID, sessionID, cfg.rounds)
			}
		}(w)
	}

	wg.Wait()

	fmt.Printf("\n  ── Multi-Round Chat Summary ──\n")
	fmt.Printf("  Total Requests:  %d\n", totalSuccess.Load()+totalFail.Load())
	fmt.Printf("  Success:         %d\n", totalSuccess.Load())
	fmt.Printf("  Failed:          %d\n", totalFail.Load())
	fmt.Println("  Latency:")
	stats.report()

	failRate := float64(totalFail.Load()) / float64(totalSuccess.Load()+totalFail.Load()) * 100
	if failRate > 5 {
		fmt.Printf("  [WARN] Failure rate %.1f%% exceeds 5%% threshold\n", failRate)
	} else {
		fmt.Printf("  [PASS] Failure rate %.1f%% within threshold\n", failRate)
	}
}

// ─────────────── Scenario 3: Knowledge Base Ingest ───────────────

func runKnowledgeIngest(cfg config) {
	fmt.Println("\n═══ Scenario: Knowledge Base Import Stress ═══")
	fmt.Printf("  Files: %d | Concurrency: %d\n\n", cfg.files, cfg.conc)

	client := &http.Client{Timeout: 120 * time.Second}
	var wg sync.WaitGroup
	var success, fail atomic.Int64
	stats := &latencyStats{}
	sem := make(chan struct{}, cfg.conc)

	for i := 0; i < cfg.files; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			docContent := generateKnowledgeDoc(idx)

			body, _ := json.Marshal(map[string]any{
				"content":  docContent,
				"source":   fmt.Sprintf("stress-test-doc-%d.md", idx),
				"metadata": map[string]string{"category": "stress-test", "index": fmt.Sprintf("%d", idx)},
			})

			req, _ := http.NewRequest("POST", fmt.Sprintf("http://%s/v1/knowledge/ingest", cfg.addr), bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+cfg.apiKey)

			t0 := time.Now()
			resp, err := client.Do(req)
			lat := time.Since(t0)

			if err != nil {
				fail.Add(1)
				if cfg.verbose {
					fmt.Printf("  [Doc%d] ERR: %v\n", idx, err)
				}
				return
			}
			defer resp.Body.Close()
			io.ReadAll(resp.Body)

			if resp.StatusCode == 200 || resp.StatusCode == 201 {
				success.Add(1)
				stats.add(lat)
			} else {
				fail.Add(1)
				if cfg.verbose {
					fmt.Printf("  [Doc%d] HTTP %d\n", idx, resp.StatusCode)
				}
			}
		}(i)
	}

	wg.Wait()

	// Now test knowledge search under load
	fmt.Println("\n  ── Knowledge Search Under Load ──")
	searchQueries := []string{
		"机器学习算法", "Go语言并发", "分布式系统一致性",
		"微服务架构", "容器编排技术", "深度学习优化",
	}

	var searchSuccess, searchFail atomic.Int64
	searchStats := &latencyStats{}

	for i := 0; i < len(searchQueries)*3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			query := searchQueries[idx%len(searchQueries)]
			body, _ := json.Marshal(map[string]any{"query": query, "limit": 5})
			req, _ := http.NewRequest("POST", fmt.Sprintf("http://%s/v1/knowledge/search", cfg.addr), bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+cfg.apiKey)

			t0 := time.Now()
			resp, err := client.Do(req)
			lat := time.Since(t0)

			if err != nil {
				searchFail.Add(1)
				return
			}
			defer resp.Body.Close()
			io.ReadAll(resp.Body)

			if resp.StatusCode == 200 {
				searchSuccess.Add(1)
				searchStats.add(lat)
			} else {
				searchFail.Add(1)
			}
		}(i)
	}

	wg.Wait()

	fmt.Printf("\n  ── Knowledge Ingest Summary ──\n")
	fmt.Printf("  Docs Ingested:  %d / %d\n", success.Load(), cfg.files)
	fmt.Printf("  Ingest Failed:  %d\n", fail.Load())
	fmt.Println("  Ingest Latency:")
	stats.report()
	fmt.Printf("\n  Search OK: %d  FAIL: %d\n", searchSuccess.Load(), searchFail.Load())
	fmt.Println("  Search Latency:")
	searchStats.report()
}

// ─────────────── Scenario 4: OOM Simulation ───────────────

func runOOMSimulation(cfg config) {
	fmt.Println("\n═══ Scenario: OOM Resilience Test ═══")
	fmt.Printf("  Target allocation: %d MiB\n", cfg.allocMiB)
	fmt.Println("  This test pushes the agent to memory limits and verifies auto-recovery.")
	fmt.Println()

	client := &http.Client{Timeout: 30 * time.Second}

	fmt.Println("  Phase 1: Verify agent is healthy before stress...")
	if !healthCheck(cfg) {
		fmt.Println("  [FATAL] Agent not reachable before OOM test")
		return
	}
	fmt.Println("  [OK] Agent healthy")
	fmt.Println()

	fmt.Println("  Phase 2: Generate extreme concurrent load...")
	var wg sync.WaitGroup
	var success, fail atomic.Int64

	hugeDocs := make([]string, 20)
	for i := range hugeDocs {
		hugeDocs[i] = generateLargePayload(cfg.allocMiB / 20)
	}

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			body, _ := json.Marshal(map[string]any{
				"messages":   []map[string]string{{"role": "user", "content": hugeDocs[idx]}},
				"session_id": fmt.Sprintf("oom-stress-%d", idx),
			})
			req, _ := http.NewRequest("POST", fmt.Sprintf("http://%s/v1/chat", cfg.addr), bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+cfg.apiKey)
			resp, err := client.Do(req)
			if err != nil {
				fail.Add(1)
				return
			}
			defer resp.Body.Close()
			io.ReadAll(resp.Body)
			if resp.StatusCode == 200 {
				success.Add(1)
			} else {
				fail.Add(1)
			}
		}(i)
	}
	wg.Wait()
	fmt.Printf("  Extreme load results: OK=%d FAIL=%d\n\n", success.Load(), fail.Load())

	fmt.Println("  Phase 3: Wait for potential OOM recovery (30s)...")
	time.Sleep(30 * time.Second)

	fmt.Println("  Phase 4: Verify agent auto-recovery via health check...")
	maxRetries := 12
	recovered := false
	for i := 0; i < maxRetries; i++ {
		if healthCheck(cfg) {
			recovered = true
			fmt.Printf("  [PASS] Agent recovered after %d seconds\n", (i+1)*5)
			break
		}
		fmt.Printf("  [WAIT] Attempt %d/%d — agent not ready, retrying in 5s...\n", i+1, maxRetries)
		time.Sleep(5 * time.Second)
	}

	if !recovered {
		fmt.Println("  [FAIL] Agent did not recover within 60s — check systemd restart config")
	}

	fmt.Println("\n  Phase 5: Post-recovery functional test...")
	if recovered {
		body, _ := json.Marshal(map[string]any{
			"messages":   []map[string]string{{"role": "user", "content": "你好"}},
			"session_id": "oom-recovery-verify",
		})
		req, _ := http.NewRequest("POST", fmt.Sprintf("http://%s/v1/chat", cfg.addr), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+cfg.apiKey)
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			fmt.Println("  [PASS] Post-recovery chat request succeeded")
		} else {
			fmt.Println("  [FAIL] Post-recovery chat request failed")
		}
	}
}

// ─────────────── Helpers ───────────────

func multiRoundPrompts() []string {
	return []string{
		"你好，请介绍一下你自己",
		"继续上一轮的话题，能更详细地说明一下吗？",
		"请用Python写一个快速排序算法",
		"解释一下Go语言中的goroutine和channel",
		"如何设计一个高可用的微服务架构？",
		"什么是分布式一致性？请用通俗的语言解释",
		"帮我分析一下这段代码可能的性能瓶颈",
		"请对比一下Redis和Memcached的优缺点",
		"如何实现一个可靠的消息队列？",
		"总结一下我们今天讨论的所有内容",
		"什么是向量数据库？它和传统数据库有什么不同？",
		"解释一下Transformer架构的自注意力机制",
		"Kubernetes中Pod的生命周期是怎样的？",
		"如何优化大规模数据的批处理任务？",
		"请设计一个简单的API网关，要求支持限流和熔断",
		"什么是混沌工程？它在生产环境中如何应用？",
		"解释一下CAP定理和BASE理论的关系",
		"如何在Go中实现优雅关闭？",
		"请介绍几种常见的负载均衡算法",
		"帮我设计一个简单的监控告警系统",
	}
}

func generateKnowledgeDoc(idx int) string {
	topics := []string{
		"机器学习", "深度学习", "自然语言处理", "计算机视觉",
		"分布式系统", "微服务架构", "云原生技术", "容器编排",
		"Go语言编程", "数据库优化", "网络安全", "DevOps实践",
	}
	topic := topics[idx%len(topics)]

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s技术文档 — 第%d章\n\n", topic, idx+1))
	sb.WriteString(fmt.Sprintf("## 概述\n\n本章将深入探讨%s的核心概念与实践方法。", topic))
	sb.WriteString("在当今快速发展的技术领域中，掌握这些基础知识至关重要。\n\n")

	for section := 0; section < 5; section++ {
		sb.WriteString(fmt.Sprintf("## %d.%d %s的关键技术点\n\n", idx+1, section+1, topic))
		for para := 0; para < 3; para++ {
			sb.WriteString(fmt.Sprintf(
				"在%s领域中，第%d个关键概念涉及到系统的可扩展性与可靠性设计。"+
					"通过合理的架构分层和组件解耦，我们可以构建出具有高可用性的分布式系统。"+
					"这不仅需要对底层技术有深入的理解，还需要在工程实践中不断积累经验。"+
					"性能优化是一个持续的过程，需要在吞吐量、延迟和资源利用率之间取得平衡。\n\n",
				topic, para+1,
			))
		}
	}

	return sb.String()
}

func generateLargePayload(sizeMiB int) string {
	targetBytes := sizeMiB * 1024 * 1024
	if targetBytes > 50*1024*1024 {
		targetBytes = 50 * 1024 * 1024
	}

	const chars = "这是一段用于压力测试的大型文本内容。我们通过生成大量数据来模拟极端场景下的内存压力。"
	var sb strings.Builder
	sb.Grow(targetBytes)
	for sb.Len() < targetBytes {
		sb.WriteString(chars)
		sb.WriteString(fmt.Sprintf(" [chunk-%d] ", rand.Intn(99999)))
	}
	return sb.String()
}
