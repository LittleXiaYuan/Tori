/** Lightweight cognis-federation SDK facade over the Cognis slice. */
import {
  CognisClient,
  CognisClientError,
  createCognisClient,
  type CogniMutationResponse,
  type CognisClientOptions,
} from "./cognis.js";

export type {
  CogniMutationResponse,
  CognisClientOptions as CognisFederationClientOptions,
};

export { CognisClientError as CognisFederationClientError };

export class CognisFederationClient {
  private readonly client: CognisClient;

  constructor(options: CognisClientOptions) {
    this.client = createCognisClient(options);
  }

  status(): Promise<Record<string, unknown>> {
    return this.client.federation();
  }

  peers(): Promise<Record<string, unknown>> {
    return this.client.federationPeers();
  }

  discover(request: Record<string, unknown> = {}): Promise<Record<string, unknown>> {
    return this.client.discoverFederation(request);
  }

  expose(id: string): Promise<CogniMutationResponse> {
    return this.client.expose(id);
  }

  unexpose(id: string): Promise<CogniMutationResponse> {
    return this.client.unexpose(id);
  }

  economics(): Promise<Record<string, unknown>> {
    return this.client.economics();
  }
}

export function createCognisFederationClient(options: CognisClientOptions): CognisFederationClient {
  return new CognisFederationClient(options);
}
