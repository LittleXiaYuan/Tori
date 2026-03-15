package education

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"yunque-agent/pkg/skills"
)

// LessonPlanSkill generates lesson plans based on curriculum standards.
type LessonPlanSkill struct{}

func NewLessonPlanSkill() *LessonPlanSkill { return &LessonPlanSkill{} }

func (s *LessonPlanSkill) Name() string        { return "lesson_plan" }
func (s *LessonPlanSkill) Description() string { return "根据课标和学情生成教案" }
func (s *LessonPlanSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"subject":    map[string]any{"type": "string", "description": "学科"},
			"topic":      map[string]any{"type": "string", "description": "课题"},
			"grade":      map[string]any{"type": "string", "description": "年级"},
			"class_id":   map[string]any{"type": "string", "description": "班级ID（自动注入学情）"},
			"objectives": map[string]any{"type": "string", "description": "教学目标（可选）"},
		},
		"required": []string{"subject", "topic"},
	}
}

func (s *LessonPlanSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	subject, _ := args["subject"].(string)
	topic, _ := args["topic"].(string)
	grade, _ := args["grade"].(string)
	objectives, _ := args["objectives"].(string)
	if subject == "" || topic == "" {
		return "", fmt.Errorf("subject and topic required")
	}
	if env.LLMCall == nil {
		return "", fmt.Errorf("LLM not available")
	}

	// Fetch memory context if available
	var memCtx string
	if env.MemorySearch != nil && env.TenantID != "" {
		memCtx, _ = env.MemorySearch(ctx, env.TenantID, subject+" "+topic+" 学情", 5)
	}

	system := `你是一位资深教研专家。根据提供的信息生成结构化教案。
输出JSON格式：
{
  "title": "课题名称",
  "grade": "年级",
  "subject": "学科",
  "duration": "建议课时(分钟)",
  "objectives": {
    "knowledge": ["知识目标1", "..."],
    "ability": ["能力目标1", "..."],
    "emotion": ["情感目标1", "..."]
  },
  "key_points": ["教学重点1", "..."],
  "difficult_points": ["教学难点1", "..."],
  "teaching_steps": [
    {"phase": "导入", "duration": "5分钟", "activity": "...", "method": "..."},
    {"phase": "新授", "duration": "20分钟", "activity": "...", "method": "..."},
    {"phase": "练习", "duration": "10分钟", "activity": "...", "method": "..."},
    {"phase": "总结", "duration": "5分钟", "activity": "...", "method": "..."}
  ],
  "homework": "课后作业",
  "reflection_points": ["课后反思要点1", "..."]
}
严格输出JSON，不要输出其他内容。`

	var parts []string
	parts = append(parts, fmt.Sprintf("学科：%s\n课题：%s", subject, topic))
	if grade != "" {
		parts = append(parts, fmt.Sprintf("年级：%s", grade))
	}
	if objectives != "" {
		parts = append(parts, fmt.Sprintf("教学目标要求：%s", objectives))
	}
	if env.ClassID != "" {
		parts = append(parts, fmt.Sprintf("班级ID：%s", env.ClassID))
	}
	if memCtx != "" {
		parts = append(parts, fmt.Sprintf("\n参考学情数据：\n%s", memCtx))
	}
	user := strings.Join(parts, "\n")

	reply, err := env.LLMCall(ctx, system, user)
	if err != nil {
		return "", fmt.Errorf("lesson plan generation failed: %w", err)
	}
	return extractJSON(reply), nil
}

// QuizGenSkill generates quiz questions from lesson content.
type QuizGenSkill struct{}

func NewQuizGenSkill() *QuizGenSkill { return &QuizGenSkill{} }

func (s *QuizGenSkill) Name() string        { return "quiz_generate" }
func (s *QuizGenSkill) Description() string { return "根据教学内容和学生水平生成试题" }
func (s *QuizGenSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"subject":    map[string]any{"type": "string", "description": "学科"},
			"topic":      map[string]any{"type": "string", "description": "知识点/章节"},
			"difficulty": map[string]any{"type": "integer", "description": "难度1-5"},
			"count":      map[string]any{"type": "integer", "description": "题目数量"},
			"types":      map[string]any{"type": "string", "description": "题型(choice/fill/short_answer/mixed)"},
		},
		"required": []string{"subject", "topic"},
	}
}

func (s *QuizGenSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	subject, _ := args["subject"].(string)
	topic, _ := args["topic"].(string)
	if subject == "" || topic == "" {
		return "", fmt.Errorf("subject and topic required")
	}
	if env.LLMCall == nil {
		return "", fmt.Errorf("LLM not available")
	}

	difficulty := 3
	if d, ok := args["difficulty"].(float64); ok && d >= 1 && d <= 5 {
		difficulty = int(d)
	}
	count := 5
	if c, ok := args["count"].(float64); ok && c >= 1 && c <= 20 {
		count = int(c)
	}
	qTypes := "mixed"
	if t, ok := args["types"].(string); ok && t != "" {
		qTypes = t
	}

	// Fetch student context
	var memCtx string
	if env.MemorySearch != nil && env.TenantID != "" {
		memCtx, _ = env.MemorySearch(ctx, env.TenantID, subject+" "+topic+" 错题 薄弱", 5)
	}

	system := `你是一位出题专家。根据要求生成高质量试题。
输出JSON格式：
{
  "subject": "学科",
  "topic": "知识点",
  "difficulty": 3,
  "questions": [
    {
      "id": 1,
      "type": "choice|fill|short_answer",
      "question": "题目内容",
      "options": ["A. ...", "B. ...", "C. ...", "D. ..."],
      "correct_answer": "A",
      "explanation": "解析",
      "knowledge_points": ["知识点1"],
      "difficulty": 3
    }
  ]
}
- choice题必须有4个选项
- fill题options为空数组，correct_answer为填空答案
- short_answer题options为空数组，correct_answer为参考答案
严格输出JSON，不要输出其他内容。`

	var parts []string
	parts = append(parts, fmt.Sprintf("学科：%s\n知识点：%s\n难度：%d/5\n数量：%d题\n题型：%s",
		subject, topic, difficulty, count, qTypes))
	if env.StudentID != "" {
		parts = append(parts, fmt.Sprintf("学生ID：%s", env.StudentID))
	}
	if memCtx != "" {
		parts = append(parts, fmt.Sprintf("\n学生薄弱点参考：\n%s", memCtx))
	}
	user := strings.Join(parts, "\n")

	reply, err := env.LLMCall(ctx, system, user)
	if err != nil {
		return "", fmt.Errorf("quiz generation failed: %w", err)
	}
	return extractJSON(reply), nil
}

// GradingSkill grades student work using LLM analysis.
type GradingSkill struct{}

func NewGradingSkill() *GradingSkill { return &GradingSkill{} }

func (s *GradingSkill) Name() string { return "grade_work" }
func (s *GradingSkill) Description() string {
	return "批改学生作业（基于文本或题目内容）"
}
func (s *GradingSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"student_id":     map[string]any{"type": "string", "description": "学生ID"},
			"student_answer": map[string]any{"type": "string", "description": "学生答案文本"},
			"question":       map[string]any{"type": "string", "description": "原题内容"},
			"answer_key":     map[string]any{"type": "string", "description": "标准答案"},
			"rubric":         map[string]any{"type": "string", "description": "评分标准"},
			"subject":        map[string]any{"type": "string", "description": "学科"},
		},
		"required": []string{"student_answer", "question"},
	}
}

func (s *GradingSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	studentAnswer, _ := args["student_answer"].(string)
	question, _ := args["question"].(string)
	if studentAnswer == "" || question == "" {
		return "", fmt.Errorf("student_answer and question required")
	}
	if env.LLMCall == nil {
		return "", fmt.Errorf("LLM not available")
	}

	answerKey, _ := args["answer_key"].(string)
	rubric, _ := args["rubric"].(string)
	subject, _ := args["subject"].(string)

	system := `你是一位严谨的教师批改助手。分析学生答案并给出评价。
输出JSON格式：
{
  "score": 85,
  "max_score": 100,
  "is_correct": true,
  "feedback": "整体评价",
  "details": [
    {
      "aspect": "评分维度",
      "score": 20,
      "max_score": 25,
      "comment": "具体评语"
    }
  ],
  "error_analysis": {
    "error_type": "概念错误|计算错误|理解偏差|表述不清|无",
    "cause": "错误原因分析",
    "knowledge_gaps": ["薄弱知识点"]
  },
  "suggestions": ["改进建议1", "..."]
}
评分公平客观。严格输出JSON。`

	var parts []string
	if subject != "" {
		parts = append(parts, fmt.Sprintf("学科：%s", subject))
	}
	parts = append(parts, fmt.Sprintf("题目：\n%s\n\n学生答案：\n%s", question, studentAnswer))
	if answerKey != "" {
		parts = append(parts, fmt.Sprintf("\n标准答案：\n%s", answerKey))
	}
	if rubric != "" {
		parts = append(parts, fmt.Sprintf("\n评分标准：\n%s", rubric))
	}
	user := strings.Join(parts, "\n")

	reply, err := env.LLMCall(ctx, system, user)
	if err != nil {
		return "", fmt.Errorf("grading failed: %w", err)
	}
	return extractJSON(reply), nil
}

// extractJSON attempts to extract a valid JSON object from LLM output.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	// Strip markdown code fences
	if idx := strings.Index(s, "```json"); idx != -1 {
		s = s[idx+7:]
		if end := strings.Index(s, "```"); end != -1 {
			s = s[:end]
		}
	} else if idx := strings.Index(s, "```"); idx != -1 {
		s = s[idx+3:]
		if end := strings.Index(s, "```"); end != -1 {
			s = s[:end]
		}
	}
	s = strings.TrimSpace(s)
	// Validate it's valid JSON
	if json.Valid([]byte(s)) {
		return s
	}
	// Try to find JSON object boundaries
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start != -1 && end > start {
		candidate := s[start : end+1]
		if json.Valid([]byte(candidate)) {
			return candidate
		}
	}
	return s
}
