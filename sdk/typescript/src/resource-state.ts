/** Lightweight resource-state SDK facade over the State slice. */
import {
  createStateClient,
  StateClient,
  StateClientError,
  type StateClientOptions,
  type StateResource,
  type StateResourceMutationResponse,
  type StateResourcesResponse,
} from "./state.js";

export type {
  StateClientOptions as ResourceStateClientOptions,
  StateResource,
  StateResourceMutationResponse,
  StateResourcesResponse,
};

export { StateClientError as ResourceStateClientError };

export class ResourceStateClient {
  private readonly client: StateClient;

  constructor(options: StateClientOptions) {
    this.client = createStateClient(options);
  }

  list(): Promise<StateResourcesResponse> {
    return this.client.resources();
  }

  track(resource: StateResource): Promise<StateResourceMutationResponse> {
    return this.client.trackResource(resource);
  }

  release(id: string): Promise<StateResourceMutationResponse> {
    return this.client.releaseResource(id);
  }
}

export function createResourceStateClient(options: StateClientOptions): ResourceStateClient {
  return new ResourceStateClient(options);
}
