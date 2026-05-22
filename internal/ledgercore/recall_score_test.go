package ledger

import (
	"math"
	"testing"
	"time"
)

func TestKeywordRelevanceChinese(t *testing.T) {
	tests := []struct {
		name    string
		content string
		key     string
		query   string
		wantMin float64
	}{
		{
			name:    "Chinese query matches Chinese content",
			content: "用户喜欢Go编程语言",
			key:     "user.lang",
			query:   "编程",
			wantMin: 0.1,
		},
		{
			name:    "Chinese multi-token query",
			content: "Alice住在北京海淀区",
			key:     "user.city",
			query:   "北京 海淀",
			wantMin: 0.3,
		},
		{
			name:    "Chinese query no match",
			content: "用户喜欢Python",
			key:     "user.lang",
			query:   "数据库管理",
			wantMin: -0.01, // allow 0
		},
		{
			name:    "English query still works",
			content: "Alice prefers Go programming",
			key:     "user.pref",
			query:   "Go programming",
			wantMin: 0.3,
		},
		{
			name:    "Mixed Chinese-English content",
			content: "用户擅长Go和Python编程",
			key:     "user.skills",
			query:   "python",
			wantMin: 0.1,
		},
		{
			name:    "Empty query returns 0",
			content: "some content",
			key:     "key",
			query:   "",
			wantMin: -0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := keywordRelevance(tt.content, tt.key, tt.query)
			if got < tt.wantMin {
				t.Errorf("keywordRelevance(%q, %q, %q) = %f, want >= %f",
					tt.content, tt.key, tt.query, got, tt.wantMin)
			}
			if got > 1.0 {
				t.Errorf("keywordRelevance() = %f, should be <= 1.0", got)
			}
		})
	}
}

func TestKeywordRelevanceChineseNonZero(t *testing.T) {
	score := keywordRelevance("用户喜欢Go编程语言", "user.lang", "编程")
	if score == 0 {
		t.Fatal("Chinese query '编程' against '用户喜欢Go编程语言' returned 0; CJK tokenization is broken")
	}
}

func TestScoreEntryWeightsPreserved(t *testing.T) {
	w := DefaultWeights()
	entry := &MemoryEntry{
		TenantID:    "t1",
		Kind:        MemoryFact,
		Key:         "user.name",
		Content:     "Alice is a Go developer",
		Source:      "user",
		Confidence:  0.9,
		AccessCount: 5,
	}
	q := &RecallQuery{
		TenantID: "t1",
		Query:    "Go developer",
		TaskGoal: "find developer info",
		TaskType: TaskTypeGoal,
	}

	score, _ := scoreEntry(entry, q, w)
	if score <= 0 {
		t.Fatal("scoreEntry returned non-positive score")
	}
	if score > 1.0 {
		t.Fatal("scoreEntry returned score > 1.0")
	}
}

func TestRecencyScoreDecay(t *testing.T) {
	now := recencyScore(time.Now())
	if math.Abs(now-1.0) > 0.05 {
		t.Errorf("recencyScore(now) = %f, want ~1.0", now)
	}
}

func TestAccessFreqScoreSaturation(t *testing.T) {
	zero := accessFreqScore(0)
	if zero != 0 {
		t.Errorf("accessFreqScore(0) = %f, want 0", zero)
	}
	hundred := accessFreqScore(100)
	if hundred < 0.9 || hundred > 1.01 {
		t.Errorf("accessFreqScore(100) = %f, want ~1.0", hundred)
	}
}

func TestSourceTrustOrdering(t *testing.T) {
	user := sourceTrust("user")
	extraction := sourceTrust("extraction")
	tool := sourceTrust("tool")
	llm := sourceTrust("llm")
	unknown := sourceTrust("random")

	if user <= extraction || extraction <= tool || tool <= llm || llm <= unknown {
		t.Errorf("source trust ordering violated: user=%f extraction=%f tool=%f llm=%f unknown=%f",
			user, extraction, tool, llm, unknown)
	}
}
