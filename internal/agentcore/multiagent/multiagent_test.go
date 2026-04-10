package multiagent

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestBusSubscribePublish(t *testing.T) {
	bus := NewBus(100)

	var received []Message
	var mu sync.Mutex
	bus.Subscribe("agent-1", func(msg Message) *Message {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
		return nil
	})

	bus.Publish(Message{
		ID: "msg-1", Type: MsgTask, From: "system", To: "agent-1",
		Content: "hello", Timestamp: time.Now(),
	})

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("received = %d, want 1", len(received))
	}
	if received[0].Content != "hello" {
		t.Errorf("content = %s, want hello", received[0].Content)
	}
}

func TestBusBroadcast(t *testing.T) {
	bus := NewBus(100)

	count := 0
	var mu sync.Mutex
	handler := func(msg Message) *Message {
		mu.Lock()
		count++
		mu.Unlock()
		return nil
	}

	bus.Subscribe("agent-1", handler)
	bus.Subscribe("agent-2", handler)

	bus.Publish(Message{
		ID: "msg-1", Type: MsgControl, From: "system", To: "*",
		Content: "broadcast", Timestamp: time.Now(),
	})

	mu.Lock()
	defer mu.Unlock()
	if count != 2 {
		t.Errorf("broadcast count = %d, want 2", count)
	}
}

func TestBusHistory(t *testing.T) {
	bus := NewBus(100)

	for i := 0; i < 5; i++ {
		bus.Publish(Message{
			ID: fmt.Sprintf("msg-%d", i), Type: MsgTask, From: "s", To: "*",
			Content: "msg", Timestamp: time.Now(),
		})
	}

	hist := bus.History(3)
	if len(hist) != 3 {
		t.Errorf("history = %d, want 3", len(hist))
	}
}

func TestSupervisorRun(t *testing.T) {
	team := Team{
		ID:      "team-1",
		Name:    "Test Team",
		Pattern: PatternSupervisor,
		Roles: []AgentRole{
			{ID: "coordinator", Name: "Coordinator", Description: "Coordinates tasks"},
			{ID: "worker", Name: "Worker", Description: "Does work"},
		},
		Supervisor: "coordinator",
		MaxRounds:  5,
	}

	round := 0
	agentFn := func(ctx context.Context, role AgentRole, msg Message) (string, error) {
		round++
		if round >= 3 {
			return "[DONE] Task completed successfully.", nil
		}
		if role.ID == "coordinator" {
			return "Delegating to worker", nil
		}
		return "Work result: done", nil
	}

	supervisor := NewSupervisor(team, agentFn)
	session, err := supervisor.Run(context.Background(), "Test goal", "tenant1")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if session.Status != SessionCompleted {
		t.Errorf("status = %s, want completed", session.Status)
	}
	if session.Rounds == 0 {
		t.Error("expected > 0 rounds")
	}
}

func TestSupervisorTimeout(t *testing.T) {
	team := Team{
		ID:      "team-2",
		Name:    "Timeout Team",
		Pattern: PatternSupervisor,
		Roles: []AgentRole{
			{ID: "coord", Name: "Coord", Description: "Never finishes"},
		},
		Supervisor: "coord",
		MaxRounds:  3,
	}

	agentFn := func(ctx context.Context, role AgentRole, msg Message) (string, error) {
		return "Still working...", nil // never says DONE
	}

	supervisor := NewSupervisor(team, agentFn)
	session, err := supervisor.Run(context.Background(), "Infinite goal", "t1")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if session.Status != SessionTimeout {
		t.Errorf("status = %s, want timeout", session.Status)
	}
}

func TestSupervisorAgentError(t *testing.T) {
	team := Team{
		ID:      "team-3",
		Name:    "Error Team",
		Pattern: PatternSupervisor,
		Roles:   []AgentRole{{ID: "coord", Name: "Coord", Description: "Fails"}},
		Supervisor: "coord",
		MaxRounds:  3,
	}

	agentFn := func(ctx context.Context, role AgentRole, msg Message) (string, error) {
		return "", fmt.Errorf("agent crashed")
	}

	supervisor := NewSupervisor(team, agentFn)
	session, err := supervisor.Run(context.Background(), "Failing goal", "t1")
	if err == nil {
		t.Fatal("expected error")
	}
	if session.Status != SessionFailed {
		t.Errorf("status = %s, want failed", session.Status)
	}
}

func TestSupervisorContextCancel(t *testing.T) {
	team := Team{
		ID:      "team-4",
		Name:    "Cancel Team",
		Pattern: PatternSupervisor,
		Roles:   []AgentRole{{ID: "coord", Name: "Coord", Description: "Waits"}},
		Supervisor: "coord",
		MaxRounds:  100,
	}

	agentFn := func(ctx context.Context, role AgentRole, msg Message) (string, error) {
		return "working", nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled immediately

	supervisor := NewSupervisor(team, agentFn)
	session, _ := supervisor.Run(ctx, "Cancelled goal", "t1")
	if session.Status != SessionFailed {
		t.Errorf("status = %s, want failed (cancelled)", session.Status)
	}
}

func TestTeamPatternConstants(t *testing.T) {
	if PatternSupervisor != "supervisor" {
		t.Error("wrong supervisor constant")
	}
	if PatternPeer != "peer" {
		t.Error("wrong peer constant")
	}
	if PatternPipeline != "pipeline" {
		t.Error("wrong pipeline constant")
	}
}

func TestIsCompletionSignal(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"[DONE] All complete", true},
		{"任务完成了", true},
		{"Still working...", false},
	}
	for _, tt := range tests {
		got := isCompletionSignal(tt.input)
		if got != tt.want {
			t.Errorf("isCompletionSignal(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
