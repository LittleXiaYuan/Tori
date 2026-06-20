import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import GuardrailFuzzerPackPage from "../packs/guardrail-fuzzer/page";

const guardrailClientMock = vi.hoisted(() => ({
  status: vi.fn(),
  reports: vi.fn(),
  report: vi.fn(),
  saveCorpus: vi.fn(),
  run: vi.fn(),
  ciGatePlan: vi.fn(),
  nativeCorpusPlan: vi.fn(),
  evidence: vi.fn(),
}));

vi.mock("@/lib/guardrail-fuzzer-pack-client", () => ({
  createGuardrailFuzzerPackClient: () => guardrailClientMock,
}));

vi.mock("@/components/toast-provider", () => ({
  showToast: vi.fn(),
}));

vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

describe("GuardrailFuzzerPackPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    guardrailClientMock.status.mockResolvedValue({
      pack_id: "yunque.pack.guardrail-fuzzer",
      stage: "pack-shell",
      fuzzer_ready: true,
      ci_gate_plan_ready: true,
      ci_gate_ready: false,
      rule_writeback_plan_ready: true,
      rule_writeback_ready: false,
      alert_plan_ready: true,
      alert_ready: false,
      native_corpus_plan_ready: true,
      native_corpus_sync_ready: false,
      go_native_fuzz_plan_ready: true,
      go_native_fuzz_ready: false,
      seed_count: 2,
      report_count: 0,
      policy: {
        mutants_per_seed: 6,
        max_input_len: 4000,
        max_mutations_per_seed: 8,
        bypass_fail_threshold: 1,
        false_positive_warn_threshold: 1,
      },
      mutations: [],
      capabilities: ["guardrail.fuzz.run", "guardrail.ci.plan"],
    });
    guardrailClientMock.reports.mockResolvedValue({ reports: [], count: 0 });
  });

  it("explains fuzzer output as guardrail regression evidence and plan-only follow-up", async () => {
    render(<GuardrailFuzzerPackPage />);

    expect(await screen.findByText("这个能力包现在适合做什么")).toBeInTheDocument();
    expect(screen.getByText("实验中")).toBeInTheDocument();
    expect(screen.getByText("可运行回归")).toBeInTheDocument();
    expect(screen.getByText("规则只生成计划")).toBeInTheDocument();
    expect(screen.getByText(/检查云雀的安全护栏有没有被提示注入、越权请求或变体样本绕过/)).toBeInTheDocument();
    expect(screen.getByText("1. 准备测试语料")).toBeInTheDocument();
    expect(screen.getByText("2. 运行护栏回归")).toBeInTheDocument();
    expect(screen.getByText("3. 输出修复计划")).toBeInTheDocument();
    expect(screen.getByText("当前不会做什么")).toBeInTheDocument();
    expect(screen.getByText("不会自动改写护栏规则。")).toBeInTheDocument();
    expect(screen.getByText("不会创建 CI 定时任务或阻断发布。")).toBeInTheDocument();
    expect(screen.getByText("不会发送告警、开 issue 或上传 artifacts。")).toBeInTheDocument();
    expect(screen.getByText("技术状态")).toBeInTheDocument();
    expect(screen.getByText("从绕过报告到护栏修复")).toBeInTheDocument();
    expect(screen.getByText("2. 带回 Chat")).toBeInTheDocument();
    expect(screen.getByText("3. 看修复依据")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /带回 Chat/ })).toHaveAttribute("href", expect.stringContaining("/chat?q="));
    expect(screen.getByRole("link", { name: /看任务/ })).toHaveAttribute("href", "/missions");
    expect(screen.getByRole("link", { name: "核对执行轨迹" })).toHaveAttribute("href", "/trace");
    expect(screen.getByRole("link", { name: "让小羽继续改" })).toHaveAttribute("href", "/packs/studio?packId=yunque.pack.guardrail-fuzzer");
  });
});
