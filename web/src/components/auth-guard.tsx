"use client";

import { useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const [ready, setReady] = useState(false);
  const isLoginPage = pathname === "/login";

  useEffect(() => {
    if (isLoginPage) {
      setReady(true);
      return;
    }

    async function check() {
      try {
        const token = localStorage.getItem("yunque_token");
        const res = await fetch("/v1/auth/status", {
          headers: token ? { Authorization: `Bearer ${token}` } : {},
        });
        const data = await res.json();

        if (!data.password_set || !data.authenticated) {
          router.replace("/login");
          return;
        }

        setReady(true);
      } catch {
        setReady(true);
      }
    }

    check();
  }, [pathname, router, isLoginPage]);

  if (!ready && !isLoginPage) return null;

  // Login page renders WITHOUT the main layout (no navbar, no padding)
  if (isLoginPage) return <>{children}</>;

  return <>{children}</>;
}
