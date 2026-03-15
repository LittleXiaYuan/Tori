package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"yunque-agent/pkg/manifest"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "init":
		if len(os.Args) < 3 {
			fatal("usage: tori plugin init <plugin-name>")
		}
		cmdInit(os.Args[2])
	case "validate":
		dir := "."
		if len(os.Args) >= 3 {
			dir = os.Args[2]
		}
		cmdValidate(dir)
	case "build":
		dir := "."
		if len(os.Args) >= 3 {
			dir = os.Args[2]
		}
		cmdBuild(dir)
	case "test":
		dir := "."
		if len(os.Args) >= 3 {
			dir = os.Args[2]
		}
		cmdTest(dir)
	case "install":
		if len(os.Args) < 3 {
			fatal("usage: tori plugin install <source>\n  source: local path, .zip file, or URL")
		}
		targetDir := "data/plugins"
		if len(os.Args) >= 4 {
			targetDir = os.Args[3]
		}
		cmdInstall(os.Args[2], targetDir)
	case "list":
		targetDir := "data/plugins"
		if len(os.Args) >= 3 {
			targetDir = os.Args[2]
		}
		cmdList(targetDir)
	case "version":
		fmt.Printf("tori-plugin %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`tori-plugin — Tori Plugin CLI

Usage:
  tori plugin init <name>              Create a new plugin project
  tori plugin validate [path]          Validate plugin structure and manifest
  tori plugin build [path]             Build plugin and compute signature
  tori plugin test [path]              Run plugin tests
  tori plugin install <source> [dir]   Install plugin from path, .zip, or URL
  tori plugin list [dir]               List installed plugins
  tori plugin version                  Show version

Examples:
  tori plugin init weather
  tori plugin install ./my-plugin
  tori plugin install https://example.com/plugin.zip
  tori plugin list`)
}

// --- init ---

type scaffoldData struct {
	Name      string // e.g. "weather"
	NameTitle string // e.g. "Weather"
	Module    string // e.g. "yunque-plugin-weather"
	SkillName string // e.g. "get_weather"
}

func cmdInit(name string) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		fatal("plugin name cannot be empty")
	}

	dir := filepath.Join(".", name)
	if _, err := os.Stat(dir); err == nil {
		fatal("directory %q already exists", dir)
	}

	data := scaffoldData{
		Name:      name,
		NameTitle: strings.ToUpper(name[:1]) + name[1:],
		Module:    "yunque-plugin-" + name,
		SkillName: name + "_query",
	}

	files := map[string]string{
		"manifest.json": tplManifest,
		"plugin.go":     tplPlugin,
		"skill.go":      tplSkill,
		"skill_test.go": tplSkillTest,
		"go.mod":        tplGoMod,
		"README.md":     tplReadme,
	}

	// Create directory
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fatal("create directory: %v", err)
	}

	for filename, tplStr := range files {
		path := filepath.Join(dir, filename)
		tmpl, err := template.New(filename).Parse(tplStr)
		if err != nil {
			fatal("parse template %s: %v", filename, err)
		}
		f, err := os.Create(path)
		if err != nil {
			fatal("create file %s: %v", path, err)
		}
		if err := tmpl.Execute(f, data); err != nil {
			f.Close()
			fatal("render template %s: %v", filename, err)
		}
		f.Close()
		fmt.Printf("  created %s\n", path)
	}

	// Run go mod tidy to resolve dependencies
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = dir
	tidy.Stdout = os.Stdout
	tidy.Stderr = os.Stderr
	if err := tidy.Run(); err != nil {
		fmt.Printf("  ⚠ go mod tidy failed: %v (run manually)\n", err)
	} else {
		fmt.Println("  ✓ go mod tidy")
	}

	fmt.Printf("\n✓ Plugin %q scaffolded in ./%s\n", name, name)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. cd", name)
	fmt.Println("  2. Edit skill.go to implement your skill logic")
	fmt.Println("  3. Update manifest.json with permissions")
	fmt.Println("  4. yunque-plugin validate .")
	fmt.Println("  5. yunque-plugin test .")
	fmt.Println("  6. yunque-plugin build .")
}

// --- validate ---

func cmdValidate(dir string) {
	fmt.Printf("Validating plugin in %s ...\n", dir)
	errors := 0

	// 1. Check manifest.json
	mPath := filepath.Join(dir, "manifest.json")
	m, err := manifest.LoadFromFile(mPath)
	if err != nil {
		fmt.Printf("  ✗ manifest.json: %v\n", err)
		errors++
	} else {
		fmt.Printf("  ✓ manifest.json: %s v%s (%d skills, %d permissions)\n",
			m.Name, m.Version, len(m.Skills), len(m.Permissions))
	}

	// 2. Check plugin.go exists
	pluginGo := filepath.Join(dir, "plugin.go")
	if _, err := os.Stat(pluginGo); os.IsNotExist(err) {
		fmt.Println("  ✗ plugin.go: not found")
		errors++
	} else {
		// Check it has the required package and function
		data, _ := os.ReadFile(pluginGo)
		content := string(data)
		if !strings.Contains(content, "func New(") && !strings.Contains(content, "func New()") {
			fmt.Println("  ⚠ plugin.go: missing New() constructor")
		} else {
			fmt.Println("  ✓ plugin.go: found")
		}
	}

	// 3. Check at least one skill file
	skillFiles, _ := filepath.Glob(filepath.Join(dir, "skill*.go"))
	testFiles, _ := filepath.Glob(filepath.Join(dir, "*_test.go"))
	nonTestSkills := 0
	for _, f := range skillFiles {
		if !strings.HasSuffix(f, "_test.go") {
			nonTestSkills++
		}
	}
	if nonTestSkills == 0 {
		fmt.Println("  ⚠ no skill*.go files found")
	} else {
		fmt.Printf("  ✓ skill files: %d found\n", nonTestSkills)
	}

	// 4. Check tests exist
	if len(testFiles) == 0 {
		fmt.Println("  ⚠ no test files found")
	} else {
		fmt.Printf("  ✓ test files: %d found\n", len(testFiles))
	}

	// 5. Check go.mod
	goMod := filepath.Join(dir, "go.mod")
	if _, err := os.Stat(goMod); os.IsNotExist(err) {
		fmt.Println("  ✗ go.mod: not found (not a Go module)")
		errors++
	} else {
		fmt.Println("  ✓ go.mod: found")
	}

	// 6. Cross-check manifest skills vs actual code
	if m != nil && nonTestSkills > 0 {
		for _, sd := range m.Skills {
			found := false
			for _, sf := range skillFiles {
				if strings.HasSuffix(sf, "_test.go") {
					continue
				}
				data, _ := os.ReadFile(sf)
				if strings.Contains(string(data), `"`+sd.Name+`"`) {
					found = true
					break
				}
			}
			if !found {
				fmt.Printf("  ⚠ manifest declares skill %q but not found in code\n", sd.Name)
			}
		}
	}

	// 7. Check dangerous skills have permissions
	if m != nil {
		for _, sd := range m.Skills {
			if sd.Dangerous {
				if !m.HasPermission(manifest.PermSandbox) && !m.HasPermission(manifest.PermNetwork) {
					fmt.Printf("  ⚠ skill %q is dangerous but no sandbox/network permission declared\n", sd.Name)
				}
			}
		}
	}

	fmt.Println()
	if errors > 0 {
		fmt.Printf("✗ Validation failed with %d error(s)\n", errors)
		os.Exit(1)
	}
	fmt.Println("✓ Validation passed")
}

// --- build ---

func cmdBuild(dir string) {
	fmt.Printf("Building plugin in %s ...\n", dir)

	// Validate first
	mPath := filepath.Join(dir, "manifest.json")
	m, err := manifest.LoadFromFile(mPath)
	if err != nil {
		fatal("manifest validation failed: %v", err)
	}

	// Run go build
	outName := m.Name
	if os.Getenv("GOOS") == "windows" || (os.Getenv("GOOS") == "" && isWindows()) {
		outName += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", outName, ".")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	outPath := filepath.Join(dir, outName) // full path for signature
	fmt.Printf("  go build -o %s .\n", outName)
	if err := cmd.Run(); err != nil {
		// Plugin might be a library (no main), that's OK
		fmt.Println("  ℹ go build failed (plugin may be a library, not a binary)")
		// Try building as test to verify it compiles
		cmd2 := exec.Command("go", "build", "./...")
		cmd2.Dir = dir
		cmd2.Stdout = os.Stdout
		cmd2.Stderr = os.Stderr
		if err2 := cmd2.Run(); err2 != nil {
			fatal("compilation failed: %v", err2)
		}
		fmt.Println("  ✓ compilation successful (library mode)")
	} else {
		// Compute signature
		sig, err := manifest.ComputeSignature(outPath)
		if err != nil {
			fmt.Printf("  ⚠ could not compute signature: %v\n", err)
		} else {
			m.Signature = sig
			if err := m.SaveToFile(mPath); err != nil {
				fmt.Printf("  ⚠ could not update manifest signature: %v\n", err)
			} else {
				fmt.Printf("  ✓ signature: %s\n", sig[:16]+"...")
			}
		}
		fmt.Printf("  ✓ built: %s\n", outPath)
	}

	fmt.Printf("\n✓ Plugin %q build complete\n", m.Name)
}

// --- test ---

func cmdTest(dir string) {
	fmt.Printf("Testing plugin in %s ...\n\n", dir)

	cmd := exec.Command("go", "test", "-v", "-count=1", "-timeout", "30s", "./...")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("\n✗ Tests failed")
		os.Exit(1)
	}
	fmt.Println("\n✓ All tests passed")
}

// --- install ---

func cmdInstall(source, targetDir string) {
	fmt.Printf("Installing plugin from %s ...\n", source)

	// Ensure target directory exists
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		fatal("create target dir: %v", err)
	}

	// Determine source type
	switch {
	case strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://"):
		installFromURL(source, targetDir)
	case strings.HasSuffix(source, ".zip"):
		installFromZip(source, targetDir)
	default:
		installFromLocal(source, targetDir)
	}
}

func installFromLocal(src, targetDir string) {
	info, err := os.Stat(src)
	if err != nil {
		fatal("source not found: %v", err)
	}
	if !info.IsDir() {
		fatal("source must be a directory (got file)")
	}

	// Verify it has a manifest
	hasManifest := false
	for _, name := range []string{"plugin.json", "plugin.yaml", "manifest.json"} {
		if _, err := os.Stat(filepath.Join(src, name)); err == nil {
			hasManifest = true
			break
		}
	}
	if !hasManifest {
		fatal("no plugin.json, plugin.yaml, or manifest.json found in %s", src)
	}

	pluginName := filepath.Base(src)
	dest := filepath.Join(targetDir, pluginName)

	if _, err := os.Stat(dest); err == nil {
		fatal("plugin %q already installed at %s (remove first)", pluginName, dest)
	}

	// Copy directory
	if err := copyDir(src, dest); err != nil {
		fatal("copy plugin: %v", err)
	}

	fmt.Printf("  ✓ installed %q to %s\n", pluginName, dest)
}

func installFromZip(zipPath, targetDir string) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		fatal("open zip: %v", err)
	}
	defer r.Close()

	// Find plugin name from first directory in zip
	pluginName := ""
	for _, f := range r.File {
		parts := strings.SplitN(f.Name, "/", 2)
		if len(parts) > 0 && parts[0] != "" {
			pluginName = parts[0]
			break
		}
	}
	if pluginName == "" {
		pluginName = strings.TrimSuffix(filepath.Base(zipPath), ".zip")
	}

	dest := filepath.Join(targetDir, pluginName)
	if _, err := os.Stat(dest); err == nil {
		fatal("plugin %q already installed (remove first)", pluginName)
	}

	for _, f := range r.File {
		path := filepath.Join(targetDir, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(path, 0o755)
			continue
		}
		os.MkdirAll(filepath.Dir(path), 0o755)
		outFile, err := os.Create(path)
		if err != nil {
			fatal("create file: %v", err)
		}
		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			fatal("open zip entry: %v", err)
		}
		io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
	}

	fmt.Printf("  ✓ installed %q from zip to %s\n", pluginName, dest)
}

func installFromURL(url, targetDir string) {
	fmt.Printf("  downloading %s ...\n", url)

	resp, err := http.Get(url)
	if err != nil {
		fatal("download: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fatal("download failed: HTTP %d", resp.StatusCode)
	}

	// Save to temp zip file
	tmpFile, err := os.CreateTemp("", "tori-plugin-*.zip")
	if err != nil {
		fatal("create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		fatal("save download: %v", err)
	}
	tmpFile.Close()

	installFromZip(tmpPath, targetDir)
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

// --- list ---

func cmdList(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Printf("No plugins directory at %s\n", dir)
		return
	}

	found := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pluginDir := filepath.Join(dir, e.Name())

		// Try to read manifest
		pType := "function"
		desc := ""
		skillCount := 0

		for _, mName := range []string{"plugin.json", "plugin.yaml", "manifest.json"} {
			mPath := filepath.Join(pluginDir, mName)
			data, err := os.ReadFile(mPath)
			if err != nil {
				continue
			}
			var m map[string]any
			if json.Unmarshal(data, &m) == nil {
				if d, ok := m["description"].(string); ok {
					desc = d
				}
				if t, ok := m["type"].(string); ok {
					pType = t
				}
				if s, ok := m["skills"].([]any); ok {
					skillCount = len(s)
				}
			}
			break
		}

		fmt.Printf("  %-20s [%s] %d skills  %s\n", e.Name(), pType, skillCount, desc)
		found++
	}

	if found == 0 {
		fmt.Println("  (no plugins installed)")
	} else {
		fmt.Printf("\n  %d plugin(s) found in %s\n", found, dir)
	}
}

// --- helpers ---

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}

func isWindows() bool {
	return os.PathSeparator == '\\'
}

// --- templates ---

var tplManifest = `{
  "name": "{{.Name}}",
  "version": "0.1.0",
  "description": "{{.NameTitle}} plugin for Yunque Agent",
  "author": "",
  "license": "MIT",
  "min_agent": "0.1.0",
  "permissions": [
    {
      "name": "network",
      "description": "Access external {{.Name}} API",
      "required": true
    }
  ],
  "skills": [
    {
      "name": "{{.SkillName}}",
      "description": "Query {{.Name}} information",
      "dangerous": false
    }
  ]
}
`

var tplPlugin = `package {{.Name}}

import "yunque-agent/pkg/skills"

// {{.NameTitle}}Plugin provides {{.Name}} domain capabilities.
type {{.NameTitle}}Plugin struct{}

func New() *{{.NameTitle}}Plugin {
	return &{{.NameTitle}}Plugin{}
}

func (p *{{.NameTitle}}Plugin) Name() string        { return "{{.Name}}" }
func (p *{{.NameTitle}}Plugin) Description() string { return "{{.NameTitle}} domain plugin" }

func (p *{{.NameTitle}}Plugin) Skills() []skills.Skill {
	return []skills.Skill{
		New{{.NameTitle}}Skill(),
	}
}

func (p *{{.NameTitle}}Plugin) SystemPrompt() string {
	return ` + "`" + `你具备{{.Name}}领域能力：
- 查询{{.Name}}相关信息` + "`" + `
}
`

var tplSkill = `package {{.Name}}

import (
	"context"
	"encoding/json"
	"fmt"

	"yunque-agent/pkg/skills"
)

// {{.NameTitle}}Skill queries {{.Name}} information.
type {{.NameTitle}}Skill struct{}

func New{{.NameTitle}}Skill() *{{.NameTitle}}Skill {
	return &{{.NameTitle}}Skill{}
}

func (s *{{.NameTitle}}Skill) Name() string        { return "{{.SkillName}}" }
func (s *{{.NameTitle}}Skill) Description() string { return "查询{{.Name}}信息" }

func (s *{{.NameTitle}}Skill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "查询内容",
			},
		},
		"required": []string{"query"},
	}
}

func (s *{{.NameTitle}}Skill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	// TODO: Implement your {{.Name}} logic here
	result := map[string]any{
		"query":  query,
		"status": "not_implemented",
		"hint":   "Replace this with your actual {{.Name}} API call",
	}

	out, _ := json.Marshal(result)
	return string(out), nil
}
`

var tplSkillTest = `package {{.Name}}

import (
	"context"
	"testing"
)

func TestNew{{.NameTitle}}Skill(t *testing.T) {
	s := New{{.NameTitle}}Skill()
	if s.Name() != "{{.SkillName}}" {
		t.Fatalf("expected name %q, got %q", "{{.SkillName}}", s.Name())
	}
	if s.Description() == "" {
		t.Fatal("description should not be empty")
	}
	params := s.Parameters()
	if params == nil {
		t.Fatal("parameters should not be nil")
	}
}

func TestNew{{.NameTitle}}SkillExecute(t *testing.T) {
	s := New{{.NameTitle}}Skill()
	ctx := context.Background()
	result, err := s.Execute(ctx, map[string]any{"query": "test"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Fatal("result should not be empty")
	}
}

func TestNew{{.NameTitle}}SkillNoQuery(t *testing.T) {
	s := New{{.NameTitle}}Skill()
	ctx := context.Background()
	_, err := s.Execute(ctx, map[string]any{}, nil)
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func Test{{.NameTitle}}Plugin(t *testing.T) {
	p := New()
	if p.Name() != "{{.Name}}" {
		t.Fatalf("expected plugin name %q, got %q", "{{.Name}}", p.Name())
	}
	skills := p.Skills()
	if len(skills) == 0 {
		t.Fatal("plugin should provide at least one skill")
	}
	if p.SystemPrompt() == "" {
		t.Fatal("system prompt should not be empty")
	}
}
`

var tplGoMod = `module {{.Module}}

go 1.21

require yunque-agent v0.0.0

replace yunque-agent => ..
`

var tplReadme = `# {{.NameTitle}} Plugin

A Yunque Agent plugin for {{.Name}} domain capabilities.

## Skills

- **{{.SkillName}}**: Query {{.Name}} information

## Quick Start

` + "```" + `bash
# Validate
yunque-plugin validate .

# Test
yunque-plugin test .

# Build
yunque-plugin build .
` + "```" + `

## Integration

Register this plugin in your agent:

` + "```" + `go
import "{{.Module}}"

pluginReg.Register({{.Name}}.New())
` + "```" + `

## Manifest

See [manifest.json](manifest.json) for version, permissions, and skill declarations.

## License

MIT
`

// Ensure manifest is importable even without the init templates using it at runtime.
var _ = json.Marshal
