package cognisdk

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var packExtensions = map[string]string{
	".json":      "json",
	".yaml":      "yaml",
	".yml":       "yaml",
	".pack.json": "json",
	".pack.yaml": "yaml",
	".pack.yml":  "yaml",
}

// LoadPackManifest reads a PackManifest from JSON or YAML and validates it.
func LoadPackManifest(path string) (*PackManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cognisdk: read %q: %w", path, err)
	}

	var pack PackManifest
	switch detectPackFormat(path) {
	case "yaml":
		if err := unmarshalPackYAML(data, &pack); err != nil {
			return nil, fmt.Errorf("cognisdk: parse yaml %q: %w", path, err)
		}
	default:
		if err := json.Unmarshal(data, &pack); err != nil {
			return nil, fmt.Errorf("cognisdk: parse json %q: %w", path, err)
		}
	}

	if strings.TrimSpace(pack.ID) == "" {
		base := filepath.Base(path)
		base = strings.TrimSuffix(base, filepath.Ext(base))
		base = strings.TrimSuffix(base, ".pack")
		pack.ID = base
	}
	if err := ValidatePack(pack); err != nil {
		return nil, fmt.Errorf("cognisdk: validate %q: %w", path, err)
	}
	return &pack, nil
}

// LoadPacksFromDir scans a directory for pack manifests.
func LoadPacksFromDir(dir string) ([]PackManifest, []PackLoadError, error) {
	if dir == "" {
		return nil, nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("cognisdk: scan dir %q: %w", dir, err)
	}

	var packs []PackManifest
	var loadErrs []PackLoadError
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isPackFile(name) {
			continue
		}
		full := filepath.Join(dir, name)
		pack, err := LoadPackManifest(full)
		if err != nil {
			loadErrs = append(loadErrs, PackLoadError{Path: full, Err: err})
			continue
		}
		packs = append(packs, *pack)
	}
	sort.Slice(packs, func(i, j int) bool { return packs[i].ID < packs[j].ID })
	return packs, loadErrs, nil
}

// NewPackManagerFromDir loads packs from a directory and returns a manager.
func NewPackManagerFromDir(dir string) (*PackManager, []PackLoadError, error) {
	packs, loadErrs, err := LoadPacksFromDir(dir)
	if err != nil {
		return nil, nil, err
	}
	pm := NewPackManager()
	for _, pack := range packs {
		if err := pm.Add(pack); err != nil {
			loadErrs = append(loadErrs, PackLoadError{Path: filepath.Join(dir, pack.ID), Err: err})
		}
	}
	return pm, loadErrs, nil
}

// SavePackManifest writes a pack manifest as pretty JSON.
func SavePackManifest(pack PackManifest, path string) error {
	if err := ValidatePack(pack); err != nil {
		return err
	}
	data, err := json.MarshalIndent(pack, "", "  ")
	if err != nil {
		return fmt.Errorf("cognisdk: marshal %q: %w", pack.ID, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("cognisdk: write %q: %w", path, err)
	}
	return nil
}

// PackLoadError captures a manifest parse/validation failure.
type PackLoadError struct {
	Path string
	Err  error
}

func (l PackLoadError) Error() string {
	return fmt.Sprintf("%s: %v", l.Path, l.Err)
}

func detectPackFormat(path string) string {
	lower := strings.ToLower(path)
	for ext, format := range packExtensions {
		if strings.HasSuffix(lower, ext) {
			return format
		}
	}
	return "json"
}

func isPackFile(name string) bool {
	lower := strings.ToLower(name)
	for ext := range packExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func unmarshalPackYAML(data []byte, pack *PackManifest) error {
	var raw interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}
	converted := convertPackYAMLToJSON(raw)
	jsonData, err := json.Marshal(converted)
	if err != nil {
		return fmt.Errorf("yaml→json conversion: %w", err)
	}
	return json.Unmarshal(jsonData, pack)
}

func convertPackYAMLToJSON(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, v := range val {
			out[k] = convertPackYAMLToJSON(v)
		}
		return out
	case []interface{}:
		for i, item := range val {
			val[i] = convertPackYAMLToJSON(item)
		}
		return val
	default:
		return val
	}
}
