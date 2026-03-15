package task

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"yunque-agent/pkg/skills"
)

// ──────────────────────────────────────────────
// DynamicSkill — LLM-backed skill generated at runtime
//
// When the system detects a capability gap (skill_missing), the
// SkillGenerator creates a DynamicSkill: a lightweight wrapper that
// delegates execution to the LLM with a generated instruction prompt.
//
// The DynamicSkill can optionally compose existing skills as "tools"
// to accomplish its task (e.g., a "report_gen" skill might compose
// file_create + web_search + translate).
// ──────────────────────────────────────────────

// DynamicSkillDef defines a generated skill's shape.
type DynamicSkillDef struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Parameters  map[string]any    `json:"parameters"`
	Instruction string            `json:"instruction"` // system prompt for LLM execution
	ComposedOf  []string          `json:"composed_of"` // existing skills this can delegate to
	Source      string            `json:"source"`       // "self_generated"
}

// DynamicSkill implements skills.Skill backed by LLM.
type DynamicSkill struct {
	def      DynamicSkillDef
	llmCall  LLMFunc
	registry *skills.Registry
	env      *skills.Environment
}

// NewDynamicSkill creates an LLM-backed skill from a definition.
func NewDynamicSkill(def DynamicSkillDef, llmCall LLMFunc, registry *skills.Registry, env *skills.Environment) *DynamicSkill {
	return &DynamicSkill{
		def:      def,
		llmCall:  llmCall,
		registry: registry,
		env:      env,
	}
}

func (d *DynamicSkill) Name() string               { return d.def.Name }
func (d *DynamicSkill) Description() string        { return d.def.Description }
func (d *DynamicSkill) Parameters() map[string]any { return d.def.Parameters }

// Execute runs the dynamic skill via LLM.
func (d *DynamicSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	// Build execution prompt
	argsJSON, _ := json.Marshal(args)

	// List available sub-skills if this skill composes others
	var subSkillInfo string
	if len(d.def.ComposedOf) > 0 {
		var parts []string
		for _, name := range d.def.ComposedOf {
			if sk, ok := d.registry.Get(name); ok {
				parts = append(parts, fmt.Sprintf("- %s: %s", sk.Name(), sk.Description()))
			}
		}
		if len(parts) > 0 {
			subSkillInfo = "\n\n可用工具:\n" + strings.Join(parts, "\n")
		}
	}

	systemPrompt := d.def.Instruction + subSkillInfo + "\n\n直接返回执行结果，不要返回JSON或代码块包装。"

	userPrompt := fmt.Sprintf("请执行以下操作:\n参数: %s", string(argsJSON))

	result, err := d.llmCall(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("dynamic skill %s: %w", d.def.Name, err)
	}

	return strings.TrimSpace(result), nil
}

// Def returns the skill definition (for persistence).
func (d *DynamicSkill) Def() DynamicSkillDef { return d.def }

// ──────────────────────────────────────────────
// SkillGenerator — creates DynamicSkills from capability gaps
// ──────────────────────────────────────────────

// SkillGenerator uses LLM to generate skill definitions from gaps.
type SkillGenerator struct {
	llmCall  LLMFunc
	registry *skills.Registry
	env      *skills.Environment
}

// NewSkillGenerator creates a skill generator.
func NewSkillGenerator(llmCall LLMFunc, registry *skills.Registry, env *skills.Environment) *SkillGenerator {
	return &SkillGenerator{
		llmCall:  llmCall,
		registry: registry,
		env:      env,
	}
}

// Generate creates a DynamicSkill from a GapRecord.
// Returns the skill definition and registers it in the registry.
func (g *SkillGenerator) Generate(ctx context.Context, gap *GapRecord) (*DynamicSkill, error) {
	// Build available skills list for composition reference
	var existingSkills []string
	for _, sk := range g.registry.All() {
		existingSkills = append(existingSkills, fmt.Sprintf("- %s: %s", sk.Name(), sk.Description()))
	}

	systemPrompt := `你是一个技能生成器。根据失败的任务步骤信息，生成一个新的技能定义。

当前系统已有的技能：
` + strings.Join(existingSkills, "\n") + `

生成规则：
1. name: 技能的英文标识符（snake_case），与gap中缺失的技能名一致或更合理
2. description: 简明中文描述（一句话）
3. parameters: JSON Schema 格式的参数定义
4. instruction: 详细的执行指令（中文），描述这个技能应该怎么完成任务
5. composed_of: 如果可以通过组合现有技能实现，列出依赖的技能名
6. 如果无法通过组合实现，instruction 应该足够详细让 LLM 直接完成任务

返回JSON对象（仅JSON，无代码块包装）：
{"name":"xxx","description":"xxx","parameters":{"type":"object","properties":{...},"required":[...]},"instruction":"xxx","composed_of":["skill1","skill2"]}`

	userPrompt := fmt.Sprintf(`失败的步骤信息：
- 步骤描述: %s
- 期望的技能: %s
- 错误信息: %s
- 缺口类型: %s
- 修复建议: %s`, gap.StepAction, gap.SkillName, gap.ErrorMsg, gap.GapType, gap.Suggestion)

	resp, err := g.llmCall(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM generation: %w", err)
	}

	resp = strings.TrimSpace(resp)
	resp = trimCodeFences(resp)

	var def DynamicSkillDef
	if err := json.Unmarshal([]byte(resp), &def); err != nil {
		return nil, fmt.Errorf("parse skill definition: %w (raw: %s)", err, resp[:min(len(resp), 200)])
	}

	// Validate
	if def.Name == "" {
		return nil, fmt.Errorf("generated skill has no name")
	}
	if def.Instruction == "" {
		return nil, fmt.Errorf("generated skill has no instruction")
	}

	def.Source = "self_generated"

	// Check for name conflict with existing skills
	if _, exists := g.registry.Get(def.Name); exists {
		def.Name = def.Name + "_gen"
	}

	// Create and register
	skill := NewDynamicSkill(def, g.llmCall, g.registry, g.env)
	g.registry.Register(skill)

	slog.Info("growth: generated skill", "name", def.Name, "composed_of", def.ComposedOf)
	return skill, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
