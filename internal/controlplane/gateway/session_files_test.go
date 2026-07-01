package gateway

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"yunque-agent/internal/agentcore/planner"
)

func TestRegisterSessionFilesFromWorkspacePaths(t *testing.T) {
	gw, _ := newTestGateway()
	gw.convStore.GetOrCreate("s1", "t1")

	dir := t.TempDir()
	uploaded := filepath.Join(dir, "cat.png")
	if err := os.WriteFile(uploaded, []byte("fake-png"), 0644); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}

	gw.registerSessionFiles("s1", []string{uploaded, filepath.Join(dir, "does-not-exist.png")}, nil)

	files := gw.convStore.Files("s1")
	if len(files) != 1 {
		t.Fatalf("expected 1 registered file, got %#v", files)
	}
	if files[0].Kind != "uploaded" || files[0].Name != "cat.png" {
		t.Fatalf("unexpected file entry: %#v", files[0])
	}
}

func TestRegisterSessionFilesFromGeneratedResult(t *testing.T) {
	gw, _ := newTestGateway()
	gw.convStore.GetOrCreate("s1", "t1")

	// CollectGeneratedFileRefs resolves relative paths against
	// sessionFileSearchRoots (CWD-relative, matching how skills actually
	// write files), so the fixture must live under one of those roots rather
	// than an arbitrary absolute temp dir.
	if err := os.MkdirAll("data/output", 0755); err != nil {
		t.Fatalf("mkdir data/output: %v", err)
	}
	relPath := "data/output/session_files_test_report.docx"
	if err := os.WriteFile(relPath, []byte("fake-docx"), 0644); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}
	t.Cleanup(func() { os.Remove(relPath) })

	result := &planner.PlanResult{
		Reply: "已经帮你生成好了",
		Plan: []planner.PlanStep{
			{Skill: "docx_create", Result: fmt.Sprintf("已生成 Word 文档: %s (100 bytes, 3 块, engine=Go-OOXML(fast))", relPath)},
		},
	}
	gw.registerSessionFiles("s1", nil, result)

	files := gw.convStore.Files("s1")
	if len(files) != 1 {
		t.Fatalf("expected 1 registered file, got %#v", files)
	}
	if files[0].Kind != "generated" || files[0].Name != "session_files_test_report.docx" {
		t.Fatalf("unexpected file entry: %#v", files[0])
	}
}

func TestRegisterSessionFilesNoopWithoutSessionID(t *testing.T) {
	gw, _ := newTestGateway()
	// Should not panic and should not create a session as a side effect.
	gw.registerSessionFiles("", []string{"whatever.png"}, nil)
	if gw.convStore.GetSession("") != nil {
		t.Fatal("expected no session to be created for empty session id")
	}
}

func TestSessionFilesForRequestConvertsStoredFiles(t *testing.T) {
	gw, _ := newTestGateway()
	gw.convStore.GetOrCreate("s1", "t1")
	gw.registerSessionFiles("s1", nil, &planner.PlanResult{})

	dir := t.TempDir()
	imgPath := filepath.Join(dir, "cat.png")
	if err := os.WriteFile(imgPath, []byte("fake-png"), 0644); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}
	gw.registerSessionFiles("s1", []string{imgPath}, nil)

	refs := gw.sessionFilesForRequest("s1")
	if len(refs) != 1 || refs[0].Kind != "uploaded" || refs[0].Path != imgPath || refs[0].Name != "cat.png" {
		t.Fatalf("unexpected session file refs: %#v", refs)
	}
}

func TestSessionFilesForRequestEmptyWithoutSessionID(t *testing.T) {
	gw, _ := newTestGateway()
	if refs := gw.sessionFilesForRequest(""); refs != nil {
		t.Fatalf("expected nil refs for empty session id, got %#v", refs)
	}
}
