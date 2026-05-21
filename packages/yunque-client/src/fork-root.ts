/** Lightweight fork-root SDK facade over the Fork slice. */
import {
  ForkClient,
  ForkClientError,
  createForkClient,
  type ConversationFork,
  type ForkClientOptions,
  type ForkMessage,
  type ForkRootResponse,
} from "./fork.js";

export type {
  ConversationFork,
  ForkClientOptions as ForkRootClientOptions,
  ForkMessage,
  ForkRootResponse,
};

export { ForkClientError as ForkRootClientError };

export class ForkRootClient {
  private readonly client: ForkClient;
  constructor(options: ForkClientOptions) { this.client = createForkClient(options); }
  root(sessionId: string): Promise<ForkRootResponse> { return this.client.root(sessionId); }
  get(id: string): Promise<ConversationFork> { return this.client.get(id); }
}

export function createForkRootClient(options: ForkClientOptions): ForkRootClient { return new ForkRootClient(options); }
