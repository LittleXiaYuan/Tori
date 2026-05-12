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
  } satisfies StateClientOptions & ReflectClientOptions & MissionsParseClientOptions & SchedulerClientOptions & CronClientOptions & TriggersClientOptions & MemoryClientOptions & GraphClientOptions & KnowledgeClientOptions & LoRAClientOptions & WorkflowClientOptions & ConnectorsClientOptions & NotifyClientOptions & ProjectsClientOptions & SkillMarketClientOptions & DispatchClientOptions & OrchestratorClientOptions;

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
    plugin: createPluginApiClient(pluginOptions),
  };
}

export { createMissionsParseClient, createPluginApiClient, createCronClient, createReflectClient, createSchedulerClient, createStateClient, createTriggersClient, createMemoryClient, createGraphClient, createKnowledgeClient, createLoRAClient, createWorkflowClient, createConnectorsClient, createNotifyClient, createProjectsClient, createSkillMarketClient, createDispatchClient, createOrchestratorClient };
export type { MissionsParseClient, MissionsParseClientOptions, PluginApiClient, PluginApiClientOptions, ReflectClient, ReflectClientOptions, CronClient, CronClientOptions, SchedulerClient, SchedulerClientOptions, StateClient, StateClientOptions, TriggersClient, TriggersClientOptions, MemoryClient, MemoryClientOptions, GraphClient, GraphClientOptions, KnowledgeClient, KnowledgeClientOptions, LoRAClient, LoRAClientOptions, WorkflowClient, WorkflowClientOptions, ConnectorsClient, ConnectorsClientOptions, NotifyClient, NotifyClientOptions, ProjectsClient, ProjectsClientOptions, SkillMarketClient, SkillMarketClientOptions, DispatchClient, DispatchClientOptions, OrchestratorClient, OrchestratorClientOptions };

