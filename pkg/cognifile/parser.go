package cognifile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Recognized file extensions (order matters for compound extensions).
var extensions = []string{
	".cognifile.yaml",
	".cognifile.yml",
	".cognifile.json",
	".cognifile",
}

// Parse reads a Cognifile from raw bytes, auto-detecting JSON or YAML.
func Parse(data []byte) (*Cognifile, error) {
	data = stripBOM(data)

	var cf Cognifile
	if isJSON(data) {
		if err := json.Unmarshal(data, &cf); err != nil {
			return nil, fmt.Errorf("cognifile: parse json: %w", err)
		}
	} else {
		if err := unmarshalYAML(data, &cf); err != nil {
			return nil, fmt.Errorf("cognifile: parse yaml: %w", err)
		}
	}

	applyDefaults(&cf)
	if err := cf.Validate(); err != nil {
		return nil, err
	}
	return &cf, nil
}

const maxCognifileSize = 10 * 1024 * 1024 // 10 MB

// LoadFile reads a Cognifile from disk.
func LoadFile(path string) (*Cognifile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cognifile: stat %q: %w", path, err)
	}
	if info.Size() > maxCognifileSize {
		return nil, fmt.Errorf("cognifile: %q is too large (%d bytes, limit %d bytes)", path, info.Size(), maxCognifileSize)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cognifile: read %q: %w", path, err)
	}
	cf, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("cognifile: %q: %w", path, err)
	}
	return cf, nil
}

// ScanDir scans a directory for Cognifile files and returns all valid ones.
// Invalid files are reported as errors but do not prevent other files from loading.
func ScanDir(dir string) ([]*Cognifile, []ScanError, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("cognifile: scan %q: %w", dir, err)
	}

	var files []*Cognifile
	var scanErrs []ScanError

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isCognifileFile(name) {
			continue
		}
		path := filepath.Join(dir, name)
		cf, err := LoadFile(path)
		if err != nil {
			scanErrs = append(scanErrs, ScanError{Path: path, Err: err})
			continue
		}
		files = append(files, cf)
	}
	return files, scanErrs, nil
}

// SaveFile writes a Cognifile to disk as YAML (the canonical human-editable format).
func SaveFile(cf *Cognifile, path string) error {
	if err := cf.Validate(); err != nil {
		return err
	}

	if cf.Schema == "" {
		cf.Schema = SchemaVersion
	}

	data, err := yaml.Marshal(cf)
	if err != nil {
		return fmt.Errorf("cognifile: marshal: %w", err)
	}

	header := "# Cognifile — declarative AI Agent specification\n" +
		"# https://yunque.owo.today/docs/cognifile\n\n"

	return os.WriteFile(path, []byte(header+string(data)), 0o644)
}

// ScanError records a per-file parse failure.
type ScanError struct {
	Path string
	Err  error
}

func (e ScanError) Error() string {
	return fmt.Sprintf("%s: %v", e.Path, e.Err)
}

func isCognifileFile(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range extensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func isJSON(data []byte) bool {
	for _, b := range data {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		case '{', '[':
			return true
		default:
			return false
		}
	}
	return false
}

func stripBOM(data []byte) []byte {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return data[3:]
	}
	return data
}

// unmarshalYAML parses YAML into a Cognifile via a JSON round-trip to reuse
// the existing JSON struct tags (same approach as pkg/cogni/loader.go).
func unmarshalYAML(data []byte, cf *Cognifile) error {
	var raw interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}
	converted := normalizeYAML(raw)
	jsonData, err := json.Marshal(converted)
	if err != nil {
		return fmt.Errorf("yaml→json: %w", err)
	}
	return json.Unmarshal(jsonData, cf)
}

// normalizeYAML converts YAML map keys from interface{} to string.
func normalizeYAML(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, v := range val {
			out[k] = normalizeYAML(v)
		}
		return out
	case []interface{}:
		for i, item := range val {
			val[i] = normalizeYAML(item)
		}
		return val
	default:
		return val
	}
}

func applyDefaults(cf *Cognifile) {
	if cf.Schema == "" {
		cf.Schema = SchemaVersion
	}
	if cf.Version == "" {
		cf.Version = "0.1.0"
	}
	if cf.Persona.Language == "" {
		cf.Persona.Language = "zh-CN"
	}
	if cf.Model.Tier == "" {
		cf.Model.Tier = "smart"
	}
}
