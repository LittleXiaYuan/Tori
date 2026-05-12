/** Lightweight fork-read SDK facade over the Fork slice. */
import {
  ForkClient,
  ForkClientError,
  createForkClient,
  type ConversationFork,
  type ForkClientOptions,
  type ForkListResponse,
  type ForkMessage,
  type ForkRootResponse,
} from "./fork.js";

export type {
  ConversationFork,
  ForkClientOptions as ForkReadClientOptions,
  ForkListResponse,
  ForkMessage,
  ForkRootResponse,
};

export { ForkClientError as ForkReadClientError };

export class ForkReadClient {
  private readonly client: ForkClient;
  constructor(options: ForkClientOptions) { this.client = createForkClient(options); }
  root(sessionId: string): Promise<ForkRootResponse> { return this.client.root(sessionId); }
  get(id: string): Promise<ConversationFork> { return this.client.get(id); }
  list(sessionId: string): Promise<ForkListResponse> { return this.client.list(sessionId); }
}

export function createForkReadClient(options: ForkClientOptions): ForkReadClient { return new ForkReadClient(options); }
