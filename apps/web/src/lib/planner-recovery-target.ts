import type { PlannerExecutionStateRecoveryTarget } from "@/lib/api-types";
import { connectorFocusHrefFromText } from "@/lib/connector-focus";
import { providerFocusHrefFromText } from "@/lib/provider-focus";
import { recoveryGroupKeyForTarget } from "@/lib/recovery-group-key";

export interface PlannerResolvedRecoveryTarget {
  category?: string;
  label: string;
  href: string;
  action?: string;
  groupKey?: string;
}

type PlannerRecoveryTargetInput = Partial<PlannerExecutionStateRecoveryTarget> | null | undefined;

const FALLBACK_LABELS: Record<string, string> = {
  dependency: "查看依赖关系",
  provider: "检查模型供应商",
  connector: "修复连接器",
  browser: "打开浏览器包",
  skill: "检查技能",
  tool: "检查工具",
  sandbox: "检查桌面沙箱",
  approval: "处理审批",
};

interface PlannerRecoveryFallbackContext {
  planId?: string;
  connectorTextParts?: Array<string | null | undefined>;
  providerTextParts?: Array<string | null | undefined>;
}

function fallbackHref(category: string, context: PlannerRecoveryFallbackContext = {}): string {
  switch (category) {
    case "dependency": {
      const normalizedPlanId = (context.planId || "").trim();
      return normalizedPlanId ? `/planner-checkpoint?plan_id=${encodeURIComponent(normalizedPlanId)}#dependency-view` : "";
    }
    case "provider":
      return providerFocusHrefFromText(context.providerTextParts || []);
    case "connector":
      return connectorFocusHrefFromText(context.connectorTextParts || []);
    case "browser":
      return "/packs/browser";
    case "skill":
      return "/skills";
    case "tool":
      return "/tools";
    case "sandbox":
      return "/packs/computer-use";
    case "approval":
      return "/approvals";
    default:
      return "";
  }
}

function targetLabel(target: PlannerRecoveryTargetInput, category: string): string {
  return target?.label?.trim() || FALLBACK_LABELS[category] || "打开恢复目标";
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value) ? value as Record<string, unknown> : null;
}

function asString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() ? value.trim() : undefined;
}

function asStringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string" && item.trim().length > 0) : [];
}

export function resolvePlannerRecoveryTarget(
  target: PlannerExecutionStateRecoveryTarget | undefined,
  planId?: string,
): PlannerExecutionStateRecoveryTarget | undefined {
  if (!target) return undefined;
  const category = (target.category || "").toLowerCase();
  const href = target.href || fallbackHref(category, {
    planId,
    connectorTextParts: [target.label, target.action, target.category],
    providerTextParts: [target.label, target.action, target.category],
  });
  const label = targetLabel(target, category);
  const groupKey = href ? target.group_key || recoveryGroupKeyForTarget(category, href) : target.group_key;
  return href ? { ...target, group_key: groupKey, label, href } : { ...target, group_key: groupKey, label };
}

export function resolvePlannerRecoveryTargetFromDetail(detail: Record<string, unknown>): PlannerResolvedRecoveryTarget | null {
  const target = asRecord(detail.primary_target) || asRecord(detail.recovery_target);
  if (!target) return null;
  const category = asString(target.category)?.toLowerCase() || "";
  const href = asString(target.href) || fallbackHref(category, {
    planId: asString(detail.plan_id),
    connectorTextParts: [
      asString(target.label),
      asString(target.action),
      asString(target.category),
      asString(detail.failure_pattern),
      asString(detail.recommendation),
      asString(detail.next_step),
      ...asStringArray(detail.failed_tools),
      ...asStringArray(detail.ruled_out),
    ],
    providerTextParts: [
      asString(target.label),
      asString(target.action),
      asString(target.category),
      asString(detail.failure_pattern),
      asString(detail.recommendation),
      asString(detail.next_step),
      ...asStringArray(detail.failed_tools),
      ...asStringArray(detail.ruled_out),
    ],
  });
  if (!href) return null;
  const groupKey = asString(target.group_key) || recoveryGroupKeyForTarget(category, href);
  return {
    category,
    label: asString(target.label) || FALLBACK_LABELS[category] || "打开恢复入口",
    href,
    action: asString(target.action),
    groupKey,
  };
}
