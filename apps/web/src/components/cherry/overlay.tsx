"use client";

/**
 * Cherry-style overlay primitives.
 *
 * These are deliberately framework-light — a thin shell around a portal plus
 * ESC / backdrop-click semantics — so Cherry mode can open Settings, Assistant
 * manager, Knowledge, Tools, etc. *on top of* the chat canvas instead of
 * navigating away to the classic HeroUI dashboard routes.
 *
 * Shared conventions:
 *   • No route changes. The chat column stays mounted under the overlay.
 *   • ESC and backdrop click dismiss, but we give consumers an `onClose`
 *     callback if they need to intercept (e.g. "Discard unsaved changes?").
 *   • Mount is deferred one tick so the enter animation runs every time.
 */

import { useEffect, useRef, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";

interface OverlayShellProps {
  open: boolean;
  onClose: () => void;
  children: ReactNode;
  closeOnBackdrop?: boolean;
  closeOnEsc?: boolean;
  ariaLabel?: string;
}

function useEscClose(active: boolean, onClose: () => void, enabled: boolean) {
  useEffect(() => {
    if (!active || !enabled) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.stopPropagation();
        onClose();
      }
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [active, enabled, onClose]);
}

// Tiny portal helper. We mount into document.body so backdrop covers everything
// even if a parent has `overflow: hidden` or `transform` set.
function OverlayPortal({ children }: { children: ReactNode }) {
  if (typeof document === "undefined") return null;
  return createPortal(children, document.body);
}

/* ══════════════════════════════════════════════════════════════
   CherryModal — full-screen centered modal.
   Matches Cherry Studio's settings panel: large rounded rectangle
   with a soft backdrop blur.
   ══════════════════════════════════════════════════════════════ */

export interface CherryModalProps extends OverlayShellProps {
  /** Optional header above the body (title + optional actions). */
  header?: ReactNode;
  /** If true, the modal has no internal padding; consumer owns the layout. */
  bodyFlush?: boolean;
  /** Width token. "lg" matches Cherry settings. */
  size?: "sm" | "md" | "lg" | "xl";
}

export function CherryModal({
  open,
  onClose,
  header,
  children,
  closeOnBackdrop = true,
  closeOnEsc = true,
  bodyFlush = false,
  size = "lg",
  ariaLabel,
}: CherryModalProps) {
  useEscClose(open, onClose, closeOnEsc);
  const cardRef = useRef<HTMLDivElement>(null);

  if (!open) return null;
  return (
    <OverlayPortal>
      <div
        className="cherry-overlay-backdrop"
        role="presentation"
        onMouseDown={(e) => {
          // Only close on clicks that started on the backdrop itself — avoids
          // surprise-closing while dragging a text selection out of the card.
          if (closeOnBackdrop && e.target === e.currentTarget) onClose();
        }}
      >
        <div
          ref={cardRef}
          className={`cherry-overlay-card cherry-overlay-card-${size}`}
          role="dialog"
          aria-modal="true"
          aria-label={ariaLabel}
        >
          {header !== undefined && (
            <div className="cherry-overlay-header">
              <div className="cherry-overlay-header-content">{header}</div>
              <button
                type="button"
                className="cherry-overlay-close"
                onClick={onClose}
                aria-label="Close"
              >
                <X size={16} />
              </button>
            </div>
          )}
          <div className={`cherry-overlay-body ${bodyFlush ? "flush" : ""}`}>{children}</div>
        </div>
      </div>
    </OverlayPortal>
  );
}

/* ══════════════════════════════════════════════════════════════
   CherryDrawer — side-slide panel.
   Right-aligned by default, used for ephemeral side content like
   file uploads, web search history, memory preview.
   ══════════════════════════════════════════════════════════════ */

export interface CherryDrawerProps extends OverlayShellProps {
  side?: "right" | "left";
  width?: number;
  title?: ReactNode;
  description?: ReactNode;
}

export function CherryDrawer({
  open,
  onClose,
  side = "right",
  width = 420,
  title,
  description,
  children,
  closeOnBackdrop = true,
  closeOnEsc = true,
  ariaLabel,
}: CherryDrawerProps) {
  useEscClose(open, onClose, closeOnEsc);

  if (!open) return null;
  return (
    <OverlayPortal>
      <div
        className="cherry-overlay-backdrop cherry-overlay-backdrop-drawer"
        role="presentation"
        onMouseDown={(e) => {
          if (closeOnBackdrop && e.target === e.currentTarget) onClose();
        }}
      >
        <aside
          className={`cherry-drawer cherry-drawer-${side}`}
          role="dialog"
          aria-modal="true"
          aria-label={ariaLabel}
          style={{ width }}
        >
          {(title || description) && (
            <header className="cherry-drawer-header">
              <div>
                {title && <h3 className="cherry-drawer-title">{title}</h3>}
                {description && <p className="cherry-drawer-desc">{description}</p>}
              </div>
              <button
                type="button"
                className="cherry-overlay-close"
                onClick={onClose}
                aria-label="Close"
              >
                <X size={16} />
              </button>
            </header>
          )}
          <div className="cherry-drawer-body">{children}</div>
        </aside>
      </div>
    </OverlayPortal>
  );
}
