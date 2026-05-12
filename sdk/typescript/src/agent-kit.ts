/** Lightweight Agent Kit SDK bundle for external scripts and plugins. */
import { createPluginApiClient, type PluginApiClient, type PluginApiClientOptions } from "./plugin-api.js";
import { createReflectClient, type ReflectClient, type ReflectClientOptions } from "./reflect.js";
import { createSchedulerClient, type SchedulerClient, type SchedulerClientOptions } from "./scheduler.js";
import { createCronClient, type CronClient, type CronClientOptions } from "./cron.js";
import { createTriggersClient, type TriggersClient, type TriggersClientOptions } from "./triggers.js";
import { createStateClient, type StateClient, type StateClientOptions } from "./state.js";
import { createMissionsParseClient, type MissionsParseClient, type MissionsParseClientOptions } from "./missions-parse.js";
import { createMemoryClient, type MemoryClient, type MemoryClientOptions } from "./memory.js";
import { createGraphClient, type GraphClient, type GraphClientOptions } from "./graph.js";
import { createKnowledgeClient, type KnowledgeClient, type KnowledgeClientOptions } from "./knowledge.js";
import { createLoRAClient, type LoRAClient, type LoRAClientOptions } from "./lora.js";
import { createWorkflowClient, type WorkflowClient, type WorkflowClientOptions } from "./workflow.js";
import { createConnectorsClient, type ConnectorsClient, type ConnectorsClientOptions } from "./connectors.js";
import { createNotifyClient, type NotifyClient, type NotifyClientOptions } from "./notify.js";
import { createProjectsClient, type ProjectsClient, type ProjectsClientOptions } from "./projects.js";
import { createSkillMarketClient, type SkillMarketClient, type SkillMarketClientOptions } from "./market.js";
import { createDispatchClient, type DispatchClient, type DispatchClientOptions } from "./dispatch.js";
import { createOrchestratorClient, type OrchestratorClient, type OrchestratorClientOptions } from "./orchestrator.js";
import { createForkClient, type ForkClient, type ForkClientOptions } from "./fork.js";
import { createCostClient, type CostClient, type CostClientOptions } from "./cost.js";
import { createProvidersClient, type ProvidersClient, type ProvidersClientOptions } from "./providers.js";
import { createCognisClient, type CognisClient, type CognisClientOptions } from "./cognis.js";
import { createTraceClient, type TraceClient, type TraceClientOptions } from "./trace.js";
import { createHeartbeatClient, type HeartbeatClient, type HeartbeatClientOptions } from "./heartbeat.js";
import { createEventsClient, type EventsClient, type EventStreamClientOptions } from "./events.js";
import { createReverieClient, type ReverieClient, type ReverieClientOptions } from "./reverie.js";
import { createRealtimeClient, type RealtimeClient, type RealtimeClientOptions } from "./realtime.js";
import { createChatClient, type ChatClient, type ChatClientOptions } from "./chat.js";
import { createConversationsClient, type ConversationsClient, type ConversationsClientOptions } from "./conversations.js";
import { createApprovalsClient, type ApprovalsClient, type ApprovalsClientOptions } from "./approvals.js";
import { createRBACClient, type RBACClient, type RBACClientOptions } from "./rbac.js";
import { createFilesClient, type FilesClient, type FilesClientOptions } from "./files.js";
import { createBrowserClient, type BrowserClient, type BrowserClientOptions } from "./browser.js";
import { createRuntimeClient, type RuntimeClient, type RuntimeClientOptions } from "./runtime.js";
import { createSubagentsClient, type SubagentsClient, type SubagentsClientOptions } from "./subagents.js";
import { createToolsClient, type ToolsClient, type ToolsClientOptions } from "./tools.js";
import { createAuditClient, type AuditClient, type AuditClientOptions } from "./audit.js";
import { createTrustClient, type TrustClient, type TrustClientOptions } from "./trust.js";
import { createIterateClient, type IterateClient, type IterateClientOptions } from "./iterate.js";
import { createPersonaClient, type PersonaClient, type PersonaClientOptions } from "./persona.js";
import { createEmotionClient, type EmotionClient, type EmotionClientOptions } from "./emotion.js";

export type AgentKitOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  pluginToken?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export type AgentKit = {
  state: StateClient;
  reflect: ReflectClient;
  missions: MissionsParseClient;
  scheduler: SchedulerClient;
  cron: CronClient;
  triggers: TriggersClient;
  memory: MemoryClient;
  graph: GraphClient;
  knowledge: KnowledgeClient;
  lora: LoRAClient;
  workflows: WorkflowClient;
  connectors: ConnectorsClient;
  notify: NotifyClient;
  projects: ProjectsClient;
  market: SkillMarketClient;
  dispatch: DispatchClient;
  orchestrator: OrchestratorClient;
  fork: ForkClient;
  cost: CostClient;
  providers: ProvidersClient;
  cognis: CognisClient;
  trace: TraceClient;
  heartbeat: HeartbeatClient;
  events: EventsClient;
  reverie: ReverieClient;
  realtime: RealtimeClient;
  chat: ChatClient;
  conversations: ConversationsClient;
  approvals: ApprovalsClient;
  rbac: RBACClient;
  files: FilesClient;
  browser: BrowserClient;
  runtime: RuntimeClient;
  subagents: SubagentsClient;
  tools: ToolsClient;
  audit: AuditClient;
  trust: TrustClient;
  iterate: IterateClient;
  persona: PersonaClient;
  emotion: EmotionClient;
  plugin: PluginApiClient;
};

function requirePluginToken(options: AgentKitOptions): string {
  const token = options.pluginToken ?? options.token;
  if (!token) throw new Error("createAgentKit requires pluginToken or token for Plugin API access");
  return token;
}

export function createAgentKit(options: AgentKitOptions): AgentKit {
  const common = {
    baseUrl: options.baseUrl,
    token: options.token,
    apiKey: options.apiKey,
    headers: options.headers,
    fetch: options.fetch,
  } satisfies StateClientOptions & ReflectClientOptions & MissionsParseClientOptions & SchedulerClientOptions & CronClientOptions & TriggersClientOptions & MemoryClientOptions & GraphClientOptions & KnowledgeClientOptions & LoRAClientOptions & WorkflowClientOptions & ConnectorsClientOptions & NotifyClientOptions & ProjectsClientOptions & SkillMarketClientOptions & DispatchClientOptions & OrchestratorClientOptions & ForkClientOptions & CostClientOptions & ProvidersClientOptions & CognisClientOptions & TraceClientOptions & HeartbeatClientOptions & EventStreamClientOptions & ReverieClientOptions & RealtimeClientOptions & ChatClientOptions & ConversationsClientOptions & ApprovalsClientOptions & RBACClientOptions & FilesClientOptions & BrowserClientOptions & RuntimeClientOptions & SubagentsClientOptions & ToolsClientOptions & AuditClientOptions & TrustClientOptions & IterateClientOptions & PersonaClientOptions & EmotionClientOptions;

  const pluginOptions: PluginApiClientOptions = {
    baseUrl: options.baseUrl,
    token: requirePluginToken(options),
    headers: options.headers,
    fetch: options.fetch,
  };

  return {
    state: createStateClient(common),
    reflect: createReflectClient(common),
    missions: createMissionsParseClient(common),
    scheduler: createSchedulerClient(common),
    cron: createCronClient(common),
    triggers: createTriggersClient(common),
    memory: createMemoryClient(common),
    graph: createGraphClient(common),
    knowledge: createKnowledgeClient(common),
    lora: createLoRAClient(common),
    workflows: createWorkflowClient(common),
    connectors: createConnectorsClient(common),
    notify: createNotifyClient(common),
    projects: createProjectsClient(common),
    market: createSkillMarketClient(common),
    dispatch: createDispatchClient(common),
    orchestrator: createOrchestratorClient(common),
    fork: createForkClient(common),
    cost: createCostClient(common),
    providers: createProvidersClient(common),
    cognis: createCognisClient(common),
    trace: createTraceClient(common),
    heartbeat: createHeartbeatClient(common),
    events: createEventsClient(common),
    reverie: createReverieClient(common),
    realtime: createRealtimeClient(common),
    chat: createChatClient(common),
    conversations: createConversationsClient(common),
    approvals: createApprovalsClient(common),
    rbac: createRBACClient(common),
    files: createFilesClient(common),
    browser: createBrowserClient(common),
    runtime: createRuntimeClient(common),
    subagents: createSubagentsClient(common),
    tools: createToolsClient(common),
    audit: createAuditClient(common),
    trust: createTrustClient(common),
    iterate: createIterateClient(common),
    persona: createPersonaClient(common),
    emotion: createEmotionClient(common),
    plugin: createPluginApiClient(pluginOptions),
  };
}

export { createMissionsParseClient, createPluginApiClient, createCronClient, createReflectClient, createSchedulerClient, createStateClient, createTriggersClient, createMemoryClient, createGraphClient, createKnowledgeClient, createLoRAClient, createWorkflowClient, createConnectorsClient, createNotifyClient, createProjectsClient, createSkillMarketClient, createDispatchClient, createOrchestratorClient, createForkClient, createCostClient, createProvidersClient, createCognisClient, createTraceClient, createHeartbeatClient, createEventsClient, createReverieClient, createRealtimeClient, createChatClient, createConversationsClient, createApprovalsClient, createRBACClient, createFilesClient, createBrowserClient, createRuntimeClient, createSubagentsClient, createToolsClient, createAuditClient, createTrustClient, createIterateClient, createPersonaClient, createEmotionClient };
export type { MissionsParseClient, MissionsParseClientOptions, PluginApiClient, PluginApiClientOptions, ReflectClient, ReflectClientOptions, CronClient, CronClientOptions, SchedulerClient, SchedulerClientOptions, StateClient, StateClientOptions, TriggersClient, TriggersClientOptions, MemoryClient, MemoryClientOptions, GraphClient, GraphClientOptions, KnowledgeClient, KnowledgeClientOptions, LoRAClient, LoRAClientOptions, WorkflowClient, WorkflowClientOptions, ConnectorsClient, ConnectorsClientOptions, NotifyClient, NotifyClientOptions, ProjectsClient, ProjectsClientOptions, SkillMarketClient, SkillMarketClientOptions, DispatchClient, DispatchClientOptions, OrchestratorClient, OrchestratorClientOptions, ForkClient, ForkClientOptions, CostClient, CostClientOptions, ProvidersClient, ProvidersClientOptions, CognisClient, CognisClientOptions, TraceClient, TraceClientOptions, HeartbeatClient, HeartbeatClientOptions, EventsClient, EventStreamClientOptions, ReverieClient, ReverieClientOptions, RealtimeClient, RealtimeClientOptions, ChatClient, ChatClientOptions, ConversationsClient, ConversationsClientOptions, ApprovalsClient, ApprovalsClientOptions, RBACClient, RBACClientOptions, FilesClient, FilesClientOptions, BrowserClient, BrowserClientOptions, RuntimeClient, RuntimeClientOptions, SubagentsClient, SubagentsClientOptions, ToolsClient, ToolsClientOptions, AuditClient, AuditClientOptions, TrustClient, TrustClientOptions, IterateClient, IterateClientOptions, PersonaClient, PersonaClientOptions, EmotionClient, EmotionClientOptions };

