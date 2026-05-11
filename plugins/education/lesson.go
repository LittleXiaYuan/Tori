package education

import (
	"context"
	"fmt"
	"strings"

	"yunque-agent/pkg/jsonutil"
	"yunque-agent/pkg/skills"
)

// LessonPlanSkill generates lesson plans based on curriculum standards.
type LessonPlanSkill struct{}

func NewLessonPlanSkill() *LessonPlanSkill { return &LessonPlanSkill{} }

func (s *LessonPlanSkill) Name() string        { return "lesson_plan" }
func (s *LessonPlanSkill) Description() string { return "根据课程标准和教材内容，生成结构化的教学设计方案" }
func (s *LessonPlanSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"subject":    map[string]any{"type": "string", "description": "学科名"},
			"topic":      map[string]any{"type": "string", "description": "课题名"},
			"grade":      map[string]any{"type": "string", "description": "年级"},
			"class_id":   map[string]any{"type": "string", "description": "班级ID，用于获取学情数据进行个性化设计"},
			"objectives": map[string]any{"type": "string", "description": "教学目标，多个目标请用逗号分隔"},
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

	var memCtx string
	if env.MemorySearch != nil && env.TenantID != "" {
		memCtx, _ = env.MemorySearch(ctx, env.TenantID, subject+" "+topic+" 教学设计", 5)
	}

	system := `你是一位经验丰富的教育专家，擅长根据课程标准设计完整的教学方案。请根据用户提供的学科、课题等信息，生成一份结构化的教案。
请严格按以下JSON格式输出：
{
  "title": "教案标题",
  "grade": "年级",
  "subject": "学科名",
  "duration": "建议课时时长(如四十五分钟)",
  "objectives": {
    "knowledge": ["知识与理解目标1", "..."],
    "ability": ["能力与技能目标1", "..."],
    "emotion": ["情感态度价值观目标1", "..."]
  },
  "key_points": ["教学重点知识点1", "..."],
  "difficult_points": ["教学难点知识点1", "..."],
  "teaching_steps": [
    {"phase": "导入环节", "duration": "5分钟左右", "activity": "...", "method": "..."},
    {"phase": "新知讲授", "duration": "20分钟左右", "activity": "...", "method": "..."},
    {"phase": "巩固练习", "duration": "10分钟左右", "activity": "...", "method": "..."},
    {"phase": "总结拓展", "duration": "5分钟左右", "activity": "...", "method": "..."}
  ],
  "homework": "课后作业内容",
  "reflection_points": ["教学反思要点1", "..."]
}
请仅输出纯JSON，不要添加任何额外文字说明。`

	var parts []string
	parts = append(parts, fmt.Sprintf("学科名称：%s\n课题名称：%s", subject, topic))
	if grade != "" {
		parts = append(parts, fmt.Sprintf("年级：%s", grade))
	}
	if objectives != "" {
		parts = append(parts, fmt.Sprintf("教师期望的教学目标：%s", objectives))
	}
	if env.ClassID != "" {
		parts = append(parts, fmt.Sprintf("班级ID：%s", env.ClassID))
	}
	if memCtx != "" {
		parts = append(parts, fmt.Sprintf("\n以下是相关的历史教学记录：\n%s", memCtx))
	}
	user := strings.Join(parts, "\n")

	reply, err := env.LLMCall(ctx, system, user)
	if err != nil {
		return "", fmt.Errorf("lesson plan generation failed: %w", err)
	}
	return jsonutil.Extract(reply), nil
}

// QuizGenSkill generates quiz questions from lesson content.
type QuizGenSkill struct{}

func NewQuizGenSkill() *QuizGenSkill { return &QuizGenSkill{} }

func (s *QuizGenSkill) Name() string        { return "quiz_generate" }
func (s *QuizGenSkill) Description() string { return "根据指定学科和知识点，自动生成不同题型和难度的练习题" }
func (s *QuizGenSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"subject":    map[string]any{"type": "string", "description": "学科名"},
			"topic":      map[string]any{"type": "string", "description": "知识点主题或考查范围"},
			"difficulty": map[string]any{"type": "integer", "description": "难度等级1-5"},
			"count":      map[string]any{"type": "integer", "description": "生成题目的数量"},
			"types":      map[string]any{"type": "string", "description": "题目类型(choice/fill/short_answer/mixed)"},
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

	var memCtx string
	if env.MemorySearch != nil && env.TenantID != "" {
		memCtx, _ = env.MemorySearch(ctx, env.TenantID, subject+" "+topic+" 学生答题 错题分析", 5)
	}

	system := `你是一位专业的教育测评专家，擅长根据课程内容设计高质量的练习题和测验。
请严格按以下JSON格式输出：
{
  "subject": "学科名",
  "topic": "考查知识点",
  "difficulty": 3,
  "questions": [
    {
      "id": 1,
      "type": "choice|fill|short_answer",
      "question": "题目内容描述",
      "options": ["A. ...", "B. ...", "C. ...", "D. ..."],
      "correct_answer": "A",
      "explanation": "解题思路",
      "knowledge_points": ["涉及知识点1"],
      "difficulty": 3
    }
  ]
}
- choice类型必须提供4个选项及正确答案
- fill类型无需options选项字段，correct_answer为标准答案
- short_answer类型无需options选项字段，correct_answer为参考答案要点说明
请仅输出纯JSON，不要添加任何额外文字说明。`

	var parts []string
	parts = append(parts, fmt.Sprintf("学科名称：%s\n考查知识范围：%s\n难度等级：%d/5\n生成题目数量：%d道\n题目类型：%s",
		subject, topic, difficulty, count, qTypes))
	if env.StudentID != "" {
		parts = append(parts, fmt.Sprintf("学生用户ID：%s", env.StudentID))
	}
	if memCtx != "" {
		parts = append(parts, fmt.Sprintf("\n以下是该学生的历史答题与错题记录：\n%s", memCtx))
	}
	user := strings.Join(parts, "\n")

	reply, err := env.LLMCall(ctx, system, user)
	if err != nil {
		return "", fmt.Errorf("quiz generation failed: %w", err)
	}
	return jsonutil.Extract(reply), nil
}

// GradingSkill grades student work using LLM analysis.
type GradingSkill struct{}

func NewGradingSkill() *GradingSkill { return &GradingSkill{} }

func (s *GradingSkill) Name() string { return "grade_work" }
func (s *GradingSkill) Description() string {
	return "智能批改学生作业，给出评分、错误分析和针对性改进建议"
}
func (s *GradingSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"student_id":     map[string]any{"type": "string", "description": "学生用户ID"},
			"student_answer": map[string]any{"type": "string", "description": "学生提交的作答内容"},
			"question":       map[string]any{"type": "string", "description": "题目或作业要求"},
			"answer_key":     map[string]any{"type": "string", "description": "参考标准答案"},
			"rubric":         map[string]any{"type": "string", "description": "评分标准或评分细则"},
			"subject":        map[string]any{"type": "string", "description": "学科名"},
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

	system := `你是一位经验丰富的教师，擅长批改学生作业并提供有建设性的反馈。请根据题目、学生答案和评分标准进行评分。
请严格按以下JSON格式输出：
{
  "score": 85,
  "max_score": 100,
  "is_correct": true,
  "feedback": "整体评价反馈",
  "details": [
    {
      "aspect": "评分维度名称",
      "score": 20,
      "max_score": 25,
      "comment": "该维度的评语"
    }
  ],
  "error_analysis": {
    "error_type": "概念性错误|计算性错误|理解偏差|表述不清|其他",
    "cause": "错误原因的具体分析说明",
    "knowledge_gaps": ["需要加强的知识点"]
  },
  "suggestions": ["改进建议1", "..."]
}
请客观公正地评分，提供有价值的反馈。仅输出JSON格式。`

	var parts []string
	if subject != "" {
		parts = append(parts, fmt.Sprintf("学科名称：%s", subject))
	}
	parts = append(parts, fmt.Sprintf("题目内容：\n%s\n\n学生提交的答案：\n%s", question, studentAnswer))
	if answerKey != "" {
		parts = append(parts, fmt.Sprintf("\n参考标准答案：\n%s", answerKey))
	}
	if rubric != "" {
		parts = append(parts, fmt.Sprintf("\n评分标准与细则：\n%s", rubric))
	}
	user := strings.Join(parts, "\n")

	reply, err := env.LLMCall(ctx, system, user)
	if err != nil {
		return "", fmt.Errorf("grading failed: %w", err)
	}
	return jsonutil.Extract(reply), nil
}
