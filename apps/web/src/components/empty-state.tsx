import { Button } from "@heroui/react";
import { MessageCircle } from "lucide-react";

interface EmptyStateProps {
  icon: React.ReactNode;
  title: string;
  description?: string;
  actionLabel?: string;
  onAction?: () => void;
  nlHint?: string;
  onNlHint?: (text: string) => void;
}

export default function EmptyState({ icon, title, description, actionLabel, onAction, nlHint, onNlHint }: EmptyStateProps) {
  return (
    <div className="empty-box animate-fade-in-up" role="status">
      <div className="empty-state-icon">{icon}</div>
      <div className="empty-state-body">
        <div className="empty-state-title">{title}</div>
        {description && <div className="empty-state-desc">{description}</div>}
      </div>
      {actionLabel && onAction && (
        <Button size="sm" className="empty-state-action" onPress={onAction}>
          {actionLabel}
        </Button>
      )}
      {nlHint && (
        <button
          className="flex items-center gap-2 mt-2 px-4 py-2.5 rounded-xl text-xs transition-all"
          style={{
            background: "var(--yunque-accent-soft)",
            border: "1px solid var(--yunque-accent-muted)",
            color: "var(--yunque-text-secondary)",
            maxWidth: 360,
          }}
          onClick={() => onNlHint?.(nlHint)}
        >
          <MessageCircle size={12} style={{ color: "var(--yunque-accent)", flexShrink: 0 }} />
          <span>💡 试试对 Agent 说：<span style={{ color: "var(--yunque-accent)", fontWeight: 500 }}>"{nlHint}"</span></span>
        </button>
      )}
    </div>
  );
}
