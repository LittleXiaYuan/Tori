"use client";

import { useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { Spinner } from "@heroui/react";

const PUBLIC_PATHS = ["/login", "/setup"];

export default function AuthGuard({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    if (PUBLIC_PATHS.some((p) => pathname?.startsWith(p))) {
      setChecking(false);
      return;
    }

    const token = localStorage.getItem("yunque_token");
    if (!token) {
      router.replace("/login");
      return;
    }

    // Validate token with backend
    fetch("/v1/auth/status", {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then((res) => res.json())
      .then((data) => {
        if (!data.authenticated) {
          localStorage.removeItem("yunque_token");
          router.replace("/login");
        } else {
          setChecking(false);
        }
      })
      .catch(() => {
        // If backend is down, allow access with existing token
        setChecking(false);
      });
  }, [pathname, router]);

  if (PUBLIC_PATHS.some((p) => pathname?.startsWith(p))) {
    return <>{children}</>;
  }

  if (checking) {
    return (
      <div className="fixed inset-0 flex items-center justify-center" style={{ background: "var(--yunque-bg)" }}>
        <Spinner size="lg" />
      </div>
    );
  }

  return <>{children}</>;
}
