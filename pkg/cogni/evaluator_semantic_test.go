package cogni

import (
	"strings"
	"testing"
)

// fakeEmbedder maps text to a 2-axis [doc, img] concept vector so cosine
// similarity is deterministic and easy to reason about in tests.
func fakeEmbedder(text string) []float32 {
	var doc, img float32
	for _, kw := range []string{"汇报", "幻灯", "文档", "材料", "册子", "报表"} {
		if strings.Contains(text, kw) {
			doc = 1
		}
	}
	for _, kw := range []string{"图", "画", "视觉", "海报", "插画"} {
		if strings.Contains(text, kw) {
			img = 1
		}
	}
	if doc == 0 && img == 0 {
		return []float32{0.2, 0.2}
	}
	return []float32{doc, img}
}

func docCogni() *Declaration {
	return &Declaration{
		ID: "office",
		Activation: ActivationRules{
			MinScore: 0.3,
			// no keywords: only semantic can activate it
			Semantic: &SemanticActivation{
				Examples: []string{"帮我做一份汇报材料", "把内容整理成展示册子"},
				Weight:   0.5,
				Floor:    0.55,
			},
		},
	}
}

// A paraphrase with NO declared keyword still activates via semantic similarity.
func TestSemantic_ParaphraseActivates(t *testing.T) {
	e := NewEvaluator()
	e.SetEmbedder(fakeEmbedder)

	sess := Session{Message: "帮我把内容做成幻灯", MessageVec: fakeEmbedder("帮我把内容做成幻灯")}
	got := e.Evaluate([]*Declaration{docCogni()}, sess)
	if len(got) != 1 || !got[0].Activated {
		t.Fatalf("doc-semantic message should activate office cogni; got %+v", got)
	}
}

// Without a message vector (no embedder wired upstream) semantic scoring is
// skipped entirely, preserving keyword-only behaviour.
func TestSemantic_NoVectorFallsBack(t *testing.T) {
	e := NewEvaluator()
	e.SetEmbedder(fakeEmbedder)

	sess := Session{Message: "帮我把内容做成幻灯"} // MessageVec deliberately nil
	got := e.Evaluate([]*Declaration{docCogni()}, sess)
	if len(got) != 1 || got[0].Activated {
		t.Fatalf("without message vector semantic must not activate; got %+v", got)
	}
}

// An orthogonal (image) message stays below the floor and does not activate the
// document cogni — the floor prevents "everything sort of matches" drift.
func TestSemantic_OrthogonalDoesNotActivate(t *testing.T) {
	e := NewEvaluator()
	e.SetEmbedder(fakeEmbedder)

	sess := Session{Message: "画一张海报", MessageVec: fakeEmbedder("画一张海报")}
	got := e.Evaluate([]*Declaration{docCogni()}, sess)
	if len(got) != 1 || got[0].Activated {
		t.Fatalf("orthogonal image message must not activate office cogni; got %+v", got)
	}
}

// Semantic adds to (does not replace) keyword scoring.
func TestSemantic_AddsToKeyword(t *testing.T) {
	e := NewEvaluator()
	e.SetEmbedder(fakeEmbedder)

	d := docCogni()
	d.Activation.Keywords = []string{"PPT"}
	d.Activation.KeywordWeight = 0.2
	d.Activation.MinScore = 0.6 // neither keyword(0.2) nor semantic(0.5) alone crosses

	msg := "用PPT帮我做一份汇报" // keyword 0.2 + semantic ~0.5 = ~0.7 ≥ 0.6
	sess := Session{Message: msg, MessageVec: fakeEmbedder(msg)}
	got := e.Evaluate([]*Declaration{d}, sess)
	if len(got) != 1 || !got[0].Activated {
		t.Fatalf("keyword+semantic should together cross MinScore; got score=%.2f reasons=%v", got[0].Score, got[0].Reasons)
	}
}
