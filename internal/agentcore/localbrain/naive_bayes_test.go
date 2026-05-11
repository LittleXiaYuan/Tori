package localbrain

import (
	"testing"
)

func TestNaiveBayes_NotTrainedByDefault(t *testing.T) {
	nb := NewNaiveBayesClassifier()
	if nb.Trained() {
		t.Error("empty classifier should not be Trained")
	}
}

func TestNaiveBayes_TrainAndPredict(t *testing.T) {
	nb := NewNaiveBayesClassifier()

	chatQueries := []string{"hello", "hi there", "how are you", "good morning", "hey"}
	codeQueries := []string{"write a function", "debug this code", "refactor the module", "fix the bug", "implement the API"}
	toolQueries := []string{"search the web", "open the file", "run the command", "execute shell", "fetch the URL"}

	for _, q := range chatQueries {
		nb.Train(q, "chat")
	}
	for _, q := range codeQueries {
		nb.Train(q, "code")
	}
	for _, q := range toolQueries {
		nb.Train(q, "tool")
	}

	if !nb.Trained() {
		t.Fatal("classifier should be Trained after 15 samples across 3 classes")
	}

	intent, conf := nb.Predict("hello world how are you doing")
	if intent == nil {
		t.Fatal("Predict returned nil intent")
	}
	if intent.Category != "chat" {
		t.Errorf("expected 'chat', got %q", intent.Category)
	}
	if conf <= 0 {
		t.Errorf("confidence should be positive, got %.3f", conf)
	}
}

func TestNaiveBayes_CJKTokenization(t *testing.T) {
	nb := NewNaiveBayesClassifier()

	for i := 0; i < 5; i++ {
		nb.Train("你好世界", "chat")
		nb.Train("调试代码修复漏洞", "code")
	}

	docs, classes, vocab := nb.Stats()
	if docs != 10 || classes != 2 {
		t.Errorf("expected 10 docs / 2 classes, got %d / %d", docs, classes)
	}
	if vocab == 0 {
		t.Error("vocab should be non-zero after CJK training")
	}
}

func TestNaiveBayes_EmptyQuery(t *testing.T) {
	nb := NewNaiveBayesClassifier()
	nb.Train("hello world", "chat")
	nb.Train("debug code", "code")

	intent, conf := nb.Predict("")
	if intent != nil {
		t.Error("empty query should return nil intent")
	}
	if conf != 0 {
		t.Error("empty query should return 0 confidence")
	}
}

func TestNbTokenize_MixedContent(t *testing.T) {
	tokens := nbTokenize("Hello 你好 World")
	if len(tokens) == 0 {
		t.Fatal("tokenization produced no tokens")
	}

	hasCJK := false
	hasLatin := false
	for _, tok := range tokens {
		for _, r := range tok {
			if isCJK(r) {
				hasCJK = true
			} else {
				hasLatin = true
			}
		}
	}
	if !hasCJK || !hasLatin {
		t.Errorf("mixed content should produce both CJK and Latin tokens: CJK=%v Latin=%v", hasCJK, hasLatin)
	}
}
