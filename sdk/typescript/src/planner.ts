/** Lightweight Planner SDK facade for checkpoint recovery and execution state. */
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
  type RecoverPlannerCheckpointResponse,
  type RecoveryAction,
  type RecoveryNextAction,
  type ResumePlannerCheckpointPlanResponse,
  type ResumePlannerCheckpointTaskResponse,
} from "./planner-recovery.js";

export type {
  CheckpointRecoveryAction,
  GetPlannerExecutionStateResponse,
  GetPlannerResumePlanJobResponse,
  ListPlannerCheckpointsResponse,
  PlannerCheckpoint,
  PlannerCheckpointStep,
  PlannerFailureSummary,
  PlannerRecoveryClientOptions as PlannerClientOptions,
  PlannerRecoveryPlan,
  PlannerResumePlanEvent,
  PlannerResumePlanJob,
  RecoverPlannerCheckpointResponse,
  RecoveryAction,
  RecoveryNextAction,
  ResumePlannerCheckpointPlanResponse,
  ResumePlannerCheckpointTaskResponse,
};

export { PlannerRecoveryError as PlannerClientError };

export class PlannerClient {
  private readonly client: PlannerRecoveryClient;

  constructor(options: PlannerRecoveryClientOptions) {
    this.client = createPlannerRecoveryClient(options);
  }

  listCheckpoints(query?: { limit?: number; plan_id?: string; include_snapshot?: boolean }): Promise<ListPlannerCheckpointsResponse> {
    return this.client.listCheckpoints(query);
  }

  recoverCheckpoint(body: { plan_id: string; action?: CheckpointRecoveryAction }): Promise<RecoverPlannerCheckpointResponse> {
    return this.client.recoverCheckpoint(body);
  }

  resumeCheckpointTask(body: { plan_id: string; action?: CheckpointRecoveryAction; run?: boolean }): Promise<ResumePlannerCheckpointTaskResponse> {
    return this.client.resumeCheckpointTask(body);
  }

  resumeCheckpointPlan(body: { plan_id: string; action?: CheckpointRecoveryAction; async?: boolean }): Promise<ResumePlannerCheckpointPlanResponse> {
    return this.client.resumeCheckpointPlan(body);
  }

  getResumePlanJob(query: { job_id?: string; id?: string; plan_id?: string }): Promise<GetPlannerResumePlanJobResponse> {
    return this.client.getResumePlanJob(query);
  }

  getExecutionState(query: { plan_id: string; action?: CheckpointRecoveryAction }): Promise<GetPlannerExecutionStateResponse> {
    return this.client.getExecutionState(query);
  }
}

export function createPlannerClient(options: PlannerRecoveryClientOptions): PlannerClient {
  return new PlannerClient(options);
}
