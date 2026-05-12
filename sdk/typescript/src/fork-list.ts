/** Lightweight fork-list SDK facade over the Fork slice. */
import {
  ForkClient,
  ForkClientError,
  createForkClient,
  type ConversationFork,
  type ForkClientOptions,
  type ForkListResponse,
  type ForkMessage,
} from "./fork.js";

export type {
  ConversationFork,
  ForkClientOptions as ForkListClientOptions,
  ForkListResponse,
  ForkMessage,
};

export { ForkClientError as ForkListClientError };

export class ForkListClient {
  private readonly client: ForkClient;
  constructor(options: ForkClientOptions) { this.client = createForkClient(options); }
  list(sessionId: string): Promise<ForkListResponse> { return this.client.list(sessionId); }
}

export function createForkListClient(options: ForkClientOptions): ForkListClient { return new ForkListClient(options); }
