/** Lightweight state-actions SDK facade over the State slice. */
import {
  createStateClient,
  StateClient,
  StateClientError,
  type StateActionRecord,
  type StateClientOptions,
} from "./state.js";

export type {
  StateActionRecord,
  StateClientOptions as StateActionsClientOptions,
} from "./state.js";

export { StateClientError as StateActionsClientError };

export class StateActionsClient {
  private readonly client: StateClient;

  constructor(options: StateClientOptions) {
    this.client = createStateClient(options);
  }

  async list(): Promise<StateActionRecord[]> {
    const snapshot = await this.client.snapshotTyped();
    return snapshot.recent_actions ?? [];
  }
}

export function createStateActionsClient(options: StateClientOptions): StateActionsClient {
  return new StateActionsClient(options);
}
