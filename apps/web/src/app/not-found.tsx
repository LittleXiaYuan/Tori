"use client";

import { Button } from "@heroui/react";
import Link from "next/link";

export default function NotFound() {
  return (
    <div className="flex flex-col items-center justify-center h-[60vh] gap-4">
      <div className="text-6xl font-bold" style={{ color: "var(--yunque-text-muted)" }}>404</div>
      <p className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>页面不存在</p>
      <Link href="/">
        <Button size="sm" className="btn-accent">返回首页</Button>
      </Link>
    </div>
  );
}
