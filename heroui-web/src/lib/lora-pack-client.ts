import { fetcher } from "./api-core";
import type {
  EvolutionState,
  LoRAConfig,
  LoRAStatus,
  TrainingDataPreview,
  TrainingRecord,
  TrainingSummary,
} from "./api-types/lora";

export interface LoRAPackClient {
  status(): Promise<LoRAStatus>;
  history(): Promise<{ records: TrainingRecord[]; count: number }>;
  summary(): Promise<{ summary: TrainingSummary }>;
  preview(tenantId?: string): Promise<{ preview: TrainingDataPreview }>;
  trigger(tenantId?: string): Promise<{ status: string; tenant_id: string }>;
  rollback(): Promise<{ status: string }>;
  evolution(): Promise<{ state: EvolutionState }>;
  config(): Promise<{ config: LoRAConfig }>;
  updateConfig(patch: Partial<LoRAConfig> & Record<string, unknown>): Promise<{ config: LoRAConfig; status: string }>;
}

export function createLoRAPackClient(): LoRAPackClient {
  return {
    status: () => fetcher<LoRAStatus>("/v1/lora/status"),
    history: () => fetcher<{ records: TrainingRecord[]; count: number }>("/v1/lora/history"),
    summary: () => fetcher<{ summary: TrainingSummary }>("/v1/lora/summary"),
    preview: (tenantId?: string) =>
      fetcher<{ preview: TrainingDataPreview }>(
        `/v1/lora/preview${tenantId ? `?tenant_id=${encodeURIComponent(tenantId)}` : ""}`,
      ),
    trigger: (tenantId?: string) =>
      fetcher<{ status: string; tenant_id: string }>("/v1/lora/trigger", {
        method: "POST",
        body: JSON.stringify(tenantId ? { tenant_id: tenantId } : {}),
      }),
    rollback: () => fetcher<{ status: string }>("/v1/lora/rollback", { method: "POST" }),
    evolution: () => fetcher<{ state: EvolutionState }>("/v1/lora/evolution"),
    config: () => fetcher<{ config: LoRAConfig }>("/v1/lora/config"),
    updateConfig: (patch) =>
      fetcher<{ config: LoRAConfig; status: string }>("/v1/lora/config", {
        method: "PUT",
        body: JSON.stringify(patch),
      }),
  };
}

