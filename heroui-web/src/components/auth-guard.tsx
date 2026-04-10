"use client";

import { useEffect, useState, useRef } from "react";
import { usePathname, useRouter } from "next/navigation";
import { Spinner } from "@heroui/react";
import { useI18n } from "@/lib/i18n";

const PUBLIC_PATHS = ["/login", "/setup"];

export default function AuthGuard({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { t } = useI18n();
  const [checking, setChecking] = useState(true);
  const authedRef = useRef(false);

  useEffect(() => {
    if (PUBLIC_PATHS.some((path) => pathname?.startsWith(path))) {
      setChecking(false);
      return;
    }

    if (authedRef.current) {
      setChecking(false);
      return;
    }

    const token = localStorage.getItem("yunque_token");
    if (!token) {
      router.replace("/login");
      return;
    }

    const controller = new AbortController();
    setChecking(true);

    fetch("/v1/auth/status", {
      headers: { Authorization: `Bearer ${token}` },
      signal: controller.signal,
    })
      .then((res) => res.json())
      .then((data) => {
        if (!data?.authenticated) {
          localStorage.removeItem("yunque_token");
          router.replace("/login");
          return;
        }
        authedRef.current = true;
        setChecking(false);
      })
      .catch((error) => {
        if (error?.name === "AbortError") return;
        authedRef.current = true;
        setChecking(false);
      });

    return () => controller.abort();
  }, [pathname, router]);

  if (PUBLIC_PATHS.some((path) => pathname?.startsWith(path))) {
    return <>{children}</>;
  }

  if (checking) {
    return (
      <div className="fixed inset-0 flex flex-col items-center justify-center gap-3" style={{ background: "var(--yunque-bg)" }}>
        <Spinner size="lg" />
        <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>{t("auth.loading")}</div>
      </div>
    );
  }

  return <>{children}</>;
}
