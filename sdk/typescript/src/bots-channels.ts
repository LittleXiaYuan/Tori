/** Lightweight bots-channels SDK facade over bot channel group reads. */
import {
  BotsClient,
  BotsClientError,
  createBotsClient,
  type BotsClientOptions,
  type ChannelGroup,
  type ChannelGroupsResponse,
} from "./bots.js";

export type {
  BotsClientOptions as BotsChannelsClientOptions,
  ChannelGroup,
  ChannelGroupsResponse,
};

export { BotsClientError as BotsChannelsClientError };

export class BotsChannelsClient {
  private readonly client: BotsClient;

  constructor(options: BotsClientOptions) { this.client = createBotsClient(options); }
  groups(type?: string): Promise<ChannelGroupsResponse> { return this.client.channelGroups(type); }
}

export function createBotsChannelsClient(options: BotsClientOptions): BotsChannelsClient { return new BotsChannelsClient(options); }
