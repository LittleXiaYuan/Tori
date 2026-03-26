"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { useI18n } from "@/lib/i18n";
import { BlurFade } from "@/components/ui/blur-fade";
import {
  Globe,
  Monitor,
  MonitorOff,
  Eye,
  ScanText,
  UserCheck,
  RefreshCw,
  Power,
  PowerOff,
  AlertTriangle,
  Check,
  Camera,
  MessageSquare,
  Send,
  X,
} from "lucide-react";

interface BrowserStatus {
  enabled: boolean;
  headless: boolean;
  state: string;
  data_dir: string;
  ocr_capabilities: string[];
  worker_state?: string;
}

interface ProblemData {
  id: string;
  description: string;
  screenshot?: string;
  url?: string;
  options?: { key: string; label: string }[];
}

interface ActionEvent {
  action: string;
  detail: string;
  time: string;
}

const ocrLabels: Record<string, { zh: string; en: string; icon: React.ElementType }> = {
  dom:    { zh: "DOM 直读",      en: "DOM Text",     icon: ScanText },
  ocr:    { zh: "Tesseract OCR", en: "Tesseract OCR", icon: ScanText },
  vision: { zh: "视觉模型",      en: "Vision LLM",   icon: Eye },
  human:  { zh: "人工兜底",      en: "Human Fallback", icon: UserCheck },
};

function apiHeaders() {
  return {
    "Content-Type": "application/json",
    "X-API-Key": localStorage.getItem("yunque_api_key") || "",
  };
}

export default function BrowserPage() {
  const { locale } = useI18n();
  const zh = locale === "zh";

  const [status, setStatus] = useState<BrowserStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [toggling, setToggling] = useState(false);
  const [toast, setToast] = useState<{ msg: string; ok: boolean } | null>(null);

  // Screenshot stream
  const [screenshot, setScreenshot] = useState<string>("");
  const [screenshotUrl, setScreenshotUrl] = useState<string>("");

  // OPP Problems
  const [problems, setProblems] = useState<ProblemData[]>([]);
  const [decisionText, setDecisionText] = useState<Record<string, string>>({});
  const [deciding, setDeciding] = useState<string>("");

  // Action log
  const [actions, setActions] = useState<ActionEvent[]>([]);
  const logRef = useRef<HTMLDivElement>(null);

  const showToast = (msg: string, ok = true) => {
    setToast({ msg, ok });
    setTimeout(() => setToast(null), 2500);
  };

  // Load status
  const loadStatus = useCallback(async () => {
    try {
      const res = await fetch("/v1/browser/status", { headers: apiHeaders() });
      const data = await res.json();
      setStatus(data);
    } catch {
      setStatus(null);
    } finally {
      setLoading(false);
    }
  }, []);

  // Load pending problems
  const loadProblems = useCallback(async () => {
    try {
      const res = await fetch("/v1/browser/opp/pending", { headers: apiHeaders() });
      const data = await res.json();
      setProblems(data.problems || []);
    } catch {}
  }, []);

  // SSE event stream
  useEffect(() => {
    const key = localStorage.getItem("yunque_api_key") || "";
    const es = new EventSource(`/v1/events/stream?key=${key}`);

    es.addEventListener("browser.screenshot", (e) => {
      try {
        const data = JSON.parse(e.data);
        if (data.data?.image) {
          setScreenshot(data.data.image);
        }
        if (data.data?.url) {
          setScreenshotUrl(data.data.url);
        }
      } catch {}
    });

    es.addEventListener("browser.action", (e) => {
      try {
        const data = JSON.parse(e.data);
        setActions((prev) => [
          ...prev.slice(-49),
          {
            action: data.data?.action || "",
            detail: data.data?.detail || "",
            time: new Date().toLocaleTimeString(),
          },
        ]);
      } catch {}
    });

    es.addEventListener("browser.problem", () => {
      loadProblems();
    });

    es.addEventListener("browser.decided", () => {
      loadProblems();
    });

    return () => es.close();
  }, [loadProblems]);

  // Auto-scroll action log
  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [actions]);

  useEffect(() => {
    loadStatus();
    loadProblems();
    const iv = setInterval(loadStatus, 5000);
    const iv2 = setInterval(loadProblems, 3000);
    return () => { clearInterval(iv); clearInterval(iv2); };
  }, [loadStatus, loadProblems]);

  // Take manual screenshot
  const takeScreenshot = async () => {
    try {
      const res = await fetch("/v1/browser/screenshot/latest", { headers: apiHeaders() });
      const data = await res.json();
      if (data.image) {
        setScreenshot(data.image);
        setScreenshotUrl(data.url || "");
      }
    } catch {
      showToast(zh ? "截图失败" : "Screenshot failed", false);
    }
  };

  // Submit decision
  const submitDecision = async (problemId: string, decision: string) => {
    setDeciding(problemId);
    try {
      await fetch("/v1/browser/opp/decide", {
        method: "POST",
        headers: apiHeaders(),
        body: JSON.stringify({ problem_id: problemId, decision }),
      });
      showToast(zh ? "✅ 决策已提交" : "✅ Decision submitted");
      loadProblems();
    } catch {
      showToast(zh ? "提交失败" : "Submit failed", false);
    } finally {
      setDeciding("");
    }
  };

  // Toggle handlers
  const toggleEnabled = async () => {
    if (!status) return;
    setToggling(true);
    try {
      const newEnabled = !status.enabled;
      await fetch("/v1/browser/config", {
        method: "POST",
        headers: apiHeaders(),
        body: JSON.stringify({ enabled: newEnabled, headless: status.headless }),
      });
      showToast(newEnabled
        ? (zh ? "🌐 浏览器已启动" : "🌐 Browser started")
        : (zh ? "浏览器已关闭" : "Browser stopped"));
      await loadStatus();
    } catch {
      showToast(zh ? "操作失败" : "Failed", false);
    } finally {
      setToggling(false);
    }
  };

  const toggleHeadless = async () => {
    if (!status || !status.enabled) return;
    setToggling(true);
    try {
      await fetch("/v1/browser/config", {
        method: "POST",
        headers: apiHeaders(),
        body: JSON.stringify({ enabled: true, headless: !status.headless }),
      });
      showToast(!status.headless
        ? (zh ? "🔒 无头模式" : "🔒 Headless")
        : (zh ? "🖥️ 可视化模式" : "🖥️ Headed"));
      await loadStatus();
    } catch {
      showToast(zh ? "切换失败" : "Switch failed", false);
    } finally {
      setToggling(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <RefreshCw size={20} className="animate-spin" style={{ color: "var(--text-muted)" }} />
      </div>
    );
  }

  const enabled = status?.enabled ?? false;
  const headless = status?.headless ?? true;
  const caps = status?.ocr_capabilities ?? ["dom", "human"];

  return (
    <div className="max-w-4xl">
      <BlurFade delay={0}>
        <h1 className="text-xl font-semibold mb-6">
          {zh ? "🌐 浏览器引擎" : "🌐 Browser Engine"}
        </h1>
      </BlurFade>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Left: Controls */}
        <div className="space-y-4">
          {/* Engine Control */}
          <BlurFade delay={0.05}>
            <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
              {/* Enable toggle */}
              <div className="flex items-center justify-between mb-3">
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded-lg flex items-center justify-center"
                    style={{ background: enabled ? "rgba(34,197,94,0.15)" : "rgba(239,68,68,0.1)" }}>
                    {enabled ? <Power size={20} style={{ color: "#22c55e" }} /> : <PowerOff size={20} style={{ color: "#ef4444" }} />}
                  </div>
                  <div>
                    <div className="text-sm font-medium">{zh ? "浏览器引擎" : "Browser Engine"}</div>
                    <div className="text-xs" style={{ color: "var(--text-muted)" }}>
                      {enabled ? (zh ? "运行中" : "Running") : (zh ? "未启动" : "Stopped")}
                    </div>
                  </div>
                </div>
                <button onClick={toggleEnabled} disabled={toggling}
                  className="relative w-12 h-7 rounded-full cursor-pointer transition-colors"
                  style={{ background: enabled ? "var(--accent, #3b82f6)" : "#333", opacity: toggling ? 0.5 : 1 }}>
                  <div className="absolute w-5 h-5 rounded-full bg-white top-1 transition-transform"
                    style={{ left: enabled ? "calc(100% - 24px)" : "4px" }} />
                </button>
              </div>
              {/* Headless toggle */}
              <div className="flex items-center justify-between py-3 border-t" style={{ borderColor: "var(--border)" }}>
                <div className="flex items-center gap-2">
                  {headless ? <MonitorOff size={16} style={{ color: "var(--text-muted)" }} /> : <Monitor size={16} style={{ color: "var(--accent)" }} />}
                  <span className="text-sm">{zh ? "可视化模式" : "Headed"}</span>
                </div>
                <button onClick={toggleHeadless} disabled={toggling || !enabled}
                  className="relative w-12 h-7 rounded-full cursor-pointer transition-colors"
                  style={{ background: !headless ? "var(--accent)" : "#333", opacity: (toggling || !enabled) ? 0.3 : 1 }}>
                  <div className="absolute w-5 h-5 rounded-full bg-white top-1 transition-transform"
                    style={{ left: !headless ? "calc(100% - 24px)" : "4px" }} />
                </button>
              </div>
              {/* OCR tags */}
              <div className="pt-3 border-t" style={{ borderColor: "var(--border)" }}>
                <div className="text-xs mb-2" style={{ color: "var(--text-muted)" }}>OCR</div>
                <div className="flex flex-wrap gap-1.5">
                  {["dom", "ocr", "vision", "human"].map((tier) => {
                    const active = caps.includes(tier);
                    const meta = ocrLabels[tier];
                    return (
                      <span key={tier} className="text-[11px] px-2 py-0.5 rounded-full"
                        style={{
                          background: active ? "rgba(34,197,94,0.1)" : "var(--bg-hover)",
                          color: active ? "#22c55e" : "var(--text-muted)",
                          border: `1px solid ${active ? "rgba(34,197,94,0.3)" : "var(--border)"}`,
                        }}>
                        {zh ? meta?.zh : meta?.en}
                      </span>
                    );
                  })}
                </div>
              </div>
            </div>
          </BlurFade>

          {/* OPP Problem Cards */}
          {problems.length > 0 && (
            <BlurFade delay={0.1}>
              <div className="space-y-3">
                <div className="flex items-center gap-2 text-sm font-medium">
                  <AlertTriangle size={16} style={{ color: "#eab308" }} />
                  {zh ? "需要你的帮助" : "Needs Your Help"}
                  <span className="text-xs px-1.5 py-0.5 rounded-full" style={{ background: "#eab30820", color: "#eab308" }}>
                    {problems.length}
                  </span>
                </div>
                {problems.map((p) => (
                  <div key={p.id} className="rounded-xl border p-4"
                    style={{ background: "var(--bg-card)", borderColor: "#eab30840" }}>
                    <div className="text-sm mb-2">{p.description}</div>
                    {p.url && <div className="text-xs mb-2 truncate" style={{ color: "var(--text-muted)" }}>{p.url}</div>}
                    {p.screenshot && (
                      <img src={`data:image/png;base64,${p.screenshot}`} alt="problem" className="rounded-lg mb-3 w-full" />
                    )}
                    {/* Option buttons */}
                    {p.options && p.options.length > 0 && (
                      <div className="flex flex-wrap gap-2 mb-2">
                        {p.options.map((opt) => (
                          <button key={opt.key} onClick={() => submitDecision(p.id, opt.key)}
                            disabled={deciding === p.id}
                            className="px-3 py-1.5 rounded-lg text-xs font-medium border cursor-pointer transition-colors"
                            style={{ borderColor: "var(--accent)", color: "var(--accent)" }}>
                            {opt.label}
                          </button>
                        ))}
                      </div>
                    )}
                    {/* Free text input */}
                    <div className="flex gap-2">
                      <input type="text" placeholder={zh ? "输入决策..." : "Type decision..."}
                        value={decisionText[p.id] || ""}
                        onChange={(e) => setDecisionText((prev) => ({ ...prev, [p.id]: e.target.value }))}
                        className="flex-1 rounded-lg px-3 py-1.5 text-xs outline-none border"
                        style={{ background: "var(--bg-hover)", borderColor: "var(--border)", color: "var(--text)" }} />
                      <button onClick={() => submitDecision(p.id, decisionText[p.id] || "continue")}
                        disabled={deciding === p.id}
                        className="rounded-lg px-3 py-1.5 text-xs cursor-pointer flex items-center gap-1"
                        style={{ background: "var(--accent)", color: "white" }}>
                        <Send size={12} />
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </BlurFade>
          )}

          {/* Action Log */}
          {actions.length > 0 && (
            <BlurFade delay={0.15}>
              <div className="rounded-xl border p-4" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center gap-2 text-sm font-medium">
                    <MessageSquare size={14} style={{ color: "var(--accent)" }} />
                    {zh ? "操作日志" : "Action Log"}
                  </div>
                  <button onClick={() => setActions([])} className="cursor-pointer" style={{ color: "var(--text-muted)" }}>
                    <X size={14} />
                  </button>
                </div>
                <div ref={logRef} className="max-h-40 overflow-y-auto space-y-1 text-xs font-mono"
                  style={{ color: "var(--text-muted)" }}>
                  {actions.map((a, i) => (
                    <div key={i}>
                      <span style={{ color: "var(--text-muted)" }}>{a.time}</span>{" "}
                      <span style={{ color: "var(--accent)" }}>{a.action}</span>{" "}
                      {a.detail}
                    </div>
                  ))}
                </div>
              </div>
            </BlurFade>
          )}
        </div>

        {/* Right: Screenshot Canvas */}
        <BlurFade delay={0.1}>
          <div className="rounded-xl border overflow-hidden" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="flex items-center justify-between px-4 py-3 border-b" style={{ borderColor: "var(--border)" }}>
              <div className="flex items-center gap-2 text-sm font-medium">
                <Camera size={14} style={{ color: "var(--accent)" }} />
                {zh ? "实时画面" : "Live View"}
              </div>
              <button onClick={takeScreenshot} disabled={!enabled}
                className="text-xs px-2.5 py-1 rounded-lg border cursor-pointer transition-colors flex items-center gap-1"
                style={{ borderColor: "var(--border)", color: "var(--text-muted)", opacity: enabled ? 1 : 0.3 }}>
                <RefreshCw size={11} />
                {zh ? "刷新" : "Refresh"}
              </button>
            </div>
            <div className="relative" style={{ minHeight: 300 }}>
              {screenshot ? (
                <img src={`data:image/png;base64,${screenshot}`} alt="browser view"
                  className="w-full h-auto" style={{ imageRendering: "auto" }} />
              ) : (
                <div className="flex flex-col items-center justify-center py-20" style={{ color: "var(--text-muted)" }}>
                  <Globe size={40} className="mb-3 opacity-20" />
                  <div className="text-sm">
                    {enabled
                      ? (zh ? "等待浏览器操作..." : "Waiting for browser action...")
                      : (zh ? "浏览器未启动" : "Browser not running")}
                  </div>
                </div>
              )}
            </div>
            {screenshotUrl && (
              <div className="px-4 py-2 border-t text-xs truncate" style={{ borderColor: "var(--border)", color: "var(--text-muted)" }}>
                {screenshotUrl}
              </div>
            )}
          </div>
        </BlurFade>
      </div>

      {/* Toast */}
      {toast && (
        <div className="fixed bottom-6 right-6 px-4 py-3 rounded-xl border text-sm z-50 animate-in slide-in-from-bottom-4"
          style={{ background: "var(--bg-card)", borderColor: toast.ok ? "#22c55e40" : "#ef444440" }}>
          {toast.msg}
        </div>
      )}
    </div>
  );
}
