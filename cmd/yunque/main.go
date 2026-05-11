// Command yunque is the CLI for managing Cognifiles — declarative AI Agent
// specifications that can be pulled, shared, and run like container images.
//
// Usage:
//
//	yunque pull <name|file>   — install a Cognifile from a file or hub
//	yunque run  <name>        — activate an installed Cognifile
//	yunque list               — list installed Cognifiles
//	yunque info <name>        — show details of an installed Cognifile
//	yunque stop <name>        — deactivate a running Cognifile
//	yunque rm   <name>        — uninstall a Cognifile
//	yunque init <name>        — scaffold a new Cognifile template
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"yunque-agent/internal/appdir"
	"yunque-agent/pkg/cogni"
	"yunque-agent/pkg/cognifile"
)

const banner = `
╔══════════════════════════════════════════════════════╗
║  云雀 Cognifile — AI Agent 的 Docker               ║
║  声明式定义 · 一键运行 · 可分享可版本化             ║
╚══════════════════════════════════════════════════════╝
`

const usage = `用法: yunque <command> [arguments]

命令:
  pull <file|name>   安装 Cognifile（从文件或 Hub）
  run  <name>        激活已安装的 Cognifile
  list               列出已安装的 Cognifile
  info <name>        查看 Cognifile 详情
  stop <name>        停止运行中的 Cognifile
  rm   <name>        卸载 Cognifile
  init <name>        创建新的 Cognifile 模板

示例:
  yunque pull ./legal-advisor.cognifile.yaml
  yunque run  legal-advisor
  yunque list
  yunque init my-agent
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(banner)
		fmt.Print(usage)
		os.Exit(0)
	}

	dataDir := appdir.DataDir()
	cfDir := filepath.Join(dataDir, "cognifiles")
	registry := cognifile.NewLocalRegistry(cfDir)

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "pull":
		err = cmdPull(registry, args)
	case "run":
		err = cmdRun(registry, args)
	case "list", "ls":
		err = cmdList(registry)
	case "info":
		err = cmdInfo(registry, args)
	case "stop":
		err = cmdStop(registry, args)
	case "rm", "remove", "uninstall":
		err = cmdRemove(registry, args)
	case "init":
		err = cmdInit(args)
	case "help", "--help", "-h":
		fmt.Print(banner)
		fmt.Print(usage)
	case "version", "--version", "-v":
		fmt.Println("yunque cognifile v0.1.0")
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n\n", cmd)
		fmt.Print(usage)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

func cmdPull(registry *cognifile.LocalRegistry, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("用法: yunque pull <file|name>")
	}
	target := args[0]

	if fileExists(target) {
		return pullFromFile(registry, target)
	}

	// TODO: Hub pull — for now only local file pull is supported.
	// Future: yunque pull hub:legal-advisor
	return fmt.Errorf("暂不支持从 Hub 拉取，请提供本地文件路径: yunque pull ./xxx.cognifile.yaml")
}

func pullFromFile(registry *cognifile.LocalRegistry, path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	cf, err := cognifile.LoadFile(absPath)
	if err != nil {
		return err
	}

	if err := registry.Install(cf, "file:"+absPath); err != nil {
		return err
	}

	fmt.Printf("✓ 已安装 %s", cf.Name)
	if cf.DisplayName != "" {
		fmt.Printf(" (%s)", cf.DisplayName)
	}
	fmt.Printf(" v%s\n", cf.Version)
	fmt.Printf("  运行: yunque run %s\n", cf.Name)
	return nil
}

func cmdRun(registry *cognifile.LocalRegistry, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("用法: yunque run <name>")
	}
	name := args[0]

	entry, ok := registry.Get(name)
	if !ok {
		return fmt.Errorf("%q 未安装，请先运行: yunque pull <file>", name)
	}

	cogniRegistry := cogni.NewRegistry()
	runner := cognifile.NewRunner(cogniRegistry, registry)

	result, err := runner.Run(entry.Cognifile)
	if err != nil {
		return err
	}

	switch result.Status {
	case "activated":
		fmt.Printf("✓ %s 已激活\n", result.Name)
		if entry.Cognifile.Persona.Greeting != "" {
			fmt.Printf("\n  %s\n", entry.Cognifile.Persona.Greeting)
		}
	case "already_active":
		fmt.Printf("• %s 已经在运行中\n", result.Name)
	default:
		fmt.Printf("✗ %s: %s\n", result.Name, result.Error)
	}
	return nil
}

func cmdList(registry *cognifile.LocalRegistry) error {
	entries := registry.List()
	if len(entries) == 0 {
		fmt.Println("暂无已安装的 Cognifile")
		fmt.Println("  安装: yunque pull ./xxx.cognifile.yaml")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "名称\t版本\t角色\t来源\t安装时间")
	fmt.Fprintln(w, "────\t────\t────\t────\t────────")
	for _, e := range entries {
		role := e.Cognifile.Persona.Role
		if len(role) > 30 {
			role = role[:27] + "..."
		}
		source := e.Source
		if strings.HasPrefix(source, "file:") {
			source = "本地文件"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			e.Cognifile.Name,
			e.Cognifile.Version,
			role,
			source,
			e.InstalledAt.Format("2006-01-02 15:04"),
		)
	}
	w.Flush()
	return nil
}

func cmdInfo(registry *cognifile.LocalRegistry, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("用法: yunque info <name>")
	}
	entry, ok := registry.Get(args[0])
	if !ok {
		return fmt.Errorf("%q 未安装", args[0])
	}

	fmt.Println(cognifile.FormatCognifileInfo(entry.Cognifile))
	fmt.Printf("来源: %s\n", entry.Source)
	fmt.Printf("安装时间: %s\n", entry.InstalledAt.Format(time.RFC3339))
	fmt.Printf("文件: %s\n", entry.FilePath)

	if len(entry.Cognifile.Persona.Traits) > 0 {
		fmt.Printf("特质: %s\n", strings.Join(entry.Cognifile.Persona.Traits, ", "))
	}
	if len(entry.Cognifile.Persona.Constraints) > 0 {
		fmt.Printf("约束: %s\n", strings.Join(entry.Cognifile.Persona.Constraints, ", "))
	}
	if entry.Cognifile.Persona.Greeting != "" {
		fmt.Printf("开场白: %s\n", entry.Cognifile.Persona.Greeting)
	}
	return nil
}

func cmdStop(registry *cognifile.LocalRegistry, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("用法: yunque stop <name>")
	}
	name := args[0]
	if _, ok := registry.Get(name); !ok {
		return fmt.Errorf("%q 未安装", name)
	}

	cogniRegistry := cogni.NewRegistry()
	runner := cognifile.NewRunner(cogniRegistry, registry)
	if runner.Stop(name) {
		fmt.Printf("✓ %s 已停止\n", name)
	} else {
		fmt.Printf("• %s 未在运行中\n", name)
	}
	return nil
}

func cmdRemove(registry *cognifile.LocalRegistry, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("用法: yunque rm <name>")
	}
	name := args[0]
	if err := registry.Uninstall(name); err != nil {
		return err
	}
	fmt.Printf("✓ %s 已卸载\n", name)
	return nil
}

func cmdInit(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("用法: yunque init <name>")
	}
	name := args[0]
	filename := name + ".cognifile.yaml"

	if fileExists(filename) {
		return fmt.Errorf("%q 已存在", filename)
	}

	template := &cognifile.Cognifile{
		Schema:      cognifile.SchemaVersion,
		Name:        name,
		DisplayName: name,
		Version:     "0.1.0",
		Description: "我的 AI Agent",
		Author:      "",
		Tags:        []string{"custom"},
		Persona: cognifile.Persona{
			Role:        "智能助手",
			Traits:      []string{"友善", "专业"},
			Constraints: []string{"不编造事实"},
			Greeting:    "你好！有什么我可以帮助你的吗？",
			Language:    "zh-CN",
			Tone:        "friendly",
		},
		Model: cognifile.ModelSpec{
			Tier: "smart",
		},
	}

	if err := cognifile.SaveFile(template, filename); err != nil {
		return err
	}

	fmt.Printf("✓ 已创建 %s\n", filename)
	fmt.Printf("  编辑后安装: yunque pull ./%s\n", filename)
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
