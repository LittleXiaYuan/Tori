package cognisdk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPackManifestJSONAndYAML(t *testing.T) {
	dir := t.TempDir()

	jsonPath := filepath.Join(dir, "companion.pack.json")
	yamlPath := filepath.Join(dir, "work.pack.yaml")

	jsonPack := XiaoyuCompanionPack()
	yamlPack := YunqueWorkPack()

	if err := SavePackManifest(jsonPack, jsonPath); err != nil {
		t.Fatalf("save json pack: %v", err)
	}
	yamlData := []byte(strings.TrimSpace(`
id: yunque-work-pack
version: 0.1.0
type: work
display_name: Yunque Work Pack
provides:
  - work_delivery
  - tool_confirmation_policy
belief_seeds:
  - id: yw.value.deliver_work
    kind: value
    statement: 工作任务优先交付可验收结果
    confidence: 1
boundary:
  high_risk_actions:
    - delete
    - remove
disposition_rules:
  - id: yunque.work.deliver_first
    when:
      intent: work_task
    mode: deliver_work
    tone: focused_warm
    priority: 10
`))
	if err := os.WriteFile(yamlPath, yamlData, 0o644); err != nil {
		t.Fatalf("write yaml pack: %v", err)
	}

	loadedJSON, err := LoadPackManifest(jsonPath)
	if err != nil {
		t.Fatalf("load json pack: %v", err)
	}
	loadedYAML, err := LoadPackManifest(yamlPath)
	if err != nil {
		t.Fatalf("load yaml pack: %v", err)
	}

	if loadedJSON.ID != jsonPack.ID {
		t.Fatalf("json id = %q, want %q", loadedJSON.ID, jsonPack.ID)
	}
	if loadedYAML.ID != yamlPack.ID {
		t.Fatalf("yaml id = %q, want %q", loadedYAML.ID, yamlPack.ID)
	}
}

func TestLoadPacksFromDirAndBuildManager(t *testing.T) {
	dir := t.TempDir()
	if err := SavePackManifest(XiaoyuCompanionPack(), filepath.Join(dir, "companion.pack.json")); err != nil {
		t.Fatalf("save companion pack: %v", err)
	}
	if err := SavePackManifest(YunqueWorkPack(), filepath.Join(dir, "work.pack.json")); err != nil {
		t.Fatalf("save work pack: %v", err)
	}

	pm, errs, err := NewPackManagerFromDir(dir)
	if err != nil {
		t.Fatalf("load pack manager: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected load errors: %#v", errs)
	}
	statuses := pm.List()
	if len(statuses) != 2 {
		t.Fatalf("expected 2 packs, got %d", len(statuses))
	}
}

func TestPackManagerRejectsDuplicateIDs(t *testing.T) {
	pm := NewPackManager()
	if err := pm.Add(PackManifest{ID: "dup", Version: "0.1.0", Type: "cogni"}); err != nil {
		t.Fatalf("add first pack: %v", err)
	}
	if err := pm.Add(PackManifest{ID: "dup", Version: "0.1.0", Type: "work"}); err == nil {
		t.Fatal("expected duplicate id to fail")
	}
}
