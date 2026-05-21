/** Lightweight setup-templates SDK facade over the Setup slice. */
import {
  createSetupClient,
  SetupClient,
  SetupClientError,
  type SetupClientOptions,
  type SetupTemplate,
  type SetupTemplatesResponse,
} from "./setup.js";

export type {
  SetupClientOptions as SetupTemplatesClientOptions,
  SetupTemplate,
  SetupTemplatesResponse,
};

export { SetupClientError as SetupTemplatesClientError };

export class SetupTemplatesClient {
  private readonly client: SetupClient;

  constructor(options: SetupClientOptions) {
    this.client = createSetupClient(options);
  }

  list(): Promise<SetupTemplatesResponse> {
    return this.client.templates();
  }
}

export function createSetupTemplatesClient(options: SetupClientOptions): SetupTemplatesClient {
  return new SetupTemplatesClient(options);
}
