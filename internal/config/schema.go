package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"yunque-agent/pkg/safego"
)

// ──────────────────────────────────────────────
// Schema validation
// ──────────────────────────────────────────────

// FieldType for schema validation.
type FieldType string

const (
	TypeString  FieldType = "string"
	TypeInt     FieldType = "int"
	TypeBool    FieldType = "bool"
	TypeFloat   FieldType = "float"
	TypeArray   FieldType = "array"
	TypeObject  FieldType = "object"
	TypeAny     FieldType = "any"
)

// FieldSchema defines validation for a single config field.
type FieldSchema struct {
	Type        FieldType    `json:"type"`
	Required    bool         `json:"required,omitempty"`
	Default     any          `json:"default,omitempty"`
	Description string       `json:"description,omitempty"`
	Enum        []any        `json:"enum,omitempty"`      // allowed values
	Min         *float64     `json:"min,omitempty"`       // for int/float
	Max         *float64     `json:"max,omitempty"`       // for int/float
	Fields      SchemaMap    `json:"fields,omitempty"`    // for nested objects
	Items       *FieldSchema `json:"items,omitempty"`     // for arrays
}

// SchemaMap is a map of field names to their schemas.
type SchemaMap map[string]*FieldSchema

// ValidationError captures a single validation failure.
type ValidationError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// Validate checks a config map against a schema.
func Validate(data map[string]any, schema SchemaMap) []ValidationError {
	return validateObject(data, schema, "")
}

func validateObject(data map[string]any, schema SchemaMap, prefix string) []ValidationError {
	var errs []ValidationError

	for name, field := range schema {
		path := joinPath(prefix, name)
		val, exists := data[name]

		if !exists || val == nil {
			if field.Required {
				errs = append(errs, ValidationError{Path: path, Message: "required field missing"})
			}
			continue
		}

		errs = append(errs, validateField(val, field, path)...)
	}
	return errs
}

func validateField(val any, schema *FieldSchema, path string) []ValidationError {
	var errs []ValidationError

	switch schema.Type {
	case TypeString:
		s, ok := val.(string)
		if !ok {
			errs = append(errs, ValidationError{Path: path, Message: fmt.Sprintf("expected string, got %T", val)})
			return errs
		}
		if len(schema.Enum) > 0 && !containsAny(schema.Enum, s) {
			errs = append(errs, ValidationError{Path: path, Message: fmt.Sprintf("value %q not in enum %v", s, schema.Enum)})
		}

	case TypeInt:
		n, ok := toFloat64(val)
		if !ok {
			errs = append(errs, ValidationError{Path: path, Message: fmt.Sprintf("expected number, got %T", val)})
			return errs
		}
		if schema.Min != nil && n < *schema.Min {
			errs = append(errs, ValidationError{Path: path, Message: fmt.Sprintf("value %v below minimum %v", n, *schema.Min)})
		}
		if schema.Max != nil && n > *schema.Max {
			errs = append(errs, ValidationError{Path: path, Message: fmt.Sprintf("value %v above maximum %v", n, *schema.Max)})
		}

	case TypeFloat:
		n, ok := toFloat64(val)
		if !ok {
			errs = append(errs, ValidationError{Path: path, Message: fmt.Sprintf("expected number, got %T", val)})
			return errs
		}
		if schema.Min != nil && n < *schema.Min {
			errs = append(errs, ValidationError{Path: path, Message: fmt.Sprintf("value %v below minimum %v", n, *schema.Min)})
		}
		if schema.Max != nil && n > *schema.Max {
			errs = append(errs, ValidationError{Path: path, Message: fmt.Sprintf("value %v above maximum %v", n, *schema.Max)})
		}

	case TypeBool:
		if _, ok := val.(bool); !ok {
			errs = append(errs, ValidationError{Path: path, Message: fmt.Sprintf("expected bool, got %T", val)})
		}

	case TypeArray:
		arr, ok := val.([]any)
		if !ok {
			errs = append(errs, ValidationError{Path: path, Message: fmt.Sprintf("expected array, got %T", val)})
			return errs
		}
		if schema.Items != nil {
			for i, item := range arr {
				itemPath := fmt.Sprintf("%s[%d]", path, i)
				errs = append(errs, validateField(item, schema.Items, itemPath)...)
			}
		}

	case TypeObject:
		obj, ok := val.(map[string]any)
		if !ok {
			errs = append(errs, ValidationError{Path: path, Message: fmt.Sprintf("expected object, got %T", val)})
			return errs
		}
		if schema.Fields != nil {
			errs = append(errs, validateObject(obj, schema.Fields, path)...)
		}

	case TypeAny:
		// no validation
	}

	return errs
}

// ──────────────────────────────────────────────
// Secret references
// ──────────────────────────────────────────────

// SecretType determines how to resolve a secret.
type SecretType string

const (
	SecretEnv  SecretType = "env"  // from environment variable
	SecretFile SecretType = "file" // from file contents
)

// SecretRef is a reference to a secret value.
type SecretRef struct {
	Type SecretType `json:"type"`
	Key  string     `json:"key"` // env var name or file path
}

// ResolveSecrets replaces SecretRef objects in the config with their values.
func ResolveSecrets(data map[string]any) map[string]any {
	return resolveSecretsRecursive(data).(map[string]any)
}

func resolveSecretsRecursive(val any) any {
	switch v := val.(type) {
	case map[string]any:
		// Check if it's a SecretRef
		if typ, ok := v["$secret"].(string); ok {
			key, _ := v["key"].(string)
			switch SecretType(typ) {
			case SecretEnv:
				return os.Getenv(key)
			case SecretFile:
				data, err := os.ReadFile(key)
				if err != nil {
					slog.Warn("config: secret file read failed", "key", key, "err", err)
					return ""
				}
				return strings.TrimSpace(string(data))
			}
		}
		result := make(map[string]any, len(v))
		for k, child := range v {
			result[k] = resolveSecretsRecursive(child)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, child := range v {
			result[i] = resolveSecretsRecursive(child)
		}
		return result
	default:
		return val
	}
}

// ──────────────────────────────────────────────
// Config loader with hot reload
// ──────────────────────────────────────────────

// ReloadMode controls how config changes are applied.
type ReloadMode string

const (
	ReloadHot     ReloadMode = "hot"     // apply immediately
	ReloadRestart ReloadMode = "restart" // requires restart
	ReloadOff     ReloadMode = "off"     // ignore changes
)

// ChangeHandler is called when config changes.
type ChangeHandler func(old, new map[string]any)

// Loader loads, validates, and watches config files.
type Loader struct {
	mu       sync.RWMutex
	path     string
	schema   SchemaMap
	current  map[string]any
	reload   ReloadMode
	handlers []ChangeHandler
	stopCh   chan struct{}
	stopped  bool
}

// NewLoader creates a config loader.
func NewLoader(path string, schema SchemaMap, reload ReloadMode) *Loader {
	return &Loader{
		path:   path,
		schema: schema,
		reload: reload,
		stopCh: make(chan struct{}),
	}
}

// OnChange registers a handler for config changes.
func (l *Loader) OnChange(h ChangeHandler) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.handlers = append(l.handlers, h)
}

// Load reads, parses, validates, and resolves the config file.
func (l *Loader) Load() (map[string]any, []ValidationError, error) {
	data, err := os.ReadFile(l.path)
	if err != nil {
		return nil, nil, fmt.Errorf("config: read %s: %w", l.path, err)
	}

	// Parse JSON (with comment stripping for JSON5-like support)
	cleaned := stripComments(string(data))
	cleaned = stripTrailingCommas(cleaned)

	var raw map[string]any
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, nil, fmt.Errorf("config: parse %s: %w", l.path, err)
	}

	// Process $include directives
	raw = processIncludes(raw, filepath.Dir(l.path))

	// Validate against schema
	var validationErrs []ValidationError
	if l.schema != nil {
		validationErrs = Validate(raw, l.schema)
	}

	// Apply defaults
	if l.schema != nil {
		applyDefaults(raw, l.schema)
	}

	// Resolve secrets
	raw = ResolveSecrets(raw)

	l.mu.Lock()
	l.current = raw
	l.mu.Unlock()

	return raw, validationErrs, nil
}

// Get returns the current config snapshot.
func (l *Loader) Get() map[string]any {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return deepCopy(l.current)
}

// GetValue retrieves a value by dot-separated path (e.g. "gateway.port").
func (l *Loader) GetValue(path string) (any, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return getByPath(l.current, path)
}

// Watch starts polling the config file for changes.
func (l *Loader) Watch(interval time.Duration) {
	if l.reload == ReloadOff {
		return
	}
	safego.Go("config-watcher", func() {
		var lastMod time.Time
		info, err := os.Stat(l.path)
		if err == nil {
			lastMod = info.ModTime()
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-l.stopCh:
				return
			case <-ticker.C:
				info, err := os.Stat(l.path)
				if err != nil {
					continue
				}
				if !info.ModTime().After(lastMod) {
					continue
				}
				lastMod = info.ModTime()

				old := l.Get()
				newCfg, errs, err := l.Load()
				if err != nil {
					slog.Warn("config: reload failed", "err", err)
					continue
				}
				if len(errs) > 0 {
					slog.Warn("config: reload validation errors", "count", len(errs))
					for _, e := range errs {
						slog.Warn("config: validation", "path", e.Path, "msg", e.Message)
					}
				}

				slog.Info("config: reloaded", "path", l.path)

				l.mu.RLock()
				handlers := make([]ChangeHandler, len(l.handlers))
				copy(handlers, l.handlers)
				l.mu.RUnlock()

				for _, h := range handlers {
					h(old, newCfg)
				}
			}
		}
	})
}

// Stop stops watching.
func (l *Loader) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.stopped {
		l.stopped = true
		close(l.stopCh)
	}
}

// ──────────────────────────────────────────────
// JSON5-like helpers
// ──────────────────────────────────────────────

// stripComments removes // and /* */ comments from JSON-like content.
func stripComments(s string) string {
	var result strings.Builder
	inString := false
	inLineComment := false
	inBlockComment := false
	i := 0
	for i < len(s) {
		if inLineComment {
			if s[i] == '\n' {
				inLineComment = false
				result.WriteByte('\n')
			}
			i++
			continue
		}
		if inBlockComment {
			if i+1 < len(s) && s[i] == '*' && s[i+1] == '/' {
				inBlockComment = false
				i += 2
			} else {
				i++
			}
			continue
		}
		if s[i] == '"' {
			inString = !inString
			result.WriteByte(s[i])
			i++
			continue
		}
		if !inString && i+1 < len(s) {
			if s[i] == '/' && s[i+1] == '/' {
				inLineComment = true
				i += 2
				continue
			}
			if s[i] == '/' && s[i+1] == '*' {
				inBlockComment = true
				i += 2
				continue
			}
		}
		result.WriteByte(s[i])
		i++
	}
	return result.String()
}

// stripTrailingCommas removes trailing commas before } or ].
func stripTrailingCommas(s string) string {
	var result strings.Builder
	inString := false
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '"' {
			inString = !inString
		}
		if !inString && runes[i] == ',' {
			// Look ahead for closing bracket
			j := i + 1
			for j < len(runes) && (runes[j] == ' ' || runes[j] == '\t' || runes[j] == '\n' || runes[j] == '\r') {
				j++
			}
			if j < len(runes) && (runes[j] == '}' || runes[j] == ']') {
				continue // skip trailing comma
			}
		}
		result.WriteRune(runes[i])
	}
	return result.String()
}

// processIncludes resolves $include directives in config.
func processIncludes(data map[string]any, baseDir string) map[string]any {
	for k, v := range data {
		if k == "$include" {
			if path, ok := v.(string); ok {
				abs := filepath.Join(baseDir, path)
				incData, err := os.ReadFile(abs)
				if err != nil {
					slog.Warn("config: include failed", "path", path, "err", err)
					delete(data, k)
					continue
				}
				cleaned := stripComments(string(incData))
				cleaned = stripTrailingCommas(cleaned)
				var included map[string]any
				if err := json.Unmarshal([]byte(cleaned), &included); err != nil {
					slog.Warn("config: include parse failed", "path", path, "err", err)
					delete(data, k)
					continue
				}
				// Merge included into data (included fields don't override existing)
				for ik, iv := range included {
					if _, exists := data[ik]; !exists {
						data[ik] = iv
					}
				}
				delete(data, k)
			}
			continue
		}
		if obj, ok := v.(map[string]any); ok {
			data[k] = processIncludes(obj, baseDir)
		}
	}
	return data
}

// ──────────────────────────────────────────────
// Utility helpers
// ──────────────────────────────────────────────

func applyDefaults(data map[string]any, schema SchemaMap) {
	for name, field := range schema {
		if _, exists := data[name]; !exists && field.Default != nil {
			data[name] = field.Default
		}
		if field.Type == TypeObject && field.Fields != nil {
			if obj, ok := data[name].(map[string]any); ok {
				applyDefaults(obj, field.Fields)
			}
		}
	}
}

func getByPath(data map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	current := any(data)
	for _, part := range parts {
		obj, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = obj[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func deepCopy(data map[string]any) map[string]any {
	if data == nil {
		return nil
	}
	b, _ := json.Marshal(data)
	var result map[string]any
	json.Unmarshal(b, &result)
	return result
}

func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func containsAny(enum []any, val any) bool {
	for _, e := range enum {
		if reflect.DeepEqual(e, val) {
			return true
		}
	}
	return false
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}
