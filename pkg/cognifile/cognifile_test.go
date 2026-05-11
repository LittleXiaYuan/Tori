package cognifile

import (
	"testing"
)

func TestParseMinimalYAML(t *testing.T) {
	yaml := `
name: test-agent
persona:
  role: 测试助手
`
	cf, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cf.Name != "test-agent" {
		t.Errorf("Name = %q, want %q", cf.Name, "test-agent")
	}
	if cf.Persona.Role != "测试助手" {
		t.Errorf("Persona.Role = %q, want %q", cf.Persona.Role, "测试助手")
	}
	if cf.Schema != SchemaVersion {
		t.Errorf("Schema = %q, want %q", cf.Schema, SchemaVersion)
	}
	if cf.Version != "0.1.0" {
		t.Errorf("Version = %q, want %q", cf.Version, "0.1.0")
	}
}

func TestParseFullJSON(t *testing.T) {
	jsonData := `{
		"name": "code-assistant",
		"display_name": "代码助手",
		"version": "1.0.0",
		"persona": {
			"role": "资深代码审查专家",
			"traits": ["严谨", "高效"],
			"constraints": ["不直接修改生产代码"]
		},
		"model": {
			"tier": "expert",
			"temperature": 0.3
		},
		"activation": {
			"keywords": ["代码", "review", "bug"]
		}
	}`

	cf, err := Parse([]byte(jsonData))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cf.Name != "code-assistant" {
		t.Errorf("Name = %q", cf.Name)
	}
	if cf.Model.Tier != "expert" {
		t.Errorf("Model.Tier = %q", cf.Model.Tier)
	}
	if len(cf.Activation.Keywords) != 3 {
		t.Errorf("Keywords count = %d, want 3", len(cf.Activation.Keywords))
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name    string
		cf      Cognifile
		wantErr bool
	}{
		{
			name:    "valid minimal",
			cf:      Cognifile{Name: "test", Persona: Persona{Role: "helper"}},
			wantErr: false,
		},
		{
			name:    "missing name",
			cf:      Cognifile{Persona: Persona{Role: "helper"}},
			wantErr: true,
		},
		{
			name:    "missing role",
			cf:      Cognifile{Name: "test"},
			wantErr: true,
		},
		{
			name:    "name with spaces",
			cf:      Cognifile{Name: "bad name", Persona: Persona{Role: "helper"}},
			wantErr: true,
		},
		{
			name:    "bad temperature",
			cf:      Cognifile{Name: "test", Persona: Persona{Role: "helper"}, Model: ModelSpec{Temperature: 3.0}},
			wantErr: true,
		},
		{
			name:    "bad trust level",
			cf:      Cognifile{Name: "test", Persona: Persona{Role: "helper"}, Runtime: RuntimeSpec{TrustLevel: 200}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cf.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestToDeclaration(t *testing.T) {
	cf := &Cognifile{
		Name:        "legal-advisor",
		DisplayName: "法律顾问",
		Description: "专业法律咨询",
		Persona: Persona{
			Role:        "资深法律顾问",
			Traits:      []string{"严谨", "专业"},
			Constraints: []string{"不提供具体法律意见"},
			Language:    "zh-CN",
			Tone:        "professional",
		},
		Priority: 50,
	}

	decl := cf.ToDeclaration()

	if decl.ID != "legal-advisor" {
		t.Errorf("ID = %q", decl.ID)
	}
	if decl.DisplayName != "法律顾问" {
		t.Errorf("DisplayName = %q", decl.DisplayName)
	}
	if decl.Priority != 50 {
		t.Errorf("Priority = %d", decl.Priority)
	}
	if decl.Context.Static == "" {
		t.Error("Context.Static should not be empty")
	}
	if decl.Context.Static == "" {
		t.Skip()
	}
	// The synthesized prompt should contain role, traits, and constraints
	for _, want := range []string{"资深法律顾问", "严谨", "不提供具体法律意见"} {
		if !containsStr(decl.Context.Static, want) {
			t.Errorf("Context.Static missing %q", want)
		}
	}
}

func TestMerge(t *testing.T) {
	base := &Cognifile{
		Name: "base-assistant",
		Persona: Persona{
			Role:    "通用助手",
			Traits:  []string{"友善"},
			Language: "zh-CN",
		},
		Model: ModelSpec{Tier: "smart"},
	}

	overlay := &Cognifile{
		Name: "legal-advisor",
		Persona: Persona{
			Role:   "法律顾问",
			Traits: []string{"严谨"},
		},
		Model: ModelSpec{Tier: "expert"},
	}

	result := merge(base, overlay)

	if result.Name != "legal-advisor" {
		t.Errorf("Name = %q", result.Name)
	}
	if result.Persona.Role != "法律顾问" {
		t.Errorf("Role = %q", result.Persona.Role)
	}
	if result.Model.Tier != "expert" {
		t.Errorf("Model.Tier = %q", result.Model.Tier)
	}
	// Traits should be merged (base + overlay)
	if len(result.Persona.Traits) != 2 {
		t.Errorf("Traits count = %d, want 2", len(result.Persona.Traits))
	}
	// Language should be inherited from base
	if result.Persona.Language != "zh-CN" {
		t.Errorf("Language = %q, want zh-CN", result.Persona.Language)
	}
}

func containsStr(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
