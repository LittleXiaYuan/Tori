package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"yunque-agent/pkg/skills"
)

// ScriptPlugin is a plugin defined by a YAML manifest + script files.
// Users can create plugins by dropping a folder into the plugins directory:
//
//	plugins/
//	  my-plugin/
//	    plugin.yaml        # manifest
//	    handler.py         # or handler.js / handler.sh
//
// plugin.yaml format:
//
//	name: my-plugin
//	description: Does something useful
//	language: python         # python | node | shell
//	system_prompt: |
//	  You can use the my_tool skill to do X.
//	skills:
//	  - name: my_tool
//	    description: Does X with Y
//	    parameters:
//	      input: {type: string, description: "The input"}
//	    handler: handler.py   # relative to plugin dir
type ScriptPlugin struct {
	dir      string
	manifest Manifest
	skills   []skills.Skill
	apiToken string // injected by agent runtime for SDK access
}

// SetAPIToken sets the plugin-scoped API token for SDK calls.
// This is called by the agent when issuing tokens during init.
// The token is propagated to all child scriptSkill instances.
func (s *ScriptPlugin) SetAPIToken(token string) {
	s.apiToken = token
	for _, sk := range s.skills {
		if ss, ok := sk.(*scriptSkill); ok {
			ss.token = token
		}
	}
}

// APIToken returns the plugin's API token.
func (s *ScriptPlugin) APIToken() string { return s.apiToken }

// PluginType defines how a plugin runs.
type PluginType string

const (
	PluginTypeFunction PluginType = "function" // one-shot skill execution (default)
	PluginTypeService  PluginType = "service"  // long-running background process
)

// Manifest is the YAML-compatible plugin definition.
type Manifest struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Language     string          `json:"language"` // python, node, shell
	SystemPrompt string          `json:"system_prompt"`
	Skills       []SkillManifest `json:"skills"`
	Type         PluginType      `json:"type,omitempty"`         // "function" or "service"
	Slot         string          `json:"slot,omitempty"`         // exclusive slot (e.g. "memory", "channel-whatsapp")
	Hooks        []string        `json:"hooks,omitempty"`        // events to subscribe to
	Entrypoint   string          `json:"entrypoint,omitempty"`   // for service: startup command/script
	Port         int             `json:"port,omitempty"`         // for service: communication port
	HealthCheck  string          `json:"health_check,omitempty"` // for service: health endpoint path
	UI           *UIManifest     `json:"ui,omitempty"`           // UI tab & API route declarations
}

// UIManifest declares UI tabs and API routes for a plugin.
type UIManifest struct {
	Tabs []UITabManifest `json:"tabs"`          // tabs to register in web dashboard
	API  []APIRoute      `json:"api,omitempty"` // custom API endpoints
}

// UITabManifest declares a single navigation tab.
type UITabManifest struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	LabelEn     string `json:"label_en,omitempty"`
	Icon        string `json:"icon"`
	Description string `json:"description,omitempty"`
}

// APIRoute declares a custom API endpoint.
type APIRoute struct {
	Method  string `json:"method"`  // GET, POST, etc.
	Path    string `json:"path"`    // relative path, e.g. "/upload"
	Handler string `json:"handler"` // script filename to execute
}

// SkillManifest defines a single skill within a plugin.
type SkillManifest struct {
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	Parameters  map[string]ParameterField `json:"parameters"`
	Handler     string                    `json:"handler"` // script filename
}

// ParameterField describes a skill parameter.
type ParameterField struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required,omitempty"`
	Default     string `json:"default,omitempty"`
}

// LoadScriptPlugin loads a plugin from a directory containing plugin.yaml.
func LoadScriptPlugin(dir string) (*ScriptPlugin, error) {
	manifestPath := filepath.Join(dir, "plugin.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		// Try plugin.json as fallback
		manifestPath = filepath.Join(dir, "plugin.json")
		data, err = os.ReadFile(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("no plugin.yaml or plugin.json in %s", dir)
		}
	}

	var m Manifest
	if strings.HasSuffix(manifestPath, ".json") {
		err = json.Unmarshal(data, &m)
	} else {
		err = parseYAML(data, &m)
	}
	if err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	if m.Name == "" {
		m.Name = filepath.Base(dir)
	}
	if m.Language == "" {
		m.Language = "python"
	}

	sp := &ScriptPlugin{dir: dir, manifest: m}

	// Build skills from manifest
	for _, sm := range m.Skills {
		sp.skills = append(sp.skills, &scriptSkill{
			name:        sm.Name,
			description: sm.Description,
			parameters:  sm.Parameters,
			handler:     sm.Handler,
			language:    m.Language,
			dir:         dir,
		})
	}

	slog.Info("loaded script plugin", "name", m.Name, "skills", len(sp.skills), "lang", m.Language)
	return sp, nil
}

func (s *ScriptPlugin) Name() string           { return s.manifest.Name }
func (s *ScriptPlugin) Description() string    { return s.manifest.Description }
func (s *ScriptPlugin) SystemPrompt() string   { return s.manifest.SystemPrompt }
func (s *ScriptPlugin) Skills() []skills.Skill { return s.skills }
func (s *ScriptPlugin) Dir() string            { return s.dir }
func (s *ScriptPlugin) Manifest() Manifest     { return s.manifest }

// CallHook invokes the plugin's hook handler script for a lifecycle event.
// The handler script is named "hook.py" / "hook.js" / "hook.sh" in the plugin directory.
// If no hook script exists, the call is a no-op (returns "", nil).
// The hook payload is passed as JSON via stdin and via HOOK_EVENT/HOOK_DATA env vars.
func (s *ScriptPlugin) CallHook(ctx context.Context, payload HookPayload) (string, error) {
	// Determine hook script filename
	var hookScript string
	switch s.manifest.Language {
	case "node":
		hookScript = "hook.js"
	case "shell":
		hookScript = "hook.sh"
	default:
		hookScript = "hook.py"
	}
	hookPath := filepath.Join(s.dir, hookScript)
	if _, err := os.Stat(hookPath); err != nil {
		return "", nil // no hook script, silently skip
	}

	payloadJSON, _ := json.Marshal(payload)
	dataJSON, _ := json.Marshal(payload.Data)

	envVars := os.Environ()
	envVars = append(envVars,
		"HOOK_EVENT="+payload.Event,
		"HOOK_DATA="+string(dataJSON),
		"PLUGIN_NAME="+s.manifest.Name,
		"PLUGIN_DIR="+s.dir,
	)

	hookCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	switch s.manifest.Language {
	case "node":
		cmd = exec.CommandContext(hookCtx, "node", hookPath)
	case "shell":
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(hookCtx, "cmd", "/c", hookPath)
		} else {
			cmd = exec.CommandContext(hookCtx, "sh", hookPath)
		}
	default: // python
		interpreter := "python3"
		if runtime.GOOS == "windows" {
			interpreter = "python"
		}
		cmd = exec.CommandContext(hookCtx, interpreter, hookPath)
	}
	cmd.Dir = s.dir
	cmd.Env = envVars
	cmd.Stdin = strings.NewReader(string(payloadJSON))

	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Warn("plugin hook script error", "plugin", s.manifest.Name, "event", payload.Event, "err", err)
		return string(out), err
	}
	return strings.TrimSpace(string(out)), nil
}

// UITabs implements UIPlugin. Returns tabs declared in the manifest's ui section.
func (s *ScriptPlugin) UITabs() []UITab {
	if s.manifest.UI == nil {
		return nil
	}
	tabs := make([]UITab, 0, len(s.manifest.UI.Tabs))
	for _, t := range s.manifest.UI.Tabs {
		tabs = append(tabs, UITab{
			Key:         t.Key,
			Label:       t.Label,
			LabelEn:     t.LabelEn,
			Icon:        t.Icon,
			Description: t.Description,
			Plugin:      s.manifest.Name,
		})
	}
	return tabs
}

// HTTPHandlers implements UIPlugin. Returns script-backed HTTP handlers for each API route.
func (s *ScriptPlugin) HTTPHandlers() map[string]http.HandlerFunc {
	if s.manifest.UI == nil || len(s.manifest.UI.API) == 0 {
		return nil
	}
	handlers := make(map[string]http.HandlerFunc, len(s.manifest.UI.API))
	for _, route := range s.manifest.UI.API {
		handlerScript := route.Handler
		handlers[route.Path] = s.makeScriptHTTPHandler(handlerScript)
	}
	return handlers
}

// makeScriptHTTPHandler creates an http.HandlerFunc that runs a script and returns its output.
func (s *ScriptPlugin) makeScriptHTTPHandler(handlerScript string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handlerPath := filepath.Join(s.dir, handlerScript)

		// Read request body
		body, _ := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024)) // 10MB limit
		defer r.Body.Close()

		// Build environment
		envVars := os.Environ()
		envVars = append(envVars, "REQUEST_METHOD="+r.Method)
		envVars = append(envVars, "REQUEST_PATH="+r.URL.Path)
		envVars = append(envVars, "REQUEST_QUERY="+r.URL.RawQuery)
		envVars = append(envVars, "REQUEST_BODY="+string(body))
		envVars = append(envVars, "CONTENT_TYPE="+r.Header.Get("Content-Type"))
		envVars = append(envVars, "PLUGIN_NAME="+s.manifest.Name)
		envVars = append(envVars, "PLUGIN_DIR="+s.dir)

		// Determine interpreter
		var cmd *exec.Cmd
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()

		switch s.manifest.Language {
		case "python":
			interpreter := "python3"
			if runtime.GOOS == "windows" {
				interpreter = "python"
			}
			cmd = exec.CommandContext(ctx, interpreter, handlerPath)
		case "node":
			cmd = exec.CommandContext(ctx, "node", handlerPath)
		case "shell":
			if runtime.GOOS == "windows" {
				cmd = exec.CommandContext(ctx, "cmd", "/c", handlerPath)
			} else {
				cmd = exec.CommandContext(ctx, "sh", handlerPath)
			}
		default:
			http.Error(w, "unsupported plugin language", http.StatusInternalServerError)
			return
		}

		cmd.Dir = s.dir
		cmd.Env = envVars
		cmd.Stdin = strings.NewReader(string(body))

		out, err := cmd.CombinedOutput()
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error":  fmt.Sprintf("plugin script error: %v", err),
				"output": string(out),
			})
			return
		}

		// Script output is the response body. Default to JSON.
		w.Header().Set("Content-Type", "application/json")
		w.Write(out)
	}
}

// scriptSkill implements skills.Skill by executing an external script.
type scriptSkill struct {
	name        string
	description string
	parameters  map[string]ParameterField
	handler     string
	language    string
	dir         string
	token       string // API token for yunque SDK (inherited from parent ScriptPlugin)
}

func (s *scriptSkill) Name() string        { return s.name }
func (s *scriptSkill) Description() string { return s.description }

func (s *scriptSkill) Parameters() map[string]any {
	props := map[string]any{}
	var required []string
	for k, v := range s.parameters {
		props[k] = map[string]any{"type": v.Type, "description": v.Description}
		if v.Required {
			required = append(required, k)
		}
	}
	schema := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func (s *scriptSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	handlerPath := filepath.Join(s.dir, s.handler)

	// Pass arguments as JSON via stdin (and also as env vars)
	argsJSON, _ := json.Marshal(args)

	// Build environment with yunque SDK variables
	envVars := os.Environ()
	envVars = append(envVars, "PLUGIN_ARGS="+string(argsJSON))
	envVars = append(envVars, "PLUGIN_SKILL="+s.name)
	agentPort := os.Getenv("AGENT_PORT")
	if agentPort == "" {
		agentPort = "9090"
	}
	envVars = append(envVars, "YUNQUE_API_BASE=http://localhost:"+agentPort)
	envVars = append(envVars, "YUNQUE_PLUGIN_NAME="+s.name)
	envVars = append(envVars, "YUNQUE_PLUGIN_DIR="+s.dir)
	if s.token != "" {
		envVars = append(envVars, "YUNQUE_PLUGIN_TOKEN="+s.token)
	}
	if env != nil {
		envVars = append(envVars, "TENANT_ID="+env.TenantID)
	}

	// Determine interpreter
	var cmd *exec.Cmd
	switch s.language {
	case "python":
		interpreter := "python3"
		if runtime.GOOS == "windows" {
			interpreter = "python"
		}
		cmd = exec.CommandContext(ctx, interpreter, handlerPath)
	case "node":
		cmd = exec.CommandContext(ctx, "node", handlerPath)
	case "shell":
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(ctx, "cmd", "/c", handlerPath)
		} else {
			cmd = exec.CommandContext(ctx, "sh", handlerPath)
		}
	default:
		return "", fmt.Errorf("unsupported language: %s", s.language)
	}

	cmd.Dir = s.dir
	cmd.Env = envVars
	cmd.Stdin = strings.NewReader(string(argsJSON))

	execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd = exec.CommandContext(execCtx, cmd.Path, cmd.Args[1:]...)
	cmd.Dir = s.dir
	cmd.Env = envVars
	cmd.Stdin = strings.NewReader(string(argsJSON))

	out, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(out))
	if len(result) > 64*1024 {
		result = result[:64*1024] + "\n...(truncated)"
	}
	if err != nil {
		return fmt.Sprintf("error: %v\n%s", err, result), nil
	}
	return result, nil
}

// parseYAML is a minimal YAML-subset parser for plugin manifests.
// Supports the flat key-value + skills array format without importing a YAML library.
// For full YAML, users can use plugin.json instead.
func parseYAML(data []byte, m *Manifest) error {
	// Simple approach: convert our subset of YAML to JSON, then unmarshal
	lines := strings.Split(string(data), "\n")
	jsonMap := map[string]any{}
	var currentSkill map[string]any
	var currentParams map[string]any
	var inSkills, inParams bool
	var multilineKey string
	var multilineVal strings.Builder

	flushMultiline := func() {
		if multilineKey != "" {
			jsonMap[multilineKey] = strings.TrimSpace(multilineVal.String())
			multilineKey = ""
			multilineVal.Reset()
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			if multilineKey != "" {
				multilineVal.WriteString("\n")
			}
			continue
		}

		// Detect multiline continuation
		if multilineKey != "" {
			indent := len(line) - len(strings.TrimLeft(line, " "))
			if indent >= 2 {
				multilineVal.WriteString(trimmed + "\n")
				continue
			}
			flushMultiline()
		}

		// Top-level key: value
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, "-") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				if val == "|" {
					multilineKey = key
					continue
				}
				if key == "skills" {
					inSkills = true
					inParams = false
					continue
				}
				if val != "" {
					jsonMap[key] = val
				}
				inSkills = false
				inParams = false
			}
			continue
		}

		if inSkills {
			if strings.HasPrefix(trimmed, "- name:") {
				if currentSkill != nil {
					if currentParams != nil {
						currentSkill["parameters"] = currentParams
					}
					appendSkill(jsonMap, currentSkill)
				}
				currentSkill = map[string]any{}
				currentParams = nil
				inParams = false
				currentSkill["name"] = strings.TrimSpace(strings.TrimPrefix(trimmed, "- name:"))
			} else if currentSkill != nil {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					val := strings.TrimSpace(parts[1])
					if key == "parameters" {
						inParams = true
						currentParams = map[string]any{}
						continue
					}
					if inParams {
						// parameter line: name: {type: string, description: "..."}
						paramVal := strings.TrimSpace(val)
						if strings.HasPrefix(paramVal, "{") {
							pf := ParameterField{}
							// Mini parse: {type: string, description: "..."}
							paramVal = strings.Trim(paramVal, "{}")
							for _, kv := range strings.Split(paramVal, ",") {
								kvParts := strings.SplitN(strings.TrimSpace(kv), ":", 2)
								if len(kvParts) == 2 {
									pk := strings.TrimSpace(kvParts[0])
									pv := strings.Trim(strings.TrimSpace(kvParts[1]), "\"'")
									switch pk {
									case "type":
										pf.Type = pv
									case "description":
										pf.Description = pv
									case "required":
										pf.Required = pv == "true"
									}
								}
							}
							currentParams[key] = pf
						}
					} else {
						currentSkill[key] = val
					}
				}
			}
		}
	}
	flushMultiline()
	if currentSkill != nil {
		if currentParams != nil {
			currentSkill["parameters"] = currentParams
		}
		appendSkill(jsonMap, currentSkill)
	}

	// Convert to JSON and unmarshal
	m.Name, _ = jsonMap["name"].(string)
	m.Description, _ = jsonMap["description"].(string)
	m.Language, _ = jsonMap["language"].(string)
	m.SystemPrompt, _ = jsonMap["system_prompt"].(string)
	m.Slot, _ = jsonMap["slot"].(string)
	if t, ok := jsonMap["type"].(string); ok {
		m.Type = PluginType(t)
	}
	m.Entrypoint, _ = jsonMap["entrypoint"].(string)
	m.HealthCheck, _ = jsonMap["health_check"].(string)

	if skillsRaw, ok := jsonMap["_skills"]; ok {
		if skillsList, ok := skillsRaw.([]map[string]any); ok {
			for _, s := range skillsList {
				sm := SkillManifest{
					Name:        s["name"].(string),
					Description: fmt.Sprintf("%v", s["description"]),
					Handler:     fmt.Sprintf("%v", s["handler"]),
				}
				if params, ok := s["parameters"].(map[string]any); ok {
					sm.Parameters = make(map[string]ParameterField)
					for k, v := range params {
						if pf, ok := v.(ParameterField); ok {
							sm.Parameters[k] = pf
						}
					}
				}
				m.Skills = append(m.Skills, sm)
			}
		}
	}
	return nil
}

func appendSkill(m map[string]any, skill map[string]any) {
	existing, _ := m["_skills"].([]map[string]any)
	m["_skills"] = append(existing, skill)
}
