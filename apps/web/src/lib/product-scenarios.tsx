import type { ReactNode } from "react";
import { BookOpen, ClipboardList, Cpu, FileText, Search } from "lucide-react";

export interface ProductScenario {
  id: string;
  label: string;
  description: string;
  prompt: string;
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
];

export const ONBOARDING_SCENARIOS = PRODUCT_SCENARIOS.slice(0, 3);
export const CHAT_EMPTY_SCENARIOS = PRODUCT_SCENARIOS.slice(0, 4);
export const DASHBOARD_SCENARIOS = PRODUCT_SCENARIOS.slice(0, 3);

export function scenarioChatHref(prompt: string): string {
  return `/chat?q=${encodeURIComponent(prompt)}`;
}
