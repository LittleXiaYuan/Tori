"use client";

import { useEffect, useState } from "react";
import { Button, Card, Chip, Input, Label, Spinner, TextArea, TextField } from "@heroui/react";
import { Camera, MonitorCog, Play, RotateCcw, ShieldAlert, ShieldCheck } from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { createYunqueSDKClientOptions } from "@/lib/sdk-client";
import { formatErrorMessage } from "@/lib/error-utils";

const sdkOptions = createYunqueSDKClientOptions();

type ComputerStatus = {
  execution_ready?: boolean;
  surfaces?: Record<string, ComputerSurfaceStatus | undefined>;
  safety?: Record<string, unknown>;
  capabilities?: string[];
};

type ComputerSurfaceStatus = {
  available?: boolean;
  connected?: boolean;
  running?: boolean;
  status?: string;
  health?: Record<string, unknown>;
};

type ComputerPlan = {
  goal: string;
  surface: string;
  status: string;
  plan_ready: boolean;
  execution_ready: boolean;
  required_permissions?: string[];
  steps?: Array<{ index: number; action: string; surface: string; read_only: boolean; permission: string; executor: string; description: string; blocked_by?: string[] }>;
  gates?: Array<{ gate: string; ready: boolean; allows_execute: boolean; human_approval: boolean; policy_enforced: boolean; blocked_by?: string[] }>;
  blocked_by?: string[];
  notes?: string[];
};

type ScreenshotEvidence = {
  surface: string;
  screenshot?: string;
  timestamp?: string;
  safety?: Record<string, unknown>;
};

const surfaceCards: Array<{
  key: "browser" | "desktop_sandbox" | "local_desktop";
  title: string;
  desc: string;
  readyLabel: string;
  unavailableLabel: string;
}> = [
  {
    key: "browser",
    title: "浏览器",
    desc: "可读取当前浏览器页面和截图证据，适合网页调研、表单检查、发布前确认。",
    readyLabel: "可读",
    unavailableLabel: "未连接",
  },
  {
    key: "desktop_sandbox",
    title: "云桌面",
    desc: "用于未来把高风险操作放进隔离环境；当前只读取配置状态，不执行动作。",
    readyLabel: "已配置",
    unavailableLabel: "未配置",
  },
  {
    key: "local_desktop",
    title: "本机桌面",
    desc: "本机鼠标、键盘、命令和文件写入仍关闭，后续必须经过授权和审批。",
    readyLabel: "已开放",
    unavailableLabel: "Beta 关闭",
  },
];

const explainGate = (gate: string) => {
  if (gate.includes("permission")) return "权限策略";
  if (gate.includes("approval")) return "人工审批";
  return gate;
};

async function requestJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await sdkOptions.fetch(`${sdkOptions.baseUrl}${path}`, {
    ...init,
    headers: init?.headers,
  });
  const text = await response.text();
  const body = text ? JSON.parse(text) : undefined;
  if (!response.ok) throw new Error(typeof body?.error === "string" ? body.error : `HTTP ${response.status}`);
  return body as T;
}

export default function ComputerUsePackPage() {
  const [status, setStatus] = useState<ComputerStatus | null>(null);
  const [goal, setGoal] = useState("帮我检查当前浏览器页面，并给出下一步操作计划");
  const [surface, setSurface] = useState("auto");
  const [plan, setPlan] = useState<ComputerPlan | null>(null);
  const [screenshot, setScreenshot] = useState<ScreenshotEvidence | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [screenshotLoading, setScreenshotLoading] = useState(false);

  const loadStatus = async () => {
    setLoading(true);
    try {
      setStatus(await requestJSON<ComputerStatus>("/v1/computer/status"));
    } catch (error) {
      showToast(formatErrorMessage(error, "读取电脑使用状态失败"), "error");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadStatus();
  }, []);

  const createPlan = async () => {
    setBusy(true);
    try {
      const result = await requestJSON<{ plan: ComputerPlan }>("/v1/computer/intent/plan", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ goal, surface, requested_by: "packs/computer-use", allow_execute: false }),
      });
      setPlan(result.plan);
      showToast("已生成电脑使用计划；当前不会执行本机控制", "success");
    } catch (error) {
      showToast(formatErrorMessage(error, "生成计划失败"), "error");
    } finally {
      setBusy(false);
    }
  };

  const captureBrowserScreenshot = async () => {
    setScreenshotLoading(true);
    try {
      const result = await requestJSON<ScreenshotEvidence>("/v1/computer/screenshot?surface=browser");
      setScreenshot(result);
      showToast("已读取浏览器截图证据", "success");
    } catch (error) {
      showToast(formatErrorMessage(error, "读取浏览器截图失败，请确认浏览器连接器已连接"), "error");
    } finally {
      setScreenshotLoading(false);
    }
  };

  if (loading) {
    return <div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>;
  }

  return (
    <div className="page-root space-y-5 animate-fade-in-up">
      <PageHeader
        icon={<MonitorCog size={20} />}
        title="电脑使用"
        description="把目标转成可审阅的电脑使用计划。当前只规划和读取浏览器截图，不执行本机桌面控制。"
        actions={<Button variant="ghost" onPress={loadStatus}><RotateCcw size={14} /> 刷新状态</Button>}
      />

      <Card className="section-card overflow-hidden p-0">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_320px]">
          <div className="p-5">
            <div className="text-base font-semibold" style={{ color: "var(--yunque-text)" }}>这个能力包现在能做什么</div>
            <div className="mt-2 max-w-3xl text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>
              它是云雀 Computer Use 的第一层：先把目标变成可审阅计划，确认要看哪个界面、需要什么权限、为什么暂不执行；浏览器连接后还能读取一张只读截图作为证据。
            </div>
            <div className="mt-4 grid gap-3 md:grid-cols-3">
              <div className="rounded-lg p-3" style={{ background: "var(--yunque-bg-hover)", border: "1px solid var(--yunque-border)" }}>
                <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>1. 先规划</div>
                <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>把“帮我操作电脑”拆成界面、步骤、权限和阻塞原因。</div>
              </div>
              <div className="rounded-lg p-3" style={{ background: "var(--yunque-bg-hover)", border: "1px solid var(--yunque-border)" }}>
                <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>2. 再取证</div>
                <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>浏览器连接器可用时，只读取当前页面截图，不点击、不输入。</div>
              </div>
              <div className="rounded-lg p-3" style={{ background: "var(--yunque-bg-hover)", border: "1px solid var(--yunque-border)" }}>
                <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>3. 后审批</div>
                <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>未来真实执行必须经过权限策略、人工确认和执行器接入。</div>
              </div>
            </div>
          </div>
          <div className="p-5" style={{ background: "rgba(245,158,11,0.06)", borderLeft: "1px solid var(--yunque-border)" }}>
            <div className="mb-3 flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
              <ShieldAlert size={16} style={{ color: "var(--yunque-warning)" }} />
              当前边界
            </div>
            <div className="space-y-2 text-xs leading-5" style={{ color: "var(--yunque-text-secondary)" }}>
              <div>可以：生成电脑使用计划、读取浏览器截图证据。</div>
              <div>不会：移动鼠标、敲键盘、运行命令、写文件或控制本机桌面。</div>
              <div>原因：执行器、权限策略和人工审批运行时还没有开放。</div>
            </div>
          </div>
        </div>
      </Card>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
        {surfaceCards.map((item) => {
          const surface = status?.surfaces?.[item.key];
          const ready = item.key === "browser"
            ? Boolean(surface?.connected)
            : item.key === "desktop_sandbox"
              ? Boolean(surface?.available)
              : Boolean(surface?.available);
          return (
            <Card key={item.key} className="section-card p-4">
              <div className="mb-2 flex items-center justify-between gap-2">
                <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>{item.title}</div>
                <Chip size="sm" style={{ background: ready ? "rgba(34,197,94,0.10)" : "rgba(245,158,11,0.12)", color: ready ? "var(--yunque-success)" : "var(--yunque-warning)" }}>
                  {ready ? item.readyLabel : item.unavailableLabel}
                </Chip>
              </div>
              <div className="text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>{item.desc}</div>
              {surface?.status ? <div className="mt-2 text-[11px] font-mono" style={{ color: "var(--yunque-text-secondary)" }}>{surface.status}</div> : null}
            </Card>
          );
        })}
      </div>

      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_360px]">
        <Card className="section-card p-4 space-y-3">
          <div>
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>生成计划</div>
            <div className="mt-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>先让云雀说明准备怎么做，而不是直接动你的电脑。</div>
          </div>
          <TextField value={surface} onChange={setSurface} className="max-w-xs">
            <Label>目标界面</Label>
            <Input placeholder="auto / browser / desktop_sandbox" />
          </TextField>
          <div>
            <Label>你想让云雀做什么</Label>
            <TextArea value={goal} onChange={(event) => setGoal(event.target.value)} rows={3} />
          </div>
          <Button className="btn-accent w-fit" isDisabled={!goal.trim() || busy} onPress={createPlan}>
            {busy ? <Spinner size="sm" /> : <Play size={14} />} 生成计划
          </Button>
        </Card>

        <Card className="section-card p-4 space-y-3">
          <div>
            <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
              <Camera size={15} /> 浏览器截图证据
            </div>
            <div className="mt-1 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
              只在浏览器连接器已连接时可用；这一步只读取截图，不会点击、输入或改变页面。
            </div>
          </div>
          <Button variant="outline" isPending={screenshotLoading} onPress={captureBrowserScreenshot}>
            <Camera size={14} /> 读取截图
          </Button>
          {screenshot?.screenshot ? (
            <div>
              <img src={`data:image/png;base64,${screenshot.screenshot}`} alt="浏览器截图证据" className="w-full rounded-md" style={{ border: "1px solid var(--yunque-border)" }} />
              <div className="mt-2 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{screenshot.timestamp || "刚刚读取"} · 只读证据</div>
            </div>
          ) : (
            <div className="rounded-md border border-dashed p-6 text-center text-xs" style={{ borderColor: "var(--yunque-border)", color: "var(--yunque-text-muted)" }}>
              还没有截图证据。连接浏览器后可读取当前页面。
            </div>
          )}
        </Card>
      </div>

      {plan && (
        <Card className="section-card p-4 space-y-4">
          <div className="flex items-center justify-between gap-3">
            <div>
              <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>计划结果</div>
              <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>{plan.status}</div>
            </div>
            <Chip size="sm" style={{ background: plan.execution_ready ? "rgba(34,197,94,0.10)" : "rgba(245,158,11,0.12)", color: plan.execution_ready ? "var(--yunque-success)" : "var(--yunque-warning)" }}>
              {plan.execution_ready ? "可执行" : "仅计划"}
            </Chip>
          </div>

          {(plan.blocked_by?.length ?? 0) > 0 && (
            <div className="rounded-md p-3" style={{ background: "rgba(245,158,11,0.10)" }}>
              <div className="flex items-center gap-2 text-sm font-medium" style={{ color: "var(--yunque-warning)" }}>
                <ShieldAlert size={14} /> 为什么还不能自动执行
              </div>
              <div className="flex flex-wrap gap-1.5 mt-2">
                {plan.blocked_by?.map((item) => <Chip key={item} size="sm" variant="soft">{item}</Chip>)}
              </div>
            </div>
          )}

          <div className="space-y-2">
            {(plan.steps || []).map((step) => (
              <div key={step.index} className="rounded-md p-3 border" style={{ borderColor: "var(--yunque-border)" }}>
                <div className="flex items-center gap-2 text-sm" style={{ color: "var(--yunque-text)" }}>
                  <ShieldCheck size={14} style={{ color: "var(--yunque-success)" }} />
                  <span>{step.index}. {step.description}</span>
                  <Chip size="sm" variant="soft">{step.surface}</Chip>
                  {step.read_only && <Chip size="sm" style={{ background: "rgba(34,197,94,0.10)", color: "var(--yunque-success)" }}>只读</Chip>}
                </div>
                <div className="mt-2 flex flex-wrap gap-1.5">
                  <Chip size="sm" variant="soft">动作：{step.action}</Chip>
                  <Chip size="sm" variant="soft">权限：{step.permission}</Chip>
                  <Chip size="sm" variant="soft">执行器：{step.executor}</Chip>
                </div>
              </div>
            ))}
          </div>

          {(plan.gates?.length ?? 0) > 0 && (
            <div className="grid gap-2 md:grid-cols-2">
              {plan.gates?.map((gate) => (
                <div key={gate.gate} className="rounded-md border p-3" style={{ borderColor: "var(--yunque-border)" }}>
                  <div className="mb-2 flex items-center justify-between gap-2">
                    <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{explainGate(gate.gate)}</div>
                    <Chip size="sm" style={{ background: gate.allows_execute ? "rgba(34,197,94,0.10)" : "rgba(245,158,11,0.12)", color: gate.allows_execute ? "var(--yunque-success)" : "var(--yunque-warning)" }}>
                      {gate.allows_execute ? "允许执行" : "暂不执行"}
                    </Chip>
                  </div>
                  <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                    {gate.human_approval ? "需要人工确认" : "无需人工确认"} · {gate.policy_enforced ? "策略已启用" : "策略未启用"}
                  </div>
                </div>
              ))}
            </div>
          )}

          {(plan.notes?.length ?? 0) > 0 && (
            <div className="text-xs space-y-1" style={{ color: "var(--yunque-text-muted)" }}>
              {plan.notes?.map((note) => <div key={note}>• {note}</div>)}
            </div>
          )}
        </Card>
      )}
    </div>
  );
}
