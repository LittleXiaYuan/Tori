"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { useRouter } from "next/navigation";
import { api, type MetricsSnapshot, type VersionInfo, type ConversationInfo } from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import { useI18n } from "@/lib/i18n";
import {
  MessageCircle,
  Zap,
  BookOpen,
  ScanFace,
  Sparkles,
  Clock,
  ChevronRight,
  ArrowUp,
  Plus,
  Code2,
  Mic,
  Search,
  PenTool,
  BarChart3,
  FileText,
  Globe,
} from "lucide-react";

/* ── Quick action pills (Manus-style horizontal tags) ── */
const quickActions = [
  { icon: MessageCircle, label: "对话", labelEn: "Chat", href: "/chat" },
  { icon: Zap, label: "创建任务", labelEn: "New Task", href: "/missions" },
  { icon: BookOpen, label: "知识库", labelEn: "Knowledge", href: "/knowledge" },
  { icon: ScanFace, label: "人格", labelEn: "Persona", href: "/persona" },
  { icon: Code2, label: "技能", labelEn: "Skills", href: "/skills" },
];

/* ── "开始使用" task templates (Manus-style 2-column card grid) ── */
const quickTemplates = [
  {
    icon: "🔍",
    title: "智能搜索",
    titleEn: "Smart Search",
    desc: "帮我搜索并整理关于某个主题的最新资讯",
    descEn: "Search and compile the latest information on a topic",
    prompt: "帮我搜索并整理关于",
    bg: "rgba(59, 130, 246, 0.08)",
  },
  {
    icon: "📝",
    title: "内容创作",
    titleEn: "Content Writing",
    desc: "撰写文章、报告、邮件或创意文案",
    descEn: "Write articles, reports, emails or creative copy",
    prompt: "帮我写一篇关于",
    bg: "rgba(168, 85, 247, 0.08)",
  },
  {
    icon: "💻",
    title: "代码助手",
    titleEn: "Code Assistant",
    desc: "编写、调试或优化代码，解释技术概念",
    descEn: "Write, debug or optimize code and explain concepts",
    prompt: "帮我编写一个",
    bg: "rgba(16, 185, 129, 0.08)",
  },
  {
    icon: "📊",
    title: "数据分析",
    titleEn: "Data Analysis",
    desc: "分析数据、制作图表或提取关键洞见",
    descEn: "Analyze data, create charts or extract key insights",
    prompt: "帮我分析这份数据",
    bg: "rgba(245, 158, 11, 0.08)",
  },
  {
    icon: "🎨",
    title: "设计灵感",
    titleEn: "Design Ideas",
    desc: "获取设计建议、配色方案或 UI 参考",
    descEn: "Get design advice, color palettes or UI references",
    prompt: "帮我设计一个",
    bg: "rgba(236, 72, 153, 0.08)",
  },
  {
    icon: "📚",
    title: "知识问答",
    titleEn: "Knowledge Q&A",
    desc: "深入解释概念、原理或技术细节",
    descEn: "Deep explanation of concepts, principles or tech details",
    prompt: "请详细解释",
    bg: "rgba(6, 182, 212, 0.08)",
  },
];

function RecentConversationItem({ conv, onClick }: { conv: ConversationInfo; onClick: () => void }) {
  const timeAgo = (ts: string) => {
    const diff = Date.now() - new Date(ts).getTime();
    const mins = Math.floor(diff / 60000);
    if (mins < 1) return "刚刚";
    if (mins < 60) return `${mins}分钟前`;
    const hours = Math.floor(mins / 60);
    if (hours < 24) return `${hours}小时前`;
    return `${Math.floor(hours / 24)}天前`;
  };

  return (
    <button
      onClick={onClick}
      className="group flex items-center gap-3 w-full p-3 rounded-xl transition-all duration-200 text-left cursor-pointer relative"
      style={{ background: "transparent" }}
      onMouseEnter={(e) => { e.currentTarget.style.background = "var(--bg-hover)"; }}
      onMouseLeave={(e) => { e.currentTarget.style.background = "transparent"; }}
    >
      {/* Hover accent bar */}
      <div
        className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] rounded-full transition-all duration-200 opacity-0 group-hover:opacity-100 group-hover:h-6"
        style={{ background: "var(--accent)", height: 0 }}
      />
      <div className="w-8 h-8 rounded-lg flex items-center justify-center shrink-0" style={{ background: "var(--accent-subtle)", color: "var(--accent)" }}>
        <MessageCircle size={14} />
      </div>
      <div className="flex-1 min-w-0">
        <div className="text-sm font-medium truncate" style={{ color: "var(--text)" }}>
          {conv.name || conv.id}
        </div>
        <div className="text-xs mt-0.5" style={{ color: "var(--text-muted)" }}>
          {timeAgo(conv.updated_at)}
        </div>
      </div>
      <ChevronRight size={14} className="opacity-0 group-hover:opacity-100 transition-opacity shrink-0" style={{ color: "var(--text-muted)" }} />
    </button>
  );
}

export default function HomePage() {
  const router = useRouter();
  const { locale } = useI18n();
  const [metrics, setMetrics] = useState<MetricsSnapshot | null>(null);
  const [version, setVersion] = useState<VersionInfo | null>(null);
  const [conversations, setConversations] = useState<ConversationInfo[]>([]);
  const [quickInput, setQuickInput] = useState("");
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    const load = async () => {
      try {
        const [m, v] = await Promise.all([api.metrics(), api.version()]);
        setMetrics(m);
        setVersion(v);
      } catch { /* offline */ }
    };
    load();
    const interval = setInterval(load, 10000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    api.conversations(false)
      .then((data) => setConversations((data.sessions || []).slice(0, 5)))
      .catch(() => {});
  }, []);

  // Auto-resize textarea
  useEffect(() => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = Math.min(el.scrollHeight, 120) + "px";
  }, [quickInput]);

  const handleQuickSend = useCallback(() => {
    if (!quickInput.trim()) return;
    const sid = `s_${Date.now()}`;
    if (typeof window !== "undefined") {
      localStorage.setItem("yunque_session_id", sid);
      localStorage.setItem("yunque_quick_msg", quickInput.trim());
    }
    router.push("/chat");
  }, [quickInput, router]);

  const online = !!metrics;

  return (
    <div className="animate-in" style={{ maxWidth: 720, margin: "0 auto" }}>
      {/* ── Hero: centered title (Manus-style) ── */}
      <BlurFade delay={0}>
        <div className="flex flex-col items-center text-center" style={{ paddingTop: "15vh", paddingBottom: 32 }}>
          <h1
            className="heading-serif text-4xl tracking-tight"
            style={{ color: "var(--text)", lineHeight: 1.2 }}
          >
            {locale === "zh" ? "我能为你做什么？" : "What can I do for you?"}
          </h1>
          <p className="text-sm mt-3" style={{ color: "var(--text-muted)", fontFamily: "'Inter', sans-serif" }}>
            Less structure, more intelligence.
          </p>
          {/* Status line */}
          <div className="flex items-center gap-2 mt-3">
            <span
              className={online ? "breathe" : ""}
              style={{
                width: 6, height: 6, borderRadius: "50%",
                background: online ? "var(--success)" : "var(--text-muted)",
                display: "inline-block",
              }}
            />
            <span className="text-xs" style={{ color: "var(--text-muted)" }}>
              {online
                ? `${locale === "zh" ? "在线" : "Online"} · ${metrics?.skills?.length ?? 0} ${locale === "zh" ? "技能就绪" : "skills ready"}`
                : locale === "zh" ? "Agent 未连接" : "Agent offline"}
            </span>
          </div>
        </div>
      </BlurFade>

      {/* ── Input box (Manus-style: large, icons inside) ── */}
      <BlurFade delay={0.04}>
        <div
          className="rounded-2xl border mb-5 input-manus"
          style={{ background: "var(--bg-card)", borderColor: "var(--border)", boxShadow: "var(--shadow-md)" }}
        >
          <textarea
            ref={textareaRef}
            value={quickInput}
            onChange={(e) => setQuickInput(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); handleQuickSend(); } }}
            placeholder={locale === "zh" ? "分配一个任务或提问任何问题..." : "Assign a task or ask any question..."}
            rows={1}
            className="w-full bg-transparent px-5 pt-4 pb-2 text-sm outline-none resize-none"
            style={{ color: "var(--text)", maxHeight: 120 }}
          />
          {/* Toolbar row */}
          <div className="flex items-center justify-between px-4 pb-3">
            <div className="flex items-center gap-1">
              <button
                className="p-2 rounded-lg transition-colors hover:bg-[var(--bg-hover)]"
                style={{ color: "var(--text-muted)" }}
                title={locale === "zh" ? "附件" : "Attach"}
              >
                <Plus size={16} />
              </button>
              <button
                className="p-2 rounded-lg transition-colors hover:bg-[var(--bg-hover)]"
                style={{ color: "var(--text-muted)" }}
                title={locale === "zh" ? "语音" : "Voice"}
              >
                <Mic size={16} />
              </button>
            </div>
            <button
              onClick={handleQuickSend}
              disabled={!quickInput.trim()}
              className={`btn-send-manus ${quickInput.trim() ? "active" : "inactive"}`}
            >
              <ArrowUp size={18} />
            </button>
          </div>
        </div>
      </BlurFade>

      {/* ── Quick action pills (horizontal, Manus-style) ── */}
      <BlurFade delay={0.07}>
        <div className="flex items-center justify-center gap-2 flex-wrap mb-8">
          {quickActions.map((action) => {
            const Icon = action.icon;
            return (
              <button
                key={action.href}
                onClick={() => router.push(action.href)}
                className="inline-flex items-center gap-1.5 px-4 py-2 rounded-full text-xs font-medium border transition-all duration-200 cursor-pointer hover:scale-[1.03]"
                style={{ borderColor: "var(--border)", color: "var(--text-secondary)", background: "var(--bg-card)", boxShadow: "var(--shadow-sm)" }}
                onMouseEnter={(e) => { e.currentTarget.style.borderColor = "var(--accent)"; e.currentTarget.style.color = "var(--accent)"; e.currentTarget.style.boxShadow = "var(--shadow-md)"; }}
                onMouseLeave={(e) => { e.currentTarget.style.borderColor = "var(--border)"; e.currentTarget.style.color = "var(--text-secondary)"; e.currentTarget.style.boxShadow = "var(--shadow-sm)"; }}
              >
                <Icon size={13} />
                {locale === "zh" ? action.label : action.labelEn}
              </button>
            );
          })}
        </div>
      </BlurFade>

      {/* ── "开始使用" Template cards (Manus-style 2-col grid) ── */}
      <BlurFade delay={0.09}>
        <div className="mb-10">
          <p className="text-xs font-medium mb-3 px-1" style={{ color: "var(--text-muted)" }}>
            {locale === "zh" ? "开始使用" : "Get started"}
          </p>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {quickTemplates.map((tmpl) => (
              <button
                key={tmpl.title}
                className="template-card"
                onClick={() => {
                  setQuickInput(tmpl.prompt);
                  textareaRef.current?.focus();
                }}
              >
                <div className="template-card-icon" style={{ background: tmpl.bg }}>
                  {tmpl.icon}
                </div>
                <div className="template-card-content">
                  <div className="template-card-title">
                    {locale === "zh" ? tmpl.title : tmpl.titleEn}
                    <ChevronRight size={14} />
                  </div>
                  <div className="template-card-desc">
                    {locale === "zh" ? tmpl.desc : tmpl.descEn}
                  </div>
                </div>
              </button>
            ))}
          </div>
        </div>
      </BlurFade>

      {/* ── Recent conversations ── */}
      {conversations.length > 0 && (
        <BlurFade delay={0.1}>
          <div
            className="rounded-2xl border p-4 mb-6"
            style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
          >
            <div className="flex items-center justify-between mb-2 px-1">
              <div className="flex items-center gap-2">
                <Clock size={14} style={{ color: "var(--text-muted)" }} />
                <span className="text-xs font-medium" style={{ color: "var(--text-secondary)" }}>
                  {locale === "zh" ? "最近对话" : "Recent Conversations"}
                </span>
              </div>
              <button
                onClick={() => router.push("/chat")}
                className="text-xs transition-colors"
                style={{ color: "var(--accent)" }}
              >
                {locale === "zh" ? "查看全部" : "View All"} →
              </button>
            </div>
            <div className="space-y-0.5">
              {conversations.map((conv) => (
                <RecentConversationItem
                  key={conv.id}
                  conv={conv}
                  onClick={() => {
                    if (typeof window !== "undefined") {
                      localStorage.setItem("yunque_session_id", conv.id);
                    }
                    router.push("/chat");
                  }}
                />
              ))}
            </div>
          </div>
        </BlurFade>
      )}

      {/* ── Yunque's inner monologue (unique to Yunque, not in Manus) ── */}
      <BlurFade delay={0.13}>
        <div
          className="rounded-2xl border px-6 py-5 text-center"
          style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
        >
          <Sparkles size={16} className="mx-auto mb-3" style={{ color: "var(--accent)", opacity: 0.5 }} />
          <p className="text-sm italic" style={{ color: "var(--text-secondary)" }}>
            "凡是过往，皆为序章。"
          </p>
          <p className="text-xs mt-2" style={{ color: "var(--text-muted)" }}>
            —— 云雀 · 内心独白
          </p>
        </div>
      </BlurFade>
    </div>
  );
}
