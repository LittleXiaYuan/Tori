#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";

const root = process.cwd();
const failures = [];

function read(relPath) {
  const abs = path.join(root, relPath);
  if (!fs.existsSync(abs)) {
    failures.push(`missing file: ${relPath}`);
    return "";
  }
  return fs.readFileSync(abs, "utf8");
}

function walk(dir, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const abs = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      walk(abs, out);
    } else if (entry.isFile() && entry.name.endsWith(".go")) {
      out.push(abs);
    }
  }
  return out;
}

function rel(abs) {
  return path.relative(root, abs).replaceAll(path.sep, "/");
}

function stripComments(text) {
  return text
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/(^|\s)\/\/.*$/gm, "$1");
}

const plannerDir = path.join(root, "internal/agentcore/planner");
if (!fs.existsSync(plannerDir)) {
  failures.push("missing planner directory: internal/agentcore/planner");
}

const goFiles = fs.existsSync(plannerDir) ? walk(plannerDir) : [];
const contextAssemblyRel = "internal/agentcore/planner/context_assembly_service.go";
const allowedCogniInternals = new Set([
  contextAssemblyRel,
  "internal/agentcore/planner/cogni_context_service.go",
  "internal/agentcore/planner/cogni_context_service_test.go",
]);

const modelRuntimeRel = "internal/agentcore/planner/model_runtime_service.go";
const modelRuntimeTasksRel = "internal/agentcore/planner/model_runtime_tasks.go";
const runtimeStrategyRel = "internal/agentcore/planner/runtime_strategy_service.go";
const runtimeStrategySelectionRel = "internal/agentcore/planner/runtime_strategy_selection.go";
const runtimeClassificationApplicationRel = "internal/agentcore/planner/runtime_classification_application.go";
const runtimeRequestLifecycleRel = "internal/agentcore/planner/runtime_request_lifecycle.go";
const runtimeRequestPipelineRel = "internal/agentcore/planner/runtime_request_pipeline.go";
const runtimeRequestMessagesRel = "internal/agentcore/planner/runtime_request_messages.go";
const runtimeRequestContractsRel = "internal/agentcore/planner/runtime_request_contracts.go";
const runtimeExtensionContractsRel = "internal/agentcore/planner/runtime_extension_contracts.go";
const plannerRuntimeSettersRel = "internal/agentcore/planner/planner_runtime_setters.go";
const plannerRuntimeFacadesRel = "internal/agentcore/planner/planner_runtime_facades.go";
const plannerRuntimeServicesRel = "internal/agentcore/planner/planner_runtime_services.go";
const toolFreeChatRuntimeRel = "internal/agentcore/planner/tool_free_chat_runtime.go";
const toolExecutionAsyncRel = "internal/agentcore/planner/tool_execution_async.go";
const executionModeDispatchRel = "internal/agentcore/planner/execution_mode_dispatch.go";
const promptBuilderRel = "internal/agentcore/planner/prompt_builder.go";
const promptRuntimeRel = "internal/agentcore/planner/prompt_runtime_service.go";
const contextWindowRuntimeRel = "internal/agentcore/planner/context_window_runtime_service.go";
const executionRuntimeRel = "internal/agentcore/planner/execution_runtime_service.go";
const delegationRuntimeRel = "internal/agentcore/planner/delegation_runtime_service.go";
const allowedDirectModelCallFiles = new Set([
  modelRuntimeRel,
  executionRuntimeRel,
]);

for (const abs of goFiles) {
  const file = rel(abs);
  const text = stripComments(fs.readFileSync(abs, "utf8"));

  if (text.includes("maybeEmitCogniTrace")) {
    failures.push(`${file} references removed Planner wrapper maybeEmitCogniTrace`);
  }

  for (const staleMethod of [
    ".HasCogniTrace(",
    ".CogniTrace(",
    ".HasCogniSkillFilter(",
    ".FilterCogniSkills(",
  ]) {
    if (file !== contextAssemblyRel && text.includes(staleMethod)) {
      failures.push(`${file} references stale ContextAssemblyService Cogni method ${staleMethod}`);
    }
  }

  if (!allowedCogniInternals.has(file) && text.includes(".cogniService")) {
    failures.push(`${file} reaches into ContextAssemblyService.cogniService directly`);
  }

  for (const removedModelWrapper of [
    "func (p *Planner) chatFallback(",
    "func (p *Planner) chatFallbackFull(",
    "func (p *Planner) chatWithToolsFallback(",
  ]) {
    if (text.includes(removedModelWrapper)) {
      failures.push(`${file} defines removed Planner model wrapper ${removedModelWrapper}`);
    }
  }

  for (const staleModelCall of [
    ".chatFallback(",
    ".chatFallbackFull(",
    ".chatWithToolsFallback(",
  ]) {
    if (text.includes(staleModelCall)) {
      failures.push(`${file} calls removed Planner model wrapper ${staleModelCall}`);
    }
  }

  if (!allowedDirectModelCallFiles.has(file)) {
    for (const directModelCall of [
      ".Chat(ctx,",
      ".ChatWithTools(ctx,",
    ]) {
      if (text.includes(directModelCall)) {
        failures.push(`${file} directly calls LLM ${directModelCall}; route request-level chat through ModelRuntimeService`);
      }
    }
  }

  for (const removedHandoffWrapper of [
    "func (p *Planner) executeHandoffForRequest(",
  ]) {
    if (text.includes(removedHandoffWrapper)) {
      failures.push(`${file} defines removed Planner handoff wrapper ${removedHandoffWrapper}; keep request-level handoff execution in DelegationRuntimeService`);
    }
  }


  if (file === promptBuilderRel) {
    if (text.includes('"yunque-agent/internal/agentcore/localbrain"') || text.includes("localbrain.")) {
      failures.push(`${file} imports or references localbrain DTOs directly; use RuntimeStrategyService RuntimeContextItem/RuntimeContextFilterResult`);
    }
  }

  for (const runtimeStrategyCaller of [
    "internal/agentcore/planner/react_integration.go",
    "internal/agentcore/planner/long_horizon.go",
  ]) {
    if (file === runtimeStrategyCaller && (text.includes('"yunque-agent/internal/agentcore/localbrain"') || text.includes("localbrain."))) {
      failures.push(`${file} imports or references localbrain adaptive-thinking DTOs directly; use RuntimeThinkRequest/RuntimeThinkStepSummary via RuntimeStrategyService`);
    }
  }

  if (file === "internal/agentcore/planner/planner.go") {
    for (const localBrainImportToken of [
      '"yunque-agent/internal/agentcore/localbrain"',
      "localbrain.",
      "*localbrain.LocalBrain",
      "*localbrain.AgenticThinking",
    ]) {
      if (text.includes(localBrainImportToken)) {
        failures.push(`${file} imports or exposes concrete LocalBrain runtime ${JSON.stringify(localBrainImportToken)}; use LocalBrainRuntime/AgenticThinkerRuntime setter interfaces`);
      }
    }

    for (const promptRuntimeToken of [
      "[时间:",
      "[任务焦点]",
      "ContentParts",
      "核心目标",
      ".PersonaPrompt(",
      ".DomainPrompt(",
      "GroupSystemPrompt != \"\"",
      ".BuildDynamicContext(",
      "DynamicContextRequest{",
      "[动态上下文]",
      ".PruneToolResults(",
      ".CompressAndTrim(",
      "func (p *Planner) buildEnv(",
      "func (p *Planner) clientForRequest(",
      "func (p *Planner) adaptiveRoute(",
      "func (p *Planner) selectClientWithCaps(",
      "func (p *Planner) buildFallbackChain(",
      "func (p *Planner) thinkingFlagForRequest(",
      "func (p *Planner) reasoningCallbacks(",
      "func (p *Planner) LLMBreaker(",
      "func (p *Planner) LLMPool(",
      "func (p *Planner) LLMClient(",
      "func (p *Planner) LLMClientFor(",
      "func (p *Planner) GraphContext(",
      "func (p *Planner) LocalBrain(",
      "func shouldAutoThink(",
      "LLMCall:",
      "MemorySearch:",
      "planner LLM client not configured",
      ".Translate(\"planner.task_stopped\")",
    ]) {
      if (text.includes(promptRuntimeToken)) {
        failures.push(`${file} contains prompt-runtime conversation preparation token ${JSON.stringify(promptRuntimeToken)}`);
      }
    }

    for (const executionModeToken of [
      ".shouldUseLongHorizon(",
      ".runtimeStrategy.ReActMode(",
      ".promptRuntime.NativeFC(",
      "assessCognitiveLoad(req)",
      "plan.NeedsPlan(",
    ]) {
      if (text.includes(executionModeToken)) {
        failures.push(`${file} owns execution-mode input assembly token ${JSON.stringify(executionModeToken)}; keep it in ${runtimeStrategySelectionRel}`);
      }
    }

    if (text.includes("func (p *Planner) executionMode(")) {
      failures.push(`${file} defines executionMode; keep request-level mode selection in ${runtimeStrategySelectionRel}`);
    }

    for (const localBrainDecisionToken of [
      "localbrain.Decision",
      "localbrain.Intent",
      ".Intent.Category",
      ".Intent.Complexity",
      ".Intent.Confidence",
      ".Intent.NeedTools",
      ".IntentCategory(",
      ".IntentComplexity(",
      ".IntentConfidence(",
      ".IntentNeedTools(",
    ]) {
      if (text.includes(localBrainDecisionToken)) {
        failures.push(`${file} consumes LocalBrain decision internals ${JSON.stringify(localBrainDecisionToken)}; use RuntimeClassificationResult from RuntimeStrategyService`);
      }
    }
  }
}

const allowedGatewayRawModelFiles = new Set([
  "internal/controlplane/gateway/handlers_nl_config_exec_test.go",
]);
const gatewayDir = path.join(root, "internal/controlplane/gateway");
if (fs.existsSync(gatewayDir)) {
  for (const abs of walk(gatewayDir)) {
    const file = rel(abs);
    const text = stripComments(fs.readFileSync(abs, "utf8"));
    if (text.includes(".Chat(ctx,")) {
      failures.push(`${file} directly calls LLM .Chat(ctx,); route small control-plane LLM tasks through Planner/ModelRuntimeService`);
    }
    if (!allowedGatewayRawModelFiles.has(file) && (text.includes("LLMClientFor(") || text.includes("LLMClient()") || text.includes("LLMBreaker(") || text.includes(".Cache()"))) {
      failures.push(`${file} reaches raw Planner LLM client/cache/breaker; use Planner model-runtime facade such as ModelIDForTier, LLMResponseCacheStats, or ModelRuntimeHealth`);
    }
  }
}

const cmdAgentDir = path.join(root, "cmd/agent");
if (fs.existsSync(cmdAgentDir)) {
  for (const abs of walk(cmdAgentDir)) {
    const file = rel(abs);
    const text = stripComments(fs.readFileSync(abs, "utf8"));
    for (const rawPlannerFacade of [
      ".LLMPool(",
      ".LLMClient(",
      ".LLMClientFor(",
      ".LLMBreaker(",
      ".GraphContext(",
    ]) {
      if (text.includes(rawPlannerFacade)) {
        failures.push(`${file} reaches raw Planner runtime facade ${rawPlannerFacade}; use app bootstrap state or Planner runtime append/snapshot APIs`);
      }
    }
  }
}

const runtimeStrategy = read(runtimeStrategyRel);
for (const needle of [
  "type RuntimeContextItem struct",
  "type RuntimeContextFilterResult struct",
  "type LocalBrainRuntime interface",
  "type AgenticThinkerRuntime interface",
  "func (s *RuntimeStrategyService) SetLocalBrain(brain LocalBrainRuntime)",
  "func (s *RuntimeStrategyService) SetAgenticThinking(thinking AgenticThinkerRuntime)",
  "func (s *RuntimeStrategyService) FilterContext(ctx context.Context, query string, items []RuntimeContextItem, maxItems int) (*RuntimeContextFilterResult, error)",
  "func toLocalBrainContextItems(items []RuntimeContextItem) []localbrain.ContextItem",
  "func fromLocalBrainFilterResult(result *localbrain.FilterResult) *RuntimeContextFilterResult",
  "type RuntimeThinkRequest struct",
  "type RuntimeThinkStepSummary struct",
  "type RuntimeThinkResult struct",
  "type RuntimeIntent struct",
  "type RuntimeDecision struct",
  "type RuntimeClassificationResult struct",
  "type PlanExecutionMode string",
  "type PlanExecutionModeRequest struct",
  "type PlanExecutionModeDecision struct",
  "func (s *RuntimeStrategyService) Classify(ctx context.Context, query, tenantID string) (*RuntimeDecision, error)",
  "func (s *RuntimeStrategyService) ClassifyRequest(ctx context.Context, req PlanRequest, query string) (*RuntimeClassificationResult, error)",
  "func (s *RuntimeStrategyService) SelectExecutionMode(req PlanExecutionModeRequest) PlanExecutionModeDecision",
  "func fromLocalBrainDecision(decision *localbrain.Decision) *RuntimeDecision",
  "func (d *RuntimeDecision) IntentCategory() string",
  "func (d *RuntimeDecision) IntentComplexity() string",
  "func (d *RuntimeDecision) IntentConfidence() float64",
  "func (d *RuntimeDecision) IntentNeedTools() bool",
  "func (s *RuntimeStrategyService) Think(ctx context.Context, req RuntimeThinkRequest) (*RuntimeThinkResult, error)",
  "func (s *RuntimeStrategyService) SelectTierFromThinking(ctx context.Context, req RuntimeThinkRequest) (tier string, stop bool, result *RuntimeThinkResult)",
  "func toLocalBrainThinkRequest(req RuntimeThinkRequest) localbrain.ThinkRequest",
  "func toLocalBrainStepSummaries(steps []RuntimeThinkStepSummary) []localbrain.StepSummary",
  "func fromLocalBrainThinkResult(result *localbrain.ThinkResult) *RuntimeThinkResult",
]) {
  if (!runtimeStrategy.includes(needle)) {
    failures.push(`${runtimeStrategyRel} missing required LocalBrain DTO boundary ${JSON.stringify(needle)}`);
  }
}

const runtimeStrategySelection = read(runtimeStrategySelectionRel);
for (const needle of [
  "func (p *Planner) executionMode(req PlanRequest) PlanExecutionModeDecision",
  "assessCognitiveLoad(req)",
  "plan.NeedsPlan(extractGoal(req))",
  "SelectExecutionMode(modeReq)",
]) {
  if (!runtimeStrategySelection.includes(needle)) {
    failures.push(`${runtimeStrategySelectionRel} missing request-level execution-mode selection boundary ${JSON.stringify(needle)}`);
  }
}

const plannerSourceForMode = read("internal/agentcore/planner/planner.go");
for (const forbidden of [
  '"yunque-agent/internal/agentcore/plan"',
  "func (p *Planner) executionMode(",
  "assessCognitiveLoad(req)",
  "plan.NeedsPlan(",
  ".promptRuntime.NativeFC(",
]) {
  if (plannerSourceForMode.includes(forbidden)) {
    failures.push(`internal/agentcore/planner/planner.go leaks execution-mode selection detail ${JSON.stringify(forbidden)}; keep it in ${runtimeStrategySelectionRel}`);
  }
}

const toolFreeChatRuntime = read(toolFreeChatRuntimeRel);
const runtimeRequestPipeline = read(runtimeRequestPipelineRel);
const runtimeRequestMessages = read(runtimeRequestMessagesRel);
for (const needle of [
  "func (p *Planner) runToolFreeChat(ctx context.Context, req PlanRequest, errPrefix string, steps int) (*PlanResult, error)",
  "EmitCogniTraceForRequest(req)",
  "ChatFallbackForRequest(ctx, req, messages, p.runtimeStrategy, p.modelFallbackEvents(req))",
  "extractNextMoves(cleaned)",
]) {
  if (!toolFreeChatRuntime.includes(needle)) {
    failures.push(`${toolFreeChatRuntimeRel} missing tool-free chat runtime boundary ${JSON.stringify(needle)}`);
  }
}

const plannerSourceForToolFree = read("internal/agentcore/planner/planner.go");
for (const forbidden of [
  "EmitCogniTraceForRequest(req)",
  "ChatFallback(",
]) {
  if (plannerSourceForToolFree.includes(forbidden)) {
    failures.push(`internal/agentcore/planner/planner.go leaks tool-free chat execution detail ${JSON.stringify(forbidden)}; keep it in ${toolFreeChatRuntimeRel}`);
  }
}
if (!runtimeRequestPipeline.includes('runToolFreeChat(ctx, req, "planner chat-only", 0)')) {
  failures.push(`${runtimeRequestPipelineRel} should route DisableTools through runToolFreeChat`);
}
if (!runtimeRequestPipeline.includes('runToolFreeChat(ctx, req, "planner tool-free chat", 1)')) {
  failures.push(`${runtimeRequestPipelineRel} should route LocalBrain tool-free path through runToolFreeChat`);
}

for (const needle of [
  "func (p *Planner) BuildMessages(ctx context.Context, req PlanRequest) ([]llm.Message, []string)",
  "BuildStablePrefix(req.DisableDelegation, req.GroupSystemPrompt, p.buildSystemPrompt, p.buildSubagentSystemPrompt)",
  "AppendDynamicContextMessage(ctx, msgs, DynamicContextAssemblyRequest{",
  "PrepareConversationMessages(req.Messages, time.Now())",
  "FitMessagesForRequest(ctx, msgs, p.ensureModelRuntime().ClientForRequest(req))",
]) {
  if (!runtimeRequestMessages.includes(needle)) {
    failures.push(`${runtimeRequestMessagesRel} missing request message assembly boundary ${JSON.stringify(needle)}`);
  }
}

const plannerSourceForMessages = read("internal/agentcore/planner/planner.go");
for (const forbidden of [
  "func (p *Planner) BuildMessages(",
  "BuildStablePrefix(req.DisableDelegation, req.GroupSystemPrompt",
  "AppendDynamicContextMessage(ctx, msgs, DynamicContextAssemblyRequest{",
  "PrepareConversationMessages(req.Messages, time.Now())",
  "FitMessagesForRequest(ctx, msgs, p.ensureModelRuntime().ClientForRequest(req))",
]) {
  if (plannerSourceForMessages.includes(forbidden)) {
    failures.push(`internal/agentcore/planner/planner.go leaks request message assembly detail ${JSON.stringify(forbidden)}; keep it in ${runtimeRequestMessagesRel}`);
  }
}

const runtimeRequestContracts = read(runtimeRequestContractsRel);
for (const needle of [
  "type PlanRequest struct",
  "type StepEventType string",
  "type StepEvent struct",
  "type StepCallback func(event observe.AgentEvent)",
  "type ctxKeyStepCB struct{}",
  "func WithStepCallback(ctx context.Context, cb StepCallback) context.Context",
  "func StepCallbackFromCtx(ctx context.Context) StepCallback",
  "type StepStatus string",
  "type PlanStep struct",
  "type PlanResult struct",
  "func (r *PlanResult) ExecutionSummary() string",
]) {
  if (!runtimeRequestContracts.includes(needle)) {
    failures.push(`${runtimeRequestContractsRel} missing request contract token ${JSON.stringify(needle)}`);
  }
}

const plannerSourceForContracts = read("internal/agentcore/planner/planner.go");
for (const forbidden of [
  '"yunque-agent/internal/agentcore/emotion"',
  '"yunque-agent/internal/observe"',
  "type PlanRequest struct",
  "type StepEventType string",
  "type StepEvent struct",
  "type StepCallback func",
  "type ctxKeyStepCB struct{}",
  "func WithStepCallback(",
  "func StepCallbackFromCtx(",
  "type StepStatus string",
  "type PlanStep struct",
  "type PlanResult struct",
  "func (r *PlanResult) ExecutionSummary(",
]) {
  if (plannerSourceForContracts.includes(forbidden)) {
    failures.push(`internal/agentcore/planner/planner.go leaks request contract detail ${JSON.stringify(forbidden)}; keep it in ${runtimeRequestContractsRel}`);
  }
}

const runtimeExtensionContracts = read(runtimeExtensionContractsRel);
for (const needle of [
  "type SkillMetricsFunc func(skillName string, duration time.Duration, err error)",
  "type SkillIndexEntry struct",
  "type SkillIndexFunc func() []SkillIndexEntry",
  "type CogniContextFunc func(ctx context.Context, message, tenantID, channel string) string",
  "type BeliefContextFunc func(ctx context.Context, message, tenantID, channel string) string",
  "type CogniSkillFilterFunc func(message, tenantID, channel string, in []skills.Skill) []skills.Skill",
  "type CogniTraceFunc func(message, tenantID, channel string) (CogniTraceDetail, bool)",
  "type MemorySearchFunc func(ctx context.Context, tenantID, query string) string",
  "type ReflectFunc func(ctx context.Context, intent, reply string) bool",
  "const DynContextBudgetDefault = 0",
]) {
  if (!runtimeExtensionContracts.includes(needle)) {
    failures.push(`${runtimeExtensionContractsRel} missing extension contract token ${JSON.stringify(needle)}`);
  }
}

const plannerSourceForExtensionContracts = read("internal/agentcore/planner/planner.go");
for (const forbidden of [
  "type SkillMetricsFunc func",
  "type SkillIndexEntry struct",
  "type SkillIndexFunc func",
  "type CogniContextFunc func",
  "type BeliefContextFunc func",
  "type CogniSkillFilterFunc func",
  "type CogniTraceFunc func",
  "type MemorySearchFunc func",
  "type ReflectFunc func",
  "const DynContextBudgetDefault",
]) {
  if (plannerSourceForExtensionContracts.includes(forbidden)) {
    failures.push(`internal/agentcore/planner/planner.go leaks extension contract detail ${JSON.stringify(forbidden)}; keep it in ${runtimeExtensionContractsRel}`);
  }
}

const plannerRuntimeSetters = read(plannerRuntimeSettersRel);
for (const needle of [
  "func (p *Planner) SetMemory(fn MemorySearchFunc)",
  "func (p *Planner) SetMetaCogBridge(b *iledger.MetaCogBridge)",
  "func (p *Planner) SetPersonaPrompt(fn func() string)",
  "func (p *Planner) SetGraphContext(fn func(query string) string)",
  "func (p *Planner) AppendGraphContext(fn func(query string) string)",
  "func (p *Planner) SetDynContextBudget(tokens int)",
  "func (p *Planner) SetSkillScorer(scorer *skills.SkillScorer)",
  "func (p *Planner) SetSkillRecommendationEngine(engine *recommend.Engine)",
  "func (p *Planner) SetLLMPool(pool *llm.Pool)",
  "func (p *Planner) SetHandoffRegistry(hr *subagent.HandoffRegistry)",
  "func (p *Planner) SetLedger(l *ldg.Ledger)",
  "func (p *Planner) SetLocalBrain(lb LocalBrainRuntime)",
  "func (p *Planner) SetAgenticThinking(at AgenticThinkerRuntime)",
  "func (p *Planner) SetProviderRegistry(reg *llm.ProviderRegistry)",
  "func (p *Planner) SetCogniTrace(fn CogniTraceFunc)",
  "func (p *Planner) SetToolTimeout(d time.Duration)",
]) {
  if (!plannerRuntimeSetters.includes(needle)) {
    failures.push(`${plannerRuntimeSettersRel} missing planner runtime setter ${JSON.stringify(needle)}`);
  }
}

const plannerRuntimeFacades = read(plannerRuntimeFacadesRel);
for (const needle of [
  "func (p *Planner) maxPlanSteps() int",
  "func (p *Planner) perToolTimeout() time.Duration",
  "func (p *Planner) dynamicContextBudget() int",
  "func (p *Planner) ModelIDForTier(tier string) string",
  "func (p *Planner) LLMResponseCacheStats() map[string]any",
  "func (p *Planner) ModelRuntimeHealth() ModelRuntimeHealth",
  "func (p *Planner) GenerateConversationTitle(ctx context.Context, userMsg, assistReply string) string",
  "func (p *Planner) ParseMissionIntent(ctx context.Context, description string) (MissionParseResult, error)",
]) {
  if (!plannerRuntimeFacades.includes(needle)) {
    failures.push(`${plannerRuntimeFacadesRel} missing planner runtime facade ${JSON.stringify(needle)}`);
  }
}

const plannerRuntimeServices = read(plannerRuntimeServicesRel);
for (const needle of [
  "func (p *Planner) ensureContextAssembly() *ContextAssemblyService",
  "func (p *Planner) ensureLearningSidecar() *LearningSidecar",
  "func (p *Planner) ensureSkillRuntime() *SkillRuntimeService",
  "func (p *Planner) ensureTrustGate() *SkillTrustGate",
  "func (p *Planner) ensureProactiveCognition() *ProactiveCognitionService",
  "func (p *Planner) ensureDelegationRuntime() *DelegationRuntimeService",
  "func (p *Planner) ensureRuntimeStrategy() *RuntimeStrategyService",
  "func (p *Planner) ensurePromptRuntime() *PromptRuntimeService",
  "func (p *Planner) ensureExecutionRuntime() *ExecutionRuntimeService",
  "func (p *Planner) ensureContextWindowRuntime() *ContextWindowRuntimeService",
  "func (p *Planner) ensureModelRuntime() *ModelRuntimeService",
]) {
  if (!plannerRuntimeServices.includes(needle)) {
    failures.push(`${plannerRuntimeServicesRel} missing planner runtime service factory ${JSON.stringify(needle)}`);
  }
}

const plannerSourceForServiceFactorySplit = read("internal/agentcore/planner/planner.go");
for (const forbidden of [
  "func (p *Planner) ensureContextAssembly(",
  "func (p *Planner) ensureLearningSidecar(",
  "func (p *Planner) ensureSkillRuntime(",
  "func (p *Planner) ensureTrustGate(",
  "func (p *Planner) ensureProactiveCognition(",
  "func (p *Planner) ensureDelegationRuntime(",
  "func (p *Planner) ensureRuntimeStrategy(",
  "func (p *Planner) ensurePromptRuntime(",
  "func (p *Planner) ensureExecutionRuntime(",
  "func (p *Planner) ensureContextWindowRuntime(",
  "func (p *Planner) ensureModelRuntime(",
]) {
  if (plannerSourceForServiceFactorySplit.includes(forbidden)) {
    failures.push(`internal/agentcore/planner/planner.go leaks runtime service factory detail ${JSON.stringify(forbidden)}; keep it in ${plannerRuntimeServicesRel}`);
  }
}

const plannerSourceForSetterFacadeSplit = read("internal/agentcore/planner/planner.go");
for (const forbidden of [
  '"context"',
  '"fmt"',
  '"time"',
  '"yunque-agent/internal/agentcore/context"',
  '"yunque-agent/internal/agentcore/subagent"',
  '"yunque-agent/internal/cognicore/recommend"',
  '"yunque-agent/internal/ledger"',
  "func (p *Planner) SetMemory(",
  "func (p *Planner) SetMetaCogBridge(",
  "func (p *Planner) SetPersonaPrompt(",
  "func (p *Planner) SetGraphContext(",
  "func (p *Planner) AppendGraphContext(",
  "func (p *Planner) SetDynContextBudget(",
  "func (p *Planner) SetToolTimeout(",
  "func (p *Planner) SetCogniTrace(",
  "func (p *Planner) maxPlanSteps(",
  "func (p *Planner) perToolTimeout(",
  "func (p *Planner) dynamicContextBudget(",
  "func (p *Planner) ModelIDForTier(",
  "func (p *Planner) LLMResponseCacheStats(",
  "func (p *Planner) ModelRuntimeHealth(",
  "func (p *Planner) GenerateConversationTitle(",
  "func (p *Planner) ParseMissionIntent(",
]) {
  if (plannerSourceForSetterFacadeSplit.includes(forbidden)) {
    failures.push(`internal/agentcore/planner/planner.go leaks runtime setter/facade detail ${JSON.stringify(forbidden)}; keep it in ${plannerRuntimeSettersRel} or ${plannerRuntimeFacadesRel}`);
  }
}

const runtimeClassificationApplication = read(runtimeClassificationApplicationRel);
for (const needle of [
  "type AppliedRuntimeClassification struct",
  "func (p *Planner) applyRuntimeClassification(ctx context.Context, req PlanRequest) AppliedRuntimeClassification",
  "ClassifyRequest(ctx, req, extractGoal(req))",
  "classified.LogHandler",
  "classified.TraceHandler",
  "tracer.Decide(ctx, classified.TraceHandler, classified.TraceReason, classified.TraceScore, classified.TraceMeta)",
]) {
  if (!runtimeClassificationApplication.includes(needle)) {
    failures.push(`${runtimeClassificationApplicationRel} missing runtime classification application boundary ${JSON.stringify(needle)}`);
  }
}

const plannerSourceForClassification = read("internal/agentcore/planner/planner.go");
for (const forbidden of [
  "ClassifyRequest(ctx, req, extractGoal(req))",
  "classified.LogHandler",
  "classified.LogIntent",
  "classified.TraceHandler",
  "classified.TraceMeta",
  "tracer.Decide(ctx, classified.",
]) {
  if (plannerSourceForClassification.includes(forbidden)) {
    failures.push(`internal/agentcore/planner/planner.go leaks runtime classification application detail ${JSON.stringify(forbidden)}; keep it in ${runtimeClassificationApplicationRel}`);
  }
}
if (!runtimeRequestPipeline.includes("classified := p.applyRuntimeClassification(ctx, req)")) {
  failures.push(`${runtimeRequestPipelineRel} should apply LocalBrain classification through applyRuntimeClassification`);
}

const executionModeDispatch = read(executionModeDispatchRel);
for (const needle of [
  "func (p *Planner) dispatchExecutionMode(ctx context.Context, req PlanRequest) (*PlanResult, error)",
  "switch mode.Mode",
  "PlanExecutionLongHorizon",
  "p.emitCognitiveLoadEvent(req, mode.CognitiveLoad)",
  "return p.runLongHorizon(ctx, req)",
  "return p.runReAct(ctx, req)",
  "return p.runNativeFC(ctx, req)",
  "return p.runTextBased(ctx, req)",
]) {
  if (!executionModeDispatch.includes(needle)) {
    failures.push(`${executionModeDispatchRel} missing execution-mode dispatch boundary ${JSON.stringify(needle)}`);
  }
}

const plannerSourceForDispatch = read("internal/agentcore/planner/planner.go");
for (const forbidden of [
  "switch mode.Mode",
  "PlanExecutionLongHorizon",
  "PlanExecutionReAct",
  "PlanExecutionNativeFC",
  "runLongHorizon(ctx, req)",
  "runReAct(ctx, req)",
  "runNativeFC(ctx, req)",
  "runTextBased(ctx, req)",
]) {
  if (plannerSourceForDispatch.includes(forbidden)) {
    failures.push(`internal/agentcore/planner/planner.go leaks execution-mode dispatch detail ${JSON.stringify(forbidden)}; keep it in ${executionModeDispatchRel}`);
  }
}
for (const needle of [
  "func (p *Planner) runInner(ctx context.Context, req PlanRequest) (*PlanResult, error)",
  "slog.Debug(\"planner: model override\"",
  "slog.Info(\"planner: chat-only mode, skipping all tools\")",
  "classified := p.applyRuntimeClassification(ctx, req)",
  "return p.runToolFreeChat(ctx, req, \"planner chat-only\", 0)",
  "return p.runToolFreeChat(ctx, req, \"planner tool-free chat\", 1)",
  "return p.dispatchExecutionMode(ctx, req)",
]) {
  if (!runtimeRequestPipeline.includes(needle)) {
    failures.push(`${runtimeRequestPipelineRel} missing runtime request pipeline token ${JSON.stringify(needle)}`);
  }
}

const plannerSourceForPipeline = read("internal/agentcore/planner/planner.go");
for (const forbidden of [
  "func (p *Planner) Run(",
  "func (p *Planner) runInner(",
  "func safeToolGo(",
  "observe.StartSpan(ctx, \"planner.Run\")",
  ".AfterRun(ctx, req, result, err, p.reflect)",
  "planner: chat-only mode, skipping all tools",
  "NeedTools=false, using tool-free chat path",
  "classified := p.applyRuntimeClassification(ctx, req)",
  "return p.dispatchExecutionMode(ctx, req)",
  '"log/slog"',
]) {
  if (plannerSourceForPipeline.includes(forbidden)) {
    failures.push(`internal/agentcore/planner/planner.go leaks runtime request pipeline detail ${JSON.stringify(forbidden)}; keep it in ${runtimeRequestPipelineRel}`);
  }
}

const runtimeRequestLifecycle = read(runtimeRequestLifecycleRel);
for (const needle of [
  "func (p *Planner) Run(ctx context.Context, req PlanRequest) (*PlanResult, error)",
  "observe.StartSpan(ctx, \"planner.Run\")",
  "span.Attrs[\"tenant_id\"] = req.TenantID",
  "span.Attrs[\"mode\"] = string(p.executionMode(req).Mode)",
  "result, err := p.runInner(ctx, req)",
  "observe.EndSpan(span, err)",
  "p.learningSidecar.AfterRun(ctx, req, result, err, p.reflect)",
]) {
  if (!runtimeRequestLifecycle.includes(needle)) {
    failures.push(`${runtimeRequestLifecycleRel} missing request lifecycle token ${JSON.stringify(needle)}`);
  }
}

const toolExecutionAsync = read(toolExecutionAsyncRel);
for (const needle of [
  "func safeToolGo(ctx context.Context, timeout time.Duration, fn func(ctx context.Context))",
  "slog.Error(\"planner: tool goroutine panic\"",
  "context.WithTimeout(ctx, timeout)",
]) {
  if (!toolExecutionAsync.includes(needle)) {
    failures.push(`${toolExecutionAsyncRel} missing async tool execution helper ${JSON.stringify(needle)}`);
  }
}

const promptBuilder = read(promptBuilderRel);
for (const forbidden of [
  '"yunque-agent/internal/agentcore/localbrain"',
  "localbrain.ContextItem",
  "localbrain.FilterResult",
]) {
  if (promptBuilder.includes(forbidden)) {
    failures.push(`${promptBuilderRel} leaks LocalBrain DTO ${JSON.stringify(forbidden)}; use runtime strategy DTOs instead`);
  }
}

for (const forbidden of [
  "func (s *RuntimeStrategyService) AgenticThinking()",
  "func (s *RuntimeStrategyService) Classify(ctx context.Context, query, tenantID string) (*localbrain.Decision, error)",
  "func (s *RuntimeStrategyService) SetLocalBrain(brain *localbrain.LocalBrain)",
  "func (s *RuntimeStrategyService) SetAgenticThinking(thinking *localbrain.AgenticThinking)",
]) {
  if (runtimeStrategy.includes(forbidden)) {
    failures.push(`${runtimeStrategyRel} exposes raw AgenticThinking via removed getter ${JSON.stringify(forbidden)}`);
  }
}

for (const [relPath, label] of [
  ["internal/agentcore/planner/react_integration.go", "ReAct"],
  ["internal/agentcore/planner/long_horizon.go", "long-horizon"],
]) {
  const source = read(relPath);
  for (const forbidden of [
    '"yunque-agent/internal/agentcore/localbrain"',
    "localbrain.ThinkRequest",
    "localbrain.StepSummary",
    "localbrain.ThinkResult",
  ]) {
    if (source.includes(forbidden)) {
      failures.push(`${relPath} leaks LocalBrain adaptive-thinking DTO ${JSON.stringify(forbidden)} in ${label}; use RuntimeStrategyService runtime DTOs instead`);
    }
  }
}

const contextAssembly = read(contextAssemblyRel);
for (const needle of [
  "func (s *ContextAssemblyService) BuildDynamicContext",
  "func (s *ContextAssemblyService) AppendDynamicContextMessage",
  "type DynamicContextAssemblyRequest",
  "type DynamicContextAssemblyResult",
  "func (s *ContextAssemblyService) AppendGraphContext",
  "func (s *ContextAssemblyService) GraphContextFor",
  "func (s *ContextAssemblyService) CogniContext",
  "func (s *ContextAssemblyService) ApplyCogniSkillFilter",
  "func (s *ContextAssemblyService) EmitCogniTrace",
  "func (s *ContextAssemblyService) EmitCogniTraceForRequest",
]) {
  if (!contextAssembly.includes(needle)) {
    failures.push(`${contextAssemblyRel} missing required Cogni boundary entrypoint ${JSON.stringify(needle)}`);
  }
}

for (const removed of [
  "func (s *ContextAssemblyService) HasCogniTrace",
  "func (s *ContextAssemblyService) CogniTrace",
  "func (s *ContextAssemblyService) HasCogniSkillFilter",
  "func (s *ContextAssemblyService) FilterCogniSkills",
  "func (p *Planner) maybeEmitCogniTrace",
]) {
  if (contextAssembly.includes(removed) || read("internal/agentcore/planner/cogni_observability.go").includes(removed)) {
    failures.push(`removed Cogni boundary wrapper still exists: ${removed}`);
  }
}

const conceptMap = read("doc/AGENTCORE-CONCEPT-MAP.md");
for (const needle of [
  "EmitCogniTraceForRequest",
  "AppendDynamicContextMessage",
  "ExecutionRuntimeService.BuildSkillEnvironment",
  "ModelRuntimeService.ChatForRequest",
  "ModelRuntimeService.AnalyzeUploadedFile",
  "ModelRuntimeService.GenerateConversationTitle",
  "ParseMissionIntent",
  "ModelRuntimeService.ModelIDForTier",
  "ModelRuntimeService.Health",
  "ModelRuntimeHealth",
  "RuntimeStrategyService.FilterContext",
  "RuntimeContextItem",
  "RuntimeContextFilterResult",
  "RuntimeThinkRequest",
  "RuntimeThinkStepSummary",
  "RuntimeThinkResult",
  "RuntimeIntent",
  "RuntimeDecision",
  "RuntimeClassificationResult",
  "runtime_classification_application.go",
  "runtime_request_lifecycle.go",
  "runtime_request_pipeline.go",
  "PlanExecutionMode",
  "SelectExecutionMode",
  "runtime_strategy_selection.go",
  "execution_mode_dispatch.go",
  "tool_free_chat_runtime.go",
  "runtime_request_contracts.go",
  "runtime_extension_contracts.go",
  "planner_runtime_setters.go",
  "planner_runtime_facades.go",
  "planner_runtime_services.go",
  "request DTO / callback / result-summary contracts",
  "extension/callback contracts",
  "runtime setter/facade split",
  "runtime service factory split",
  "ExecutionRuntimeService.ApplyToolFailureRecoveryForRequest",
  "tool-failure recovery post-processing",
  "ExecutionRuntimeService.ToolPostprocessStateForRequest",
  "tool postprocess state helper",
  "ExecutionRuntimeService.CollectToolResultsInOrder",
  "ordered tool-result collection helper",
  "ExecutionRuntimeService.ApplyToolResultPostprocessForState",
  "tool-result postprocess application helper",
  "ExecutionRuntimeService.PartialPlanResultForRequest",
  "partial-result fallback post-processing",
  "ExecutionRuntimeService.EmitToolStartForRequest",
  "tool-start event emission",
  "ExecutionRuntimeService.ApplyReflectRetryForRequest",
  "reflect-retry post-processing",
  "ExecutionRuntimeService.PlanResultStateForRequest",
  "plan-result state helper",
  "ExecutionRuntimeService.ToolPostprocessStateForRequest",
  "tool postprocess state helper",
  "ExecutionRuntimeService.CollectToolResultsInOrder",
  "ordered tool-result collection helper",
  "ExecutionRuntimeService.ApplyToolResultPostprocessForState",
  "tool-result postprocess application helper",
  "ChatFallbackForRequest",
  "ChatWithToolsFallbackForRequest",
  "LocalBrainRuntime",
  "AgenticThinkerRuntime",
  "HasContextFilter",
  "ModelRuntimeService.FallbackChainForRequest",
  "rather than constructing dynamic context messages, reading raw graph callbacks from Planner, or reaching into `CogniContextService`",
]) {
  if (!conceptMap.includes(needle)) {
    failures.push(`doc/AGENTCORE-CONCEPT-MAP.md missing ${JSON.stringify(needle)}`);
  }
}

const taskLedger = read("doc/LONG-TERM-TASKS.md");
for (const needle of [
  "第十六批删除 Planner 层 `maybeEmitCogniTrace`",
  "ContextAssemblyService.EmitCogniTraceForRequest",
  "第十七批删除 Planner 层 `chatFallback`",
  "ModelRuntimeService.ChatFallback",
  "第二十二批删除 Planner 层 `buildEnv`",
  "ExecutionRuntimeService.BuildSkillEnvironment",
  "ExecutionRuntimeService.ApplyToolResultForRequest",
  "第二十三批新增 `ModelRuntimeService.ChatForRequest`",
  "ChatWithToolsForRequest",
  "第二十四批新增 `ModelRuntimeService.FallbackChainForRequest`",
  "第二十五批把 native-FC thinking flag 自动启用",
  "第二十六批把上传文件分析",
  "第二十七批把 Gateway 对话标题生成",
  "ModelRuntimeService.AnalyzeUploadedFile",
  "ModelRuntimeService.GenerateConversationTitle",
  "ParseMissionIntent",
  "ModelRuntimeService.ModelIDForTier",
  "ModelRuntimeService.Health",
  "LLMResponseCacheStats",
  "ModelRuntimeHealth",
  "第二十九批把 Gateway/health/cmd 对 `LLMBreaker()` 原始 breaker 的读取收进 `ModelRuntimeService.Health`",
  "第三十批删除 Planner 对外 `LLMPool` / `LLMClient` / `LLMClientFor` 原始模型对象 facade",
  "第三十一批把 graph context 链式拼接下沉到 `ContextAssemblyService.AppendGraphContext`",
  "第三十二批把 PromptBuilder 的 LocalBrain 上下文过滤下沉到 `RuntimeStrategyService.FilterContext`",
  "第三十三批把 LocalBrain filter DTO 转换继续收进 `RuntimeStrategyService`",
  "第三十四批把 ReAct/long-horizon 的 AgenticThinking 请求/结果 DTO 转换继续收进 `RuntimeStrategyService`",
  "第三十五批把 `Planner.runInner` 的 LocalBrain Decision/Intent 消费收进 `RuntimeStrategyService`",
  "第三十六批",
  "LocalBrainRuntime",
  "AgenticThinkerRuntime",
  "planner.go` 不再 import `internal/agentcore/localbrain`",
  "第三十七批",
  "RuntimeClassificationResult",
  "ClassifyRequest",
  "第三十八批",
  "第三十九批",
  "PlanExecutionMode",
  "SelectExecutionMode",
  "runtime_strategy_selection.go",
  "planner.go` 不再 import `internal/agentcore/plan`",
  "第四十批",
  "tool_free_chat_runtime.go",
  "runToolFreeChat",
  "第四十一批",
  "runtime_classification_application.go",
  "applyRuntimeClassification",
  "第四十二批",
  "execution_mode_dispatch.go",
  "dispatchExecutionMode",
  "第四十三批",
  "runtime_request_pipeline.go",
  "tool_execution_async.go",
  "第四十四批",
  "runtime_request_lifecycle.go",
  "第四十七批",
  "runtime_request_contracts.go",
  "WithStepCallback",
  "ExecutionSummary",
  "第四十八批",
  "runtime_extension_contracts.go",
  "SkillMetricsFunc",
  "DynContextBudgetDefault",
  "第四十九批",
  "planner_runtime_setters.go",
  "planner_runtime_facades.go",
  "runtime setter/facade",
  "第五十批",
  "planner_runtime_services.go",
  "ensureContextAssembly",
  "runtime service factory",
  "第五十一批",
  "ChatFallbackForRequest",
  "ChatWithToolsFallbackForRequest",
  "request fallback wrapper",
  "第五十二批",
  "ExecuteHandoffForRequest",
  "handoff execution wrapper",
  "第五十三批",
  "tool-result post-processing helper",
  "ToolResultPostprocessRequest",
  "ApplyToolResultForRequest",
  "第五十四批",
  "tool-failure recovery post-processing helper",
  "ToolFailureRecoveryRequest",
  "ApplyToolFailureRecoveryForRequest",
  "第六十七批",
  "tool postprocess state helper",
  "ToolPostprocessStateForRequest",
  "ToolResultPostprocessRequestForState",
  "ToolFailureRecoveryRequestForState",
  "第六十八批",
  "ordered tool-result collection helper",
  "ToolExecutionResult",
  "CollectToolResultsInOrder",
  "第六十九批",
  "tool-result postprocess application helper",
  "ToolResultPostprocessApplicationRequest",
  "ApplyToolResultPostprocessForState",
  "第五十五批",
  "partial-result fallback post-processing helper",
  "PartialPlanResultRequest",
  "PartialPlanResultForRequest",
  "第五十六批",
  "tool-start event emission helper",
  "ToolStartEventRequest",
  "EmitToolStartForRequest",
  "第五十七批",
  "reflect-retry post-processing helper",
  "ReflectRetryRequest",
  "ApplyReflectRetryForRequest",
  "第五十九批",
  "text reflection prompt post-processing helper",
  "TextReflectionPromptRequest",
  "BuildTextReflectionPromptForRequest",
  "第六十批",
  "final answer prompt post-processing helper",
  "FinalAnswerPromptRequest",
  "BuildFinalAnswerPromptForRequest",
  "第六十一批",
  "terminal safe result post-processing helper",
  "TerminalPlanResultRequest",
  "TerminalPlanResultForRequest",
  "第六十二批",
  "text reflection message post-processing helper",
  "AssistantReply",
  "Messages",
  "AssistantToolCallMessageForRequest",
  "RecoveryPromptMessageForRequest",
  "第六十三批",
  "task stopped reply helper",
  "TaskStoppedReply",
  "第六十四批",
  "task-stopped result helper",
  "TaskStoppedPlanResultForRequest",
  "第六十五批",
  "successful result helper",
  "SuccessfulPlanResultForRequest",
  "第六十六批",
  "plan-result state helper",
  "PlanResultStateForRequest",
  "ToolPostprocessStateForRequest",
  "ToolResultPostprocessRequestForState",
  "ToolFailureRecoveryRequestForState",
  "ToolExecutionResult",
  "CollectToolResultsInOrder",
  "ToolResultPostprocessApplicationRequest",
  "ApplyToolResultPostprocessForState",
  "RuntimeContextItem",
  "RuntimeContextFilterResult",
  "RuntimeThinkRequest",
  "RuntimeThinkStepSummary",
  "RuntimeThinkResult",
  "scripts/check-planner-cogni-boundary.mjs",
]) {
  if (!taskLedger.includes(needle)) {
    failures.push(`doc/LONG-TERM-TASKS.md missing ${JSON.stringify(needle)}`);
  }
}

const modelRuntime = read(modelRuntimeRel);
const modelRuntimeTasks = read(modelRuntimeTasksRel);
const delegationRuntime = read(delegationRuntimeRel);
for (const needle of [
  "func (s *ModelRuntimeService) ChatFallback",
  "func (s *ModelRuntimeService) ChatFallbackFull",
  "func (s *ModelRuntimeService) ChatWithToolsFallback",
  "func (s *ModelRuntimeService) ChatFallbackForRequest",
  "func (s *ModelRuntimeService) ChatFallbackFullForRequest",
  "func (s *ModelRuntimeService) ChatWithToolsFallbackForRequest",
  "func (s *ModelRuntimeService) ChatForRequest",
  "func (s *ModelRuntimeService) ChatForRequestTier",
  "func (s *ModelRuntimeService) ChatWithToolsForRequest",
  "func (s *ModelRuntimeService) ModelIDForTier",
  "func (s *ModelRuntimeService) DefaultResponseCacheStats",
  "func (s *ModelRuntimeService) Health",
  "func (s *ModelRuntimeService) AnalyzeUploadedFile",
  "func (s *ModelRuntimeService) FallbackChainForRequest",
  "func (s *ModelRuntimeService) FallbackChainForPlannerRequest",
  "func (s *ModelRuntimeService) AdaptiveRoute",
  "func (s *ModelRuntimeService) ThinkingFlagForRequest",
  "func (s *ModelRuntimeService) ReasoningCallbacks",
  "func requiredCapabilitiesForMessages",
  "func shouldAutoThink",
  "type ModelRuntimeHealth struct",
]) {
  if (!modelRuntime.includes(needle)) {
    failures.push(`${modelRuntimeRel} missing required model runtime entrypoint ${JSON.stringify(needle)}`);
  }
}

for (const [relPath, label] of [
  ["internal/agentcore/planner/executor_fc.go", "native-FC executor"],
  ["internal/agentcore/planner/executor_text.go", "text executor"],
  [toolFreeChatRuntimeRel, "tool-free runtime"],
]) {
  const source = read(relPath);
  for (const forbidden of [
    "FallbackChainForPlannerRequest(req, messages, p.runtimeStrategy)",
    "ThinkingFlagForRequest(req)",
    "ReasoningCallbacks(p.modelReasoningEvents(req))",
  ]) {
    if (source.includes(forbidden)) {
      failures.push(`${relPath} leaks model request fallback assembly detail ${JSON.stringify(forbidden)} in ${label}; use ModelRuntimeService request fallback wrappers`);
    }
  }
}

for (const needle of [
  "type HandoffExecutionHooks struct",
  "type HandoffExecutionResult struct",
  "func (s *DelegationRuntimeService) HandoffTimeoutForTool",
  "func (s *DelegationRuntimeService) ExecuteHandoffForRequest",
  "func handoffInputFromArgs",
  "func emitHandoffStart",
  "func emitHandoffDone",
]) {
  if (!delegationRuntime.includes(needle)) {
    failures.push(`${delegationRuntimeRel} missing required handoff execution wrapper entrypoint ${JSON.stringify(needle)}`);
  }
}

for (const [relPath, label] of [
  ["internal/agentcore/planner/executor_fc.go", "native-FC executor"],
  ["internal/agentcore/planner/executor_text.go", "text executor"],
]) {
  const source = read(relPath);
  for (const forbidden of [
    "ExecuteHandoff(cbCtx,",
    "EventHandoffStart",
    "EventHandoffDone",
    "buildHandoffFailureDetail(",
    "handoffFailureSummary(",
    "WithStepCallback(toolCtx, req.StepCallback)",
    ".RecordExecutionFailure(err != nil)",
  ]) {
    if (source.includes(forbidden)) {
      failures.push(`${relPath} leaks handoff execution detail ${JSON.stringify(forbidden)} in ${label}; use DelegationRuntimeService.ExecuteHandoffForRequest`);
    }
  }
  for (const required of [
    "HandoffTimeoutForTool",
    "ExecuteHandoffForRequest",
    "EmitToolStartForRequest",
    "ApplyReflectRetryForRequest",
    "ApplyToolResultPostprocessForState",
    "ApplyToolFailureRecoveryForRequest",
    "ToolPostprocessStateForRequest",
    "ToolFailureRecoveryRequestForState",
    "CollectToolResultsInOrder",
    "PartialPlanResultForRequest",
    "PlanResultStateForRequest",
    "TaskStoppedPlanResultForRequest",
    "SuccessfulPlanResultForRequest",
  ]) {
    if (!source.includes(required)) {
      failures.push(`${relPath} should route request-level execution helper ${required} through its runtime service`);
    }
  }
  const pathSpecificRequired = relPath.endsWith("executor_fc.go")
    ? [
      "EmitStepThinkingForRequest",
      "AssistantToolCallMessageForRequest",
      "RecoveryPromptMessageForRequest",
      "BuildFinalAnswerPromptForRequest",
      "TerminalPlanResultForRequest",
    ]
    : ["ReasoningDeltaCallbackForRequest", "BuildTextReflectionPromptForRequest"];
  for (const required of pathSpecificRequired) {
    if (!source.includes(required)) {
      failures.push(`${relPath} should route request-level execution helper ${required} through its runtime service`);
    }
  }
  for (const forbidden of [
    "EventToolStart",
    "observe.ToolStartDetail",
    "EventToolResult",
    "observe.ToolResultDetail",
    "buildToolResultMsg(",
    "pruneToolResult(",
    "recordSkillRecommendationOutcome(",
    "plannerFriendlyFailureText(r.err.Error())",
  ]) {
    if (source.includes(forbidden)) {
      failures.push(`${relPath} leaks tool-result post-processing detail ${JSON.stringify(forbidden)} in ${label}; use ExecutionRuntimeService.ApplyToolResultForRequest`);
    }
  }
  for (const forbidden of [
    "buildPlannerFailureSummary(",
    "maybeEmitFailureRecovery(",
    "formatFailureRecoveryPrompt(",
    "lastRecoveryFailedCount = summary.FailedCount",
  ]) {
    if (source.includes(forbidden)) {
      failures.push(`${relPath} leaks tool-failure recovery post-processing detail ${JSON.stringify(forbidden)} in ${label}; use ExecutionRuntimeService.ApplyToolFailureRecoveryForRequest`);
    }
  }
  for (const forbidden of [
    "llm.Message{Role: \"assistant\"",
    "llm.Message{Role: \"user\"",
    "工具调用结果:\\n",
    "请评估以上结果",
    "你已执行了足够多的步骤",
    "连接暂时中断",
    "现场已保留",
  ]) {
    if (source.includes(forbidden)) {
      failures.push(`${relPath} leaks execution prompt post-processing detail ${JSON.stringify(forbidden)} in ${label}; use ExecutionRuntimeService request prompt helpers`);
    }
  }
  for (const forbidden of [
    "partialPlanResult(",
    "maybeEmitPartialResult(",
    "buildPartialPlanReply(",
    "buildPartialResultDetail(",
    "PlanResult{",
  ]) {
    if (source.includes(forbidden)) {
      failures.push(`${relPath} leaks PlanResult shaping detail ${JSON.stringify(forbidden)} in ${label}; use ExecutionRuntimeService result helpers`);
    }
  }
  for (const forbidden of [
    "UsedSkills:    usedSkills",
    "PlanSteps:     planSteps",
    "ContextLayers: ctxLayers",
    "UsedSkills:       usedSkills",
    "PlanSteps:        planSteps",
    "ContextLayers:    ctxLayers",
  ]) {
    if (source.includes(forbidden) && !source.includes("PlanResultStateForRequest")) {
      failures.push(`${relPath} repeats PlanResult state mapping ${JSON.stringify(forbidden)} in ${label}; use ExecutionRuntimeService.PlanResultStateForRequest`);
    }
  }
  for (const forbidden of [
    "append(usedSkills, processed.UsedSkill)",
    "append(planSteps, processed.Step)",
    "processed := p.ensureExecutionRuntime().ApplyToolResultForRequest",
    "ToolResultPostprocessRequest{",
    "ToolFailureRecoveryRequest{",
  ]) {
    if (source.includes(forbidden)) {
      failures.push(`${relPath} repeats tool postprocess request construction ${JSON.stringify(forbidden)} in ${label}; use ExecutionRuntimeService.ToolPostprocessStateForRequest and request constructors`);
    }
  }
  for (const forbidden of [
    "type tcResult struct",
    "type callResult struct",
    "make([]tcResult",
    "make([]callResult",
    "for range toolCalls",
    "for range calls",
  ]) {
    if (source.includes(forbidden)) {
      failures.push(`${relPath} repeats ordered tool-result collection detail ${JSON.stringify(forbidden)} in ${label}; use ExecutionRuntimeService.CollectToolResultsInOrder`);
    }
  }
  for (const forbidden of [
    "EventThinking",
    "thinking_delta",
    "正在思考 (第",
    "EventReflect",
    "planner.reflect_retry",
    "你的回答质量不够好，请重新组织更完善的回答。",
  ]) {
    if (source.includes(forbidden)) {
      failures.push(`${relPath} leaks reflect-retry post-processing detail ${JSON.stringify(forbidden)} in ${label}; use ExecutionRuntimeService.ApplyReflectRetryForRequest`);
    }
  }
}

const plannerSource = read(plannerRuntimeFacadesRel);
for (const needle of [
  "func (p *Planner) ModelRuntimeHealth() ModelRuntimeHealth",
]) {
  if (!plannerSource.includes(needle)) {
    failures.push(`${plannerRuntimeFacadesRel} missing required model runtime facade ${JSON.stringify(needle)}`);
  }
}

for (const needle of [
  "func (s *ModelRuntimeService) GenerateConversationTitle",
  "func (s *ModelRuntimeService) ParseMissionIntent",
  "type MissionParseResult struct",
]) {
  if (!modelRuntimeTasks.includes(needle)) {
    failures.push(`${modelRuntimeTasksRel} missing required model runtime task entrypoint ${JSON.stringify(needle)}`);
  }
}

const promptRuntime = read(promptRuntimeRel);
for (const needle of [
  "func (s *PromptRuntimeService) BuildStablePrefix",
  "func (s *PromptRuntimeService) PrepareConversationMessages",
  "func (s *PromptRuntimeService) ReflectRetryPrompt",
  "func (s *PromptRuntimeService) TaskStoppedReply",
  "[时间:",
  "[任务焦点] 用户的核心目标:",
  "ContentParts",
]) {
  if (!promptRuntime.includes(needle)) {
    failures.push(`${promptRuntimeRel} missing required conversation preparation behavior ${JSON.stringify(needle)}`);
  }
}

const contextWindowRuntime = read(contextWindowRuntimeRel);
for (const needle of [
  "func (s *ContextWindowRuntimeService) FitMessagesForRequest",
  "s.PruneToolResults(messages, 6000)",
  "s.CompressAndTrim(ctx, messages, client)",
]) {
  if (!contextWindowRuntime.includes(needle)) {
    failures.push(`${contextWindowRuntimeRel} missing required message fitting behavior ${JSON.stringify(needle)}`);
  }
}

const executionRuntime = read(executionRuntimeRel);
for (const needle of [
  "func (s *ExecutionRuntimeService) BuildSkillEnvironment",
  "type ToolStartEventRequest struct",
  "func (s *ExecutionRuntimeService) EmitToolStartForRequest",
  "type ThinkingEventRequest struct",
  "func (s *ExecutionRuntimeService) EmitThinkingForRequest",
  "func (s *ExecutionRuntimeService) EmitStepThinkingForRequest",
  "func (s *ExecutionRuntimeService) ReasoningDeltaCallbackForRequest",
  "type ReflectRetryRequest struct",
  "type ReflectRetryResult struct",
  "func (s *ExecutionRuntimeService) ApplyReflectRetryForRequest",
  "type AssistantToolCallMessageRequest struct",
  "func (s *ExecutionRuntimeService) AssistantToolCallMessageForRequest",
  "type RecoveryPromptMessageRequest struct",
  "func (s *ExecutionRuntimeService) RecoveryPromptMessageForRequest",
  "type ToolResultPostprocessRequest struct",
  "type ToolResultPostprocessResult struct",
  "type ToolPostprocessExecutionState struct",
  "type ToolResultPostprocessApplicationRequest struct",
  "type ToolResultPostprocessApplicationResult struct",
  "type ToolExecutionResult struct",
  "func (s *ExecutionRuntimeService) ToolPostprocessStateForRequest",
  "func (s *ExecutionRuntimeService) CollectToolResultsInOrder",
  "func (s *ExecutionRuntimeService) ToolResultPostprocessRequestForState",
  "func (s *ExecutionRuntimeService) ToolFailureRecoveryRequestForState",
  "func (s *ExecutionRuntimeService) ApplyToolResultForRequest",
  "func (s *ExecutionRuntimeService) ApplyToolResultPostprocessForState",
  "type ToolFailureRecoveryRequest struct",
  "type ToolFailureRecoveryResult struct",
  "func (s *ExecutionRuntimeService) ApplyToolFailureRecoveryForRequest",
  "type TextReflectionPromptRequest struct",
  "type TextReflectionPromptResult struct",
  "func (s *ExecutionRuntimeService) BuildTextReflectionPromptForRequest",
  "type FinalAnswerPromptRequest struct",
  "type FinalAnswerPromptResult struct",
  "func (s *ExecutionRuntimeService) BuildFinalAnswerPromptForRequest",
  "type PlanResultExecutionState struct",
  "func (state PlanResultExecutionState) ResultSteps() int",
  "type PlanResultStateRequest struct",
  "func (s *ExecutionRuntimeService) PlanResultStateForRequest",
  "type TerminalPlanResultRequest struct",
  "func (s *ExecutionRuntimeService) TerminalPlanResultForRequest",
  "type TaskStoppedPlanResultRequest struct",
  "func (s *ExecutionRuntimeService) TaskStoppedPlanResultForRequest",
  "type SuccessfulPlanResultRequest struct",
  "func (s *ExecutionRuntimeService) SuccessfulPlanResultForRequest",
  "type PartialPlanResultRequest struct",
  "func (s *ExecutionRuntimeService) PartialPlanResultForRequest",
  "func emitToolResultEvent",
  "func emitFailureRecoveryEvent",
  "func emitPartialResultEvent",
  "func emitReflectRetryEvent",
  "func pruneToolResult",
  "func buildToolResultMsg",
  "LLMCall:",
  "MemorySearch:",
  "modelRuntime.ClientForRequest(req)",
  "contextAssembly.Memory(ctx, tenantID, query)",
]) {
  if (!executionRuntime.includes(needle)) {
    failures.push(`${executionRuntimeRel} missing required skill environment behavior ${JSON.stringify(needle)}`);
  }
}

if (failures.length > 0) {
  console.error("Planner Cogni boundary check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log("Planner Cogni boundary check passed.");
console.log("Cogni context, skill surface, trace emission, and model fallback loops stay behind runtime services.");
