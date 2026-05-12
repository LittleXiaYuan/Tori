/** Lightweight cognis-observe SDK facade over the Cognis slice. */
import {
  CognisClient,
  CognisClientError,
  createCognisClient,
  type CogniAlertsResponse,
  type CogniHealthResponse,
  type CogniStatsResponse,
  type CogniTraceResponse,
  type CogniVerifyResponse,
  type CognisClientOptions,
} from "./cognis.js";

export type {
  CogniAlertsResponse,
  CogniHealthResponse,
  CogniStatsResponse,
  CogniTraceResponse,
  CogniVerifyResponse,
  CognisClientOptions as CognisObserveClientOptions,
};

export { CognisClientError as CognisObserveClientError };

export class CognisObserveClient {
  private readonly client: CognisClient;

  constructor(options: CognisClientOptions) {
    this.client = createCognisClient(options);
  }

  traces(limit?: number): Promise<CogniTraceResponse> { return this.client.traces(limit); }
  trace(id: string, limit?: number): Promise<CogniTraceResponse> { return this.client.trace(id, limit); }
  stats(): Promise<CogniStatsResponse> { return this.client.stats(); }
  health(id?: string): Promise<CogniHealthResponse> { return this.client.health(id); }
  verify(id?: string): Promise<CogniVerifyResponse> { return this.client.verify(id); }
  alerts(): Promise<CogniAlertsResponse> { return this.client.alerts(); }
  scanAlerts(): Promise<CogniAlertsResponse> { return this.client.scanAlerts(); }
}

export function createCognisObserveClient(options: CognisClientOptions): CognisObserveClient {
  return new CognisObserveClient(options);
}
