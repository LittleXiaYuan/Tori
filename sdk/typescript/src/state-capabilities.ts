/** Lightweight state-capabilities SDK facade over the State slice. */
import {
  createStateClient,
  StateClient,
  StateClientError,
  type StateCapabilities,
  type StateClientOptions,
} from "./state.js";

export type {
  StateCapabilities,
  StateClientOptions as StateCapabilitiesClientOptions,
} from "./state.js";

export { StateClientError as StateCapabilitiesClientError };

export class StateCapabilitiesClient {
  private readonly client: StateClient;

  constructor(options: StateClientOptions) {
    this.client = createStateClient(options);
  }

  async get(): Promise<StateCapabilities> {
    const snapshot = await this.client.snapshotTyped();
    return snapshot.capabilities ?? {};
  }
}

export function createStateCapabilitiesClient(options: StateClientOptions): StateCapabilitiesClient {
  return new StateCapabilitiesClient(options);
}
