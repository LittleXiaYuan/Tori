/** Lightweight Agent Kit SDK bundle for external scripts and plugins. */
import { createPluginApiClient, type PluginApiClient, type PluginApiClientOptions } from "./plugin-api.js";
import { createReflectClient, type ReflectClient, type ReflectClientOptions } from "./reflect.js";
import { createSchedulerClient, type SchedulerClient, type SchedulerClientOptions } from "./scheduler.js";
import { createCronClient, type CronClient, type CronClientOptions } from "./cron.js";
import { createTriggersClient, type TriggersClient, type TriggersClientOptions } from "./triggers.js";
import { createStateClient, type StateClient, type StateClientOptions } from "./state.js";
import { createMissionsParseClient, type MissionsParseClient, type MissionsParseClientOptions } from "./missions-parse.js";

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
  } satisfies StateClientOptions & ReflectClientOptions & MissionsParseClientOptions & SchedulerClientOptions & CronClientOptions & TriggersClientOptions;

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
    plugin: createPluginApiClient(pluginOptions),
  };
}

export { createMissionsParseClient, createPluginApiClient, createCronClient, createReflectClient, createSchedulerClient, createStateClient, createTriggersClient };
export type { MissionsParseClient, MissionsParseClientOptions, PluginApiClient, PluginApiClientOptions, ReflectClient, ReflectClientOptions, CronClient, CronClientOptions, SchedulerClient, SchedulerClientOptions, StateClient, StateClientOptions, TriggersClient, TriggersClientOptions };

