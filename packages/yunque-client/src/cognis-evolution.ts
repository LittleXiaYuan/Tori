/** Lightweight cognis-evolution SDK facade over the Cognis slice. */
import {
  CognisClient,
  CognisClientError,
  createCognisClient,
  type CogniMutationResponse,
  type CognisClientOptions,
} from "./cognis.js";

export type {
  CogniMutationResponse,
  CognisClientOptions as CognisEvolutionClientOptions,
};

export { CognisClientError as CognisEvolutionClientError };

export class CognisEvolutionClient {
  private readonly client: CognisClient;

  constructor(options: CognisClientOptions) {
    this.client = createCognisClient(options);
  }

  evolve(id: string, request: Record<string, unknown> = {}): Promise<CogniMutationResponse> {
    return this.client.evolve(id, request);
  }

  status(id?: string): Promise<Record<string, unknown>> {
    return this.client.evolution(id);
  }
}

export function createCognisEvolutionClient(options: CognisClientOptions): CognisEvolutionClient {
  return new CognisEvolutionClient(options);
}
