package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"yunque-agent/pkg/jsonutil"
)

// ConflictDetector — finds contradictions between memories.
// Example: user says "我搬到上海了" but we stored "用户住在北京”.
//
// Resolution strategies: Overwrite (new wins), Merge (coexist with
// temporal annotation), KeepBoth (flag for review).
type Resolution string

const (
	ResOverwrite Resolution = "overwrite" // new supersedes old
	ResMerge     Resolution = "merge"     // coexist with annotation
	ResKeepBoth  Resolution = "keep_both" // ambiguous, needs review
)

type Conflict struct {
	Subject    string     `json:"subject"`     // the entity/topic
	OldFact    string     `json:"old_fact"`    // existing memory content
	OldSource  string     `json:"old_source"`  // which layer: "mid", "long", "editable"
	NewFact    string     `json:"new_fact"`    // incoming content
	Resolution Resolution `json:"resolution"`
	Confidence float64    `json:"confidence"`  // 0.0–1.0
	DetectedAt time.Time  `json:"detected_at"`
}

type LLMConflictFunc func(ctx context.Context, system, user string) (string, error)

type ConflictDetector struct {
	llmCall LLMConflictFunc // nil = use heuristic only
	embGate *embeddingGate  // nil = no embedding pre-filter
}

// NewConflictDetector: llmCall can be nil for heuristic-only mode.
func NewConflictDetector(llmCall LLMConflictFunc) *ConflictDetector {
	return &ConflictDetector{llmCall: llmCall}
}

func (d *ConflictDetector) DetectConflicts(
	ctx context.Context,
	newContent string,
	existing []RecallItem,
) []Conflict {
	if len(existing) == 0 || newContent == "" {
		return nil
	}

	// Stage 1: when an embedding gate is configured, pre-filter `existing`
	// down to only items that are semantically close to the new fact. This
	// kills false positives the keyword path generates (e.g. two unrelated
	// sentences that happen to share "改为" / "搬到") and gives the LLM a
	// tight candidate set instead of 10+ noisy items.
	//
	// On gate failure (embedder offline, partial embed error) we fall
	// through with the full existing list — degraded mode, never crashed.
	if d.embGate != nil {
		if filtered, ok := d.embGate.filterByEmbedding(ctx, newContent, existing); ok {
			if len(filtered) == 0 {
				// Nothing semantically similar → genuinely no conflict. The
				// keyword path would have returned [] anyway because it
				// requires both negation words AND overlap, but we short-
				// circuit here to save the downstream LLM token cost.
				return nil
			}
			existing = filtered
		}
	}

	// Stage 2: use LLM if available, else the keyword/negation heuristic.
	if d.llmCall != nil {
		return d.detectWithLLM(ctx, newContent, existing)
	}
	return d.detectHeuristic(newContent, existing)
}

// detectHeuristic: keyword-based contradiction check.
// Looks for negation words + entity overlap between old and new facts.
func (d *ConflictDetector) detectHeuristic(newContent string, existing []RecallItem) []Conflict {
	newLower := strings.ToLower(newContent)

	// Negation keywords that suggest contradiction
	negations := []string{
		"不再", "不是", "不喜欢", "换了", "搬到", "改为", "变成",
		"取消", "放弃", "改变", "不用", "停止", "不想",
		"no longer", "not anymore", "changed to", "moved to",
		"switched to", "stopped", "quit", "don't",
	}

	hasNegation := false
	for _, neg := range negations {
		if strings.Contains(newLower, neg) {
			hasNegation = true
			break
		}
	}
	if !hasNegation {
		return nil
	}

	var conflicts []Conflict
	for _, item := range existing {
		// Simple overlap: if old and new share significant words, may conflict
		overlap := d.wordOverlap(newLower, strings.ToLower(item.Content))
		if overlap >= 2 { // at least 2 shared meaningful words
			conflicts = append(conflicts, Conflict{
				Subject:    d.extractSubject(newContent, item.Content),
				OldFact:    item.Content,
				OldSource:  item.Source,
				NewFact:    newContent,
				Resolution: ResKeepBoth, // heuristic can't determine with certainty
				Confidence: 0.3 + float64(overlap)*0.1,
				DetectedAt: time.Now(),
			})
		}
	}
	return conflicts
}

func (d *ConflictDetector) detectWithLLM(ctx context.Context, newContent string, existing []RecallItem) []Conflict {
	// Build existing facts list (limit to top 10 by score to save tokens)
	var factList strings.Builder
	count := 0
	for _, item := range existing {
		if count >= 10 {
			break
		}
		factList.WriteString(fmt.Sprintf("- [%s] %s\n", item.Source, item.Content))
		count++
	}

	system := `你是记忆冲突检测器。比较"新事实"和"已有记忆"列表，找出矛盾。
输出JSON数组，每个元素: {"subject":"主题","old_fact":"旧事实","new_fact":"新事实","resolution":"overwrite/merge/keep_both","confidence":0.0-1.0}
规则:
- 如果新事实明确推翻旧事实（如搬家、改名、换工作），resolution="overwrite"，confidence>0.7
- 如果两个事实可以并存（如"喜欢苹果"和"也喜欢香蕉"），不算冲突
- 如果不确定是否冲突，resolution="keep_both"，confidence<0.5
- 如果没有冲突，返回空数组 []
只输出JSON，不要其他文字。`

	user := fmt.Sprintf("新事实:\n%s\n\n已有记忆:\n%s", newContent, factList.String())

	reply, err := d.llmCall(ctx, system, user)
	if err != nil {
		slog.Warn("conflict: llm call failed, falling back to heuristic", "err", err)
		return d.detectHeuristic(newContent, existing)
	}

	// Parse response
	jsonStr := jsonutil.ExtractArray(reply)
	var rawConflicts []struct {
		Subject    string  `json:"subject"`
		OldFact    string  `json:"old_fact"`
		NewFact    string  `json:"new_fact"`
		Resolution string  `json:"resolution"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &rawConflicts); err != nil {
		slog.Warn("conflict: parse failed", "err", err, "raw", reply)
		return nil
	}

	var conflicts []Conflict
	for _, rc := range rawConflicts {
		res := ResKeepBoth
		switch rc.Resolution {
		case "overwrite":
			res = ResOverwrite
		case "merge":
			res = ResMerge
		}

		// Find the source layer of the old fact
		oldSource := "unknown"
		for _, item := range existing {
			if strings.Contains(item.Content, rc.OldFact) || strings.Contains(rc.OldFact, item.Content) {
				oldSource = item.Source
				break
			}
		}

		conflicts = append(conflicts, Conflict{
			Subject:    rc.Subject,
			OldFact:    rc.OldFact,
			OldSource:  oldSource,
			NewFact:    rc.NewFact,
			Resolution: res,
			Confidence: rc.Confidence,
			DetectedAt: time.Now(),
		})
	}

	return conflicts
}

// ---- helpers ----

// wordOverlap counts shared words (>1 rune) between two lowercased strings.
func (d *ConflictDetector) wordOverlap(a, b string) int {
	wordsA := strings.Fields(a)
	wordSet := make(map[string]bool, len(wordsA))
	for _, w := range wordsA {
		if len([]rune(w)) > 1 { // skip single-char words
			wordSet[w] = true
		}
	}
	overlap := 0
	for _, w := range strings.Fields(b) {
		if wordSet[w] {
			overlap++
		}
	}
	return overlap
}

func (d *ConflictDetector) extractSubject(a, b string) string {
	wordsA := strings.Fields(a)
	wordSet := make(map[string]bool)
	for _, w := range wordsA {
		if len([]rune(w)) > 1 {
			wordSet[w] = true
		}
	}
	for _, w := range strings.Fields(b) {
		if wordSet[w] && len([]rune(w)) > 1 {
			return w
		}
	}
	return ""
}

