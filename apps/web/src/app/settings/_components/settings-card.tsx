import type { ReactNode } from "react";
import Link from "next/link";
import { Card } from "@heroui/react";

type Tone = "default" | "accent" | "warning" | "danger";

interface SettingsCardProps {
  icon?: ReactNode;
  title: ReactNode;
  desc?: ReactNode;
  tone?: Tone;
  /** When set, the whole card becomes a navigation link. */
  href?: string;
  /** Right-aligned slot in the header (buttons, chips, a chevron…). */
  action?: ReactNode;
  /** Card body rendered below the header (inputs, button groups…). */
  children?: ReactNode;
  className?: string;
}

export function SettingsCard({
  icon,
  title,
  desc,
  tone = "default",
  href,
  action,
  children,
  className,
}: SettingsCardProps) {
  const isAccent = tone === "accent";
  const isWarning = tone === "warning";
  const isDanger = tone === "danger";

  // Different tones can be rendered using background and border colors in HeroUI.
  let cardStyle = {};
  let iconBg = "rgba(255,255,255,0.05)";
  
  if (isAccent) {
    cardStyle = { border: "1px solid rgba(59,130,246,0.3)", background: "var(--yunque-surface-1)" };
    iconBg = "rgba(59,130,246,0.1)";
  } else if (isWarning) {
    cardStyle = { border: "1px solid rgba(245,158,11,0.3)", background: "var(--yunque-surface-1)" };
    iconBg = "rgba(245,158,11,0.1)";
  } else if (isDanger) {
    cardStyle = { border: "1px solid rgba(239,68,68,0.3)", background: "var(--yunque-surface-1)" };
    iconBg = "rgba(239,68,68,0.1)";
  } else {
    cardStyle = { border: "1px solid var(--yunque-border)", background: "var(--yunque-surface-1)" };
  }

  const inner = (
    <Card 
      className={`hover-lift transition-all duration-300 w-full ${href ? "cursor-pointer" : ""} ${className || ""}`}
      style={cardStyle}
      
    >
      <Card.Header className="flex flex-row gap-4 items-start p-5 pb-3">
        {icon && (
          <div 
            className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl"
            style={{ background: iconBg, border: "1px solid rgba(255,255,255,0.05)" }}
          >
            {icon}
          </div>
        )}
        <div className="flex flex-1 flex-col gap-1.5">
          <Card.Title className="text-base font-semibold tracking-tight" style={{ color: "var(--yunque-text)" }}>
            {title}
          </Card.Title>
          {desc && (
            <Card.Description className="text-sm leading-relaxed" style={{ color: "var(--yunque-text-secondary)" }}>
              {desc}
            </Card.Description>
          )}
        </div>
        {action && <div className="ml-4 shrink-0">{action}</div>}
      </Card.Header>
      
      {children && (
        <Card.Content className="px-5 pb-5 pt-2">
          {children}
        </Card.Content>
      )}
    </Card>
  );

  if (href) {
    return (
      <Link href={href} className="block w-full">
        {inner}
      </Link>
    );
  }
  return <div className="block w-full">{inner}</div>;
}
