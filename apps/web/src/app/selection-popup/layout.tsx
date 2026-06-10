export default function SelectionPopupLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="selection-popup-root" style={{ minHeight: "100vh" }}>
      {children}
    </div>
  );
}
