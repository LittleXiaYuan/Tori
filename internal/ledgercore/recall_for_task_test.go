package ledger_test

import (
	"context"
	"strings"
	"testing"

	"yunque-agent/internal/ledgercore"
)

func TestRecallForTaskUsesTaskTenant(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	task, err := ldg.Tasks.CreateTask(ctx, "tenant-scoped recall", ledger.TaskTypeGoal, "t1")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	taskID := task.ID
	memories := []*ledger.MemoryEntry{
		{
			TenantID:   "t1",
			TaskID:     &taskID,
			Kind:       ledger.MemoryFact,
			Key:        "shared",
			Content:    "shared recall target",
			Source:     "user",
			Confidence: 0.9,
		},
		{
			TenantID:   "t2",
			TaskID:     &taskID,
			Kind:       ledger.MemoryFact,
			Key:        "shared",
			Content:    "shared recall target",
			Source:     "user",
			Confidence: 0.9,
		},
	}
	for _, m := range memories {
		if err := ldg.Memory.Put(ctx, m); err != nil {
			t.Fatalf("Put memory %s: %v", m.TenantID, err)
		}
	}

	result, err := ldg.Recall.RecallForTask(ctx, task, "shared recall target", 10)
	if err != nil {
		t.Fatalf("RecallForTask: %v", err)
	}
	if len(result.Entries) == 0 {
		t.Fatal("expected tenant-scoped recall results")
	}
	for _, entry := range result.Entries {
		if entry.Entry.TenantID != "t1" {
			t.Fatalf("RecallForTask leaked tenant %q for task tenant %q", entry.Entry.TenantID, task.TenantID)
		}
	}
}

func TestRecallUsesGraphRAGCommunityCandidates(t *testing.T) {
	ldg := newTestLedger(t)
	ctx := context.Background()

	t1Memory := &ledger.MemoryEntry{
		TenantID:   "t1",
		Kind:       ledger.MemoryFact,
		Key:        "runbook.alpha",
		Content:    "Alpha runbook keeps the deployment checklist and rollback owner.",
		Source:     "user",
		Confidence: 0.9,
	}
	if err := ldg.Memory.Put(ctx, t1Memory); err != nil {
		t.Fatalf("Put t1 memory: %v", err)
	}
	if err := ldg.Graph.LinkMemoryToTopic(ctx, "t1", t1Memory.ID, "workflow orchestration"); err != nil {
		t.Fatalf("LinkMemoryToTopic t1: %v", err)
	}

	t2Memory := &ledger.MemoryEntry{
		TenantID:   "t2",
		Kind:       ledger.MemoryFact,
		Key:        "runbook.hidden",
		Content:    "Hidden tenant deployment checklist.",
		Source:     "user",
		Confidence: 0.9,
	}
	if err := ldg.Memory.Put(ctx, t2Memory); err != nil {
		t.Fatalf("Put t2 memory: %v", err)
	}
	if err := ldg.Graph.LinkMemoryToTopic(ctx, "t2", t2Memory.ID, "workflow orchestration"); err != nil {
		t.Fatalf("LinkMemoryToTopic t2: %v", err)
	}

	gr := ledger.NewGraphRAG(ldg.Backend())
	if err := gr.BuildCommunities(ctx, 10); err != nil {
		t.Fatalf("BuildCommunities: %v", err)
	}
	ldg.Recall.SetGraphRAG(gr)

	result, err := ldg.Recall.Recall(ctx, ledger.RecallQuery{
		TenantID: "t1",
		Query:    "orchestration",
		Limit:    10,
		MinScore: 0.1,
	})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}

	foundT1 := false
	for _, entry := range result.Entries {
		if entry.Entry.ID == t2Memory.ID {
			t.Fatalf("GraphRAG recall leaked tenant t2 memory into t1 result")
		}
		if entry.Entry.ID == t1Memory.ID {
			foundT1 = true
			if !strings.Contains(entry.Reason, "community match") {
				t.Fatalf("expected community match reason, got %q", entry.Reason)
			}
		}
	}
	if !foundT1 {
		t.Fatalf("expected GraphRAG community recall to include t1 memory; entries=%v", result.Entries)
	}
}
