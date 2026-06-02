// Command recall-eval is a smoke/eval tool for memory recall quality.
//
// It seeds a running Yunque agent with a set of facts, then queries it with
// paraphrased questions (different wording than the stored fact) and measures
// how often the expected memory is recalled. This isolates the lever that makes
// the companion "remember you": semantic recall (paraphrase still hits) vs
// keyword recall (only exact-word overlap hits).
//
// Usage:
//
//	go run ./cmd/recall-eval -base http://localhost:9090 -key ya_xxx
//	go run ./cmd/recall-eval -dataset probes.json   # custom dataset
//
// A non-zero exit code is returned when the hit rate is below -min (default 0).
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Probe is one fact to seed plus a paraphrased query that should recall it.
type Probe struct {
	Fact   string `json:"fact"`   // stored memory
	Query  string `json:"query"`  // paraphrased question (different wording)
	Expect string `json:"expect"` // substring that must appear in a recalled item
}

// defaultProbes is a small Chinese companion-style dataset. Each query is
// deliberately worded differently from the fact so keyword-only recall fails
// while semantic recall succeeds.
var defaultProbes = []Probe{
	{Fact: "用户是一名后端工程师，主要写 Go 语言", Query: "我平时用什么编程语言工作？", Expect: "Go"},
	{Fact: "用户对花生过敏，吃了会起疹子", Query: "我有什么食物不能碰？", Expect: "花生"},
	{Fact: "用户养了一只叫团子的橘猫", Query: "我家的宠物叫什么名字？", Expect: "团子"},
	{Fact: "用户最近在准备雅思考试，目标分数 7 分", Query: "我最近在为哪门英语考试做准备？", Expect: "雅思"},
	{Fact: "用户喜欢在晚上工作，白天效率比较低", Query: "我一天里什么时候状态最好？", Expect: "晚上"},
	{Fact: "用户住在青岛，靠近海边", Query: "我现在生活在哪座城市？", Expect: "青岛"},
	{Fact: "用户的女儿今年六岁，刚上小学一年级", Query: "我孩子多大了？", Expect: "六岁"},
	{Fact: "用户讨厌开会，觉得大部分会议浪费时间", Query: "我最不喜欢的工作安排是什么？", Expect: "会议"},
}

type client struct {
	base string
	key  string
	hc   *http.Client
}

func (c *client) do(method, path string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.base+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("X-API-Key", c.key)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s %s: status %d: %s", method, path, resp.StatusCode, string(data))
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

type recallDebugResponse struct {
	Query string `json:"query"`
	Count int    `json:"count"`
	Items []struct {
		Content  string  `json:"content"`
		Source   string  `json:"source"`
		Score    float64 `json:"score"`
		RawScore float64 `json:"raw_score"`
	} `json:"items"`
}

func main() {
	var (
		base    = flag.String("base", "http://localhost:9090", "agent base URL")
		key     = flag.String("key", envOr("DEFAULT_API_KEY", ""), "API key (or DEFAULT_API_KEY env)")
		dataset = flag.String("dataset", "", "optional JSON file of probes ([]{fact,query,expect})")
		wait    = flag.Duration("wait", 2*time.Second, "wait after seeding for embedding/promotion")
		minRate = flag.Float64("min", 0, "minimum hit rate (0-1); exit non-zero if below")
		noSeed  = flag.Bool("no-seed", false, "skip seeding facts (query only)")
	)
	flag.Parse()

	probes := defaultProbes
	if *dataset != "" {
		raw, err := os.ReadFile(*dataset)
		if err != nil {
			fail("read dataset: %v", err)
		}
		if err := json.Unmarshal(raw, &probes); err != nil {
			fail("parse dataset: %v", err)
		}
	}
	if len(probes) == 0 {
		fail("no probes to evaluate")
	}

	c := &client{base: strings.TrimRight(*base, "/"), key: *key, hc: &http.Client{Timeout: 30 * time.Second}}

	if !*noSeed {
		fmt.Printf("Seeding %d facts...\n", len(probes))
		for _, p := range probes {
			body := map[string]any{"value": p.Fact, "layer": "long", "source": "recall-eval"}
			if err := c.do(http.MethodPost, "/v1/memory/add", body, nil); err != nil {
				fail("seed fact %q: %v", p.Fact, err)
			}
		}
		fmt.Printf("Waiting %s for embedding/promotion...\n", *wait)
		time.Sleep(*wait)
	}

	hits, semanticHits := 0, 0
	fmt.Println()
	for i, p := range probes {
		var resp recallDebugResponse
		body := map[string]any{"query": p.Query, "limit": 10}
		if err := c.do(http.MethodPost, "/v1/memory/recall/debug", body, &resp); err != nil {
			fail("recall %q: %v", p.Query, err)
		}

		hit := false
		topSource, topScore := "-", 0.0
		for j, it := range resp.Items {
			if j == 0 {
				topSource, topScore = it.Source, it.Score
			}
			if strings.Contains(it.Content, p.Expect) {
				hit = true
				if it.Source == "long" || it.Source == "graph" {
					semanticHits++
				}
				break
			}
		}
		if hit {
			hits++
		}
		status := "MISS"
		if hit {
			status = "HIT "
		}
		fmt.Printf("%2d. [%s] expect=%-8q top=%s(%.2f) n=%d  q=%s\n",
			i+1, status, p.Expect, topSource, topScore, resp.Count, p.Query)
	}

	rate := float64(hits) / float64(len(probes))
	fmt.Printf("\nHit rate: %d/%d = %.0f%%  (semantic-layer hits: %d)\n",
		hits, len(probes), rate*100, semanticHits)
	if semanticHits == 0 && hits > 0 {
		fmt.Println("Note: all hits came from non-semantic layers — likely EMBED_BASE_URL is unset (keyword-only recall).")
	}
	if rate < *minRate {
		fail("hit rate %.0f%% below minimum %.0f%%", rate*100, *minRate*100)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "recall-eval: "+format+"\n", args...)
	os.Exit(1)
}
