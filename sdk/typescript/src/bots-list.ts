/** Lightweight bots-list SDK facade over bot list reads. */
import {
  BotsClient,
  BotsClientError,
  createBotsClient,
  type Bot,
  type BotConfig,
  type BotsClientOptions,
  type BotsResponse,
  type BotStatus,
} from "./bots.js";

export type {
  Bot,
  BotConfig,
  BotsClientOptions as BotsListClientOptions,
  BotsResponse,
  BotStatus,
};

export { BotsClientError as BotsListClientError };

export class BotsListClient {
  private readonly client: BotsClient;

  constructor(options: BotsClientOptions) { this.client = createBotsClient(options); }
  list(): Promise<BotsResponse> { return this.client.list(); }
}

export function createBotsListClient(options: BotsClientOptions): BotsListClient { return new BotsListClient(options); }
