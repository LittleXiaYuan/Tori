package cogni

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Supported file extensions for Cogni declarations.
var cogniExtensions = map[string]string{
	".json":       "json",
	".yaml":       "yaml",
	".yml":        "yaml",
	".cogni.json": "json",
	".cogni.yaml": "yaml",
	".cogni.yml":  "yaml",
}

// LoadDeclaration reads a single Cogni Declaration from a JSON or YAML file
// and validates it. Supported extensions: .json, .yaml, .yml, .cogni.json,
// .cogni.yaml, .cogni.yml.
func LoadDeclaration(path string) (*Declaration, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cogni: read %q: %w", path, err)
	}

	format := detectFormat(path)

	var d Declaration
	switch format {
	case "yaml":
		if err := unmarshalYAML(data, &d); err != nil {
			return nil, fmt.Errorf("cogni: parse yaml %q: %w", path, err)
		}
	default:
		if err := json.Unmarshal(data, &d); err != nil {
			return nil, fmt.Errorf("cogni: parse json %q: %w", path, err)
		}
	}

	if strings.TrimSpace(d.ID) == "" {
		base := filepath.Base(path)
		base = strings.TrimSuffix(base, filepath.Ext(base))
		base = strings.TrimSuffix(base, ".cogni")
		d.ID = base
	}
	if err := d.Validate(); err != nil {
		return nil, fmt.Errorf("cogni: validate %q: %w", path, err)
	}
	return &d, nil
}

// LoadDeclarationsFromDir scans `dir` (non-recursive) for *.json, *.yaml,
// *.yml, *.cogni.json, *.cogni.yaml, *.cogni.yml files and returns one
// Declaration per file.
//
// A missing directory is not an error; the function returns (nil, nil, nil).
func LoadDeclarationsFromDir(dir string) ([]*Declaration, []LoadError, error) {
	if dir == "" {
		return nil, nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("cogni: scan dir %q: %w", dir, err)
	}

	var out []*Declaration
	var loadErrs []LoadError
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !isCogniFile(name) {
			continue
		}
		full := filepath.Join(dir, name)
		decl, err := LoadDeclaration(full)
		if err != nil {
			loadErrs = append(loadErrs, LoadError{Path: full, Err: err})
			continue
		}
		out = append(out, decl)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, loadErrs, nil
}

func detectFormat(path string) string {
	lower := strings.ToLower(path)
	// Check compound extensions first (.cogni.yaml, .cogni.json, .cogni.yml)
	for ext, format := range cogniExtensions {
		if strings.HasSuffix(lower, ext) {
			return format
		}
	}
	return "json"
}

func isCogniFile(name string) bool {
	lower := strings.ToLower(name)
	for ext := range cogniExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// unmarshalYAML parses YAML into a Declaration. It first unmarshals into a
// generic map, then re-marshals to JSON and uses the existing JSON struct
// tags. This avoids duplicating struct tags across the Declaration hierarchy.
func unmarshalYAML(data []byte, d *Declaration) error {
	var raw interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}
	converted := convertYAMLToJSON(raw)
	jsonData, err := json.Marshal(converted)
	if err != nil {
		return fmt.Errorf("yaml→json conversion: %w", err)
	}
	return json.Unmarshal(jsonData, d)
}

// convertYAMLToJSON normalizes YAML-parsed values (map[string]interface{} with
// interface{} keys) into JSON-compatible types (map[string]interface{} with
// string keys).
func convertYAMLToJSON(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, v := range val {
			out[k] = convertYAMLToJSON(v)
		}
		return out
	case []interface{}:
		for i, item := range val {
			val[i] = convertYAMLToJSON(item)
		}
		return val
	default:
		return val
	}
}

// LoadError captures a per-file parse/validation failure produced by
// LoadDeclarationsFromDir. The directory scan continues even when individual
// files fail, so the caller sees a complete picture in one pass.
type LoadError struct {
	Path string
	Err  error
}

func (l LoadError) Error() string {
	return fmt.Sprintf("%s: %v", l.Path, l.Err)
}

// SaveDeclaration writes a Declaration to `path` as pretty-printed JSON,
// validating it first. The parent directory must exist.
func SaveDeclaration(d *Declaration, path string) error {
	if err := d.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("cogni: marshal %q: %w", d.ID, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("cogni: write %q: %w", path, err)
	}
	return nil
}
