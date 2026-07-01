import type { TaskInfo } from "@/lib/api-types";
import { connectorFocusHrefFromText } from "@/lib/connector-focus";
import { providerFocusHrefFromText } from "@/lib/provider-focus";

export interface TaskRecoveryTarget {
  href: string;
  label: string;
}

function connectorFocusHref(hint: NonNullable<TaskInfo["recovery_hint"]>): string {
  const action = hint.primary_action;
  return connectorFocusHrefFromText([
    hint.summary,
    hint.detail,
    action?.id,
    action?.label,
    ...(hint.secondary_actions || []).flatMap((item) => [item.id, item.label]),
  ]);
}

function providerFocusHref(hint: NonNullable<TaskInfo["recovery_hint"]>): string {
  const action = hint.primary_action;
  return providerFocusHrefFromText([
    hint.summary,
    hint.detail,
    action?.id,
    action?.label,
    ...(hint.secondary_actions || []).flatMap((item) => [item.id, item.label]),
  ]);
}

export function taskRecoveryTarget(task: Pick<TaskInfo, "id" | "recovery_hint">): TaskRecoveryTarget | null {
  const hint = task.recovery_hint;
  if (!hint) return null;
  const action = hint.primary_action;
  if (action?.href) return { href: action.href, label: action.label || "恢复入口" };

  switch ((hint.category || "").toLowerCase()) {
  case "provider":
  case "model":
    return { href: providerFocusHref(hint), label: action?.label || "检查模型供应商" };
  case "connector":
    return { href: connectorFocusHref(hint), label: action?.label || "修复连接器" };
  case "browser":
    return { href: "/packs/browser", label: action?.label || "打开浏览器包" };
  case "skill":
    return { href: "/skills", label: action?.label || "检查技能" };
  case "tool":
    return { href: "/tools", label: action?.label || "检查工具" };
  case "sandbox":
    return { href: "/packs/computer-use", label: action?.label || "检查桌面沙箱" };
  case "approval":
    return { href: "/approvals", label: action?.label || "处理审批" };
  case "dependency":
    return {
      href: `/task-detail?id=${encodeURIComponent(task.id)}&tab=execution`,
      label: action?.label || "查看执行链",
    };
  default:
    return null;
  }
}
