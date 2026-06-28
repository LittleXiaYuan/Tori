"use client";

import { useState, useMemo } from "react";
import {
  Button,
  Chip,
  Label,
  ListBox,
  Popover,
  SearchField,
} from "@heroui/react";
import { Cpu, ChevronDown, Check } from "lucide-react";
import { useI18n } from "@/lib/i18n";

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
}

// 纯模型列表弹窗（HeroUI v3 Popover + ListBox）。模式(Agent/Fast/Chat)与
// 思维深度已移到输入框的「高级」按钮，这里只负责选模型，保持单一职责。
// Popover 自动 portal 到 body，不受 header 毛玻璃 stacking context 影响。

export function ModelSelectorPopup({
  models, currentModelId, currentModelLabel, onSelect,
}: Props) {
  const { t } = useI18n();
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
        {currentModelLabel || t("model.unconfigured")}
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
          aria-label={t("model.switch")}
          className="model-selector-trigger gap-1.5 rounded-full px-2.5 h-8 text-[11px] font-mono"
        >
          <Cpu size={12} />
          <span className="max-w-[180px] truncate">{currentModelLabel}</span>
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
                aria-label={t("model.searchAria")}
              >
                <SearchField.Group>
                  <SearchField.SearchIcon />
                  <SearchField.Input placeholder={t("model.searchPlaceholder")} autoFocus />
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
                {t("model.noMatch")}
              </div>
            ) : (
              <ListBox
                aria-label={t("model.listAria")}
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
                      className="text-[12px] rounded-lg flex items-center gap-2"
                      style={{
                        padding: "7px 10px",
                        minHeight: 0,
                        background: isActive ? "var(--yunque-accent-soft)" : "transparent",
                        color: isActive ? "var(--yunque-text)" : "var(--yunque-text-secondary)",
                      }}
                    >
                      <Label className="truncate flex-1">{label}</Label>
                      {isActive && <Check size={13} style={{ color: "var(--yunque-accent)" }} />}
                    </ListBox.Item>
                  );
                })}
              </ListBox>
            )}
          </div>

        </Popover.Dialog>
      </Popover.Content>
    </Popover>
  );
}
