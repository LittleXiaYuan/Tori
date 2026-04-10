// bench — concurrent load test for yunque-agent core chain.
// Usage: go run ./cmd/bench -addr localhost:9090 -n 50 -c 5
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	addr := flag.String("addr", "localhost:9090", "agent address")
	total := flag.Int("n", 50, "total requests")
	concurrency := flag.Int("c", 5, "concurrent workers")
	apiKey := flag.String("key", "", "API key (default: DEFAULT_API_KEY or API_KEY)")
	flag.Parse()

	if *apiKey == "" {
		*apiKey = os.Getenv("DEFAULT_API_KEY")
		if *apiKey == "" {
			*apiKey = os.Getenv("API_KEY")
		}
	}

	url := fmt.Sprintf("http://%s/v1/chat", *addr)

	prompts := []string{
		"你好，请介绍一下你自己",
		"什么是机器学习？",
		"帮我写一首关于春天的诗",
		"解释一下量子计算",
		"今天天气怎么样？",
		"Go语言的并发模型是什么？",
		"请帮我总结一下项目进展",
		"如何提高代码质量？",
	}

	fmt.Printf("=== yunque-agent bench ===\n")
	fmt.Printf("Target: %s\n", url)
	fmt.Printf("Requests: %d, Concurrency: %d\n\n", *total, *concurrency)

	var (
		success   atomic.Int64
		fail      atomic.Int64
		latMu     sync.Mutex
		latencies []time.Duration
		wg        sync.WaitGroup
		sem       = make(chan struct{}, *concurrency)
	)

	client := &http.Client{Timeout: 120 * time.Second}
	start := time.Now()

	for i := 0; i < *total; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			prompt := prompts[idx%len(prompts)]
			body, _ := json.Marshal(map[string]any{
				"messages":   []map[string]string{{"role": "user", "content": prompt}},
				"session_id": fmt.Sprintf("bench-%d", idx),
			})

			req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+*apiKey)

			t0 := time.Now()
			resp, err := client.Do(req)
			lat := time.Since(t0)

			if err != nil {
				fail.Add(1)
				fmt.Printf("  [%d] ERR: %v\n", idx, err)
				return
			}
			defer resp.Body.Close()
			io.ReadAll(resp.Body)

			if resp.StatusCode == 200 {
				success.Add(1)
			} else {
				fail.Add(1)
				fmt.Printf("  [%d] HTTP %d\n", idx, resp.StatusCode)
			}

			latMu.Lock()
			latencies = append(latencies, lat)
			latMu.Unlock()
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	// Results
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	fmt.Printf("\n=== Results ===\n")
	fmt.Printf("Total time:  %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("Success:     %d\n", success.Load())
	fmt.Printf("Failed:      %d\n", fail.Load())
	fmt.Printf("RPS:         %.1f\n", float64(*total)/elapsed.Seconds())

	if len(latencies) > 0 {
		var sum time.Duration
		for _, l := range latencies {
			sum += l
		}
		avg := sum / time.Duration(len(latencies))
		p50 := latencies[len(latencies)*50/100]
		p95 := latencies[len(latencies)*95/100]
		p99 := latencies[min(len(latencies)*99/100, len(latencies)-1)]

		fmt.Printf("Avg latency: %v\n", avg.Round(time.Millisecond))
		fmt.Printf("P50:         %v\n", p50.Round(time.Millisecond))
		fmt.Printf("P95:         %v\n", p95.Round(time.Millisecond))
		fmt.Printf("P99:         %v\n", p99.Round(time.Millisecond))
		fmt.Printf("Min:         %v\n", latencies[0].Round(time.Millisecond))
		fmt.Printf("Max:         %v\n", latencies[len(latencies)-1].Round(time.Millisecond))
	}
}
