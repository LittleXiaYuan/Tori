"use client";

interface CircleProgressProps {
  /** 0-100 percentage */
  value: number;
  /** Diameter in px */
  size?: number;
  /** Stroke width in px */
  strokeWidth?: number;
  /** Track color */
  trackColor?: string;
  /** Progress color — can be a CSS color or gradient ID */
  color?: string;
  /** Label inside the circle */
  label?: string;
  /** Sub label below the value */
  subLabel?: string;
}

/**
 * CircleProgress — SVG ring progress indicator.
 * Inspired by NapCat's CPU/memory circular gauges.
 */
export function CircleProgress({
  value,
  size = 120,
  strokeWidth = 8,
  trackColor = "rgba(255,255,255,0.06)",
  color = "var(--accent)",
  label,
  subLabel,
}: CircleProgressProps) {
  const radius = (size - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;
  const offset = circumference - (Math.min(value, 100) / 100) * circumference;

  // Determine text color based on value
  const valueColor =
    value >= 90 ? "var(--success)" :
    value >= 60 ? "var(--accent)" :
    value >= 30 ? "var(--warning)" :
    "var(--danger)";

  return (
    <div className="relative inline-flex items-center justify-center" style={{ width: size, height: size }}>
      <svg width={size} height={size} className="transform -rotate-90">
        {/* Track */}
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke={trackColor}
          strokeWidth={strokeWidth}
        />
        {/* Progress */}
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke={color}
          strokeWidth={strokeWidth}
          strokeDasharray={circumference}
          strokeDashoffset={offset}
          strokeLinecap="round"
          style={{
            transition: "stroke-dashoffset 0.8s cubic-bezier(0.4, 0, 0.2, 1)",
          }}
        />
      </svg>
      {/* Center text */}
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className="font-bold" style={{ fontSize: size * 0.22, color: valueColor }}>
          {Math.round(value)}%
        </span>
        {label && (
          <span className="text-[10px] mt-0.5" style={{ color: "var(--text-muted)" }}>
            {label}
          </span>
        )}
      </div>
      {/* Sub label below */}
      {subLabel && (
        <span
          className="absolute text-[10px] font-medium"
          style={{ bottom: -20, color: "var(--text-secondary)" }}
        >
          {subLabel}
        </span>
      )}
    </div>
  );
}
