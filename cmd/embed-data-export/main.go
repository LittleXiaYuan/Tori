// Command embed-data-export builds contrastive training data for fine-tuning a
// Yunque-specialized embedding model.
//
// It collects memory facts (from the agent's daily memory files and/or a plain
// facts file), then for each fact generates paraphrased questions via an
// optional OpenAI-compatible LLM. Each (paraphrase_question, fact) pair becomes
// an (anchor, positive) training example; embedding fine-tuning (e.g.
// sentence-transformers MultipleNegativesRankingLoss) supplies negatives
// in-batch, so only positive pairs are needed.
//
// Usage:
//
//	# with an LLM to generate paraphrase positives (recommended):
//	go run ./cmd/embed-data-export -daily data/memory/daily \
//	    -llm-base https://api.example.com/v1 -llm-key sk-xxx -llm-model gpt-4o-mini \
//	    -out train.jsonl -eval eval.jsonl
//
//	# without an LLM: just extract+dedup facts to facts.jsonl
//	go run ./cmd/embed-data-export -daily data/memory/daily
//
// Outputs JSONL consumed by scripts/embedding/finetune.py.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type pair struct {
	Anchor   string `json:"anchor"`
	Positive string `json:"positive"`
}

func main() {
	var (
		dailyDir = flag.String("daily", "data/memory/daily", "dir of daily memory .md files")
		factsIn  = flag.String("facts", "", "optional extra file with one fact per line")
		out      = flag.String("out", "train.jsonl", "output training pairs JSONL")
		evalOut  = flag.String("eval", "eval.jsonl", "output held-out eval pairs JSONL")
		evalFrac = flag.Float64("eval-frac", 0.1, "fraction of pairs held out for eval")
		llmBase  = flag.String("llm-base", "", "OpenAI-compatible base URL for paraphrase generation")
		llmKey   = flag.String("llm-key", "", "LLM API key")
		llmModel = flag.String("llm-model", "", "LLM model for paraphrase generation")
		nPara    = flag.Int("paraphrases", 2, "paraphrase questions per fact")
		maxFacts = flag.Int("max", 0, "limit number of facts (0 = all)")
		seed     = flag.Int64("seed", 42, "shuffle seed for eval split")
	)
	flag.Parse()

	facts := collectFacts(*dailyDir, *factsIn)
	if *maxFacts > 0 && len(facts) > *maxFacts {
		facts = facts[:*maxFacts]
	}
	if len(facts) == 0 {
		fail("no facts found; populate data/memory/daily or pass -facts FILE")
	}
	fmt.Printf("Collected %d unique facts\n", len(facts))

	// Without an LLM we cannot synthesize meaningful query/positive pairs, so
	// just emit the deduped facts for the user to pair (or rerun with an LLM).
	if *llmBase == "" || *llmModel == "" {
		writeFacts("facts.jsonl", facts)
		fmt.Printf("No -llm-base/-llm-model set: wrote %d facts to facts.jsonl.\n", len(facts))
		fmt.Println("Re-run with -llm-base/-llm-key/-llm-model to generate (anchor,positive) pairs.")
		return
	}

	gen := &llm{base: strings.TrimRight(*llmBase, "/"), key: *llmKey, model: *llmModel, hc: &http.Client{Timeout: 60 * time.Second}}
	var pairs []pair
	for i, f := range facts {
		qs, err := gen.paraphrase(f, *nPara)
		if err != nil {
			fmt.Fprintf(os.Stderr, "paraphrase failed (fact %d): %v\n", i+1, err)
			continue
		}
		for _, q := range qs {
			q = strings.TrimSpace(q)
			if q != "" {
				pairs = append(pairs, pair{Anchor: q, Positive: f})
			}
		}
		if (i+1)%20 == 0 {
			fmt.Printf("  ...%d/%d facts\n", i+1, len(facts))
		}
	}
	if len(pairs) == 0 {
		fail("no pairs generated")
	}

	rng := rand.New(rand.NewSource(*seed))
	rng.Shuffle(len(pairs), func(i, j int) { pairs[i], pairs[j] = pairs[j], pairs[i] })
	nEval := int(float64(len(pairs)) * *evalFrac)
	evalPairs, trainPairs := pairs[:nEval], pairs[nEval:]

	writePairs(*out, trainPairs)
	writePairs(*evalOut, evalPairs)
	fmt.Printf("Wrote %d train + %d eval pairs (%s, %s)\n", len(trainPairs), len(evalPairs), *out, *evalOut)
}

// collectFacts reads facts from daily memory files and an optional facts file.
func collectFacts(dailyDir, factsFile string) []string {
	seen := map[string]bool{}
	var facts []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if len(s) < 4 || seen[s] {
			return
		}
		seen[s] = true
		facts = append(facts, s)
	}

	if dailyDir != "" {
		matches, _ := filepath.Glob(filepath.Join(dailyDir, "*.md"))
		for _, m := range matches {
			data, err := os.ReadFile(m)
			if err != nil {
				continue
			}
			for _, line := range strings.Split(string(data), "\n") {
				if f := parseDailyLine(line); f != "" {
					add(f)
				}
			}
		}
	}
	if factsFile != "" {
		data, err := os.ReadFile(factsFile)
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				add(line)
			}
		}
	}
	return facts
}

// parseDailyLine extracts the fact text from "- [HH:MM:SS][tenantID] fact".
func parseDailyLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "-")
	line = strings.TrimSpace(line)
	// Strip up to two leading [..] bracket groups.
	for i := 0; i < 2 && strings.HasPrefix(line, "["); i++ {
		if end := strings.IndexByte(line, ']'); end >= 0 {
			line = strings.TrimSpace(line[end+1:])
		}
	}
	return strings.TrimSpace(line)
}

// --- LLM paraphrase ---

type llm struct {
	base, key, model string
	hc               *http.Client
}

func (l *llm) paraphrase(fact string, n int) ([]string, error) {
	system := "你是一个数据标注助手。给定一条关于用户的记忆事实，生成用户可能会问出、且这条事实能回答的自然问题。" +
		"问题要口语化、措辞与事实不同（换种说法），不要直接重复事实里的词。每行一个问题，不要编号。"
	user := fmt.Sprintf("记忆事实：%s\n\n请生成 %d 个不同的问题：", fact, n)

	body, _ := json.Marshal(map[string]any{
		"model":       l.model,
		"messages":    []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}},
		"temperature": 0.8,
	})
	req, err := http.NewRequest(http.MethodPost, l.base+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if l.key != "" {
		req.Header.Set("Authorization", "Bearer "+l.key)
	}
	resp, err := l.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(data))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("empty LLM response")
	}
	var qs []string
	for _, line := range strings.Split(out.Choices[0].Message.Content, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "0123456789.、)-· ")
		if line != "" {
			qs = append(qs, line)
		}
	}
	return qs, nil
}

// --- output ---

func writePairs(path string, pairs []pair) {
	f, err := os.Create(path)
	if err != nil {
		fail("create %s: %v", path, err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()
	enc := json.NewEncoder(w)
	for _, p := range pairs {
		_ = enc.Encode(p)
	}
}

func writeFacts(path string, facts []string) {
	f, err := os.Create(path)
	if err != nil {
		fail("create %s: %v", path, err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()
	enc := json.NewEncoder(w)
	for _, fact := range facts {
		_ = enc.Encode(map[string]string{"text": fact})
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "embed-data-export: "+format+"\n", args...)
	os.Exit(1)
}
