package llm

// ProviderPreset is a template for quickly configuring a known LLM provider.
// The user only needs to supply their API key; everything else is pre-filled.
type ProviderPreset struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	LogoURL     string        `json:"logo_url,omitempty"`
	BaseURL     string        `json:"base_url"`
	Models      []PresetModel `json:"models"`
	AuthHeader  string        `json:"auth_header,omitempty"` // default "Authorization: Bearer"
	DocsURL     string        `json:"docs_url,omitempty"`
	Dialect     Dialect       `json:"dialect,omitempty"` // API dialect: "" = OpenAI, "anthropic" = Claude
	// Aggregator providers route to many underlying models; capabilities depend on the model chosen.
	IsAggregator bool `json:"is_aggregator,omitempty"`
}

// ModelPurpose distinguishes chat models from image generation models.
type ModelPurpose string

const (
	PurposeChat  ModelPurpose = "chat"
	PurposeImage ModelPurpose = "image"
)

// PresetModel is a model available from a provider preset.
type PresetModel struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Type          ProviderType `json:"type"`
	Purpose       ModelPurpose `json:"purpose,omitempty"` // "chat" (default) or "image"
	Tier          string       `json:"tier,omitempty"`    // fast / smart / expert
	Capabilities  []Capability `json:"capabilities,omitempty"`
	ContextWindow int          `json:"context_window,omitempty"` // in K tokens, e.g. 128 = 128K
}

// Capability shorthand sets used by presets
var (
	openAIAgent = []Capability{
		CapVision, CapReasoning, CapFunctionCalling, CapStructuredOutput,
		CapLongContext, CapWebSearch, CapFileSearch, CapCodeInterpreter,
		CapComputerUse, CapStreaming, CapMCP,
	}
	claudeAgent = []Capability{
		CapVision, CapReasoning, CapFunctionCalling, CapStructuredOutput,
		CapComputerUse, CapPromptCaching, CapMCP, CapStreaming,
	}
	claudeFast = []Capability{
		CapVision, CapFunctionCalling, CapStructuredOutput,
		CapStreaming, CapPromptCaching,
	}
	geminiAgent = []Capability{
		CapVision, CapReasoning, CapFunctionCalling, CapStructuredOutput,
		CapLongContext, CapWebSearch, CapCodeInterpreter,
		CapAudioIn, CapVideoIn, CapStreaming,
	}
	geminiFast = []Capability{
		CapVision, CapAudioIn, CapVideoIn, CapFunctionCalling,
		CapStructuredOutput, CapLongContext, CapWebSearch, CapStreaming,
	}
	qwenAgent = []Capability{
		CapVision, CapVideoIn, CapFunctionCalling, CapStructuredOutput,
		CapWebSearch, CapCodeInterpreter, CapLongContext, CapStreaming,
	}
	deepseekChat = []Capability{
		CapFunctionCalling, CapStructuredOutput, CapLongContext, CapStreaming,
	}
	deepseekReasoner = []Capability{
		CapReasoning, CapFunctionCalling, CapStructuredOutput, CapLongContext, CapStreaming,
	}
	kimiAgent = []Capability{
		CapVision, CapReasoning, CapFunctionCalling, CapLongContext, CapStreaming,
	}
	glmAgent = []Capability{
		CapReasoning, CapFunctionCalling, CapStructuredOutput,
		CapLongContext, CapPromptCaching, CapMCP, CapStreaming,
	}
	basicFC = []Capability{
		CapFunctionCalling, CapStructuredOutput, CapStreaming,
	}
	visionFC = []Capability{
		CapVision, CapFunctionCalling, CapStructuredOutput, CapStreaming,
	}
	imageGen = []Capability{CapImageGen}
	imageGenEdit = []Capability{CapImageGen, CapImageEdit}
)

// Presets returns all built-in provider presets.
// Only GA/stable models are included. Preview/beta models should NOT be added.
func Presets() []ProviderPreset {
	return []ProviderPreset{
		// ── Tier 1 Global ──

		// ── OpenAI ──
		{ID: "openai", Name: "OpenAI", BaseURL: "https://api.openai.com/v1",
			DocsURL: "https://platform.openai.com/docs", Description: "GPT-5.4 / GPT-4.1 / o4-mini / GPT Image",
			Models: []PresetModel{
				{ID: "gpt-5.4-nano", Name: "GPT-5.4 Nano", Type: ProviderTypeChat, Tier: "fast",
					Capabilities: openAIAgent, ContextWindow: 400},
				{ID: "gpt-5.4-mini", Name: "GPT-5.4 Mini", Type: ProviderTypeChat, Tier: "smart",
					Capabilities: openAIAgent, ContextWindow: 400},
				{ID: "gpt-5.4", Name: "GPT-5.4", Type: ProviderTypeChat, Tier: "expert",
					Capabilities: openAIAgent, ContextWindow: 1000},
				{ID: "gpt-4.1-nano", Name: "GPT-4.1 Nano", Type: ProviderTypeChat, Tier: "fast",
					Capabilities: []Capability{CapVision, CapFunctionCalling, CapStructuredOutput, CapStreaming}, ContextWindow: 128},
				{ID: "gpt-4.1-mini", Name: "GPT-4.1 Mini", Type: ProviderTypeChat, Tier: "fast",
					Capabilities: openAIAgent, ContextWindow: 128},
				{ID: "gpt-4.1", Name: "GPT-4.1", Type: ProviderTypeChat, Tier: "smart",
					Capabilities: openAIAgent, ContextWindow: 1024},
				{ID: "o4-mini", Name: "o4-mini", Type: ProviderTypeChat, Tier: "smart",
					Capabilities: append([]Capability{CapReasoning}, openAIAgent...), ContextWindow: 200},
				{ID: "o3", Name: "o3", Type: ProviderTypeChat, Tier: "expert",
					Capabilities: append([]Capability{CapReasoning}, openAIAgent...), ContextWindow: 200},
				{ID: "gpt-image-1", Name: "GPT Image 1", Type: ProviderTypeChat, Purpose: PurposeImage, Tier: "smart",
					Capabilities: imageGenEdit},
			}},

		// ── Anthropic / Claude ──
		{ID: "anthropic", Name: "Anthropic (Claude)", BaseURL: "https://api.anthropic.com/v1",
			DocsURL: "https://docs.anthropic.com", Description: "Claude Opus 4.6 / Sonnet 4.6 / Haiku 4.5",
			Dialect: DialectAnthropic,
			Models: []PresetModel{
				{ID: "claude-haiku-4-5-20250514", Name: "Claude Haiku 4.5", Type: ProviderTypeChat, Tier: "fast",
					Capabilities: claudeFast, ContextWindow: 200},
				{ID: "claude-sonnet-4-6", Name: "Claude Sonnet 4.6", Type: ProviderTypeChat, Tier: "smart",
					Capabilities: claudeAgent, ContextWindow: 1000},
				{ID: "claude-opus-4-6", Name: "Claude Opus 4.6", Type: ProviderTypeChat, Tier: "expert",
					Capabilities: claudeAgent, ContextWindow: 1000},
			}},

		// ── Google / Gemini ──
		{ID: "google", Name: "Google (Gemini)", BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai",
			DocsURL: "https://ai.google.dev/docs", Description: "Gemini 2.5 (GA) / 3.1 Pro (Preview) 1M 多模态",
			Models: []PresetModel{
				{ID: "gemini-2.5-flash-lite", Name: "Gemini 2.5 Flash-Lite", Type: ProviderTypeChat, Tier: "fast",
					Capabilities: geminiFast, ContextWindow: 1024},
				{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Type: ProviderTypeChat, Tier: "smart",
					Capabilities: geminiAgent, ContextWindow: 1024},
				{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Type: ProviderTypeChat, Tier: "expert",
					Capabilities: geminiAgent, ContextWindow: 1024},
				{ID: "gemini-3.1-pro-preview", Name: "Gemini 3.1 Pro (Preview)", Type: ProviderTypeChat, Tier: "expert",
					Capabilities: geminiAgent, ContextWindow: 1024},
				{ID: "imagen-4.0-generate-001", Name: "Imagen 4.0", Type: ProviderTypeChat, Purpose: PurposeImage, Tier: "smart",
					Capabilities: imageGen},
			}},

		// ── Mistral ──
		{ID: "mistral", Name: "Mistral AI", BaseURL: "https://api.mistral.ai/v1",
			DocsURL: "https://docs.mistral.ai", Description: "Mistral Large 128K 欧洲替代方案",
			Models: []PresetModel{
				{ID: "mistral-large-latest", Name: "Mistral Large", Type: ProviderTypeChat, Tier: "smart",
					Capabilities: basicFC, ContextWindow: 128},
			}},

		// ── Tier 2 China ──

		// ── DeepSeek ──
		{ID: "deepseek", Name: "DeepSeek", BaseURL: "https://api.deepseek.com/v1",
			DocsURL: "https://platform.deepseek.com/docs", Description: "DeepSeek V3 128K / R1 推理",
			Models: []PresetModel{
				{ID: "deepseek-chat", Name: "DeepSeek V3", Type: ProviderTypeChat, Tier: "smart",
					Capabilities: deepseekChat, ContextWindow: 128},
				{ID: "deepseek-reasoner", Name: "DeepSeek R1", Type: ProviderTypeChat, Tier: "expert",
					Capabilities: deepseekReasoner, ContextWindow: 128},
			}},

		// ── Qwen 通义千问 ──
		{ID: "qwen", Name: "通义千问 (Qwen)", BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
			DocsURL: "https://help.aliyun.com/zh/model-studio", Description: "Qwen Max / Plus / Turbo / VL 128K",
			Models: []PresetModel{
				{ID: "qwen-turbo-latest", Name: "Qwen Turbo", Type: ProviderTypeChat, Tier: "fast",
					Capabilities: basicFC, ContextWindow: 128},
				{ID: "qwen-plus-latest", Name: "Qwen Plus", Type: ProviderTypeChat, Tier: "smart",
					Capabilities: basicFC, ContextWindow: 128},
				{ID: "qwen-max-latest", Name: "Qwen Max", Type: ProviderTypeChat, Tier: "expert",
					Capabilities: append([]Capability{CapReasoning}, basicFC...), ContextWindow: 128},
				{ID: "qwen-vl-max-latest", Name: "Qwen VL Max", Type: ProviderTypeChat, Tier: "expert",
					Capabilities: append([]Capability{CapVision, CapVideoIn}, basicFC...), ContextWindow: 128},
				{ID: "wanx2.1-t2i-turbo", Name: "通义万相 Turbo", Type: ProviderTypeChat, Purpose: PurposeImage, Tier: "fast",
					Capabilities: imageGen},
				{ID: "wanx2.1-t2i-plus", Name: "通义万相 Plus", Type: ProviderTypeChat, Purpose: PurposeImage, Tier: "smart",
					Capabilities: imageGen},
			}},

		// ── 智谱 GLM ──
		{ID: "zhipu", Name: "智谱 (GLM)", BaseURL: "https://open.bigmodel.cn/api/paas/v4",
			DocsURL: "https://docs.bigmodel.cn", Description: "GLM-5 200K Agent / GLM-4V 视觉",
			Models: []PresetModel{
				{ID: "glm-4-flash", Name: "GLM-4 Flash", Type: ProviderTypeChat, Tier: "fast",
					Capabilities: basicFC, ContextWindow: 128},
				{ID: "glm-5", Name: "GLM-5", Type: ProviderTypeChat, Tier: "smart",
					Capabilities: glmAgent, ContextWindow: 200},
				{ID: "glm-4v-flash", Name: "GLM-4V Flash", Type: ProviderTypeChat, Tier: "fast",
					Capabilities: visionFC, ContextWindow: 128},
				{ID: "glm-4v-plus", Name: "GLM-4V Plus", Type: ProviderTypeChat, Tier: "smart",
					Capabilities: visionFC, ContextWindow: 128},
				{ID: "cogview-4-plus", Name: "CogView-4 Plus", Type: ProviderTypeChat, Purpose: PurposeImage, Tier: "smart",
					Capabilities: imageGen},
			}},

		// ── MiniMax ──
		{ID: "minimax", Name: "MiniMax", BaseURL: "https://api.minimax.chat/v1",
			DocsURL: "https://platform.minimaxi.com/document", Description: "M2.7 Agent 200K / 编程",
			Models: []PresetModel{
				{ID: "MiniMax-M2.7-highspeed", Name: "MiniMax M2.7 高速", Type: ProviderTypeChat, Tier: "fast",
					Capabilities: basicFC, ContextWindow: 200},
				{ID: "MiniMax-M2.7", Name: "MiniMax M2.7", Type: ProviderTypeChat, Tier: "smart",
					Capabilities: append([]Capability{CapReasoning}, basicFC...), ContextWindow: 200},
			}},

		// ── 月之暗面 Kimi ──
		{ID: "moonshot", Name: "月之暗面 (Kimi)", BaseURL: "https://api.moonshot.cn/v1",
			DocsURL: "https://platform.kimi.com/docs", Description: "Kimi K2.5 256K 多模态 Agent",
			Models: []PresetModel{
				{ID: "kimi-k2.5", Name: "Kimi K2.5", Type: ProviderTypeChat, Tier: "smart",
					Capabilities: kimiAgent, ContextWindow: 256},
				{ID: "moonshot-v1-128k", Name: "Moonshot V1 128K", Type: ProviderTypeChat, Tier: "smart",
					Capabilities: append([]Capability{CapLongContext}, basicFC...), ContextWindow: 128},
			}},

		// ── 字节豆包 ──
		{ID: "doubao", Name: "字节豆包 (Doubao)", BaseURL: "https://ark.cn-beijing.volces.com/api/v3",
			DocsURL: "https://www.volcengine.com/docs/82379", Description: "doubao-1.5-pro/lite/vision 32K",
			Models: []PresetModel{
				{ID: "doubao-1.5-lite-32k", Name: "Doubao 1.5 Lite", Type: ProviderTypeChat, Tier: "fast",
					Capabilities: basicFC, ContextWindow: 32},
				{ID: "doubao-1.5-pro-32k", Name: "Doubao 1.5 Pro", Type: ProviderTypeChat, Tier: "smart",
					Capabilities: basicFC, ContextWindow: 32},
				{ID: "doubao-1.5-vision-pro-32k", Name: "Doubao 1.5 Vision Pro", Type: ProviderTypeChat, Tier: "smart",
					Capabilities: visionFC, ContextWindow: 32},
			}},

		// ── Tier 3 Aggregators ──

		// ── SiliconFlow 硅基流动 (聚合) ──
		{ID: "siliconflow", Name: "硅基流动 (SiliconFlow)", BaseURL: "https://api.siliconflow.cn/v1",
			DocsURL: "https://docs.siliconflow.cn", Description: "国内聚合平台 — 200+ 模型",
			IsAggregator: true,
			Models: []PresetModel{
				{ID: "deepseek-ai/DeepSeek-V3", Name: "DeepSeek V3", Type: ProviderTypeChat, Tier: "smart",
					Capabilities: deepseekChat, ContextWindow: 128},
				{ID: "deepseek-ai/DeepSeek-R1", Name: "DeepSeek R1", Type: ProviderTypeChat, Tier: "expert",
					Capabilities: deepseekReasoner, ContextWindow: 128},
				{ID: "Qwen/Qwen3-VL-235B-A22B-Instruct", Name: "Qwen3 VL 235B", Type: ProviderTypeChat, Tier: "expert",
					Capabilities: []Capability{CapVision, CapFunctionCalling, CapStreaming}, ContextWindow: 128},
				{ID: "THUDM/GLM-4.6V", Name: "GLM-4.6V", Type: ProviderTypeChat, Tier: "smart",
					Capabilities: visionFC, ContextWindow: 128},
				{ID: "Moonshot/Kimi-K2.5", Name: "Kimi K2.5", Type: ProviderTypeChat, Tier: "smart",
					Capabilities: kimiAgent, ContextWindow: 256},
				{ID: "black-forest-labs/FLUX.1-schnell", Name: "FLUX.1 Schnell", Type: ProviderTypeChat, Purpose: PurposeImage, Tier: "fast",
					Capabilities: imageGen},
			}},

		// ── GitCode AI ──
		{ID: "gitcode", Name: "GitCode AI", BaseURL: "https://api-ai.gitcode.com/v1",
			DocsURL: "https://gitcode.com/docs", Description: "GitCode 平台免费模型",
			IsAggregator: true,
			Models: []PresetModel{
				{ID: "zai-org/GLM-5", Name: "GLM-5", Type: ProviderTypeChat, Tier: "smart",
					Capabilities: glmAgent, ContextWindow: 200},
				{ID: "zai-org/DeepSeek-R1", Name: "DeepSeek R1", Type: ProviderTypeChat, Tier: "expert",
					Capabilities: deepseekReasoner, ContextWindow: 128},
				{ID: "zai-org/Qwen3-235B-A22B", Name: "Qwen3 235B", Type: ProviderTypeChat, Tier: "expert",
					Capabilities: basicFC, ContextWindow: 128},
			}},

		// ── OpenRouter (聚合) ──
		{ID: "openrouter", Name: "OpenRouter", BaseURL: "https://openrouter.ai/api/v1",
			DocsURL: "https://openrouter.ai/docs", Description: "国际聚合路由 — 一个 Key 访问所有模型",
			IsAggregator: true},

		// ── Ollama (本地) ──
		{ID: "ollama", Name: "Ollama (本地)", BaseURL: "http://localhost:11434/v1",
			DocsURL: "https://ollama.com", Description: "本地部署的开源模型"},

		// ── Custom ──
		{ID: "custom", Name: "自定义 / Custom", BaseURL: "", DocsURL: "", Description: "填入任意 OpenAI 兼容 API 的 Base URL"},
	}
}

// PresetByID returns a preset by its ID, or nil if not found.
func PresetByID(id string) *ProviderPreset {
	for _, p := range Presets() {
		if p.ID == id {
			return &p
		}
	}
	return nil
}
