"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Button, Card, Chip, Label, ListBox, Select, Spinner, TextArea } from "@heroui/react";
import { Camera, ClipboardList, MonitorCog, Play, RotateCcw, Send, ShieldAlert, ShieldCheck } from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { createYunqueSDKClientOptions } from "@/lib/sdk-client";
import { formatErrorMessage } from "@/lib/error-utils";
import { chatPromptHref } from "@/lib/pack-action-links";
import { PackAbout, PackSectionTitle, PackStepsGrid, type PackBoundaryItem, type PackStep } from "@/components/packs/pack-page-kit";

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

const workflowSteps: PackStep[] = [
  { key: "plan", label: "生成计划", detail: "把目标拆成界面、只读证据、权限和阻塞原因，先让用户看懂它准备怎么做。" },
  { key: "chat", label: "带回 Chat", detail: "让云雀解释计划、补充缺失信息，或把可执行部分拆成需要人工确认的任务。" },
  { key: "evidence", label: "核对证据", detail: "浏览器截图和计划结果只作为证据，不代表云雀已经控制了电脑。" },
  { key: "improve", label: "继续交给小羽改", detail: "如果用户需要云桌面、审批或站点适配，把缺口带进工坊继续补能力。" },
];

const boundaryItems: PackBoundaryItem[] = [
  { key: "control", label: "不控制本机", detail: "不会移动鼠标、敲键盘、运行命令、写文件或控制本机桌面。", tone: "warning" },
  { key: "execute", label: "不自动执行", detail: "执行器、权限策略和人工审批运行时还没有开放，当前只规划。", tone: "warning" },
];

const surfaceOptions = [
  { id: "auto", label: "自动选择", description: "让后端按当前能力状态选择安全界面" },
  { id: "browser", label: "浏览器截图", description: "只读浏览器页面与截图证据" },
  { id: "desktop_sandbox", label: "云桌面规划", description: "隔离桌面环境，当前仅规划" },
  { id: "local_desktop", label: "本机桌面规划", description: "当前关闭真实执行，仅生成计划" },
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

      <PackAbout
        chips={<>
          <Chip size="sm" color="success">可直接使用</Chip>
          <Chip size="sm" variant="soft">规划电脑操作</Chip>
          <Chip size="sm" variant="soft">浏览器只读取证</Chip>
        </>}
        description="云雀 Computer Use 的第一层：先把“帮我操作电脑”的目标变成可审阅计划，确认要看哪个界面、需要什么权限、为什么暂不执行；浏览器连接后还能读取一张只读截图作为证据。真实执行需先经过权限策略、人工确认和执行器接入。"
        boundaries={boundaryItems}
      />

      <Card variant="default">
        <Card.Header className="flex-row flex-wrap items-start justify-between gap-3">
          <div className="flex flex-col gap-1">
            <PackSectionTitle icon={<MonitorCog size={15} />} tone="accent">从电脑使用计划到可验证任务</PackSectionTitle>
            <span className="text-xs leading-5 text-muted">
              Computer Use 先帮你把“让云雀操作电脑”的目标变成可审阅计划和只读证据，再交给 Chat、任务中心和小羽决定下一步，而不是直接接管本机。
            </span>
          </div>
          <div className="flex flex-wrap gap-2">
            <Link href={chatPromptHref("请检查 Computer Use 当前生成的计划和浏览器截图证据，指出哪些步骤可以安全推进、哪些需要人工确认，并把下一步拆成任务。")}>
              <Button size="sm" className="btn-accent">
                <Send size={13} /> 带回 Chat
              </Button>
            </Link>
            <Link href="/missions">
              <Button size="sm" variant="outline">
                <ClipboardList size={13} /> 看任务
              </Button>
            </Link>
          </div>
        </Card.Header>
        <Card.Content className="flex flex-col gap-3">
          <PackStepsGrid steps={workflowSteps} columns={4} />
          <div className="flex flex-wrap gap-2 text-xs">
            <Link href="/trace"><Button size="sm" variant="ghost">核对执行轨迹</Button></Link>
            <Link href="/packs/studio?packId=yunque.pack.computer-use"><Button size="sm" variant="ghost">让小羽继续改</Button></Link>
          </div>
        </Card.Content>
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
            <Card key={item.key} variant="default">
              <Card.Content className="p-4">
                <div className="mb-2 flex items-center justify-between gap-2">
                  <div className="text-sm font-semibold text-foreground">{item.title}</div>
                  <Chip size="sm" color={ready ? "success" : "warning"}>
                    {ready ? item.readyLabel : item.unavailableLabel}
                  </Chip>
                </div>
                <div className="text-xs leading-5 text-muted">{item.desc}</div>
                {surface?.status ? <div className="mt-2 font-mono text-[11px] text-muted">{surface.status}</div> : null}
              </Card.Content>
            </Card>
          );
        })}
      </div>

      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_360px]">
        <Card variant="default">
          <Card.Content className="flex flex-col gap-3 p-4">
            <div>
              <div className="text-sm font-semibold text-foreground">生成计划</div>
              <div className="mt-1 text-xs text-muted">先让云雀说明准备怎么做，而不是直接动你的电脑。</div>
            </div>
            <Select
              selectedKey={surface}
              onSelectionChange={(key) => setSurface(String(key))}
              className="max-w-xs"
            >
              <Label>目标界面</Label>
              <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
              <Select.Popover>
                <ListBox>
                  {surfaceOptions.map((option) => (
                    <ListBox.Item key={option.id} id={option.id} textValue={option.label}>
                      <div className="flex flex-col">
                        <span>{option.label}</span>
                        <span className="text-xs text-muted">{option.description}</span>
                      </div>
                    </ListBox.Item>
                  ))}
                </ListBox>
              </Select.Popover>
            </Select>
            <div>
              <Label>你想让云雀做什么</Label>
              <TextArea value={goal} onChange={(event) => setGoal(event.target.value)} rows={3} />
            </div>
            <Button className="btn-accent w-fit" isDisabled={!goal.trim() || busy} onPress={createPlan}>
              {busy ? <Spinner size="sm" /> : <Play size={14} />} 生成计划
            </Button>
          </Card.Content>
        </Card>

        <Card variant="default">
          <Card.Content className="flex flex-col gap-3 p-4">
            <div>
              <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
                <Camera size={15} /> 浏览器截图证据
              </div>
              <div className="mt-1 text-xs leading-5 text-muted">
                只在浏览器连接器已连接时可用；这一步只读取截图，不会点击、输入或改变页面。
              </div>
            </div>
            <Button variant="outline" isPending={screenshotLoading} onPress={captureBrowserScreenshot}>
              <Camera size={14} /> 读取截图
            </Button>
            {screenshot?.screenshot ? (
              <div>
                <img src={`data:image/png;base64,${screenshot.screenshot}`} alt="浏览器截图证据" className="w-full rounded-md border border-border" />
                <div className="mt-2 text-[11px] text-muted">{screenshot.timestamp || "刚刚读取"} · 只读证据</div>
              </div>
            ) : (
              <div className="rounded-md border border-dashed border-border p-6 text-center text-xs text-muted">
                还没有截图证据。连接浏览器后可读取当前页面。
              </div>
            )}
          </Card.Content>
        </Card>
      </div>

      {plan && (
        <Card variant="default">
          <Card.Content className="flex flex-col gap-4 p-4">
            <div className="flex items-center justify-between gap-3">
              <div>
                <div className="text-sm font-semibold text-foreground">计划结果</div>
                <div className="mt-1 text-xs text-muted">{plan.status}</div>
              </div>
              <Chip size="sm" color={plan.execution_ready ? "success" : "warning"}>
                {plan.execution_ready ? "可执行" : "仅计划"}
              </Chip>
            </div>

            {(plan.blocked_by?.length ?? 0) > 0 && (
              <div className="rounded-md bg-warning/10 p-3">
                <div className="flex items-center gap-2 text-sm font-medium text-warning">
                  <ShieldAlert size={14} /> 为什么还不能自动执行
                </div>
                <div className="mt-2 flex flex-wrap gap-1.5">
                  {plan.blocked_by?.map((item) => <Chip key={item} size="sm" variant="soft">{item}</Chip>)}
                </div>
              </div>
            )}

            <div className="flex flex-col gap-2">
              {(plan.steps || []).map((step) => (
                <div key={step.index} className="rounded-md border border-border p-3">
                  <div className="flex items-center gap-2 text-sm text-foreground">
                    <ShieldCheck size={14} className="text-success" />
                    <span>{step.index}. {step.description}</span>
                    <Chip size="sm" variant="soft">{step.surface}</Chip>
                    {step.read_only && <Chip size="sm" color="success">只读</Chip>}
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
                  <div key={gate.gate} className="rounded-md border border-border p-3">
                    <div className="mb-2 flex items-center justify-between gap-2">
                      <div className="text-sm font-medium text-foreground">{explainGate(gate.gate)}</div>
                      <Chip size="sm" color={gate.allows_execute ? "success" : "warning"}>
                        {gate.allows_execute ? "允许执行" : "暂不执行"}
                      </Chip>
                    </div>
                    <div className="text-xs text-muted">
                      {gate.human_approval ? "需要人工确认" : "无需人工确认"} · {gate.policy_enforced ? "策略已启用" : "策略未启用"}
                    </div>
                  </div>
                ))}
              </div>
            )}

            {(plan.notes?.length ?? 0) > 0 && (
              <div className="flex flex-col gap-1 text-xs text-muted">
                {plan.notes?.map((note) => <div key={note}>• {note}</div>)}
              </div>
            )}
          </Card.Content>
        </Card>
      )}
    </div>
  );
}
