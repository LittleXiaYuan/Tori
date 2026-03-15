"use client";

import { useEffect, useRef, useState } from "react";
import { useInView } from "motion/react";
import { cn } from "@/lib/utils";

interface NumberTickerProps {
  value: number;
  className?: string;
  duration?: number;
  format?: (n: number) => string;
}

export function NumberTicker({
  value,
  className,
  duration = 800,
  format,
}: NumberTickerProps) {
  const ref = useRef(null);
  const isInView = useInView(ref, { once: true });
  const [display, setDisplay] = useState(0);

  useEffect(() => {
    if (!isInView) return;
    const start = Date.now();
    const from = 0;
    const to = value;
    const tick = () => {
      const elapsed = Date.now() - start;
      const progress = Math.min(elapsed / duration, 1);
      const eased = 1 - Math.pow(1 - progress, 3);
      setDisplay(Math.round(from + (to - from) * eased));
      if (progress < 1) requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  }, [isInView, value, duration]);

  return (
    <span ref={ref} className={cn("tabular-nums", className)}>
      {format ? format(display) : display.toLocaleString()}
    </span>
  );
}
