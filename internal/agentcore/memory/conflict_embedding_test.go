package memory

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

// fakeEmbed synthesises deterministic unit vectors so we can test the
// cosine math without hitting a real provider. Each string gets projected
// onto a 4-dimensional axis by hashing the first character into a bucket;
// identical characters yield identical vectors, so conceptually-related
// inputs (same seed char) produce cos=1, unrelated inputs yield 0 by
// construction.
func fakeEmbed(seed byte, axis int) SingleEmbedFunc {
	return func(_ context.Context, text string) ([]float32, error) {
		// Special testing hooks:
		//  - text starting with "!err:" returns an error (to assert graceful
		//    degradation)
		//  - text starting with "!nil:" returns (nil, nil) (empty vector)
		if len(text) >= 5 && text[:5] == "!err:" {
			return nil, errors.New("embed provider offline")
		}
		if len(text) >= 5 && text[:5] == "!nil:" {
			return nil, nil
		}
		vec := make([]float32, 4)
		ax := axis
		if len(text) > 0 && text[0] == seed {
			// "similar" axis → axis 0
			ax = 0
		}
		vec[ax] = 1
		return vec, nil
	}
}

// simpleEmbed maps the first rune into a 4-dim basis vector, so strings
// starting with the same character are cos=1 similar, different starting
// characters are cos=0. This gives a clean signal for threshold tests.
//
// Special testing prefixes mirror fakeEmbed's so we can drive the error
// paths without a real provider:
//   - "!err:…" returns a non-nil error (embed provider failure)
//   - "!nil:…" returns (nil, nil)       (empty vector)
func simpleEmbed(_ context.Context, text string) ([]float32, error) {
	if len(text) >= 5 && text[:5] == "!err:" {
		return nil, errors.New("embed provider offline")
	}
	if len(text) >= 5 && text[:5] == "!nil:" {
		return nil, nil
	}
	vec := make([]float32, 4)
	if len(text) == 0 {
		return vec, nil
	}
	vec[int(text[0])%4] = 1
	return vec, nil
}

func mustConflictDetectorWithGate(t *testing.T, embed SingleEmbedFunc, cfg EmbeddingGateConfig) *ConflictDetector {
	t.Helper()
	// We do not pass an LLM. Without an LLM, DetectConflicts falls back to
	// the heuristic arbiter — which is exactly what we want here because
	// we're testing the gate's ability to trim the candidate set, not the
	// arbiter's final classification.
	d := NewConflictDetector(nil)
	d.SetEmbeddingGate(embed, cfg)
	return d
}

func TestEmbeddingGate_DisabledWhenNotConfigured(t *testing.T) {
	d := NewConflictDetector(nil) // no gate
	if d.embGate != nil {
		t.Fatalf("expected nil embedding gate")
	}
	// detect should still run the heuristic path cleanly.
	newContent := "我搬到深圳了"
	existing := []RecallItem{{Content: "用户住在北京", Source: "mid"}}
	conflicts := d.DetectConflicts(context.Background(), newContent, existing)
	_ = conflicts // heuristic behavior isn't under test; just ensure no panic / no crash
}

func TestEmbeddingGate_KeepsOnlySimilarCandidates(t *testing.T) {
	cfg := DefaultEmbeddingGateConfig()
	cfg.Threshold = 0.5 // loose for deterministic bucketing
	cfg.MaxCandidates = 10
	d := mustConflictDetectorWithGate(t, simpleEmbed, cfg)

	// "a_new" and "a_old" share the same first char → cos=1, pass the gate.
	// "z_noise" has a different first char → cos=0, filtered out.
	newContent := "a_new_fact_user_moved_to_sz"
	existing := []RecallItem{
		{Content: "a_old_user_lives_in_bj", Source: "mid"},
		{Content: "z_noise_totally_unrelated", Source: "long"},
		{Content: "a_other_related_context", Source: "mid"},
	}

	// Access the gate directly to test filtering semantics without caring
	// about the heuristic's final classification.
	filtered, ran := d.embGate.filterByEmbedding(context.Background(), newContent, existing)
	if !ran {
		t.Fatalf("expected gate to run")
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2 similar candidates, got %d: %+v", len(filtered), filtered)
	}
	// The noise item must not be present.
	for _, f := range filtered {
		if f.Content == "z_noise_totally_unrelated" {
			t.Fatalf("noise item leaked past the gate: %+v", f)
		}
	}
}

func TestEmbeddingGate_CapsByMaxCandidates(t *testing.T) {
	cfg := DefaultEmbeddingGateConfig()
	cfg.Threshold = 0.0 // accept everything, so the cap is what limits us
	cfg.MaxCandidates = 2
	d := mustConflictDetectorWithGate(t, simpleEmbed, cfg)

	newContent := "a_new"
	existing := []RecallItem{
		{Content: "a1"}, {Content: "a2"}, {Content: "a3"}, {Content: "a4"},
	}
	filtered, ran := d.embGate.filterByEmbedding(context.Background(), newContent, existing)
	if !ran {
		t.Fatalf("expected gate to run")
	}
	if len(filtered) != 2 {
		t.Fatalf("cap not applied: got %d items", len(filtered))
	}
}

func TestEmbeddingGate_DegradesGracefullyOnEmbedError(t *testing.T) {
	cfg := DefaultEmbeddingGateConfig()
	d := mustConflictDetectorWithGate(t, simpleEmbed, cfg)

	// When the new content itself fails to embed, the gate must surface
	// `ran=false` and return `existing` untouched so the caller runs the
	// heuristic path as if the gate never existed.
	filtered, ran := d.embGate.filterByEmbedding(context.Background(), "!err:fail", []RecallItem{{Content: "x"}})
	if ran {
		t.Fatalf("expected ran=false on new-content embed failure")
	}
	if len(filtered) != 1 {
		t.Fatalf("expected existing items to be passed through untouched")
	}

	// When *some* existing items fail to embed, the gate runs but drops
	// only those specific items rather than failing the entire call.
	newContent := "a_new"
	existing := []RecallItem{
		{Content: "a_related"},   // embeds fine, similar
		{Content: "!err:broken"}, // embed error, should be dropped
		{Content: "a_also_ok"},   // embeds fine, similar
	}
	filtered, ran = d.embGate.filterByEmbedding(context.Background(), newContent, existing)
	if !ran {
		t.Fatalf("expected gate to run even when some items fail")
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2 surviving items, got %d", len(filtered))
	}
}

func TestEmbeddingGate_CacheEvictsOldest(t *testing.T) {
	cfg := DefaultEmbeddingGateConfig()
	cfg.NewContentCacheTTL = 1 * time.Second
	d := mustConflictDetectorWithGate(t, simpleEmbed, cfg)
	g := d.embGate

	// Prime with 513 entries → one beyond the internal 512 cap → the
	// oldest should have been evicted by the time we're done.
	ctx := context.Background()
	for i := 0; i < 513; i++ {
		_, err := g.embedCached(ctx, "key-"+string(rune('a'+i%26))+"-"+string(rune('A'+i%26))+"-"+intStr(i))
		if err != nil {
			t.Fatalf("unexpected embed error at %d: %v", i, err)
		}
	}
	g.mu.Lock()
	size := len(g.recentCache)
	g.mu.Unlock()
	if size > 512 {
		t.Fatalf("cache grew past the 512 cap: size=%d", size)
	}
}

func TestCosineSimilarity32_OrthogonalIsZero(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	got := cosineSimilarity32(a, b)
	if got != 0 {
		t.Fatalf("orthogonal vectors should score 0, got %v", got)
	}
}

func TestCosineSimilarity32_IdenticalIsOne(t *testing.T) {
	a := []float32{0.5, 0.5, 0.5, 0.5}
	got := cosineSimilarity32(a, a)
	if math.Abs(got-1.0) > 1e-9 {
		t.Fatalf("identical vectors should score 1.0, got %v", got)
	}
}

func TestCosineSimilarity32_LengthMismatchIsZero(t *testing.T) {
	if cosineSimilarity32([]float32{1, 2}, []float32{1, 2, 3}) != 0 {
		t.Fatalf("mismatched lengths must return 0 (not panic)")
	}
	if cosineSimilarity32(nil, nil) != 0 {
		t.Fatalf("empty/nil vectors must return 0 (not NaN)")
	}
}

// intStr is a trivial itoa used in the cache-eviction test to build
// unique keys without pulling in strconv. Keeping it local avoids a
// test-only import and keeps the compile list stable.
func intStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := make([]byte, 0, 8)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}

// silence unused warning for the `fakeEmbed` builder in case we reintroduce
// more scenarios using it later. It's kept because the multi-axis version
// is useful for PR reviewers writing quick regression tests without the
// simpleEmbed bucketing collisions.
var _ = fakeEmbed(0, 0)
