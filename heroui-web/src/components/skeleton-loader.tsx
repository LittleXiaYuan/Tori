"use client";

interface SkeletonProps {
  className?: string;
  style?: React.CSSProperties;
}

function Bone({ className = "", style }: SkeletonProps) {
  return (
    <div
      className={`skeleton-bone ${className}`}
      style={{
        borderRadius: 8,
        background: "var(--yunque-bg-hover)",
        ...style,
      }}
    />
  );
}

export function DashboardSkeleton() {
  return (
    <div className="page-root space-y-6 animate-fade-in-up" role="status" aria-busy="true" aria-label="页面加载中">
      <div className="flex items-center justify-between">
        <Bone style={{ width: 200, height: 28 }} />
        <Bone style={{ width: 32, height: 32, borderRadius: "50%" }} />
      </div>
      <div className="grid grid-cols-4 gap-4">
        {[...Array(4)].map((_, i) => (
          <div key={i} className="section-card p-5 space-y-3">
            <Bone style={{ width: 80, height: 12 }} />
            <Bone style={{ width: 120, height: 28 }} />
            <Bone style={{ width: "100%", height: 4 }} />
          </div>
        ))}
      </div>
      <div className="grid grid-cols-12 gap-6">
        <div className="col-span-8 section-card p-5 space-y-3">
          <Bone style={{ width: 140, height: 16 }} />
          {[...Array(5)].map((_, i) => (
            <div key={i} className="flex items-center gap-3">
              <Bone style={{ width: 32, height: 32, borderRadius: "50%" }} />
              <div className="flex-1 space-y-2">
                <Bone style={{ width: `${60 + Math.random() * 30}%`, height: 12 }} />
                <Bone style={{ width: `${30 + Math.random() * 20}%`, height: 10 }} />
              </div>
            </div>
          ))}
        </div>
        <div className="col-span-4 section-card p-5 space-y-3">
          <Bone style={{ width: 100, height: 16 }} />
          {[...Array(4)].map((_, i) => (
            <div key={i} className="flex items-center justify-between">
              <Bone style={{ width: `${40 + Math.random() * 30}%`, height: 12 }} />
              <Bone style={{ width: 50, height: 12 }} />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

export function ChatSkeleton() {
  return (
    <div className="page-root animate-fade-in-up" role="status" aria-busy="true" aria-label="页面加载中" style={{ display: "flex", flexDirection: "column", height: "calc(100vh - 64px)" }}>
      <div className="flex items-center justify-between p-4">
        <Bone style={{ width: 160, height: 24 }} />
        <div className="flex gap-2">
          <Bone style={{ width: 32, height: 32, borderRadius: 8 }} />
          <Bone style={{ width: 32, height: 32, borderRadius: 8 }} />
        </div>
      </div>
      <div className="flex-1 px-4 space-y-4 overflow-hidden">
        {[...Array(4)].map((_, i) => {
          const isUser = i % 2 === 0;
          return (
            <div key={i} className={`flex ${isUser ? "justify-end" : "justify-start"}`}>
              <div className="space-y-2" style={{ maxWidth: "70%" }}>
                <Bone style={{ width: `${150 + Math.random() * 200}px`, height: 14 }} />
                {!isUser && <Bone style={{ width: `${100 + Math.random() * 150}px`, height: 14 }} />}
                {!isUser && Math.random() > 0.5 && <Bone style={{ width: `${80 + Math.random() * 100}px`, height: 14 }} />}
              </div>
            </div>
          );
        })}
      </div>
      <div className="p-4">
        <Bone style={{ width: "100%", height: 48, borderRadius: 12 }} />
      </div>
    </div>
  );
}

export function ListSkeleton({ rows = 6 }: { rows?: number }) {
  return (
    <div className="page-root space-y-4 animate-fade-in-up" role="status" aria-busy="true" aria-label="页面加载中">
      <div className="flex items-center justify-between">
        <Bone style={{ width: 180, height: 24 }} />
        <Bone style={{ width: 80, height: 32, borderRadius: 8 }} />
      </div>
      <div className="space-y-3">
        {[...Array(rows)].map((_, i) => (
          <div key={i} className="section-card p-4 flex items-center gap-4">
            <Bone style={{ width: 40, height: 40, borderRadius: 8 }} />
            <div className="flex-1 space-y-2">
              <Bone style={{ width: `${50 + Math.random() * 30}%`, height: 14 }} />
              <Bone style={{ width: `${20 + Math.random() * 30}%`, height: 10 }} />
            </div>
            <Bone style={{ width: 60, height: 24, borderRadius: 12 }} />
          </div>
        ))}
      </div>
    </div>
  );
}

export { Bone };
