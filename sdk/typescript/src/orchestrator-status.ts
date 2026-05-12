/** Lightweight orchestrator-status SDK facade over the Orchestrator slice. */
import {
  OrchestratorClient,
  OrchestratorClientError,
  createOrchestratorClient,
  type OrchestratorClientOptions,
  type OrchestratorPolicy,
  type OrchestratorSession,
  type OrchestratorSessionsResponse,
  type OrchestratorStatusResponse,
} from "./orchestrator.js";

export type {
  OrchestratorClientOptions as OrchestratorStatusClientOptions,
  OrchestratorPolicy,
  OrchestratorSession,
  OrchestratorSessionsResponse,
  OrchestratorStatusResponse,
};

export { OrchestratorClientError as OrchestratorStatusClientError };

export class OrchestratorStatusClient {
  private readonly client: OrchestratorClient;
  constructor(options: OrchestratorClientOptions) { this.client = createOrchestratorClient(options); }
  status(): Promise<OrchestratorStatusResponse> { return this.client.status(); }
  sessions(): Promise<OrchestratorSessionsResponse> { return this.client.sessions(); }
  policy(): Promise<OrchestratorPolicy> { return this.client.policy(); }
}

export function createOrchestratorStatusClient(options: OrchestratorClientOptions): OrchestratorStatusClient { return new OrchestratorStatusClient(options); }
