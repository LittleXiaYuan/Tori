"use client";

import { Button, Tooltip } from "@heroui/react";
import { RefreshCw } from "lucide-react";

interface PageHeaderProps {
  icon: React.ReactNode;
  iconColor?: string;
  title: string;
  description?: string;
  onRefresh?: () => void;
  actions?: React.ReactNode;
}

export default function PageHeader({ icon, iconColor, title, description, onRefresh, actions }: PageHeaderProps) {
  return (
    <div className="page-header">
      <div>
        <h1 className="page-title" style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <span style={{ color: iconColor || "var(--yunque-accent)", display: "flex" }}>{icon}</span>
          {title}
        </h1>
        {description && <p className="page-subtitle">{description}</p>}
      </div>
      <div style={{ display: "flex", alignItems: "center", gap: "var(--sp-2)" }}>
        {onRefresh && (
          <Tooltip delay={0}>
            <Button isIconOnly variant="ghost" size="sm" onPress={onRefresh} style={{ color: "var(--yunque-text-muted)" }}>
              <RefreshCw size={13} />
            </Button>
            <Tooltip.Content>刷新</Tooltip.Content>
          </Tooltip>
        )}
        {actions}
      </div>
    </div>
  );
}
