import { fetcher } from "./api-core";

export interface SBOMDriftComponent {
  ecosystem: string;
  name: string;
  version?: string;
  scope?: string;
  path?: string;
  direct: boolean;
}

export interface SBOMDriftSnapshotSummary {
  id: string;
  source: string;
  created_at: string;
  component_count: number;
  ecosystems: Record<string, number>;
}

export interface SBOMDriftSnapshot extends SBOMDriftSnapshotSummary {
  components: SBOMDriftComponent[];
}

export interface SBOMDriftStatus {
  pack_id: string;
  stage: string;
  scanner_ready: boolean;
  cyclonedx_ready: boolean;
  ci_gate_plan_ready: boolean;
  ci_gate_ready: boolean;
  ci_baseline_store_ready?: boolean;
  ci_baseline_writeback_ready?: boolean;
  ci_workflow_writeback_plan_ready?: boolean;
  ci_workflow_writeback_ready?: boolean;
  artifact_source_plan_ready?: boolean;
  baseline_fetch_plan_ready?: boolean;
  baseline_fetch_ready?: boolean;
  artifact_baseline_ready?: boolean;
  consumes_artifact_repository?: boolean;
  fetches_artifact_baseline?: boolean;
  consumes_ci_baseline_store?: boolean;
  vulnerability_ready: boolean;
  govulncheck_plan_ready: boolean;
  govulncheck_ready: boolean;
  writes_ci_baseline_store?: boolean;
  writes_baseline_snapshot?: boolean;
  writes_ci_workflow?: boolean;
  executes_govulncheck?: boolean;
  blocks_release?: boolean;
  ci_baseline_store?: SBOMDriftCIBaselineStoreSummary;
  snapshot_count: number;
  repo_root?: string;
  store_dir?: string;
  capabilities: string[];
  notes?: string[];
}

export interface SBOMDriftCIBaselineStoreSummary {
  pack_id: string;
  store: string;
  store_ready: boolean;
  ci_baseline_store_ready: boolean;
  ci_baseline_writeback_ready: boolean;
  ci_workflow_writeback_plan_ready?: boolean;
  ci_workflow_writeback_ready?: boolean;
  ci_gate_plan_ready: boolean;
  ci_gate_ready: boolean;
  govulncheck_plan_ready: boolean;
  govulncheck_ready: boolean;
  vulnerability_ready: boolean;
  writes_ci_baseline_store: boolean;
  writes_ci_workflow: boolean;
  executes_govulncheck: boolean;
  blocks_release: boolean;
  record_count: number;
  artifact: string;
  record_artifact: string;
  notes?: string[];
}

export interface SBOMDriftCIWorkflowWritebackInput {
  record_id?: string;
  request_id?: string;
  request_key?: string;
  base_id?: string;
  requested_by?: string;
  reason?: string;
  workflow_path?: string;
  job_name?: string;
}

export interface SBOMDriftCIWorkflowStepPlan {
  step: number;
  name: string;
  run?: string;
  artifact?: string;
  required: boolean;
  writes_files: boolean;
  executes_govulncheck: boolean;
  blocks_release: boolean;
  description?: string;
}

export interface SBOMDriftCIWorkflowHandoffPlan {
  target: string;
  source_store: string;
  source_record_artifact: string;
  workflow_path: string;
  job_name: string;
  dedup_key: string;
  consumes_ci_baseline_store: boolean;
  ci_workflow_writeback_plan_ready: boolean;
  ci_workflow_writeback_ready: boolean;
  ci_gate_plan_ready: boolean;
  govulncheck_plan_ready: boolean;
  govulncheck_ready: boolean;
  writes_ci_workflow: boolean;
  executes_govulncheck: boolean;
  blocks_release: boolean;
  steps: SBOMDriftCIWorkflowStepPlan[];
  blocked_by: string[];
  notes?: string[];
}

export interface SBOMDriftReleaseBlockerPlan {
  gate_name: string;
  threshold: string;
  risk_level: string;
  would_block: boolean;
  blocks_release: boolean;
  source_record_id: string;
  source_record_artifact: string;
  blocked_by: string[];
  notes?: string[];
}

export interface SBOMDriftBaselineArtifactSourceInput {
  source_name?: string;
  provider?: string;
  artifact_url?: string;
  artifact_name?: string;
  baseline_id?: string;
  expected_sha256?: string;
  auth_ref?: string;
  requested_by?: string;
  reason?: string;
}

export interface SBOMDriftBaselineArtifactSourceSpec {
  source_name: string;
  provider: string;
  artifact_url: string;
  artifact_name: string;
  baseline_id: string;
  expected_sha256?: string;
  auth_ref?: string;
  fetches_network: boolean;
  uses_credentials: boolean;
  writes_baseline: boolean;
  notes?: string[];
}

export interface SBOMDriftBaselineFetchHandoffPlan {
  target: string;
  dedup_key: string;
  source: SBOMDriftBaselineArtifactSourceSpec;
  artifact_source_plan_ready: boolean;
  baseline_fetch_plan_ready: boolean;
  baseline_fetch_ready: boolean;
  artifact_baseline_ready: boolean;
  ci_baseline_store_ready: boolean;
  ci_baseline_writeback_ready: boolean;
  consumes_artifact_repository: boolean;
  fetches_artifact_baseline: boolean;
  writes_ci_baseline_store: boolean;
  writes_baseline_snapshot: boolean;
  writes_ci_workflow: boolean;
  executes_govulncheck: boolean;
  blocks_release: boolean;
  expected_artifacts: string[];
  blocked_by: string[];
  notes?: string[];
}

export interface SBOMDriftBaselineArtifactSourcePlan {
  pack_id: string;
  generated_at: string;
  status: string;
  stage: string;
  requested_by?: string;
  reason?: string;
  artifact_source_plan_ready: boolean;
  baseline_fetch_plan_ready: boolean;
  baseline_fetch_ready: boolean;
  artifact_baseline_ready: boolean;
  ci_baseline_store_ready: boolean;
  ci_baseline_writeback_ready: boolean;
  ci_gate_plan_ready: boolean;
  ci_gate_ready: boolean;
  govulncheck_plan_ready: boolean;
  govulncheck_ready: boolean;
  vulnerability_ready: boolean;
  consumes_artifact_repository: boolean;
  fetches_artifact_baseline: boolean;
  writes_ci_baseline_store: boolean;
  writes_baseline_snapshot: boolean;
  writes_ci_workflow: boolean;
  executes_govulncheck: boolean;
  blocks_release: boolean;
  source: SBOMDriftBaselineArtifactSourceSpec;
  baseline_fetch_handoff_plan: SBOMDriftBaselineFetchHandoffPlan;
  ci_baseline_store: SBOMDriftCIBaselineStoreSummary;
  artifacts: string[];
  actions: string[];
  blocked_by: string[];
  labels: string[];
  notes?: string[];
}

export interface SBOMDriftChange {
  ecosystem: string;
  name: string;
  path?: string;
  old_version?: string;
  new_version?: string;
  risk: string;
}

export interface SBOMDriftDiff {
  base: SBOMDriftSnapshotSummary;
  target: SBOMDriftSnapshotSummary;
  added: SBOMDriftChange[];
  removed: SBOMDriftChange[];
  changed: SBOMDriftChange[];
  risk_level: string;
  notes?: string[];
}

export interface SBOMDriftCycloneDXDocument {
  bomFormat: string;
  specVersion: string;
  version: number;
  metadata: Record<string, unknown>;
  components: Array<Record<string, unknown>>;
  dependencies?: Array<Record<string, unknown>>;
}

export interface SBOMDriftGovulncheckPackagePlan {
  ecosystem: string;
  module: string;
  version?: string;
  scope?: string;
  path?: string;
  direct: boolean;
  labels?: string[];
}

export interface SBOMDriftGovulncheckPlan {
  plan_ready: boolean;
  ready: boolean;
  status: string;
  command: string;
  target_package: string;
  report_artifact: string;
  executes: boolean;
  writes_files: boolean;
  vulnerability_db_fetch: boolean;
  package_count: number;
  module_count: number;
  packages: SBOMDriftGovulncheckPackagePlan[];
  labels: string[];
  notes?: string[];
}

export interface SBOMDriftCIGatePlan {
  pack_id: string;
  generated_at: string;
  status: string;
  blocked: boolean;
  fail_on_risk: string;
  cyclonedx_ready: boolean;
  ci_gate_plan_ready: boolean;
  ci_gate_ready: boolean;
  govulncheck_plan_ready: boolean;
  govulncheck_ready: boolean;
  requested_by?: string;
  reason?: string;
  diff: SBOMDriftDiff;
  govulncheck_plan: SBOMDriftGovulncheckPlan;
  artifacts: string[];
  commands: string[];
  actions: string[];
  notes?: string[];
}

export interface SBOMDriftCIBaselineWritebackInput {
  base_id: string;
  target_id?: string;
  target_current?: boolean;
  fail_on_risk?: string;
  requested_by?: string;
  reason?: string;
  approval_id?: string;
  request_id?: string;
  request_key?: string;
}

export interface SBOMDriftCIBaselineRecord {
  pack_id: string;
  created_at: string;
  status: string;
  record_id: string;
  request_id: string;
  request_key: string;
  approval_id?: string;
  base_id: string;
  target_id: string;
  target_current: boolean;
  fail_on_risk: string;
  requested_by?: string;
  reason?: string;
  blocked: boolean;
  risk_level: string;
  ci_baseline_store_ready: boolean;
  ci_baseline_writeback_ready: boolean;
  writes_ci_baseline_store: boolean;
  ci_gate_plan_ready: boolean;
  ci_gate_ready: boolean;
  govulncheck_plan_ready: boolean;
  govulncheck_ready: boolean;
  vulnerability_ready: boolean;
  writes_ci_workflow: boolean;
  executes_govulncheck: boolean;
  blocks_release: boolean;
  base: SBOMDriftSnapshotSummary;
  target: SBOMDriftSnapshotSummary;
  ci_gate_plan: SBOMDriftCIGatePlan;
  artifacts: string[];
  actions: string[];
  blocked_by: string[];
  labels: string[];
  notes?: string[];
}

export interface SBOMDriftCIBaselineWriteback {
  pack_id: string;
  generated_at: string;
  status: string;
  record_id: string;
  request_id: string;
  request_key: string;
  approval_id?: string;
  base_id: string;
  target_id: string;
  target_current: boolean;
  fail_on_risk: string;
  requested_by?: string;
  reason?: string;
  blocked: boolean;
  risk_level: string;
  ci_baseline_store_ready: boolean;
  ci_baseline_writeback_ready: boolean;
  writes_ci_baseline_store: boolean;
  ci_gate_plan_ready: boolean;
  ci_gate_ready: boolean;
  govulncheck_plan_ready: boolean;
  govulncheck_ready: boolean;
  vulnerability_ready: boolean;
  writes_ci_workflow: boolean;
  executes_govulncheck: boolean;
  blocks_release: boolean;
  ci_gate_plan: SBOMDriftCIGatePlan;
  ci_baseline_store: SBOMDriftCIBaselineStoreSummary;
  ci_baseline_record: SBOMDriftCIBaselineRecord;
  artifacts: string[];
  actions: string[];
  blocked_by: string[];
  labels: string[];
  notes?: string[];
}

export interface SBOMDriftCIWorkflowWritebackPlan {
  pack_id: string;
  generated_at: string;
  status: string;
  stage: string;
  record_id: string;
  request_id: string;
  request_key: string;
  requested_by?: string;
  reason?: string;
  blocked: boolean;
  risk_level: string;
  fail_on_risk: string;
  ci_baseline_store_ready: boolean;
  ci_baseline_writeback_ready: boolean;
  ci_workflow_writeback_plan_ready: boolean;
  ci_workflow_writeback_ready: boolean;
  consumes_ci_baseline_store: boolean;
  ci_gate_plan_ready: boolean;
  ci_gate_ready: boolean;
  govulncheck_plan_ready: boolean;
  govulncheck_ready: boolean;
  vulnerability_ready: boolean;
  writes_ci_baseline_store: boolean;
  writes_ci_workflow: boolean;
  executes_govulncheck: boolean;
  blocks_release: boolean;
  ci_baseline_store: SBOMDriftCIBaselineStoreSummary;
  ci_baseline_record: SBOMDriftCIBaselineRecord;
  ci_workflow_handoff_plan: SBOMDriftCIWorkflowHandoffPlan;
  release_blocker_plan: SBOMDriftReleaseBlockerPlan;
  artifacts: string[];
  actions: string[];
  blocked_by: string[];
  labels: string[];
  notes?: string[];
}

export interface SBOMDriftEvidence {
  pack_id: string;
  exported_at: string;
  format: string;
  files: string[];
  snapshot: SBOMDriftSnapshot;
  cyclonedx?: SBOMDriftCycloneDXDocument;
  ci_gate_plan?: SBOMDriftCIGatePlan;
  govulncheck_plan?: SBOMDriftGovulncheckPlan;
  ci_baseline_store?: SBOMDriftCIBaselineStoreSummary;
  ci_baseline_records?: SBOMDriftCIBaselineRecord[];
  ci_workflow_writeback_plan?: SBOMDriftCIWorkflowWritebackPlan;
  ci_workflow_handoff_plan?: SBOMDriftCIWorkflowHandoffPlan;
  baseline_artifact_source_plan?: SBOMDriftBaselineArtifactSourcePlan;
  baseline_fetch_handoff_plan?: SBOMDriftBaselineFetchHandoffPlan;
  release_blocker_plan?: SBOMDriftReleaseBlockerPlan;
}

export interface SBOMDriftPackClient {
  status(): Promise<SBOMDriftStatus>;
  snapshots(): Promise<{ snapshots: SBOMDriftSnapshotSummary[]; count: number }>;
  createSnapshot(input?: { id?: string; source?: string }): Promise<{ snapshot: SBOMDriftSnapshot; status: string }>;
  snapshot(id: string): Promise<{ snapshot: SBOMDriftSnapshot }>;
  diff(input: { base_id: string; target_id?: string; target_current?: boolean }): Promise<{ diff: SBOMDriftDiff }>;
  cycloneDX(id?: string): Promise<{ bom: SBOMDriftCycloneDXDocument; snapshot: SBOMDriftSnapshotSummary }>;
  ciGatePlan(input: { base_id: string; target_id?: string; target_current?: boolean; fail_on_risk?: string; requested_by?: string; reason?: string }): Promise<{ plan: SBOMDriftCIGatePlan }>;
  baselineArtifactSourcePlan(input?: SBOMDriftBaselineArtifactSourceInput): Promise<{ plan: SBOMDriftBaselineArtifactSourcePlan }>;
  ciBaselineWriteback(input: SBOMDriftCIBaselineWritebackInput): Promise<{ writeback: SBOMDriftCIBaselineWriteback }>;
  ciWorkflowWritebackPlan(input?: SBOMDriftCIWorkflowWritebackInput): Promise<{ plan: SBOMDriftCIWorkflowWritebackPlan }>;
  evidence(id: string): Promise<SBOMDriftEvidence>;
}

function enc(value: string): string {
  return encodeURIComponent(value);
}

export function createSBOMDriftPackClient(): SBOMDriftPackClient {
  return {
    status: () => fetcher<SBOMDriftStatus>("/v1/sbom-drift/status"),
    snapshots: () => fetcher<{ snapshots: SBOMDriftSnapshotSummary[]; count: number }>("/v1/sbom-drift/snapshots"),
    createSnapshot: (input = {}) =>
      fetcher<{ snapshot: SBOMDriftSnapshot; status: string }>("/v1/sbom-drift/snapshots", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    snapshot: (id) => fetcher<{ snapshot: SBOMDriftSnapshot }>(`/v1/sbom-drift/snapshots/${enc(id)}`),
    diff: (input) =>
      fetcher<{ diff: SBOMDriftDiff }>("/v1/sbom-drift/diff", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    cycloneDX: (id = "current") => fetcher<{ bom: SBOMDriftCycloneDXDocument; snapshot: SBOMDriftSnapshotSummary }>(`/v1/sbom-drift/cyclonedx/${enc(id)}`),
    ciGatePlan: (input) =>
      fetcher<{ plan: SBOMDriftCIGatePlan }>("/v1/sbom-drift/ci-gate/plan", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    baselineArtifactSourcePlan: (input = {}) =>
      fetcher<{ plan: SBOMDriftBaselineArtifactSourcePlan }>("/v1/sbom-drift/baseline/artifact-source/plan", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    ciBaselineWriteback: (input) =>
      fetcher<{ writeback: SBOMDriftCIBaselineWriteback }>("/v1/sbom-drift/ci-gate/baseline/writeback", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    ciWorkflowWritebackPlan: (input = {}) =>
      fetcher<{ plan: SBOMDriftCIWorkflowWritebackPlan }>("/v1/sbom-drift/ci-gate/workflow/writeback/plan", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    evidence: (id) => fetcher<SBOMDriftEvidence>(`/v1/sbom-drift/evidence/${enc(id)}`),
  };
}
