"use client";

import { useState, useCallback } from "react";
import { Card, Button, Spinner, Chip, Tooltip } from "@heroui/react";
import { api, type ApprovalRequest, type ApprovalRule } from "@/lib/api";
import { useApiData } from "@/lib/use-api-data";
import { ShieldAlert, Check, XCircle, RefreshCw, Plus, Trash2, Clock, Shield } from "lucide-react";
import { showToast } from "@/components/toast-provider";
import EmptyState from "@/components/empty-state";

const RISK_COLORS: Record<string, { bg: string; fg: string }> = {
  safe: { bg: "rgba(34,197,94,0.1)", fg: "#22c55e" },
  caution: { bg: "rgba(245,158,11,0.1)", fg: "#f59e0b" },
  danger: { bg: "rgba(239,68,68,0.1)", fg: "#ef4444" },
  critical: { bg: "rgba(168,85,247,0.1)", fg: "#a855f7" },
};

const STATUS_COLORS: Record<string, { bg: string; fg: string }> = {
  pending: { bg: "rgba(245,158,11,0.1)", fg: "#f59e0b" },
  approved: { bg: "rgba(34,197,94,0.1)", fg: "#22c55e" },
  denied: { bg: "rgba(239,68,68,0.1)", fg: "#ef4444" },
};

const STATUS_LABELS: Record<string, string> = {
  pending: "待审批", approved: "已批准", denied: "已拒绝",
};

export default function ApprovalsPage() {
  const [filter, setFilter] = useState<string>("pending");
  const [deciding, setDeciding] = useState<string | null>(null);
  const [showRules, setShowRules] = useState(false);
  const [rulePattern, setRulePattern] = useState("");
  const [ruleAction, setRuleAction] = useState<"allow" | "deny">("allow");
  const [creatingRule, setCreatingRule] = useState(false);

  const { data: approvals, loading, refresh } = useApiData(
    useCallback(() => api.approvalsList(filter).then(r => r.approvals || []), [filter]),
    [] as ApprovalRequest[],
    [filter],
  );

  const { data: rules, loading: rulesLoading, refresh: refreshRules } = useApiData(
    useCallback(() => api.approvalRules().then(r => r.rules || []), []),
    [] as ApprovalRule[],
  );

  const handleDecide = async (id: string, decision: "allow_once" | "allow_always" | "deny_always") => {
    setDeciding(id);
    try {
      await api.approvalDecide(id, decision);
      showToast(decision.includes("allow") ? "已批准" : "已拒绝", "success");
      refresh();
    } catch (e) {
      showToast(e instanceof Error ? e.message : "操作失败", "error");
    }
    setDeciding(null);
  };

  const handleCreateRule = async () => {
    if (!rulePattern.trim()) return;
    setCreatingRule(true);
    try {
      await api.approvalRuleCreate({ pattern: rulePattern, action: ruleAction, scope: "global" });
      showToast("规则已创建", "success");
      setRulePattern("");
      refreshRules();
    } catch (e) {
      showToast(e instanceof Error ? e.message : "创建失败", "error");
    }
    setCreatingRule(false);
  };

  const handleDeleteRule = async (id: string) => {
    try {
      await api.approvalRuleDelete(id);
      showToast("规则已删除", "success");
      refreshRules();
    } catch (e) {
      showToast(e instanceof Error ? e.message : "删除失败", "error");
    }
  };

  const pendingCount = approvals.filter(a => a.status === "pending").length;

  return (
    <div className="page-root space-y-4 animate-fade-in-up">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <ShieldAlert size={20} style={{ color: "var(--yunque-accent)" }} />
          <h1 className="page-title">审批中心</h1>
          {pendingCount > 0 && (
            <Chip size="sm" style={{ background: "rgba(245,158,11,0.1)", color: "#f59e0b", fontSize: "var(--text-2xs)" }}>
              {pendingCount} 待处理
            </Chip>
          )}
        </div>
        <div className="flex items-center gap-2">
          <Button size="sm" variant="ghost" onPress={() => setShowRules(!showRules)}>
            <Shield size={14} /> {showRules ? "审批列表" : "自动规则"}
          </Button>
          <Tooltip delay={0}>
            <Button isIconOnly aria-label="刷新审批与规则" variant="ghost" size="sm" onPress={() => { refresh(); refreshRules(); }}>
              <RefreshCw size={16} />
            </Button>
            <Tooltip.Content>刷新</Tooltip.Content>
          </Tooltip>
        </div>
      </div>

      {!showRules ? (
        <>
          {/* Filter pills */}
          <div className="flex items-center gap-2">
            {["pending", "approved", "denied", ""].map(f => (
              <button key={f || "all"} type="button" onClick={() => setFilter(f)}
                aria-current={filter === f ? "true" : undefined}
                className="filter-pill filter-pill-subtle" data-active={filter === f}>
                {f ? STATUS_LABELS[f] || f : "全部"}
              </button>
            ))}
          </div>

          {/* Approval list */}
          {loading ? (
            <div className="flex justify-center py-16"><Spinner /></div>
          ) : approvals.length === 0 ? (
            <EmptyState icon={<ShieldAlert size={32} />} title="暂无审批请求" description={filter === "pending" ? "所有操作已处理" : "该筛选条件下无记录"} />
          ) : (
            <div className="space-y-3">
              {approvals.map(item => {
                const risk = RISK_COLORS[item.risk_level] || RISK_COLORS.caution;
                const status = STATUS_COLORS[item.status] || STATUS_COLORS.pending;
                return (
                  <Card key={item.id} className="section-card p-5 hover-lift">
                    <div className="flex items-start justify-between gap-4">
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
                            {item.tool_name}
                          </span>
                          <Chip size="sm" style={{ background: risk.bg, color: risk.fg, fontSize: "var(--text-2xs)" }}>
                            {item.risk_level}
                          </Chip>
                          <Chip size="sm" style={{ background: status.bg, color: status.fg, fontSize: "var(--text-2xs)" }}>
                            {STATUS_LABELS[item.status] || item.status}
                          </Chip>
                        </div>
                        <div className="text-xs mt-1 font-mono" style={{ color: "var(--yunque-text-secondary)" }}>
                          {item.action}
                        </div>
                        {item.args && Object.keys(item.args).length > 0 && (
                          <div className="text-xs mt-1.5 px-2 py-1 rounded" style={{ background: "var(--yunque-bg-hover)", color: "var(--yunque-text-muted)" }}>
                            {Object.entries(item.args).slice(0, 3).map(([k, v]) => (
                              <span key={k} className="mr-3">{k}: {typeof v === "string" ? v : JSON.stringify(v)}</span>
                            ))}
                          </div>
                        )}
                        <div className="flex items-center gap-3 mt-2 text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>
                          <span className="flex items-center gap-1"><Clock size={10} />{new Date(item.created_at).toLocaleString()}</span>
                          {item.requester && <span>来源: {item.requester}</span>}
                          {item.reason && <span>原因: {item.reason}</span>}
                        </div>
                      </div>
                      {item.status === "pending" && (
                        <div className="flex flex-col gap-1.5 shrink-0">
                          <Button size="sm" isPending={deciding === item.id}
                            onPress={() => handleDecide(item.id, "allow_once")}
                            style={{ background: "rgba(34,197,94,0.12)", color: "#22c55e" }}>
                            <Check size={14} /> 允许一次
                          </Button>
                          <Button size="sm" isPending={deciding === item.id}
                            onPress={() => handleDecide(item.id, "allow_always")}
                            style={{ background: "var(--yunque-accent-muted)", color: "var(--yunque-accent-strong)" }}>
                            <Check size={14} /> 始终允许
                          </Button>
                          <Button size="sm" isPending={deciding === item.id}
                            onPress={() => handleDecide(item.id, "deny_always")}
                            style={{ background: "rgba(239,68,68,0.12)", color: "#ef4444" }}>
                            <XCircle size={14} /> 拒绝
                          </Button>
                        </div>
                      )}
                    </div>
                  </Card>
                );
              })}
            </div>
          )}
        </>
      ) : (
        <>
          {/* Rules view */}
          <Card className="section-card p-5">
            <div className="text-sm font-medium mb-3" style={{ color: "var(--yunque-text)" }}>创建自动规则</div>
            <div className="flex items-center gap-2">
              <input
                className="flex-1 text-sm px-3 py-1.5 rounded-lg border"
                style={{ background: "var(--yunque-bg-hover)", borderColor: "var(--yunque-border)", color: "var(--yunque-text)" }}
                placeholder="工具模式 (如: fs.read, process.*, *)"
                aria-label="自动规则工具模式"
                value={rulePattern}
                onChange={e => setRulePattern(e.target.value)}
                onKeyDown={e => { if (e.key === "Enter") handleCreateRule(); }}
              />
              <div className="flex gap-1">
                {(["allow", "deny"] as const).map(a => (
                  <button key={a} type="button" onClick={() => setRuleAction(a)}
                    aria-current={ruleAction === a ? "true" : undefined}
                    className="filter-pill filter-pill-subtle" data-active={ruleAction === a}>
                    {a === "allow" ? "允许" : "拒绝"}
                  </button>
                ))}
              </div>
              <Button size="sm" className="btn-accent" onPress={handleCreateRule} isPending={creatingRule}>
                <Plus size={14} /> 添加
              </Button>
            </div>
          </Card>

          {rulesLoading ? (
            <div className="flex justify-center py-8"><Spinner /></div>
          ) : rules.length === 0 ? (
            <EmptyState icon={<Shield size={32} />} title="暂无自动规则" description="添加规则来自动处理常见审批请求" />
          ) : (
            <div className="space-y-2">
              {rules.map(rule => (
                <Card key={rule.id} className="section-card p-4">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <Chip size="sm" style={{
                        background: rule.action === "allow" ? "rgba(34,197,94,0.1)" : "rgba(239,68,68,0.1)",
                        color: rule.action === "allow" ? "#22c55e" : "#ef4444",
                        fontSize: "var(--text-2xs)",
                      }}>
                        {rule.action === "allow" ? "允许" : "拒绝"}
                      </Chip>
                      <span className="text-sm font-mono" style={{ color: "var(--yunque-text)" }}>{rule.pattern}</span>
                      <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{rule.scope}</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>
                        {new Date(rule.created_at).toLocaleDateString()}
                      </span>
                      <Tooltip delay={0}>
                        <Button isIconOnly aria-label={`删除自动规则 ${rule.pattern}`} size="sm" variant="ghost" onPress={() => handleDeleteRule(rule.id)}>
                          <Trash2 size={14} style={{ color: "#ef4444" }} />
                        </Button>
                        <Tooltip.Content>删除</Tooltip.Content>
                      </Tooltip>
                    </div>
                  </div>
                </Card>
              ))}
            </div>
          )}
        </>
      )}
    </div>
  );
}
