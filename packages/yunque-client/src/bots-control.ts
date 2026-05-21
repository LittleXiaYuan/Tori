/** Lightweight bots-control SDK facade over the Bots and Inbox slice. */
import {
  BotsClient,
  BotsClientError,
  createBotsClient,
  type Bot,
  type BotConfig,
  type BotsClientOptions,
  type BotStatus,
  type CreateBotRequest,
  type DeleteBotResponse,
  type InboxAction,
  type InboxDeleteResponse,
  type InboxItem,
  type InboxReadResponse,
  type PushInboxRequest,
  type UpdateBotRequest,
} from "./bots.js";

export type {
  Bot,
  BotConfig,
  BotsClientOptions as BotsControlClientOptions,
  BotStatus,
  CreateBotRequest,
  DeleteBotResponse,
  InboxAction,
  InboxDeleteResponse,
  InboxItem,
  InboxReadResponse,
  PushInboxRequest,
  UpdateBotRequest,
};

export { BotsClientError as BotsControlClientError };

export class BotsControlClient {
  private readonly client: BotsClient;

  constructor(options: BotsClientOptions) { this.client = createBotsClient(options); }
  create(request: CreateBotRequest): Promise<Bot> { return this.client.create(request); }
  update(id: string, request: UpdateBotRequest): Promise<Bot> { return this.client.update(id, request); }
  setActive(id: string, active: boolean): Promise<Bot> { return this.client.setActive(id, active); }
  delete(id: string): Promise<DeleteBotResponse> { return this.client.delete(id); }
  pushInbox(request: PushInboxRequest): Promise<InboxItem> { return this.client.pushInbox(request); }
  deleteInbox(id: string): Promise<InboxDeleteResponse> { return this.client.deleteInbox(id); }
  markInboxRead(ids: string[]): Promise<InboxReadResponse> { return this.client.markInboxRead(ids); }
  markAllInboxRead(): Promise<InboxReadResponse> { return this.client.markAllInboxRead(); }
}

export function createBotsControlClient(options: BotsClientOptions): BotsControlClient { return new BotsControlClient(options); }
