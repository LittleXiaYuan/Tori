"use client";

import { useEffect, useState } from "react";
import { Button, Card, Chip, Input, Label, Spinner, TextArea, TextField } from "@heroui/react";
import { MonitorCog, Play, RotateCcw, ShieldAlert, ShieldCheck } from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { createYunqueSDKClientOptions } from "@/lib/sdk-client";
import { formatErrorMessage } from "@/lib/error-utils";

const sdkOptions = createYunqueSDKClientOptions();

type ComputerStatus = {
  execution_ready?: boolean;
  surfaces?: Record<string, unknown>;
  safety?: Record<string, unknown>;
  capabilities?: string[];
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
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);

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

      <Card className="section-card p-4 flex items-start gap-3">
        <ShieldAlert size={18} style={{ color: "var(--yunque-warning)" }} />
        <div>
          <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>当前边界</div>
          <div className="text-sm mt-1" style={{ color: "var(--yunque-text-secondary)" }}>
            这个能力包处于实验阶段：Planner 可以生成电脑使用计划，浏览器连接后可读取截图；不会移动鼠标、敲键盘、运行命令、写文件或控制本机桌面。
          </div>
        </div>
      </Card>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
        <Card className="section-card p-4">
          <div className="kpi-label">执行器</div>
          <div className="kpi-value">{status?.execution_ready ? "可执行" : "未接入"}</div>
          <div className="kpi-sub">execution_ready=false 是当前安全边界</div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">能力</div>
          <div className="kpi-value">{status?.capabilities?.length || 0}</div>
          <div className="kpi-sub">status / intent plan / browser screenshot</div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">安全</div>
          <div className="flex flex-wrap gap-1.5 mt-2">
            <Chip size="sm" style={{ background: "rgba(34,197,94,0.10)", color: "var(--yunque-success)" }}>只读优先</Chip>
            <Chip size="sm" style={{ background: "rgba(245,158,11,0.12)", color: "var(--yunque-warning)" }}>需要人工确认</Chip>
          </div>
        </Card>
      </div>

      <Card className="section-card p-4 space-y-3">
        <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>生成计划</div>
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
                <ShieldAlert size={14} /> 暂不执行的原因
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
                  <span>{step.index}. {step.action}</span>
                  <Chip size="sm" variant="soft">{step.surface}</Chip>
                  {step.read_only && <Chip size="sm" style={{ background: "rgba(34,197,94,0.10)", color: "var(--yunque-success)" }}>只读</Chip>}
                </div>
                <div className="text-xs mt-2" style={{ color: "var(--yunque-text-muted)" }}>{step.description}</div>
              </div>
            ))}
          </div>

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
