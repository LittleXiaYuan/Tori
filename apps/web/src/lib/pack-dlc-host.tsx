"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { showToast } from "@/components/toast-provider";
import { BASE, getAuthHeaders } from "@/lib/api-core";
import { collectPackTheme, observePackTheme } from "@/lib/pack-theme";
import { resolvePackUIOrigin } from "@/lib/pack-ui-origin";
import { useI18n } from "@/lib/i18n";
import {
  BRIDGE_VERSION,
  createBridgeRateLimiter,
  dispatchBridgeRequest,
  makeBackendCallHandler,
  makeNavHandler,
  makeStorageHandlers,
  PackEventSubscriptions,
  type AllowedRoute,
  type BridgeEnvelope,
  type BridgeMethodHandler,
  type BridgeViolation,
} from "@/lib/pack-bridge";

export interface PackDlcHostProps {
  packId: string;
  /** Entry HTML relative to the bundle root; defaults to index.html. */
  entry?: string;
  title?: string;
  /** The pack's backend.routeSpecs — the backend.call whitelist. */
  allowedRoutes?: AllowedRoute[];
  /** The pack's frontend.routes paths — the nav.push whitelist. */
  allowedNavPaths?: string[];
  /** Host SSE paths the pack may subscribe to (from events:subscribe:* permissions). */
  allowedEventPaths?: string[];
  /** Extra capability handlers merged over the built-ins. */
  extraHandlers?: Record<string, BridgeMethodHandler>;
}

/** Builds the bundle URL served by GET /v1/packs/{id}/ui/* (M1). Root-relative
 *  so it resolves against the gateway origin in both packaged and dev modes. */
export function packBundleUrl(packId: string, entry?: string): string {
  const file = (entry || "index.html").replace(/^\/+/, "");
  return `/v1/packs/${encodeURIComponent(packId)}/ui/${file}`;
}

/** Caps how many bridge violations one mount reports to the audit trail, so a
 *  hostile bundle cannot turn the reporter itself into a write-amplifier. */
const MaxViolationReports = 50;

/** reportBridgeViolation files one refused bridge request into the backend
 *  audit chain (spec §7.3). Fire-and-forget: auditing must never break the UI. */
function reportBridgeViolation(packId: string, v: BridgeViolation): void {
  void fetch(`${BASE}/v1/packs/${encodeURIComponent(packId)}/bridge-violation`, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...getAuthHeaders() },
    body: JSON.stringify({ method: v.method, code: v.code, message: v.message }),
  }).catch(() => {});
}

/**
 * PackDlcHost renders a Pack's iframe-bundle frontend in a sandboxed iframe and
 * bridges it to the host over postMessage. See docs/spec/pack-frontend-dlc.md.
 *
 * Isolation: sandbox="allow-scripts" (no allow-same-origin) gives the frame an
 * opaque origin, so it cannot read the host's localStorage/token. Inbound
 * messages are authenticated by source identity (event.source === the frame's
 * contentWindow), because the opaque origin makes event.origin === "null".
 */
export function PackDlcHost({ packId, entry, title, allowedRoutes, allowedNavPaths, allowedEventPaths, extraHandlers }: PackDlcHostProps) {
  const { locale } = useI18n();
  const router = useRouter();
  const iframeRef = useRef<HTMLIFrameElement | null>(null);
  const [height, setHeight] = useState(640);
  const [ready, setReady] = useState(false);
  // null = origin not resolved yet (don't mount the iframe to avoid a double
  // load); "" = isolation listener disabled, serve same-origin.
  const [uiOrigin, setUiOrigin] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    resolvePackUIOrigin().then((origin) => {
      if (!cancelled) setUiOrigin(origin);
    });
    return () => { cancelled = true; };
  }, []);
  // Keep mutable bits in refs so the message listener (bound once) sees fresh values.
  const langRef = useRef(locale);
  langRef.current = locale;
  const extraRef = useRef(extraHandlers);
  extraRef.current = extraHandlers;
  const routesRef = useRef(allowedRoutes);
  routesRef.current = allowedRoutes;
  const navRef = useRef(allowedNavPaths);
  navRef.current = allowedNavPaths;
  const eventPathsRef = useRef(allowedEventPaths);
  eventPathsRef.current = allowedEventPaths;

  useEffect(() => {
    const post = (msg: BridgeEnvelope) => {
      // Opaque-origin frame: targetOrigin must be "*"; the frame only contains
      // our own pack bundle.
      iframeRef.current?.contentWindow?.postMessage(msg, "*");
    };

    // SSE-over-bridge: the sandboxed bundle cannot open network connections
    // (CSP connect-src 'none'), so the host owns the SSE streams and forwards
    // each event into the iframe as a kind:"event" envelope.
    const eventSubs = new PackEventSubscriptions({
      paths: eventPathsRef.current || [],
      authHeaders: getAuthHeaders,
      baseUrl: BASE,
      emit: (subID, evt) => post({
        v: BRIDGE_VERSION,
        kind: "event",
        method: "events.message",
        payload: { sub_id: subID, event: evt.event, data: evt.data },
      }),
      onClose: (subID, reason) => post({
        v: BRIDGE_VERSION,
        kind: "event",
        method: "events.closed",
        payload: { sub_id: subID, reason },
      }),
    });

    const defaultHandlers: Record<string, BridgeMethodHandler> = {
      "host.handshake": () => {
        setReady(true);
        // Theme + language ride along so the pack UI can match the shell from
        // its very first paint (live changes follow as theme/i18n events).
        return { v: BRIDGE_VERSION, packId, lang: langRef.current, theme: collectPackTheme() };
      },
      "ui.toast": (payload) => {
        const p = (payload || {}) as { message?: string; type?: "success" | "error" | "warning" | "info" };
        if (p.message) showToast(p.message, p.type || "info");
        return { ok: true };
      },
      "ui.resize": (payload) => {
        const p = (payload || {}) as { height?: number };
        if (typeof p.height === "number" && p.height > 0) {
          setHeight(Math.min(Math.max(Math.round(p.height), 160), 4000));
        }
        return { ok: true };
      },
      // backend.call: token injected host-side, path gated to the pack's routeSpecs.
      "backend.call": makeBackendCallHandler({
        routes: routesRef.current || [],
        authHeaders: getAuthHeaders,
        baseUrl: BASE,
      }),
      "nav.push": makeNavHandler(navRef.current || [], (path) => router.push(path)),
      ...makeStorageHandlers(packId, typeof window !== "undefined" ? window.localStorage : undefined),
      ...eventSubs.handlers(),
    };

    // Per-mount inbound limiter + capped violation reporting (spec §7.3).
    const rateLimit = createBridgeRateLimiter();
    let violationReports = 0;
    const onViolation = (v: BridgeViolation) => {
      if (violationReports >= MaxViolationReports) return;
      violationReports++;
      reportBridgeViolation(packId, v);
    };

    const onMessage = (event: MessageEvent) => {
      // Source-identity check (opaque origin → event.origin is "null").
      if (!iframeRef.current || event.source !== iframeRef.current.contentWindow) return;
      void dispatchBridgeRequest(
        { packId, post, handlers: { ...defaultHandlers, ...(extraRef.current || {}) }, rateLimit, onViolation },
        event.data,
      );
    };

    window.addEventListener("message", onMessage);
    // Live theme sync: shell theme mutations are pushed as theme.changed events.
    const stopThemeObserver = observePackTheme((theme) => {
      post({ v: BRIDGE_VERSION, kind: "event", method: "theme.changed", payload: { theme } });
    });
    return () => {
      window.removeEventListener("message", onMessage);
      stopThemeObserver();
      eventSubs.closeAll();
    };
  }, [packId]);

  // Live language sync: locale switches are pushed as i18n.changed events.
  const localePostedRef = useRef(false);
  useEffect(() => {
    if (!localePostedRef.current) {
      // Skip the mount value — handshake already carried it.
      localePostedRef.current = true;
      return;
    }
    iframeRef.current?.contentWindow?.postMessage(
      { v: BRIDGE_VERSION, kind: "event", method: "i18n.changed", payload: { lang: locale } },
      "*",
    );
  }, [locale]);

  // Wait for the isolation-origin probe before mounting, so the iframe loads
  // exactly once from the right origin.
  if (uiOrigin === null) {
    return (
      <div
        style={{
          width: "100%", height, border: "1px solid var(--yunque-border)",
          borderRadius: 12, background: "var(--yunque-surface)", opacity: 0.4,
        }}
      />
    );
  }

  return (
    <iframe
      ref={iframeRef}
      src={`${uiOrigin}${packBundleUrl(packId, entry)}`}
      title={title || packId}
      sandbox="allow-scripts"
      referrerPolicy="no-referrer"
      allow=""
      style={{
        width: "100%",
        height,
        border: "1px solid var(--yunque-border)",
        borderRadius: 12,
        background: "var(--yunque-surface)",
        opacity: ready ? 1 : 0.6,
        transition: "opacity 160ms ease",
      }}
    />
  );
}

export default PackDlcHost;
