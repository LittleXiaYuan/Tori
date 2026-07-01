import type { ConnectorView } from "@/lib/api-types";

export type ConnectorRecoveryKind = "browser" | "auth" | "allowlist" | "rate_limit" | "upstream" | "generic";
export type ConnectorRecoverySeverity = "danger" | "warning";

export interface ConnectorRecoveryHint {
  kind: ConnectorRecoveryKind;
  title: string;
  summary: string;
  detail: string;
  actionLabel: string;
  href: string;
  severity: ConnectorRecoverySeverity;
}

export function connectorEventFailure(event: ConnectorView["last_event"]): boolean {
  if (!event) return false;
  return !["ok", "success", "connected", "disconnected"].includes(String(event.status || "").toLowerCase());
}

export function connectorEventLabel(event: ConnectorView["last_event"]): string {
  if (!event) return "连接器状态异常";
  const action = event.action_id ? ` ${event.action_id}` : "";
  switch (event.kind) {
    case "connect":
    case "oauth2_connect":
      return "连接失败";
    case "refresh":
      return "刷新凭据失败";
    case "execute":
      return `执行${action}失败`;
    default:
      return `${event.kind}${action} 失败`;
  }
}

export function connectorIssueDetail(connector: ConnectorView): string {
  return connector.last_event?.message || connector.error || "最近连接器事件失败，请检查凭据、授权或 allowlist。";
}

export function connectorRecoveryHint(connector: ConnectorView): ConnectorRecoveryHint | null {
  if (connector.status !== "error" && !connector.error && !connectorEventFailure(connector.last_event)) {
    return null;
  }

  const name = connector.name || connector.id;
  const detail = connectorIssueDetail(connector);
  const text = [
    connector.id,
    connector.error,
    connector.last_event?.kind,
    connector.last_event?.action_id,
    connector.last_event?.message,
  ].filter(Boolean).join(" ").toLowerCase();

  if (/browser|extension|pair|paired|not paired|浏览器|扩展|配对/.test(text)) {
    return {
      kind: "browser",
      title: `${name} 未配对`,
      summary: "浏览器通道不在线，网页任务会暂停。",
      detail,
      actionLabel: "打开浏览器包",
      href: "/packs/browser",
      severity: "warning",
    };
  }

  if (/allowlist|allow list|not allowed|unsupported action|action.*(denied|forbidden)|动作.*(未授权|不允许)|权限边界/.test(text)) {
    return {
      kind: "allowlist",
      title: `${name} 动作未在 Allowlist 中`,
      summary: "任务请求超出当前连接器动作边界。",
      detail,
      actionLabel: "检查能力边界",
      href: `/settings/connectors?focus=${encodeURIComponent(connector.id)}`,
      severity: "danger",
    };
  }

  if (/401|403|unauthori[sz]ed|forbidden|invalid|expired|token|credential|oauth|auth|api key|apikey|凭证|令牌|密钥|认证|授权|失效|过期|无效/.test(text)) {
    return {
      kind: "auth",
      title: `${name} 凭据需要重新授权`,
      summary: connector.auth_type === "oauth2" ? "OAuth 或 Token 已失效。" : "API Token 不可用或已过期。",
      detail,
      actionLabel: "重新授权",
      href: `/settings/connectors?focus=${encodeURIComponent(connector.id)}`,
      severity: "danger",
    };
  }

  if (/429|rate limit|too many requests|throttle|限流|频率|请求过多/.test(text)) {
    return {
      kind: "rate_limit",
      title: `${name} 连接器被限流`,
      summary: "上游服务正在限制调用频率。",
      detail,
      actionLabel: "稍后重试",
      href: `/settings/connectors?focus=${encodeURIComponent(connector.id)}`,
      severity: "warning",
    };
  }

  if (/timeout|timed out|network|dns|econn|upstream|5\d\d|unavailable|网络|超时|上游|不可用/.test(text)) {
    return {
      kind: "upstream",
      title: `${name} 上游暂不可用`,
      summary: "连接器或第三方服务暂时不可达。",
      detail,
      actionLabel: "刷新状态",
      href: `/settings/connectors?focus=${encodeURIComponent(connector.id)}`,
      severity: "warning",
    };
  }

  return {
    kind: "generic",
    title: `${name} 连接器需要处理`,
    summary: "连接器最近一次操作失败。",
    detail,
    actionLabel: "查看异常",
    href: `/settings/connectors?focus=${encodeURIComponent(connector.id)}`,
    severity: "warning",
  };
}
