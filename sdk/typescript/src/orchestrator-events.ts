/** Lightweight orchestrator-events SDK facade over the Orchestrator slice. */
import {
  OrchestratorClient,
  OrchestratorClientError,
  createOrchestratorClient,
  type OrchestratorClientOptions,
  type OrchestratorEvent,
  type OrchestratorEventsResponse,
  type OrchestratorTaskTimelineResponse,
} from "./orchestrator.js";

export type {
  OrchestratorClientOptions as OrchestratorEventsClientOptions,
  OrchestratorEvent,
  OrchestratorEventsResponse,
  OrchestratorTaskTimelineResponse,
};

export { OrchestratorClientError as OrchestratorEventsClientError };

export class OrchestratorEventsClient {
  private readonly client: OrchestratorClient;
  constructor(options: OrchestratorClientOptions) { this.client = createOrchestratorClient(options); }
  events(limit?: number): Promise<OrchestratorEventsResponse> { return this.client.events(limit); }
  taskTimeline(taskId: string): Promise<OrchestratorTaskTimelineResponse> { return this.client.taskTimeline(taskId); }
}

export function createOrchestratorEventsClient(options: OrchestratorClientOptions): OrchestratorEventsClient { return new OrchestratorEventsClient(options); }
