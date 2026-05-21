/** Lightweight planner-read SDK facade over checkpoint recovery and execution state. */
import {
  createPlannerRecoveryClient,
  PlannerRecoveryClient,
  PlannerRecoveryError,
  type CheckpointRecoveryAction,
  type GetPlannerExecutionStateResponse,
  type GetPlannerResumePlanJobResponse,
  type ListPlannerCheckpointsResponse,
  type PlannerCheckpoint,
  type PlannerCheckpointStep,
  type PlannerFailureSummary,
  type PlannerRecoveryClientOptions,
  type PlannerRecoveryPlan,
  type PlannerResumePlanEvent,
  type PlannerResumePlanJob,
  type RecoveryAction,
  type RecoveryNextAction,
} from "./planner-recovery.js";

export type {
  CheckpointRecoveryAction,
  GetPlannerExecutionStateResponse,
  GetPlannerResumePlanJobResponse,
  ListPlannerCheckpointsResponse,
  PlannerCheckpoint,
  PlannerCheckpointStep,
  PlannerFailureSummary,
  PlannerRecoveryClientOptions as PlannerReadClientOptions,
  PlannerRecoveryPlan,
  PlannerResumePlanEvent,
  PlannerResumePlanJob,
  RecoveryAction,
  RecoveryNextAction,
};

export { PlannerRecoveryError as PlannerReadClientError };

export class PlannerReadClient {
  private readonly client: PlannerRecoveryClient;

  constructor(options: PlannerRecoveryClientOptions) {
    this.client = createPlannerRecoveryClient(options);
  }

  listCheckpoints(query?: { limit?: number; plan_id?: string; include_snapshot?: boolean }): Promise<ListPlannerCheckpointsResponse> {
    return this.client.listCheckpoints(query);
  }

  getResumePlanJob(query: { job_id?: string; id?: string; plan_id?: string }): Promise<GetPlannerResumePlanJobResponse> {
    return this.client.getResumePlanJob(query);
  }

  getExecutionState(query: { plan_id: string; action?: CheckpointRecoveryAction }): Promise<GetPlannerExecutionStateResponse> {
    return this.client.getExecutionState(query);
  }
}

export function createPlannerReadClient(options: PlannerRecoveryClientOptions): PlannerReadClient {
  return new PlannerReadClient(options);
}
