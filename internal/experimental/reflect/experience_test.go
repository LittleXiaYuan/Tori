package reflect

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExperienceStoreAddAndAll(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exp.json")
	s := NewExperienceStore(path)

	s.Add(Experience{
		Source:   "task",
		SourceID: "t-1",
		Category: "strategy",
		Outcome:  "success",
		Lesson:   "web_search 在网络搜索任务中表现良好",
		Context:  "搜索相关新闻",
	})
	s.Add(Experience{
		Source:   "interaction",
		SourceID: "sess-1",
		Category: "error_pattern",
		Outcome:  "failure",
		Lesson:   "translate 技能参数格式不正确",
		Context:  "翻译文档",
		Tags:     []string{"translate", "param_error"},
	})

	all := s.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 experiences, got %d", len(all))
	}
	// All returns newest first
	if all[0].Outcome != "failure" {
		t.Errorf("newest first: expected failure, got %s", all[0].Outcome)
	}
}

func TestExperienceStorePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exp.json")
	s := NewExperienceStore(path)
	s.Add(Experience{
		Source:  "task",
		Lesson:  "persistent lesson",
		Outcome: "success",
	})

	// Reload from disk
	s2 := NewExperienceStore(path)
	all := s2.All()
	if len(all) != 1 || all[0].Lesson != "persistent lesson" {
		t.Fatalf("persistence failed: got %v", all)
	}
}

func TestExperienceStoreSearch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exp.json")
	s := NewExperienceStore(path)
	s.Add(Experience{Lesson: "web_search works well", Tags: []string{"web"}})
	s.Add(Experience{Lesson: "translate needs fixing", Tags: []string{"translate"}})
	s.Add(Experience{Lesson: "code generation is fast", Tags: []string{"code"}})

	results := s.Search("translate", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'translate', got %d", len(results))
	}
	if results[0].Lesson != "translate needs fixing" {
		t.Errorf("wrong result: %s", results[0].Lesson)
	}

	// Search by tag
	results = s.Search("web", 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for tag 'web', got %d", len(results))
	}
}

func TestExperienceStoreStats(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exp.json")
	s := NewExperienceStore(path)
	s.Add(Experience{Source: "task", Category: "strategy", Outcome: "success"})
	s.Add(Experience{Source: "task", Category: "error_pattern", Outcome: "failure"})
	s.Add(Experience{Source: "interaction", Category: "strategy", Outcome: "success"})

	st := s.Stats()
	if st.Total != 3 {
		t.Fatalf("expected 3 total, got %d", st.Total)
	}
	if st.BySource["task"] != 2 {
		t.Errorf("expected 2 task source, got %d", st.BySource["task"])
	}
	if st.ByCategory["strategy"] != 2 {
		t.Errorf("expected 2 strategy, got %d", st.ByCategory["strategy"])
	}
	if st.ByOutcome["failure"] != 1 {
		t.Errorf("expected 1 failure, got %d", st.ByOutcome["failure"])
	}
	if st.Recent != 3 {
		t.Errorf("expected 3 recent, got %d", st.Recent)
	}
}

func TestCompileStrategies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exp.json")
	s := NewExperienceStore(path)
	s.Add(Experience{Category: "strategy", Outcome: "success", Lesson: "web_search 配合 summarize 效果好，可以先搜索再总结"})
	s.Add(Experience{Category: "strategy", Outcome: "partial", Lesson: "代码审查基本完成但需要补充测试验证和风险说明"})
	s.Add(Experience{Category: "error_pattern", Outcome: "failure", Lesson: "translate 对长文本容易超时，需要先分段再翻译"})

	strategies := s.CompileStrategies(10)
	if strategies == "" {
		t.Fatal("expected non-empty strategies")
	}
	if !contains(strategies, "推荐") {
		t.Error("missing success directive")
	}
	if !contains(strategies, "避免") {
		t.Error("missing failure directive")
	}
	if !contains(strategies, "改进") {
		t.Error("missing partial directive")
	}
	if !contains(strategies, "web_search") {
		t.Error("missing web_search strategy")
	}
}

func TestCompileStrategiesForQuery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exp.json")
	s := NewExperienceStore(path)
	s.Add(Experience{Category: "strategy", Outcome: "success", Lesson: "web_search 配合 summarize 效果好，可以先搜索再总结", Context: "搜索新闻"})
	s.Add(Experience{Category: "strategy", Outcome: "success", Lesson: "code review 需要先跑测试再总结风险", Context: "代码审查"})

	strategies := s.CompileStrategiesForQuery("请做 code review", 10)
	if strategies == "" {
		t.Fatal("expected query-specific strategies")
	}
	if !contains(strategies, "code review") {
		t.Fatalf("missing query-matching strategy: %s", strategies)
	}
	if contains(strategies, "web_search") {
		t.Fatalf("query-specific strategies leaked unrelated lesson: %s", strategies)
	}
}

func TestCompileStrategiesForQueryRanksHigherScoreBeforeRecency(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exp.json")
	s := NewExperienceStore(path)
	s.Add(Experience{Category: "strategy", Outcome: "success", Lesson: "code review 应先阅读 diff 再按风险分类给出结论", Context: "代码审查"})
	s.Add(Experience{Category: "strategy", Outcome: "success", Lesson: "review 前先运行 code tests 能减少遗漏但不是完整审查策略", Context: "测试验证"})

	strategies := s.CompileStrategiesForQuery("code review", 1)
	if !contains(strategies, "阅读 diff") {
		t.Fatalf("expected exact phrase match to outrank newer token match, got: %s", strategies)
	}
	if contains(strategies, "code tests") {
		t.Fatalf("expected limit to keep only the highest-scored strategy, got: %s", strategies)
	}
}

func TestMatchesQueryRequiresMultipleUsefulTokenHits(t *testing.T) {
	codeReview := Experience{
		Lesson:  "code review 需要先跑测试再总结风险",
		Context: "代码审查",
		Tags:    []string{"review", "test"},
	}
	webSearch := Experience{
		Lesson:  "web search 需要先确认来源时间再总结",
		Context: "搜索新闻",
		Tags:    []string{"search", "test"},
	}

	if !MatchesQuery(codeReview, "请做 code review") {
		t.Fatal("expected multi-token code review query to match")
	}
	if MatchesQuery(webSearch, "请做 code review") {
		t.Fatal("expected unrelated experience with no two useful token hits to be filtered out")
	}
	if !MatchesQuery(webSearch, "search") {
		t.Fatal("expected single-token query to keep exact lightweight search behavior")
	}
}

func TestCompileStrategiesEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exp.json")
	s := NewExperienceStore(path)
	if s.CompileStrategies(10) != "" {
		t.Error("expected empty strategies for empty store")
	}
}

func TestTaskReflectorRuleBased(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exp.json")
	store := NewExperienceStore(path)
	reflector := NewTaskReflector(nil, store) // no LLM client — rule-based only

	trace := TaskTrace{
		TaskID:      "task-1",
		Title:       "搜索新闻",
		Description: "搜索近期AI新闻并总结",
		Outcome:     "completed",
		Duration:    30 * time.Second,
		Steps: []StepTrace{
			{Action: "搜索新闻", SkillName: "web_search", Status: "done", Retries: 1},
			{Action: "总结内容", SkillName: "summarize", Status: "done"},
		},
	}

	reflector.AfterTask(context.Background(), trace)

	// Should have: 1 retry success + 1 task success
	all := store.All()
	if len(all) < 2 {
		t.Fatalf("expected at least 2 experiences, got %d", len(all))
	}

	// Check retry experience exists
	found := false
	for _, e := range all {
		if e.Category == "strategy" && contains(e.Lesson, "重试") {
			found = true
			break
		}
	}
	if !found {
		t.Error("missing retry strategy experience")
	}
}

func TestTaskReflectorGapExperience(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exp.json")
	store := NewExperienceStore(path)
	reflector := NewTaskReflector(nil, store)

	trace := TaskTrace{
		TaskID:  "task-2",
		Title:   "发送邮件",
		Outcome: "failed",
		Steps: []StepTrace{
			{Action: "发送邮件", SkillName: "email_send", Status: "failed", Error: "skill not found", GapType: "skill_missing"},
		},
	}

	reflector.AfterTask(context.Background(), trace)

	all := store.All()
	found := false
	for _, e := range all {
		if e.Category == "error_pattern" && contains(e.Lesson, "能力缺口") {
			found = true
			break
		}
	}
	if !found {
		t.Error("missing gap error pattern experience")
	}
}

func TestExperienceStoreMaxCap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exp.json")
	s := NewExperienceStore(path)
	for i := 0; i < 510; i++ {
		s.Add(Experience{Lesson: "lesson " + intToStr(i)})
	}
	all := s.All()
	if len(all) != 500 {
		t.Errorf("expected 500 max, got %d", len(all))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstr(s, substr)
}

func searchSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func init() {
	// Suppress file I/O errors in tests
	_ = os.MkdirAll("data", 0755)
}
