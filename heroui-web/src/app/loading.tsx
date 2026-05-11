export default function Loading() {
  return (
    <div className="flex h-[60vh] flex-col items-center justify-center gap-4">
      <div className="flex h-14 w-14 items-center justify-center rounded-3xl text-xl font-black text-white shadow-lg" style={{ background: "linear-gradient(135deg, var(--yunque-accent), #7c3aed)" }}>
        云
      </div>
      <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>云雀 Agent 正在加载...</div>
    </div>
  );
}
