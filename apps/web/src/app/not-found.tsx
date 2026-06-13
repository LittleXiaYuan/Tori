"use client";

import { Button } from "@heroui/react";
import Link from "next/link";
import { useI18n } from "@/lib/i18n";

export default function NotFound() {
  const { t } = useI18n();
  return (
    <div className="flex flex-col items-center justify-center h-[60vh] gap-4">
      <div className="text-6xl font-bold" style={{ color: "var(--yunque-text-muted)" }}>404</div>
      <p className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>{t("notfound.desc")}</p>
      <Link href="/">
        <Button size="sm" className="btn-accent">{t("notfound.home")}</Button>
      </Link>
    </div>
  );
}
