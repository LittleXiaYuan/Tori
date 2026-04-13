package quality

import (
	"testing"
)

func TestBasicQualityScoring(t *testing.T) {
	s := NewScorer()

	score := s.Evaluate(
		"What is machine learning?",
		"Machine learning is a subset of artificial intelligence that enables systems to learn and improve from experience without being explicitly programmed.",
	)

	if score.Overall <= 0.4 {
		t.Errorf("good answer should score above 0.4, got %.3f", score.Overall)
	}
	if !score.Satisfied {
		t.Error("good answer should be satisfied")
	}
}

func TestEmptyReply(t *testing.T) {
	s := NewScorer()
	score := s.Evaluate("What is AI?", "")
	if score.Satisfied {
		t.Error("empty reply should not be satisfied")
	}
	if score.Overall != 0 {
		t.Errorf("empty reply should score 0, got %.3f", score.Overall)
	}
}

func TestShortReplyPenalty(t *testing.T) {
	s := NewScorer()
	shortScore := s.Evaluate("Explain quantum computing in detail", "It's complicated.")
	longScore := s.Evaluate("Explain quantum computing in detail",
		"Quantum computing uses quantum bits or qubits that can exist in superposition states, "+
			"enabling parallel computation. Key concepts include entanglement, interference, and decoherence. "+
			"Applications include cryptography, drug discovery, and optimization problems.")

	if shortScore.Overall >= longScore.Overall {
		t.Errorf("detailed answer should score higher than terse one, short=%.3f long=%.3f",
			shortScore.Overall, longScore.Overall)
	}
}

func TestKeywordCoverage(t *testing.T) {
	s := NewScorer()
	goodScore := s.Evaluate("How does React handle state management?",
		"React handles state management through useState hooks and context API. State changes trigger re-renders.")
	badScore := s.Evaluate("How does React handle state management?",
		"The weather is nice today. I like pizza.")

	if goodScore.KeywordCoverage <= badScore.KeywordCoverage {
		t.Errorf("relevant reply should have higher keyword coverage, good=%.3f bad=%.3f",
			goodScore.KeywordCoverage, badScore.KeywordCoverage)
	}
}

func TestNGramDiversity(t *testing.T) {
	s := NewScorer()

	diverseScore := s.Evaluate("Tell me about AI",
		"Artificial intelligence encompasses machine learning, natural language processing, computer vision, and robotics.")
	repetitiveScore := s.Evaluate("Tell me about AI",
		"great great great thing great great great thing great great great thing great great great")

	if diverseScore.NGramDiversity <= repetitiveScore.NGramDiversity {
		t.Errorf("diverse text should score higher, diverse=%.3f repetitive=%.3f",
			diverseScore.NGramDiversity, repetitiveScore.NGramDiversity)
	}
}

func TestChineseDialogue(t *testing.T) {
	s := NewScorer()
	score := s.Evaluate("什么是人工智能？",
		"人工智能是计算机科学的一个分支，致力于创建能够模拟人类智能行为的系统。"+
			"它包括机器学习、自然语言处理、计算机视觉等多个子领域。")

	if score.Overall <= 0.3 {
		t.Errorf("good Chinese answer should score above 0.3, got %.3f", score.Overall)
	}
}

func TestQuestionAlignment(t *testing.T) {
	s := NewScorer()

	explanatoryScore := s.Evaluate("Why does this code crash?",
		"Because the null pointer is not checked before dereferencing. You should add a nil check at line 42.")
	refusalScore := s.Evaluate("Why does this code crash?",
		"I'm sorry, I cannot help with that.")

	if refusalScore.QuestionAlignment >= explanatoryScore.QuestionAlignment {
		t.Errorf("explanatory answer should align better than refusal, explain=%.3f refusal=%.3f",
			explanatoryScore.QuestionAlignment, refusalScore.QuestionAlignment)
	}
}

func TestCodeQuestion(t *testing.T) {
	s := NewScorer()
	codeScore := s.Evaluate("Write code to sort an array",
		"Here's a function:\n```python\ndef sort_array(arr):\n    return sorted(arr)\n```")
	textScore := s.Evaluate("Write code to sort an array",
		"Sorting is a fundamental operation in computer science.")

	if codeScore.QuestionAlignment <= textScore.QuestionAlignment {
		t.Errorf("code response should align better for code questions, code=%.3f text=%.3f",
			codeScore.QuestionAlignment, textScore.QuestionAlignment)
	}
}

func TestInformationDensity(t *testing.T) {
	s := NewScorer()
	denseScore := s.Evaluate("Describe Go", "Go is a statically typed compiled language designed by Google featuring goroutines channels garbage collection and simplicity")
	sparseScore := s.Evaluate("Describe Go", "Go is is is is is is is is is is is is is is is is a thing that is is is is a thing")

	if sparseScore.InformationDensity >= denseScore.InformationDensity {
		t.Errorf("dense text should have higher info density, dense=%.3f sparse=%.3f",
			denseScore.InformationDensity, sparseScore.InformationDensity)
	}
}

func TestScorerCustomWeights(t *testing.T) {
	s := &Scorer{
		SatisfiedThreshold: 0.5,
		Weights: ScorerWeights{
			KeywordCoverage:    1.0,
			NGramDiversity:     0.0,
			LengthRatio:        0.0,
			InformationDensity: 0.0,
			QuestionAlignment:  0.0,
		},
	}

	score := s.Evaluate("machine learning", "machine learning is great")
	if score.Overall < 0.5 {
		t.Errorf("with keyword-only weight, matching reply should score high, got %.3f", score.Overall)
	}
}
