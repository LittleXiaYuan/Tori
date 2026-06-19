"use client";

import Link from "next/link";
import { Button, Card, Chip } from "@heroui/react";
import { BrainCircuit, ExternalLink, Route, ShieldCheck } from "lucide-react";
import PageHeader from "@/components/page-header";

const kernelActions = [
  {
    title: "管理 Cogni 声明",
    desc: "在 Cogni 管理页创建、启用、停用、校验和导入导出声明。",
  },
  {
    title: "观察路由与健康",
    desc: "查看 Planner 选择 Cogni 的 trace、健康检查、告警和运行状态。",
  },
  {
    title: "连接能力包状态",
    desc: "让 Cogni 知道哪些能力包可用、哪些被停用，从而减少无效调用。",
  },
];

const boundaryItems = [
  "不会替代能力包本身的安装、权限授权或启用流程。",
  "不会直接执行本机电脑控制、联网写入或高风险动作。",
  "不会绕过 Pack Runtime 门禁；能力包停用后 Cogni 只能看到受限状态。",
  "更适合作为底层运行治理包，普通用户优先从 /cognis 管理 Cogni。",
];

export default function PacksCognisPage() {
  return (
    <div className="page-root space-y-5 animate-fade-in-up">
      <PageHeader
        icon={<BrainCircuit size={20} />}
        title="Cogni 内核"
        description="把 Cogni 声明、路由、健康检查和能力包状态连到 Planner 的底层治理能力包。"
        actions={(
          <Link href="/cognis">
            <Button className="btn-accent">
              <ExternalLink size={14} /> 打开 Cogni 管理
            </Button>
          </Link>
        )}
      />

      <Card className="section-card overflow-hidden p-0">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_320px]">
          <div className="p-5">
            <div className="mb-3 flex flex-wrap items-center gap-2">
              <Chip size="sm" color="success">默认启用</Chip>
              <Chip size="sm" variant="soft">基础能力</Chip>
              <Chip size="sm" variant="soft">Cogni / Planner</Chip>
            </div>
            <div className="text-base font-semibold" style={{ color: "var(--yunque-text)" }}>
              这个能力包现在能做什么
            </div>
            <div className="mt-2 max-w-3xl text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>
              它不是一个单独给用户日常操作的应用，而是 Cogni 的运行内核：负责声明注册、路由选择、健康检查、运行轨迹和能力包状态门禁。你真正管理 Cogni 的入口在「我的 Cogni」页面。
            </div>
            <div className="mt-4 grid gap-3 md:grid-cols-3">
              {kernelActions.map((item) => (
                <div key={item.title} className="rounded-lg p-3" style={{ background: "var(--yunque-bg-hover)", border: "1px solid var(--yunque-border)" }}>
                  <div className="mb-2 flex items-center gap-2 text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
                    <Route size={14} style={{ color: "var(--yunque-accent)" }} />
                    {item.title}
                  </div>
                  <div className="text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>{item.desc}</div>
                </div>
              ))}
            </div>
          </div>
          <div className="p-5" style={{ background: "rgba(245,158,11,0.06)", borderLeft: "1px solid var(--yunque-border)" }}>
            <div className="mb-3 flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
              <ShieldCheck size={16} style={{ color: "var(--yunque-warning)" }} />
              当前边界
            </div>
            <div className="space-y-2 text-xs leading-5" style={{ color: "var(--yunque-text-secondary)" }}>
              {boundaryItems.map((item) => <div key={item}>{item}</div>)}
            </div>
          </div>
        </div>
      </Card>

      <Card className="section-card p-4">
        <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>下一步去哪</div>
        <div className="mt-2 text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>
          想创建或编辑 Cogni，请打开「我的 Cogni」；想看具体能力包是否可用，请回到能力包中心或对应功能页。这个页面只解释 Cogni Kernel 与能力包、Planner 的关系。
        </div>
        <div className="mt-3">
          <Link href="/cognis">
            <Button variant="outline" size="sm">
              打开 Cogni 管理
            </Button>
          </Link>
        </div>
      </Card>
    </div>
  );
}
