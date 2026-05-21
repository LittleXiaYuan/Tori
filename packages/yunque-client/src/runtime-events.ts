/** Lightweight runtime-events SDK facade over the Runtime slice. */
import {
  createRuntimeClient,
  RuntimeClient,
  RuntimeClientError,
  type RuntimeClientOptions,
  type RuntimeEvent,
} from "./runtime.js";

export type {
  RuntimeClientOptions as RuntimeEventsClientOptions,
  RuntimeEvent,
};

export { RuntimeClientError as RuntimeEventsClientError };

export class RuntimeEventsClient {
  private readonly client: RuntimeClient;

  constructor(options: RuntimeClientOptions) {
    this.client = createRuntimeClient(options);
  }

  events(): AsyncGenerator<RuntimeEvent> {
    return this.client.events();
  }
}

export function createRuntimeEventsClient(options: RuntimeClientOptions): RuntimeEventsClient {
  return new RuntimeEventsClient(options);
}
