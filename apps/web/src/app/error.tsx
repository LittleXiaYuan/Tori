"use client";

import { Button } from "@heroui/react";
import Link from "next/link";
import { AlertTriangle, Home, RefreshCw } from "lucide-react";
import { formatErrorMessage } from "@/lib/error-utils";
import { useI18n } from "@/lib/i18n";

export default function Error({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  const { t } = useI18n();
  return (
    <div className="flex h-[70vh] flex-col items-center justify-center gap-4 px-6 text-center animate-fade-in-up">
      <div className="flex h-16 w-16 items-center justify-center rounded-3xl text-2xl font-black text-white shadow-lg" style={{ background: "linear-gradient(135deg, var(--yunque-accent), #7c3aed)" }}>
        云
      </div>
      <div className="text-center space-y-1.5">
        <div className="inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs" style={{ background: "rgba(239,68,68,0.10)", color: "rgb(220,38,38)" }}>
          <AlertTriangle size={13} /> {t("error.badge")}
        </div>
        <h2 className="text-xl font-black" style={{ color: "var(--yunque-text)" }}>{t("error.title")}</h2>
        <p className="text-sm max-w-md" style={{ color: "var(--yunque-text-muted)" }}>
          {formatErrorMessage(error, t("error.fallback"))}
        </p>
      </div>
      <div className="flex gap-2">
        <Button size="sm" className="gap-1.5 rounded-lg btn-accent" onPress={reset}>
          <RefreshCw size={14} /> {t("error.retry")}
        </Button>
        <Link
          href="/"
          className="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium"
          style={{ color: "var(--yunque-text-muted)", border: "1px solid var(--yunque-border)" }}
        >
          <Home size={14} /> {t("error.home")}
        </Link>
      </div>
    </div>
  );
}
