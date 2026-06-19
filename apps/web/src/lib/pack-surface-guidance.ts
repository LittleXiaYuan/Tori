import {
  Brain,
  BriefcaseBusiness,
  Cpu,
  Database,
  Gauge,
  GraduationCap,
  Network,
  PackageCheck,
  ShieldCheck,
  Sparkles,
  Wrench,
} from "lucide-react";
import type { ComponentType } from "react";

export interface PackSurfaceItem {
  id: string;
  title: string;
  kind: "可操作" | "基础能力" | "治理";
  summary: string;
  actions: string[];
}

export interface PackSurfaceGuide {
  title: string;
  description: string;
  icon: ComponentType<{ size?: number; "aria-hidden"?: boolean }>;
  items: PackSurfaceItem[];
}

export const packSurfaceGuides = {
  memory: {
    title: "这里承接的能力包",
    description: "记忆不是一个孤立页面，它承接长期偏好、情绪线索和任务复盘结果，让 Chat 与任务能带着上下文继续工作。",
    icon: Brain,
    items: [
      {
        id: "yunque.pack.memory",
        title: "记忆",
        kind: "可操作",
        summary: "查看、搜索、添加和整理短期/中期/长期记忆。",
        actions: ["搜索一条记忆", "手动添加重要偏好", "整理压缩旧记忆"],
      },
      {
        id: "yunque.pack.emotion",
        title: "情绪上下文",
        kind: "基础能力",
        summary: "把对话和渠道消息里的情绪信号沉淀到记忆视图。",
        actions: ["查看情感历史", "确认近期互动倾向", "辅助 Chat 调整语气"],
      },
      {
        id: "yunque.pack.reflection",
        title: "反思",
        kind: "基础能力",
        summary: "把任务反馈和经验策略转成后续可召回的记忆线索。",
        actions: ["查看任务沉淀", "配合夜校复盘", "把经验带回下一次任务"],
      },
    ],
  },
  knowledge: {
    title: "这里承接的能力包",
    description: "知识库负责把外部材料变成可检索上下文，供 Chat、任务和 Cogni 使用。",
    icon: Database,
    items: [
      {
        id: "yunque.pack.knowledge",
        title: "知识库",
        kind: "可操作",
        summary: "导入文件、网页、仓库或纯文本，并管理知识源。",
        actions: ["上传文件", "导入 URL/仓库", "编辑或删除知识源"],
      },
      {
        id: "yunque.pack.retrieval",
        title: "检索",
        kind: "基础能力",
        summary: "把知识片段召回到 Chat 与任务上下文里。",
        actions: ["搜索知识片段", "用筛选缩小范围", "让任务引用知识库"],
      },
      {
        id: "yunque.pack.graph",
        title: "图谱上下文",
        kind: "基础能力",
        summary: "把知识和记忆关系变成可复用的上下文结构。",
        actions: ["检查知识覆盖", "辅助 RAG 召回", "为 Cogni 提供背景"],
      },
    ],
  },
  missions: {
    title: "这里承接的能力包",
    description: "任务中心是工作类能力包的共同入口：从 Chat 发起任务，在这里查看进度、自动化、产物和可恢复现场。",
    icon: BriefcaseBusiness,
    items: [
      {
        id: "yunque.pack.missions",
        title: "任务",
        kind: "可操作",
        summary: "查看任务状态、继续运行、暂停、取消或重启任务。",
        actions: ["查看进行中任务", "打开任务执行页", "处理失败任务"],
      },
      {
        id: "yunque.pack.work",
        title: "工作流",
        kind: "可操作",
        summary: "把项目、工作流、任务执行和产物区连成闭环。",
        actions: ["从模板创建任务", "查看产物", "回到 Chat 继续处理"],
      },
      {
        id: "yunque.pack.scheduler",
        title: "调度与自动化",
        kind: "基础能力",
        summary: "支撑定时任务、触发器、队列、恢复现场和子代理分工。",
        actions: ["创建定时任务", "配置触发器", "恢复暂停现场"],
      },
    ],
  },
  skills: {
    title: "这里承接的能力包",
    description: "技能页承接 Skill 生态：本地技能、市场安装、动态技能审批，以及云雀在任务中实际会调用的工具说明。",
    icon: Wrench,
    items: [
      {
        id: "yunque.pack.skills",
        title: "技能",
        kind: "可操作",
        summary: "查看已安装技能、扫描本地技能目录，并观察使用量和成功率。",
        actions: ["扫描 data/skills", "按类别筛选", "查看使用指标"],
      },
      {
        id: "yunque.pack.market",
        title: "技能市场",
        kind: "基础能力",
        summary: "从本地或远程来源发现可安装技能。",
        actions: ["浏览市场", "搜索技能", "安装社区技能"],
      },
      {
        id: "yunque.pack.skillhub",
        title: "远程技能生态",
        kind: "基础能力",
        summary: "连接 ClawHub/ToriHub，并对远程技能做安全策略检查。",
        actions: ["切换来源", "安装 GitHub 技能", "处理动态技能审批"],
      },
    ],
  },
  workers: {
    title: "这里承接的能力包",
    description: "AI IDE 协作页承接外部执行器：云雀负责派发任务，Cursor、Claude Code、Windsurf 等工具负责实际编码和回报结果。",
    icon: Cpu,
    items: [
      {
        id: "yunque.pack.mcp-dispatch",
        title: "MCP 分派",
        kind: "可操作",
        summary: "管理 Worker MCP 配置和外部执行器列表。",
        actions: ["连接 AI IDE", "复制 MCP 配置", "移除离线执行器"],
      },
      {
        id: "yunque.pack.orchestrator",
        title: "编排守护进程",
        kind: "可操作",
        summary: "检测可用 IDE、启动守护进程，并查看活跃会话。",
        actions: ["检测 IDE", "启动/停止守护", "查看分派状态"],
      },
    ],
  },
  cognis: {
    title: "这里承接的能力包",
    description: "Cogni 是给模型增设可选择能力的层：它会把技能、工作流、经验和触发方式组织成更省上下文的助手声明。",
    icon: Sparkles,
    items: [
      {
        id: "yunque.pack.cogni-console",
        title: "Cogni 控制台",
        kind: "可操作",
        summary: "用一句话创建 Cogni，并查看健康、轨迹、经验和演化。",
        actions: ["新建 Cogni", "启用/停用声明", "查看运行轨迹"],
      },
      {
        id: "yunque.pack.cogni-kernel",
        title: "Cogni 内核",
        kind: "基础能力",
        summary: "让 Planner 能选择 Cogni、读取声明，并把结果写回运行状态。",
        actions: ["检查内核状态", "导入/导出声明", "观察能力包门禁"],
      },
      {
        id: "yunque.pack.cognitive-layer",
        title: "认知层",
        kind: "基础能力",
        summary: "把记忆、反思、夜校和 Cogni 组合成可召回的认知上下文。",
        actions: ["查看经验模式", "确认演化结果", "关联记忆与任务"],
      },
    ],
  },
  dashboard: {
    title: "这里承接的能力包",
    description: "工作台不是能力包本身，而是云雀启动后的总览入口：看状态、最近任务、成本和运行健康。",
    icon: Gauge,
    items: [
      {
        id: "yunque.pack.workspace",
        title: "工作台",
        kind: "可操作",
        summary: "汇总启动状态、最近任务、提醒和主路径入口。",
        actions: ["开始对话", "进入任务中心", "检查配置提醒"],
      },
      {
        id: "yunque.pack.heartbeat",
        title: "心跳",
        kind: "基础能力",
        summary: "提供自主运行、后台执行和运行日志的状态信号。",
        actions: ["观察运行状态", "排查离线", "回到记忆查看心跳日志"],
      },
      {
        id: "yunque.pack.modules",
        title: "模块诊断",
        kind: "基础能力",
        summary: "展示运行配置档、模块状态和基础诊断信息。",
        actions: ["查看版本", "检查指标", "确认服务在线"],
      },
    ],
  },
  trust: {
    title: "这里承接的能力包",
    description: "信任中心承接高权限能力治理，控制工具、联网、写入、远程包和模型配置的授权边界。",
    icon: ShieldCheck,
    items: [
      {
        id: "yunque.pack.control-plane",
        title: "控制面",
        kind: "治理",
        summary: "查看审批、审计、指标、模型和高风险能力状态。",
        actions: ["处理审批", "查看审计", "检查运行健康"],
      },
      {
        id: "yunque.pack.rbac",
        title: "权限",
        kind: "基础能力",
        summary: "为管理员授权和权限检查提供基础信任分数。",
        actions: ["查看信任分", "授予临时信任", "重置异常权限"],
      },
    ],
  },
} satisfies Record<string, PackSurfaceGuide>;

export type PackSurfaceKey = keyof typeof packSurfaceGuides;

export function packSurfaceGuide(surface: PackSurfaceKey): PackSurfaceGuide {
  return packSurfaceGuides[surface];
}

export function allGuidedPackIDs(): string[] {
  return Object.values(packSurfaceGuides).flatMap((guide) => guide.items.map((item) => item.id));
}
