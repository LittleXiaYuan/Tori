/** Lightweight ide-status SDK facade over the IDE slice. */
import {
  IDEClient,
  IDEClientError,
  createIDEClient,
  type IDEClientOptions,
  type IDEStatusResponse,
} from "./ide.js";

export type {
  IDEClientOptions as IDEStatusClientOptions,
  IDEStatusResponse,
};

export { IDEClientError as IDEStatusClientError };

export class IDEStatusClient {
  private readonly client: IDEClient;

  constructor(options: IDEClientOptions) {
    this.client = createIDEClient(options);
  }

  status(): Promise<IDEStatusResponse> {
    return this.client.status();
  }
}

export function createIDEStatusClient(options: IDEClientOptions): IDEStatusClient {
  return new IDEStatusClient(options);
}
