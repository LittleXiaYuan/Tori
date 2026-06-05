/**
 * 全局导航条目，被 `command-palette`（⌘K）和 `account-rail` 的 hover flyout
 * 同时复用。默认面遵循「场景 → 行动 → 产物 → 反馈 → 记忆」主路径；
 * 启用后的 Pack 菜单也会进入主路径，未启用或高级能力仍保留在 Pack、Lab
 * 或控制面里，避免继续平铺。
 */
import type React from "react";
import {
  MessageCircle, Zap, BookOpen, ScanFace, Package, Settings,
  MailWarning, Puzzle, Brain, BrainCircuit,
  Shield, ShieldCheck, BarChart3, Globe, Blocks,
  Terminal, Cpu, LayoutDashboard, Wrench, SmilePlus, HeartPulse,
  Lightbulb, Share2, Bot, FolderGit2, Boxes, Users, Workflow,
} from "lucide-react";
import type { ProfileMode } from "@/lib/profile-mode";

export type NavGroup = "概览" | "工作" | "智能" | "系统" | "扩展";
export type CapabilityLayer = "core" | "pack" | "lab" | "control-plane";

export const CAPABILITY_LAYER_LABELS: Record<CapabilityLayer, string> = {
  core: "主路径",
  pack: "能力包",
  lab: "实验室",
  "control-plane": "控制面",
};

export interface NavItem {
  id: string;
  href: string;
  label: string;
  group: NavGroup;
  layer: CapabilityLayer;
  /** Visible in the default "easy" surface. Everything else is progressive discovery. */
  defaultVisible?: boolean;
  icon: React.ReactNode;
  keywords?: string;
}

export const DEFAULT_NAV_ITEM_IDS = new Set<string>([
  "nav-dashboard",
  "nav-chat",
  "nav-missions",
  "nav-knowledge",
  "nav-memory",
  "nav-packs",
  "nav-cognis",
  "nav-settings",
]);

export const NAV_ITEMS: NavItem[] = [
  // 概览 - 核心功能（easy 模式可见）
  { id: "nav-dashboard", href: "/dashboard", label: "工作台", group: "概览", layer: "core", defaultVisible: true, icon: <LayoutDashboard size={16} />, keywords: "dashboard home 主页 仪表盘 overview workspace 工作台 场景" },
  { id: "nav-chat", href: "/chat", label: "对话", group: "概览", layer: "core", defaultVisible: true, icon: <MessageCircle size={16} />, keywords: "chat 聊天 会话 行动 产物" },

  // 工作 - 核心功能
  { id: "nav-missions", href: "/missions", label: "任务中心", group: "工作", layer: "core", defaultVisible: true, icon: <Zap size={16} />, keywords: "missions tasks 任务 反馈 验收" },

  // 工作 - 辅助功能（full 模式）
  { id: "nav-task-run", href: "/task-run", label: "执行视图", group: "工作", layer: "core", icon: <Terminal size={16} />, keywords: "task run 执行 运行" },
  { id: "nav-projects", href: "/projects", label: "项目", group: "工作", layer: "core", icon: <FolderGit2 size={16} />, keywords: "projects 项目 repo workspace" },
  { id: "nav-workflows", href: "/workflows", label: "工作流", group: "工作", layer: "lab", icon: <Blocks size={16} />, keywords: "workflow 工作流 流程 lab 实验" },
  { id: "nav-workers", href: "/workers", label: "Worker", group: "工作", layer: "control-plane", icon: <Cpu size={16} />, keywords: "workers worker 进程 运维" },

  // 智能 - 核心功能
  { id: "nav-knowledge", href: "/knowledge", label: "知识库", group: "智能", layer: "core", defaultVisible: true, icon: <BookOpen size={16} />, keywords: "knowledge 知识 RAG" },
  { id: "nav-memory", href: "/memory", label: "记忆", group: "智能", layer: "core", defaultVisible: true, icon: <Brain size={16} />, keywords: "memory 记忆 反馈 沉淀" },

  // 扩展 - 核心功能
  { id: "nav-packs", href: "/packs", label: "能力包", group: "扩展", layer: "pack", defaultVisible: true, icon: <Boxes size={16} />, keywords: "packs pack runtime 增量包 能力包 热插拔 可选能力 默认入口" },
  { id: "nav-cognis", href: "/cognis", label: "Cogni", group: "扩展", layer: "core", defaultVisible: true, icon: <BrainCircuit size={16} />, keywords: "Cogni cognis 助手 assistant 智体 认知内核 我的 Cogni" },

  // 扩展 - 开发工具（full 模式）
  { id: "nav-skills", href: "/skills", label: "技能库", group: "扩展", layer: "lab", icon: <Package size={16} />, keywords: "skills 技能 运行时技能 高级入口 legacy pack 的原子能力来源" },
  { id: "nav-plugins", href: "/plugins", label: "插件宿主", group: "扩展", layer: "control-plane", icon: <Puzzle size={16} />, keywords: "plugins 插件 宿主 运行时 高级入口 legacy pack 的代码载体" },

  // 系统 - 核心功能
  { id: "nav-settings", href: "/settings", label: "设置", group: "系统", layer: "core", defaultVisible: true, icon: <Settings size={16} />, keywords: "settings 偏好 设置" },

  // 系统 - 控制面（full 模式）
  { id: "nav-inbox", href: "/inbox", label: "收件箱", group: "系统", layer: "control-plane", icon: <MailWarning size={16} />, keywords: "inbox 消息 通知 渠道 控制面" },
  { id: "nav-tools", href: "/tools", label: "终端", group: "系统", layer: "control-plane", icon: <Wrench size={16} />, keywords: "tools terminal shell 工具 运维" },
  { id: "nav-models", href: "/models", label: "模型", group: "系统", layer: "control-plane", icon: <Cpu size={16} />, keywords: "models 模型 LLM 控制面" },
  { id: "nav-providers", href: "/settings/providers", label: "提供商", group: "系统", layer: "control-plane", icon: <Globe size={16} />, keywords: "providers 提供商 api key 控制面" },
  { id: "nav-metrics", href: "/metrics", label: "指标", group: "系统", layer: "control-plane", icon: <BarChart3 size={16} />, keywords: "metrics 统计 指标 控制面" },
  { id: "nav-approvals", href: "/approvals", label: "审批", group: "系统", layer: "control-plane", icon: <ShieldCheck size={16} />, keywords: "approvals 审批 控制面" },
  { id: "nav-audit", href: "/audit", label: "审计", group: "系统", layer: "control-plane", icon: <Shield size={16} />, keywords: "audit 日志 审计 控制面" },
  { id: "nav-trust", href: "/trust", label: "信任", group: "系统", layer: "control-plane", icon: <ShieldCheck size={16} />, keywords: "trust 安全 信任 控制面" },
  { id: "nav-tenants", href: "/tenants", label: "租户", group: "系统", layer: "control-plane", icon: <Users size={16} />, keywords: "tenants 租户 团队 控制面" },
  { id: "nav-bots", href: "/bots", label: "Bot", group: "系统", layer: "control-plane", icon: <Bot size={16} />, keywords: "bots bot 机器人 渠道 控制面" },
];

export const NAV_GROUP_ORDER: NavGroup[] = ["概览", "工作", "智能", "系统", "扩展"];

export function filterNavItemsByProfile(items: NavItem[], mode: ProfileMode): NavItem[] {
  if (mode === "full") return items;
  return items.filter((item) => item.defaultVisible === true);
}

export function groupNavItems(items: NavItem[]): Record<NavGroup, NavItem[]> {
  const out: Record<NavGroup, NavItem[]> = {
    概览: [], 工作: [], 智能: [], 系统: [], 扩展: [],
  };
  for (const it of items) out[it.group].push(it);
  return out;
}
