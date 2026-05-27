package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	ledger "yunque-agent/internal/ledgercore"
	lsqlite "yunque-agent/internal/ledgercore/backend/sqlite"
)

type latencyStats struct {
	Runs int     `json:"runs"`
	Avg  float64 `json:"avg_ms"`
	P50  float64 `json:"p50_ms"`
	P95  float64 `json:"p95_ms"`
	Max  float64 `json:"max_ms"`
}

type report struct {
	GeneratedAt time.Time    `json:"generated_at"`
	GoVersion   string       `json:"go_version"`
	MemoryRows  int          `json:"memory_rows"`
	SearchRuns  int          `json:"search_runs"`
	InsertMS    float64      `json:"insert_ms"`
	Search      latencyStats `json:"sqlite_memory_search"`
	DBPath      string       `json:"db_path"`
}

func main() {
	rows := flag.Int("memory-rows", 1000, "number of Ledger memory rows to insert")
	runs := flag.Int("search-runs", 20, "number of memory search runs")
	out := flag.String("out", "", "optional JSON output path")
	flag.Parse()

	if *rows <= 0 {
		fatalf("memory-rows must be > 0")
	}
	if *runs <= 0 {
		fatalf("search-runs must be > 0")
	}

	tmp, err := os.MkdirTemp("", "yunque-perf-ledger-*")
	if err != nil {
		fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tmp)

	dbPath := filepath.Join(tmp, "ledger.db")
	backend, err := lsqlite.New(dbPath)
	if err != nil {
		fatalf("sqlite open: %v", err)
	}
	ldg, err := ledger.Open(backend)
	if err != nil {
		fatalf("ledger open: %v", err)
	}
	defer ldg.Close()

	ctx := context.Background()
	insertStart := time.Now()
	for i := 0; i < *rows; i++ {
		kind := ledger.MemoryFact
		if i%5 == 0 {
			kind = ledger.MemoryExperience
		}
		if err := ldg.Memory.Put(ctx, &ledger.MemoryEntry{
			TenantID:   "perf",
			Kind:       kind,
			Key:        fmt.Sprintf("perf.memory.%06d", i),
			Content:    fmt.Sprintf("云雀 performance baseline memory row %06d. browser rpa ledger recall sqlite query latency sample.", i),
			Source:     "perf-baseline",
			Confidence: 0.8,
		}); err != nil {
			fatalf("insert memory %d: %v", i, err)
		}
	}
	insertMS := msSince(insertStart)

	latencies := make([]float64, 0, *runs)
	for i := 0; i < *runs; i++ {
		start := time.Now()
		results, err := ldg.Memory.Search(ctx, ledger.MemoryQuery{
			TenantID: "perf",
			Query:    "browser rpa ledger recall",
			Limit:    20,
		})
		if err != nil {
			fatalf("search run %d: %v", i, err)
		}
		if len(results) == 0 {
			fatalf("search run %d returned no results", i)
		}
		latencies = append(latencies, msSince(start))
	}

	rep := report{
		GeneratedAt: time.Now(),
		GoVersion:   runtime.Version(),
		MemoryRows:  *rows,
		SearchRuns:  *runs,
		InsertMS:    round(insertMS),
		Search:      summarize(latencies),
		DBPath:      dbPath,
	}

	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		fatalf("marshal report: %v", err)
	}
	if *out != "" {
		if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
			fatalf("mkdir out: %v", err)
		}
		if err := os.WriteFile(*out, data, 0o644); err != nil {
			fatalf("write report: %v", err)
		}
	}
	fmt.Println(string(data))
}

func summarize(values []float64) latencyStats {
	sort.Float64s(values)
	var sum float64
	var max float64
	for _, v := range values {
		sum += v
		if v > max {
			max = v
		}
	}
	return latencyStats{
		Runs: len(values),
		Avg:  round(sum / float64(len(values))),
		P50:  percentile(values, 50),
		P95:  percentile(values, 95),
		Max:  round(max),
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int((p/100.0)*float64(len(sorted)-1) + 0.5)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return round(sorted[idx])
}

func msSince(start time.Time) float64 {
	return float64(time.Since(start).Microseconds()) / 1000.0
}

func round(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
