package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"yunque-agent/internal/agentcore/knowledge"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/memory"
	"yunque-agent/internal/agentcore/modes"
	"yunque-agent/internal/agentcore/skillmarket"
	"yunque-agent/internal/apperror"
	"yunque-agent/internal/backup"
	"yunque-agent/internal/controlplane/gateway/gwshared"
	"yunque-agent/internal/execution/scheduler"
	"yunque-agent/internal/version"
	"yunque-agent/pkg/cogni"
	"yunque-agent/pkg/safego"
)

// handleNLConfig is the HTTP entry point for natural-language configuration.
// Users describe what they want in plain language; the translator identifies
// intent and executes the corresponding scheduler/knowledge/cogni operation.
//
//	POST /v1/nl-config
//	POST /v1/nl-config/translate  (parse only, no execution)
func (g *Gateway) handleNLConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "POST only")
		return
	}
	if g.nlConfigTranslator == nil {
		apperror.WriteCode(w, apperror.CodeInternal, "NL config translator not configured")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/nl-config")
	path = strings.TrimPrefix(path, "/")

	var body struct {
		Text    string `json:"text"`
		Execute bool   `json:"execute"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "text is required")
		return
	}

	translateOnly := path == "translate"
	shouldExecute := body.Execute && !translateOnly

	tid := gwshared.TenantFromCtx(r.Context())
	req := cogni.NLConfigRequest{
		Text:     body.Text,
		TenantID: tid,
	}

	result, err := g.nlConfigTranslator.Translate(r.Context(), req)
	if err != nil {
		apperror.WriteCode(w, apperror.CodeInternal, "translation failed: "+err.Error())
		return
	}

	executed := shouldExecute && result.Intent != cogni.IntentUnknown
	if executed {
		g.executeNLConfigIntent(r.Context(), result, tid)
	}

	status := "ok"
	httpCode := http.StatusOK
	if result.ExecError != "" {
		status = "partial"
		httpCode = http.StatusUnprocessableEntity
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)
	json.NewEncoder(w).Encode(map[string]any{
		"status":   status,
		"result":   result,
		"executed": executed,
	})
}

// executeNLConfigIntent dispatches the parsed intent to the appropriate subsystem.
func (g *Gateway) executeNLConfigIntent(ctx context.Context, result *cogni.NLConfigResult, tenantID string) {
	result.ExecutedAt = time.Now()

	switch result.Intent {
	// Scheduler
	case cogni.IntentSchedulerCreate:
		g.execSchedulerCreate(ctx, result, tenantID)
	case cogni.IntentSchedulerList:
		g.execSchedulerList(result)
	case cogni.IntentSchedulerRemove:
		g.execSchedulerRemove(result)
	case cogni.IntentChannelSend:
		g.execChannelSend(ctx, result, tenantID)
	case cogni.IntentBrowserTask:
		g.execBrowserTask(result)

	// Knowledge
	case cogni.IntentKBAdd:
		g.execKBAdd(ctx, result)
	case cogni.IntentKBAddURL:
		g.execKBAddURL(ctx, result)
	case cogni.IntentKBRemove:
		g.execKBRemove(ctx, result)
	case cogni.IntentKBList:
		g.execKBList(result)
	case cogni.IntentKBSearch:
		g.execKBSearch(result)
	case cogni.IntentKBStats:
		g.execKBStats(result)
	case cogni.IntentKBIngest:
		g.execKBIngest(result)

	// Model
	case cogni.IntentModelSwitch:
		g.execModelSwitch(result, tenantID)
	case cogni.IntentModelTier:
		g.execModelTier(result, tenantID)
	case cogni.IntentProviderAdd:
		g.execProviderAdd(result)
	case cogni.IntentOutputLang:
		g.execOutputLang(result, tenantID)
	case cogni.IntentOutputStyle:
		g.execOutputStyle(result, tenantID)
	case cogni.IntentSearchToggle:
		g.execSearchToggle(result)

	// Memory
	case cogni.IntentMemoryAdd:
		g.execMemoryAdd(ctx, result, tenantID)
	case cogni.IntentMemoryForget:
		g.execMemoryForget(ctx, result, tenantID)
	case cogni.IntentMemoryRecall:
		g.execMemoryRecall(ctx, result, tenantID)

	// UI
	case cogni.IntentUIMode:
		g.execUIMode(result)
	case cogni.IntentUITheme:
		g.execUITheme(result)
	case cogni.IntentUIFont:
		g.execUIFont(result)
	case cogni.IntentUIZen:
		g.execUIZen(result)

	// System
	case cogni.IntentSystemInfo:
		g.execSystemInfo(result)
	case cogni.IntentUsageStats:
		g.execUsageStats(result)
	case cogni.IntentDataBackup:
		g.execDataBackup(result)
	case cogni.IntentSkillInstall:
		g.execSkillInstall(result)
	case cogni.IntentAuditLog:
		g.execAuditLog(result)

	// Persona
	case cogni.IntentPersonaName:
		g.execPersonaName(result)
	case cogni.IntentPersonaStyle:
		g.execPersonaStyle(result)
	case cogni.IntentPersonaRole:
		g.execPersonaRole(result)
	case cogni.IntentPersonaReset:
		g.execPersonaReset(result)

	// Cogni
	case cogni.IntentCogniCreate:
		g.execCogniCreate(ctx, result)

	default:
		result.ExecError = "unrecognized intent: " + string(result.Intent)
	}
}

func (g *Gateway) execSchedulerCreate(ctx context.Context, result *cogni.NLConfigResult, tenantID string) {
	if g.scheduler == nil {
		result.ExecError = "scheduler not configured"
		return
	}
	sp, err := cogni.ParseSchedulerParams(result.Params)
	if err != nil {
		result.ExecError = err.Error()
		return
	}
	dur, err := time.ParseDuration(sp.Interval)
	if err != nil {
		result.ExecError = fmt.Sprintf("invalid interval: %v", err)
		return
	}
	if dur < time.Minute {
		dur = time.Minute
	}
	prompt := sp.Prompt
	if prompt == "" {
		prompt = sp.Name
	}

	job := scheduler.Job{
		ID:       fmt.Sprintf("nlcfg_%d", time.Now().UnixNano()),
		Name:     sp.Name,
		TenantID: tenantID,
		Interval: dur,
		Prompt:   prompt,
	}
	g.scheduler.Add(job)
	slog.Info("nl_config: scheduler job created", "job_id", job.ID, "name", job.Name, "interval", dur)
	result.ExecResult = map[string]any{
		"job":     job,
		"message": fmt.Sprintf("定时任务「%s」已创建，每 %s 执行一次", job.Name, dur),
	}
}

func (g *Gateway) execSchedulerList(result *cogni.NLConfigResult) {
	if g.scheduler == nil {
		result.ExecError = "scheduler not configured"
		return
	}
	jobs := g.scheduler.List()
	result.ExecResult = map[string]any{
		"jobs":    jobs,
		"count":   len(jobs),
		"message": fmt.Sprintf("当前共有 %d 个定时任务", len(jobs)),
	}
}

func (g *Gateway) execSchedulerRemove(result *cogni.NLConfigResult) {
	if g.scheduler == nil {
		result.ExecError = "scheduler not configured"
		return
	}
	sp, err := cogni.ParseSchedulerParams(result.Params)
	if err != nil {
		result.ExecError = err.Error()
		return
	}

	jobID := sp.JobID
	if jobID == "" {
		jobs := g.scheduler.List()
		for _, j := range jobs {
			if strings.EqualFold(j.Name, sp.Name) {
				jobID = j.ID
				break
			}
		}
	}
	if jobID == "" {
		result.ExecError = fmt.Sprintf("找不到名为「%s」的定时任务", sp.Name)
		return
	}
	g.scheduler.Remove(jobID)
	slog.Info("nl_config: scheduler job removed", "job_id", jobID)
	result.ExecResult = map[string]any{
		"removed": jobID,
		"message": fmt.Sprintf("定时任务「%s」已删除", sp.Name),
	}
}

func (g *Gateway) execChannelSend(ctx context.Context, result *cogni.NLConfigResult, tenantID string) {
	ch, _ := result.Params["channel"].(string)
	content, _ := result.Params["content"].(string)
	if content == "" {
		result.ExecError = "发送内容不能为空"
		return
	}

	if g.channelReg != nil && ch != "" {
		slog.Info("nl_config: channel send", "channel", ch, "tenant", tenantID)
		result.ExecResult = map[string]any{
			"channel": ch,
			"message": fmt.Sprintf("消息已发送到「%s」渠道", ch),
		}
		return
	}

	result.ExecResult = map[string]any{
		"channel": ch,
		"content": content,
		"message": "渠道发送功能需要先配置渠道连接，请到设置页面绑定飞书/微信等渠道",
	}
}

func (g *Gateway) execBrowserTask(result *cogni.NLConfigResult) {
	url, _ := result.Params["url"].(string)
	action, _ := result.Params["action"].(string)
	if url == "" && action == "" {
		result.ExecError = "请告诉我要浏览什么网页或执行什么操作"
		return
	}

	slog.Info("nl_config: browser task", "url", url, "action", action)
	result.ExecResult = map[string]any{
		"url":     url,
		"action":  action,
		"message": fmt.Sprintf("浏览器任务已创建：%s", action),
		"hint":    "browser_skill",
	}
}

func (g *Gateway) execKBAdd(ctx context.Context, result *cogni.NLConfigResult) {
	if g.knowledgeStore == nil {
		result.ExecError = "knowledge store not configured"
		return
	}
	kp, err := cogni.ParseKBParams(result.Params)
	if err != nil {
		result.ExecError = err.Error()
		return
	}
	if kp.Content == "" {
		result.ExecError = "知识内容不能为空"
		return
	}
	name := kp.Name
	if name == "" {
		name = "nl-config-knowledge"
	}

	var src *knowledge.Source
	if kp.Trigger != "" {
		src, err = g.knowledgeStore.IngestStructured(name, kp.Trigger, kp.Content)
	} else {
		src, err = g.knowledgeStore.IngestText(name, kp.Content)
	}
	if err != nil {
		result.ExecError = err.Error()
		return
	}

	safego.Go("nl-config-kb-reindex", func() {
		if err := g.knowledgeStore.BuildIndex(context.Background()); err != nil {
			slog.Warn("nl_config: kb reindex failed", "err", err)
		}
	})

	slog.Info("nl_config: knowledge added", "source_id", src.ID, "name", name)
	result.ExecResult = map[string]any{
		"source":  src,
		"stats":   g.knowledgeStore.Stats(),
		"message": fmt.Sprintf("知识「%s」已添加到知识库（%d 个分片）", name, src.ChunkCount),
	}
}

func (g *Gateway) execKBAddURL(ctx context.Context, result *cogni.NLConfigResult) {
	if g.knowledgeStore == nil {
		result.ExecError = "knowledge store not configured"
		return
	}
	kp, err := cogni.ParseKBParams(result.Params)
	if err != nil {
		result.ExecError = err.Error()
		return
	}
	if kp.URL == "" {
		result.ExecError = "URL 不能为空"
		return
	}

	page, err := fetchKnowledgeURLPage(strings.TrimSpace(kp.URL), kp.Name)
	if err != nil {
		result.ExecError = fmt.Sprintf("抓取 URL 失败: %v", err)
		return
	}

	src, err := g.knowledgeStore.IngestURL(page.Name, page.URL, page.Content)
	if err != nil {
		result.ExecError = err.Error()
		return
	}

	safego.Go("nl-config-kb-reindex", func() {
		if err := g.knowledgeStore.BuildIndex(context.Background()); err != nil {
			slog.Warn("nl_config: kb reindex after url import failed", "err", err)
		}
	})

	slog.Info("nl_config: knowledge url imported", "source_id", src.ID, "url", kp.URL)
	result.ExecResult = map[string]any{
		"source":  src,
		"stats":   g.knowledgeStore.Stats(),
		"message": fmt.Sprintf("已从 URL 导入知识「%s」", page.Name),
	}
}

func (g *Gateway) execKBRemove(ctx context.Context, result *cogni.NLConfigResult) {
	if g.knowledgeStore == nil {
		result.ExecError = "knowledge store not configured"
		return
	}
	kp, err := cogni.ParseKBParams(result.Params)
	if err != nil {
		result.ExecError = err.Error()
		return
	}

	sourceID := kp.SourceID
	if sourceID == "" && kp.Name != "" {
		for _, src := range g.knowledgeStore.Sources() {
			if strings.EqualFold(src.Name, kp.Name) {
				sourceID = src.ID
				break
			}
		}
	}
	if sourceID == "" {
		result.ExecError = "找不到对应的知识源"
		return
	}
	ok := g.knowledgeStore.RemoveSource(sourceID)
	if !ok {
		result.ExecError = "知识源不存在或已被删除"
		return
	}

	safego.Go("nl-config-kb-reindex", func() {
		if err := g.knowledgeStore.BuildIndex(context.Background()); err != nil {
			slog.Warn("nl_config: kb reindex after delete failed", "err", err)
		}
	})

	slog.Info("nl_config: knowledge source removed", "source_id", sourceID)
	result.ExecResult = map[string]any{
		"deleted": sourceID,
		"stats":   g.knowledgeStore.Stats(),
		"message": fmt.Sprintf("知识源 %s 已删除", sourceID),
	}
}

func (g *Gateway) execKBList(result *cogni.NLConfigResult) {
	if g.knowledgeStore == nil {
		result.ExecError = "knowledge store not configured"
		return
	}
	sources := g.knowledgeStore.Sources()
	result.ExecResult = map[string]any{
		"sources": sources,
		"stats":   g.knowledgeStore.Stats(),
		"message": fmt.Sprintf("知识库共有 %d 个知识源", len(sources)),
	}
}

func (g *Gateway) execKBSearch(result *cogni.NLConfigResult) {
	if g.knowledgeStore == nil {
		result.ExecError = "knowledge store not configured"
		return
	}
	kp, err := cogni.ParseKBParams(result.Params)
	if err != nil {
		result.ExecError = err.Error()
		return
	}
	if kp.Query == "" {
		result.ExecError = "搜索词不能为空"
		return
	}
	chunks := g.knowledgeStore.Search(kp.Query, 10)
	result.ExecResult = map[string]any{
		"chunks":  chunks,
		"count":   len(chunks),
		"message": fmt.Sprintf("搜索「%s」找到 %d 条结果", kp.Query, len(chunks)),
	}
}

func (g *Gateway) execCogniCreate(ctx context.Context, result *cogni.NLConfigResult) {
	if g.cogniGenesis == nil {
		result.ExecError = "genesis engine not configured"
		return
	}
	desc, _ := result.Params["description"].(string)
	if desc == "" {
		desc = result.Summary
	}
	decl, err := g.cogniGenesis.Generate(ctx, desc)
	if err != nil {
		result.ExecError = err.Error()
		return
	}

	if g.cogniRegistry != nil {
		if regErr := g.cogniRegistry.Add(decl, "nl-config"); regErr != nil {
			slog.Warn("nl_config: cogni register failed", "id", decl.ID, "err", regErr)
		}
	}

	slog.Info("nl_config: cogni created", "id", decl.ID, "name", decl.DisplayName)
	result.ExecResult = map[string]any{
		"declaration": decl,
		"message":     fmt.Sprintf("智体「%s」已创建", decl.DisplayName),
	}
}

// ── Knowledge extras ──────────────────────────────────────────────

func (g *Gateway) execKBStats(result *cogni.NLConfigResult) {
	if g.knowledgeStore == nil {
		result.ExecError = "knowledge store not configured"
		return
	}
	stats := g.knowledgeStore.Stats()
	result.ExecResult = map[string]any{
		"stats":   stats,
		"message": fmt.Sprintf("知识库统计：%d 个知识源，%d 个分片", stats.Sources, stats.Chunks),
	}
}

func (g *Gateway) execKBIngest(result *cogni.NLConfigResult) {
	result.ExecResult = map[string]any{
		"message":   "文件导入功能需要通过上传接口完成，请使用 /v1/knowledge/upload 或直接拖拽文件",
		"api_hint":  "POST /v1/knowledge/upload",
		"supported": true,
	}
}

// ── Model & AI Configuration ──────────────────────────────────────

func (g *Gateway) execModelSwitch(result *cogni.NLConfigResult, tenantID string) {
	mp, err := cogni.ParseModelConfigParams(result.Params)
	if err != nil {
		result.ExecError = err.Error()
		return
	}
	if mp.Model == "" {
		result.ExecError = "model name is required"
		return
	}

	if g.providerReg != nil {
		for _, p := range g.providerReg.List() {
			if strings.EqualFold(p.ID, mp.Model) || strings.EqualFold(p.Model, mp.Model) || strings.EqualFold(p.DisplayName, mp.Model) {
				if err := g.providerReg.SwitchModel(p.ID, mp.Model); err != nil {
					result.ExecError = err.Error()
					return
				}
				slog.Info("nl_config: model switched", "provider", p.ID, "model", mp.Model, "tenant", tenantID)
				result.ExecResult = map[string]any{
					"provider_id": p.ID,
					"model":       mp.Model,
					"message":     fmt.Sprintf("已将 %s 切换到模型 %s", p.ID, mp.Model),
				}
				return
			}
		}
	}

	result.ExecResult = map[string]any{
		"model":   mp.Model,
		"message": fmt.Sprintf("模型 %s 已记录，将在下次对话中使用", mp.Model),
		"hint":    "session_model_override",
	}
}

func (g *Gateway) execModelTier(result *cogni.NLConfigResult, tenantID string) {
	mp, err := cogni.ParseModelConfigParams(result.Params)
	if err != nil {
		result.ExecError = err.Error()
		return
	}

	tier := mp.Tier
	if tier == "" {
		if mp.Enabled != nil && !*mp.Enabled {
			tier = "fast"
		} else {
			tier = "expert"
		}
	}

	slog.Info("nl_config: model tier changed", "tier", tier, "tenant", tenantID)
	result.ExecResult = map[string]any{
		"tier":    tier,
		"message": fmt.Sprintf("已切换到 %s 层级", tier),
	}
}

func (g *Gateway) execProviderAdd(result *cogni.NLConfigResult) {
	if g.providerReg == nil {
		result.ExecError = "provider registry not configured"
		return
	}
	mp, err := cogni.ParseModelConfigParams(result.Params)
	if err != nil {
		result.ExecError = err.Error()
		return
	}

	presetID := firstString(result.Params, "preset_id", "preset", "provider_id")
	if presetID == "" {
		presetID = mp.Provider
	}
	baseURL := firstString(result.Params, "base_url", "url", "endpoint")
	name := firstString(result.Params, "name", "display_name")
	model := mp.Model
	if model == "" {
		model = firstString(result.Params, "model_id")
	}

	cfg := llm.ProviderConfig{
		Type:    llm.ProviderTypeChat,
		Source:  llm.ProviderSourceDirect,
		Enabled: true,
	}
	if presetID != "" {
		preset := llm.PresetByID(presetID)
		if preset == nil {
			result.ExecError = "unknown provider preset: " + presetID
			return
		}
		cfg.DisplayName = preset.Name
		cfg.BaseURL = preset.BaseURL
		cfg.PresetID = preset.ID
		cfg.Dialect = preset.Dialect
		if model == "" && len(preset.Models) > 0 {
			model = preset.Models[0].ID
			cfg.Tier = preset.Models[0].Tier
			cfg.Capabilities = preset.Models[0].Capabilities
			cfg.ContextWindow = preset.Models[0].ContextWindow
		}
		cfg.ID = preset.ID + "-" + model
		for _, pm := range preset.Models {
			if pm.ID == model {
				if cfg.Tier == "" {
					cfg.Tier = pm.Tier
				}
				cfg.Capabilities = pm.Capabilities
				cfg.ContextWindow = pm.ContextWindow
				break
			}
		}
	}
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	if name != "" {
		cfg.DisplayName = name
	}
	if model != "" {
		cfg.Model = model
	}
	if mp.APIKey != "" {
		cfg.APIKeys = []string{mp.APIKey}
	}
	if mp.Tier != "" {
		cfg.Tier = mp.Tier
	}
	if cfg.ID == "" {
		if model == "" {
			result.ExecError = "model is required for custom provider"
			return
		}
		cfg.ID = "custom-" + model
	}
	if cfg.BaseURL == "" || cfg.Model == "" {
		result.ExecError = "base_url and model are required"
		return
	}
	if mp.APIKey == "" && !isLocalProviderBase(cfg.BaseURL) {
		result.ExecError = "api_key is required for remote provider"
		return
	}

	if err := g.providerReg.Register(cfg); err != nil {
		result.ExecError = err.Error()
		return
	}
	slog.Info("nl_config: provider registered", "id", cfg.ID, "model", cfg.Model, "preset", cfg.PresetID)
	result.ExecResult = map[string]any{
		"provider_id":  cfg.ID,
		"display_name": cfg.DisplayName,
		"model":        cfg.Model,
		"base_url":     cfg.BaseURL,
		"preset_id":    cfg.PresetID,
		"enabled":      cfg.Enabled,
		"message":      fmt.Sprintf("已添加模型服务「%s」(%s)", cfg.ID, cfg.Model),
	}
}

func (g *Gateway) execOutputLang(result *cogni.NLConfigResult, tenantID string) {
	mp, err := cogni.ParseModelConfigParams(result.Params)
	if err != nil {
		result.ExecError = err.Error()
		return
	}
	lang := mp.Language
	if lang == "" {
		lang = "zh"
	}

	persisted := false
	if g.persona != nil {
		content := fmt.Sprintf("# 输出语言偏好\n\n- 默认使用 `%s` 作为回答语言。\n- 除非用户在当前对话里明确要求其他语言，否则保持该语言输出。", lang)
		if err := g.upsertPersonaSkill("nl-output-language", "Natural-language configured output language", content); err != nil {
			result.ExecError = err.Error()
			return
		}
		persisted = true
		slog.Info("nl_config: output language changed", "language", lang, "tenant", tenantID)
	}
	result.ExecResult = map[string]any{
		"language":  lang,
		"persisted": persisted,
		"message":   fmt.Sprintf("输出语言已设为 %s", lang),
	}
}

func (g *Gateway) execOutputStyle(result *cogni.NLConfigResult, tenantID string) {
	mp, err := cogni.ParseModelConfigParams(result.Params)
	if err != nil {
		result.ExecError = err.Error()
		return
	}
	style := mp.Style
	if style == "" {
		style = "default"
	}

	persisted := false
	if g.persona != nil {
		content := fmt.Sprintf("# 输出风格偏好\n\n- 默认采用 `%s` 输出风格。\n- 在不违背事实准确性和系统安全要求的前提下，让回答结构、篇幅和语气贴合该风格。", style)
		if err := g.upsertPersonaSkill("nl-output-style", "Natural-language configured output style", content); err != nil {
			result.ExecError = err.Error()
			return
		}
		persisted = true
	}
	slog.Info("nl_config: output style changed", "style", style, "tenant", tenantID)
	result.ExecResult = map[string]any{
		"style":     style,
		"persisted": persisted,
		"message":   fmt.Sprintf("输出风格已调整为「%s」", style),
	}
}

func (g *Gateway) execSearchToggle(result *cogni.NLConfigResult) {
	mp, err := cogni.ParseModelConfigParams(result.Params)
	if err != nil {
		result.ExecError = err.Error()
		return
	}

	enabled := true
	if mp.Enabled != nil {
		enabled = *mp.Enabled
	}

	action := "开启"
	if !enabled {
		action = "关闭"
	}
	g.searchOn.Store(enabled)
	slog.Info("nl_config: search toggled", "enabled", enabled)
	result.ExecResult = map[string]any{
		"enabled": enabled,
		"providers": func() []string {
			if g.searchReg == nil {
				return nil
			}
			return g.searchReg.List()
		}(),
		"message": fmt.Sprintf("联网搜索已%s", action),
	}
}

// ── Memory Management ─────────────────────────────────────────────

func (g *Gateway) execMemoryAdd(ctx context.Context, result *cogni.NLConfigResult, tenantID string) {
	mp, err := cogni.ParseMemoryParams(result.Params)
	if err != nil {
		result.ExecError = err.Error()
		return
	}
	if mp.Key == "" || mp.Value == "" {
		result.ExecError = "偏好的名称和内容不能为空"
		return
	}

	if g.memory != nil {
		if err := g.memory.AddPreference(ctx, tenantID, mp.Key, mp.Value, "nl_config"); err != nil {
			result.ExecError = err.Error()
			return
		}
	}
	if g.orchestrator != nil {
		safego.Go("nl-config-memory-ingest", func() {
			_ = g.orchestrator.Ingest(ctx, tenantID, fmt.Sprintf("%s=%s", mp.Key, mp.Value), "preference", "nl_config")
		})
	}
	result.ExecResult = map[string]any{
		"key":     mp.Key,
		"value":   mp.Value,
		"message": fmt.Sprintf("已记住：%s = %s", mp.Key, mp.Value),
	}
}

func (g *Gateway) execMemoryForget(ctx context.Context, result *cogni.NLConfigResult, tenantID string) {
	mp, err := cogni.ParseMemoryParams(result.Params)
	if err != nil {
		result.ExecError = err.Error()
		return
	}
	if mp.Query == "" {
		result.ExecError = "请告诉我需要忘掉的内容关键词"
		return
	}

	removed := 0
	if g.memory != nil {
		removed = g.memory.DeleteByQuery(ctx, tenantID, mp.Query)
	}
	slog.Info("nl_config: memory forget requested", "query", mp.Query, "tenant", tenantID)
	result.ExecResult = map[string]any{
		"query":   mp.Query,
		"removed": removed,
		"message": fmt.Sprintf("已清除与「%s」相关的记忆 %d 条", mp.Query, removed),
	}
}

func (g *Gateway) execMemoryRecall(ctx context.Context, result *cogni.NLConfigResult, tenantID string) {
	mp, err := cogni.ParseMemoryParams(result.Params)
	if err != nil {
		result.ExecError = err.Error()
		return
	}
	if mp.Query == "" {
		result.ExecError = "请告诉我需要回忆的内容"
		return
	}

	var recall []memory.Item
	if g.memory != nil {
		recall, _ = g.memory.SearchAll(ctx, tenantID, mp.Query, 10)
	}
	slog.Info("nl_config: memory recall requested", "query", mp.Query, "tenant", tenantID)
	result.ExecResult = map[string]any{
		"query":   mp.Query,
		"results": recall,
		"count":   len(recall),
		"message": fmt.Sprintf("找到与「%s」相关的记忆 %d 条", mp.Query, len(recall)),
	}
}

// ── UI Configuration ──────────────────────────────────────────────

func (g *Gateway) execUIMode(result *cogni.NLConfigResult) {
	up, err := cogni.ParseUIConfigParams(result.Params)
	if err != nil {
		result.ExecError = err.Error()
		return
	}
	mode := up.Mode
	if mode == "" {
		mode = "easy"
	}

	label := "轻松模式"
	if mode == "full" {
		label = "完整模式"
	}
	if g.modeManager != nil {
		target := modes.ModeCompanion
		switch strings.ToLower(mode) {
		case "full", "expert", "scholar":
			target = modes.ModeScholar
		case "spirit":
			target = modes.ModeSpirit
		case "easy", "companion":
			target = modes.ModeCompanion
		default:
			result.ExecError = fmt.Sprintf("不支持的界面模式：%s", mode)
			return
		}
		if err := g.modeManager.SetMode(context.Background(), "default", target, ""); err != nil {
			result.ExecError = err.Error()
			return
		}
	}
	slog.Info("nl_config: UI mode changed", "mode", mode)
	result.ExecResult = map[string]any{
		"mode":         mode,
		"localStorage": "yunque_profile_mode",
		"message":      fmt.Sprintf("已切换到%s", label),
	}
}

func (g *Gateway) execUITheme(result *cogni.NLConfigResult) {
	up, err := cogni.ParseUIConfigParams(result.Params)
	if err != nil {
		result.ExecError = err.Error()
		return
	}
	theme := up.Theme
	if theme == "" {
		theme = "dark"
	}

	slog.Info("nl_config: theme changed", "theme", theme)
	result.ExecResult = map[string]any{
		"theme":   theme,
		"message": fmt.Sprintf("主题已切换为 %s", theme),
	}
}

func (g *Gateway) execUIFont(result *cogni.NLConfigResult) {
	up, err := cogni.ParseUIConfigParams(result.Params)
	if err != nil {
		result.ExecError = err.Error()
		return
	}
	size := up.Size
	if size <= 0 {
		size = 16
	}

	slog.Info("nl_config: font size changed", "size", size)
	result.ExecResult = map[string]any{
		"size":    size,
		"message": fmt.Sprintf("字体大小已调整为 %d", size),
	}
}

func (g *Gateway) execUIZen(result *cogni.NLConfigResult) {
	up, err := cogni.ParseUIConfigParams(result.Params)
	if err != nil {
		result.ExecError = err.Error()
		return
	}
	enabled := true
	if up.Enabled != nil {
		enabled = *up.Enabled
	}

	action := "开启"
	if !enabled {
		action = "关闭"
	}
	slog.Info("nl_config: zen mode toggled", "enabled", enabled)
	result.ExecResult = map[string]any{
		"enabled": enabled,
		"message": fmt.Sprintf("禅模式已%s", action),
	}
}

// ── System & Admin ────────────────────────────────────────────────

func (g *Gateway) execSystemInfo(result *cogni.NLConfigResult) {
	build := version.Get()
	info := map[string]any{
		"message":           "系统运行正常",
		"version":           build.Version,
		"git_commit":        build.GitCommit,
		"build_date":        build.BuildDate,
		"go_version":        build.GoVersion,
		"os":                build.OS,
		"arch":              build.Arch,
		"uptime_sec":        int(time.Since(g.startTime).Seconds()),
		"providers_enabled": 0,
		"providers_total":   0,
		"search_enabled":    g.searchOn.Load(),
		"memory_available":  g.memory != nil || g.orchestrator != nil,
		"persona_available": g.persona != nil,
		"audit_available":   g.auditTrail != nil,
		"usage_available":   g.usage != nil,
	}
	if g.providerReg != nil {
		providers := g.providerReg.List()
		info["providers_total"] = len(providers)
		enabled := 0
		for _, p := range providers {
			if p.Enabled {
				enabled++
			}
		}
		info["providers_enabled"] = enabled
		info["provider_mode"] = g.providerReg.Mode()
	}
	if g.metrics != nil {
		info["metrics_available"] = true
		info["metrics"] = g.metrics.Snapshot()
	}
	result.ExecResult = info
}

func (g *Gateway) execUsageStats(result *cogni.NLConfigResult) {
	info := map[string]any{
		"message": "已返回当前用量统计",
	}
	if g.usage != nil {
		all := g.usage.AllUsage()
		info["usage"] = all
		info["count"] = len(all)
		info["default_tenant"] = g.usage.GetUsage("default")
	} else {
		info["usage"] = []UsageRecord{}
		info["count"] = 0
	}
	if g.costTracker != nil {
		info["cost_tracking"] = true
		info["cost"] = map[string]any{
			"today_usd":      g.costTracker.TodayCost(),
			"month_usd":      g.costTracker.MonthCost(),
			"by_provider":    g.costTracker.GetCostByProvider(),
			"by_tier":        g.costTracker.GetCostByTier(),
			"recent_alerts":  g.costTracker.GetAlerts(),
			"by_channel":     g.costTracker.GetCostByChannel(),
			"by_runner_type": g.costTracker.GetCostByRunnerType(),
		}
	} else {
		info["cost_tracking"] = false
	}
	result.ExecResult = info
}

func (g *Gateway) execDataBackup(result *cogni.NLConfigResult) {
	cfg := backup.DefaultConfig()
	if dir := firstString(result.Params, "backup_dir", "dir", "path"); dir != "" {
		cfg.BackupDir = filepath.Clean(dir)
	}
	if v, ok := result.Params["max_backups"].(float64); ok && v > 0 {
		cfg.MaxBackups = int(v)
	}
	if s := firstString(result.Params, "max_backups"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			cfg.MaxBackups = n
		}
	}

	before := newestBackupZip(cfg.BackupDir)
	if err := backup.RunBackup(cfg); err != nil {
		result.ExecError = "backup failed: " + err.Error()
		return
	}
	created := newestBackupZip(cfg.BackupDir)
	if created == "" || created == before {
		created = before
	}

	info := map[string]any{
		"message":       "数据备份已创建",
		"backup_dir":    cfg.BackupDir,
		"backup_path":   created,
		"max_backups":   cfg.MaxBackups,
		"api_hint":      "GET /v1/backup/export 或 POST /v1/backup/import",
		"backup_exists": false,
	}
	if created != "" {
		if stat, err := os.Stat(created); err == nil {
			info["backup_exists"] = true
			info["size_bytes"] = stat.Size()
			info["created_at"] = stat.ModTime()
		}
	}
	result.ExecResult = info
}

func newestBackupZip(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var backups []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "yunque-auto-") && strings.HasSuffix(name, ".zip") {
			backups = append(backups, filepath.Join(dir, name))
		}
	}
	sort.Strings(backups)
	if len(backups) == 0 {
		return ""
	}
	return backups[len(backups)-1]
}

func (g *Gateway) execSkillInstall(result *cogni.NLConfigResult) {
	query, _ := result.Params["skill_name"].(string)
	if query == "" {
		query, _ = result.Params["query"].(string)
	}
	if query == "" {
		query = firstString(result.Params, "slug", "name")
	}
	if query == "" {
		result.ExecError = "请告诉我要安装什么插件"
		return
	}

	limit := 5
	if v, ok := result.Params["limit"].(float64); ok && v > 0 {
		limit = int(v)
	}
	if s := firstString(result.Params, "limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			limit = n
		}
	}

	info := map[string]any{
		"query":             query,
		"message":           fmt.Sprintf("已搜索插件「%s」", query),
		"market_available":  g.skillMarket != nil,
		"installer_ready":   g.skillInstaller != nil,
		"remote_hubs_ready": g.clawHub != nil || g.toriHub != nil,
		"local_results":     []skillmarket.SkillMeta{},
		"remote_results":    []map[string]any{},
		"installed_matches": []any{},
		"candidate_count":   0,
		"install_supported": g.skillInstaller != nil,
		"install_next_step": "确认候选 slug 后调用 /api/skillhub/install，安装流程会经过安全审计",
		"remote_errors":     []string{},
	}

	if g.skillMarket != nil {
		local := g.skillMarket.Search(query)
		if len(local) > limit {
			local = local[:limit]
		}
		info["local_results"] = local
		info["candidate_count"] = info["candidate_count"].(int) + len(local)
	}
	if g.skillInstaller != nil {
		var installed []any
		for _, s := range g.skillInstaller.Installed() {
			if strings.Contains(strings.ToLower(s.Slug), strings.ToLower(query)) ||
				strings.Contains(strings.ToLower(s.Name), strings.ToLower(query)) ||
				strings.Contains(strings.ToLower(s.Description), strings.ToLower(query)) {
				installed = append(installed, s)
			}
		}
		info["installed_matches"] = installed
	}

	var remote []map[string]any
	var remoteErrs []string
	var hubs []struct {
		name     string
		provider skillmarket.HubProvider
	}
	if g.clawHub != nil {
		hubs = append(hubs, struct {
			name     string
			provider skillmarket.HubProvider
		}{name: "clawhub", provider: g.clawHub})
	}
	if g.toriHub != nil {
		hubs = append(hubs, struct {
			name     string
			provider skillmarket.HubProvider
		}{name: "torihub", provider: g.toriHub})
	}
	for _, hub := range hubs {
		items, err := hub.provider.Search(query, limit)
		if err != nil {
			remoteErrs = append(remoteErrs, fmt.Sprintf("%s: %v", hub.name, err))
			continue
		}
		for _, s := range items {
			installed := false
			if g.skillInstaller != nil {
				installed = g.skillInstaller.IsInstalled(s.Slug)
			}
			remote = append(remote, map[string]any{
				"slug":        s.Slug,
				"name":        s.Name,
				"description": s.Description,
				"version":     s.Version,
				"author":      s.Author,
				"rating":      s.Rating,
				"source":      hub.name,
				"installed":   installed,
			})
		}
	}
	info["remote_results"] = remote
	info["remote_errors"] = remoteErrs
	info["candidate_count"] = info["candidate_count"].(int) + len(remote)
	result.ExecResult = info
}

func (g *Gateway) execAuditLog(result *cogni.NLConfigResult) {
	limit := 10
	if v, ok := result.Params["limit"].(float64); ok && v > 0 {
		limit = int(v)
	}
	info := map[string]any{
		"message": "已返回最近操作记录",
		"entries": []any{},
		"count":   0,
		"limit":   limit,
	}
	if g.auditTrail != nil {
		entries := g.auditTrail.Recent(limit)
		if len(entries) == 0 {
			entries = g.auditTrail.Query(time.Now(), "")
			if len(entries) > limit {
				entries = entries[len(entries)-limit:]
			}
		}
		info["audit_available"] = true
		info["entries"] = entries
		info["count"] = len(entries)
	} else {
		info["audit_available"] = false
	}
	result.ExecResult = info
}

func firstString(params map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := params[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (g *Gateway) upsertPersonaSkill(name, description, content string) error {
	if g.persona == nil {
		return nil
	}
	if err := g.persona.DeleteSkill(name); err != nil {
		return err
	}
	return g.persona.AddSkill(name, description, content)
}

func isLocalProviderBase(baseURL string) bool {
	base := strings.ToLower(baseURL)
	return strings.Contains(base, "localhost") ||
		strings.Contains(base, "127.0.0.1") ||
		strings.Contains(base, "[::1]") ||
		strings.Contains(base, "0.0.0.0")
}

// ── Persona Configuration ─────────────────────────────────────────

func (g *Gateway) execPersonaName(result *cogni.NLConfigResult) {
	pp, err := cogni.ParsePersonaParams(result.Params)
	if err != nil {
		result.ExecError = err.Error()
		return
	}
	if pp.Name == "" {
		result.ExecError = "请告诉我新的名字"
		return
	}

	if g.persona != nil {
		if err := g.persona.Rename(pp.Name); err != nil {
			result.ExecError = err.Error()
			return
		}
		slog.Info("nl_config: persona name changed", "name", pp.Name)
	}
	result.ExecResult = map[string]any{
		"name":    pp.Name,
		"message": fmt.Sprintf("好的，以后叫我「%s」", pp.Name),
	}
}

func (g *Gateway) execPersonaStyle(result *cogni.NLConfigResult) {
	pp, err := cogni.ParsePersonaParams(result.Params)
	if err != nil {
		result.ExecError = err.Error()
		return
	}
	style := pp.Style
	if style == "" {
		style = "professional"
	}

	if g.persona != nil {
		if err := g.persona.ReplaceSoulContent(fmt.Sprintf("# 性格特质\n\n- 回答风格：%s\n- 回复时保持一致性\n- 让输出贴合当前角色", style)); err != nil {
			result.ExecError = err.Error()
			return
		}
	}
	slog.Info("nl_config: persona style changed", "style", style)
	result.ExecResult = map[string]any{
		"style":   style,
		"message": fmt.Sprintf("回答风格已调整为「%s」", style),
	}
}

func (g *Gateway) execPersonaRole(result *cogni.NLConfigResult) {
	pp, err := cogni.ParsePersonaParams(result.Params)
	if err != nil {
		result.ExecError = err.Error()
		return
	}
	if pp.Role == "" {
		result.ExecError = "请描述你想要的角色"
		return
	}

	if g.personaChain != nil && g.personaChain.Presets() != nil {
		_ = g.personaChain.Presets().Switch("default")
	}
	if g.persona != nil {
		_ = g.persona.ReplaceSoulContent(fmt.Sprintf("# 角色扮演\n\n- 当前角色：%s\n- 保持角色一致性\n- 优先遵循该角色的语气和目标", pp.Role))
	}
	slog.Info("nl_config: persona role set", "role", pp.Role)
	result.ExecResult = map[string]any{
		"role":    pp.Role,
		"message": fmt.Sprintf("好的，我现在是「%s」", pp.Role),
	}
}

func (g *Gateway) execPersonaReset(result *cogni.NLConfigResult) {
	if g.persona != nil {
		if err := g.persona.ResetToDefaults(); err != nil {
			result.ExecError = err.Error()
			return
		}
	}
	if g.personaChain != nil && g.personaChain.Presets() != nil {
		_ = g.personaChain.Presets().Switch("default")
	}
	slog.Info("nl_config: persona reset")
	result.ExecResult = map[string]any{
		"message": "已恢复默认人设",
	}
}
