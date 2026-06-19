import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import BackupPage from "../packs/backup/page";

const backupClientMock = vi.hoisted(() => ({
  info: vi.fn(),
  export: vi.fn(),
  import: vi.fn(),
}));

vi.mock("@/lib/backup-pack-client", () => ({
  createBackupPackClient: () => backupClientMock,
}));

describe("BackupPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    backupClientMock.info.mockResolvedValue({
      files: {
        "memory.json": 1024,
        "tasks.json": 2048,
      },
      file_count: 2,
      total_bytes: 3072,
      version: "v0.1.0-beta.1",
    });
  });

  it("explains backup as a reversible local data workflow with destructive restore warning", async () => {
    render(<BackupPage />);

    expect(await screen.findByText("这个能力包现在适合做什么")).toBeInTheDocument();
    expect(screen.getByText("可直接使用")).toBeInTheDocument();
    expect(screen.getByText("本地数据")).toBeInTheDocument();
    expect(screen.getByText("导入会覆盖")).toBeInTheDocument();
    expect(screen.getByText(/升级、迁移或排障前保留一份云雀本地数据副本/)).toBeInTheDocument();
    expect(screen.getByText("1. 先看备份范围")).toBeInTheDocument();
    expect(screen.getByText("2. 导出可回滚副本")).toBeInTheDocument();
    expect(screen.getByText("3. 谨慎导入恢复")).toBeInTheDocument();
    expect(screen.getByText("操作前请确认")).toBeInTheDocument();
    expect(screen.getByText("导入恢复会覆盖当前记录，请先导出当前副本。")).toBeInTheDocument();
    expect(screen.getByText("不会把备份自动上传到云端。")).toBeInTheDocument();
    expect(screen.getByText("不会只恢复单个模块，当前按备份包整体恢复。")).toBeInTheDocument();
  });
});
