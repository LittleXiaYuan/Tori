/** Lightweight fork-control SDK facade over the Fork slice. */
import {
  ForkClient,
  ForkClientError,
  createForkClient,
  type ConversationFork,
  type ForkBranchRequest,
  type ForkClientOptions,
  type ForkCreateRequest,
  type ForkDeleteResponse,
  type ForkMessage,
} from "./fork.js";

export type {
  ConversationFork,
  ForkBranchRequest,
  ForkClientOptions as ForkControlClientOptions,
  ForkCreateRequest,
  ForkDeleteResponse,
  ForkMessage,
};

export { ForkClientError as ForkControlClientError };

export class ForkControlClient {
  private readonly client: ForkClient;
  constructor(options: ForkClientOptions) { this.client = createForkClient(options); }
  create(request: ForkCreateRequest): Promise<ConversationFork> { return this.client.create(request); }
  branch(request: ForkBranchRequest): Promise<ConversationFork> { return this.client.branch(request); }
  remove(id: string): Promise<ForkDeleteResponse> { return this.client.remove(id); }
}

export function createForkControlClient(options: ForkClientOptions): ForkControlClient { return new ForkControlClient(options); }
