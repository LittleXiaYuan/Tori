/** Lightweight bots-detail SDK facade over bot detail reads. */
import {
  BotsClient,
  BotsClientError,
  createBotsClient,
  type Bot,
  type BotConfig,
  type BotsClientOptions,
  type BotStatus,
} from "./bots.js";

export type {
  Bot,
  BotConfig,
  BotsClientOptions as BotsDetailClientOptions,
  BotStatus,
};

export { BotsClientError as BotsDetailClientError };

export class BotsDetailClient {
  private readonly client: BotsClient;

  constructor(options: BotsClientOptions) { this.client = createBotsClient(options); }
  get(id: string): Promise<Bot> { return this.client.get(id); }
}

export function createBotsDetailClient(options: BotsClientOptions): BotsDetailClient { return new BotsDetailClient(options); }
