"use client";

import { useEffect } from "react";

/**
 * CursorGlow tracks mouse position and sets CSS custom properties
 * on .card-hover elements to create a cursor-following light effect.
 * This component renders nothing — it just adds a global mousemove listener.
 */
export function CursorGlow() {
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      const cards = document.querySelectorAll<HTMLElement>(".card-hover");
      cards.forEach((card) => {
        const rect = card.getBoundingClientRect();
        const x = e.clientX - rect.left;
        const y = e.clientY - rect.top;
        card.style.setProperty("--mouse-x", `${x}px`);
        card.style.setProperty("--mouse-y", `${y}px`);
      });
    };
    document.addEventListener("mousemove", handler, { passive: true });
    return () => document.removeEventListener("mousemove", handler);
  }, []);

  return null;
}
