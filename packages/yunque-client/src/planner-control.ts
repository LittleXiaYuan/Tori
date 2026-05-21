/** Lightweight planner-control SDK facade over checkpoint recovery actions. */
import {
  createPlannerRecoveryClient,
  PlannerRecoveryClient,
  PlannerRecoveryError,
  type CheckpointRecoveryAction,
  type PlannerRecoveryClientOptions,
  type RecoverPlannerCheckpointResponse,
  type ResumePlannerCheckpointPlanResponse,
  type ResumePlannerCheckpointTaskResponse,
} from "./planner-recovery.js";

export type {
  CheckpointRecoveryAction,
  PlannerRecoveryClientOptions as PlannerControlClientOptions,
  RecoverPlannerCheckpointResponse,
  ResumePlannerCheckpointPlanResponse,
  ResumePlannerCheckpointTaskResponse,
};

export { PlannerRecoveryError as PlannerControlClientError };

export class PlannerControlClient {
  private readonly client: PlannerRecoveryClient;

  constructor(options: PlannerRecoveryClientOptions) {
    this.client = createPlannerRecoveryClient(options);
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
}

export function createPlannerControlClient(options: PlannerRecoveryClientOptions): PlannerControlClient {
  return new PlannerControlClient(options);
}
