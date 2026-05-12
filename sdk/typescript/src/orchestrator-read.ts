/** Lightweight orchestrator-read SDK facade over the Orchestrator slice. */
import {
  OrchestratorClient,
  OrchestratorClientError,
  createOrchestratorClient,
  type OrchestratorClientOptions,
  type OrchestratorDetectResponse,
  type OrchestratorEvent,
  type OrchestratorEventsResponse,
  type OrchestratorIDE,
  type OrchestratorPolicy,
  type OrchestratorSession,
  type OrchestratorSessionsResponse,
  type OrchestratorStatusResponse,
  type OrchestratorTaskTimelineResponse,
} from "./orchestrator.js";

export type {
  OrchestratorClientOptions as OrchestratorReadClientOptions,
  OrchestratorDetectResponse,
  OrchestratorEvent,
  OrchestratorEventsResponse,
  OrchestratorIDE,
  OrchestratorPolicy,
  OrchestratorSession,
  OrchestratorSessionsResponse,
  OrchestratorStatusResponse,
  OrchestratorTaskTimelineResponse,
};

export { OrchestratorClientError as OrchestratorReadClientError };

export class OrchestratorReadClient {
  private readonly client: OrchestratorClient;
  constructor(options: OrchestratorClientOptions) { this.client = createOrchestratorClient(options); }
  status(): Promise<OrchestratorStatusResponse> { return this.client.status(); }
  sessions(): Promise<OrchestratorSessionsResponse> { return this.client.sessions(); }
  detectIDEs(): Promise<OrchestratorDetectResponse> { return this.client.detectIDEs(); }
  events(limit?: number): Promise<OrchestratorEventsResponse> { return this.client.events(limit); }
  taskTimeline(taskId: string): Promise<OrchestratorTaskTimelineResponse> { return this.client.taskTimeline(taskId); }
  policy(): Promise<OrchestratorPolicy> { return this.client.policy(); }
}

export function createOrchestratorReadClient(options: OrchestratorClientOptions): OrchestratorReadClient { return new OrchestratorReadClient(options); }
