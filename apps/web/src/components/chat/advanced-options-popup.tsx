"use client";

import {
  Button,
  Popover,
  ToggleButton,
  ToggleButtonGroup,
} from "@heroui/react";
import { SlidersHorizontal, Sparkles, Zap, MessageCircle, Cpu, Bird, Terminal } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import type { ChatMode, ThinkingLevel } from "@/components/model-selector-popup";

// 对话高级选项：从模型选择弹窗里移出的「模式 + 思维深度」，
// 收进输入框工具栏一个 ⚙ 小按钮，默认不占地，需要才展开。
const MODE_DEFS: { key: ChatMode; label: string; icon: typeof Sparkles }[] = [
  { key: "agent", label: "Agent", icon: Sparkles },
  { key: "fast", label: "Fast", icon: Zap },
  { key: "chat", label: "Chat", icon: MessageCircle },
];

const EFFORT_KEYS: Record<ThinkingLevel, string> = {
  none: "model.think.none",
  auto: "model.think.auto",
  deep: "model.think.deep",
};

type ExecMode = "xiaoyu" | "api";

const EXEC_MODE_DEFS: { key: ExecMode; label: string; icon: typeof Bird }[] = [
  { key: "xiaoyu", label: "小羽", icon: Bird },
  { key: "api", label: "API", icon: Terminal },
];

interface Props {
  chatMode: ChatMode;
  onModeChange: (mode: ChatMode) => void;
  thinkingLevel: ThinkingLevel;
  onThinkingChange: (level: ThinkingLevel) => void;
  execMode: ExecMode;
  onExecModeChange: (mode: ExecMode) => void;
}

export function AdvancedOptionsPopup({ chatMode, onModeChange, thinkingLevel, onThinkingChange, execMode, onExecModeChange }: Props) {
  const { t } = useI18n();
  return (
    <Popover>
      <Popover.Trigger>
        <Button
          variant="ghost"
          size="sm"
          className="text-xs font-medium px-2 min-w-0"
          aria-label={t("composer.advanced")}
          style={{ color: "var(--yunque-text-secondary)" }}
        >
          <SlidersHorizontal size={14} className="mr-1" />
          {t("composer.advanced")}
        </Button>
      </Popover.Trigger>
      <Popover.Content placement="top start" offset={6} className="advanced-options-popup">
        <Popover.Dialog
          className="flex flex-col gap-3"
          style={{
            padding: 12,
            minWidth: 280,
            borderRadius: 14,
            background: "var(--yunque-elevated, var(--yunque-card))",
            border: "1px solid var(--yunque-border)",
            boxShadow: "0 12px 40px rgba(0,0,0,0.35)",
          }}
        >
          {/* 模式 */}
          <div>
            <div className="mb-1.5 text-[11px] font-medium" style={{ color: "var(--yunque-text-secondary)" }}>
              {t("composer.mode")}
            </div>
            <ToggleButtonGroup
              selectionMode="single"
              disallowEmptySelection
              fullWidth
              size="sm"
              isDetached
              selectedKeys={new Set([chatMode])}
              onSelectionChange={(keys) => {
                const next = Array.from(keys)[0] as ChatMode | undefined;
                if (next) onModeChange(next);
              }}
              aria-label={t("model.modeAria")}
            >
              {MODE_DEFS.map((md) => {
                const Icon = md.icon;
                return (
                  <ToggleButton key={md.key} id={md.key} variant="ghost" className="text-[11px] gap-1.5">
                    <Icon size={12} />
                    {md.label}
                  </ToggleButton>
                );
              })}
            </ToggleButtonGroup>
          </div>

          {/* 执行模式：小羽(自研执行器，参与自蒸馏学习) / API(外部模型直给，
              并发上限更高，不参与自蒸馏) — 与上面的对话模式是独立的一条轴 */}
          <div>
            <div className="mb-1.5 text-[11px] font-medium" style={{ color: "var(--yunque-text-secondary)" }}>
              {t("composer.execMode")}
            </div>
            <ToggleButtonGroup
              selectionMode="single"
              disallowEmptySelection
              fullWidth
              size="sm"
              isDetached
              selectedKeys={new Set([execMode])}
              onSelectionChange={(keys) => {
                const next = Array.from(keys)[0] as ExecMode | undefined;
                if (next) onExecModeChange(next);
              }}
              aria-label={t("composer.execModeAria")}
            >
              {EXEC_MODE_DEFS.map((md) => {
                const Icon = md.icon;
                return (
                  <ToggleButton key={md.key} id={md.key} variant="ghost" className="text-[11px] gap-1.5">
                    <Icon size={12} />
                    {md.label}
                  </ToggleButton>
                );
              })}
            </ToggleButtonGroup>
          </div>

          {/* 思维深度（仅非纯聊天模式可调） */}
          {chatMode !== "chat" && (
            <div>
              <div className="mb-1.5 flex items-center justify-between">
                <span className="text-[11px] font-medium" style={{ color: "var(--yunque-text-secondary)" }}>{t("model.thinkDepth")}</span>
                <span className="text-[10px] tabular-nums" style={{ color: "var(--yunque-text-muted)" }}>
                  {thinkingLevel === "none" ? t("model.cost.low") : thinkingLevel === "deep" ? t("model.cost.high") : t("model.cost.balanced")}
                </span>
              </div>
              <div className="flex items-center gap-1 rounded-[10px] p-1" style={{ background: "var(--yunque-bg-muted)" }}>
                {(["none", "auto", "deep"] as ThinkingLevel[]).map((lvl) => {
                  const active = thinkingLevel === lvl;
                  return (
                    <button
                      type="button"
                      key={lvl}
                      aria-current={active ? "true" : undefined}
                      onClick={() => onThinkingChange(lvl)}
                      className="flex-1 flex items-center justify-center gap-1 rounded-lg py-1.5 text-[11px] font-medium transition-colors duration-150 whitespace-nowrap"
                      style={{
                        background: active ? "var(--yunque-bg-hover)" : "transparent",
                        color: active ? "var(--yunque-text)" : "var(--yunque-text-muted)",
                      }}
                    >
                      {lvl === "none" && <Zap size={11} />}
                      {lvl === "auto" && <Sparkles size={11} />}
                      {lvl === "deep" && <Cpu size={11} />}
                      {t(EFFORT_KEYS[lvl])}
                    </button>
                  );
                })}
              </div>
            </div>
          )}
        </Popover.Dialog>
      </Popover.Content>
    </Popover>
  );
}
