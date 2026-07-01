const fs = require('fs');
const path = 'c:/Code/AI/云雀/yunque-agent/apps/web/src/app/settings/page.tsx';
let code = fs.readFileSync(path, 'utf8');

// Replace <div className="settings-layout"> -> <div className="flex gap-8 mt-6">
code = code.replace(/<div className="settings-layout">/g, '<div className="flex gap-8 mt-6">');

// Replace settings-content
code = code.replace(/<div className="settings-content">/g, '<div className="flex-1 max-w-4xl min-w-0">');

// Replace settings-section-head
code = code.replace(/<div className="settings-section-head">/g, '<div className="flex items-center gap-3 mb-6 pb-4 border-b border-white/5">');

// Replace settings-section-head__title and count
code = code.replace(/<span className="settings-section-head__title">([^<]+)<\/span>/g, '<span className="text-xl font-semibold tracking-tight" style={{ color: "var(--yunque-text)" }}>$1</span>');
code = code.replace(/<span className="settings-section-head__count">([^<]+)<\/span>/g, '<span className="ml-3 text-sm font-medium" style={{ color: "var(--yunque-text-muted)" }}>$1</span>');

// Replace settings-fields
code = code.replace(/<div className="settings-fields">/g, '<div className="flex flex-col gap-5">');

// Replace settings-field-card
code = code.replace(/<div key=\{field\.key\} className="settings-field-card">/g, '<div key={field.key} className="p-5 rounded-xl border border-white/5 bg-[var(--yunque-surface-1)] transition-colors hover:border-white/10">');

// Replace settings-field-hint
code = code.replace(/<div className="settings-field-hint">([^<]+)<\/div>/g, '<div className="mt-2 text-xs leading-relaxed" style={{ color: "var(--yunque-text-muted)" }}>$1</div>');

// Replace settings-hero
code = code.replace(/<header className="settings-hero">/g, '<header className="flex flex-col md:flex-row md:items-end justify-between gap-6 mb-8 pt-4">');
code = code.replace(/<div className="settings-hero__copy">/g, '<div className="flex flex-col gap-2">');
code = code.replace(/<span className="settings-hero__eyebrow">([^<]+)<\/span>/g, '<span className="text-xs font-semibold tracking-wider uppercase text-[var(--yunque-accent)]">$1</span>');
code = code.replace(/<h1 id="settings-title" className="settings-hero__title">([^<]+)<\/h1>/g, '<h1 id="settings-title" className="text-3xl font-bold tracking-tight text-[var(--yunque-text)]">$1</h1>');
code = code.replace(/<p className="settings-hero__desc">([^<]+)<\/p>/g, '<p className="text-sm text-[var(--yunque-text-secondary)] max-w-xl">$1</p>');
code = code.replace(/<div className="settings-hero__actions">/g, '<div className="flex items-center gap-3">');

// Replace settings-quick-panel__head
code = code.replace(/<div className="settings-quick-panel__head">/g, '<div className="flex items-end justify-between mb-4">');

// Replace settings-advanced-panel__head
code = code.replace(/<div className="settings-advanced-panel__head">/g, '<div className="flex items-end justify-between mb-6 mt-12">');

// Replace settings-panel-title
code = code.replace(/<h2 id="([^"]+)" className="settings-panel-title">([^<]+)<\/h2>/g, '<h2 id="$1" className="text-xl font-semibold tracking-tight text-[var(--yunque-text)] mb-1">$2</h2>');

// Replace settings-panel-desc
code = code.replace(/<p className="settings-panel-desc">([^<]+)<\/p>/g, '<p className="text-sm text-[var(--yunque-text-secondary)]">$1</p>');

// Clean up directory item styles
code = code.replace(/className="settings-dir-item" data-added=\{isAdded \|\| undefined\}/g, 'className={`flex items-center gap-3 p-3 rounded-lg border text-left transition-all ${isAdded ? "bg-[rgba(34,197,94,0.05)] border-[rgba(34,197,94,0.2)]" : "bg-[var(--yunque-surface-1)] border-white/5 hover:border-[var(--yunque-accent-muted)] cursor-pointer"}`}');

fs.writeFileSync(path, code);
console.log("Refactored more settings classes in page.tsx");
