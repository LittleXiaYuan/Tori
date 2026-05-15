/**
 * 全局导航条目，被 `command-palette`（⌘K）和 `account-rail` 的 hover flyout
 * 同时复用。撤掉旧 sidebar 之后，发现性主要靠这两个入口。
 */
import type React from "react";
import {
  MessageCircle, Zap, BookOpen, ScanFace, Package, Settings,
  MailWarning, Puzzle, Brain, BrainCircuit,
  Shield, ShieldCheck, BarChart3, Globe, Blocks, HardDriveDownload,
  Terminal, Cpu, LayoutDashboard, Wrench, SmilePlus, HeartPulse,
  Lightbulb, Share2, Bot,
  FolderGit2, Boxes, Users, Workflow,
} from "lucide-react";

export type NavGroup = "概览" | "工作" | "智能" | "系统" | "扩展";

export interface NavItem {
  id: string;
  href: string;
  label: string;
  group: NavGroup;
  icon: React.ReactNode;
  keywords?: string;
}

export const NAV_ITEMS: NavItem[] = [
  { id: "nav-dashboard", href: "/dashboard", label: "概览", group: "概览", icon: <LayoutDashboard size={16} />, keywords: "dashboard home 主页 仪表盘 overview" },
  { id: "nav-chat", href: "/chat", label: "对话", group: "概览", icon: <MessageCircle size={16} />, keywords: "chat 聊天 会话" },
  { id: "nav-inbox", href: "/inbox", label: "收件箱", group: "概览", icon: <MailWarning size={16} />, keywords: "inbox 消息 通知" },

  { id: "nav-missions", href: "/missions", label: "任务中心", group: "工作", icon: <Zap size={16} />, keywords: "missions tasks 任务" },
  { id: "nav-task-run", href: "/task-run", label: "执行视图", group: "工作", icon: <Terminal size={16} />, keywords: "task run 执行 运行" },
  { id: "nav-workflows", href: "/workflows", label: "工作流", group: "工作", icon: <Blocks size={16} />, keywords: "workflow 工作流 流程" },
  { id: "nav-workflow-editor", href: "/workflow-editor", label: "工作流编辑器", group: "工作", icon: <Workflow size={16} />, keywords: "workflow editor 编辑器" },
  { id: "nav-workers", href: "/workers", label: "Worker", group: "工作", icon: <Cpu size={16} />, keywords: "workers worker 进程" },
  { id: "nav-projects", href: "/projects", label: "项目", group: "工作", icon: <FolderGit2 size={16} />, keywords: "projects 项目 repo" },
  { id: "nav-skills", href: "/skills", label: "技能", group: "工作", icon: <Package size={16} />, keywords: "skills 技能" },
  { id: "nav-plugins", href: "/plugins", label: "插件", group: "工作", icon: <Puzzle size={16} />, keywords: "plugins 插件" },
  { id: "nav-packs", href: "/packs", label: "增量包", group: "工作", icon: <Boxes size={16} />, keywords: "packs pack runtime 增量包 热插拔 可选能力" },
  { id: "nav-tools", href: "/tools", label: "终端", group: "工作", icon: <Wrench size={16} />, keywords: "tools terminal shell 工具" },

  { id: "nav-knowledge", href: "/knowledge", label: "知识库", group: "智能", icon: <BookOpen size={16} />, keywords: "knowledge 知识 RAG" },
  { id: "nav-memory", href: "/memory", label: "记忆", group: "智能", icon: <Brain size={16} />, keywords: "memory 记忆" },
  { id: "nav-graph", href: "/graph", label: "知识图谱", group: "智能", icon: <Share2 size={16} />, keywords: "graph 图谱 关系 知识" },
  { id: "nav-persona", href: "/persona", label: "角色", group: "智能", icon: <ScanFace size={16} />, keywords: "persona 人设 角色" },
  { id: "nav-emotions", href: "/emotions", label: "情绪", group: "智能", icon: <SmilePlus size={16} />, keywords: "emotion 情感 情绪" },
  { id: "nav-reflect", href: "/reflect", label: "反思", group: "智能", icon: <Lightbulb size={16} />, keywords: "reflect 反思 思考" },
  { id: "nav-reverie", href: "/reverie", label: "思考记录", group: "智能", icon: <BrainCircuit size={16} />, keywords: "reverie 思考 记录" },
  { id: "nav-heartbeat", href: "/heartbeat", label: "心跳", group: "智能", icon: <HeartPulse size={16} />, keywords: "heartbeat 心跳" },

  { id: "nav-models", href: "/models", label: "模型", group: "系统", icon: <Cpu size={16} />, keywords: "models 模型 LLM" },
  { id: "nav-providers", href: "/settings/providers", label: "提供商", group: "系统", icon: <Globe size={16} />, keywords: "providers 提供商 api key" },
  { id: "nav-metrics", href: "/metrics", label: "指标", group: "系统", icon: <BarChart3 size={16} />, keywords: "metrics 统计 指标" },
  { id: "nav-approvals", href: "/approvals", label: "审批", group: "系统", icon: <ShieldCheck size={16} />, keywords: "approvals 审批" },
  { id: "nav-audit", href: "/audit", label: "审计", group: "系统", icon: <Shield size={16} />, keywords: "audit 日志 审计" },
  { id: "nav-trust", href: "/trust", label: "信任", group: "系统", icon: <ShieldCheck size={16} />, keywords: "trust 安全 信任" },
  { id: "nav-tenants", href: "/tenants", label: "租户", group: "系统", icon: <Users size={16} />, keywords: "tenants 租户 团队" },
  { id: "nav-bots", href: "/bots", label: "Bot", group: "系统", icon: <Bot size={16} />, keywords: "bots bot 机器人" },
  { id: "nav-settings", href: "/settings", label: "设置", group: "系统", icon: <Settings size={16} />, keywords: "settings 偏好 设置" },
];

export const NAV_GROUP_ORDER: NavGroup[] = ["概览", "工作", "智能", "系统", "扩展"];

export function groupNavItems(items: NavItem[]): Record<NavGroup, NavItem[]> {
  const out: Record<NavGroup, NavItem[]> = {
    概览: [], 工作: [], 智能: [], 系统: [], 扩展: [],
  };
  for (const it of items) out[it.group].push(it);
  return out;
}
