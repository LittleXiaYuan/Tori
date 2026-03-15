"use client";

import { AnimatePresence, motion } from "motion/react";
import { cn } from "@/lib/utils";

interface AnimatedListProps {
  children: React.ReactNode[];
  className?: string;
  delay?: number;
}

export function AnimatedList({ children, className, delay = 0.05 }: AnimatedListProps) {
  return (
    <div className={cn("flex flex-col gap-2", className)}>
      <AnimatePresence initial={true}>
        {children.map((child, i) => (
          <motion.div
            key={i}
            initial={{ opacity: 0, y: 10, filter: "blur(4px)" }}
            animate={{ opacity: 1, y: 0, filter: "blur(0px)" }}
            exit={{ opacity: 0, y: -10, filter: "blur(4px)" }}
            transition={{ delay: i * delay, duration: 0.3, ease: "easeOut" }}
          >
            {child}
          </motion.div>
        ))}
      </AnimatePresence>
    </div>
  );
}
