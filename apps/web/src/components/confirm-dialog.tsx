"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { KeyboardEvent } from "react";

export interface ConfirmOptions {
  title: string;
  body: string;
  confirmLabel?: string;
  cancelLabel?: string;
  tone?: "danger" | "warning" | "default";
}

type ConfirmRequest = Required<ConfirmOptions> & {
  id: string;
  resolve: (confirmed: boolean) => void;
};

const DEFAULTS: Pick<Required<ConfirmOptions>, "confirmLabel" | "cancelLabel" | "tone"> = {
  confirmLabel: "确认",
  cancelLabel: "取消",
  tone: "default",
};

let globalConfirm: (options: ConfirmOptions) => Promise<boolean> = async () => false;

export function confirmAction(options: ConfirmOptions): Promise<boolean> {
  return globalConfirm(options);
}

function getFocusable(container: HTMLElement): HTMLElement[] {
  return Array.from(
    container.querySelectorAll<HTMLElement>(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
    ),
  ).filter((el) => !el.hasAttribute("disabled") && !el.getAttribute("aria-hidden"));
}

export function ConfirmDialogProvider() {
  const [request, setRequest] = useState<ConfirmRequest | null>(null);
  const dialogRef = useRef<HTMLDivElement | null>(null);
  const headingRef = useRef<HTMLHeadingElement | null>(null);
  const previousFocusRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    globalConfirm = (options) =>
      new Promise<boolean>((resolve) => {
        setRequest({
          ...DEFAULTS,
          ...options,
          id: `confirm-${Date.now()}-${Math.random().toString(36).slice(2)}`,
          resolve,
        });
      });
    return () => {
      globalConfirm = async () => false;
    };
  }, []);

  const close = useCallback(
    (confirmed: boolean) => {
      setRequest((current) => {
        current?.resolve(confirmed);
        return null;
      });
    },
    [],
  );

  useEffect(() => {
    if (!request) return;
    previousFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const inertTargets = Array.from(document.querySelectorAll<HTMLElement>("#main-content, [data-sidebar]"));
    inertTargets.forEach((el) => el.setAttribute("inert", ""));
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    window.setTimeout(() => headingRef.current?.focus(), 0);

    return () => {
      inertTargets.forEach((el) => el.removeAttribute("inert"));
      document.body.style.overflow = previousOverflow;
      previousFocusRef.current?.focus();
    };
  }, [request]);

  const onKeyDown = useCallback(
    (event: KeyboardEvent<HTMLDivElement>) => {
      if (!request || !dialogRef.current) return;
      if (event.key === "Escape") {
        event.preventDefault();
        close(false);
        return;
      }
      if (event.key !== "Tab") return;
      const focusable = getFocusable(dialogRef.current);
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    },
    [close, request],
  );

  if (!request) return null;

  const titleId = `${request.id}-title`;
  const bodyId = `${request.id}-body`;

  return (
    <div className="confirm-dialog-backdrop" onMouseDown={() => close(false)}>
      <div
        ref={dialogRef}
        className="confirm-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={bodyId}
        data-tone={request.tone}
        onKeyDown={onKeyDown}
        onMouseDown={(event) => event.stopPropagation()}
      >
        <h2 id={titleId} ref={headingRef} tabIndex={-1} className="confirm-dialog__title">
          {request.title}
        </h2>
        <p id={bodyId} className="confirm-dialog__body">
          {request.body}
        </p>
        <div className="confirm-dialog__actions">
          <button type="button" className="confirm-dialog__button" onClick={() => close(false)}>
            {request.cancelLabel}
          </button>
          <button
            type="button"
            className="confirm-dialog__button confirm-dialog__button--primary"
            onClick={() => close(true)}
          >
            {request.confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
