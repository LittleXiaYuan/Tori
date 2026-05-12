/** Lightweight state-snapshot SDK facade over the State slice. */
import {
  createStateClient,
  StateClient,
  StateClientError,
  type StateClientOptions,
  type StateSnapshot,
} from "./state.js";

export type {
  StateActionRecord,
  StateCapabilities,
  StateClientOptions as StateSnapshotClientOptions,
  StateGoal,
  StateResource,
  StateSnapshot,
} from "./state.js";

export { StateClientError as StateSnapshotClientError };

export class StateSnapshotClient {
  private readonly client: StateClient;

  constructor(options: StateClientOptions) {
    this.client = createStateClient(options);
  }

  get(): Promise<StateSnapshot> {
    return this.client.snapshotTyped();
  }
}

export function createStateSnapshotClient(options: StateClientOptions): StateSnapshotClient {
  return new StateSnapshotClient(options);
}
