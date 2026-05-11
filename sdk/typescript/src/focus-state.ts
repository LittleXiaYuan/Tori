/** Lightweight focus-state SDK facade over the State slice. */
import {
  createStateClient,
  StateClient,
  StateClientError,
  type StateClientOptions,
  type StateFocusResponse,
  type StateFocusUpdateResponse,
} from "./state.js";

export type {
  StateClientOptions as FocusStateClientOptions,
  StateFocusResponse,
  StateFocusUpdateResponse,
};

export { StateClientError as FocusStateClientError };

export class FocusStateClient {
  private readonly client: StateClient;

  constructor(options: StateClientOptions) {
    this.client = createStateClient(options);
  }

  get(): Promise<StateFocusResponse> {
    return this.client.focus();
  }

  update(focus?: string, topics?: string[]): Promise<StateFocusUpdateResponse> {
    return this.client.updateFocus(focus, topics);
  }
}

export function createFocusStateClient(options: StateClientOptions): FocusStateClient {
  return new FocusStateClient(options);
}
