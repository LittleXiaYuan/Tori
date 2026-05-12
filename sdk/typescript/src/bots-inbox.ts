/** Lightweight bots-inbox SDK facade over inbox reads and mutations. */
import {
  BotsClient,
  BotsClientError,
  createBotsClient,
  type BotsClientOptions,
  type InboxAction,
  type InboxCount,
  type InboxDeleteResponse,
  type InboxItem,
  type InboxReadResponse,
  type InboxResponse,
  type PushInboxRequest,
} from "./bots.js";

export type {
  BotsClientOptions as BotsInboxClientOptions,
  InboxAction,
  InboxCount,
  InboxDeleteResponse,
  InboxItem,
  InboxReadResponse,
  InboxResponse,
  PushInboxRequest,
};

export { BotsClientError as BotsInboxClientError };

export class BotsInboxClient {
  private readonly client: BotsClient;

  constructor(options: BotsClientOptions) { this.client = createBotsClient(options); }
  list(unread?: boolean): Promise<InboxResponse> { return this.client.inbox(unread); }
  push(request: PushInboxRequest): Promise<InboxItem> { return this.client.pushInbox(request); }
  delete(id: string): Promise<InboxDeleteResponse> { return this.client.deleteInbox(id); }
  markRead(ids: string[]): Promise<InboxReadResponse> { return this.client.markInboxRead(ids); }
  markAllRead(): Promise<InboxReadResponse> { return this.client.markAllInboxRead(); }
}

export function createBotsInboxClient(options: BotsClientOptions): BotsInboxClient { return new BotsInboxClient(options); }
