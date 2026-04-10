export default function Loading() {
  return (
    <div className="flex items-center justify-center h-[60vh]">
      <div className="w-8 h-8 border-3 border-current border-t-transparent rounded-full animate-spin" style={{ color: "var(--yunque-accent)" }} />
    </div>
  );
}
