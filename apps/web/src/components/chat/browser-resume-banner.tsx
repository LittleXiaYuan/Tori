import { Button } from "@heroui/react";
import { Plug } from "lucide-react";
import { useI18n } from "@/lib/i18n";

export interface BrowserResumeBannerProps {
  /** The last user prompt that blocked on the browser connector. */
  prompt: string;
  /** True when the browser runtime extension is currently connected. */
  bridgeConnected: boolean;
  /** True while the "Continue task" action is in-flight. */
  resumePending: boolean;
  /** True while the chat is otherwise processing a request. */
  chatLoading: boolean;
  /** Called when the user clicks "Continue task". */
  onResume: () => void;
  /**
   * Called when the user clicks "Refresh status". Parent wires this to a
   * compound action (sync bridge state + probe /api/browser/ext + publish
   * a toast-style notice) so the component stays free of API dependencies.
   */
  onRefresh: () => void;
}

/**
 * BrowserResumeBanner renders the amber / green banner that appears above
 * the chat scroll when a prior message failed because the browser
 * connector was missing. Two states:
 *
 *   - amber (bridgeConnected=false) — tells the user the connector must
 *     come online before the blocked turn can replay.
 *   - green (bridgeConnected=true)  — the connector is back; one click
 *     resumes the turn.
 *
 * Parent owns:
 *   * deciding when to render (null-check of resumePromptForBrowser)
 *   * the continueBlockedBrowserTask / syncBridgeState side effects
 *   * the bridge-notice toast plumbing
 *
 * Extracted as part of the PR4a step of TECH-DEBT-2026-04-18.md §7.
 * Previously lived inline in chat/page.tsx at lines ~1083–1150.
 */
export function BrowserResumeBanner({
  prompt,
  bridgeConnected,
  resumePending,
  chatLoading,
  onResume,
  onRefresh,
}: BrowserResumeBannerProps) {
  const { t } = useI18n();
  return (
    <div className="px-4 pt-3 xl:px-5 shrink-0">
      <div
        className="rounded-[18px] border px-4 py-3"
        style={{
          background: bridgeConnected
            ? "linear-gradient(180deg, rgba(34,197,94,0.1), rgba(34,197,94,0.03))"
            : "linear-gradient(180deg, rgba(245,158,11,0.12), rgba(245,158,11,0.03))",
          borderColor: bridgeConnected ? "rgba(34,197,94,0.18)" : "rgba(245,158,11,0.18)",
        }}
      >
        <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div className="min-w-0">
            <div
              className="flex items-center gap-2 text-sm font-semibold"
              style={{ color: "var(--yunque-text)" }}
            >
              <Plug size={15} style={{ color: bridgeConnected ? "#86efac" : "#fbbf24" }} />
              {bridgeConnected
                ? t("browser.resume.titleReady")
                : t("browser.resume.titleBlocked")}
            </div>
            <div
              className="mt-1 text-xs leading-6"
              style={{ color: "var(--yunque-text-muted)" }}
            >
              {bridgeConnected
                ? t("browser.resume.descReady")
                : t("browser.resume.descBlocked")}
            </div>
            <div
              className="mt-2 truncate rounded-xl px-2.5 py-2 text-[11px]"
              style={{ background: "rgba(15,23,42,0.3)", color: "var(--yunque-text-secondary)" }}
            >
              {prompt}
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button
              size="sm"
              className="rounded-full px-3"
              variant={bridgeConnected ? "primary" : "ghost"}
              isDisabled={!bridgeConnected || resumePending || chatLoading}
              isPending={resumePending}
              onPress={onResume}
            >
              {t("browser.resume.continue")}
            </Button>
            <Button
              size="sm"
              variant="ghost"
              className="rounded-full px-3"
              onPress={() => window.open("/packs/browser", "_blank", "noopener,noreferrer")}
            >
              {t("browser.resume.setup")}
            </Button>
            <Button
              size="sm"
              variant="ghost"
              className="rounded-full px-3"
              onPress={onRefresh}
            >
              {t("browser.resume.refresh")}
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
