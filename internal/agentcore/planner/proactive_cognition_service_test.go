package planner

import (
	"testing"
	"time"
)

func TestProactiveCognitionServiceRecordExecutionFailure(t *testing.T) {
	bus := NewReverieEventBus(0)
	monitor := NewTaskFailureMonitor(bus, 0.5, time.Minute, 2)
	service := NewProactiveCognitionService()
	service.SetTaskFailureMonitor(monitor)

	if emitted := service.RecordExecutionFailure(true); emitted {
		t.Fatal("expected first failure to stay below minCalls")
	}
	if emitted := service.RecordExecutionFailure(true); !emitted {
		t.Fatal("expected repeated failures to emit proactive event")
	}
}

func TestProactiveCognitionServiceJournalContext(t *testing.T) {
	cfg := DefaultReverieConfig()
	cfg.Enabled = false
	reverie := NewReverie(cfg)
	reverie.journal = append(reverie.journal, Thought{
		ID:           "thought-1",
		Content:      "Go 协程泄漏需要用 context 取消",
		Significance: 0.9,
		CreatedAt:    time.Now(),
	})
	service := NewProactiveCognitionService()
	service.SetReverie(reverie)

	got := service.JournalContext(1, "怎么处理 Go 协程泄漏")
	if got == "" {
		t.Fatal("expected relevant reverie journal context")
	}
}

func TestNilProactiveCognitionServiceIsNoop(t *testing.T) {
	var service *ProactiveCognitionService
	if service.RecordExecutionFailure(true) {
		t.Fatal("nil service should not emit events")
	}
	if got := service.JournalContext(1, "anything"); got != "" {
		t.Fatalf("nil service should return empty journal context, got %q", got)
	}
}
