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
import { createSkillHubClient, type SkillHubClient, type SkillHubClientOptions } from "./skillhub.js";
import { createPluginsClient, type PluginsClient, type PluginsClientOptions } from "./plugins.js";
import { createPluginUIClient, type PluginUIClient, type PluginUIClientOptions } from "./plugin-ui.js";
import { createSkillsClient, type SkillsClient, type SkillsClientOptions } from "./skills.js";
import { createSkillsCatalogClient, type SkillsCatalogClient, type SkillsCatalogClientOptions } from "./skills-catalog.js";
import { createSkillsScanClient, type SkillsScanClient, type SkillsScanClientOptions } from "./skills-scan.js";
import { createSkillsSuggestionsClient, type SkillsSuggestionsClient, type SkillsSuggestionsClientOptions } from "./skills-suggestions.js";
import { createDispatchClient, type DispatchClient, type DispatchClientOptions } from "./dispatch.js";
import { createOrchestratorClient, type OrchestratorClient, type OrchestratorClientOptions } from "./orchestrator.js";
import { createForkClient, type ForkClient, type ForkClientOptions } from "./fork.js";
import { createCostClient, type CostClient, type CostClientOptions } from "./cost.js";
import { createProvidersClient, type ProvidersClient, type ProvidersClientOptions } from "./providers.js";
import { createBreakerClient, type BreakerClient, type BreakerClientOptions } from "./breaker.js";
import { createModelsClient, type ModelsClient, type ModelsClientOptions } from "./models.js";
import { createCognisClient, type CognisClient, type CognisClientOptions } from "./cognis.js";
import { createTraceClient, type TraceClient, type TraceClientOptions } from "./trace.js";
import { createHeartbeatClient, type HeartbeatClient, type HeartbeatClientOptions } from "./heartbeat.js";
import { createEventsClient, type EventsClient, type EventStreamClientOptions } from "./events.js";
import { createReverieClient, type ReverieClient, type ReverieClientOptions } from "./reverie.js";
import { createRealtimeClient, type RealtimeClient, type RealtimeClientOptions } from "./realtime.js";
import { createChatClient, type ChatClient, type ChatClientOptions } from "./chat.js";
import { createWebChatClient, type WebChatClient, type WebChatWidgetOptions } from "./webchat.js";
import { createConversationsClient, type ConversationsClient, type ConversationsClientOptions } from "./conversations.js";
import { createApprovalsClient, type ApprovalsClient, type ApprovalsClientOptions } from "./approvals.js";
import { createRBACClient, type RBACClient, type RBACClientOptions } from "./rbac.js";
import { createFilesClient, type FilesClient, type FilesClientOptions } from "./files.js";
import { createBrowserClient, type BrowserClient, type BrowserClientOptions } from "./browser.js";
import { createRuntimeClient, type RuntimeClient, type RuntimeClientOptions } from "./runtime.js";
import { createRuntimeQueueClient, type RuntimeQueueClient, type RuntimeQueueClientOptions } from "./runtime-queue.js";
import { createSubagentsClient, type SubagentsClient, type SubagentsClientOptions } from "./subagents.js";
import { createToolsClient, type ToolsClient, type ToolsClientOptions } from "./tools.js";
import { createSandboxClient, type SandboxClient, type SandboxClientOptions } from "./sandbox.js";
import { createAuditClient, type AuditClient, type AuditClientOptions } from "./audit.js";
import { createTrustClient, type TrustClient, type TrustClientOptions } from "./trust.js";
import { createSkillGrowClient, type SkillGrowClient, type SkillGrowClientOptions } from "./skillgrow.js";
import { createReviewClient, type ReviewClient, type ReviewClientOptions } from "./review.js";
import { createIterateClient, type IterateClient, type IterateClientOptions } from "./iterate.js";
import { createPersonaClient, type PersonaClient, type PersonaClientOptions } from "./persona.js";
import { createModesClient, type ModesClient, type ModesClientOptions } from "./modes.js";
import { createEmotionClient, type EmotionClient, type EmotionClientOptions } from "./emotion.js";
import { createInteractionsClient, type InteractionsClient, type InteractionsClientOptions } from "./interactions.js";
import { createInstructionsClient, type InstructionsClient, type InstructionsClientOptions } from "./instructions.js";
import { createReactionsClient, type ReactionsClient, type ReactionsClientOptions } from "./reactions.js";
import { createPermissionsClient, type PermissionsClient, type PermissionsClientOptions } from "./permissions.js";
import { createTasksClient, type TasksClient, type TasksClientOptions } from "./tasks.js";
import { createDocumentsClient, type DocumentsClient, type DocumentsClientOptions } from "./documents.js";
import { createBotsClient, type BotsClient, type BotsClientOptions } from "./bots.js";
import { createAuthClient, type AuthClient, type AuthClientOptions } from "./auth.js";
import { createSystemClient, type SystemClient, type SystemClientOptions } from "./system.js";
import { createSettingsClient, type SettingsClient, type SettingsClientOptions } from "./settings.js";
import { createBackupClient, type BackupClient, type BackupClientOptions } from "./backup.js";
import { createToriClient, type ToriClient, type ToriClientOptions } from "./tori.js";
import { createUploadClient, type UploadClient, type UploadClientOptions } from "./upload.js";
import { createSpeechClient, type SpeechClient, type SpeechClientOptions } from "./speech.js";
import { createSetupClient, type SetupClient, type SetupClientOptions } from "./setup.js";
import { createAdminClient, type AdminClient, type AdminClientOptions } from "./admin.js";
import { createFederationClient, type FederationClient, type FederationClientOptions } from "./federation.js";
import { createPlannerClient, type PlannerClient, type PlannerClientOptions } from "./planner.js";
import { createIDEClient, type IDEClient, type IDEClientOptions } from "./ide.js";
import { createDiscoveryClient, type DiscoveryClient, type DiscoveryClientOptions } from "./discovery.js";
import { createIdentityClient, type IdentityClient, type IdentityClientOptions } from "./identity.js";
import { createEmbeddingsClient, type EmbeddingsClient, type EmbeddingsClientOptions } from "./embeddings.js";
import { createSearchClient, type SearchClient, type SearchClientOptions } from "./search.js";
import { createRouterClient, type RouterClient, type RouterClientOptions } from "./router.js";
import { createAiriClient, type AiriClient, type AiriClientOptions } from "./airi.js";

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
  skillhub: SkillHubClient;
  plugins: PluginsClient;
  pluginUI: PluginUIClient;
  skills: SkillsClient;
  skillsCatalog: SkillsCatalogClient;
  skillsScan: SkillsScanClient;
  skillsSuggestions: SkillsSuggestionsClient;
  dispatch: DispatchClient;
  orchestrator: OrchestratorClient;
  fork: ForkClient;
  cost: CostClient;
  providers: ProvidersClient;
  breaker: BreakerClient;
  models: ModelsClient;
  cognis: CognisClient;
  trace: TraceClient;
  heartbeat: HeartbeatClient;
  events: EventsClient;
  reverie: ReverieClient;
  realtime: RealtimeClient;
  chat: ChatClient;
  webchat: WebChatClient;
  conversations: ConversationsClient;
  approvals: ApprovalsClient;
  rbac: RBACClient;
  files: FilesClient;
  browser: BrowserClient;
  runtime: RuntimeClient;
  runtimeQueue: RuntimeQueueClient;
  subagents: SubagentsClient;
  tools: ToolsClient;
  sandbox: SandboxClient;
  audit: AuditClient;
  trust: TrustClient;
  skillgrow: SkillGrowClient;
  review: ReviewClient;
  iterate: IterateClient;
  persona: PersonaClient;
  modes: ModesClient;
  emotion: EmotionClient;
  interactions: InteractionsClient;
  instructions: InstructionsClient;
  reactions: ReactionsClient;
  permissions: PermissionsClient;
  backup: BackupClient;
  tori: ToriClient;
  upload: UploadClient;
  speech: SpeechClient;
  setup: SetupClient;
  admin: AdminClient;
  federation: FederationClient;
  planner: PlannerClient;
  ide: IDEClient;
  discovery: DiscoveryClient;
  identity: IdentityClient;
  embeddings: EmbeddingsClient;
  search: SearchClient;
  router: RouterClient;
  airi: AiriClient;
  settings: SettingsClient;
  system: SystemClient;
  auth: AuthClient;
  tasks: TasksClient;
  documents: DocumentsClient;
  bots: BotsClient;
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
  };

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
    skillhub: createSkillHubClient(common),
    plugins: createPluginsClient(common),
    pluginUI: createPluginUIClient(common),
    skills: createSkillsClient(common),
    skillsCatalog: createSkillsCatalogClient(common),
    skillsScan: createSkillsScanClient(common),
    skillsSuggestions: createSkillsSuggestionsClient(common),
    dispatch: createDispatchClient(common),
    orchestrator: createOrchestratorClient(common),
    fork: createForkClient(common),
    cost: createCostClient(common),
    providers: createProvidersClient(common),
    breaker: createBreakerClient(common),
    models: createModelsClient(common),
    cognis: createCognisClient(common),
    trace: createTraceClient(common),
    heartbeat: createHeartbeatClient(common),
    events: createEventsClient(common),
    reverie: createReverieClient(common),
    realtime: createRealtimeClient(common),
    chat: createChatClient(common),
    webchat: createWebChatClient(common),
    conversations: createConversationsClient(common),
    approvals: createApprovalsClient(common),
    rbac: createRBACClient(common),
    files: createFilesClient(common),
    browser: createBrowserClient(common),
    runtime: createRuntimeClient(common),
    runtimeQueue: createRuntimeQueueClient(common),
    subagents: createSubagentsClient(common),
    tools: createToolsClient(common),
    sandbox: createSandboxClient(common),
    audit: createAuditClient(common),
    trust: createTrustClient(common),
    skillgrow: createSkillGrowClient(common),
    review: createReviewClient(common),
    iterate: createIterateClient(common),
    persona: createPersonaClient(common),
    modes: createModesClient(common),
    emotion: createEmotionClient(common),
    interactions: createInteractionsClient(common),
    instructions: createInstructionsClient(common),
    reactions: createReactionsClient(common),
    permissions: createPermissionsClient(common),
    backup: createBackupClient(common),
    tori: createToriClient(common),
    upload: createUploadClient(common),
    speech: createSpeechClient(common),
    setup: createSetupClient(common),
    admin: createAdminClient(common),
    federation: createFederationClient(common),
    planner: createPlannerClient(common),
    ide: createIDEClient(common),
    discovery: createDiscoveryClient(common),
    identity: createIdentityClient(common),
    embeddings: createEmbeddingsClient(common),
    search: createSearchClient(common),
    router: createRouterClient(common),
    airi: createAiriClient(common),
    settings: createSettingsClient(common),
    system: createSystemClient(common),
    auth: createAuthClient(common),
    tasks: createTasksClient(common),
    documents: createDocumentsClient(common),
    bots: createBotsClient(common),
    plugin: createPluginApiClient(pluginOptions),
  };
}

export { createMissionsParseClient, createPluginApiClient, createCronClient, createReflectClient, createSchedulerClient, createStateClient, createTriggersClient, createMemoryClient, createGraphClient, createKnowledgeClient, createLoRAClient, createWorkflowClient, createConnectorsClient, createNotifyClient, createProjectsClient, createSkillMarketClient, createSkillHubClient, createPluginsClient, createPluginUIClient, createSkillsClient, createSkillsCatalogClient, createSkillsScanClient, createSkillsSuggestionsClient, createDispatchClient, createOrchestratorClient, createForkClient, createCostClient, createProvidersClient, createBreakerClient, createModelsClient, createCognisClient, createTraceClient, createHeartbeatClient, createEventsClient, createReverieClient, createRealtimeClient, createChatClient, createWebChatClient, createConversationsClient, createApprovalsClient, createRBACClient, createFilesClient, createBrowserClient, createRuntimeClient, createRuntimeQueueClient, createSubagentsClient, createToolsClient, createSandboxClient, createAuditClient, createTrustClient, createSkillGrowClient, createReviewClient, createIterateClient, createPersonaClient, createModesClient, createEmotionClient, createInteractionsClient, createInstructionsClient, createReactionsClient, createPermissionsClient, createBackupClient, createToriClient, createUploadClient, createSpeechClient, createSetupClient, createAdminClient, createFederationClient, createPlannerClient, createIDEClient, createDiscoveryClient, createIdentityClient, createEmbeddingsClient, createSearchClient, createRouterClient, createAiriClient, createSettingsClient, createSystemClient, createAuthClient, createTasksClient, createDocumentsClient, createBotsClient };
export type { MissionsParseClient, MissionsParseClientOptions, PluginApiClient, PluginApiClientOptions, ReflectClient, ReflectClientOptions, CronClient, CronClientOptions, SchedulerClient, SchedulerClientOptions, StateClient, StateClientOptions, TriggersClient, TriggersClientOptions, MemoryClient, MemoryClientOptions, GraphClient, GraphClientOptions, KnowledgeClient, KnowledgeClientOptions, LoRAClient, LoRAClientOptions, WorkflowClient, WorkflowClientOptions, ConnectorsClient, ConnectorsClientOptions, NotifyClient, NotifyClientOptions, ProjectsClient, ProjectsClientOptions, SkillMarketClient, SkillMarketClientOptions, SkillHubClient, SkillHubClientOptions, PluginsClient, PluginsClientOptions, PluginUIClient, PluginUIClientOptions, SkillsClient, SkillsClientOptions, SkillsCatalogClient, SkillsCatalogClientOptions, SkillsScanClient, SkillsScanClientOptions, SkillsSuggestionsClient, SkillsSuggestionsClientOptions, DispatchClient, DispatchClientOptions, OrchestratorClient, OrchestratorClientOptions, ForkClient, ForkClientOptions, CostClient, CostClientOptions, ProvidersClient, ProvidersClientOptions, BreakerClient, BreakerClientOptions, ModelsClient, ModelsClientOptions, CognisClient, CognisClientOptions, TraceClient, TraceClientOptions, HeartbeatClient, HeartbeatClientOptions, EventsClient, EventStreamClientOptions, ReverieClient, ReverieClientOptions, RealtimeClient, RealtimeClientOptions, ChatClient, ChatClientOptions, WebChatClient, WebChatWidgetOptions, ConversationsClient, ConversationsClientOptions, ApprovalsClient, ApprovalsClientOptions, RBACClient, RBACClientOptions, FilesClient, FilesClientOptions, BrowserClient, BrowserClientOptions, RuntimeClient, RuntimeClientOptions, RuntimeQueueClient, RuntimeQueueClientOptions, SubagentsClient, SubagentsClientOptions, ToolsClient, ToolsClientOptions, SandboxClient, SandboxClientOptions, AuditClient, AuditClientOptions, TrustClient, TrustClientOptions, SkillGrowClient, SkillGrowClientOptions, ReviewClient, ReviewClientOptions, IterateClient, IterateClientOptions, PersonaClient, PersonaClientOptions, ModesClient, ModesClientOptions, EmotionClient, EmotionClientOptions, InteractionsClient, InteractionsClientOptions, InstructionsClient, InstructionsClientOptions, ReactionsClient, ReactionsClientOptions, PermissionsClient, PermissionsClientOptions, BackupClient, BackupClientOptions, ToriClient, ToriClientOptions, UploadClient, UploadClientOptions, SpeechClient, SpeechClientOptions, SetupClient, SetupClientOptions, AdminClient, AdminClientOptions, FederationClient, FederationClientOptions, PlannerClient, PlannerClientOptions, IDEClient, IDEClientOptions, DiscoveryClient, DiscoveryClientOptions, IdentityClient, IdentityClientOptions, EmbeddingsClient, EmbeddingsClientOptions, SearchClient, SearchClientOptions, RouterClient, RouterClientOptions, AiriClient, AiriClientOptions, SettingsClient, SettingsClientOptions, SystemClient, SystemClientOptions, AuthClient, AuthClientOptions, TasksClient, TasksClientOptions, DocumentsClient, DocumentsClientOptions, BotsClient, BotsClientOptions };

