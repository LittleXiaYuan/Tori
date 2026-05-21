/** Lightweight setup-detect SDK facade over the Setup slice. */
import {
  createSetupClient,
  SetupClient,
  SetupClientError,
  type SetupClientOptions,
  type SetupDetectResponse,
  type SetupHealthResponse,
  type SetupProviderProbe,
} from "./setup.js";

export type {
  SetupClientOptions as SetupDetectClientOptions,
  SetupDetectResponse,
  SetupHealthResponse,
  SetupProviderProbe,
};

export { SetupClientError as SetupDetectClientError };

export class SetupDetectClient {
  private readonly client: SetupClient;

  constructor(options: SetupClientOptions) {
    this.client = createSetupClient(options);
  }

  detect(): Promise<SetupDetectResponse> {
    return this.client.detect();
  }

  health(): Promise<SetupHealthResponse> {
    return this.client.health();
  }
}

export function createSetupDetectClient(options: SetupClientOptions): SetupDetectClient {
  return new SetupDetectClient(options);
}
