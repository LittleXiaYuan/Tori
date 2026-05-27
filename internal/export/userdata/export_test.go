package userdata

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	reflectpkg "yunque-agent/internal/experimental/reflect"
	yledger "yunque-agent/internal/ledger"
	ledger "yunque-agent/internal/ledgercore"
)

func TestExportCreatesReadableUserDataBundle(t *testing.T) {
	dataDir := t.TempDir()
	outDir := t.TempDir()

	writeJSON(t, filepath.Join(dataDir, "memory.json"), []map[string]any{{
		"id": "mem-1", "kind": "fact", "key": "project", "content": "云雀是本地优先 Agent", "source": "test",
	}})
	writeJSON(t, filepath.Join(dataDir, "experience.json"), []reflectpkg.Experience{{
		ID: "exp-1", Source: "task", Category: "strategy", Outcome: "success", Lesson: "先做可验证增量", Context: "长期任务", CreatedAt: time.Date(2026, 5, 23, 9, 0, 0, 0, time.UTC),
	}})
	writeJSON(t, filepath.Join(dataDir, "sessions", "session-1.json"), map[string]any{
		"id": "session-1", "tenant_id": "default", "name": "交付讨论", "messages": []map[string]string{{"role": "user", "content": "请导出我的数据"}, {"role": "assistant", "content": "好的"}},
	})

	ldg, err := yledger.InitLedgerAt(filepath.Join(dataDir, "ledger", "ledger.db"))
	if err != nil {
		t.Fatalf("InitLedgerAt: %v", err)
	}
	defer ldg.Close()
	if err := ldg.Memory.Put(context.Background(), &ledger.MemoryEntry{TenantID: "default", Kind: ledger.MemoryPreference, Key: "tone", Content: "中文简洁回答", Source: "user"}); err != nil {
		t.Fatalf("put ledger memory: %v", err)
	}
	if err := ldg.KV.Put(context.Background(), "workload_feedback", "data", []reflectpkg.Experience{{
		ID: "wf-1", Source: "workload_feedback", SourceID: "pack-runtime", Category: "workload_feedback", Outcome: "partial", Lesson: "最顺手：能力清单\n最不顺手：入口不明显", Tags: []string{"workload:pack-runtime", "findability:partial"}, CreatedAt: time.Date(2026, 5, 23, 10, 0, 0, 0, time.UTC),
	}}); err != nil {
		t.Fatalf("put feedback: %v", err)
	}

	report, err := Export(context.Background(), Options{DataDir: dataDir, OutDir: outDir, Now: func() time.Time { return time.Date(2026, 5, 23, 11, 0, 0, 0, time.UTC) }})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if report.MemoryCount != 2 || report.SessionCount != 1 || report.FeedbackCount != 2 {
		t.Fatalf("unexpected counts: %#v", report)
	}

	for _, rel := range []string{"README.md", "manifest.json", "memory.md", "conversations.md", "feedback.md", "raw/memory.json", "raw/experience.json", "raw/sessions/session-1.json", "raw/ledger/ledger.db"} {
		if _, err := os.Stat(filepath.Join(report.ExportDir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
	assertContains(t, filepath.Join(report.ExportDir, "memory.md"), "云雀是本地优先 Agent")
	assertContains(t, filepath.Join(report.ExportDir, "memory.md"), "中文简洁回答")
	assertContains(t, filepath.Join(report.ExportDir, "conversations.md"), "请导出我的数据")
	assertContains(t, filepath.Join(report.ExportDir, "feedback.md"), "入口不明显")
	assertContains(t, filepath.Join(report.ExportDir, "README.md"), "记忆：2 条")
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertContains(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s does not contain %q\n%s", path, want, string(data))
	}
}
