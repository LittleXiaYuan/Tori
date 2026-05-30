import type { ReactNode } from "react";
import Link from "next/link";

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

/**
 * SettingsCard is the single building block for the Settings page's guide
 * cards (setup banner, provider redirect, API key, tier selector, error
 * notice). It replaces the per-card inline styles that previously redefined
 * the accent bar, icon, title, and description on every card.
 */
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
  const cls = [
    "settings-card",
    tone !== "default" && `settings-card--${tone}`,
    href && "settings-card--link",
    className,
  ]
    .filter(Boolean)
    .join(" ");

  const inner = (
    <>
      <div className="settings-card__head">
        {icon && <span className="settings-card__icon">{icon}</span>}
        <div className="settings-card__text">
          <div className="settings-card__title">{title}</div>
          {desc && <div className="settings-card__desc">{desc}</div>}
        </div>
        {action && <div className="settings-card__action">{action}</div>}
      </div>
      {children && <div className="settings-card__body">{children}</div>}
    </>
  );

  if (href) {
    return (
      <Link href={href} className={cls}>
        {inner}
      </Link>
    );
  }
  return <div className={cls}>{inner}</div>;
}
