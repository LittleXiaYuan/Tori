"use client";

import Link from "next/link";
import { Button, Card, Chip } from "@heroui/react";
import { BrainCircuit, ExternalLink, Route, ShieldCheck } from "lucide-react";
import PageHeader from "@/components/page-header";
import { PackAbout, PackSectionTitle, type PackBoundaryItem } from "@/components/packs/pack-page-kit";

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

const boundaryItems: PackBoundaryItem[] = [
  { key: "install", label: "不替代安装授权", detail: "不会替代能力包本身的安装、权限授权或启用流程。" },
  { key: "ecosystem", label: "不吞 Skill / MCP", detail: "不会假装已经吞掉 Skill 或 MCP 生态；第一阶段是兼容、组织和减少无效上下文。" },
  { key: "exec", label: "不执行高风险动作", detail: "不会直接执行本机电脑控制、联网写入或高风险动作。" },
  { key: "gate", label: "不绕过门禁", detail: "不会绕过 Pack Runtime 门禁；能力包停用后 Cogni 只能看到受限状态。" },
  { key: "audience", label: "面向底层治理", detail: "更适合作为底层运行治理包，普通用户优先从 /cognis 管理 Cogni。" },
];

const relationItems = [
  {
    term: "能力包",
    desc: "扩展云雀底座：安装、启用、权限、路由、前端界面、WASM/DLC 都在这里被治理。",
  },
  {
    term: "Cogni",
    desc: "增设模型可选择的能力声明：把技能、MCP、能力包、记忆和经验组织成更省上下文的调用线索。",
  },
  {
    term: "Skill / MCP",
    desc: "外部生态入口：Cogni 应该兼容并观察它们的可用性，而不是在当前阶段宣称完全替代。",
  },
];

export default function PacksCognisPage() {
  return (
    <div className="page-root space-y-6 animate-fade-in-up">
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

      <PackAbout
        chips={<>
          <Chip size="sm" color="success">默认启用</Chip>
          <Chip size="sm" variant="soft">基础能力</Chip>
          <Chip size="sm" variant="soft">Cogni / Planner</Chip>
        </>}
        description="它不是一个单独给用户日常操作的应用，而是 Cogni 的运行内核：负责声明注册、路由选择、健康检查、运行轨迹和能力包状态门禁。Cogni 不是能力包的替代品，它更像模型侧的能力目录和选择层；你真正管理 Cogni 的入口在「我的 Cogni」页面。"
        boundaries={boundaryItems}
      />

      <Card variant="default">
        <Card.Header className="flex-row items-center gap-2">
          <PackSectionTitle icon={<Route size={15} />} tone="accent">这个能力包现在能做什么</PackSectionTitle>
        </Card.Header>
        <Card.Content>
          <div className="grid gap-3 md:grid-cols-3">
            {kernelActions.map((item) => (
              <div key={item.title} className="rounded-xl bg-surface-secondary px-4 py-3 text-sm leading-6 text-muted">
                <div className="mb-1.5 flex items-center gap-2 font-semibold text-foreground">
                  <Route size={14} className="text-accent" />
                  {item.title}
                </div>
                <div className="text-xs leading-5 text-muted">{item.desc}</div>
              </div>
            ))}
          </div>
        </Card.Content>
      </Card>

      <Card variant="default">
        <Card.Header className="flex-row items-center gap-2">
          <PackSectionTitle icon={<ShieldCheck size={15} />} tone="accent">Pack / Cogni / Skill / MCP 的关系</PackSectionTitle>
        </Card.Header>
        <Card.Content>
          <dl className="grid gap-3 md:grid-cols-3">
            {relationItems.map((item) => (
              <div key={item.term} className="rounded-xl bg-surface-secondary px-4 py-3">
                <dt className="text-sm font-medium text-foreground">{item.term}</dt>
                <dd className="mt-1 text-xs leading-5 text-muted">{item.desc}</dd>
              </div>
            ))}
          </dl>
        </Card.Content>
      </Card>

      <Card variant="default">
        <Card.Header className="flex-row items-center gap-2">
          <PackSectionTitle tone="accent">下一步去哪</PackSectionTitle>
        </Card.Header>
        <Card.Content className="flex flex-col gap-3">
          <div className="text-sm leading-6 text-muted">
            想创建或编辑 Cogni，请打开「我的 Cogni」；想看具体能力包是否可用，请回到能力包中心或对应功能页。这个页面只解释 Cogni Kernel 与能力包、Planner 的关系。
          </div>
          <div>
            <Link href="/cognis">
              <Button variant="outline" size="sm">
                打开 Cogni 管理
              </Button>
            </Link>
          </div>
        </Card.Content>
      </Card>
    </div>
  );
}
