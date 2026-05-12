/** Lightweight planner-resume SDK facade over checkpoint resume actions. */
import {
  createPlannerRecoveryClient,
  PlannerRecoveryClient,
  PlannerRecoveryError,
  type CheckpointRecoveryAction,
  type PlannerRecoveryClientOptions,
  type ResumePlannerCheckpointPlanResponse,
  type ResumePlannerCheckpointTaskResponse,
} from "./planner-recovery.js";

export type {
  CheckpointRecoveryAction,
  PlannerRecoveryClientOptions as PlannerResumeClientOptions,
  ResumePlannerCheckpointPlanResponse,
  ResumePlannerCheckpointTaskResponse,
};

export { PlannerRecoveryError as PlannerResumeClientError };

export class PlannerResumeClient {
  private readonly client: PlannerRecoveryClient;

  constructor(options: PlannerRecoveryClientOptions) { this.client = createPlannerRecoveryClient(options); }
  task(body: { plan_id: string; action?: CheckpointRecoveryAction; run?: boolean }): Promise<ResumePlannerCheckpointTaskResponse> { return this.client.resumeCheckpointTask(body); }
  plan(body: { plan_id: string; action?: CheckpointRecoveryAction; async?: boolean }): Promise<ResumePlannerCheckpointPlanResponse> { return this.client.resumeCheckpointPlan(body); }
}

export function createPlannerResumeClient(options: PlannerRecoveryClientOptions): PlannerResumeClient { return new PlannerResumeClient(options); }
