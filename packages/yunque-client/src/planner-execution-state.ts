/** Lightweight planner-execution-state SDK facade over recovery execution state. */
import {
  createPlannerRecoveryClient,
  PlannerRecoveryClient,
  PlannerRecoveryError,
  type CheckpointRecoveryAction,
  type GetPlannerExecutionStateResponse,
  type GetPlannerResumePlanJobResponse,
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
  PlannerFailureSummary,
  PlannerRecoveryClientOptions as PlannerExecutionStateClientOptions,
  PlannerRecoveryPlan,
  PlannerResumePlanEvent,
  PlannerResumePlanJob,
  RecoveryAction,
  RecoveryNextAction,
};

export { PlannerRecoveryError as PlannerExecutionStateClientError };

export class PlannerExecutionStateClient {
  private readonly client: PlannerRecoveryClient;

  constructor(options: PlannerRecoveryClientOptions) { this.client = createPlannerRecoveryClient(options); }
  get(query: { plan_id: string; action?: CheckpointRecoveryAction }): Promise<GetPlannerExecutionStateResponse> { return this.client.getExecutionState(query); }
  job(query: { job_id?: string; id?: string; plan_id?: string }): Promise<GetPlannerResumePlanJobResponse> { return this.client.getResumePlanJob(query); }
}

export function createPlannerExecutionStateClient(options: PlannerRecoveryClientOptions): PlannerExecutionStateClient { return new PlannerExecutionStateClient(options); }
