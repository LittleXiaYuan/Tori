import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import WASMPluginPackPage from "../packs/wasm-plugin/page";

const wasmClientMock = vi.hoisted(() => ({
  status: vi.fn(),
  plugins: vi.fn(),
  installPlugin: vi.fn(),
  remoteInstallPlan: vi.fn(),
  remoteInstallApprovalPlan: vi.fn(),
  remoteInstallApprovalDecisionPlan: vi.fn(),
  remoteInstallApprovalWritebackPlan: vi.fn(),
  remoteInstallApprovalQueueWriteback: vi.fn(),
  remoteInstallInstallerContinuationPlan: vi.fn(),
  remoteInstallInstallerDownloadWriteback: vi.fn(),
  remoteInstallSignatureVerificationWriteback: vi.fn(),
  remoteInstallPackageInspectWriteback: vi.fn(),
  remoteInstallInstallerRegistrationPlan: vi.fn(),
  load: vi.fn(),
  unload: vi.fn(),
  execute: vi.fn(),
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

vi.mock("yunque-client/wasm-plugin", () => ({
  createWASMPluginClient: () => wasmClientMock,
}));

describe("WASMPluginPackPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    wasmClientMock.status.mockResolvedValue({
      pack_id: "yunque.pack.wasm-plugin",
      stage: "pack-shell",
      runtime_ready: true,
      abi_ready: false,
      abi_plan_ready: true,
      host_abi_execution_gate_ready: true,
      host_abi_enforcement_ready: false,
      module_integrity_gate_ready: true,
      remote_install_plan_ready: true,
      remote_install_ready: false,
      approval_gate_plan_ready: true,
      approval_gate_ready: false,
      approval_decision_plan_ready: true,
      approval_writeback_plan_ready: true,
      approval_queue_store_ready: false,
      installer_continuation_plan_ready: true,
      installer_download_writeback_ready: true,
      signature_verification_writeback_ready: true,
      package_inspect_writeback_ready: true,
      installer_registration_plan_ready: true,
      registration_ready: false,
      installer_ready: false,
      installer_blocked_until_registration: true,
      plugin_count: 0,
      loaded_count: 0,
    });
    wasmClientMock.plugins.mockResolvedValue({ plugins: [] });
  });

  it("leads with a user-facing WASM intake explanation before technical gates", async () => {
    render(<WASMPluginPackPage />);

    expect(await screen.findByText("这个能力包现在能做什么")).toBeInTheDocument();
    expect(screen.getByText("高风险实验能力")).toBeInTheDocument();
    expect(screen.getByText("远程安装先计划")).toBeInTheDocument();
    expect(screen.getByText("沙箱执行先 dry-run")).toBeInTheDocument();
    expect(screen.getByText(/第三方 WASM 能力进入云雀前的验收台/)).toBeInTheDocument();
    expect(screen.getByText("1. 先看能不能接入")).toBeInTheDocument();
    expect(screen.getByText("2. 再预演远程安装")).toBeInTheDocument();
    expect(screen.getByText("3. 最后留证据再放行")).toBeInTheDocument();
    expect(screen.getByText("当前不会做什么")).toBeInTheDocument();
    expect(screen.getByText("不会绕过审批直接安装远程包。")).toBeInTheDocument();
    expect(screen.getByText("不会把未验签包解包到 plugin_dir。")).toBeInTheDocument();
    expect(screen.getByText("技术状态")).toBeInTheDocument();
    expect(screen.getByText("查看技术链路详情")).toBeInTheDocument();
  });
});
