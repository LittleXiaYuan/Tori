"use client";

import { useEffect, useRef, useCallback } from "react";
import { X } from "lucide-react";

interface ModalProps {
  open: boolean;
  onClose: () => void;
  title?: string;
  children: React.ReactNode;
  width?: string; // e.g. "480px", "600px"
}

/**
 * Modal — glassmorphism dialog with backdrop blur and slide-in animation.
 * Closes on Escape key and backdrop click.
 */
export function Modal({ open, onClose, title, children, width = "480px" }: ModalProps) {
  const overlayRef = useRef<HTMLDivElement>(null);

  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    if (e.key === "Escape") onClose();
  }, [onClose]);

  useEffect(() => {
    if (open) {
      document.addEventListener("keydown", handleKeyDown);
      document.body.style.overflow = "hidden";
    }
    return () => {
      document.removeEventListener("keydown", handleKeyDown);
      document.body.style.overflow = "";
    };
  }, [open, handleKeyDown]);

  if (!open) return null;

  return (
    <div
      ref={overlayRef}
      className="fixed inset-0 z-50 flex items-center justify-center"
      onClick={(e) => { if (e.target === overlayRef.current) onClose(); }}
    >
      {/* Backdrop */}
      <div
        className="absolute inset-0"
        style={{
          background: "rgba(0, 0, 0, 0.6)",
          backdropFilter: "blur(8px)",
          WebkitBackdropFilter: "blur(8px)",
          animation: "modal-backdrop-in 0.2s ease forwards",
        }}
      />

      {/* Dialog */}
      <div
        className="relative rounded-2xl border overflow-hidden"
        style={{
          width,
          maxWidth: "calc(100vw - 48px)",
          maxHeight: "calc(100vh - 96px)",
          background: "var(--bg-elevated)",
          borderColor: "var(--border)",
          boxShadow: "var(--shadow-lg), 0 0 0 1px rgba(255,255,255,0.05)",
          animation: "modal-slide-in 0.25s cubic-bezier(0.16, 1, 0.3, 1) forwards",
        }}
      >
        {/* Header */}
        {title && (
          <div
            className="flex items-center justify-between px-6 py-4 border-b"
            style={{ borderColor: "var(--border)" }}
          >
            <h2 className="text-base font-semibold">{title}</h2>
            <button
              onClick={onClose}
              className="w-8 h-8 rounded-lg flex items-center justify-center transition-colors"
              style={{ color: "var(--text-muted)" }}
              onMouseEnter={(e) => (e.currentTarget.style.background = "var(--bg-hover)")}
              onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
            >
              <X size={16} />
            </button>
          </div>
        )}

        {/* Content */}
        <div className="px-6 py-5 overflow-y-auto" style={{ maxHeight: "calc(100vh - 200px)" }}>
          {children}
        </div>
      </div>
    </div>
  );
}
