export default function Loading() {
  return (
    <div className="flex items-center justify-center h-[80vh]">
      <div className="text-center">
        <div
          className="w-8 h-8 border-2 rounded-full animate-spin mx-auto mb-3"
          style={{ borderColor: "var(--border)", borderTopColor: "var(--accent)" }}
        />
        <p className="text-xs" style={{ color: "var(--text-muted)" }}>加载任务...</p>
      </div>
    </div>
  );
}
