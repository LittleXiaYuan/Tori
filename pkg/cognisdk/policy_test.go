package cognisdk

import "testing"

// Category is an independent classification channel from Intent — it must
// never disturb the existing general/work_task/seek_reassurance Intent
// buckets that Cogni Declarations already key IntentMatch rules on.
func TestDetectPerceptionCategoryClassification(t *testing.T) {
	cases := []struct {
		name         string
		message      string
		wantCategory string
	}{
		{"coding keyword", "帮我修复这个 bug，代码报错了", "coding"},
		{"coding english", "please implement and fix the test", "coding"},
		{"writing keyword", "帮我写一篇文案，润色一下大纲", "writing"},
		{"writing english", "please draft and summarize this article", "writing"},
		{"research keyword", "帮我调研一下最新的行业资料，做个对比分析", "research"},
		{"research english", "please research and compare these options", "research"},
		{"no category", "你好呀", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := detectPerception(Input{Message: tc.message})
			if p.Category != tc.wantCategory {
				t.Fatalf("detectPerception(%q).Category = %q, want %q", tc.message, p.Category, tc.wantCategory)
			}
		})
	}
}

// Category must be purely additive: existing Intent classification (general/
// work_task/seek_reassurance) is untouched by the new classifier.
func TestDetectPerceptionCategoryDoesNotAffectIntent(t *testing.T) {
	cases := []struct {
		message    string
		wantIntent string
	}{
		{"帮我修复这个 bug", "work_task"},
		{"永远陪着我好不好", "seek_reassurance"},
		{"你好呀", "general"},
	}
	for _, tc := range cases {
		p := detectPerception(Input{Message: tc.message})
		if p.Intent != tc.wantIntent {
			t.Fatalf("detectPerception(%q).Intent = %q, want %q (Category=%q)", tc.message, p.Intent, tc.wantIntent, p.Category)
		}
	}
}
