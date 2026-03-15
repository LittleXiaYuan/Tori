package selfheal

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyze(t *testing.T) {
	h := New("", nil)

	gap := h.Analyze("计算两个日期之间的天数", "no skill found")
	if gap == "" {
		t.Fatal("should detect capability gap")
	}

	gap = h.Analyze("hello", "")
	if gap != "" {
		t.Fatalf("should not detect gap for simple greeting, got: %s", gap)
	}
}

func TestExtractJSON(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{`{"name":"test"}`, `{"name":"test"}`},
		{"```json\n{\"name\":\"test\"}\n```", `{"name":"test"}`},
		{"Here is the plugin:\n```\n{\"name\":\"test\"}\n```\nDone.", `{"name":"test"}`},
	}
	for _, c := range cases {
		got := extractJSON(c.input)
		if got != c.want {
			t.Fatalf("extractJSON(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestInstall(t *testing.T) {
	dir := t.TempDir()
	h := New(dir, nil)

	plugin := &GeneratedPlugin{
		Name:        "date-calc",
		Description: "Calculate days between dates",
		Language:    "python",
		SkillName:   "date_diff",
		SkillDesc:   "Calculate date difference",
		HandlerCode: "#!/usr/bin/env python3\nprint('hello')\n",
		Params: map[string]ParamDef{
			"from": {Type: "string", Description: "Start date"},
			"to":   {Type: "string", Description: "End date"},
		},
	}

	err := h.Install(plugin)
	if err != nil {
		t.Fatal(err)
	}

	// Check files exist
	manifestPath := filepath.Join(dir, "date-calc", "plugin.json")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest not found: %v", err)
	}

	handlerPath := filepath.Join(dir, "date-calc", "handler.py")
	if _, err := os.Stat(handlerPath); err != nil {
		t.Fatalf("handler not found: %v", err)
	}

	data, _ := os.ReadFile(handlerPath)
	if string(data) != "#!/usr/bin/env python3\nprint('hello')\n" {
		t.Fatalf("unexpected handler content: %s", string(data))
	}
}

func TestGenerateWithMockLLM(t *testing.T) {
	mockLLM := func(ctx context.Context, system, user string) (string, error) {
		return `{
			"name": "unit-converter",
			"description": "Convert between units",
			"language": "python",
			"skill_name": "convert_units",
			"skill_desc": "Convert between measurement units",
			"params": {"value": {"type": "string", "description": "Value with unit"}},
			"handler_code": "#!/usr/bin/env python3\nimport json, os\nargs = json.loads(os.environ.get('PLUGIN_ARGS', '{}'))\nprint(args.get('value', 'N/A'))\n"
		}`, nil
	}

	dir := t.TempDir()
	h := New(dir, mockLLM)

	plugin, err := h.GenerateAndInstall(context.Background(), "Convert 100kg to pounds")
	if err != nil {
		t.Fatal(err)
	}
	if plugin.Name != "unit-converter" {
		t.Fatalf("expected unit-converter, got %s", plugin.Name)
	}
	if plugin.SkillName != "convert_units" {
		t.Fatalf("expected convert_units, got %s", plugin.SkillName)
	}

	// Verify installed
	if _, err := os.Stat(filepath.Join(dir, "unit-converter", "plugin.json")); err != nil {
		t.Fatal("plugin not installed on disk")
	}
}
