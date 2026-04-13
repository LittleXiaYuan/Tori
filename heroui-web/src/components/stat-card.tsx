interface StatCardProps {
  icon: React.ReactNode;
  label: string;
  value: string | number;
  color?: string;
  subtitle?: string;
}

export default function StatCard({ icon, label, value, color = "var(--yunque-accent)", subtitle }: StatCardProps) {
  return (
    <div className="section-card hover-lift stat-card">
      <div className="kpi-label stat-card-header">
        <span className="stat-card-icon" style={{ color }}>{icon}</span>
        {label}
      </div>
      <div className="kpi-value">{value}</div>
      {subtitle && <div className="kpi-sub">{subtitle}</div>}
    </div>
  );
}
