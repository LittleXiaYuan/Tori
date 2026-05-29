package cognikernel

import (
	"context"
	"testing"
)

// Reflection's suggested memory updates (previously computed then discarded
// because no memoryUpdate func was wired) now flow through with the tenant id,
// so per-user memory can actually persist.
func TestReflectiveLoopAppliesMemoryUpdatesWithTenant(t *testing.T) {
	rl := NewReflectiveLoop()
	rl.SetReflectEval(func(ctx context.Context, intent, reply string, skills []string) (*ReflectEvalResult, error) {
		return &ReflectEvalResult{
			Satisfied: true,
			Quality:   9,
			MemoryUpdates: []MemUpdateReq{
				{Action: "preference", Key: "report_format", Value: "PDF"},
				{Action: "delete", Key: "stale_fact"},
			},
		}, nil
	})

	type applied struct{ tenant, action, key, value string }
	var got []applied
	rl.SetMemoryUpdate(func(ctx context.Context, tenantID, action, key, value string) error {
		got = append(got, applied{tenantID, action, key, value})
		return nil
	})

	res, err := rl.Run(context.Background(), ConversationEndData{
		TenantID:   "tenant-42",
		UserIntent: "导出报告",
		AgentReply: "已导出 PDF",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.MemoryUpdates != 2 {
		t.Fatalf("expected 2 memory updates applied, got %d", res.MemoryUpdates)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 memoryUpdate calls, got %d", len(got))
	}
	for _, a := range got {
		if a.tenant != "tenant-42" {
			t.Fatalf("memory update missing tenant scope: %+v", a)
		}
	}
	if got[0].key != "report_format" || got[0].value != "PDF" {
		t.Fatalf("unexpected preference update: %+v", got[0])
	}
	if got[1].action != "delete" {
		t.Fatalf("expected delete action, got %+v", got[1])
	}
}
