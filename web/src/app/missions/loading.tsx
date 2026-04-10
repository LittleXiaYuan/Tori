export default function Loading() {
  return (
    <div className="flex items-center justify-center h-[60vh]">
      <div
        className="w-8 h-8 border-2 rounded-full animate-spin"
        style={{ borderColor: "var(--border)", borderTopColor: "var(--accent)" }}
      />
    </div>
  );
}
