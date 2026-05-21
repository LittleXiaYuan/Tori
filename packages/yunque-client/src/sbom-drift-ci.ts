import { type SBOMDriftBaselineArtifactSourcePlan, type SBOMDriftBaselineArtifactSourcePlanRequest, type SBOMDriftClientOptions, type SBOMDriftCIWorkflowWritebackPlan, createSBOMDriftClient } from "./sbom-drift.js";

export type SBOMDriftCIWorkflowWritebackPlanRequest = {
  record_id?: string;
  request_id?: string;
  request_key?: string;
  base_id?: string;
  requested_by?: string;
  reason?: string;
  workflow_path?: string;
  job_name?: string;
};

export type { SBOMDriftCIWorkflowWritebackPlan };
export type { SBOMDriftBaselineArtifactSourcePlan, SBOMDriftBaselineArtifactSourcePlanRequest };

export function createSBOMDriftCIClient(options: SBOMDriftClientOptions) {
  const client = createSBOMDriftClient(options);
  return {
    baselineArtifactSourcePlan(input: SBOMDriftBaselineArtifactSourcePlanRequest = {}) {
      return client.baselineArtifactSourcePlan(input);
    },
    workflowWritebackPlan(input: SBOMDriftCIWorkflowWritebackPlanRequest = {}) {
      return client.ciWorkflowWritebackPlan(input);
    },
  };
}
