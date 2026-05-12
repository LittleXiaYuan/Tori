/** Lightweight cognis-traces SDK facade over the Cognis slice. */
import {
  CognisClient,
  CognisClientError,
  createCognisClient,
  type CogniTraceResponse,
  type CognisClientOptions,
} from "./cognis.js";

export type {
  CogniTraceResponse,
  CognisClientOptions as CognisTracesClientOptions,
};

export { CognisClientError as CognisTracesClientError };

export class CognisTracesClient {
  private readonly client: CognisClient;

  constructor(options: CognisClientOptions) { this.client = createCognisClient(options); }
  list(limit?: number): Promise<CogniTraceResponse> { return this.client.traces(limit); }
  get(id: string, limit?: number): Promise<CogniTraceResponse> { return this.client.trace(id, limit); }
}

export function createCognisTracesClient(options: CognisClientOptions): CognisTracesClient { return new CognisTracesClient(options); }
