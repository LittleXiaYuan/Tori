/** Lightweight planner-checkpoints SDK facade over checkpoint listing. */
import {
  createPlannerRecoveryClient,
  PlannerRecoveryClient,
  PlannerRecoveryError,
  type ListPlannerCheckpointsResponse,
  type PlannerCheckpoint,
  type PlannerCheckpointStep,
  type PlannerRecoveryClientOptions,
} from "./planner-recovery.js";

export type {
  ListPlannerCheckpointsResponse,
  PlannerCheckpoint,
  PlannerCheckpointStep,
  PlannerRecoveryClientOptions as PlannerCheckpointsClientOptions,
};

export { PlannerRecoveryError as PlannerCheckpointsClientError };

export class PlannerCheckpointsClient {
  private readonly client: PlannerRecoveryClient;

  constructor(options: PlannerRecoveryClientOptions) { this.client = createPlannerRecoveryClient(options); }
  list(query?: { limit?: number; plan_id?: string; include_snapshot?: boolean }): Promise<ListPlannerCheckpointsResponse> { return this.client.listCheckpoints(query); }
}

export function createPlannerCheckpointsClient(options: PlannerRecoveryClientOptions): PlannerCheckpointsClient { return new PlannerCheckpointsClient(options); }
