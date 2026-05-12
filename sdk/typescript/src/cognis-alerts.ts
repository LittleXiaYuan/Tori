/** Lightweight cognis-alerts SDK facade over the Cognis slice. */
import {
  CognisClient,
  CognisClientError,
  createCognisClient,
  type CogniAlertsResponse,
  type CognisClientOptions,
} from "./cognis.js";

export type {
  CogniAlertsResponse,
  CognisClientOptions as CognisAlertsClientOptions,
};

export { CognisClientError as CognisAlertsClientError };

export class CognisAlertsClient {
  private readonly client: CognisClient;

  constructor(options: CognisClientOptions) { this.client = createCognisClient(options); }
  list(): Promise<CogniAlertsResponse> { return this.client.alerts(); }
  scan(): Promise<CogniAlertsResponse> { return this.client.scanAlerts(); }
}

export function createCognisAlertsClient(options: CognisClientOptions): CognisAlertsClient { return new CognisAlertsClient(options); }
