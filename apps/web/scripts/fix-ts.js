const fs = require('fs');

// 1. Fix SettingsCard
const cardPath = 'c:/Code/AI/云雀/yunque-agent/apps/web/src/app/settings/_components/settings-card.tsx';
let cardCode = fs.readFileSync(cardPath, 'utf8');
cardCode = cardCode.replace(/shadow="none"\s+radius="lg"/g, '');
fs.writeFileSync(cardPath, cardCode);

// 2. Fix layout.tsx tabs
const layoutPath = 'c:/Code/AI/云雀/yunque-agent/apps/web/src/app/settings/layout.tsx';
let layoutCode = fs.readFileSync(layoutPath, 'utf8');
layoutCode = layoutCode.replace(
  /<Tabs[\s\S]*?<\/Tabs>/,
  `<div className="flex gap-6 border-b border-white/5 relative">
          {tabs.map((tab) => {
            const isSelected = selectedKey === tab.href;
            return (
              <button
                key={tab.href}
                onClick={() => startTransition(() => router.push(tab.href))}
                className={\`relative h-12 flex items-center space-x-2 px-1 text-sm font-medium transition-colors \${isSelected ? 'text-[var(--yunque-accent)]' : 'text-[var(--yunque-text-secondary)] hover:text-[var(--yunque-text)]'}\`}
              >
                {tab.icon}
                <span>{tab.label}</span>
                {isSelected && (
                  <span className="absolute bottom-0 left-0 w-full h-[2px] bg-[var(--yunque-accent)]" />
                )}
              </button>
            );
          })}
        </div>`
);
layoutCode = layoutCode.replace(/import \{ Tabs, Tab \} from "@heroui\/react";/, '');
fs.writeFileSync(layoutPath, layoutCode);

// 3. Fix page.tsx imports and tabs
const pagePath = 'c:/Code/AI/云雀/yunque-agent/apps/web/src/app/settings/page.tsx';
let pageCode = fs.readFileSync(pagePath, 'utf8');
pageCode = pageCode.replace(/import \{ Tabs, Tab, Button/g, 'import { Card, Button');
// Remove Tabs import if it was added
pageCode = pageCode.replace(/import \{ Tabs, Tab \} from "@heroui\/react";/g, '');

pageCode = pageCode.replace(
  /<Tabs[\s\S]*?<\/Tabs>/,
  `<div className="flex bg-[var(--yunque-surface-1)] border border-white/5 rounded-lg p-1">
              {TIER_META.map(t => {
                const active = tierLevel === t.level;
                const reveals = TIER_RANK[t.level] > TIER_RANK[tierLevel]
                  ? rawSchema.reduce((n, g) => {
                      if (providerGroups.has(g.key)) return n;
                      return n + (g.fields || []).filter(f =>
                        !fieldVisibleAt(f, tierLevel) && fieldVisibleAt(f, t.level)).length;
                    }, 0)
                  : 0;
                return (
                  <button
                    key={t.level}
                    onClick={() => selectTier(t.level as any)}
                    className={\`h-8 px-4 text-sm font-medium rounded-md transition-colors \${active ? 'bg-[var(--yunque-bg-hover)] text-[var(--yunque-text)]' : 'text-[var(--yunque-text-muted)] hover:text-[var(--yunque-text)]'}\`}
                  >
                    {t.label}{reveals ? \` +\${reveals}\` : ""}
                  </button>
                );
              })}
            </div>`
);
fs.writeFileSync(pagePath, pageCode);

// 4. Fix other random components
try {
  const emptyStatePath = 'c:/Code/AI/云雀/yunque-agent/apps/web/src/components/chat/chat-empty-state.tsx';
  let emptyState = fs.readFileSync(emptyStatePath, 'utf8');
  emptyState = emptyState.replace(/variant="flat"/g, 'variant="soft"');
  fs.writeFileSync(emptyStatePath, emptyState);

  const inputAreaPath = 'c:/Code/AI/云雀/yunque-agent/apps/web/src/components/chat/chat-input-area.tsx';
  let inputArea = fs.readFileSync(inputAreaPath, 'utf8');
  inputArea = inputArea.replace(/variant="light"/g, 'variant="ghost"');
  inputArea = inputArea.replace(/ radius="(sm|md|lg|full)"/g, '');
  fs.writeFileSync(inputAreaPath, inputArea);
} catch (e) {}

console.log("Fixed TS errors");
