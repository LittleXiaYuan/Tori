# yunque-client (TypeScript)

TypeScript client for the Yunque (云雀) Agent HTTP API. The package contains
both the generated full OpenAPI client and hand-written incremental slices for
product integrations that should avoid importing the whole platform surface.

- Source spec: [`docs/openapi.yaml`](../../docs/openapi.yaml)
- Generator: [`@hey-api/openapi-ts`](https://github.com/hey-api/openapi-ts)
- Runtime: [`@hey-api/client-fetch`](https://heyapi.dev/openapi-ts/clients/fetch) (zero-dep, native fetch)

## Install

From the repo root:

```bash
cd sdk/typescript
npm install
```

When/if we publish to npm, install with `npm i yunque-client`.

## Quick start

For app code, prefer subpath imports such as `yunque-client/chat` or
`yunque-client/planner-recovery`. The package root (`yunque-client`) re-exports
the generated all-in-one client for full API coverage and is intentionally
heavier.

### Incremental client

```ts
import { createChatClient } from "yunque-client/chat";

const chat = createChatClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});

const reply = await chat.send({
  messages: [{ role: "user", content: "帮我总结这个任务的下一步" }],
  session_id: "session_123",
});

console.log(reply);
```

### Full generated client

Use this path when you need broad OpenAPI coverage and accept the larger import
surface:

```ts
import { evolveCogni, generateCogni, listCognis } from "yunque-client";

const options = {
  baseUrl: "http://localhost:9090",
  headers: { Authorization: "Bearer <your-jwt>" },
};

// List every Cogni
const { data, error } = await listCognis(options);
if (error) throw error;
console.log(data);

// Self-generate a Cogni
const generated = await generateCogni({
  ...options,
  body: { prompt: "Build a code-review cogni" },
});

// Trigger evolution on one cogni
await evolveCogni({ ...options, path: { id: "code-reviewer" } });
```

## Incremental imports

The generated `src/sdk.gen.ts` is useful for full API coverage, but it is a
large all-in-one surface. Product integrations that only need Planner recovery
can import the hand-written incremental slice instead:

Planner recovery keeps request actions and server recommendations separate.
Use `CheckpointRecoveryAction` (`continue` / `retry_failed` / `partial`) when
posting a checkpoint resume request. Treat broader `RecoveryNextAction` values
such as `inspect_dependencies` or `create_task` as UI guidance instead of
submitting them directly. The gateway accepts common UI aliases such as
`retry-failed-step` or `重试失败`, but SDK callers should prefer the canonical
request actions for portable integrations.

The package declares `sideEffects: false`, so modern bundlers can drop unused
subpath slices when applications import only the clients they need. Prefer
subpath imports like `yunque-client/planner-recovery` for the smallest runtime
surface; reserve the package root for full generated API coverage.

```ts
import { createAuthClient } from "yunque-client/auth";
import { createAiriClient } from "yunque-client/airi";
import { createPlannerRecoveryClient } from "yunque-client/planner-recovery";
import { createPlannerClient } from "yunque-client/planner";
import { createChatClient } from "yunque-client/chat";
import { createCognisClient } from "yunque-client/cognis";
import { createCognisRegistryClient } from "yunque-client/cognis-registry";
import { createCognisObserveClient } from "yunque-client/cognis-observe";
import { createCognisExperienceClient } from "yunque-client/cognis-experience";
import { createCognisEvolutionClient } from "yunque-client/cognis-evolution";
import { createCognisFederationClient } from "yunque-client/cognis-federation";
import { createCognisWorkflowsClient } from "yunque-client/cognis-workflows";
import { createCognisBundlesClient } from "yunque-client/cognis-bundles";
import { createEventsClient } from "yunque-client/events";
import { createRealtimeClient } from "yunque-client/realtime";
import { createWebChatClient } from "yunque-client/webchat";
import { createConversationsClient } from "yunque-client/conversations";
import { createSubagentsClient } from "yunque-client/subagents";
import { createBotsClient } from "yunque-client/bots";
import { createDiscoveryClient } from "yunque-client/discovery";
import { createIdentityClient } from "yunque-client/identity";
import { createEmbeddingsClient } from "yunque-client/embeddings";
import { createSearchClient } from "yunque-client/search";
import { createInteractionsClient } from "yunque-client/interactions";
import { createEmotionClient } from "yunque-client/emotion";
import { createReactionsClient } from "yunque-client/reactions";
import { createInstructionsClient } from "yunque-client/instructions";
import { createRBACClient } from "yunque-client/rbac";
import { createRolesClient } from "yunque-client/roles";
import { createPermissionsClient } from "yunque-client/permissions";
import { createMemoryClient } from "yunque-client/memory";
import { createMemorySearchClient } from "yunque-client/memory-search";
import { createMemoryStatsClient } from "yunque-client/memory-stats";
import { createMemoryAddClient } from "yunque-client/memory-add";
import { createMemoryCompactClient } from "yunque-client/memory-compact";
import { createTasksClient } from "yunque-client/tasks";
import { createTaskContextClient } from "yunque-client/task-context";
import { createTaskObserveClient } from "yunque-client/task-observe";
import { createTaskTemplatesClient } from "yunque-client/task-templates";
import { createTaskThreadsClient } from "yunque-client/task-threads";
import { createTaskLifecycleClient } from "yunque-client/task-lifecycle";
import { createTaskReadClient } from "yunque-client/task-read";
import { createTaskCreateClient } from "yunque-client/task-create";
import { createTaskDeleteClient } from "yunque-client/task-delete";
import { createKnowledgeClient } from "yunque-client/knowledge";
import { createKnowledgeSearchClient } from "yunque-client/knowledge-search";
import { createKnowledgeIngestClient } from "yunque-client/knowledge-ingest";
import { createKnowledgeSourcesClient } from "yunque-client/knowledge-sources";
import { createKnowledgeImportClient } from "yunque-client/knowledge-import";
import { createKnowledgeUploadClient } from "yunque-client/knowledge-upload";
import { createProvidersClient } from "yunque-client/providers";
import { createProviderRegistryClient } from "yunque-client/provider-registry";
import { createProviderControlClient } from "yunque-client/provider-control";
import { createProviderHealthClient } from "yunque-client/provider-health";
import { createBreakerClient } from "yunque-client/breaker";
import { createModelsClient } from "yunque-client/models";
import { createSetupClient } from "yunque-client/setup";
import { createSetupDetectClient } from "yunque-client/setup-detect";
import { createSetupTemplatesClient } from "yunque-client/setup-templates";
import { createSetupProviderClient } from "yunque-client/setup-provider";
import { createSetupInstallClient } from "yunque-client/setup-install";
import { createDocumentsClient } from "yunque-client/documents";
import { createApprovalsClient } from "yunque-client/approvals";
import { createApprovalQueueClient } from "yunque-client/approval-queue";
import { createApprovalRulesClient } from "yunque-client/approval-rules";
import { createTraceClient } from "yunque-client/trace";
import { createTraceEventsClient } from "yunque-client/trace-events";
import { createTaskTraceClient } from "yunque-client/task-trace";
import { createBrowserClient } from "yunque-client/browser";
import { createBrowserStatusClient } from "yunque-client/browser-status";
import { createBrowserCaptureClient } from "yunque-client/browser-capture";
import { createBrowserOPPClient } from "yunque-client/browser-opp";
import { createBrowserExtensionClient } from "yunque-client/browser-extension";
import { createRuntimeClient } from "yunque-client/runtime";
import { createRuntimeQueueClient } from "yunque-client/runtime-queue";
import { createRuntimeEventsClient } from "yunque-client/runtime-events";
import { createRouterClient } from "yunque-client/router";
import { createModesClient } from "yunque-client/modes";
import { createIDEClient } from "yunque-client/ide";
import { createPersonaClient } from "yunque-client/persona";
import { createWorkflowClient } from "yunque-client/workflow";
import { createWorkflowDefinitionsClient } from "yunque-client/workflow-definitions";
import { createWorkflowRunsClient } from "yunque-client/workflow-runs";
import { createCostClient } from "yunque-client/cost";
import { createCostBudgetClient } from "yunque-client/cost-budget";
import { createCostObserveClient } from "yunque-client/cost-observe";
import { createUsageClient } from "yunque-client/usage";
import { createLoRAClient } from "yunque-client/lora";
import { createLoRAObserveClient } from "yunque-client/lora-observe";
import { createLoRAControlClient } from "yunque-client/lora-control";
import { createIterateClient } from "yunque-client/iterate";
import { createIterateReviewClient } from "yunque-client/iterate-review";
import { createTrustClient } from "yunque-client/trust";
import { createReviewClient } from "yunque-client/review";
import { createSkillGrowClient } from "yunque-client/skillgrow";
import { createAuditClient } from "yunque-client/audit";
import { createHeartbeatClient } from "yunque-client/heartbeat";
import { createReverieClient } from "yunque-client/reverie";
import { createFederationClient } from "yunque-client/federation";
import { createSystemClient } from "yunque-client/system";
import { createSettingsClient } from "yunque-client/settings";
import { createToriClient } from "yunque-client/tori";
import { createSpeechClient } from "yunque-client/speech";
import { createAdminClient } from "yunque-client/admin";
import { createFilesClient } from "yunque-client/files";
import { createCronClient } from "yunque-client/cron";
import { createSkillHubClient } from "yunque-client/skillhub";
import { createSkillHubCatalogClient } from "yunque-client/skillhub-catalog";
import { createSkillHubInstallClient } from "yunque-client/skillhub-install";
import { createSkillHubPolicyClient } from "yunque-client/skillhub-policy";
import { createSkillsClient } from "yunque-client/skills";
import { createPluginsClient } from "yunque-client/plugins";
import { createPluginCatalogClient } from "yunque-client/plugin-catalog";
import { createPluginControlClient } from "yunque-client/plugin-control";
import { createPluginFilesClient } from "yunque-client/plugin-files";
import { createPluginCrudClient } from "yunque-client/plugin-crud";
import { createConnectorsClient } from "yunque-client/connectors";
import { createConnectorCatalogClient } from "yunque-client/connector-catalog";
import { createConnectorAuthClient } from "yunque-client/connector-auth";
import { createConnectorActionsClient } from "yunque-client/connector-actions";
import { createNotifyClient } from "yunque-client/notify";
import { createNotifyShareClient } from "yunque-client/notify-share";
import { createNotifyChannelsClient } from "yunque-client/notify-channels";
import { createProjectsClient } from "yunque-client/projects";
import { createProjectReadClient } from "yunque-client/project-read";
import { createProjectWriteClient } from "yunque-client/project-write";
import { createSkillMarketClient } from "yunque-client/market";
import { createDispatchClient } from "yunque-client/dispatch";
import { createOrchestratorClient } from "yunque-client/orchestrator";
import { createForkClient } from "yunque-client/fork";
import { createSchedulerClient } from "yunque-client/scheduler";
import { createUploadClient } from "yunque-client/upload";
import { createGraphClient } from "yunque-client/graph";
import { createPluginApiClient } from "yunque-client/plugin-api";
import { createPluginLLMClient } from "yunque-client/plugin-llm";
import { createPluginSearchClient } from "yunque-client/plugin-search";
import { createPluginMemoryClient } from "yunque-client/plugin-memory";
import { createPluginAgentMemoryClient } from "yunque-client/plugin-agent-memory";
import { createPluginKnowledgeClient } from "yunque-client/plugin-knowledge";
import { createPluginCronClient } from "yunque-client/plugin-cron";
import { createPluginSendClient } from "yunque-client/plugin-send";
import { createPluginExtensionsClient } from "yunque-client/plugin-extensions";
import { createStateClient } from "yunque-client/state";
import { createResourceStateClient } from "yunque-client/resource-state";
import { createFocusStateClient } from "yunque-client/focus-state";
import { createGoalStateClient } from "yunque-client/goal-state";
import { createTriggersClient } from "yunque-client/triggers";
import { createMissionsClient } from "yunque-client/missions";
import { createReflectClient } from "yunque-client/reflect";
import { createToolsClient } from "yunque-client/tools";
import { createSandboxClient } from "yunque-client/sandbox";

const auth = createAuthClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const tokenExchange = await auth.generateToken({ role: "viewer" });
console.log(tokenExchange.type);

const planner = createPlannerRecoveryClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});

const state = await planner.getExecutionState({ plan_id: "plan_123" });
if (state.next_action === "retry_failed") {
  await planner.resumeCheckpointPlan({
    plan_id: "plan_123",
    action: "retry_failed",
    async: true,
  });
}

const plannerFacade = createPlannerClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
const plannerState = await plannerFacade.getExecutionState({ plan_id: "plan_123" });
console.log(plannerState.next_action);

const chat = createChatClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const reply = await chat.send({
  messages: [{ role: "user", content: "你好呀" }],
  session_id: "demo-session",
});
console.log(reply.reply);

const webchat = createWebChatClient({ baseUrl: "http://localhost:9090" });
console.log(webchat.embedSnippet({ apiKey: "<your-api-key>", title: "Tori Assistant" }));

const conversations = createConversationsClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const history = await conversations.messages("demo-session");
const replay = await conversations.replay("demo-session", { limit: 5 });
console.log(history.count, replay.total_turns);

const subagents = createSubagentsClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const child = await subagents.spawn({
  parent_id: "demo-session",
  name: "reviewer",
  description: "检查 Planner 输出并补充风险提示",
  skills: ["review"],
});
await subagents.appendMessages(child.id, [{ role: "user", content: "请审阅当前计划。" }]);

const bots = createBotsClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const botList = await bots.list();
await bots.pushInbox({ source: "webhook", content: "新的外部消息", action: "notify" });
console.log(botList.total);

const discovery = createDiscoveryClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const profile = await discovery.resolveIdentity({ channel: "feishu", user_id: "42", display_name: "小羽" });
const web = await discovery.search("云雀 Agent Planner", { limit: 3 });
console.log(profile.unified_id, web.results);

const identity = createIdentityClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
console.log(await identity.profiles());

const embeddings = createEmbeddingsClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
console.log((await embeddings.embed("Planner memory", "local")).dimensions);

const search = createSearchClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
console.log(await search.search("云雀 Agent", { limit: 3 }));

const interactions = createInteractionsClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
await interactions.createInstruction({ category: "style", content: "回答保持自然、简洁。" });
await interactions.react({ channel_type: "telegram", target: "chat-1", message_id: "msg-1", emoji: "👍" });

const rbac = createRBACClient({
  baseUrl: "http://localhost:9090",
  token: "<admin-jwt>",
});
await rbac.assignRole({ subject_id: "user-1", role_id: "operator", tenant_id: "tenant-a" });
const permission = await rbac.check({ resource: "tasks", action: "write" });
console.log(permission.allowed);

const memorySearch = createMemorySearchClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const memories = await memorySearch.search("Planner 经验", { limit: 5, layer: "all" });
console.log(memories.results.length);

const memoryStats = createMemoryStatsClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
console.log((await memoryStats.stats()).long ?? 0);

const memoryAdd = createMemoryAddClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
await memoryAdd.remember("用户偏好简洁中文回复", { layer: "long", source: "chat" });

const memoryCompact = createMemoryCompactClient({
  baseUrl: "http://localhost:9090",
  token: "<admin-jwt>",
});
await memoryCompact.compact({ target_count: 100, decay_days: 30 });

const knowledgeSearch = createKnowledgeSearchClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const knowledgeHits = await knowledgeSearch.search("Planner 蓝图", { limit: 5, lang: "md" });
console.log(knowledgeHits.count);

const knowledgeIngest = createKnowledgeIngestClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
await knowledgeIngest.ingestText("Planner 需要先恢复上下文", { name: "planner-note.md", trigger: "chat" });

const knowledgeSources = createKnowledgeSourcesClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
console.log((await knowledgeSources.list()).sources.length);

const knowledgeImport = createKnowledgeImportClient({
  baseUrl: "http://localhost:9090",
  token: "<admin-jwt>",
});
await knowledgeImport.importUrlString("https://deepwiki.com/org/repo", { max_pages: 2 });

const knowledgeUpload = createKnowledgeUploadClient({
  baseUrl: "http://localhost:9090",
  token: "<admin-jwt>",
});
await knowledgeUpload.uploadFile(new Blob(["# doc"]), "doc.md");

const taskLifecycle = createTaskLifecycleClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await taskLifecycle.run("task_123");

const taskRead = createTaskReadClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
console.log((await taskRead.list()).length);

const taskCreate = createTaskCreateClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await taskCreate.createFromDescription("拆 SDK 增量包", { title: "SDK" });

const taskDelete = createTaskDeleteClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await taskDelete.delete("task_123");

const connectorCatalog = createConnectorCatalogClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const connectorList = await connectorCatalog.list();
console.log(connectorList.connectors.length);

const connectorAuth = createConnectorAuthClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
await connectorAuth.disconnect("github");

const connectorActions = createConnectorActionsClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
await connectorActions.execute({ connector_id: "github", action_id: "list_issues" });

const connectors = createConnectorsClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
await connectors.detail("github");

const notifyShare = createNotifyShareClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});

await notifyShare.send({ channel_id: "feishu-main", title: "任务完成", task_id: "task-1" });

const notifyChannels = createNotifyChannelsClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});

const channels = await notifyChannels.list();

const notify = createNotifyClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
await notify.share({ channel_id: "feishu-main", message: "任务已完成", session_id: "demo-session" });

const projectRead = createProjectReadClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});

const projectList = await projectRead.list();

const projectWrite = createProjectWriteClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
await projectWrite.update("project-1", { description: "updated by SDK" });

const projects = createProjectsClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const fullProjectList = await projects.list();
console.log(fullProjectList.projects.length);

const market = createSkillMarketClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const topSkills = await market.top({ n: 5, by: "rating" });
console.log(topSkills.skills.length);

const dispatch = createDispatchClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const workerConfig = await dispatch.workerConfig("cursor");
console.log(workerConfig.server_url);

const orchestrator = createOrchestratorClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const orchestratorStatus = await orchestrator.status();
console.log(orchestratorStatus.running);

const fork = createForkClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const branches = await fork.list("session-1");
console.log(branches.forks.length);

const scheduler = createSchedulerClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const scheduledJobs = await scheduler.jobs();
console.log(scheduledJobs.count);

const upload = createUploadClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const uploaded = await upload.file(new Blob(["hello"]), "note.txt");
console.log(uploaded.parse?.status);

const skills = createSkillsClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const runtimeSkills = await skills.list();
console.log(runtimeSkills.count);

const router = createRouterClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const routerStats = await router.stats();
console.log(routerStats.status ?? routerStats.stats);

const auth = createAuthClient({ baseUrl: "http://localhost:9090" });
const authStatus = await auth.status();
console.log(authStatus.password_set);

const system = createSystemClient({ baseUrl: "http://localhost:9090" });
const sbom = await system.sbom();
console.log(sbom.bomFormat);

const cognis = createCognisClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const cogniHealth = await cognis.health();
console.log(cogniHealth);

const experience = await cognis.experience("code-reviewer");
console.log(experience.summary?.top_tools?.[0]?.tool);
for (const pattern of experience.summary?.pending_patterns ?? []) {
  if (pattern.id) await cognis.confirmExperiencePattern("code-reviewer", pattern.id);
}

const events = createEventsClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
for await (const event of events.stream()) {
  console.log(event.event, event.data);
  break;
}

const realtime = createRealtimeClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const ws = realtime.connect();
ws.addEventListener("open", () => realtime.send(ws, realtime.ping()));

const airi = createAiriClient({ baseUrl: "http://localhost:9090" });
const airiModels = await airi.models();
console.log(airiModels.data[0]?.id);

const memory = createMemoryClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

await memory.add({
  layer: "long",
  content: "用户希望回答更简洁",
  source: "demo-shell",
});

const tasks = createTasksClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const task = await tasks.create({
  title: "整理恢复现场",
  description: "读取最近 Planner checkpoint 并给出下一步建议",
  constraints: { max_steps: 6, risk_level: "low" },
});
await tasks.run(task.id);

const taskObserve = createTaskObserveClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const gaps = await taskObserve.gaps("skill_missing");
const memoryForTask = await taskObserve.workingMemory(task.id);

const taskTemplates = createTaskTemplatesClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
await taskTemplates.list();

const taskThreads = createTaskThreadsClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
await taskThreads.postMessage(task.id, "请继续，但保持低风险。", {
  channel_type: "feishu",
  channel_id: "demo-chat",
});

const taskContext = createTaskContextClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
await taskContext.resolveGap("gap_123");
console.log(gaps.length, memoryForTask.next_action);

const knowledge = createKnowledgeClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

await knowledge.ingest({
  name: "technical-blueprint.md",
  content: "Planner 恢复、任务编排、记忆与知识库是外部壳的最小闭环。",
});
const matches = await knowledge.search({ query: "Planner 恢复", limit: 5 });
console.log(matches.chunks);

const providerRegistry = createProviderRegistryClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});

const presets = await providerRegistry.presets();

const providerControl = createProviderControlClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});

await providerControl.switchModel("ollama", "qwen3.5:4b");

const providerHealth = createProviderHealthClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});

const providerStatus = await providerHealth.list();

const providers = createProvidersClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

await providers.registerProvider({
  preset_id: "deepseek",
  api_key: "<provider-key>",
  model: "deepseek-chat",
});
await providers.testProvider("deepseek-deepseek-chat");
await providers.setExecProvider("deepseek-deepseek-chat");

const breaker = createBreakerClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
await breaker.reset();

const models = createModelsClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const availableModels = await models.listModels();
console.log(availableModels.models.map((model) => model.model_id));

const setupDetect = createSetupDetectClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
await setupDetect.health();

const setupTemplates = createSetupTemplatesClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const templates = await setupTemplates.list();

const setupProvider = createSetupProviderClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

await setupProvider.test({
  base_url: "https://api.deepseek.com/v1",
  api_key: "<provider-key>",
  model: "deepseek-chat",
});
await setupProvider.apply({
  template_id: templates.templates[0]?.id ?? "hybrid",
  base_url: "https://api.deepseek.com/v1",
  api_key: "<provider-key>",
  model: "deepseek-chat",
});

const setupInstall = createSetupInstallClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
await setupInstall.install("python_office");

const documents = createDocumentsClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

await documents.generateDocx({
  title: "技术蓝图摘要",
  content: "# 云雀技术蓝图摘要\n\nPlanner、任务、记忆与知识库已经拆成增量 SDK。",
});

const approvals = createApprovalsClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const pending = await approvals.pending();
if (pending.approvals[0]) {
  await approvals.decide(pending.approvals[0].id, "allow_once");
}

const approvalQueue = createApprovalQueueClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const queue = await approvalQueue.pending();
if (queue.approvals[0]) {
  await approvalQueue.approve(queue.approvals[0].id);
}

const approvalRules = createApprovalRulesClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

await approvalRules.add({ action: "shell", pattern: "npm test", decision: "allow_once" });

const traceEvents = createTraceEventsClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});

const recentTraceEvents = await traceEvents.recent({ limit: 20 });

const taskTrace = createTaskTraceClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});

const taskEvents = await taskTrace.get("task-123", { raw: true });

const trace = createTraceClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const recentEvents = await trace.recent({ limit: 20 });
console.log(recentEvents.events);

const browserStatusClient = createBrowserStatusClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const browserHealth = await browserStatusClient.status();

const browserCapture = createBrowserCaptureClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const browserOPP = createBrowserOPPClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const pendingBrowserApprovals = await browserOPP.pending();

const browserExtension = createBrowserExtensionClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
await browserExtension.session();

const browser = createBrowserClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

if (browserHealth.connected) {
  await browser.navigate("https://example.com");
  const pageText = await browserCapture.ocr();
  console.log(pageText.text);
}

const runtimeQueue = createRuntimeQueueClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const queues = await runtimeQueue.overview();
console.log(queues.queues);

const runtimeEvents = createRuntimeEventsClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

for await (const event of runtimeEvents.events()) {
  console.log(event.type, event.data);
}

const runtime = createRuntimeClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const modes = createModesClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const currentMode = await modes.current({ session_id: "demo-session" });
console.log(currentMode.mode);

const ide = createIDEClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const ideStatus = await ide.status();
if (ideStatus.connected) {
  await ide.reviewDiff({
    file_path: "src/app.ts",
    language: "ts",
    diff: "+console.log('hello')",
  });
}

const persona = createPersonaClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});

const currentPersona = await persona.get();
await persona.addSkill({
  name: "review-style",
  description: "Review tone and output preference",
  content: "Prefer concise, evidence-first review comments.",
});
console.log(currentPersona.identity);

const workflowDefinitions = createWorkflowDefinitionsClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});

const definitions = await workflowDefinitions.list();

const workflowRuns = createWorkflowRunsClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});

const run = await workflowRuns.runDefinition("workflow-id", { topic: "sdk" });

const workflows = createWorkflowClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const saved = await workflows.save({
  name: "daily-review",
  nodes: [{ id: "review", name: "Review", type: "llm", position: { x: 0, y: 0 } }],
  edges: [],
});
await workflows.run({ definition_id: saved.id!, variables: { topic: "sdk" } });

const costs = createCostClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
console.log(await costs.summary());
await costs.setQuota({ quota: { max_chat_calls: 100, max_tokens_per_day: 200000 } });

const usage = createUsageClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
console.log(await usage.usage());

const lora = createLoRAClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const preview = await lora.preview({ tenant_id: "default" });
if (preview.preview.ready) {
  await lora.trigger({ tenant_id: "default" });
}

const iterate = createIterateClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});

const pending = await iterate.pendingProposals();
if (pending.proposals[0]) {
  await iterate.approve({ id: pending.proposals[0].id });
}

const trust = createTrustClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});

const reviewGate = await trust.reviewStatus();
if (reviewGate.trust_enabled) {
  console.log(await trust.scores());
}

const audit = createAuditClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
const integrity = await audit.verify();
console.log(integrity.valid);

const heartbeat = createHeartbeatClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await heartbeat.update({ enabled: true, interval_minutes: 30 });

const reverie = createReverieClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await reverie.think({ event_type: "task_completed", trigger: "sdk-demo" });

const federation = createFederationClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
const matches = await federation.discover({ feature: "browser", intent: "open page" });
console.log(matches.count);

const system = createSystemClient({ baseUrl: "http://localhost:9090" });
const readiness = await system.readyz();
console.log(readiness.status);

const settings = createSettingsClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
const backup = await settings.backupInfo();
console.log(backup.file_count);

const tori = createToriClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
const toriStatus = await tori.status();
console.log(toriStatus.bound);

const speech = createSpeechClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
const voices = await speech.voices();
console.log(voices.providers);

const admin = createAdminClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
const tenants = await admin.listTenants();
console.log(tenants.count);

const files = createFilesClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
const artifacts = await files.list();
console.log(artifacts.files.length);

const cron = createCronClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
const jobs = await cron.list();
console.log(jobs.jobs.length);

const skillhubCatalog = createSkillHubCatalogClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
const skills = await skillhubCatalog.search({ q: "browser", limit: 10 });
console.log(skills.count);

const skillhubInstall = createSkillHubInstallClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
const installedSkills = await skillhubInstall.installed();
console.log(installedSkills.count);

const skillhubPolicy = createSkillHubPolicyClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await skillhubPolicy.policy();

const skillhub = createSkillHubClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await skillhub.analytics();

const pluginCatalog = createPluginCatalogClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
const pluginList = await pluginCatalog.list();
console.log(pluginList.plugins.length);

const pluginControl = createPluginControlClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await pluginControl.reload();

const pluginFiles = createPluginFilesClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await pluginFiles.files("demo");

const pluginCrud = createPluginCrudClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await pluginCrud.create({ name: "demo", template: "basic" });

const plugins = createPluginsClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await plugins.ui();

const graph = createGraphClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
const graphStats = await graph.stats();
console.log(graphStats.entities);

const pluginLLM = createPluginLLMClient({
  baseUrl: "http://localhost:9090",
  token: "<plugin-token>",
});
const pluginReply = await pluginLLM.complete({
  messages: [{ role: "user", content: "Summarize current task" }],
});
console.log(pluginReply.reply);

const pluginSearch = createPluginSearchClient({
  baseUrl: "http://localhost:9090",
  token: "<plugin-token>",
});
const pluginSearchResults = await pluginSearch.search("task context", 5);
console.log(pluginSearchResults.results);

const pluginMemory = createPluginMemoryClient({
  baseUrl: "http://localhost:9090",
  token: "<plugin-token>",
});
await pluginMemory.set("last-summary", pluginReply.reply);
const pluginMemoryValue = await pluginMemory.get("last-summary");
console.log(pluginMemoryValue.value);

const resourceState = createResourceStateClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});

const resources = await resourceState.list();

const focusState = createFocusStateClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});

await focusState.update("planner", ["sdk"]);

const goalState = createGoalStateClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});

const goals = await goalState.list();

const state = createStateClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
const focus = await state.focus();
console.log(focus.focus);

const triggers = createTriggersClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await triggers.emit({ event: "task_completed", text: "SDK slice finished" });

const missions = createMissionsClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
const mission = await missions.parse("每天早上总结昨天的任务");
console.log(mission.type);

const reflect = createReflectClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
const reflectExperiences = await reflect.experiences({
  q: "code review",
  source: "task",
  outcome: "partial",
  tag: "quality:9",
  limit: 10,
});
console.log(reflectExperiences.experiences[0]?.lesson);
const reflectStats = await reflect.experienceStats({ source: "task", tag: "quality:9" });
console.log(reflectStats.by_outcome?.success ?? 0);
const strategyContext = await reflect.strategies({ source: "task", tag: "quality:9", limit: 5 });
console.log(strategyContext.strategies.split("\n")[0]);

const tools = createToolsClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
const sessions = await tools.list();
console.log(sessions.sessions.length);

const sandbox = createSandboxClient({
  baseUrl: "http://localhost:9090",
  token: "<admin-jwt>",
});
const sandboxStatus = await sandbox.probe();
console.log(sandboxStatus.key_source);
```

This keeps the SDK usable as an **incremental package**: embedder code can bring
in only `auth`, `airi`, `planner-recovery`, `planner`, `chat`, `cognis`, `cognis-registry`, `cognis-observe`, `cognis-experience`, `cognis-evolution`, `cognis-federation`, `cognis-workflows`, `cognis-bundles`, `events`, `realtime`, `webchat`, `conversations`, `subagents`, `bots`, `discovery`, `identity`, `embeddings`, `search`, `interactions`, `emotion`, `reactions`, `instructions`, `rbac`, `roles`, `permissions`, `memory`, `memory-search`, `memory-stats`, `memory-add`, `memory-compact`, `tasks`, `task-context`, `task-observe`, `task-templates`, `task-threads`, `task-lifecycle`, `task-read`, `task-create`, `task-delete`, `knowledge`, `knowledge-search`, `knowledge-ingest`, `knowledge-sources`, `knowledge-import`, `knowledge-upload`, or
`providers`/`provider-control`/`provider-health`/`provider-registry`/`breaker`/`models`/`setup`/`setup-detect`/`setup-templates`/`setup-provider`/`setup-install`/`documents`/`approvals`/`approval-queue`/`approval-rules`/`trace`/`trace-events`/`task-trace`/`browser`/`browser-status`/`browser-capture`/`browser-opp`/`browser-extension`/`runtime`/`runtime-queue`/`runtime-events`/`router`/`modes`
`/ide`/`persona`/`workflow`/`workflow-definitions`/`workflow-runs`/`cost`/`cost-budget`/`cost-observe`/`usage`/`lora`/`lora-observe`/`lora-control`/`iterate`/`iterate-review`/`trust`/`review`/`skillgrow`/`audit`/`heartbeat`
`/reverie`/`federation`/`system`/`settings`/`tori`/`speech`/`upload`/`admin`/`files`/`cron`/`skillhub`/`skills`/`plugins`/`connectors`/`notify`/`projects`/`market`/`dispatch`/`orchestrator`/`fork`/`scheduler`/`graph`/`plugin-api`/`plugin-llm`/`plugin-search`/`plugin-memory`/`plugin-agent-memory`/`plugin-knowledge`/`plugin-cron`/`plugin-send`/`plugin-extensions`/`state`/`triggers`/`missions`/`reflect`/`tools`/`sandbox` without importing the generated 500KB+ SDK/types bundle. Add future
slices in the same style when those surfaces need stable, lightweight
integration APIs.

## Regenerating

After spec changes:

```bash
# 1. Refresh OpenAPI from gateway routes
cd ../..        # back to repo root
make openapi

# 2. Regenerate this SDK
cd sdk/typescript
npm run generate
npm run typecheck           # should be silent (0 errors)
npm run check:incremental   # verifies hand-written slice exports/tests/route coverage
```

## Layout

| File / dir | Purpose |
|---|---|
| `src/sdk.gen.ts` | Per-endpoint typed functions (~263 KB) |
| `src/types.gen.ts` | All schemas, request/response types (~295 KB) |
| `src/client.gen.ts` | Default client instance |
| `src/client/` | Fetch runtime (from `@hey-api/client-fetch`) |
| `src/core/` | Internal helpers |
| `src/auth.ts` | Lightweight hand-written setup status, password login/setup, Tori OAuth URL, and API-key to JWT exchange slice |
| `src/airi.ts` | Lightweight hand-written Airi bridge status, OpenAI-compatible models, and chat completions slice |
| `src/planner-recovery.ts` | Lightweight hand-written Planner recovery slice for incremental imports |
| `src/planner.ts` | Lightweight planner facade over checkpoint recovery and execution state |
| `src/chat.ts` | Lightweight hand-written Chat/SSE slice for incremental imports |
| `src/cognis.ts` | Lightweight hand-written Cogni registry, health, traces, workflow, experience, evolution, and federation control slice |
| `src/cognis-registry.ts` | Lightweight Cogni registry list/create/get/remove/enable/disable/reload facade without traces or evolution APIs |
| `src/cognis-observe.ts` | Lightweight Cogni traces/stats/health/verify/alerts facade without registry mutation or evolution APIs |
| `src/cognis-experience.ts` | Lightweight Cogni experience read/record/confirm facade without registry, trace or evolution APIs |
| `src/cognis-evolution.ts` | Lightweight Cogni evolve/status facade without registry, trace, experience or federation APIs |
| `src/cognis-federation.ts` | Lightweight Cogni federation status/peers/discover/expose/economics facade without registry, trace, experience or workflow APIs |
| `src/cognis-workflows.ts` | Lightweight Cogni workflows list/run facade without registry, trace, experience, evolution or federation APIs |
| `src/cognis-bundles.ts` | Lightweight Cogni bundle generate/export/import facade without registry, trace, experience, evolution, workflow or federation APIs |
| `src/events.ts` | Lightweight hand-written SSE event stream slice for task/workflow/approval live updates |
| `src/realtime.ts` | Lightweight hand-written `/v1/ws` URL, connect, ping/chat message helper slice |
| `src/webchat.ts` | Lightweight hand-written embeddable WebChat widget script/snippet slice |
| `src/conversations.ts` | Lightweight hand-written conversation history, management, and replay slice |
| `src/subagents.ts` | Lightweight hand-written subagent list/spawn/message/destroy slice |
| `src/bots.ts` | Lightweight hand-written bots, inbox, and channel group operations slice |
| `src/discovery.ts` | Lightweight hand-written identity, embeddings, and web search discovery slice |
| `src/identity.ts` | Lightweight identity resolve/profile facade for `/v1/identity/*` without full SDK import |
| `src/embeddings.ts` | Lightweight embeddings providers/embed facade for `/v1/embeddings` without full SDK import |
| `src/search.ts` | Lightweight search facade for `/v1/search` and `/v1/search/providers` without full SDK import |
| `src/interactions.ts` | Lightweight hand-written emotion history, stickers, instructions, reactions, and sticker sending slice |
| `src/emotion.ts` | Lightweight emotion history/sticker mapping facade for `/v1/emotion/*` without full SDK import |
| `src/reactions.ts` | Lightweight reaction/sticker sending facade for `/v1/react` and `/v1/sticker/send` without full SDK import |
| `src/instructions.ts` | Lightweight user-instructions facade for `/v1/instructions*` without full SDK import |
| `src/rbac.ts` | Lightweight hand-written RBAC roles, assignments, and permission-check slice |
| `src/roles.ts` | Lightweight role/assignment facade over RBAC role endpoints without full SDK import |
| `src/permissions.ts` | Lightweight permission check/current roles facade over RBAC without full SDK import |
| `src/memory.ts` | Lightweight hand-written Memory stats/search/add/compact slice |
| `src/memory-search.ts` | Lightweight memory search-only facade over Memory without full SDK import |
| `src/memory-stats.ts` | Lightweight memory stats-only facade over Memory without full SDK import |
| `src/memory-add.ts` | Lightweight memory write-only facade over Memory without full SDK import |
| `src/memory-compact.ts` | Lightweight memory compaction facade over Memory without full SDK import |
| `src/tasks.ts` | Lightweight hand-written Task create/list/lifecycle slice |
| `src/task-context.ts` | Lightweight hand-written Task gaps, working memory, templates, and thread context slice |
| `src/task-observe.ts` | Lightweight task gaps/stats/working-memory observe facade without full SDK import |
| `src/task-templates.ts` | Lightweight task template list/get/create/delete/instantiate facade without full SDK import |
| `src/task-threads.ts` | Lightweight task thread list/get/post-message/update-state facade without full SDK import |
| `src/task-lifecycle.ts` | Lightweight task run/pause/resume/restart/cancel facade without full SDK import |
| `src/task-read.ts` | Lightweight task list/detail read-only facade without full SDK import |
| `src/task-create.ts` | Lightweight task creation facade without full SDK import |
| `src/task-delete.ts` | Lightweight task deletion facade without full SDK import |
| `src/knowledge.ts` | Lightweight hand-written Knowledge search/ingest/import/upload slice |
| `src/knowledge-search.ts` | Lightweight knowledge search-only facade without full SDK import |
| `src/knowledge-ingest.ts` | Lightweight inline knowledge ingestion facade without full SDK import |
| `src/knowledge-sources.ts` | Lightweight knowledge source stats/list/update/delete facade without full SDK import |
| `src/knowledge-import.ts` | Lightweight URL/repo knowledge import facade without full SDK import |
| `src/knowledge-upload.ts` | Lightweight knowledge file upload facade without full SDK import |
| `src/providers.ts` | Lightweight hand-written LLM provider/model configuration slice |
| `src/provider-registry.ts` | Lightweight provider preset, registration and discovery facade without full SDK import |
| `src/provider-control.ts` | Lightweight provider lifecycle and runtime selection facade without full SDK import |
| `src/provider-health.ts` | Lightweight provider status, mode and connectivity facade without full SDK import |
| `src/breaker.ts` | Lightweight provider breaker reset facade for `/api/breaker/reset` without full SDK import |
| `src/models.ts` | Lightweight models facade for listing and maintaining `/v1/models` without full SDK import |
| `src/setup.ts` | Lightweight hand-written first-run setup/configuration wizard slice |
| `src/setup-detect.ts` | Lightweight setup detect/health facade without setup write or install APIs |
| `src/setup-templates.ts` | Lightweight setup template catalog facade without setup write or install APIs |
| `src/setup-provider.ts` | Lightweight setup provider test/apply facade without detect, template catalog or install APIs |
| `src/setup-install.ts` | Lightweight setup component install and SSE progress facade without detect, templates or provider apply APIs |
| `src/documents.ts` | Lightweight hand-written DOCX/XLSX/PPTX/HTML generation slice |
| `src/approvals.ts` | Lightweight hand-written human-in-the-loop approval queue/rules slice |
| `src/approval-queue.ts` | Lightweight approval queue and decision facade without full SDK import |
| `src/approval-rules.ts` | Lightweight approval rule management facade without full SDK import |
| `src/trace.ts` | Lightweight hand-written execution/audit trace inspection slice |
| `src/trace-events.ts` | Lightweight trace recent/by-trace-id facade without full SDK import |
| `src/task-trace.ts` | Lightweight task trace read facade without full SDK import |
| `src/browser.ts` | Lightweight hand-written browser extension automation and OPP slice |
| `src/browser-status.ts` | Lightweight browser status/config/extension-status facade without automation or screenshot APIs |
| `src/browser-capture.ts` | Lightweight browser screenshot/latest-screenshot/OCR facade without navigation or extension action APIs |
| `src/browser-opp.ts` | Lightweight browser OPP pending/decision facade without navigation, capture or extension action APIs |
| `src/browser-extension.ts` | Lightweight browser extension session/action/scenario facade without status, capture or OPP APIs |
| `src/runtime.ts` | Lightweight hand-written session queue and events stream slice |
| `src/runtime-queue.ts` | Lightweight runtime queue overview/session/cancel facade without event stream APIs |
| `src/runtime-events.ts` | Lightweight runtime SSE event stream facade without session queue APIs |
| `src/router.ts` | Lightweight hand-written smart-router stats and status slice |
| `src/modes.ts` | Lightweight hand-written persona mode listing/switching slice |
| `src/ide.ts` | Lightweight hand-written IDE status/code-review slice |
| `src/persona.ts` | Lightweight hand-written persona identity/skills/presets slice |
| `src/workflow.ts` | Lightweight hand-written workflow definition/instance execution slice |
| `src/workflow-definitions.ts` | Lightweight workflow definition management facade without full SDK import |
| `src/workflow-runs.ts` | Lightweight workflow run and instance facade without full SDK import |
| `src/cost.ts` | Lightweight hand-written cost, usage and quota slice |
| `src/cost-budget.ts` | Lightweight cost summary/budget/alerts facade without task cost, usage or quota APIs |
| `src/cost-observe.ts` | Lightweight task cost/timeline/breakdown/history facade without budget, usage or quota APIs |
| `src/usage.ts` | Lightweight usage/quota facade for `/v1/usage` and `/v1/quota` without full SDK import |
| `src/lora.ts` | Lightweight hand-written LoRA training and evolution lifecycle slice |
| `src/lora-observe.ts` | Lightweight LoRA status/history/summary/preview/evolution facade without training or config mutation APIs |
| `src/lora-control.ts` | Lightweight LoRA trigger/rollback/config facade without status/history/summary/evolution APIs |
| `src/iterate.ts` | Lightweight hand-written self-iteration proposal approval slice |
| `src/iterate-review.ts` | Lightweight self-iteration proposal list/approve/reject facade without cycle trigger/status APIs |
| `src/trust.ts` | Lightweight hand-written trust, review-gate and skill-growth slice |
| `src/review.ts` | Lightweight review-gate status facade for `/api/review/status` without full SDK import |
| `src/skillgrow.ts` | Lightweight skill-growth pattern facade for `/api/skillgrow/patterns` without full SDK import |
| `src/audit.ts` | Lightweight hand-written audit chain and audit trail inspection slice |
| `src/heartbeat.ts` | Lightweight hand-written proactive heartbeat lifecycle slice |
| `src/reverie.ts` | Lightweight hand-written inner monologue and proactive thought slice |
| `src/federation.ts` | Lightweight hand-written federation peers, capabilities, discovery, delegation, and broadcast slice |
| `src/system.ts` | Lightweight hand-written health, version, SBOM, metrics, cache, and module observability slice |
| `src/settings.ts` | Lightweight hand-written settings, config reload, directory detection, and backup/restore slice |
| `src/tori.ts` | Lightweight hand-written Tori OAuth binding, status, health, and usage slice |
| `src/speech.ts` | Lightweight hand-written speech TTS/STT, STT stream URL, voices, and file upload slice |
| `src/admin.ts` | Lightweight hand-written desktop controls, tenants, and natural-language config slice |
| `src/files.ts` | Lightweight hand-written artifact file listing, preview, and download slice |
| `src/cron.ts` | Lightweight hand-written cron job scheduling and run-now slice |
| `src/skillhub.ts` | Lightweight hand-written SkillHub search/install/update/policy slice |
| `src/skillhub-catalog.ts` | Lightweight SkillHub search/trending/detail facade without full SDK import |
| `src/skillhub-install.ts` | Lightweight SkillHub install/update/rollback lifecycle facade without full SDK import |
| `src/skillhub-policy.ts` | Lightweight SkillHub policy/check/analytics facade without full SDK import |
| `src/skills.ts` | Lightweight hand-written runtime skills catalog, scan, dynamic review, and suggestions slice |
| `src/plugins.ts` | Lightweight hand-written plugin CRUD, files, UI tabs, reload, and folder-open slice |
| `src/plugin-catalog.ts` | Lightweight plugin list/status catalog facade without full SDK import |
| `src/plugin-control.ts` | Lightweight plugin toggle/ui/reload/open-folder facade without full SDK import |
| `src/plugin-files.ts` | Lightweight plugin file read/save facade without full SDK import |
| `src/plugin-crud.ts` | Lightweight plugin create/delete facade without full SDK import |
| `src/connectors.ts` | Lightweight hand-written connector catalog, auth, and action execution slice |
| `src/connector-catalog.ts` | Lightweight connector list/detail catalog facade without full SDK import |
| `src/connector-auth.ts` | Lightweight connector connect/disconnect facade without full SDK import |
| `src/connector-actions.ts` | Lightweight connector action execution facade without full SDK import |
| `src/notify.ts` | Lightweight hand-written notification channels, test, and share dispatch slice |
| `src/notify-share.ts` | Lightweight notification share dispatch facade without full SDK import |
| `src/notify-channels.ts` | Lightweight notification channel management facade without full SDK import |
| `src/projects.ts` | Lightweight hand-written project workspace CRUD slice |
| `src/project-read.ts` | Lightweight project list/detail read facade without full SDK import |
| `src/project-write.ts` | Lightweight project create/update/remove facade without full SDK import |
| `src/market.ts` | Lightweight hand-written skill marketplace search, ranking, and stats slice |
| `src/dispatch.ts` | Lightweight hand-written MCP dispatch worker, queue, and config slice |
| `src/orchestrator.ts` | Lightweight hand-written IDE worker orchestrator daemon, session, event, and policy slice |
| `src/fork.ts` | Lightweight hand-written conversation fork root, branch, list, and delete slice |
| `src/scheduler.ts` | Lightweight hand-written prompt scheduler job list/add/remove slice |
| `src/upload.ts` | Lightweight hand-written authenticated multipart upload and parsed-file metadata slice |
| `src/graph.ts` | Lightweight hand-written knowledge graph entity/relation/context/stats slice |
| `src/plugin-api.ts` | Lightweight hand-written plugin runtime LLM/search/memory/knowledge/cron/extensions bridge slice |
| `src/plugin-llm.ts` | Lightweight plugin LLM completion facade without plugin search, memory, knowledge, cron or extension APIs |
| `src/plugin-search.ts` | Lightweight plugin runtime search facade without LLM, memory, knowledge, cron or extension APIs |
| `src/plugin-memory.ts` | Lightweight plugin KV memory facade without LLM, search, agent memory, knowledge, cron or extension APIs |
| `src/plugin-agent-memory.ts` | Lightweight plugin access to host Agent memory search/add without plugin KV, knowledge, cron or extension APIs |
| `src/plugin-knowledge.ts` | Lightweight plugin knowledge search/ingest facade without LLM, memory or cron APIs |
| `src/plugin-cron.ts` | Lightweight plugin cron add/remove/list facade without LLM, memory, knowledge or extension APIs |
| `src/plugin-send.ts` | Lightweight plugin message send facade without LLM, memory, knowledge, cron or extension APIs |
| `src/plugin-extensions.ts` | Lightweight plugin extension register/list facade without LLM, memory, knowledge or cron APIs |
| `src/state.ts` | Lightweight hand-written state kernel snapshot, goals, focus, and resources slice |
| `src/resource-state.ts` | Lightweight state resource list/track/release facade without full SDK import |
| `src/focus-state.ts` | Lightweight state focus read/update facade without full SDK import |
| `src/goal-state.ts` | Lightweight state goal list/save/delete facade without full SDK import |
| `src/triggers.ts` | Lightweight hand-written legacy and v2 trigger CRUD, emit, runs, and events slice |
| `src/missions.ts` | Lightweight hand-written mission parsing and reflection experiences/strategies slice |
| `src/reflect.ts` | Lightweight reflect-only facade over experiences, stats, and strategy context |
| `src/tools.ts` | Lightweight hand-written guarded process execution sessions list/poll/kill slice |
| `src/sandbox.ts` | Lightweight hand-written sandbox exec, cloud probe, and desktop lifecycle slice |
| `openapi-ts.config.ts` | Generator configuration |
| `tsconfig.json` | TypeScript compiler config (`DOM.Iterable` required for `Headers.entries`) |

## Status

- 343 endpoints, ~22000 LOC, 100+ schemas
- Hand-curated `cognis` operationIds yield idiomatic names (`postV1CognisGenerate` etc.)
- Auto-generated names follow `<method><PathPascalCase>` pattern
- Streaming (`getV1ChatStream`, `getV1EventsStream`) is stubbed in the generated SDK; use the hand-written `yunque-client/chat`, `yunque-client/events`, and `yunque-client/realtime` slices when you need parsed fetch streams or `/v1/ws` helpers without importing the full bundle.
- Request/response bodies are mostly `unknown` placeholders since the source
  spec is path-only. Hand-edit `docs/openapi.yaml` to add real schemas, then
  regenerate.

## Caveats

- `npm run check:pack` guards the published package shape: no test files,
  required incremental subpath files present, no scripts/temp logs, and bounded
  package size.
- Client uses ESM (`"type": "module"` in package.json). For CommonJS consumers,
  rebuild with a different tsconfig (`"module": "CommonJS"`).


