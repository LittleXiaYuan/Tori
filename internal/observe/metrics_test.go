package observe

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRecordRequest(t *testing.T) {
	m := New()
	m.RecordRequest(100*time.Millisecond, 50, 200, nil)
	m.RecordRequest(200*time.Millisecond, 30, 100, fmt.Errorf("timeout"))

	if m.RequestsTotal.Load() != 2 {
		t.Fatalf("expected 2 total, got %d", m.RequestsTotal.Load())
	}
	if m.RequestsSuccess.Load() != 1 {
		t.Fatalf("expected 1 success, got %d", m.RequestsSuccess.Load())
	}
	if m.RequestsFailed.Load() != 1 {
		t.Fatalf("expected 1 failed, got %d", m.RequestsFailed.Load())
	}
	if m.TokensIn.Load() != 80 {
		t.Fatalf("expected 80 tokens in, got %d", m.TokensIn.Load())
	}
	if m.TokensOut.Load() != 300 {
		t.Fatalf("expected 300 tokens out, got %d", m.TokensOut.Load())
	}
}

func TestRecordSkillCall(t *testing.T) {
	m := New()
	m.RecordSkillCall("web_search", 50*time.Millisecond, nil)
	m.RecordSkillCall("web_search", 80*time.Millisecond, nil)
	m.RecordSkillCall("web_search", 200*time.Millisecond, fmt.Errorf("network error"))
	m.RecordSkillCall("code_exec", 500*time.Millisecond, nil)

	snap := m.Snapshot()
	if len(snap.Skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(snap.Skills))
	}

	// Find web_search
	var ws *SkillSnapshot
	for i := range snap.Skills {
		if snap.Skills[i].Name == "web_search" {
			ws = &snap.Skills[i]
		}
	}
	if ws == nil {
		t.Fatal("web_search not found in snapshot")
	}
	if ws.Total != 3 || ws.Success != 2 || ws.Failed != 1 {
		t.Fatalf("web_search counts wrong: %+v", ws)
	}
	if ws.SuccessRate < 0.6 || ws.SuccessRate > 0.7 {
		t.Fatalf("expected ~0.667 success rate, got %.3f", ws.SuccessRate)
	}
}

func TestLatencyStats(t *testing.T) {
	h := newHistogramStore()
	for i := 1; i <= 100; i++ {
		h.Record("test", time.Duration(i)*time.Millisecond)
	}
	stats := h.Stats("test")
	if stats.Count != 100 {
		t.Fatalf("expected 100 count, got %d", stats.Count)
	}
	if stats.P50 < 45 || stats.P50 > 55 {
		t.Fatalf("P50 should be ~50ms, got %.1f", stats.P50)
	}
	if stats.P95 < 90 || stats.P95 > 100 {
		t.Fatalf("P95 should be ~95ms, got %.1f", stats.P95)
	}
	if stats.Max < 99 || stats.Max > 101 {
		t.Fatalf("Max should be ~100ms, got %.1f", stats.Max)
	}
}

func TestErrorTracker(t *testing.T) {
	m := New()
	m.RecordRequest(10*time.Millisecond, 0, 0, fmt.Errorf("error A"))
	m.RecordRequest(10*time.Millisecond, 0, 0, fmt.Errorf("error A"))
	m.RecordRequest(10*time.Millisecond, 0, 0, fmt.Errorf("error B"))

	snap := m.Snapshot()
	if len(snap.RecentErrors) != 2 {
		t.Fatalf("expected 2 unique errors, got %d", len(snap.RecentErrors))
	}
	for _, e := range snap.RecentErrors {
		if e.Message == "error A" && e.Count != 2 {
			t.Fatalf("expected error A count=2, got %d", e.Count)
		}
	}
}

func TestPrometheusFormat(t *testing.T) {
	m := New()
	m.RecordRequest(100*time.Millisecond, 50, 200, nil)
	m.RecordSkillCall("web_search", 50*time.Millisecond, nil)

	prom := m.PrometheusFormat()
	if !strings.Contains(prom, "yunque_requests_total 1") {
		t.Fatal("missing requests_total in prometheus output")
	}
	if !strings.Contains(prom, "yunque_tokens_in 50") {
		t.Fatal("missing tokens_in in prometheus output")
	}
	if !strings.Contains(prom, "yunque_skill_calls_total") {
		t.Fatal("missing skill metrics in prometheus output")
	}
}

func TestSnapshotJSON(t *testing.T) {
	m := New()
	m.RecordRequest(100*time.Millisecond, 50, 200, nil)
	snap := m.Snapshot()
	if snap.Uptime < 0 {
		t.Fatal("uptime should not be negative")
	}
	if snap.TokensTotal != 250 {
		t.Fatalf("expected tokens_total=250, got %d", snap.TokensTotal)
	}
}

func TestChannelMetrics(t *testing.T) {
	m := New()
	m.RecordChannelMessage("telegram")
	m.RecordChannelMessage("telegram")
	m.RecordChannelMessage("discord")
	m.RecordChannelSend("telegram", nil)
	m.RecordChannelSend("telegram", fmt.Errorf("send failed"))
	m.RecordChannelSend("discord", nil)

	snap := m.Snapshot()
	if len(snap.Channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(snap.Channels))
	}

	var tg *ChannelSnapshot
	for i := range snap.Channels {
		if snap.Channels[i].Channel == "telegram" {
			tg = &snap.Channels[i]
		}
	}
	if tg == nil {
		t.Fatal("telegram channel not in snapshot")
	}
	if tg.MessagesIn != 2 {
		t.Fatalf("expected 2 messages in, got %d", tg.MessagesIn)
	}
	if tg.SendTotal != 2 {
		t.Fatalf("expected 2 send total, got %d", tg.SendTotal)
	}
	if tg.SendFailed != 1 {
		t.Fatalf("expected 1 send failed, got %d", tg.SendFailed)
	}
}

func TestChannelMetricsPrometheus(t *testing.T) {
	m := New()
	m.RecordChannelMessage("line")
	m.RecordChannelSend("line", nil)

	prom := m.PrometheusFormat()
	if !strings.Contains(prom, "yunque_channel_messages_in") {
		t.Fatal("missing channel messages_in in prometheus output")
	}
	if !strings.Contains(prom, "yunque_channel_send_total") {
		t.Fatal("missing channel send_total in prometheus output")
	}
}

func TestKnowledgeMetrics(t *testing.T) {
	m := New()
	m.RecordKnowledgeSearch("bm25", 10*time.Millisecond, 5)
	m.RecordKnowledgeSearch("hybrid", 50*time.Millisecond, 10)
	m.RecordKnowledgeSearch("bm25", 15*time.Millisecond, 3)
	m.RecordRerank("jina", 30*time.Millisecond, nil)
	m.RecordRerank("jina", 40*time.Millisecond, fmt.Errorf("api error"))

	snap := m.Snapshot()
	if snap.Knowledge.Searches["bm25"] != 2 {
		t.Fatalf("expected 2 bm25 searches, got %d", snap.Knowledge.Searches["bm25"])
	}
	if snap.Knowledge.Searches["hybrid"] != 1 {
		t.Fatalf("expected 1 hybrid search, got %d", snap.Knowledge.Searches["hybrid"])
	}
	if snap.Knowledge.TotalResults != 18 {
		t.Fatalf("expected 18 total results, got %d", snap.Knowledge.TotalResults)
	}
	if snap.Knowledge.RerankTotal["jina"] != 2 {
		t.Fatalf("expected 2 jina reranks, got %d", snap.Knowledge.RerankTotal["jina"])
	}
	if snap.Knowledge.RerankFailed["jina"] != 1 {
		t.Fatalf("expected 1 jina rerank failed, got %d", snap.Knowledge.RerankFailed["jina"])
	}

	// Verify latency stats exist
	if snap.Knowledge.SearchLatency["bm25"].Count != 2 {
		t.Fatalf("expected 2 bm25 latency records, got %d", snap.Knowledge.SearchLatency["bm25"].Count)
	}
	if snap.Knowledge.RerankLatency["jina"].Count != 2 {
		t.Fatalf("expected 2 jina rerank latency records, got %d", snap.Knowledge.RerankLatency["jina"].Count)
	}
}

func TestKnowledgeMetricsPrometheus(t *testing.T) {
	m := New()
	m.RecordKnowledgeSearch("hybrid", 20*time.Millisecond, 5)
	m.RecordRerank("cohere", 30*time.Millisecond, nil)

	prom := m.PrometheusFormat()
	if !strings.Contains(prom, "yunque_knowledge_search_total") {
		t.Fatal("missing knowledge search_total in prometheus output")
	}
	if !strings.Contains(prom, "yunque_knowledge_results_total") {
		t.Fatal("missing knowledge results_total in prometheus output")
	}
	if !strings.Contains(prom, "yunque_rerank_total") {
		t.Fatal("missing rerank_total in prometheus output")
	}
}

func TestChannelMetricsConcurrency(t *testing.T) {
	m := New()
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				m.RecordChannelMessage("telegram")
				m.RecordChannelSend("telegram", nil)
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	snap := m.Snapshot()
	var tg *ChannelSnapshot
	for i := range snap.Channels {
		if snap.Channels[i].Channel == "telegram" {
			tg = &snap.Channels[i]
		}
	}
	if tg == nil || tg.MessagesIn != 1000 || tg.SendTotal != 1000 {
		t.Fatalf("expected 1000/1000, got %+v", tg)
	}
}
