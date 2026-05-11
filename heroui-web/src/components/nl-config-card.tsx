"use client";

import { useState, useCallback } from "react";
import { Button } from "@heroui/react";
import { Settings, ArrowDown, CheckCircle2, AlertCircle, Loader2, X } from "lucide-react";
import { formatErrorMessage } from "@/lib/error-utils";

export interface NLConfigChange {
  category: string;
  field: string;
  fromValue: string;
  toValue: string;
  impact?: string;
  extra?: string;
}

interface NLConfigCardProps {
  changes: NLConfigChange[];
  onApply: () => Promise<void>;
  onCancel: () => void;
  onEdit?: () => void;
}

export function NLConfigCard({ changes, onApply, onCancel, onEdit }: NLConfigCardProps) {
  const [status, setStatus] = useState<"preview" | "applying" | "success" | "error">("preview");
  const [errorMsg, setErrorMsg] = useState("");

  const handleApply = useCallback(async () => {
    setStatus("applying");
    setErrorMsg("");
    try {
      await onApply();
      setStatus("success");
    } catch (e: unknown) {
      setErrorMsg(formatErrorMessage(e, "配置应用失败"));
      setStatus("error");
    }
  }, [onApply]);

  if (status === "success") {
    return (
      <div className="nl-config-result">
        <div className="flex items-center gap-2">
          <CheckCircle2 size={16} style={{ color: "var(--yunque-success)" }} />
          <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>已完成</span>
        </div>
        <div className="mt-1.5 text-xs" style={{ color: "var(--yunque-text-secondary)" }}>
          {changes.map((c) => `${c.field}已切换到 ${c.toValue}`).join("；")}
        </div>
        <div className="mt-2 text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>
          想做其他调整？直接告诉我就行。
        </div>
      </div>
    );
  }

  if (status === "error") {
    return (
      <div className="nl-config-result nl-config-result--error">
        <div className="flex items-center gap-2">
          <AlertCircle size={16} style={{ color: "var(--yunque-danger)" }} />
          <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>配置失败</span>
        </div>
        <div className="mt-1.5 text-xs" style={{ color: "var(--yunque-text-secondary)" }}>{errorMsg}</div>
        <div className="mt-3 flex gap-2 justify-end">
          <Button size="sm" variant="ghost" className="rounded-lg text-xs" onPress={onCancel}>取消</Button>
          <Button size="sm" className="rounded-lg text-xs btn-accent" onPress={handleApply}>重试</Button>
        </div>
      </div>
    );
  }

  return (
    <div className="nl-config-card">
      <div className="flex items-center gap-2 mb-3">
        <Settings size={14} style={{ color: "var(--yunque-accent)" }} />
        <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>配置变更预览</span>
      </div>

      {changes.map((change, i) => (
        <div key={i} className="nl-config-diff">
          <div className="flex-1 space-y-1.5">
            <div className="text-xs font-medium" style={{ color: "var(--yunque-text-muted)" }}>{change.category} · {change.field}</div>
            <div className="flex items-center gap-2">
              <span className="nl-config-diff-from">{change.fromValue}</span>
              <ArrowDown size={12} style={{ color: "var(--yunque-text-disabled)", transform: "rotate(-90deg)" }} />
              <span className="nl-config-diff-to">{change.toValue}</span>
            </div>
            {change.impact && <div className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{change.impact}</div>}
            {change.extra && <div className="text-[10px]" style={{ color: "var(--yunque-text-disabled)" }}>{change.extra}</div>}
          </div>
        </div>
      ))}

      <div className="flex gap-2 justify-end mt-4">
        <Button size="sm" variant="ghost" className="rounded-lg text-xs" onPress={onCancel} aria-label="取消配置">
          <X size={12} className="mr-1" /> 取消
        </Button>
        {onEdit && (
          <Button size="sm" variant="ghost" className="rounded-lg text-xs" style={{ border: "1px solid var(--yunque-border)" }} onPress={onEdit} aria-label="修改配置">修改</Button>
        )}
        <Button
          size="sm"
          className="rounded-lg text-xs btn-accent"
          onPress={handleApply}
          isDisabled={status === "applying"}
          aria-label="应用配置"
        >
          {status === "applying" ? <Loader2 size={12} className="animate-spin mr-1" /> : <CheckCircle2 size={12} className="mr-1" />}
          {status === "applying" ? "应用中…" : "应用"}
        </Button>
      </div>
    </div>
  );
}
