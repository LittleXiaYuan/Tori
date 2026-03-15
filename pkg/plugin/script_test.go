package plugin

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadScriptPluginJSON(t *testing.T) {
	dir := t.TempDir()
	manifest := `{
		"name": "test-plugin",
		"description": "A test plugin",
		"language": "python",
		"system_prompt": "You can greet people.",
		"skills": [
			{
				"name": "greet",
				"description": "Greet a person",
				"handler": "greet.py",
				"parameters": {
					"name": {"type": "string", "description": "Name to greet", "required": true}
				}
			}
		]
	}`
	os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0644)

	sp, err := LoadScriptPlugin(dir)
	if err != nil {
		t.Fatal(err)
	}
	if sp.Name() != "test-plugin" {
		t.Fatalf("expected test-plugin, got %s", sp.Name())
	}
	if len(sp.Skills()) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(sp.Skills()))
	}
	if sp.Skills()[0].Name() != "greet" {
		t.Fatalf("expected greet skill, got %s", sp.Skills()[0].Name())
	}
	params := sp.Skills()[0].Parameters()
	if params == nil {
		t.Fatal("expected parameters")
	}
}

func TestScriptSkillExecutePython(t *testing.T) {
	// Check if python is available
	interpreter := "python3"
	if runtime.GOOS == "windows" {
		interpreter = "python"
	}
	if _, err := exec_LookPath(interpreter); err != nil {
		t.Skipf("python not available: %v", err)
	}

	dir := t.TempDir()
	// Write a simple python handler
	handler := `import json, sys, os
args = json.loads(os.environ.get("PLUGIN_ARGS", "{}"))
name = args.get("name", "World")
print(f"Hello, {name}!")
`
	os.WriteFile(filepath.Join(dir, "greet.py"), []byte(handler), 0644)

	manifest := `{
		"name": "greet-plugin",
		"description": "Greet plugin",
		"language": "python",
		"skills": [{"name": "greet", "description": "Greet", "handler": "greet.py", "parameters": {"name": {"type": "string", "description": "Name"}}}]
	}`
	os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0644)

	sp, err := LoadScriptPlugin(dir)
	if err != nil {
		t.Fatal(err)
	}

	result, err := sp.Skills()[0].Execute(context.Background(), map[string]any{"name": "Alice"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != "Hello, Alice!" {
		t.Fatalf("expected 'Hello, Alice!', got %q", result)
	}
}

func TestLoaderLoadAll(t *testing.T) {
	dir := t.TempDir()

	// Create two plugin dirs
	for _, name := range []string{"plugin-a", "plugin-b"} {
		pDir := filepath.Join(dir, name)
		os.MkdirAll(pDir, 0755)
		manifest := `{"name": "` + name + `", "description": "Test", "language": "python", "skills": []}`
		os.WriteFile(filepath.Join(pDir, "plugin.json"), []byte(manifest), 0644)
	}

	reg := NewRegistry()
	loader := NewLoader(dir, reg, nil)
	count := loader.LoadAll()
	if count != 2 {
		t.Fatalf("expected 2 plugins loaded, got %d", count)
	}

	all := reg.AllIncludeDisabled()
	if len(all) != 2 {
		t.Fatalf("expected 2 plugins in registry, got %d", len(all))
	}
}

func TestLoaderPreservesEnabledState(t *testing.T) {
	dir := t.TempDir()
	pDir := filepath.Join(dir, "my-plugin")
	os.MkdirAll(pDir, 0755)
	os.WriteFile(filepath.Join(pDir, "plugin.json"), []byte(`{"name": "my-plugin", "description": "Test", "skills": []}`), 0644)

	reg := NewRegistry()
	loader := NewLoader(dir, reg, nil)
	loader.LoadAll()

	// Disable the plugin
	reg.SetEnabled("my-plugin", false)
	if reg.IsEnabled("my-plugin") {
		t.Fatal("should be disabled")
	}

	// Reload - should preserve disabled state
	loader.LoadAll()
	if reg.IsEnabled("my-plugin") {
		t.Fatal("should still be disabled after reload")
	}
}

// exec_LookPath wraps exec.LookPath to avoid import collision
func exec_LookPath(file string) (string, error) {
	return execLookPath(file)
}
