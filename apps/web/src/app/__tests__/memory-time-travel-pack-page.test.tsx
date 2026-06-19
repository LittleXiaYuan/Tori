import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import MemoryTimeTravelPackPage from "../packs/memory-time-travel/page";

const memoryClientMock = vi.hoisted(() => ({
  status: vi.fn(),
  snapshots: vi.fn(),
  saveSnapshot: vi.fn(),
  snapshotAt: vi.fn(),
  diff: vi.fn(),
  rollbackPlan: vi.fn(),
  approvedRollbackPlan: vi.fn(),
  rollbackWritebackStore: vi.fn(),
  rollbackWritebackExecutorPlan: vi.fn(),
  retentionPlan: vi.fn(),
  retentionPrunePlan: vi.fn(),
  retentionPruneExecute: vi.fn(),
  nativeKVHistoryPlan: vi.fn(),
  nativeKVHistoryPreview: vi.fn(),
  kvHistoryDualReadParity: vi.fn(),
  kvHistoryCutoverPlan: vi.fn(),
  kvHistoryCutoverReadiness: vi.fn(),
  auditVerify: vi.fn(),
  kvAuditLinks: vi.fn(),
  kvAuditProofLinkPreview: vi.fn(),
  kvAuditProofLinkWritebackPlan: vi.fn(),
  kvAuditProofLinkWritebackStore: vi.fn(),
  kvAuditProofLinkWritebackExecutorPlan: vi.fn(),
  evidence: vi.fn(),
}));

vi.mock("@/lib/sdk-client", () => ({
  createYunqueSDKClientOptions: () => ({
    baseUrl: "http://localhost",
    fetch: vi.fn(),
  }),
}));

vi.mock("@/components/toast-provider", () => ({
  showToast: vi.fn(),
}));

vi.mock("yunque-client/memory-time-travel", () => ({
  createMemoryTimeTravelClient: () => memoryClientMock,
}));

describe("MemoryTimeTravelPackPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    memoryClientMock.status.mockResolvedValue({
      pack_id: "yunque.pack.memory-time-travel",
      stage: "pack-shell-before-ledger-kv-history",
      ledger_history_ready: false,
      snapshot_count: 0,
      namespace_count: 0,
      retention_plan_ready: true,
      retention_pack_local_prune_ready: false,
      native_kv_history_plan_ready: true,
      native_kv_history_preview_ready: true,
      dual_read_parity_check_ready: true,
      kv_history_cutover_plan_ready: true,
      kv_history_cutover_readiness_ready: true,
      rollback_writeback_store_ready: false,
      rollback_writeback_executor_plan_ready: true,
      rollback_writeback_ready: false,
      capabilities: [],
    });
    memoryClientMock.snapshots.mockResolvedValue({ snapshots: [], count: 0 });
  });

  it("explains memory snapshots as dry-run time travel instead of live rollback", async () => {
    render(<MemoryTimeTravelPackPage />);

    expect(await screen.findByText("这个能力包现在适合做什么")).toBeInTheDocument();
    expect(screen.getByText("实验中")).toBeInTheDocument();
    expect(screen.getByText("可保存快照")).toBeInTheDocument();
    expect(screen.getByText("回滚只生成计划")).toBeInTheDocument();
    expect(screen.getByText(/给云雀的记忆做版本快照、时间点回看和漂移对比/)).toBeInTheDocument();
    expect(screen.getByText("1. 保存记忆快照")).toBeInTheDocument();
    expect(screen.getByText("2. 对比漂移与回溯")).toBeInTheDocument();
    expect(screen.getByText("3. 生成回滚证据")).toBeInTheDocument();
    expect(screen.getByText("当前不会做什么")).toBeInTheDocument();
    expect(screen.getByText("不会直接修改 live memory 或 Ledger KV。")).toBeInTheDocument();
    expect(screen.getByText("不会自动切换 kv_history adapter 或执行 schema 迁移。")).toBeInTheDocument();
    expect(screen.getByText("不会跳过审批执行真实回滚。")).toBeInTheDocument();
    expect(screen.getByText("技术状态")).toBeInTheDocument();
  });
});
