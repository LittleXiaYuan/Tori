/** Lightweight setup-provider SDK facade over the Setup slice. */
import {
  createSetupClient,
  SetupClient,
  SetupClientError,
  type SetupApplyRequest,
  type SetupApplyResponse,
  type SetupClientOptions,
  type SetupProviderProbe,
  type SetupTemplate,
  type SetupTestProviderRequest,
  type SetupTestProviderResponse,
} from "./setup.js";

export type {
  SetupApplyRequest,
  SetupApplyResponse,
  SetupClientOptions as SetupProviderClientOptions,
  SetupProviderProbe,
  SetupTemplate,
  SetupTestProviderRequest,
  SetupTestProviderResponse,
};

export { SetupClientError as SetupProviderClientError };

export class SetupProviderClient {
  private readonly client: SetupClient;

  constructor(options: SetupClientOptions) {
    this.client = createSetupClient(options);
  }

  test(body: SetupTestProviderRequest): Promise<SetupTestProviderResponse> {
    return this.client.testProvider(body);
  }

  apply(body: SetupApplyRequest): Promise<SetupApplyResponse> {
    return this.client.apply(body);
  }
}

export function createSetupProviderClient(options: SetupClientOptions): SetupProviderClient {
  return new SetupProviderClient(options);
}
