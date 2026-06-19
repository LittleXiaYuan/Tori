import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import LoRAPackPage from "../packs/lora/page";

const loraClientMock = vi.hoisted(() => ({
  status: vi.fn(),
  history: vi.fn(),
  summary: vi.fn(),
  preview: vi.fn(),
  trigger: vi.fn(),
  rollback: vi.fn(),
  evolution: vi.fn(),
  config: vi.fn(),
  updateConfig: vi.fn(),
}));

vi.mock("@/lib/lora-pack-client", () => ({
  createLoRAPackClient: () => loraClientMock,
}));

vi.mock("@/components/toast-provider", () => ({
  showToast: vi.fn(),
}));

describe("LoRAPackPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    loraClientMock.status.mockResolvedValue({
      enabled: true,
      active_model: "local",
      scheduler: {
        current_adapter: "adapter-a",
        ab_test_active: false,
        ab_test_metrics: {
          new_adapter_queries: 0,
          new_adapter_score: 0,
          old_adapter_queries: 0,
          old_adapter_score: 0,
        },
        total_trains: 0,
        total_rollbacks: 0,
      },
      rolling_success_rate: 0,
    });
    loraClientMock.history.mockResolvedValue({ records: [], count: 0 });
    loraClientMock.summary.mockResolvedValue({
      summary: {
        total_runs: 0,
        success_count: 0,
        avg_loss: 0,
        avg_duration: 0,
      },
    });
    loraClientMock.evolution.mockResolvedValue({
      state: {
        rolling_success_rate: 0,
        recent_window: [],
        total_tasks: 0,
        success_tasks: 0,
        tasks_since_strategy: 0,
        strategy_updates: 0,
      },
    });
    loraClientMock.config.mockResolvedValue({
      config: {
        enabled: true,
        min_samples: 20,
        cooldown: 86400000000000,
      },
    });
    loraClientMock.preview.mockResolvedValue({
      preview: {
        raw_samples: 0,
        usable_samples: 0,
        min_samples: 20,
        ready: false,
        filter_enabled: true,
        data_path: "",
      },
    });
  });

  it("explains LoRA training as gated adaptation with manual rollback", async () => {
    render(<LoRAPackPage />);

    expect(await screen.findByText("这个能力包现在适合做什么")).toBeInTheDocument();
    expect(screen.getByText("可直接使用")).toBeInTheDocument();
    expect(screen.getByText("训练受门槛限制")).toBeInTheDocument();
    expect(screen.getByText("可人工回滚")).toBeInTheDocument();
    expect(screen.getByText(/任务样本转成 LoRA\/LAA 训练闭环/)).toBeInTheDocument();
    expect(screen.getByText("1. 先做训练预检")).toBeInTheDocument();
    expect(screen.getByText("2. 观察 A/B 与成功率")).toBeInTheDocument();
    expect(screen.getByText("3. 必要时人工回滚")).toBeInTheDocument();
    expect(screen.getByText("当前不会做什么")).toBeInTheDocument();
    expect(screen.getByText("不会绕过样本量和间隔限制强行训练。")).toBeInTheDocument();
    expect(screen.getByText("不会保证每次训练都提升模型表现。")).toBeInTheDocument();
  });
});
