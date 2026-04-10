import { Button } from "@heroui/react";

interface EmptyStateProps {
  icon: React.ReactNode;
  title: string;
  description?: string;
  actionLabel?: string;
  onAction?: () => void;
}

export default function EmptyState({ icon, title, description, actionLabel, onAction }: EmptyStateProps) {
  return (
    <div className="empty-box animate-fade-in-up">
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
    </div>
  );
}
