package cogni

import (
	"context"
	"testing"
	"time"
)

func TestCogniBus_Route_SingleWinner(t *testing.T) {
	eval := NewEvaluator()
	bus := NewCogniBus(eval, DefaultBusConfig())

	bus.Register(&Declaration{
		ID: "code-reviewer",
		Activation: ActivationRules{
			Keywords: []string{"review", "PR"},
			MinScore: 0.2,
		},
	})
	bus.Register(&Declaration{
		ID: "chat-bot",
		Activation: ActivationRules{
			Keywords: []string{"hello", "hi"},
			MinScore: 0.2,
		},
	})

	result := bus.Route(context.Background(), Session{Message: "please review my PR"})

	if len(result.Winners) != 1 {
		t.Fatalf("winners = %d, want 1", len(result.Winners))
	}
	if result.Winners[0].CogniID != "code-reviewer" {
		t.Errorf("winner = %s, want code-reviewer", result.Winners[0].CogniID)
	}
}

func TestCogniBus_Route_MultipleWinners(t *testing.T) {
	eval := NewEvaluator()
	cfg := DefaultBusConfig()
	cfg.MaxConcurrent = 3
	cfg.MinConfidence = 0.1
	bus := NewCogniBus(eval, cfg)

	bus.Register(&Declaration{
		ID:         "a",
		Activation: ActivationRules{Keywords: []string{"test"}, MinScore: 0.1},
	})
	bus.Register(&Declaration{
		ID:         "b",
		Activation: ActivationRules{Keywords: []string{"test"}, MinScore: 0.1},
	})

	result := bus.Route(context.Background(), Session{Message: "test"})
	if len(result.Winners) != 2 {
		t.Errorf("winners = %d, want 2", len(result.Winners))
	}
}

func TestCogniBus_Route_NoMatch(t *testing.T) {
	eval := NewEvaluator()
	bus := NewCogniBus(eval, DefaultBusConfig())

	bus.Register(&Declaration{
		ID:         "specific",
		Activation: ActivationRules{Keywords: []string{"special"}, MinScore: 0.5},
	})

	result := bus.Route(context.Background(), Session{Message: "hello"})
	if len(result.Winners) != 0 {
		t.Errorf("winners = %d, want 0", len(result.Winners))
	}
}

type mockBidder struct {
	bid *Bid
}

func (m *mockBidder) Bid(ctx context.Context, session Session) (*Bid, error) {
	return m.bid, nil
}

func TestCogniBus_CustomBidder(t *testing.T) {
	eval := NewEvaluator()
	cfg := DefaultBusConfig()
	cfg.MinConfidence = 0.1
	bus := NewCogniBus(eval, cfg)

	bus.Register(&Declaration{
		ID:         "smart",
		Activation: ActivationRules{Keywords: []string{"code"}, MinScore: 0.1},
	})
	bus.RegisterBidder("smart", &mockBidder{
		bid: &Bid{Confidence: 0.95, Cost: 0.01, ETA: time.Second, Reason: "I'm the best"},
	})

	result := bus.Route(context.Background(), Session{Message: "review code"})
	if len(result.Winners) != 1 {
		t.Fatalf("winners = %d", len(result.Winners))
	}
	if result.Winners[0].Confidence != 0.95 {
		t.Errorf("confidence = %f, want 0.95", result.Winners[0].Confidence)
	}
}

func TestCogniBus_RegisterUnregister(t *testing.T) {
	eval := NewEvaluator()
	bus := NewCogniBus(eval, DefaultBusConfig())

	bus.Register(&Declaration{ID: "a", Activation: ActivationRules{AlwaysOn: true}})
	if bus.ActiveCognis() != 1 {
		t.Errorf("active = %d, want 1", bus.ActiveCognis())
	}

	bus.Unregister("a")
	if bus.ActiveCognis() != 0 {
		t.Errorf("active = %d, want 0", bus.ActiveCognis())
	}
}

func TestCogniBus_ClearRemovesCognisAndBidders(t *testing.T) {
	eval := NewEvaluator()
	cfg := DefaultBusConfig()
	cfg.MinConfidence = 0.1
	bus := NewCogniBus(eval, cfg)

	bus.Register(&Declaration{ID: "a", Activation: ActivationRules{AlwaysOn: true}})
	bus.RegisterBidder("a", &mockBidder{
		bid: &Bid{Confidence: 0.95, Cost: 0.01, ETA: time.Millisecond, Reason: "custom"},
	})
	if bus.ActiveCognis() != 1 {
		t.Fatalf("active = %d, want 1", bus.ActiveCognis())
	}
	if got := bus.Route(context.Background(), Session{Message: "anything"}); len(got.Winners) != 1 {
		t.Fatalf("expected winner before clear, got %#v", got)
	}

	bus.Clear()
	if bus.ActiveCognis() != 0 {
		t.Fatalf("active after clear = %d, want 0", bus.ActiveCognis())
	}
	if got := bus.Route(context.Background(), Session{Message: "anything"}); len(got.Winners) != 0 || len(got.AllBids) != 0 {
		t.Fatalf("expected no routing after clear, got %#v", got)
	}
}

func TestDefaultBusConfig(t *testing.T) {
	cfg := DefaultBusConfig()
	if cfg.MaxConcurrent != 1 {
		t.Errorf("MaxConcurrent = %d", cfg.MaxConcurrent)
	}
	if cfg.BidTimeout != 500*time.Millisecond {
		t.Errorf("BidTimeout = %v", cfg.BidTimeout)
	}
}

func TestJoinReasons(t *testing.T) {
	if got := joinReasons(nil); got != "" {
		t.Errorf("empty = %q, want \"\"", got)
	}
	if got := joinReasons([]string{"only"}); got != "only" {
		t.Errorf("single = %q, want \"only\"", got)
	}
	if got := joinReasons([]string{"a", "b", "c"}); got != "a (+2 more)" {
		t.Errorf("three = %q, want \"a (+2 more)\"", got)
	}
	// Regression: string(rune('0'+n)) produced garbage glyphs (':','-',...) once
	// the surplus count reached 10+. Decimal formatting must be used instead.
	many := make([]string, 12)
	for i := range many {
		many[i] = "r"
	}
	if got := joinReasons(many); got != "r (+11 more)" {
		t.Errorf("twelve = %q, want \"r (+11 more)\"", got)
	}
}
