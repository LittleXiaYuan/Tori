/** Lightweight setup-install SDK facade over the Setup slice. */
import {
  createSetupClient,
  SetupClient,
  SetupClientError,
  type SetupClientOptions,
  type SetupInstallComponentResponse,
  type SetupInstallProgress,
} from "./setup.js";

export type {
  SetupClientOptions as SetupInstallClientOptions,
  SetupInstallComponentResponse,
  SetupInstallProgress,
};

export { SetupClientError as SetupInstallClientError };

export class SetupInstallClient {
  private readonly client: SetupClient;

  constructor(options: SetupClientOptions) {
    this.client = createSetupClient(options);
  }

  install(componentId: string): Promise<SetupInstallComponentResponse> {
    return this.client.installComponent(componentId);
  }

  stream(componentId: string): AsyncGenerator<SetupInstallProgress | string> {
    return this.client.installComponentStream(componentId);
  }
}

export function createSetupInstallClient(options: SetupClientOptions): SetupInstallClient {
  return new SetupInstallClient(options);
}
