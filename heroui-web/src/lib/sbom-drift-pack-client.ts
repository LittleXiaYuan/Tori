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
  vulnerability_ready: boolean;
  snapshot_count: number;
  repo_root?: string;
  store_dir?: string;
  capabilities: string[];
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

export interface SBOMDriftPackClient {
  status(): Promise<SBOMDriftStatus>;
  snapshots(): Promise<{ snapshots: SBOMDriftSnapshotSummary[]; count: number }>;
  createSnapshot(input?: { id?: string; source?: string }): Promise<{ snapshot: SBOMDriftSnapshot; status: string }>;
  snapshot(id: string): Promise<{ snapshot: SBOMDriftSnapshot }>;
  diff(input: { base_id: string; target_id?: string; target_current?: boolean }): Promise<{ diff: SBOMDriftDiff }>;
  evidence(id: string): Promise<{ pack_id: string; exported_at: string; format: string; files: string[]; snapshot: SBOMDriftSnapshot }>;
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
    evidence: (id) => fetcher<{ pack_id: string; exported_at: string; format: string; files: string[]; snapshot: SBOMDriftSnapshot }>(`/v1/sbom-drift/evidence/${enc(id)}`),
  };
}
