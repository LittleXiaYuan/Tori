package general

import (
	"context"
	"encoding/json"
	"fmt"

	"yunque-agent/internal/execution/sandbox"
	"yunque-agent/pkg/skills"
)

// CodeGenSkill generates and executes code snippets in a sandbox.
// Uses the unified Runner interface, supporting process/docker/k8s backends.
type CodeGenSkill struct {
	runner sandbox.Runner
}

// NewCodeGenSkill creates a CodeGenSkill with the default process runner.
func NewCodeGenSkill() *CodeGenSkill {
	cfg := sandbox.LoadConfig("")
	runner, _ := sandbox.NewRunner(cfg)
	return &CodeGenSkill{runner: runner}
}

// NewCodeGenSkillWithRunner creates a CodeGenSkill with a specific runner.
func NewCodeGenSkillWithRunner(runner sandbox.Runner) *CodeGenSkill {
	return &CodeGenSkill{runner: runner}
}

func (s *CodeGenSkill) Name() string { return "code_execute" }
func (s *CodeGenSkill) Description() string {
	return "生成并执行代码片段（数据分析、计算、文件处理）"
}
func (s *CodeGenSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"language": map[string]any{"type": "string", "description": "编程语言 (python/javascript/go)"},
			"code":     map[string]any{"type": "string", "description": "要执行的代码"},
			"purpose":  map[string]any{"type": "string", "description": "代码目的说明"},
		},
		"required": []string{"language", "code"},
	}
}

func (s *CodeGenSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	lang, _ := args["language"].(string)
	code, _ := args["code"].(string)
	if code == "" {
		return "", fmt.Errorf("code is required")
	}

	result, err := s.runner.Run(ctx, sandbox.RunRequest{
		Language: lang,
		Code:     code,
	})
	if err != nil {
		return "", fmt.Errorf("exec: %w", err)
	}

	out, _ := json.Marshal(result)
	return string(out), nil
}
