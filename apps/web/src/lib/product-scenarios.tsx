import type { ReactNode } from "react";
import { BookOpen, Brain, ClipboardList, Cpu, FileText, MessageCircle, Monitor, Package, Search } from "lucide-react";

export interface ProductScenario {
  id: string;
  label: string;
  description: string;
  prompt: string;
  icon: ReactNode;
}

export interface ChatAgentScene {
  id: string;
  promptIds: string[];
  icon: ReactNode;
}

export const PRODUCT_SCENARIOS: ProductScenario[] = [
  {
    id: "weekly-report",
    label: "写周报",
    description: "把一周工作整理成可验收成果。",
    prompt: "帮我把最近一周的工作整理成周报，输出本周成果、下周计划、风险提醒，并标出值得沉淀到记忆的偏好或长期事实。",
    icon: <ClipboardList size={14} />,
  },
  {
    id: "meeting-notes",
    label: "会议纪要",
    description: "提炼结论、待办、负责人和记忆。",
    prompt: "把这次会议内容整理成纪要，提炼结论、待办、负责人和风险，并列出后续可回写到记忆的长期事实。",
    icon: <FileText size={14} />,
  },
  {
    id: "web-research",
    label: "网页调研",
    description: "搜索资料，输出对比表、建议和可复用记忆。",
    prompt: "帮我调研近期 AI 工作流产品的差异，输出对比表、建议、来源摘要，并标出下次可复用的记忆。",
    icon: <Search size={14} />,
  },
  {
    id: "code-task",
    label: "修代码",
    description: "派给 AI IDE 执行，保留确认点。",
    prompt: "请把这个需求派给 AI IDE 执行，过程中需要我确认时再回来问我。",
    icon: <Cpu size={14} />,
  },
  {
    id: "knowledge-brief",
    label: "整理知识",
    description: "把资料沉淀成可复用知识条目。",
    prompt: "帮我把下面这段资料整理成知识库条目，并给出摘要和标签。",
    icon: <BookOpen size={14} />,
  },
  {
    id: "ask-explain",
    label: "解释概念",
    description: "用通俗的话解释一个概念并举例。",
    prompt: "用通俗的话解释「向量数据库」是什么，并举一个生活中的例子。",
    icon: <MessageCircle size={14} />,
  },
  {
    id: "remember-pref",
    label: "记住偏好",
    description: "把你的习惯写进长期记忆。",
    prompt: "记住我的工作偏好：回复先给结论、尽量简洁。以后对话都按这个来。",
    icon: <Brain size={14} />,
  },
  {
    id: "pack-improve",
    label: "补强能力",
    description: "说明现有能力哪里不够，让云雀给出可执行改进方案。",
    prompt: "我觉得一个现有能力还不够好。请先帮我梳理：它应该解决什么问题、现在缺什么、下一步可以怎么补强，并给出可验收的改进清单。",
    icon: <Package size={14} />,
  },
  {
    id: "computer-use-plan",
    label: "电脑使用计划",
    description: "先规划浏览器或桌面动作，不直接控制本机。",
    prompt: "请先为这个电脑使用任务生成安全执行计划：目标、需要打开的页面或应用、每一步要做什么、哪些步骤需要我确认。暂时不要执行本机控制。",
    icon: <Monitor size={14} />,
  },
];

export const ONBOARDING_SCENARIOS = PRODUCT_SCENARIOS.slice(0, 3);
// Chat 空态起手势：固定覆盖三条主路径——问答/创作、加知识或记忆、建任务/行动。
export const CHAT_EMPTY_SCENARIOS = ["ask-explain", "knowledge-brief", "remember-pref", "weekly-report"]
  .map((id) => PRODUCT_SCENARIOS.find((s) => s.id === id))
  .filter((s): s is ProductScenario => Boolean(s));
export const DASHBOARD_SCENARIOS = PRODUCT_SCENARIOS.slice(0, 3);

export const CHAT_AGENT_SCENES: ChatAgentScene[] = [
  {
    id: "general",
    promptIds: ["ask-explain", "weekly-report", "knowledge-brief"],
    icon: <MessageCircle size={14} />,
  },
  {
    id: "task",
    promptIds: ["weekly-report", "meeting-notes", "code-task"],
    icon: <ClipboardList size={14} />,
  },
  {
    id: "knowledge",
    promptIds: ["knowledge-brief", "web-research", "meeting-notes"],
    icon: <BookOpen size={14} />,
  },
  {
    id: "memory",
    promptIds: ["remember-pref", "meeting-notes", "weekly-report"],
    icon: <Brain size={14} />,
  },
  {
    id: "capability",
    promptIds: ["pack-improve", "knowledge-brief", "code-task"],
    icon: <Package size={14} />,
  },
  {
    id: "computer-plan",
    promptIds: ["computer-use-plan", "web-research", "code-task"],
    icon: <Monitor size={14} />,
  },
];

export function scenarioChatHref(prompt: string): string {
  return `/chat?q=${encodeURIComponent(prompt)}`;
}
