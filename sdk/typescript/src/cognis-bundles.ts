/** Lightweight cognis-bundles SDK facade over the Cognis slice. */
import {
  CognisClient,
  CognisClientError,
  createCognisClient,
  type CogniMutationResponse,
  type CognisClientOptions,
} from "./cognis.js";

export type {
  CogniMutationResponse,
  CognisClientOptions as CognisBundlesClientOptions,
};

export { CognisClientError as CognisBundlesClientError };

export class CognisBundlesClient {
  private readonly client: CognisClient;

  constructor(options: CognisClientOptions) {
    this.client = createCognisClient(options);
  }

  generate(request: Record<string, unknown>): Promise<CogniMutationResponse> {
    return this.client.generate(request);
  }

  export(): Promise<Record<string, unknown>> {
    return this.client.exportBundle();
  }

  import(bundle: Record<string, unknown>): Promise<CogniMutationResponse> {
    return this.client.importBundle(bundle);
  }
}

export function createCognisBundlesClient(options: CognisClientOptions): CognisBundlesClient {
  return new CognisBundlesClient(options);
}
