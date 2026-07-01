import { CheckCircle2, Circle } from "lucide-react";

export interface ReadinessFlag {
  /** Human-readable Chinese label, e.g. "执行 Gate". */
  label: string;
  /** Whether this gate / plan / capability is ready. */
  ready: boolean | undefined;
  /** Optional tooltip explaining the flag. */
  hint?: string;
}

interface ReadinessBadgesProps {
  flags: ReadinessFlag[];
  className?: string;
}

// ReadinessBadges renders a wrapped strip of small pills, one per readiness
// flag. It replaces the recurring anti-pattern of stuffing technical status
// tokens ("plan", "pending", "sha256", "dry-run") into KPI-size font, which
// overflowed and truncated. Green pill = ready, muted pill = pending.
export default function ReadinessBadges({ flags, className }: ReadinessBadgesProps) {
  return (
    <div className={`flex flex-wrap gap-2${className ? ` ${className}` : ""}`}>
      {flags.map((flag) => {
        const ready = Boolean(flag.ready);
        return (
          <span
            key={flag.label}
            className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium"
            style={{
              background: ready ? "var(--yunque-success-muted)" : "var(--yunque-bg-hover)",
              color: ready ? "var(--yunque-success)" : "var(--yunque-text-muted)",
              border: `1px solid ${ready ? "transparent" : "var(--yunque-border)"}`,
            }}
            title={flag.hint}
          >
            {ready ? (
              <CheckCircle2 size={13} aria-hidden="true" />
            ) : (
              <Circle size={13} aria-hidden="true" />
            )}
            {flag.label}
          </span>
        );
      })}
    </div>
  );
}
