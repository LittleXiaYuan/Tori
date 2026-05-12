/** Lightweight bots-read SDK facade over the Bots and Inbox slice. */
import {
  BotsClient,
  BotsClientError,
  createBotsClient,
  type Bot,
  type BotConfig,
  type BotsClientOptions,
  type BotsResponse,
  type BotStatus,
  type ChannelGroup,
  type ChannelGroupsResponse,
  type InboxCount,
  type InboxItem,
  type InboxResponse,
} from "./bots.js";

export type {
  Bot,
  BotConfig,
  BotsClientOptions as BotsReadClientOptions,
  BotsResponse,
  BotStatus,
  ChannelGroup,
  ChannelGroupsResponse,
  InboxCount,
  InboxItem,
  InboxResponse,
};

export { BotsClientError as BotsReadClientError };

export class BotsReadClient {
  private readonly client: BotsClient;

  constructor(options: BotsClientOptions) { this.client = createBotsClient(options); }
  list(): Promise<BotsResponse> { return this.client.list(); }
  get(id: string): Promise<Bot> { return this.client.get(id); }
  inbox(unread?: boolean): Promise<InboxResponse> { return this.client.inbox(unread); }
  channelGroups(type?: string): Promise<ChannelGroupsResponse> { return this.client.channelGroups(type); }
}

export function createBotsReadClient(options: BotsClientOptions): BotsReadClient { return new BotsReadClient(options); }
