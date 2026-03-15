package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestValidateRequired(t *testing.T) {
	schema := SchemaMap{
		"name": {Type: TypeString, Required: true},
		"port": {Type: TypeInt, Required: true},
	}
	data := map[string]any{"name": "tori"}
	errs := Validate(data, schema)
	if len(errs) != 1 || errs[0].Path != "port" {
		t.Fatalf("expected 1 error on 'port', got %v", errs)
	}
}

func TestValidateTypes(t *testing.T) {
	schema := SchemaMap{
		"name":  {Type: TypeString},
		"port":  {Type: TypeInt},
		"debug": {Type: TypeBool},
	}
	data := map[string]any{"name": 123, "port": "abc", "debug": "yes"}
	errs := Validate(data, schema)
	if len(errs) != 3 {
		t.Fatalf("expected 3 type errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateEnum(t *testing.T) {
	schema := SchemaMap{
		"mode": {Type: TypeString, Enum: []any{"hot", "restart", "off"}},
	}
	errs := Validate(map[string]any{"mode": "invalid"}, schema)
	if len(errs) != 1 {
		t.Fatalf("expected 1 enum error, got %d", len(errs))
	}
	errs2 := Validate(map[string]any{"mode": "hot"}, schema)
	if len(errs2) != 0 {
		t.Fatalf("expected 0 errors, got %d", len(errs2))
	}
}

func TestValidateMinMax(t *testing.T) {
	min := float64(1)
	max := float64(65535)
	schema := SchemaMap{
		"port": {Type: TypeInt, Min: &min, Max: &max},
	}
	errs := Validate(map[string]any{"port": float64(0)}, schema)
	if len(errs) != 1 {
		t.Fatalf("expected 1 min error, got %d", len(errs))
	}
	errs2 := Validate(map[string]any{"port": float64(99999)}, schema)
	if len(errs2) != 1 {
		t.Fatalf("expected 1 max error, got %d", len(errs2))
	}
}

func TestValidateNested(t *testing.T) {
	schema := SchemaMap{
		"gateway": {Type: TypeObject, Fields: SchemaMap{
			"port": {Type: TypeInt, Required: true},
			"host": {Type: TypeString},
		}},
	}
	data := map[string]any{
		"gateway": map[string]any{"host": "localhost"},
	}
	errs := Validate(data, schema)
	if len(errs) != 1 || errs[0].Path != "gateway.port" {
		t.Fatalf("expected error on gateway.port, got %v", errs)
	}
}

func TestValidateArray(t *testing.T) {
	schema := SchemaMap{
		"tags": {Type: TypeArray, Items: &FieldSchema{Type: TypeString}},
	}
	data := map[string]any{
		"tags": []any{"a", "b", float64(3)},
	}
	errs := Validate(data, schema)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for non-string array item, got %d", len(errs))
	}
}

func TestSchemaValidatePass(t *testing.T) {
	schema := SchemaMap{
		"name": {Type: TypeString, Required: true},
		"port": {Type: TypeInt},
	}
	errs := Validate(map[string]any{"name": "tori", "port": float64(9090)}, schema)
	if len(errs) != 0 {
		t.Fatalf("expected 0 errors, got %d", len(errs))
	}
}

func TestStripComments(t *testing.T) {
	input := `{
  // this is a comment
  "name": "tori", /* inline */
  "port": 9090
}`
	cleaned := stripComments(input)
	if contains(cleaned, "//") || contains(cleaned, "/*") {
		t.Fatalf("comments not stripped: %s", cleaned)
	}
}

func TestStripTrailingCommas(t *testing.T) {
	input := `{"a": 1, "b": 2,}`
	cleaned := stripTrailingCommas(input)
	if contains(cleaned, ",}") {
		t.Fatalf("trailing comma not stripped: %s", cleaned)
	}
}

func TestResolveSecretsEnv(t *testing.T) {
	os.Setenv("TEST_SECRET_KEY", "s3cret")
	defer os.Unsetenv("TEST_SECRET_KEY")

	data := map[string]any{
		"api_key": map[string]any{
			"$secret": "env",
			"key":     "TEST_SECRET_KEY",
		},
	}
	resolved := ResolveSecrets(data)
	if resolved["api_key"] != "s3cret" {
		t.Fatalf("expected s3cret, got %v", resolved["api_key"])
	}
}

func TestResolveSecretsFile(t *testing.T) {
	dir, _ := os.MkdirTemp("", "tori-cfg-*")
	defer os.RemoveAll(dir)
	secretFile := filepath.Join(dir, "secret.txt")
	os.WriteFile(secretFile, []byte("file_secret\n"), 0o644)

	data := map[string]any{
		"password": map[string]any{
			"$secret": "file",
			"key":     secretFile,
		},
	}
	resolved := ResolveSecrets(data)
	if resolved["password"] != "file_secret" {
		t.Fatalf("expected file_secret, got %v", resolved["password"])
	}
}

func TestApplyDefaults(t *testing.T) {
	schema := SchemaMap{
		"port":  {Type: TypeInt, Default: float64(9090)},
		"debug": {Type: TypeBool, Default: false},
	}
	data := map[string]any{}
	applyDefaults(data, schema)
	if data["port"] != float64(9090) {
		t.Fatalf("expected default port 9090, got %v", data["port"])
	}
}

func TestGetByPath(t *testing.T) {
	data := map[string]any{
		"gateway": map[string]any{
			"port": float64(9090),
		},
	}
	val, ok := getByPath(data, "gateway.port")
	if !ok || val != float64(9090) {
		t.Fatalf("expected 9090, got %v", val)
	}
	_, ok = getByPath(data, "gateway.missing")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestLoaderLoadAndGet(t *testing.T) {
	dir, _ := os.MkdirTemp("", "tori-cfg-*")
	defer os.RemoveAll(dir)

	cfgPath := filepath.Join(dir, "config.json")
	os.WriteFile(cfgPath, []byte(`{
		// Tori config
		"name": "tori",
		"port": 9090,
	}`), 0o644)

	schema := SchemaMap{
		"name": {Type: TypeString, Required: true},
		"port": {Type: TypeInt},
	}

	loader := NewLoader(cfgPath, schema, ReloadOff)
	cfg, errs, err := loader.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	if cfg["name"] != "tori" {
		t.Fatalf("expected tori, got %v", cfg["name"])
	}

	// GetValue
	val, ok := loader.GetValue("port")
	if !ok || val != float64(9090) {
		t.Fatalf("expected 9090, got %v", val)
	}
}

func TestLoaderHotReload(t *testing.T) {
	dir, _ := os.MkdirTemp("", "tori-cfg-*")
	defer os.RemoveAll(dir)

	cfgPath := filepath.Join(dir, "config.json")
	os.WriteFile(cfgPath, []byte(`{"name": "v1"}`), 0o644)

	schema := SchemaMap{"name": {Type: TypeString}}
	loader := NewLoader(cfgPath, schema, ReloadHot)

	var changed bool
	loader.OnChange(func(old, new map[string]any) {
		changed = true
	})

	loader.Load()
	loader.Watch(50 * time.Millisecond)
	defer loader.Stop()

	// Modify file
	time.Sleep(100 * time.Millisecond)
	os.WriteFile(cfgPath, []byte(`{"name": "v2"}`), 0o644)
	time.Sleep(200 * time.Millisecond)

	if !changed {
		t.Fatal("change handler not called")
	}
	val, _ := loader.GetValue("name")
	if val != "v2" {
		t.Fatalf("expected v2, got %v", val)
	}
}

func TestLoaderInclude(t *testing.T) {
	dir, _ := os.MkdirTemp("", "tori-cfg-*")
	defer os.RemoveAll(dir)

	// Included file
	os.WriteFile(filepath.Join(dir, "models.json"), []byte(`{"default_model": "gpt-4"}`), 0o644)

	// Main config with $include
	cfgPath := filepath.Join(dir, "config.json")
	os.WriteFile(cfgPath, []byte(`{
		"name": "tori",
		"$include": "models.json"
	}`), 0o644)

	loader := NewLoader(cfgPath, nil, ReloadOff)
	cfg, _, err := loader.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg["default_model"] != "gpt-4" {
		t.Fatalf("include not resolved: %v", cfg)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
