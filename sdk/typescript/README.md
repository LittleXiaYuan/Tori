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

For app code, prefer subpath imports such as `yunque-client/chat`,
`yunque-client/planner-recovery`, or `yunque-client/agent-kit`. The package root (`yunque-client`) re-exports
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

### Agent Kit bundle

For external automation that needs the common SDK-first surfaces together, use
`yunque-client/agent-kit`. It composes the hand-written State Kernel,
Reflection Experience, Mission Parse, Scheduler, Cron System, Triggers, Memory Kernel, Knowledge Graph, Knowledge Base, LoRA, Workflow, Connector, Notify, Cost, Providers, Cognis, Trace, Heartbeat, Events, Reverie, Tori, Speech, Setup, and Plugin API Runtime clients without importing the
generated all-in-one SDK.

```ts
import { createAgentKit } from "yunque-client/agent-kit";

const kit = createAgentKit({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
  pluginToken: "<plugin-token>",
});

const focus = await kit.state.focus();
const strategies = await kit.reflect.strategies({ tag: "sdk", limit: 5 });
const found = await kit.memory.search({ query: "incremental SDK package", limit: 3 });
const graphStats = await kit.graph.stats();
const kbStats = await kit.knowledge.stats();
const connectors = await kit.connectors.list();
const channels = await kit.notify.channels();
const search = await kit.plugin.search("incremental SDK package", 5);

console.log(focus.focus, strategies.strategies, found.count, graphStats.entities, kbStats.sources, connectors.connectors.length, channels.channels.length, search.results.length);
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
import { createAgentKit } from "yunque-client/agent-kit";
import { createAuthClient } from "yunque-client/auth";
import { createAiriClient } from "yunque-client/airi";
import { createPlannerRecoveryClient } from "yunque-client/planner-recovery";
import { createPlannerClient } from "yunque-client/planner";
import { createPlannerReadClient } from "yunque-client/planner-read";
import { createPlannerControlClient } from "yunque-client/planner-control";
import { createPlannerCheckpointsClient } from "yunque-client/planner-checkpoints";
import { createPlannerResumeClient } from "yunque-client/planner-resume";
import { createPlannerExecutionStateClient } from "yunque-client/planner-execution-state";
import { createChatClient } from "yunque-client/chat";
import { createChatBasicClient } from "yunque-client/chat-basic";
import { createChatAgenticClient } from "yunque-client/chat-agentic";
import { createChatStreamClient } from "yunque-client/chat-stream";
import { createCognisClient } from "yunque-client/cognis";
import { createCognisRegistryClient } from "yunque-client/cognis-registry";
import { createCognisObserveClient } from "yunque-client/cognis-observe";
import { createCognisTracesClient } from "yunque-client/cognis-traces";
import { createCognisHealthClient } from "yunque-client/cognis-health";
import { createCognisAlertsClient } from "yunque-client/cognis-alerts";
import { createCognisExperienceClient } from "yunque-client/cognis-experience";
import { createCognisEvolutionClient } from "yunque-client/cognis-evolution";
import { createCognisFederationClient } from "yunque-client/cognis-federation";
import { createCognisWorkflowsClient } from "yunque-client/cognis-workflows";
import { createCognisBundlesClient } from "yunque-client/cognis-bundles";
import { createEventsClient } from "yunque-client/events";
import { createEventsStreamClient } from "yunque-client/events-stream";
import { createEventsParseClient } from "yunque-client/events-parse";
import { createRealtimeClient } from "yunque-client/realtime";
import { createRealtimeConnectClient } from "yunque-client/realtime-connect";
import { createRealtimeMessagesClient } from "yunque-client/realtime-messages";
import { createWebChatClient } from "yunque-client/webchat";
import { createWebChatWidgetClient } from "yunque-client/webchat-widget";
import { createWebChatEmbedClient } from "yunque-client/webchat-embed";
import { createConversationsClient } from "yunque-client/conversations";
import { createConversationsReadClient } from "yunque-client/conversations-read";
import { createConversationsControlClient } from "yunque-client/conversations-control";
import { createConversationSessionsClient } from "yunque-client/conversation-sessions";
import { createConversationMessagesClient } from "yunque-client/conversation-messages";
import { createConversationReplayClient } from "yunque-client/conversation-replay";
import { createConversationMessageControlClient } from "yunque-client/conversation-message-control";
import { createConversationManageClient } from "yunque-client/conversation-manage";
import { createSubagentsClient } from "yunque-client/subagents";
import { createSubagentsReadClient } from "yunque-client/subagents-read";
import { createSubagentsControlClient } from "yunque-client/subagents-control";
import { createBotsClient } from "yunque-client/bots";
import { createBotsReadClient } from "yunque-client/bots-read";
import { createBotsControlClient } from "yunque-client/bots-control";
import { createBotsListClient } from "yunque-client/bots-list";
import { createBotsDetailClient } from "yunque-client/bots-detail";
import { createBotsInboxClient } from "yunque-client/bots-inbox";
import { createBotsChannelsClient } from "yunque-client/bots-channels";
import { createDiscoveryClient } from "yunque-client/discovery";
import { createDiscoveryIdentityClient } from "yunque-client/discovery-identity";
import { createDiscoveryEmbeddingsClient } from "yunque-client/discovery-embeddings";
import { createDiscoverySearchClient } from "yunque-client/discovery-search";
import { createIdentityClient } from "yunque-client/identity";
import { createEmbeddingsClient } from "yunque-client/embeddings";
import { createSearchClient } from "yunque-client/search";
import { createInteractionsClient } from "yunque-client/interactions";
import { createEmotionClient } from "yunque-client/emotion";
import { createEmotionHistoryClient } from "yunque-client/emotion-history";
import { createEmotionStickersClient } from "yunque-client/emotion-stickers";
import { createReactionsClient } from "yunque-client/reactions";
import { createInstructionsClient } from "yunque-client/instructions";
import { createRBACClient } from "yunque-client/rbac";
import { createRolesClient } from "yunque-client/roles";
import { createRoleBindingsClient } from "yunque-client/role-bindings";
import { createMyRolesClient } from "yunque-client/my-roles";
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
import { createTaskGapsClient } from "yunque-client/task-gaps";
import { createTaskMemoryClient } from "yunque-client/task-memory";
import { createTaskThreadReadClient } from "yunque-client/task-thread-read";
import { createTaskThreadControlClient } from "yunque-client/task-thread-control";
import { createTaskLifecycleClient } from "yunque-client/task-lifecycle";
import { createTaskReadClient } from "yunque-client/task-read";
import { createTaskCreateClient } from "yunque-client/task-create";
import { createTaskDeleteClient } from "yunque-client/task-delete";
import { createKnowledgeClient } from "yunque-client/knowledge";
import { createKnowledgeSearchClient } from "yunque-client/knowledge-search";
import { createKnowledgeIngestClient } from "yunque-client/knowledge-ingest";
import { createKnowledgeSourcesClient } from "yunque-client/knowledge-sources";
import { createKnowledgeSourceReadClient } from "yunque-client/knowledge-source-read";
import { createKnowledgeSourceControlClient } from "yunque-client/knowledge-source-control";
import { createKnowledgeImportClient } from "yunque-client/knowledge-import";
import { createKnowledgeUploadClient } from "yunque-client/knowledge-upload";
import { createProvidersClient } from "yunque-client/providers";
import { createProviderRegistryClient } from "yunque-client/provider-registry";
import { createProviderControlClient } from "yunque-client/provider-control";
import { createProviderModeClient } from "yunque-client/provider-mode";
import { createProviderSessionClient } from "yunque-client/provider-session";
import { createProviderHealthClient } from "yunque-client/provider-health";
import { createBreakerClient } from "yunque-client/breaker";
import { createProviderBreakerClient } from "yunque-client/provider-breaker";
import { createModelsClient } from "yunque-client/models";
import { createSetupClient } from "yunque-client/setup";
import { createSetupDetectClient } from "yunque-client/setup-detect";
import { createSetupTemplatesClient } from "yunque-client/setup-templates";
import { createSetupProviderClient } from "yunque-client/setup-provider";
import { createSetupInstallClient } from "yunque-client/setup-install";
import { createDocumentsClient } from "yunque-client/documents";
import { createDocumentTemplatesClient } from "yunque-client/document-templates";
import { createDocumentGenerateClient } from "yunque-client/document-generate";
import { createDocumentDocxClient } from "yunque-client/document-docx";
import { createDocumentXlsxClient } from "yunque-client/document-xlsx";
import { createDocumentPptxClient } from "yunque-client/document-pptx";
import { createDocumentHtmlClient } from "yunque-client/document-html";
import { createApprovalsClient } from "yunque-client/approvals";
import { createApprovalDecisionsClient } from "yunque-client/approval-decisions";
import { createApprovalQueueClient } from "yunque-client/approval-queue";
import { createApprovalPendingClient } from "yunque-client/approval-pending";
import { createApprovalHistoryClient } from "yunque-client/approval-history";
import { createApprovalRulesClient } from "yunque-client/approval-rules";
import { createTraceClient } from "yunque-client/trace";
import { createTraceEventsClient } from "yunque-client/trace-events";
import { createTraceRecentClient } from "yunque-client/trace-recent";
import { createTraceByIdClient } from "yunque-client/trace-by-id";
import { createTaskTraceClient } from "yunque-client/task-trace";
import { createBrowserClient } from "yunque-client/browser";
import { createBrowserStatusClient } from "yunque-client/browser-status";
import { createBrowserCaptureClient } from "yunque-client/browser-capture";
import { createBrowserOPPClient } from "yunque-client/browser-opp";
import { createBrowserExtensionClient } from "yunque-client/browser-extension";
import { createRuntimeClient } from "yunque-client/runtime";
import { createRuntimeQueueClient } from "yunque-client/runtime-queue";
import { createRuntimeEventsClient } from "yunque-client/runtime-events";
import { createRuntimeQueueReadClient } from "yunque-client/runtime-queue-read";
import { createRuntimeQueueControlClient } from "yunque-client/runtime-queue-control";
import { createRouterClient } from "yunque-client/router";
import { createModesClient } from "yunque-client/modes";
import { createModesObserveClient } from "yunque-client/modes-observe";
import { createModesControlClient } from "yunque-client/modes-control";
import { createIDEClient } from "yunque-client/ide";
import { createIDEStatusClient } from "yunque-client/ide-status";
import { createIDEReviewClient } from "yunque-client/ide-review";
import { createPersonaClient } from "yunque-client/persona";
import { createPersonaStateClient } from "yunque-client/persona-state";
import { createPersonaSkillsClient } from "yunque-client/persona-skills";
import { createPersonaPresetsClient } from "yunque-client/persona-presets";
import { createWorkflowClient } from "yunque-client/workflow";
import { createWorkflowDefinitionsClient } from "yunque-client/workflow-definitions";
import { createWorkflowRunsClient } from "yunque-client/workflow-runs";
import { createWorkflowReadClient } from "yunque-client/workflow-read";
import { createWorkflowWriteClient } from "yunque-client/workflow-write";
import { createWorkflowRunClient } from "yunque-client/workflow-run";
import { createWorkflowInstancesClient } from "yunque-client/workflow-instances";
import { createCostClient } from "yunque-client/cost";
import { createCostBudgetClient } from "yunque-client/cost-budget";
import { createCostAlertsClient } from "yunque-client/cost-alerts";
import { createCostObserveClient } from "yunque-client/cost-observe";
import { createCostTaskClient } from "yunque-client/cost-task";
import { createCostHistoryClient } from "yunque-client/cost-history";
import { createUsageClient } from "yunque-client/usage";
import { createLoRAClient } from "yunque-client/lora";
import { createLoRAObserveClient } from "yunque-client/lora-observe";
import { createLoRAStatusClient } from "yunque-client/lora-status";
import { createLoRAHistoryClient } from "yunque-client/lora-history";
import { createLoRAControlClient } from "yunque-client/lora-control";
import { createLoRAConfigClient } from "yunque-client/lora-config";
import { createLoRAPreviewClient } from "yunque-client/lora-preview";
import { createLoRAEvolutionClient } from "yunque-client/lora-evolution";
import { createLoRATriggerClient } from "yunque-client/lora-trigger";
import { createLoRARollbackClient } from "yunque-client/lora-rollback";
import { createIterateClient } from "yunque-client/iterate";
import { createIterateReviewClient } from "yunque-client/iterate-review";
import { createIteratePendingClient } from "yunque-client/iterate-pending";
import { createIterateDecisionsClient } from "yunque-client/iterate-decisions";
import { createIterateCycleClient } from "yunque-client/iterate-cycle";
import { createTrustClient } from "yunque-client/trust";
import { createTrustControlClient } from "yunque-client/trust-control";
import { createReviewClient } from "yunque-client/review";
import { createSkillGrowClient } from "yunque-client/skillgrow";
import { createAuditClient } from "yunque-client/audit";
import { createAuditChainClient } from "yunque-client/audit-chain";
import { createAuditTailClient } from "yunque-client/audit-tail";
import { createAuditVerifyClient } from "yunque-client/audit-verify";
import { createAuditTrailClient } from "yunque-client/audit-trail";
import { createHeartbeatClient } from "yunque-client/heartbeat";
import { createHeartbeatObserveClient } from "yunque-client/heartbeat-observe";
import { createHeartbeatControlClient } from "yunque-client/heartbeat-control";
import { createReverieClient } from "yunque-client/reverie";
import { createReverieObserveClient } from "yunque-client/reverie-observe";
import { createReverieControlClient } from "yunque-client/reverie-control";
import { createFederationClient } from "yunque-client/federation";
import { createFederationObserveClient } from "yunque-client/federation-observe";
import { createFederationPeersClient } from "yunque-client/federation-peers";
import { createFederationStatsClient } from "yunque-client/federation-stats";
import { createFederationCapabilitiesClient } from "yunque-client/federation-capabilities";
import { createFederationControlClient } from "yunque-client/federation-control";
import { createFederationDelegateClient } from "yunque-client/federation-delegate";
import { createSystemClient } from "yunque-client/system";
import { createSystemProbesClient } from "yunque-client/system-probes";
import { createSystemOpsClient } from "yunque-client/system-ops";
import { createSettingsClient } from "yunque-client/settings";
import { createSettingsConfigClient } from "yunque-client/settings-config";
import { createSettingsBackupClient } from "yunque-client/settings-backup";
import { createSettingsSchemaClient } from "yunque-client/settings-schema";
import { createSettingsRuntimeClient } from "yunque-client/settings-runtime";
import { createToriClient } from "yunque-client/tori";
import { createToriObserveClient } from "yunque-client/tori-observe";
import { createToriBindClient } from "yunque-client/tori-bind";
import { createSpeechClient } from "yunque-client/speech";
import { createSpeechTTSClient } from "yunque-client/speech-tts";
import { createSpeechSTTClient } from "yunque-client/speech-stt";
import { createSpeechVoicesClient } from "yunque-client/speech-voices";
import { createAdminClient } from "yunque-client/admin";
import { createAdminDesktopClient } from "yunque-client/admin-desktop";
import { createAdminTenantsClient } from "yunque-client/admin-tenants";
import { createAdminConfigClient } from "yunque-client/admin-config";
import { createFilesClient } from "yunque-client/files";
import { createFilesReadClient } from "yunque-client/files-read";
import { createFilesListClient } from "yunque-client/files-list";
import { createFilesPreviewClient } from "yunque-client/files-preview";
import { createFilesDownloadClient } from "yunque-client/files-download";
import { createCronClient } from "yunque-client/cron";
import { createCronReadClient } from "yunque-client/cron-read";
import { createCronControlClient } from "yunque-client/cron-control";
import { createSkillHubClient } from "yunque-client/skillhub";
import { createSkillHubCatalogClient } from "yunque-client/skillhub-catalog";
import { createSkillHubInstallClient } from "yunque-client/skillhub-install";
import { createSkillHubUpdatesClient } from "yunque-client/skillhub-updates";
import { createSkillHubInstalledClient } from "yunque-client/skillhub-installed";
import { createSkillHubVersionsClient } from "yunque-client/skillhub-versions";
import { createSkillHubPolicyClient } from "yunque-client/skillhub-policy";
import { createSkillsClient } from "yunque-client/skills";
import { createSkillsCatalogClient } from "yunque-client/skills-catalog";
import { createSkillsScanClient } from "yunque-client/skills-scan";
import { createSkillsDynamicClient } from "yunque-client/skills-dynamic";
import { createSkillsSuggestionsClient } from "yunque-client/skills-suggestions";
import { createPluginsClient } from "yunque-client/plugins";
import { createPluginCatalogClient } from "yunque-client/plugin-catalog";
import { createPluginControlClient } from "yunque-client/plugin-control";
import { createPluginToggleClient } from "yunque-client/plugin-toggle";
import { createPluginUIClient } from "yunque-client/plugin-ui";
import { createPluginReloadClient } from "yunque-client/plugin-reload";
import { createPluginFolderClient } from "yunque-client/plugin-folder";
import { createPluginFilesClient } from "yunque-client/plugin-files";
import { createPluginFileReadClient } from "yunque-client/plugin-file-read";
import { createPluginFileSaveClient } from "yunque-client/plugin-file-save";
import { createPluginCrudClient } from "yunque-client/plugin-crud";
import { createPluginCreateClient } from "yunque-client/plugin-create";
import { createPluginDeleteClient } from "yunque-client/plugin-delete";
import { createConnectorsClient } from "yunque-client/connectors";
import { createConnectorCatalogClient } from "yunque-client/connector-catalog";
import { createConnectorAuthClient } from "yunque-client/connector-auth";
import { createConnectorActionsClient } from "yunque-client/connector-actions";
import { createConnectorListClient } from "yunque-client/connector-list";
import { createConnectorDetailClient } from "yunque-client/connector-detail";
import { createConnectorConnectClient } from "yunque-client/connector-connect";
import { createConnectorDisconnectClient } from "yunque-client/connector-disconnect";
import { createNotifyClient } from "yunque-client/notify";
import { createNotifyShareClient } from "yunque-client/notify-share";
import { createNotifyChannelsClient } from "yunque-client/notify-channels";
import { createNotifyChannelReadClient } from "yunque-client/notify-channel-read";
import { createNotifyChannelControlClient } from "yunque-client/notify-channel-control";
import { createProjectsClient } from "yunque-client/projects";
import { createProjectReadClient } from "yunque-client/project-read";
import { createProjectListClient } from "yunque-client/project-list";
import { createProjectDetailClient } from "yunque-client/project-detail";
import { createProjectWriteClient } from "yunque-client/project-write";
import { createSkillMarketClient } from "yunque-client/market";
import { createSkillMarketSearchClient } from "yunque-client/market-search";
import { createSkillMarketQueryClient } from "yunque-client/market-query";
import { createSkillMarketTopClient } from "yunque-client/market-top";
import { createSkillMarketStatsClient } from "yunque-client/market-stats";
import { createDispatchClient } from "yunque-client/dispatch";
import { createDispatchReadClient } from "yunque-client/dispatch-read";
import { createDispatchWorkersClient } from "yunque-client/dispatch-workers";
import { createDispatchQueueClient } from "yunque-client/dispatch-queue";
import { createDispatchWorkerConfigClient } from "yunque-client/dispatch-worker-config";
import { createDispatchControlClient } from "yunque-client/dispatch-control";
import { createOrchestratorClient } from "yunque-client/orchestrator";
import { createOrchestratorReadClient } from "yunque-client/orchestrator-read";
import { createOrchestratorStatusClient } from "yunque-client/orchestrator-status";
import { createOrchestratorEventsClient } from "yunque-client/orchestrator-events";
import { createOrchestratorControlClient } from "yunque-client/orchestrator-control";
import { createForkClient } from "yunque-client/fork";
import { createForkReadClient } from "yunque-client/fork-read";
import { createForkRootClient } from "yunque-client/fork-root";
import { createForkListClient } from "yunque-client/fork-list";
import { createForkControlClient } from "yunque-client/fork-control";
import { createSchedulerClient } from "yunque-client/scheduler";
import { createSchedulerReadClient } from "yunque-client/scheduler-read";
import { createSchedulerControlClient } from "yunque-client/scheduler-control";
import { createUploadClient } from "yunque-client/upload";
import { createGraphClient } from "yunque-client/graph";
import { createGraphReadClient } from "yunque-client/graph-read";
import { createGraphEntitiesClient } from "yunque-client/graph-entities";
import { createGraphRelationsClient } from "yunque-client/graph-relations";
import { createGraphContextClient } from "yunque-client/graph-context";
import { createGraphStatsClient } from "yunque-client/graph-stats";
import { createGraphWriteClient } from "yunque-client/graph-write";
import { createPluginApiClient } from "yunque-client/plugin-api";
import { createPluginLLMClient } from "yunque-client/plugin-llm";
import { createPluginSearchClient } from "yunque-client/plugin-search";
import { createPluginMemoryClient } from "yunque-client/plugin-memory";
import { createPluginMemoryReadClient } from "yunque-client/plugin-memory-read";
import { createPluginMemoryWriteClient } from "yunque-client/plugin-memory-write";
import { createPluginAgentMemoryClient } from "yunque-client/plugin-agent-memory";
import { createPluginAgentMemorySearchClient } from "yunque-client/plugin-agent-memory-search";
import { createPluginAgentMemoryWriteClient } from "yunque-client/plugin-agent-memory-write";
import { createPluginKnowledgeClient } from "yunque-client/plugin-knowledge";
import { createPluginKnowledgeSearchClient } from "yunque-client/plugin-knowledge-search";
import { createPluginKnowledgeIngestClient } from "yunque-client/plugin-knowledge-ingest";
import { createPluginCronClient } from "yunque-client/plugin-cron";
import { createPluginCronReadClient } from "yunque-client/plugin-cron-read";
import { createPluginCronControlClient } from "yunque-client/plugin-cron-control";
import { createPluginSendClient } from "yunque-client/plugin-send";
import { createPluginExtensionsClient } from "yunque-client/plugin-extensions";
import { createPluginExtensionsListClient } from "yunque-client/plugin-extensions-list";
import { createPluginExtensionRegisterClient } from "yunque-client/plugin-extension-register";
import { createStateClient } from "yunque-client/state";
import { createStateSnapshotClient } from "yunque-client/state-snapshot";
import { createStateActionsClient } from "yunque-client/state-actions";
import { createStateCapabilitiesClient } from "yunque-client/state-capabilities";
import { createResourceStateClient } from "yunque-client/resource-state";
import { createFocusStateClient } from "yunque-client/focus-state";
import { createGoalStateClient } from "yunque-client/goal-state";
import { createTriggersClient } from "yunque-client/triggers";
import { createTriggersLegacyClient } from "yunque-client/triggers-legacy";
import { createTriggersReadClient } from "yunque-client/triggers-read";
import { createTriggersControlClient } from "yunque-client/triggers-control";
import { createTriggerDefinitionsClient } from "yunque-client/trigger-definitions";
import { createTriggerDefinitionControlClient } from "yunque-client/trigger-definition-control";
import { createTriggerHistoryClient } from "yunque-client/trigger-history";
import { createTriggerEmitClient } from "yunque-client/trigger-emit";
import { createMissionsClient } from "yunque-client/missions";
import { createMissionsParseClient } from "yunque-client/missions-parse";
import { createReflectClient } from "yunque-client/reflect";
import { createReflectExperiencesClient } from "yunque-client/reflect-experiences";
import { createReflectStrategiesClient } from "yunque-client/reflect-strategies";
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

const connectorListOnly = createConnectorListClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
console.log((await connectorListOnly.list()).connectors.length);

const connectorDetailOnly = createConnectorDetailClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
await connectorDetailOnly.detail("github");

const connectorConnectOnly = createConnectorConnectClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
await connectorConnectOnly.connect({ connector_id: "github", api_key: "<connector-api-key>" });

const connectorDisconnectOnly = createConnectorDisconnectClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
await connectorDisconnectOnly.disconnect("github");

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

const notifyChannelRead = createNotifyChannelReadClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});
const readonlyChannels = await notifyChannelRead.list();

const notifyChannelControl = createNotifyChannelControlClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});
await notifyChannelControl.test("feishu-main");

const notify = createNotifyClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
await notify.share({ channel_id: "feishu-main", message: "任务已完成", session_id: "demo-session" });

const projectRead = createProjectReadClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});

const projectReadResult = await projectRead.list();

const projectList = createProjectListClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});
await projectList.list();

const projectDetail = createProjectDetailClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});
await projectDetail.detail("project-1");

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

const marketQuery = createSkillMarketQueryClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});
await marketQuery.search("docx");

const marketTop = createSkillMarketTopClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});
await marketTop.top({ n: 3 });

const dispatch = createDispatchClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const workerConfig = await dispatch.workerConfig("cursor");
console.log(workerConfig.server_url);

const dispatchWorkers = createDispatchWorkersClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});
await dispatchWorkers.list();

const dispatchQueue = createDispatchQueueClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});
await dispatchQueue.queue();

const dispatchWorkerConfig = createDispatchWorkerConfigClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});
await dispatchWorkerConfig.get("cursor");

const orchestrator = createOrchestratorClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const orchestratorStatus = await orchestrator.status();
console.log(orchestratorStatus.running);

const orchestratorStatusOnly = createOrchestratorStatusClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});
await orchestratorStatusOnly.status();

const orchestratorEvents = createOrchestratorEventsClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});
await orchestratorEvents.events(20);

const fork = createForkClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
const branches = await fork.list("session-1");
console.log(branches.forks.length);

const forkRoot = createForkRootClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});
await forkRoot.root("session-1");

const forkList = createForkListClient({
  baseUrl: "http://localhost:9090",
  token: "<your-token>",
});
await forkList.list("session-1");

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

const loraPreview = createLoRAPreviewClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
console.log((await loraPreview.preview({ tenant_id: "default" })).preview.ready);

const loraEvolution = createLoRAEvolutionClient({
  baseUrl: "http://localhost:9090",
  apiKey: "<your-api-key>",
});
console.log(await loraEvolution.evolution());

const loraTrigger = createLoRATriggerClient({
  baseUrl: "http://localhost:9090",
  token: "<admin-jwt>",
});
await loraTrigger.trigger({ tenant_id: "default" });

const loraRollback = createLoRARollbackClient({
  baseUrl: "http://localhost:9090",
  token: "<admin-jwt>",
});
await loraRollback.rollback();

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

const pluginToggle = createPluginToggleClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await pluginToggle.toggle("demo", true);

const pluginUI = createPluginUIClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await pluginUI.ui();

const pluginReload = createPluginReloadClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await pluginReload.reload();

const pluginFolder = createPluginFolderClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await pluginFolder.openFolder("demo");

const pluginFiles = createPluginFilesClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await pluginFiles.files("demo");

const pluginFileRead = createPluginFileReadClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await pluginFileRead.files("demo");

const pluginFileSave = createPluginFileSaveClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await pluginFileSave.saveFile("demo", "plugin.json", "{}");

const pluginCrud = createPluginCrudClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await pluginCrud.create({ name: "demo", template: "basic" });

const pluginCreate = createPluginCreateClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await pluginCreate.create({ name: "demo", template: "basic" });

const pluginDelete = createPluginDeleteClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await pluginDelete.delete("demo");

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

const graphEntities = createGraphEntitiesClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await graphEntities.entities("云雀");

const graphRelations = createGraphRelationsClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await graphRelations.relations("entity-1");

const graphContext = createGraphContextClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await graphContext.byName("云雀");

const graphStatsOnly = createGraphStatsClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
await graphStatsOnly.stats();

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

const pluginMemoryRead = createPluginMemoryReadClient({
  baseUrl: "http://localhost:9090",
  token: "<plugin-token>",
});
await pluginMemoryRead.search("summary", 3);

const pluginMemoryWrite = createPluginMemoryWriteClient({
  baseUrl: "http://localhost:9090",
  token: "<plugin-token>",
});
await pluginMemoryWrite.set("last-summary", pluginReply.reply);

const pluginAgentMemorySearch = createPluginAgentMemorySearchClient({
  baseUrl: "http://localhost:9090",
  token: "<plugin-token>",
});
await pluginAgentMemorySearch.search("release checklist", 4);

const pluginAgentMemoryWrite = createPluginAgentMemoryWriteClient({
  baseUrl: "http://localhost:9090",
  token: "<plugin-token>",
});
await pluginAgentMemoryWrite.add("Release checklist passed", "plugin-demo");

const pluginKnowledgeSearch = createPluginKnowledgeSearchClient({
  baseUrl: "http://localhost:9090",
  token: "<plugin-token>",
});
await pluginKnowledgeSearch.search("deployment", 5);

const pluginKnowledgeIngest = createPluginKnowledgeIngestClient({
  baseUrl: "http://localhost:9090",
  token: "<plugin-token>",
});
await pluginKnowledgeIngest.ingest("Runbook content", "plugin-demo", "runbook.md");

const pluginCronRead = createPluginCronReadClient({
  baseUrl: "http://localhost:9090",
  token: "<plugin-token>",
});
await pluginCronRead.list("demo/plugin");

const pluginCronControl = createPluginCronControlClient({
  baseUrl: "http://localhost:9090",
  token: "<plugin-token>",
});
await pluginCronControl.add("daily", "0 8 * * *", "ping");

const pluginExtensionsList = createPluginExtensionsListClient({
  baseUrl: "http://localhost:9090",
  token: "<plugin-token>",
});
await pluginExtensionsList.list();

const pluginExtensionRegister = createPluginExtensionRegisterClient({
  baseUrl: "http://localhost:9090",
  token: "<plugin-token>",
});
await pluginExtensionRegister.provider({ id: "local-llm" });

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

const stateSnapshot = createStateSnapshotClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
const currentState = await stateSnapshot.get();
console.log(currentState.goals.length, currentState.resources.length);

const stateActions = createStateActionsClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
console.log((await stateActions.list())[0]?.action);

const stateCapabilities = createStateCapabilitiesClient({
  baseUrl: "http://localhost:9090",
  token: "<your-jwt>",
});
console.log((await stateCapabilities.get()).total_skills);

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
in only `auth`, `airi`, `planner-recovery`, `planner`, `planner-read`, `planner-control`, `planner-checkpoints`, `planner-resume`, `planner-execution-state`, `chat`, `chat-basic`, `chat-agentic`, `chat-stream`, `cognis`, `cognis-registry`, `cognis-observe`, `cognis-traces`, `cognis-health`, `cognis-alerts`, `cognis-experience`, `cognis-evolution`, `cognis-federation`, `cognis-workflows`, `cognis-bundles`, `events`, `events-stream`, `events-parse`, `realtime`, `realtime-connect`, `realtime-messages`, `webchat`, `webchat-widget`, `webchat-embed`, `conversations`, `conversations-read`, `conversations-control`, `conversation-sessions`, `conversation-messages`, `conversation-replay`, `conversation-message-control`, `conversation-manage`, `subagents`, `subagents-read`, `subagents-control`, `bots`, `bots-read`, `bots-control`, `bots-list`, `bots-detail`, `bots-inbox`, `bots-channels`, `discovery`, `discovery-identity`, `discovery-embeddings`, `discovery-search`, `identity`, `embeddings`, `search`, `interactions`, `emotion`, `emotion-history`, `emotion-stickers`, `reactions`, `instructions`, `rbac`, `roles`, `role-bindings`, `my-roles`, `permissions`, `memory`, `memory-search`, `memory-stats`, `memory-add`, `memory-compact`, `tasks`, `task-context`, `task-observe`, `task-templates`, `task-threads`, `task-gaps`, `task-memory`, `task-thread-read`, `task-thread-control`, `task-lifecycle`, `task-read`, `task-create`, `task-delete`, `knowledge`, `knowledge-search`, `knowledge-ingest`, `knowledge-sources`, `knowledge-source-read`, `knowledge-source-control`, `knowledge-import`, `knowledge-upload`, or
`providers`/`provider-control`/`provider-mode`/`provider-session`/`provider-health`/`provider-registry`/`breaker`/`provider-breaker`/`models`/`setup`/`setup-detect`/`setup-templates`/`setup-provider`/`setup-install`/`documents`/`document-templates`/`document-generate`/`document-docx`/`document-xlsx`/`document-pptx`/`document-html`/`approvals`/`approval-queue`/`approval-pending`/`approval-history`/`approval-rules`/`trace`/`trace-events`/`trace-recent`/`trace-by-id`/`task-trace`/`browser`/`browser-status`/`browser-capture`/`browser-opp`/`browser-extension`/`runtime`/`runtime-queue`/`runtime-events`/`runtime-queue-read`/`runtime-queue-control`/`router`/`modes`/`modes-observe`
`/ide`/`persona`/`persona-state`/`persona-skills`/`persona-presets`/`workflow`/`workflow-definitions`/`workflow-runs`/`workflow-read`/`workflow-write`/`workflow-run`/`workflow-instances`/`cost`/`cost-budget`/`cost-alerts`/`cost-observe`/`cost-task`/`cost-history`/`usage`/`lora`/`lora-observe`/`lora-status`/`lora-history`/`lora-control`/`lora-config`/`lora-preview`/`lora-evolution`/`lora-trigger`/`lora-rollback`/`iterate`/`iterate-review`/`iterate-pending`/`iterate-decisions`/`iterate-cycle`/`trust`/`trust-control`/`review`/`skillgrow`/`audit`/`audit-chain`/`audit-tail`/`audit-verify`/`audit-trail`/`heartbeat`/`heartbeat-observe`/`heartbeat-control`
`/reverie`/`federation`/`federation-peers`/`federation-stats`/`federation-capabilities`/`system`/`system-probes`/`system-ops`/`settings`/`settings-config`/`settings-backup`/`settings-schema`/`settings-runtime`/`tori`/`tori-observe`/`tori-bind`/`speech`/`speech-tts`/`speech-stt`/`speech-voices`/`upload`/`admin`/`files`/`files-list`/`files-preview`/`cron`/`skillhub`/`skillhub-installed`/`skillhub-versions`/`skills`/`skills-catalog`/`skills-scan`/`skills-dynamic`/`skills-suggestions`/`plugins`/`plugin-toggle`/`plugin-ui`/`plugin-reload`/`plugin-folder`/`connectors`/`connector-catalog`/`connector-auth`/`connector-actions`/`connector-list`/`connector-detail`/`connector-connect`/`connector-disconnect`/`notify`/`notify-share`/`notify-channels`/`notify-channel-read`/`notify-channel-control`/`projects`/`project-read`/`project-list`/`project-detail`/`project-write`/`market`/`market-search`/`market-query`/`market-top`/`market-stats`/`dispatch`/`dispatch-read`/`dispatch-workers`/`dispatch-queue`/`dispatch-worker-config`/`dispatch-control`/`orchestrator`/`orchestrator-read`/`orchestrator-status`/`orchestrator-events`/`orchestrator-control`/`fork`/`fork-read`/`fork-root`/`fork-list`/`fork-control`/`scheduler`/`graph`/`graph-read`/`graph-entities`/`graph-relations`/`graph-context`/`graph-stats`/`graph-write`/`plugin-api`/`plugin-llm`/`plugin-search`/`plugin-memory`/`plugin-memory-read`/`plugin-memory-write`/`plugin-agent-memory`/`plugin-agent-memory-search`/`plugin-agent-memory-write`/`plugin-knowledge`/`plugin-knowledge-search`/`plugin-knowledge-ingest`/`plugin-cron`/`plugin-cron-read`/`plugin-cron-control`/`plugin-send`/`plugin-extensions`/`plugin-extensions-list`/`plugin-extension-register`/`state`/`triggers`/`trigger-definitions`/`trigger-definition-control`/`trigger-history`/`trigger-emit`/`missions`/`reflect`/`tools`/`sandbox` without importing the generated 500KB+ SDK/types bundle. Add future
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
npm run test -- state-actions state-capabilities
# optional focused run for changed slices; omit args to run all incremental tests
```

## Layout

| File / dir | Purpose |
|---|---|
| `src/sdk.gen.ts` | Per-endpoint typed functions (~263 KB) |
| `src/types.gen.ts` | All schemas, request/response types (~295 KB) |
| `src/client.gen.ts` | Default client instance |
| `src/client/` | Fetch runtime (from `@hey-api/client-fetch`) |
| `src/core/` | Internal helpers |
| `src/agent-kit.ts` | Lightweight bundle for State Kernel, Reflection Experience, Mission Parse, Scheduler, Cron System, Triggers, Memory Kernel, and Plugin API Runtime clients without generated SDK imports |
| `src/auth.ts` | Lightweight hand-written setup status, password login/setup, Tori OAuth URL, and API-key to JWT exchange slice |
| `src/airi.ts` | Lightweight hand-written Airi bridge status, OpenAI-compatible models, and chat completions slice |
| `src/planner-recovery.ts` | Lightweight hand-written Planner recovery slice for incremental imports |
| `src/planner.ts` | Lightweight planner facade over checkpoint recovery and execution state |
| `src/planner-read.ts` | Lightweight Planner checkpoint/job/execution-state read facade without recovery mutation APIs |
| `src/planner-control.ts` | Lightweight Planner checkpoint recover/resume facade without read/list APIs |
| `src/planner-checkpoints.ts` | Lightweight Planner checkpoint listing facade without recovery/resume APIs |
| `src/planner-resume.ts` | Lightweight Planner checkpoint resume facade without read or recover-plan APIs |
| `src/planner-execution-state.ts` | Lightweight Planner execution-state and resume-job facade without checkpoint listing or mutation APIs |
| `src/chat.ts` | Lightweight hand-written Chat/SSE slice for incremental imports |
| `src/chat-basic.ts` | Lightweight Chat JSON send facade without SSE stream or agentic APIs |
| `src/chat-agentic.ts` | Lightweight Chat agentic facade without basic JSON send or SSE stream APIs |
| `src/chat-stream.ts` | Lightweight Chat SSE stream/parse facade without non-streaming chat or agentic APIs |
| `src/cognis.ts` | Lightweight hand-written Cogni registry, health, traces, workflow, experience, evolution, and federation control slice |
| `src/cognis-registry.ts` | Lightweight Cogni registry list/create/get/remove/enable/disable/reload facade without traces or evolution APIs |
| `src/cognis-observe.ts` | Lightweight Cogni traces/stats/health/verify/alerts facade without registry mutation or evolution APIs |
| `src/cognis-traces.ts` | Lightweight Cogni trace list/read facade without stats, health, alerts or registry APIs |
| `src/cognis-health.ts` | Lightweight Cogni stats/health/verify facade without traces, alerts or registry APIs |
| `src/cognis-alerts.ts` | Lightweight Cogni alerts list/scan facade without traces, health or registry APIs |
| `src/cognis-experience.ts` | Lightweight Cogni experience read/record/confirm facade without registry, trace or evolution APIs |
| `src/cognis-evolution.ts` | Lightweight Cogni evolve/status facade without registry, trace, experience or federation APIs |
| `src/cognis-federation.ts` | Lightweight Cogni federation status/peers/discover/expose/economics facade without registry, trace, experience or workflow APIs |
| `src/cognis-workflows.ts` | Lightweight Cogni workflows list/run facade without registry, trace, experience, evolution or federation APIs |
| `src/cognis-bundles.ts` | Lightweight Cogni bundle generate/export/import facade without registry, trace, experience, evolution, workflow or federation APIs |
| `src/events.ts` | Lightweight hand-written SSE event stream slice for task/workflow/approval live updates |
| `src/events-stream.ts` | Lightweight SSE connection facade without standalone parser-only API |
| `src/events-parse.ts` | Lightweight SSE parser facade for local stream decoding without opening `/v1/events/stream` |
| `src/realtime.ts` | Lightweight hand-written `/v1/ws` URL, connect, ping/chat message helper slice |
| `src/realtime-connect.ts` | Lightweight WebSocket URL/connect facade without message serialization helpers |
| `src/realtime-messages.ts` | Lightweight realtime ping/chat/send/parse helper facade without opening sockets |
| `src/webchat.ts` | Lightweight hand-written embeddable WebChat widget script/snippet slice |
| `src/webchat-widget.ts` | Lightweight WebChat widget URL/script facade without embed snippet helpers |
| `src/webchat-embed.ts` | Lightweight WebChat embed snippet facade without fetching widget script |
| `src/conversations.ts` | Lightweight hand-written conversation history, management, and replay slice |
| `src/conversations-read.ts` | Lightweight conversation list/messages/replay read facade without delete/manage APIs |
| `src/conversations-control.ts` | Lightweight conversation delete/manage facade without list/replay APIs |
| `src/conversation-sessions.ts` | Lightweight conversation session list facade without messages, replay or control APIs |
| `src/conversation-messages.ts` | Lightweight conversation message read facade without session list, replay or control APIs |
| `src/conversation-replay.ts` | Lightweight conversation replay facade without session/message reads or control APIs |
| `src/conversation-message-control.ts` | Lightweight conversation message delete facade without manage or read APIs |
| `src/conversation-manage.ts` | Lightweight conversation rename/pin/archive facade without message delete or read APIs |
| `src/subagents.ts` | Lightweight hand-written subagent list/spawn/message/destroy slice |
| `src/subagents-read.ts` | Lightweight subagent list/get read facade without spawn/message/destroy APIs |
| `src/subagents-control.ts` | Lightweight subagent spawn/message/destroy facade without list/get APIs |
| `src/bots.ts` | Lightweight hand-written bots, inbox, and channel group operations slice |
| `src/bots-read.ts` | Lightweight bot/inbox/channel-group read facade without create/update/delete APIs |
| `src/bots-control.ts` | Lightweight bot and inbox mutation facade without bot/inbox/channel read APIs |
| `src/bots-list.ts` | Lightweight bot list facade without detail, mutation, inbox or channel APIs |
| `src/bots-detail.ts` | Lightweight bot detail facade without list, mutation, inbox or channel APIs |
| `src/bots-inbox.ts` | Lightweight inbox list/push/read/delete facade without bot CRUD or channel APIs |
| `src/bots-channels.ts` | Lightweight channel group read facade without bot CRUD or inbox APIs |
| `src/discovery.ts` | Lightweight hand-written identity, embeddings, and web search discovery slice |
| `src/discovery-identity.ts` | Lightweight identity resolution/profile facade without embeddings or search APIs |
| `src/discovery-embeddings.ts` | Lightweight embedding provider/embed facade without identity or search APIs |
| `src/discovery-search.ts` | Lightweight web search/provider facade without identity or embedding APIs |
| `src/identity.ts` | Lightweight identity resolve/profile facade for `/v1/identity/*` without full SDK import |
| `src/embeddings.ts` | Lightweight embeddings providers/embed facade for `/v1/embeddings` without full SDK import |
| `src/search.ts` | Lightweight search facade for `/v1/search` and `/v1/search/providers` without full SDK import |
| `src/interactions.ts` | Lightweight hand-written emotion history, stickers, instructions, reactions, and sticker sending slice |
| `src/emotion.ts` | Lightweight emotion history/sticker mapping facade for `/v1/emotion/*` without full SDK import |
| `src/emotion-history.ts` | Lightweight emotion history facade without sticker mapping or reaction APIs |
| `src/emotion-stickers.ts` | Lightweight sticker mapping facade without emotion history or reaction APIs |
| `src/reactions.ts` | Lightweight reaction/sticker sending facade for `/v1/react` and `/v1/sticker/send` without full SDK import |
| `src/instructions.ts` | Lightweight user-instructions facade for `/v1/instructions*` without full SDK import |
| `src/rbac.ts` | Lightweight hand-written RBAC roles, assignments, and permission-check slice |
| `src/roles.ts` | Lightweight role/assignment facade over RBAC role endpoints without full SDK import |
| `src/role-bindings.ts` | Lightweight role assignment/revoke facade without role CRUD or permission checks |
| `src/my-roles.ts` | Lightweight current-subject role read facade without role CRUD, assignment or permission checks |
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
| `src/task-gaps.ts` | Lightweight task gap list/stats/resolve facade without working-memory, template or thread APIs |
| `src/task-memory.ts` | Lightweight task working-memory facade without gaps, template or thread APIs |
| `src/task-thread-read.ts` | Lightweight task thread list/get facade without post-message or update-state APIs |
| `src/task-thread-control.ts` | Lightweight task thread post-message/update-state facade without thread read APIs |
| `src/task-lifecycle.ts` | Lightweight task run/pause/resume/restart/cancel facade without full SDK import |
| `src/task-read.ts` | Lightweight task list/detail read-only facade without full SDK import |
| `src/task-create.ts` | Lightweight task creation facade without full SDK import |
| `src/task-delete.ts` | Lightweight task deletion facade without full SDK import |
| `src/knowledge.ts` | Lightweight hand-written Knowledge search/ingest/import/upload slice |
| `src/knowledge-search.ts` | Lightweight knowledge search-only facade without full SDK import |
| `src/knowledge-ingest.ts` | Lightweight inline knowledge ingestion facade without full SDK import |
| `src/knowledge-sources.ts` | Lightweight knowledge source stats/list/update/delete facade without full SDK import |
| `src/knowledge-source-read.ts` | Lightweight knowledge source stats/list facade without update/delete APIs |
| `src/knowledge-source-control.ts` | Lightweight knowledge source update/delete facade without stats/list APIs |
| `src/knowledge-import.ts` | Lightweight URL/repo knowledge import facade without full SDK import |
| `src/knowledge-upload.ts` | Lightweight knowledge file upload facade without full SDK import |
| `src/providers.ts` | Lightweight hand-written LLM provider/model configuration slice |
| `src/provider-registry.ts` | Lightweight provider preset, registration and discovery facade without full SDK import |
| `src/provider-control.ts` | Lightweight provider lifecycle and runtime selection facade without full SDK import |
| `src/provider-mode.ts` | Lightweight provider mode and exec-provider facade without lifecycle or registry APIs |
| `src/provider-session.ts` | Lightweight per-session provider override facade without global provider controls |
| `src/provider-health.ts` | Lightweight provider status, mode and connectivity facade without full SDK import |
| `src/breaker.ts` | Lightweight provider breaker reset facade for `/api/breaker/reset` without full SDK import |
| `src/provider-breaker.ts` | Lightweight provider breaker reset facade named alongside provider subpaths |
| `src/models.ts` | Lightweight models facade for listing and maintaining `/v1/models` without full SDK import |
| `src/setup.ts` | Lightweight hand-written first-run setup/configuration wizard slice |
| `src/setup-detect.ts` | Lightweight setup detect/health facade without setup write or install APIs |
| `src/setup-templates.ts` | Lightweight setup template catalog facade without setup write or install APIs |
| `src/setup-provider.ts` | Lightweight setup provider test/apply facade without detect, template catalog or install APIs |
| `src/setup-install.ts` | Lightweight setup component install and SSE progress facade without detect, templates or provider apply APIs |
| `src/documents.ts` | Lightweight hand-written DOCX/XLSX/PPTX/HTML generation slice |
| `src/document-templates.ts` | Lightweight document template catalog facade without generation APIs |
| `src/document-generate.ts` | Lightweight document generation and format helper facade without template catalog APIs |
| `src/document-docx.ts` | Lightweight DOCX-only generation facade without template, XLSX, PPTX or HTML APIs |
| `src/document-xlsx.ts` | Lightweight XLSX-only generation facade without template, DOCX, PPTX or HTML APIs |
| `src/document-pptx.ts` | Lightweight PPTX-only generation facade without template, DOCX, XLSX or HTML APIs |
| `src/document-html.ts` | Lightweight HTML-only generation facade without template, DOCX, XLSX or PPTX APIs |
| `src/approvals.ts` | Lightweight hand-written human-in-the-loop approval queue/rules slice |
| `src/approval-decisions.ts` | Lightweight approval decision facade for approve/deny/decide without queue or rule reads |
| `src/approval-queue.ts` | Lightweight approval queue and decision facade without full SDK import |
| `src/approval-pending.ts` | Lightweight pending approval queue and decision facade without history or rule APIs |
| `src/approval-history.ts` | Lightweight approval history read facade without pending decisions or rule APIs |
| `src/approval-rules.ts` | Lightweight approval rule management facade without full SDK import |
| `src/trace.ts` | Lightweight hand-written execution/audit trace inspection slice |
| `src/trace-events.ts` | Lightweight trace recent/by-trace-id facade without full SDK import |
| `src/trace-recent.ts` | Lightweight trace recent-events facade without by-trace-id or by-task-id APIs |
| `src/trace-by-id.ts` | Lightweight trace-id event read facade without recent or task trace APIs |
| `src/task-trace.ts` | Lightweight task trace read facade without full SDK import |
| `src/browser.ts` | Lightweight hand-written browser extension automation and OPP slice |
| `src/browser-status.ts` | Lightweight browser status/config/extension-status facade without automation or screenshot APIs |
| `src/browser-capture.ts` | Lightweight browser screenshot/latest-screenshot/OCR facade without navigation or extension action APIs |
| `src/browser-opp.ts` | Lightweight browser OPP pending/decision facade without navigation, capture or extension action APIs |
| `src/browser-extension.ts` | Lightweight browser extension session/action/scenario facade without status, capture or OPP APIs |
| `src/runtime.ts` | Lightweight hand-written session queue and events stream slice |
| `src/runtime-queue.ts` | Lightweight runtime queue overview/session/cancel facade without event stream APIs |
| `src/runtime-events.ts` | Lightweight runtime SSE event stream facade without session queue APIs |
| `src/runtime-queue-read.ts` | Lightweight runtime queue overview/session facade without cancel or event stream APIs |
| `src/runtime-queue-control.ts` | Lightweight runtime queue cancel facade without overview/session or event stream APIs |
| `src/router.ts` | Lightweight hand-written smart-router stats and status slice |
| `src/modes.ts` | Lightweight hand-written persona mode listing/switching slice |
| `src/modes-observe.ts` | Lightweight persona modes list/current facade without switching APIs |
| `src/modes-control.ts` | Lightweight persona mode switching facade without list/current observe APIs |
| `src/ide.ts` | Lightweight hand-written IDE status/code-review slice |
| `src/ide-status.ts` | Lightweight IDE status facade without code-review APIs |
| `src/ide-review.ts` | Lightweight IDE code-review facade without status APIs |
| `src/persona.ts` | Lightweight hand-written persona identity/skills/presets slice |
| `src/persona-state.ts` | Lightweight persona identity/soul state facade without skills or presets APIs |
| `src/persona-skills.ts` | Lightweight persona skills list/add/delete facade without identity or presets APIs |
| `src/persona-presets.ts` | Lightweight persona presets/custom preset/feature flag facade without identity or skills APIs |
| `src/workflow.ts` | Lightweight hand-written workflow definition/instance execution slice |
| `src/workflow-definitions.ts` | Lightweight workflow definition management facade without full SDK import |
| `src/workflow-runs.ts` | Lightweight workflow run and instance facade without full SDK import |
| `src/workflow-read.ts` | Lightweight workflow definition read facade without save/delete or run APIs |
| `src/workflow-write.ts` | Lightweight workflow definition save/delete facade without read or run APIs |
| `src/workflow-run.ts` | Lightweight workflow run/cancel facade without definition or instance read APIs |
| `src/workflow-instances.ts` | Lightweight workflow instance list/get facade without run/cancel APIs |
| `src/cost.ts` | Lightweight hand-written cost, usage and quota slice |
| `src/cost-budget.ts` | Lightweight cost summary/budget/alerts facade without task cost, usage or quota APIs |
| `src/cost-alerts.ts` | Lightweight cost alerts facade without summary, budget, task cost, usage or quota APIs |
| `src/cost-observe.ts` | Lightweight task cost/timeline/breakdown/history facade without budget, usage or quota APIs |
| `src/cost-task.ts` | Lightweight task cost and timeline facade without breakdown, history, budget, usage or quota APIs |
| `src/cost-history.ts` | Lightweight cost history facade without task timeline, breakdown, budget, usage or quota APIs |
| `src/usage.ts` | Lightweight usage/quota facade for `/v1/usage` and `/v1/quota` without full SDK import |
| `src/lora.ts` | Lightweight hand-written LoRA training and evolution lifecycle slice |
| `src/lora-observe.ts` | Lightweight LoRA status/history/summary/preview/evolution facade without training or config mutation APIs |
| `src/lora-status.ts` | Lightweight LoRA status/preview/evolution facade without history, summary, training or config mutation APIs |
| `src/lora-history.ts` | Lightweight LoRA history/summary facade without status, preview, training or config mutation APIs |
| `src/lora-control.ts` | Lightweight LoRA trigger/rollback/config facade without status/history/summary/evolution APIs |
| `src/lora-config.ts` | Lightweight LoRA config read/update facade without status, history, trigger or rollback APIs |
| `src/lora-preview.ts` | Lightweight LoRA preview-only facade without status/history/training/config APIs |
| `src/lora-evolution.ts` | Lightweight LoRA evolution-state facade without preview/history/training/config APIs |
| `src/lora-trigger.ts` | Lightweight LoRA trigger-only facade without rollback/read/config APIs |
| `src/lora-rollback.ts` | Lightweight LoRA rollback-only facade without trigger/read/config APIs |
| `src/iterate.ts` | Lightweight hand-written self-iteration proposal approval slice |
| `src/iterate-review.ts` | Lightweight self-iteration proposal list/approve/reject facade without cycle trigger/status APIs |
| `src/iterate-pending.ts` | Lightweight pending self-iteration proposal facade without approve/reject or cycle APIs |
| `src/iterate-decisions.ts` | Lightweight self-iteration approve/reject facade without proposal list or cycle APIs |
| `src/iterate-cycle.ts` | Lightweight self-iteration trigger/status facade without proposal review APIs |
| `src/trust.ts` | Lightweight hand-written trust, review-gate and skill-growth slice |
| `src/trust-control.ts` | Lightweight trust scores/reset/grant facade without review-gate or skill-growth APIs |
| `src/review.ts` | Lightweight review-gate status facade for `/api/review/status` without full SDK import |
| `src/skillgrow.ts` | Lightweight skill-growth pattern facade for `/api/skillgrow/patterns` without full SDK import |
| `src/audit.ts` | Lightweight hand-written audit chain and audit trail inspection slice |
| `src/audit-chain.ts` | Lightweight audit tail/verify/stats facade without task audit trail APIs |
| `src/audit-tail.ts` | Lightweight audit tail facade without verify, stats or task audit trail APIs |
| `src/audit-verify.ts` | Lightweight audit verify/stats facade without tail or task audit trail APIs |
| `src/audit-trail.ts` | Lightweight task audit trail facade without audit-chain tail/verify/stats APIs |
| `src/heartbeat.ts` | Lightweight hand-written proactive heartbeat lifecycle slice |
| `src/heartbeat-observe.ts` | Lightweight heartbeat status/logs facade without update or trigger APIs |
| `src/heartbeat-control.ts` | Lightweight heartbeat update/trigger facade without status or logs APIs |
| `src/reverie.ts` | Lightweight hand-written inner monologue and proactive thought slice |
| `src/reverie-observe.ts` | Lightweight reverie journal/stats/config/actions/targets facade without write APIs |
| `src/reverie-control.ts` | Lightweight reverie config/think/delete facade without observation APIs |
| `src/federation.ts` | Lightweight hand-written federation peers, capabilities, discovery, delegation, and broadcast slice |
| `src/federation-observe.ts` | Lightweight federation peers/stats/capabilities/bridge-stats facade without delegation or broadcast APIs |
| `src/federation-peers.ts` | Lightweight federation peers facade without stats, capabilities or delegation APIs |
| `src/federation-stats.ts` | Lightweight federation stats and bridge-stats facade without peers, capabilities or delegation APIs |
| `src/federation-capabilities.ts` | Lightweight federation capabilities read/update/broadcast facade without peers, stats, discovery or delegation APIs |
| `src/federation-control.ts` | Lightweight federation capabilities update/broadcast facade without discovery or delegation APIs |
| `src/federation-delegate.ts` | Lightweight federation discover/delegate facade without status or broadcast APIs |
| `src/system.ts` | Lightweight hand-written health, version, SBOM, metrics, cache, and module observability slice |
| `src/system-probes.ts` | Lightweight system health/livez/readyz/cognitive/version facade without ops metrics APIs |
| `src/system-ops.ts` | Lightweight system info/stats/metrics/cache/modules/SBOM facade without public probe APIs |
| `src/settings.ts` | Lightweight hand-written settings, config reload, directory detection, and backup/restore slice |
| `src/settings-config.ts` | Lightweight settings schema/config/update/check/reload/detect-dirs facade without backup APIs |
| `src/settings-backup.ts` | Lightweight settings backup info/export/import facade without config APIs |
| `src/settings-schema.ts` | Lightweight settings schema read facade without config mutation, reload, directory detection or backup APIs |
| `src/settings-runtime.ts` | Lightweight settings setup check/reload/detect-dirs facade without schema/config update or backup APIs |
| `src/tori.ts` | Lightweight hand-written Tori OAuth binding, status, health, and usage slice |
| `src/tori-observe.ts` | Lightweight Tori status/health/usage facade without bind or unbind APIs |
| `src/tori-bind.ts` | Lightweight Tori bind/unbind facade without status, health, or usage APIs |
| `src/speech.ts` | Lightweight hand-written speech TTS/STT, STT stream URL, voices, and file upload slice |
| `src/speech-tts.ts` | Lightweight speech TTS facade without STT, voices, stream URL, or upload APIs |
| `src/speech-stt.ts` | Lightweight speech STT and stream URL facade without TTS, voices, or upload APIs |
| `src/speech-voices.ts` | Lightweight speech voices catalog facade without TTS, STT, stream URL, or upload APIs |
| `src/admin.ts` | Lightweight hand-written desktop controls, tenants, and natural-language config slice |
| `src/admin-desktop.ts` | Lightweight desktop console/autostart facade without tenant or NL config APIs |
| `src/admin-tenants.ts` | Lightweight tenant list/create facade without desktop or NL config APIs |
| `src/admin-config.ts` | Lightweight natural-language config facade without desktop or tenant APIs |
| `src/files.ts` | Lightweight hand-written artifact file listing, preview, and download slice |
| `src/files-read.ts` | Lightweight artifact list/preview facade without download APIs |
| `src/files-list.ts` | Lightweight artifact file listing facade without preview or download APIs |
| `src/files-preview.ts` | Lightweight artifact preview facade without list or download APIs |
| `src/files-download.ts` | Lightweight artifact download facade without list/preview APIs |
| `src/cron.ts` | Lightweight hand-written cron job scheduling and run-now slice |
| `src/cron-read.ts` | Lightweight cron job listing facade without scheduling or run-now APIs |
| `src/cron-control.ts` | Lightweight cron add/remove/run-now facade without listing APIs |
| `src/skillhub.ts` | Lightweight hand-written SkillHub search/install/update/policy slice |
| `src/skillhub-catalog.ts` | Lightweight SkillHub search/trending/detail facade without full SDK import |
| `src/skillhub-install.ts` | Lightweight SkillHub install/update/rollback lifecycle facade without full SDK import |
| `src/skillhub-updates.ts` | Lightweight SkillHub update check/update/rollback/version facade without install/uninstall APIs |
| `src/skillhub-installed.ts` | Lightweight SkillHub installed-skill list facade without catalog, install, update or policy APIs |
| `src/skillhub-versions.ts` | Lightweight SkillHub version history and rollback facade without catalog, installed or policy APIs |
| `src/skillhub-policy.ts` | Lightweight SkillHub policy/check/analytics facade without full SDK import |
| `src/skills.ts` | Lightweight hand-written runtime skills catalog, scan, dynamic review, and suggestions slice |
| `src/skills-catalog.ts` | Lightweight runtime skills catalog facade without scan, dynamic review or suggestions APIs |
| `src/skills-scan.ts` | Lightweight runtime skills scan facade without catalog, dynamic review or suggestions APIs |
| `src/skills-dynamic.ts` | Lightweight dynamic skill list/approve/reject facade without catalog, scan or suggestions APIs |
| `src/skills-suggestions.ts` | Lightweight session skill suggestions facade without catalog, scan or dynamic review APIs |
| `src/plugins.ts` | Lightweight hand-written plugin CRUD, files, UI tabs, reload, and folder-open slice |
| `src/plugin-catalog.ts` | Lightweight plugin list/status catalog facade without full SDK import |
| `src/plugin-control.ts` | Lightweight plugin toggle/ui/reload/open-folder facade without full SDK import |
| `src/plugin-toggle.ts` | Lightweight plugin enable/disable facade without UI, reload or folder-open APIs |
| `src/plugin-ui.ts` | Lightweight plugin UI tabs facade without toggle, reload or folder-open APIs |
| `src/plugin-reload.ts` | Lightweight plugin reload facade without toggle, UI or folder-open APIs |
| `src/plugin-folder.ts` | Lightweight plugin folder-open facade without toggle, UI or reload APIs |
| `src/plugin-files.ts` | Lightweight plugin file read/save facade without full SDK import |
| `src/plugin-file-read.ts` | Lightweight plugin file read facade without save or other plugin APIs |
| `src/plugin-file-save.ts` | Lightweight plugin file save facade without read or other plugin APIs |
| `src/plugin-crud.ts` | Lightweight plugin create/delete facade without full SDK import |
| `src/plugin-create.ts` | Lightweight plugin create facade without delete or other plugin APIs |
| `src/plugin-delete.ts` | Lightweight plugin delete facade without create or other plugin APIs |
| `src/connectors.ts` | Lightweight hand-written connector catalog, auth, and action execution slice |
| `src/connector-catalog.ts` | Lightweight connector list/detail catalog facade without full SDK import |
| `src/connector-auth.ts` | Lightweight connector connect/disconnect facade without full SDK import |
| `src/connector-actions.ts` | Lightweight connector action execution facade without full SDK import |
| `src/connector-list.ts` | Lightweight connector list-only facade without detail/auth/action APIs |
| `src/connector-detail.ts` | Lightweight connector detail-only facade without list/auth/action APIs |
| `src/connector-connect.ts` | Lightweight connector connect-only facade without disconnect/list/action APIs |
| `src/connector-disconnect.ts` | Lightweight connector disconnect-only facade without connect/list/action APIs |
| `src/notify.ts` | Lightweight hand-written notification channels, test, and share dispatch slice |
| `src/notify-share.ts` | Lightweight notification share dispatch facade without full SDK import |
| `src/notify-channels.ts` | Lightweight notification channel management facade without full SDK import |
| `src/notify-channel-read.ts` | Lightweight notification channel list-only facade without add/remove/toggle/test APIs |
| `src/notify-channel-control.ts` | Lightweight notification channel mutation/test facade without list or share APIs |
| `src/projects.ts` | Lightweight hand-written project workspace CRUD slice |
| `src/project-read.ts` | Lightweight project list/detail read facade without full SDK import |
| `src/project-list.ts` | Lightweight project list-only facade without detail or mutation APIs |
| `src/project-detail.ts` | Lightweight project detail-only facade without list or mutation APIs |
| `src/project-write.ts` | Lightweight project create/update/remove facade without full SDK import |
| `src/market.ts` | Lightweight hand-written skill marketplace search, ranking, and stats slice |
| `src/market-search.ts` | Lightweight skill marketplace search/top facade without stats APIs |
| `src/market-query.ts` | Lightweight skill marketplace query-only facade without top ranking or stats APIs |
| `src/market-top.ts` | Lightweight skill marketplace top-ranking facade without free-text search or stats APIs |
| `src/market-stats.ts` | Lightweight skill marketplace stats facade without search/top APIs |
| `src/dispatch.ts` | Lightweight hand-written MCP dispatch worker, queue, and config slice |
| `src/dispatch-read.ts` | Lightweight dispatch worker/queue/config read facade without enqueue/remove APIs |
| `src/dispatch-workers.ts` | Lightweight dispatch worker registry list/detail facade without queue, config or enqueue APIs |
| `src/dispatch-queue.ts` | Lightweight dispatch queue-only facade without worker registry, config or enqueue APIs |
| `src/dispatch-worker-config.ts` | Lightweight dispatch worker config facade without worker registry, queue or enqueue APIs |
| `src/dispatch-control.ts` | Lightweight dispatch enqueue/remove facade without worker registry reads |
| `src/orchestrator.ts` | Lightweight hand-written IDE worker orchestrator daemon, session, event, and policy slice |
| `src/orchestrator-read.ts` | Lightweight orchestrator status/session/detect/event/policy read facade without control APIs |
| `src/orchestrator-status.ts` | Lightweight orchestrator status/session/policy facade without detect/event or control APIs |
| `src/orchestrator-events.ts` | Lightweight orchestrator event/timeline facade without status/session/policy or control APIs |
| `src/orchestrator-control.ts` | Lightweight orchestrator toggle/policy-update/adapter-control facade without read APIs |
| `src/fork.ts` | Lightweight hand-written conversation fork root, branch, list, and delete slice |
| `src/fork-read.ts` | Lightweight conversation fork root/get/list facade without create/branch/delete APIs |
| `src/fork-root.ts` | Lightweight conversation fork root/get facade without list or mutation APIs |
| `src/fork-list.ts` | Lightweight conversation fork list-only facade without root/get or mutation APIs |
| `src/fork-control.ts` | Lightweight conversation fork create/branch/delete facade without read APIs |
| `src/scheduler.ts` | Lightweight hand-written prompt scheduler job list/add/remove slice |
| `src/scheduler-read.ts` | Lightweight prompt scheduler job list facade without add/remove APIs |
| `src/scheduler-control.ts` | Lightweight prompt scheduler add/remove facade without list APIs |
| `src/upload.ts` | Lightweight hand-written authenticated multipart upload and parsed-file metadata slice |
| `src/graph.ts` | Lightweight hand-written knowledge graph entity/relation/context/stats slice |
| `src/graph-read.ts` | Lightweight knowledge graph entities/relations/context/stats facade without write APIs |
| `src/graph-entities.ts` | Lightweight knowledge graph entity query facade without relations, context, stats or write APIs |
| `src/graph-relations.ts` | Lightweight knowledge graph relation query facade without entities, context, stats or write APIs |
| `src/graph-context.ts` | Lightweight knowledge graph context lookup facade without entity/relation list, stats or write APIs |
| `src/graph-stats.ts` | Lightweight knowledge graph stats-only facade without entity/relation/context or write APIs |
| `src/graph-write.ts` | Lightweight knowledge graph entity/relation write facade without read APIs |
| `src/plugin-api.ts` | Lightweight hand-written plugin runtime LLM/search/memory/knowledge/cron/extensions bridge slice |
| `src/plugin-llm.ts` | Lightweight plugin LLM completion facade without plugin search, memory, knowledge, cron or extension APIs |
| `src/plugin-search.ts` | Lightweight plugin runtime search facade without LLM, memory, knowledge, cron or extension APIs |
| `src/plugin-memory.ts` | Lightweight plugin KV memory facade without LLM, search, agent memory, knowledge, cron or extension APIs |
| `src/plugin-memory-read.ts` | Lightweight plugin KV memory get/list/search facade without set/delete or other plugin APIs |
| `src/plugin-memory-write.ts` | Lightweight plugin KV memory set/delete facade without get/list/search or other plugin APIs |
| `src/plugin-agent-memory.ts` | Lightweight plugin access to host Agent memory search/add without plugin KV, knowledge, cron or extension APIs |
| `src/plugin-agent-memory-search.ts` | Lightweight plugin access to host Agent memory search without add or other plugin APIs |
| `src/plugin-agent-memory-write.ts` | Lightweight plugin access to host Agent memory add without search or other plugin APIs |
| `src/plugin-knowledge.ts` | Lightweight plugin knowledge search/ingest facade without LLM, memory or cron APIs |
| `src/plugin-knowledge-search.ts` | Lightweight plugin knowledge search facade without ingest or other plugin APIs |
| `src/plugin-knowledge-ingest.ts` | Lightweight plugin knowledge ingest facade without search or other plugin APIs |
| `src/plugin-cron.ts` | Lightweight plugin cron add/remove/list facade without LLM, memory, knowledge or extension APIs |
| `src/plugin-cron-read.ts` | Lightweight plugin cron list facade without add/remove or other plugin APIs |
| `src/plugin-cron-control.ts` | Lightweight plugin cron add/remove facade without list or other plugin APIs |
| `src/plugin-send.ts` | Lightweight plugin message send facade without LLM, memory, knowledge, cron or extension APIs |
| `src/plugin-extensions.ts` | Lightweight plugin extension register/list facade without LLM, memory, knowledge or cron APIs |
| `src/plugin-extensions-list.ts` | Lightweight plugin extension list facade without registration or other plugin APIs |
| `src/plugin-extension-register.ts` | Lightweight plugin extension registration facade without list or other plugin APIs |
| `src/state.ts` | Lightweight hand-written state kernel snapshot, goals, focus, and resources slice |
| `src/state-snapshot.ts` | Lightweight typed state snapshot facade without goal/resource/focus mutation APIs |
| `src/state-actions.ts` | Lightweight recent state actions facade without state mutation APIs |
| `src/state-capabilities.ts` | Lightweight state capabilities facade without state mutation APIs |
| `src/resource-state.ts` | Lightweight state resource list/track/release facade without full SDK import |
| `src/focus-state.ts` | Lightweight state focus read/update facade without full SDK import |
| `src/goal-state.ts` | Lightweight state goal list/save/delete facade without full SDK import |
| `src/triggers.ts` | Lightweight hand-written legacy and v2 trigger CRUD, emit, runs, and events slice |
| `src/triggers-legacy.ts` | Lightweight legacy trigger list/get/create/delete/emit facade without v2 APIs |
| `src/triggers-read.ts` | Lightweight v2 trigger list/get/runs/events facade without mutation APIs |
| `src/triggers-control.ts` | Lightweight v2 trigger create/update/delete/emit facade without read/history APIs |
| `src/trigger-definitions.ts` | Lightweight v2 trigger definition list/get facade without mutation, history or emit APIs |
| `src/trigger-definition-control.ts` | Lightweight v2 trigger definition create/update/delete facade without read list, history or emit APIs |
| `src/trigger-history.ts` | Lightweight v2 trigger runs/events facade without definition CRUD or emit APIs |
| `src/trigger-emit.ts` | Lightweight v2 trigger event emit facade without definition CRUD or history APIs |
| `src/missions.ts` | Lightweight hand-written mission parsing and reflection experiences/strategies slice |
| `src/missions-parse.ts` | Lightweight mission natural-language parse facade without reflection APIs |
| `src/reflect.ts` | Lightweight reflect-only facade over experiences, stats, and strategy context |
| `src/reflect-experiences.ts` | Lightweight reflection experience list/stats facade without mission parse or strategy APIs |
| `src/reflect-strategies.ts` | Lightweight reflection strategy context facade without experience list/stats APIs |
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

## Memory Kernel 宿主回忆记忆切片

前端页面、插件 UI 或 Node.js 自动化可以用 `yunque-client/memory-search`、`memory-add`、`memory-stats` 访问宿主 `/v1/memory/*` 回忆记忆层。它不同于 `plugin-memory` 的插件私有 KV。

```ts
import { createMemorySearchClient } from "yunque-client/memory-search";
import { createMemoryAddClient } from "yunque-client/memory-add";

const options = { baseUrl: "http://localhost:9090", token: "<jwt-or-plugin-token>" };
const found = await createMemorySearchClient(options).search("用户偏好", { limit: 3 });
const added = await createMemoryAddClient(options).remember("用户偏好中文回复", { layer: "mid", source: "sdk" });
console.log(found.count, added.status);
```

## Knowledge Graph 知识图谱切片

前端页面、插件 UI 或 Node.js 自动化可以用 `yunque-client/graph-read`、`graph-entities`、`graph-relations`、`graph-context`、`graph-stats` 访问宿主 `/v1/graph/*` 知识图谱层。

```ts
import { createGraphReadClient } from "yunque-client/graph-read";
import { createGraphClient } from "yunque-client/graph";

const options = { baseUrl: "http://localhost:9090", token: "<jwt-or-plugin-token>" };
const graph = createGraphClient(options);
const entities = await createGraphReadClient(options).entities("云雀");
const entity = await graph.putEntity({ name: "云雀", type: "agent" });
const context = await graph.contextByEntityId(entity.id!);
console.log(entities.entities.length, context.context);
```

## Knowledge Base 宿主 RAG 知识库切片

前端页面、插件 UI 或 Node.js 自动化可以用 `yunque-client/knowledge-search`、`knowledge-sources`、`knowledge-ingest`、`knowledge-import` 访问宿主 `/v1/knowledge/*` RAG 知识库。它不同于 `plugin-knowledge` 的插件运行时 helper。

```ts
import { createKnowledgeSearchClient } from "yunque-client/knowledge-search";
import { createKnowledgeIngestClient } from "yunque-client/knowledge-ingest";

const options = { baseUrl: "http://localhost:9090", token: "<jwt-or-plugin-token>" };
const found = await createKnowledgeSearchClient(options).search("增量 SDK", { limit: 3 });
const ingested = await createKnowledgeIngestClient(options).ingestText("外部项目可直接调用 Knowledge Base", { name: "sdk-note" });
console.log(found.count, ingested.source?.id);
```


Agent Kit also exposes `kit.workflows` for Workflow definition list/get/save/delete, run, instances, getInstance, and cancel helpers.

Agent Kit also exposes `kit.connectors` for Connector catalog list/detail, connect/disconnect, and action execution helpers.

Agent Kit also exposes `kit.notify` for notification channel list/add/remove/toggle/test and share dispatch helpers.


### Projects SDK

Projects SDK exposes lightweight project workspace CRUD helpers (`list`, `create`, `detail`, `update`, `remove`) for external pages, plugins, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.projects` / `kit.Projects` for one-stop automation composition.


### Skill Market SDK

Skill Market SDK exposes lightweight marketplace helpers (`search`, `top`, `stats`) for external pages, plugins, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.market` / `kit.Market` for skill discovery inside one-stop automation composition.


### Dispatch SDK

Dispatch SDK exposes lightweight MCP worker and queue helpers (`workers`, `worker`, `removeWorker`, `queue`, `enqueue`, `workerConfig`) for external pages, plugins, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.dispatch` / `kit.Dispatch` for one-stop worker orchestration.


### Orchestrator SDK

Orchestrator SDK exposes lightweight IDE worker daemon helpers (`status`, `toggle`, `sessions`, `detectIDEs`, `events`, `taskTimeline`, `policy`, `updatePolicy`, `addAdapter`) for external pages, plugins, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.orchestrator` / `kit.Orchestrator` for one-stop IDE worker orchestration.


### Providers SDK

Providers SDK exposes lightweight LLM provider and model helpers (`models`, `addModel`, `deleteModel`, `list`, `test`, `enable`, `disable`, `switchModel`, `setSession`, `mode`, `setMode`, `presets`, `register`, `delete`, `discoverLocal`, `registerLocal`, `discoverTori`, `exec`, `setExec`, `resetBreakers`) for external setup pages, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.providers` / `kit.Providers` for one-stop model configuration and runtime routing workflows.

### Cognis SDK

Cognis SDK exposes lightweight Cogni registry, trace, health, experience, evolution, workflow, bundle, and federation helpers (`list`, `create`, `traces`, `experience`, `evolve`, `federation`, `exportBundle`, `importBundle`) for external pages, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.cognis` / `kit.Cognis` for one-stop CogniKernel and multi-cogni automation workflows.

### Trace SDK

Trace SDK exposes lightweight execution/audit trace helpers (`recent`, `byTraceId`, `byTaskId`) for external debugging pages, replay tools, plugins, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.trace` / `kit.Trace` for one-stop observability and replay workflows.

### Heartbeat SDK

Heartbeat SDK exposes lightweight proactive lifecycle helpers (`status`, `update`, `trigger`, `logs`) for external operator pages, plugins, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.heartbeat` / `kit.Heartbeat` for one-stop lifecycle supervision workflows.

### Events SDK

Events SDK exposes lightweight Server-Sent Events helpers (`stream`, `parse`) for external dashboards, plugin UIs, CLIs, sidecars, and automation scripts that need live task/workflow/approval/runtime updates without importing the full platform. Agent Kit also exposes this surface as `kit.events` / `kit.Events` for one-stop live observability workflows.

### Runtime SDK

Runtime SDK exposes lightweight `/v1/sessions/queue` and `/v1/events/stream` helpers (`queues`, `sessionQueue`, `cancelQueuedTask`, `events`) for external runtime dashboards, plugin UIs, CLIs, sidecars, and automation monitors without importing the full platform. Agent Kit also exposes this surface as `kit.runtime` / `kit.Runtime`.

### Subagents SDK

Subagents SDK exposes lightweight `/v1/subagent` and `/v1/subagent/message` helpers (`list`, `get`, `spawn`, `destroy`, `appendMessages`) for external operator pages, plugin UIs, CLIs, sidecars, and automation scripts to orchestrate specialist agents without importing the full platform. Agent Kit also exposes this surface as `kit.subagents` / `kit.Subagents`.

### Tools SDK

Tools SDK exposes lightweight `/v1/tools/*` helpers (`exec`, `list`, `poll`, `kill`) for external operator pages, plugin UIs, CLIs, sidecars, and automation scripts to observe and control server-side tool process sessions through the existing authenticated guardrails. Agent Kit also exposes this surface as `kit.tools` / `kit.Tools`.

### Audit SDK

Audit SDK exposes lightweight `/v1/audit/*` and `/api/audit/trail` helpers (`tail`, `verify`, `stats`, `trail`) for external compliance pages, plugin UIs, CLIs, sidecars, and automation scripts to inspect audit-chain integrity and task audit trails without importing the full platform. Agent Kit also exposes this surface as `kit.audit` / `kit.Audit`.

### Trust SDK

Trust SDK exposes lightweight `/api/trust/*`, `/api/review/status`, and `/api/skillgrow/patterns` helpers (`scores`, `reset`, `grant`, `grantAll`, `reviewStatus`, `skillGrowPatterns`) for external admin pages, plugin UIs, CLIs, sidecars, and automation scripts to inspect and operate trust governance without importing the full platform. Agent Kit also exposes this surface as `kit.trust` / `kit.Trust`.

### Reverie SDK

Reverie SDK exposes lightweight proactive thought-loop helpers (`journal`, `stats`, `config`, `updateConfig`, `think`, `deleteThought`, `actions`, `targets`) for external operator pages, plugins, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.reverie` / `kit.Reverie` for one-stop proactive reflection and delivery workflows.

### Chat SDK

Chat SDK exposes lightweight `/v1/chat`, `/v1/chat/stream`, and `/v1/chat/agentic` helpers (`send`, `stream`, `agentic`, `parseStream`) for external chat panes, plugin UIs, CLIs, sidecars, desktop widgets, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.chat` / `kit.Chat`.

### Conversations SDK

Conversations SDK exposes lightweight `/v1/conversations` helpers (`list`, `messages`, `deleteMessages`, `manage`, `rename`, `pin`, `archive`, `replay`) for external chat panes, audit/replay tools, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.conversations` / `kit.Conversations`.

### Realtime SDK

Realtime SDK exposes lightweight `/v1/ws` helpers (`wsUrl`, `connect`, `ping`, `chat`, `send`/`serialize`, `parse`) for external chat panes, desktop widgets, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.realtime` / `kit.Realtime`.

### Cost SDK

Cost SDK exposes lightweight cost governance helpers (`summary`, `setBudget`, `task`, `taskTimeline`, `breakdown`, `history`, `alerts`, `usage`, `setQuota`) for external pages, plugins, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.cost` / `kit.Cost` for one-stop budget, usage, quota, and cost observability workflows.

### Fork SDK

Fork SDK exposes lightweight conversation branch helpers (`root`, `get`, `create`, `remove`, `branch`, `list`) for external pages, plugins, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.fork` / `kit.Fork` for one-stop conversation exploration and rollback-safe alternate-path workflows.

### Approvals SDK

Approvals SDK exposes lightweight `/v1/approvals` helpers (`list`, `pending`, `history`, `approve`, `deny`, `decide`, `rules`, `addRule`, `deleteRule`) for external approval desks, plugin UIs, CLIs, sidecars, and automation guard scripts without importing the full platform. Agent Kit also exposes this surface as `kit.approvals` / `kit.Approvals`.

### RBAC SDK

RBAC SDK exposes lightweight `/v1/rbac` helpers (`roles`, `createRole`, `deleteRole`, `assignRole`, `revokeRole`, `check`, `myRoles`) for external admin pages, plugin UIs, CLIs, sidecars, and automation guard scripts without importing the full platform. Agent Kit also exposes this surface as `kit.rbac` / `kit.RBAC`.

### Files SDK

Files SDK exposes lightweight `/api/files` helpers (`list`, `preview`, `download`) for external artifact panes, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.files` / `kit.Files`.

### Browser SDK

Browser SDK exposes lightweight `/v1/browser` and `/api/browser/ext` helpers (`status`, `config`, `navigate`, `screenshot`, `latestScreenshot`, `ocr`, `oppPending`, `oppDecide`, `extensionStatus`, `extensionSession`, `extensionAction`, `scenarios`, `runScenario`) for external browser task panes, plugin UIs, extension bridges, CLIs, sidecars, and automation scripts without importing the full platform. Agent Kit also exposes this surface as `kit.browser` / `kit.Browser`.

### Iterate SDK

Use the lightweight Iterate SDK for self-iteration proposal review from admin pages, plugins, CLIs, or automation scripts without importing the full platform client. It wraps `GET /api/iterate/proposals`, `POST /api/iterate/approve`, `POST /api/iterate/reject`, `POST /api/iterate/trigger`, and `GET /api/iterate/status`; `createAgentKit(...).iterate.pendingProposals()` exposes the same proposal review surface.
### Persona SDK

Use the lightweight Persona SDK when an external page, plugin UI, CLI, or automation script needs to read or adjust persona identity without importing the full platform client. `createPersonaClient` wraps `GET/PUT /v1/persona`, persona skills, persona presets, custom presets, and preset feature flags; `createAgentKit(...).persona` exposes the same persona identity, skills, and presets surface.




### Tasks SDK

The lightweight Tasks SDK exposes task CRUD and lifecycle helpers for external plugin UIs, front-end task pages, CLIs, sidecars, and automation scripts. Use it to list, read, create, run, pause, resume, restart, cancel, and delete `/v1/tasks` records, plus list/get/create/delete/instantiate task templates, inspect/resolve task gaps, read task working memory, interact with task threads, and inspect task trace events, without importing the full platform client or coupling to the backend console.
### Permissions SDK

The lightweight Permissions SDK exposes permission checks and current-role reads for external plugin UIs, front-end pages, CLIs, sidecars, and automation guard scripts. Use it to call `/v1/rbac/check` and `/v1/rbac/my-roles` without pulling in the broader RBAC governance client.

### Reactions SDK

The lightweight Reactions SDK exposes emoji reactions and sticker sending for external plugin UIs, front-end pages, CLIs, and automation scripts. Use it to call `/v1/react` and `/v1/sticker/send` without pulling in the full platform backend.

### Instructions SDK

The lightweight Instructions SDK exposes user instructions and instruction CRUD for external plugin UIs, front-end admin pages, CLIs, and automation scripts. Use it to list, create, update, delete, and reorder `/v1/instructions` records without pulling in the full platform backend.

### Emotion SDK

The lightweight Emotion SDK exposes emotion history and emotion stickers for external plugin UIs, front-end admin pages, CLIs, and automation scripts. Use it to read `/v1/emotion/history`, export `/v1/emotion/stickers`, register sticker mappings, or clear stale mappings without pulling in the full platform backend.




### Setup SDK

The lightweight Setup SDK exposes first-run setup detection, provider health, setup templates, provider connectivity testing, template apply, and optional component installation helpers for external setup pages, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/v1/setup/detect`, `/v1/setup/health`, `/v1/setup/templates`, `/v1/setup/test-provider`, `/v1/setup/apply`, and `/v1/setup/install-component`.

### Speech SDK

The lightweight Speech SDK exposes speech TTS, speech STT, voice/provider listing, STT stream URL construction, and file upload helpers for external voice UIs, desktop widgets, plugin pages, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/v1/speech/tts`, `/v1/speech/stt`, `/v1/speech/voices`, `/v1/speech/stt/stream`, and `/v1/upload`.

### Tori SDK

The lightweight Tori SDK exposes Tori account bind/status/unbind, bound-instance health, and usage-summary helpers for external setup pages, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/v1/tori/bind`, `/v1/tori/status`, `/v1/tori/unbind`, `/v1/tori/health`, and `/v1/tori/usage`.

### Backup SDK

The lightweight Backup SDK exposes backup archive info, ZIP export, and ZIP import/restore helpers for external operator UIs, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/v1/backup/info`, `/v1/backup/export`, and `/v1/backup/import`.

### Settings SDK

The lightweight Settings SDK exposes settings schema/config reads, runtime config updates, setup checks, hot config reload, and host directory detection for external setup pages, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/api/settings/schema`, `/api/settings/config`, `/api/settings/check`, `/v1/config/reload`, and `/api/settings/detect-dirs`.

### System SDK

The lightweight System SDK exposes public health/readiness probes, version/SBOM metadata, authenticated system info/stats, metrics, cache stats, and module observability for deployment monitors, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/healthz`, `/livez`, `/readyz`, `/healthz/cognitive`, `/v1/version`, `/v1/system/info`, `/v1/system/stats`, `/v1/metrics`, `/v1/metrics/prometheus`, `/v1/cache/stats`, `/v1/modules`, and `/sbom`.

### Auth SDK

The lightweight Auth SDK exposes auth status, password login/setup, API-key-to-JWT token exchange, and Tori OAuth start URL helpers for external plugin UIs, front-end setup pages, CLIs, sidecars, and automation scripts without importing the full platform client. It maps directly to `/v1/auth/status`, `/v1/auth/login`, `/v1/auth/set-password`, `/v1/token`, and `/v1/auth/oauth/tori`.

### Admin SDK

The lightweight Admin SDK exposes operator controls for external admin pages, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform client. It covers desktop console/autostart status and toggles, tenant listing/creation, and natural-language configuration via `/v1/desktop/console`, `/v1/desktop/autostart`, `/v1/tenants`, `/v1/nl-config`, and `/v1/nl-config/translate`. Agent Kit also exposes this surface as `kit.admin` / `kit.Admin` for one-stop operator automation.

### Federation SDK

The lightweight Federation SDK exposes model-aware A2A federation helpers for external operator pages, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform client. It covers legacy peer/stat reads, OPP capability reads and updates, peer discovery, task delegation, bridge stats, and capability broadcast through `/v1/federation/peers`, `/v1/federation/stats`, `/v1/federation/capabilities`, `/v1/federation/discover`, `/v1/federation/delegate`, `/v1/federation/bridge/stats`, and `/v1/federation/broadcast`. Agent Kit also exposes this surface as `kit.federation` / `kit.Federation` for one-stop A2A federation automation.

### Planner SDK

The lightweight Planner SDK exposes Planner Recovery helpers for external task recovery pages, plugin UIs, CLIs, sidecars, and automation scripts without importing the full platform client. It covers checkpoint listing, recovery prompt generation, task resume, direct resume-plan execution, async resume-plan job lookup, and execution-state inspection through `/v1/planner/checkpoints`, `/v1/planner/checkpoints/recover`, `/v1/planner/checkpoints/resume`, `/v1/planner/checkpoints/resume-plan`, `/v1/planner/checkpoints/resume-plan/jobs`, and `/v1/planner/execution-state`. Agent Kit also exposes this surface as `kit.planner` / `kit.Planner` for one-stop recovery automation.
