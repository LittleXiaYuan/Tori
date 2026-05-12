/** Lightweight cognis-health SDK facade over the Cognis slice. */
import {
  CognisClient,
  CognisClientError,
  createCognisClient,
  type CogniHealthResponse,
  type CogniStatsResponse,
  type CogniVerifyResponse,
  type CognisClientOptions,
} from "./cognis.js";

export type {
  CogniHealthResponse,
  CogniStatsResponse,
  CogniVerifyResponse,
  CognisClientOptions as CognisHealthClientOptions,
};

export { CognisClientError as CognisHealthClientError };

export class CognisHealthClient {
  private readonly client: CognisClient;

  constructor(options: CognisClientOptions) { this.client = createCognisClient(options); }
  stats(): Promise<CogniStatsResponse> { return this.client.stats(); }
  health(id?: string): Promise<CogniHealthResponse> { return this.client.health(id); }
  verify(id?: string): Promise<CogniVerifyResponse> { return this.client.verify(id); }
}

export function createCognisHealthClient(options: CognisClientOptions): CognisHealthClient { return new CognisHealthClient(options); }
