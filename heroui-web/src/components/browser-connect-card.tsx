"use client";

import { Button, Chip } from "@heroui/react";
import { Download, ExternalLink, RefreshCw } from "lucide-react";

export interface BrowserRequirement {
  required: boolean;
  reason: string;
  message: string;
  install_path?: string;
  settings_path?: string;
}

interface BrowserConnectCardProps {
  requirement: BrowserRequirement;
  connected?: boolean;
  onOpenSetup?: () => void;
  onRefresh?: () => void;
}

export function BrowserConnectCard({ requirement, connected, onOpenSetup, onRefresh }: BrowserConnectCardProps) {
  if (!requirement?.required) return null;

  return (
    <div
      className="mt-3 rounded-[18px] border p-3"
      style={{
        background: "linear-gradient(180deg, rgba(59,130,246,0.09), rgba(59,130,246,0.03))",
        borderColor: "rgba(59,130,246,0.18)",
      }}
    >
      <div className="flex items-start gap-3">
        <div
          className="flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl"
          style={{ background: "rgba(59,130,246,0.14)", color: "#60a5fa" }}
        >
          <Download size={18} />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
              Connect browser runtime
            </div>
            <Chip
              size="sm"
              style={{
                background: connected ? "rgba(34,197,94,0.12)" : "rgba(245,158,11,0.12)",
                color: connected ? "#86efac" : "#fbbf24",
              }}
            >
              {connected ? "Connected" : "Required"}
            </Chip>
          </div>

          <div className="mt-2 text-xs leading-6" style={{ color: "var(--yunque-text-secondary)" }}>
            {requirement.message}
          </div>

          <div className="mt-3 space-y-1.5 text-[12px]" style={{ color: "var(--yunque-text-muted)" }}>
            <div>1. Open the Browser workspace.</div>
            <div>2. Install or load the Yunque Browser Connector extension.</div>
            <div>3. Connect it, then return here and continue the task.</div>
          </div>

          <div className="mt-3 flex flex-wrap items-center gap-2">
            <Button size="sm" className="rounded-full px-3" onPress={onOpenSetup}>
              <ExternalLink size={14} />
              Open browser setup
            </Button>
            <Button size="sm" variant="ghost" className="rounded-full px-3" onPress={onRefresh}>
              <RefreshCw size={14} />
              Refresh status
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

export default BrowserConnectCard;
