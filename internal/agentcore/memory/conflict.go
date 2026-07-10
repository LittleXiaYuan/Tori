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
	return d.DetectConflictsForTenant(ctx, "", newContent, existing)
}

// DetectConflictsForTenant is tenant-aware: it uses the per-tenant embedding
// cache to avoid re-embedding stored memories on every call.
func (d *ConflictDetector) DetectConflictsForTenant(
	ctx context.Context,
	tenantID string,
	newContent string,
	existing []RecallItem,
) []Conflict {
	if len(existing) == 0 || newContent == "" {
		return nil
	}

	if d.embGate != nil {
		if filtered, ok := d.embGate.filterByEmbeddingForTenant(ctx, tenantID, newContent, existing); ok {
			if len(filtered) == 0 {
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

// detectHeuristic uses Jaccard similarity + normalized Levenshtein distance
// to find contradictions between new and existing memories. Combined with
// negation-word detection for directional signals.
//
// Confidence = 0.4×Jaccard + 0.3×(1-NormEditDist) + 0.3×NegationBoost
func (d *ConflictDetector) detectHeuristic(newContent string, existing []RecallItem) []Conflict {
	newLower := strings.ToLower(newContent)
	newWords := significantWords(newLower)

	negationBoost := 0.0
	negations := []string{
		"不再", "不是", "不喜欢", "换了", "搬到", "改为", "变成",
		"取消", "放弃", "改变", "不用", "停止", "不想",
		"no longer", "not anymore", "changed to", "moved to",
		"switched to", "stopped", "quit", "don't",
	}
	for _, neg := range negations {
		if strings.Contains(newLower, neg) {
			negationBoost = 1.0
			break
		}
	}

	if negationBoost == 0 {
		return nil
	}

	var conflicts []Conflict
	for _, item := range existing {
		oldLower := strings.ToLower(item.Content)
		oldWords := significantWords(oldLower)

		jaccard := jaccardSimilarity(newWords, oldWords)
		normEdit := normalizedLevenshtein(newLower, oldLower)

		confidence := 0.4*jaccard + 0.3*(1.0-normEdit) + 0.3*negationBoost

		if confidence >= 0.35 && jaccard >= 0.1 {
			conflicts = append(conflicts, Conflict{
				Subject:    d.extractSubject(newContent, item.Content),
				OldFact:    item.Content,
				OldSource:  item.Source,
				NewFact:    newContent,
				Resolution: ResKeepBoth,
				Confidence: confidence,
				DetectedAt: time.Now(),
			})
		}
	}
	return conflicts
}

// jaccardSimilarity computes J(A,B) = |A∩B| / |A∪B| for word sets.
func jaccardSimilarity(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	setA := make(map[string]bool, len(a))
	for _, w := range a {
		setA[w] = true
	}
	setB := make(map[string]bool, len(b))
	for _, w := range b {
		setB[w] = true
	}

	intersection := 0
	for w := range setA {
		if setB[w] {
			intersection++
		}
	}
	union := len(setA)
	for w := range setB {
		if !setA[w] {
			union++
		}
	}
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// normalizedLevenshtein returns edit distance / max(len(a), len(b)).
// Result in [0, 1] where 0 = identical, 1 = completely different.
// Operates on rune slices for CJK correctness.
func normalizedLevenshtein(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)
	if la == 0 && lb == 0 {
		return 0
	}

	maxLen := la
	if lb > maxLen {
		maxLen = lb
	}

	// Optimize: if length difference alone exceeds threshold, skip full DP
	if la > 200 || lb > 200 {
		lenDiff := la - lb
		if lenDiff < 0 {
			lenDiff = -lenDiff
		}
		return float64(lenDiff) / float64(maxLen)
	}

	// Standard DP with two-row optimization
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			curr[j] = del
			if ins < curr[j] {
				curr[j] = ins
			}
			if sub < curr[j] {
				curr[j] = sub
			}
		}
		prev, curr = curr, prev
	}
	return float64(prev[lb]) / float64(maxLen)
}

// significantWords extracts words with >1 rune, filtering out common stop words.
func significantWords(s string) []string {
	fields := strings.Fields(s)
	var out []string
	for _, w := range fields {
		if len([]rune(w)) > 1 {
			out = append(out, w)
		}
	}
	return out
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

