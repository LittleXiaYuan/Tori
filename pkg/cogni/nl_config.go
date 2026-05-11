package cogni

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// IntentType classifies the user's natural-language configuration intent.
type IntentType string

// --- Category 1: Scheduler / Automation ---
const (
	IntentSchedulerCreate IntentType = "scheduler_create"
	IntentSchedulerList   IntentType = "scheduler_list"
	IntentSchedulerRemove IntentType = "scheduler_remove"
)

// --- Category 1b: Task & Channel ---
const (
	IntentChannelSend IntentType = "channel_send"
	IntentBrowserTask IntentType = "browser_task"
)

// --- Category 2: Knowledge Base ---
const (
	IntentKBAdd    IntentType = "kb_add"
	IntentKBAddURL IntentType = "kb_add_url"
	IntentKBRemove IntentType = "kb_remove"
	IntentKBList   IntentType = "kb_list"
	IntentKBSearch IntentType = "kb_search"
)

// --- Category 3: Model & AI Configuration ---
const (
	IntentModelSwitch   IntentType = "model_switch"
	IntentModelTier     IntentType = "model_tier"
	IntentProviderAdd   IntentType = "provider_add"
	IntentOutputLang    IntentType = "output_lang"
	IntentOutputStyle   IntentType = "output_style"
	IntentSearchToggle  IntentType = "search_toggle"
)

// --- Category 4: Memory & Preference ---
const (
	IntentMemoryAdd    IntentType = "memory_add"
	IntentMemoryForget IntentType = "memory_forget"
	IntentMemoryRecall IntentType = "memory_recall"
	IntentKBStats      IntentType = "kb_stats"
	IntentKBIngest     IntentType = "kb_ingest"
)

// --- Category 5: UI & Experience ---
const (
	IntentUIMode  IntentType = "ui_mode"
	IntentUITheme IntentType = "ui_theme"
	IntentUIFont  IntentType = "ui_font"
	IntentUIZen   IntentType = "ui_zen"
)

// --- Category 6: System & Admin ---
const (
	IntentSystemInfo   IntentType = "system_info"
	IntentUsageStats   IntentType = "usage_stats"
	IntentDataBackup   IntentType = "data_backup"
	IntentSkillInstall IntentType = "skill_install"
	IntentAuditLog     IntentType = "audit_log"
)

// --- Category 7: Persona ---
const (
	IntentPersonaName  IntentType = "persona_name"
	IntentPersonaStyle IntentType = "persona_style"
	IntentPersonaRole  IntentType = "persona_role"
	IntentPersonaReset IntentType = "persona_reset"
)

// --- Category 8: Cogni & Meta ---
const (
	IntentCogniCreate IntentType = "cogni_create"
	IntentUnknown     IntentType = "unknown"
)

// IntentCategory groups related intents for routing and analytics.
type IntentCategory string

const (
	CategoryScheduler IntentCategory = "scheduler"
	CategoryKnowledge IntentCategory = "knowledge"
	CategoryModel     IntentCategory = "model"
	CategoryMemory    IntentCategory = "memory"
	CategoryUI        IntentCategory = "ui"
	CategorySystem    IntentCategory = "system"
	CategoryPersona   IntentCategory = "persona"
	CategoryCogni     IntentCategory = "cogni"
	CategoryUnknown   IntentCategory = "unknown"
)

// IntentMeta describes an intent for documentation and routing.
type IntentMeta struct {
	Type        IntentType     `json:"type"`
	Category    IntentCategory `json:"category"`
	Description string         `json:"description"`
	Examples    []string       `json:"examples"`
	ParamKeys   []string       `json:"param_keys"`
	RequiresLLM bool           `json:"requires_llm"`
}

// IntentRegistry maps all supported intents to their metadata.
var IntentRegistry = map[IntentType]IntentMeta{
	// Scheduler
	IntentSchedulerCreate: {IntentSchedulerCreate, CategoryScheduler, "创建定时任务", []string{"每天早上8点给我发新闻", "帮我监控这个网页"}, []string{"name", "prompt", "interval"}, false},
	IntentSchedulerList:   {IntentSchedulerList, CategoryScheduler, "查看定时任务列表", []string{"看看我有哪些定时任务", "列出所有自动任务"}, nil, false},
	IntentSchedulerRemove: {IntentSchedulerRemove, CategoryScheduler, "删除定时任务", []string{"取消之前设的定时任务", "删掉新闻摘要任务"}, []string{"job_id", "name"}, false},
	IntentChannelSend:    {IntentChannelSend, CategoryScheduler, "发送到渠道", []string{"帮我把这段内容发到飞书群", "发个消息到微信"}, []string{"channel", "content"}, false},
	IntentBrowserTask:    {IntentBrowserTask, CategoryScheduler, "浏览器自动化", []string{"帮我打开百度搜索XXX", "帮我浏览这个网站"}, []string{"url", "action"}, false},
	// Knowledge
	IntentKBAdd:    {IntentKBAdd, CategoryKnowledge, "添加知识", []string{"记住退款政策是7天", "把这段话存到知识库"}, []string{"name", "content", "trigger"}, false},
	IntentKBAddURL: {IntentKBAddURL, CategoryKnowledge, "从URL导入知识", []string{"从这个网页导入知识", "抓取这个URL的内容"}, []string{"url", "name"}, false},
	IntentKBRemove: {IntentKBRemove, CategoryKnowledge, "删除知识源", []string{"删掉那个退款政策", "移除这个知识"}, []string{"source_id", "name"}, false},
	IntentKBList:   {IntentKBList, CategoryKnowledge, "查看知识库", []string{"我的知识库里有什么", "列出所有知识"}, nil, false},
	IntentKBSearch: {IntentKBSearch, CategoryKnowledge, "搜索知识库", []string{"在知识库里搜退款", "找找关于API的知识"}, []string{"query"}, false},
	IntentKBStats:  {IntentKBStats, CategoryKnowledge, "查看知识库统计", []string{"知识库里有多少东西", "知识库容量"}, nil, false},
	IntentKBIngest: {IntentKBIngest, CategoryKnowledge, "导入文件到知识库", []string{"导入这个PDF到知识库", "把文件存进知识库"}, []string{"file_path", "name"}, false},
	// Model
	IntentModelSwitch:  {IntentModelSwitch, CategoryModel, "切换模型", []string{"帮我切换到GPT-4", "用Claude回答"}, []string{"model"}, false},
	IntentModelTier:    {IntentModelTier, CategoryModel, "切换模型层级", []string{"用更快的模型", "这个问题需要深度思考", "关闭深度思考"}, []string{"tier", "enable"}, false},
	IntentProviderAdd:  {IntentProviderAdd, CategoryModel, "添加API密钥", []string{"添加一个新的API密钥", "配置DeepSeek"}, []string{"provider", "api_key"}, false},
	IntentOutputLang:   {IntentOutputLang, CategoryModel, "设置输出语言", []string{"把回答翻译成英文", "用日语回答"}, []string{"language"}, false},
	IntentOutputStyle:  {IntentOutputStyle, CategoryModel, "调整输出风格", []string{"回答简短一点", "详细一点", "用Markdown格式"}, []string{"style"}, false},
	IntentSearchToggle: {IntentSearchToggle, CategoryModel, "开关联网搜索", []string{"开启联网搜索", "关闭搜索功能"}, []string{"enabled"}, false},
	// Memory
	IntentMemoryAdd:    {IntentMemoryAdd, CategoryMemory, "添加用户偏好记忆", []string{"记住我喜欢Markdown", "我的邮箱是xx@xx.com"}, []string{"key", "value"}, false},
	IntentMemoryForget: {IntentMemoryForget, CategoryMemory, "删除记忆", []string{"忘掉我之前说的关于XX的事", "清除我的偏好"}, []string{"query"}, false},
	IntentMemoryRecall: {IntentMemoryRecall, CategoryMemory, "记忆召回", []string{"你还记得上周我问你什么吗", "回忆一下之前的对话"}, []string{"query"}, false},
	// UI
	IntentUIMode:  {IntentUIMode, CategoryUI, "切换界面模式", []string{"界面太复杂了简化一下", "我要看所有功能", "切换到轻松模式"}, []string{"mode"}, false},
	IntentUITheme: {IntentUITheme, CategoryUI, "切换主题", []string{"换个暗色主题", "用亮色模式"}, []string{"theme"}, false},
	IntentUIFont:  {IntentUIFont, CategoryUI, "调整字体", []string{"把字体调大一点", "字号改成16"}, []string{"size"}, false},
	IntentUIZen:   {IntentUIZen, CategoryUI, "禅模式", []string{"开启禅模式我要专注", "进入专注模式"}, []string{"enabled"}, false},
	// System
	IntentSystemInfo:   {IntentSystemInfo, CategorySystem, "系统运行状态", []string{"查看系统运行状态", "系统健康检查"}, nil, false},
	IntentUsageStats:   {IntentUsageStats, CategorySystem, "用量统计", []string{"这个月用了多少额度", "token消耗统计"}, nil, false},
	IntentDataBackup:   {IntentDataBackup, CategorySystem, "数据备份", []string{"帮我备份数据", "导出我的数据"}, nil, false},
	IntentSkillInstall: {IntentSkillInstall, CategorySystem, "安装插件", []string{"安装一个翻译插件", "搜索代码助手插件"}, []string{"skill_name", "query"}, false},
	IntentAuditLog:     {IntentAuditLog, CategorySystem, "操作记录", []string{"给我看看最近的操作记录", "查看审计日志"}, nil, false},
	// Persona
	IntentPersonaName:  {IntentPersonaName, CategoryPersona, "修改Agent名称", []string{"你以后叫小云", "改名叫助手"}, []string{"name"}, false},
	IntentPersonaStyle: {IntentPersonaStyle, CategoryPersona, "调整人设风格", []string{"用更专业的语气", "活泼一点"}, []string{"style"}, false},
	IntentPersonaRole:  {IntentPersonaRole, CategoryPersona, "临时角色切换", []string{"你现在是英语老师", "扮演面试官"}, []string{"role"}, false},
	IntentPersonaReset: {IntentPersonaReset, CategoryPersona, "重置人设", []string{"回到默认性格", "取消角色扮演"}, nil, false},
	// Cogni
	IntentCogniCreate: {IntentCogniCreate, CategoryCogni, "创建智体", []string{"创建一个翻译助手", "新建代码审查智体"}, []string{"description"}, true},
}

// CategoryOf returns the category for an intent type.
func CategoryOf(intent IntentType) IntentCategory {
	if meta, ok := IntentRegistry[intent]; ok {
		return meta.Category
	}
	return CategoryUnknown
}

// NLConfigRequest is the input to the NL translation layer.
type NLConfigRequest struct {
	Text     string `json:"text"`
	TenantID string `json:"tenant_id,omitempty"`
}

// NLConfigResult is the structured output of intent parsing.
type NLConfigResult struct {
	Intent      IntentType     `json:"intent"`
	Category    IntentCategory `json:"category"`
	Confidence  float64        `json:"confidence"`
	Summary     string         `json:"summary"`
	Params      map[string]any `json:"params"`
	RawLLMJSON  string         `json:"raw_llm_json,omitempty"`
	ExecutedAt  time.Time      `json:"executed_at,omitempty"`
	ExecResult  any            `json:"exec_result,omitempty"`
	ExecError   string         `json:"exec_error,omitempty"`

	// Disambiguation support: when confidence is borderline, the translator
	// may populate these fields to drive a confirmation dialog.
	NeedConfirm       bool           `json:"need_confirm,omitempty"`
	ConfirmQuestion   string         `json:"confirm_question,omitempty"`
	AlternativeIntent IntentType     `json:"alternative_intent,omitempty"`
	MissingParams     []string       `json:"missing_params,omitempty"`
}

// ModelConfigParams captures parameters for model/AI configuration intents.
type ModelConfigParams struct {
	Model    string `json:"model,omitempty"`
	Tier     string `json:"tier,omitempty"`
	Provider string `json:"provider,omitempty"`
	APIKey   string `json:"api_key,omitempty"`
	Language string `json:"language,omitempty"`
	Style    string `json:"style,omitempty"`
	Enabled  *bool  `json:"enabled,omitempty"`
}

// MemoryParams captures parameters for memory management intents.
type MemoryParams struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
	Query string `json:"query,omitempty"`
}

// UIConfigParams captures parameters for UI configuration intents.
type UIConfigParams struct {
	Mode    string `json:"mode,omitempty"`
	Theme   string `json:"theme,omitempty"`
	Size    int    `json:"size,omitempty"`
	Enabled *bool  `json:"enabled,omitempty"`
}

// PersonaParams captures parameters for persona configuration intents.
type PersonaParams struct {
	Name  string `json:"name,omitempty"`
	Style string `json:"style,omitempty"`
	Role  string `json:"role,omitempty"`
}

// ParseModelConfigParams extracts model configuration parameters.
func ParseModelConfigParams(params map[string]any) (*ModelConfigParams, error) {
	data, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}
	var p ModelConfigParams
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse model config params: %w", err)
	}
	return &p, nil
}

// ParseMemoryParams extracts memory management parameters.
func ParseMemoryParams(params map[string]any) (*MemoryParams, error) {
	data, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}
	var p MemoryParams
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse memory params: %w", err)
	}
	return &p, nil
}

// ParseUIConfigParams extracts UI configuration parameters.
func ParseUIConfigParams(params map[string]any) (*UIConfigParams, error) {
	data, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}
	var p UIConfigParams
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse ui config params: %w", err)
	}
	return &p, nil
}

// ParsePersonaParams extracts persona configuration parameters.
func ParsePersonaParams(params map[string]any) (*PersonaParams, error) {
	data, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}
	var p PersonaParams
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse persona params: %w", err)
	}
	return &p, nil
}

// SchedulerParams are the extracted parameters for scheduler intents.
type SchedulerParams struct {
	Name     string `json:"name"`
	Prompt   string `json:"prompt"`
	Interval string `json:"interval"`
	JobID    string `json:"job_id,omitempty"`
}

// KBParams are the extracted parameters for knowledge-base intents.
type KBParams struct {
	Name     string `json:"name,omitempty"`
	Content  string `json:"content,omitempty"`
	Trigger  string `json:"trigger,omitempty"`
	URL      string `json:"url,omitempty"`
	SourceID string `json:"source_id,omitempty"`
	Query    string `json:"query,omitempty"`
}

// NLConfigTranslator translates natural language into structured Cogni
// configuration actions. It uses an LLM to parse user intent, then maps
// the result to scheduler/knowledge/cogni operations.
type NLConfigTranslator struct {
	llm LLMFunc
}

// NewNLConfigTranslator creates a translator with the host's LLM function.
func NewNLConfigTranslator(llm LLMFunc) *NLConfigTranslator {
	return &NLConfigTranslator{llm: llm}
}

const nlConfigSystemPrompt = `你是一个自然语言配置解析器。用户用自然语言描述配置操作，你需要识别意图并提取参数。

## 意图类型（8大类35种）

### 自动化 (scheduler)
- **scheduler_create**: 创建定时任务 → name, prompt, interval(Go duration: "1h","24h","168h")
- **scheduler_list**: 查看定时任务 → 无参数
- **scheduler_remove**: 删除定时任务 → job_id 或 name
- **channel_send**: 发送到渠道 → channel(feishu/wechat/email), content
- **browser_task**: 浏览器自动化 → url, action(描述)

### 知识库 (knowledge)
- **kb_add**: 添加知识 → name, content, trigger(可选)
- **kb_add_url**: URL导入 → url, name(可选)
- **kb_remove**: 删除知识 → source_id 或 name
- **kb_list**: 列出知识 → 无参数
- **kb_search**: 搜索知识 → query
- **kb_stats**: 知识库统计 → 无参数
- **kb_ingest**: 导入文件 → file_path, name(可选)

### 模型配置 (model)
- **model_switch**: 切换模型 → model("GPT-4","Claude","DeepSeek"等)
- **model_tier**: 切换层级 → tier("fast"/"smart"/"expert"), enable(bool)
- **provider_add**: 添加密钥 → provider, api_key
- **output_lang**: 输出语言 → language("en","ja","zh"等)
- **output_style**: 输出风格 → style("concise"/"detailed"/"markdown"/"casual")
- **search_toggle**: 开关搜索 → enabled(bool)

### 记忆 (memory)
- **memory_add**: 记住偏好 → key, value
- **memory_forget**: 忘记内容 → query
- **memory_recall**: 回忆内容 → query

### 界面 (ui)
- **ui_mode**: 切换模式 → mode("easy"/"full")
- **ui_theme**: 切换主题 → theme("dark"/"light"/"auto")
- **ui_font**: 字体大小 → size(数字)
- **ui_zen**: 禅模式 → enabled(bool)

### 系统 (system)
- **system_info**: 系统状态 → 无参数
- **usage_stats**: 用量统计 → 无参数
- **data_backup**: 数据备份 → 无参数
- **skill_install**: 安装插件 → skill_name 或 query
- **audit_log**: 操作记录 → 无参数

### 人设 (persona)
- **persona_name**: 改名 → name
- **persona_style**: 风格 → style("professional"/"casual"/"humorous")
- **persona_role**: 角色扮演 → role(描述)
- **persona_reset**: 重置人设 → 无参数

### 智体 (cogni)
- **cogni_create**: 创建智体 → description

## 输出格式 (严格JSON)
{"intent":"意图","confidence":0.0-1.0,"summary":"一句话","params":{...}}

## 歧义处理
当置信度在 0.5-0.7 之间(意图不确定)时，额外输出:
{"intent":"最可能意图","confidence":0.6,"summary":"...","params":{...},"need_confirm":true,"confirm_question":"你是想XX还是YY？","alternative_intent":"次选意图","missing_params":["缺失参数名"]}

## 解析规则
1. interval 用 Go duration: "每小时"→"1h", "每天"→"24h", "每周"→"168h"
2. "定时/每天做X/监控" → scheduler_create
3. "记住/添加知识/导入" → kb_add | kb_add_url | kb_ingest
4. "切换到GPT-4/用更快模型" → model_switch | model_tier
5. "界面简化/轻松模式" → ui_mode(mode:"easy")
6. "记住我喜欢X" → memory_add(key:推断, value:偏好)
7. "你叫X/改名" → persona_name
8. "用专业语气/活泼一点" → persona_style
9. "你是XX老师/扮演" → persona_role
10. "创建XX助手/新建智体" → cogni_create
11. scheduler_create 的 prompt 需转化为可独立执行的完整提示词
12. confidence < 0.5 → intent 设为 unknown`

// Translate parses natural language into a structured NLConfigResult.
func (t *NLConfigTranslator) Translate(ctx context.Context, req NLConfigRequest) (*NLConfigResult, error) {
	if t.llm == nil {
		return nil, fmt.Errorf("nl_config: LLM not configured")
	}
	if strings.TrimSpace(req.Text) == "" {
		return nil, fmt.Errorf("nl_config: empty input")
	}

	raw, err := t.llm(ctx, nlConfigSystemPrompt, req.Text)
	if err != nil {
		return nil, fmt.Errorf("nl_config: LLM call failed: %w", err)
	}

	jsonStr := extractJSON(raw)
	if jsonStr == "" {
		return nil, fmt.Errorf("nl_config: LLM response contains no valid JSON")
	}

	var result NLConfigResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("nl_config: parse LLM output: %w", err)
	}

	result.RawLLMJSON = jsonStr

	if result.Intent == "" || result.Confidence < 0.5 {
		result.Intent = IntentUnknown
	}

	result.Category = CategoryOf(result.Intent)

	if result.Confidence >= 0.5 && result.Confidence < 0.7 && !result.NeedConfirm {
		result.NeedConfirm = true
		if result.ConfirmQuestion == "" {
			result.ConfirmQuestion = fmt.Sprintf("你是想要「%s」吗？", result.Summary)
		}
	}

	if result.Intent != IntentUnknown {
		if meta, ok := IntentRegistry[result.Intent]; ok {
			missing := checkMissingParams(result.Params, meta.ParamKeys)
			if len(missing) > 0 {
				result.MissingParams = missing
				if !result.NeedConfirm {
					result.NeedConfirm = true
					result.ConfirmQuestion = fmt.Sprintf("请补充以下信息：%s", strings.Join(missing, "、"))
				}
			}
		}
	}

	return &result, nil
}

// checkMissingParams identifies required parameters that are empty or absent.
func checkMissingParams(params map[string]any, required []string) []string {
	if len(required) == 0 {
		return nil
	}
	var missing []string
	for _, key := range required {
		v, ok := params[key]
		if !ok {
			missing = append(missing, key)
			continue
		}
		if s, isStr := v.(string); isStr && strings.TrimSpace(s) == "" {
			missing = append(missing, key)
		}
	}
	return missing
}

// ParseSchedulerParams extracts scheduler parameters from the generic params map.
func ParseSchedulerParams(params map[string]any) (*SchedulerParams, error) {
	data, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}
	var sp SchedulerParams
	if err := json.Unmarshal(data, &sp); err != nil {
		return nil, fmt.Errorf("parse scheduler params: %w", err)
	}
	if sp.Name == "" {
		return nil, fmt.Errorf("scheduler: name is required")
	}
	if sp.Interval == "" {
		sp.Interval = "1h"
	}
	if _, err := time.ParseDuration(sp.Interval); err != nil {
		return nil, fmt.Errorf("scheduler: invalid interval %q: %w", sp.Interval, err)
	}
	return &sp, nil
}

// ParseKBParams extracts knowledge-base parameters from the generic params map.
func ParseKBParams(params map[string]any) (*KBParams, error) {
	data, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}
	var kp KBParams
	if err := json.Unmarshal(data, &kp); err != nil {
		return nil, fmt.Errorf("parse kb params: %w", err)
	}
	return &kp, nil
}

// SupportedIntents returns all recognized intent types for documentation.
func SupportedIntents() []IntentType {
	intents := make([]IntentType, 0, len(IntentRegistry))
	for t := range IntentRegistry {
		intents = append(intents, t)
	}
	return intents
}
