"use client";

import { useState, useMemo } from "react";
import {
  Button,
  Chip,
  Label,
  ListBox,
  Link,
  Popover,
  SearchField,
  ToggleButton,
  ToggleButtonGroup,
} from "@heroui/react";
import { Cpu, ChevronDown, Check, Zap, Sparkles, Heart, MessageCircle, Settings } from "lucide-react";

export interface ModelOption {
  id: string;
  model: string;
  display_name?: string;
  enabled: boolean;
  type?: string;
  tier?: string;
  capabilities?: string[];
}

export type ChatMode = "agent" | "fast" | "chat";
export type ThinkingLevel = "none" | "auto" | "deep";

interface Props {
  models: ModelOption[];
  currentModelId: string;
  currentModelLabel: string;
  onSelect: (model: ModelOption) => void;
  chatMode?: ChatMode;
  onModeChange?: (mode: ChatMode) => void;
  airiAvailable?: boolean;
  thinkingLevel?: ThinkingLevel;
  onThinkingChange?: (level: ThinkingLevel) => void;
}

const MODE_DEFS: { key: ChatMode; label: string; icon: typeof Sparkles }[] = [
  { key: "agent", label: "Agent", icon: Sparkles },
  { key: "fast", label: "Fast", icon: Zap },
  { key: "chat", label: "Chat", icon: MessageCircle },
];

// 全 HeroUI v3 实现：
// - Popover 自动 portal 到 body，不会被 header 毛玻璃创建的 stacking context 困住，
//   也不会被 ChatEmptyState 的"先完成模型配置"卡片遮挡。
// - 触发按钮 / Mode Switcher / 搜索框 / 列表 / Footer 链接全部使用 HeroUI 原生组件。
// - ListBox 内置键盘导航 + ARIA + 受控选择，根除手写 click outside 监听
//   与 React 合成事件竞态导致"点击没反应"的问题。
const EFFORT_LABELS: Record<ThinkingLevel, string> = {
  none: "快速",
  auto: "自动",
  deep: "深度思考",
};

export function ModelSelectorPopup({
  models, currentModelId, currentModelLabel, onSelect,
  chatMode, onModeChange, airiAvailable,
  thinkingLevel = "auto", onThinkingChange,
}: Props) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");

  const enabledModels = useMemo(() => models.filter((m) => m.enabled), [models]);
  const showSearch = enabledModels.length > 8;
  const filtered = useMemo(() => {
    const q = search.toLowerCase().trim();
    if (!q) return enabledModels;
    return enabledModels.filter((m) =>
      (m.model || "").toLowerCase().includes(q) ||
      (m.display_name || "").toLowerCase().includes(q) ||
      (m.type || "").toLowerCase().includes(q),
    );
  }, [enabledModels, search]);

  if (enabledModels.length === 0) {
    return (
      <Chip size="sm" variant="soft" className="text-xs font-mono">
        {currentModelLabel || "未配置模型"}
      </Chip>
    );
  }

  return (
    <Popover
      isOpen={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) setSearch("");
      }}
    >
      <Popover.Trigger>
        <Button
          variant="ghost"
          size="sm"
          aria-label="切换模型"
          className="model-selector-trigger gap-1.5 rounded-full px-2.5 h-8 text-[11px] font-mono"
        >
          <Cpu size={12} />
          <span className="max-w-[180px] truncate">{currentModelLabel} · {EFFORT_LABELS[thinkingLevel]}</span>
          <ChevronDown
            size={10}
            style={{
              transform: open ? "rotate(180deg)" : "none",
              transition: "transform 0.2s",
            }}
          />
        </Button>
      </Popover.Trigger>

      <Popover.Content
        placement="bottom start"
        offset={6}
        className="model-selector-popup"
      >
        <Popover.Dialog
          className="flex flex-col"
          style={{
            padding: 0,
            minWidth: 260,
            maxWidth: 320,
            maxHeight: 380,
            borderRadius: 14,
            background: "var(--yunque-elevated, var(--yunque-card))",
            border: "1px solid var(--yunque-border)",
            boxShadow: "0 12px 40px rgba(0,0,0,0.35)",
          }}
        >
          {chatMode && onModeChange && (
            <div
              className="model-selector-modes"
              style={{
                padding: "8px 8px 4px",
                borderBottom: "1px solid var(--yunque-border)",
                flexShrink: 0,
              }}
            >
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
                aria-label="对话模式"
              >
                {MODE_DEFS.map((md) => {
                  const isAiri = md.key === "chat" && airiAvailable;
                  const Icon = isAiri ? Heart : md.icon;
                  const label = isAiri ? "Airi" : md.label;
                  return (
                    <ToggleButton
                      key={md.key}
                      id={md.key}
                      variant="ghost"
                      className="text-[11px] gap-1.5"
                    >
                      <Icon
                        size={12}
                        fill={isAiri && chatMode === md.key ? "currentColor" : "none"}
                      />
                      {label}
                    </ToggleButton>
                  );
                })}
              </ToggleButtonGroup>
            </div>
          )}

          {showSearch && (
            <div
              className="model-selector-search"
              style={{
                padding: "8px 10px 6px",
                borderBottom: "1px solid var(--yunque-border)",
                flexShrink: 0,
              }}
            >
              <SearchField
                variant="secondary"
                value={search}
                onChange={setSearch}
                fullWidth
                aria-label="搜索模型"
              >
                <SearchField.Group>
                  <SearchField.SearchIcon />
                  <SearchField.Input placeholder="搜索模型…" autoFocus />
                  <SearchField.ClearButton />
                </SearchField.Group>
              </SearchField>
            </div>
          )}

          <div
            style={{ overflowY: "auto", flex: 1, minHeight: 0 }}
            className="chat-scroll-area"
          >
            {filtered.length === 0 ? (
              <div className="text-center py-6 text-xs text-muted">
                没有找到匹配的模型
              </div>
            ) : (
              <ListBox
                aria-label="模型列表"
                selectionMode="single"
                selectedKeys={new Set([currentModelId])}
                onSelectionChange={(keys) => {
                  const key = Array.from(keys)[0];
                  if (!key) return;
                  const m = enabledModels.find((x) => x.id === String(key));
                  if (m) {
                    onSelect(m);
                    setOpen(false);
                  }
                }}
                className="model-selector-listbox p-1 border-0 shadow-none bg-transparent gap-0.5"
                style={{ gap: 2 }}
              >
                {filtered.map((m) => {
                  const label = m.display_name || m.model || m.id;
                  const isActive = m.id === currentModelId;
                  return (
                    <ListBox.Item
                      key={m.id}
                      id={m.id}
                      textValue={label}
                      className="text-[12px] rounded-lg"
                      style={{ padding: "6px 10px", minHeight: 0 }}
                    >
                      <Label className="truncate flex-1">{label}</Label>
                      {isActive && <Check size={13} />}
                    </ListBox.Item>
                  );
                })}
              </ListBox>
            )}
          </div>

          {onThinkingChange && chatMode !== "chat" && (
            <div
              style={{
                padding: "10px 12px",
                borderTop: "1px solid var(--yunque-border)",
                flexShrink: 0,
              }}
            >
              <div className="flex items-center justify-between mb-2">
                <span className="text-[10px] font-medium uppercase tracking-wider" style={{ color: "var(--yunque-text-muted)" }}>思维深度</span>
                <span className="text-[9px] tabular-nums" style={{ color: "var(--yunque-text-muted)", opacity: 0.6 }}>
                  {thinkingLevel === "none" ? "低成本" : thinkingLevel === "deep" ? "高成本" : "平衡"}
                </span>
              </div>
              <div className="flex items-center gap-1 rounded-[10px] p-1" style={{ background: "rgba(255,255,255,0.04)", border: "1px solid rgba(255,255,255,0.06)" }}>
                {(["none", "auto", "deep"] as ThinkingLevel[]).map((lvl) => {
                  const active = thinkingLevel === lvl;
                  return (
                    <button
                      key={lvl}
                      onClick={() => onThinkingChange(lvl)}
                      className="flex-1 flex items-center justify-center gap-1 rounded-lg py-1.5 text-[11px] font-medium transition-all duration-200 whitespace-nowrap"
                      style={{
                        background: active ? "var(--yunque-accent)" : "transparent",
                        color: active ? "#fff" : "var(--yunque-text-muted)",
                        boxShadow: active ? "0 2px 8px rgba(59,130,246,0.25)" : "none",
                      }}
                    >
                      {lvl === "none" && <Zap size={11} />}
                      {lvl === "auto" && <Sparkles size={11} />}
                      {lvl === "deep" && <Cpu size={11} />}
                      {EFFORT_LABELS[lvl]}
                    </button>
                  );
                })}
              </div>
            </div>
          )}

          <Link
            href="/settings/providers"
            className="flex items-center justify-center gap-1 py-1.5 text-[10px] transition-colors"
            style={{
              borderTop: "1px solid var(--yunque-border)",
              flexShrink: 0,
              color: "var(--yunque-text-muted)",
              opacity: 0.6,
            }}
          >
            <Settings size={10} />
            设置
          </Link>
        </Popover.Dialog>
      </Popover.Content>
    </Popover>
  );
}
