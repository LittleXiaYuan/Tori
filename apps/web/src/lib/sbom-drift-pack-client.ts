import {
  createSBOMDriftClient,
  type SBOMDriftClient,
  type SBOMDriftClientOptions,
} from "yunque-client/sbom-drift";
import type { SBOMDriftCIWorkflowWritebackPlanRequest } from "yunque-client/sbom-drift-ci";
import { createYunqueSDKClientOptions } from "./sdk-client";

// UI compatibility adapter only: SBOM Drift contracts and transport live in
// yunque-client/sbom-drift.
export * from "yunque-client/sbom-drift";

export type {
  SBOMDriftBaselineArtifactSourcePlanRequest as SBOMDriftBaselineArtifactSourceInput,
  SBOMDriftCIBaselineWritebackRequest as SBOMDriftCIBaselineWritebackInput,
  SBOMDriftClient as SBOMDriftPackClient,
  SBOMDriftEvidenceResponse as SBOMDriftEvidence,
  SBOMDriftStatusResponse as SBOMDriftStatus,
} from "yunque-client/sbom-drift";

export type SBOMDriftCIWorkflowWritebackInput = SBOMDriftCIWorkflowWritebackPlanRequest;

export function createSBOMDriftPackClient(
  options: Partial<SBOMDriftClientOptions> = {},
): SBOMDriftClient {
  return createSBOMDriftClient({
    ...createYunqueSDKClientOptions(),
    ...options,
  });
}
