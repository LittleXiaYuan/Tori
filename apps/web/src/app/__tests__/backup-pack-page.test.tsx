import { render, screen, fireEvent } from "@testing-library/react";
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

  it("leads with backup/restore actions and data, not boilerplate", async () => {
    render(<BackupPage />);

    // The page leads with the actual tool: export/restore actions, stats and file list.
    expect(await screen.findByText("memory.json")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /导出备份/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /选择文件恢复/ })).toBeInTheDocument();
    expect(screen.getByText("数据文件")).toBeInTheDocument();
    expect(screen.getByText("总计大小")).toBeInTheDocument();
  });

  it("keeps reversible/destructive guidance behind the about disclosure", async () => {
    render(<BackupPage />);
    await screen.findByText("memory.json");

    const trigger = screen.getByRole("button", { name: /关于这个能力包/ });
    fireEvent.click(trigger);

    expect(await screen.findByText(/升级、迁移或排障前保留一份云雀本地数据副本/)).toBeInTheDocument();
    expect(screen.getByText("1. 先看备份范围")).toBeInTheDocument();
    expect(screen.getByText("2. 导出可回滚副本")).toBeInTheDocument();
    expect(screen.getByText("3. 谨慎导入恢复")).toBeInTheDocument();
    expect(screen.getByText("操作前请确认")).toBeInTheDocument();
    expect(screen.getByText("导入恢复会覆盖当前记录，请先导出当前副本。")).toBeInTheDocument();
    expect(screen.getByText("不会把备份自动上传到云端。")).toBeInTheDocument();
    expect(screen.getByText("不会只恢复单个模块，当前按备份包整体恢复。")).toBeInTheDocument();
  });
});
