const fs = require('fs');
const path = 'c:/Code/AI/云雀/yunque-agent/apps/web/src/app/settings/page.tsx';
let code = fs.readFileSync(path, 'utf8');

// 1. Add Tabs to imports
code = code.replace(/import \{([^}]+)\} from "@heroui\/react";/, (m, p1) => {
  if (!p1.includes('Tabs')) return `import { Tabs, Tab, ${p1} } from "@heroui/react";`;
  return m;
});

// 2. Replace settings-quick-grid
code = code.replace(
  /<div className="settings-quick-grid">[\s\S]*?<\/div>\s*<\/section>/,
  `<div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-4">
          {quickSettings.map((item) => {
            const Icon = item.icon;
            return (
              <Link key={item.href} href={item.href} className="block w-full">
                <Card className="hover-lift transition-all duration-300 w-full cursor-pointer h-full border border-white/5 bg-[var(--yunque-surface-1)]" shadow="none" radius="lg">
                  <Card.Header className="flex flex-row gap-4 items-start p-5 pb-3">
                    <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-white/5 border border-white/5">
                      <Icon size={18} style={{ color: "var(--yunque-text)" }} />
                    </div>
                    <div className="flex flex-1 flex-col gap-1.5">
                      <Card.Title className="text-base font-semibold tracking-tight" style={{ color: "var(--yunque-text)" }}>
                        {item.title}
                      </Card.Title>
                      <Card.Description className="text-sm leading-relaxed" style={{ color: "var(--yunque-text-secondary)" }}>
                        {item.desc}
                      </Card.Description>
                    </div>
                  </Card.Header>
                  <Card.Footer className="px-5 pt-0 pb-5 flex justify-end">
                    <span className="text-xs font-medium text-[var(--yunque-accent)]">{item.action} &rarr;</span>
                  </Card.Footer>
                </Card>
              </Link>
            );
          })}
        </div>
      </section>`
);

// 3. Replace Tier Selector
code = code.replace(
  /<div className="settings-tier-tabs"[\s\S]*?<\/div>/,
  `<div aria-label="设置显示层级">
            <Tabs 
              selectedKey={tierLevel} 
              onSelectionChange={(k) => selectTier(k as any)}
              variant="light"
              classNames={{
                tabList: "bg-[var(--yunque-surface-1)] border border-white/5",
                cursor: "bg-[var(--yunque-bg-hover)]",
                tab: "h-8 px-4",
                tabContent: "group-data-[selected=true]:text-[var(--yunque-text)] text-[var(--yunque-text-muted)] font-medium"
              }}
            >
              {TIER_META.map(t => {
                const reveals = TIER_RANK[t.level] > TIER_RANK[tierLevel]
                  ? rawSchema.reduce((n, g) => {
                      if (providerGroups.has(g.key)) return n;
                      return n + (g.fields || []).filter(f =>
                        !fieldVisibleAt(f, tierLevel) && fieldVisibleAt(f, t.level)).length;
                    }, 0)
                  : 0;
                return (
                  <Tab key={t.level} title={\`\${t.label}\${reveals ? \` +\${reveals}\` : ""}\`} />
                );
              })}
            </Tabs>
          </div>`
);

// 4. Replace left navigation
code = code.replace(
  /<nav className="settings-nav">[\s\S]*?<\/nav>/,
  `<nav className="w-64 flex-shrink-0 flex flex-col gap-1">
            <div className="flex items-center gap-2 rounded-lg px-3 py-2 mb-4"
              style={{ background: "var(--yunque-surface-1)", border: "1px solid var(--yunque-border)" }}>
              <Search size={14} style={{ color: "var(--yunque-text-muted)" }} />
              <input
                className="bg-transparent outline-none flex-1 text-sm"
                placeholder="搜索配置项…"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                style={{ color: "var(--yunque-text)" }}
              />
            </div>
            {!showAdvanced && !configError && (
              <div className="text-xs mb-4 px-2" style={{ color: "var(--yunque-text-muted)" }}>
                当前为常用层级；切换到高级/专家可见更多，搜索始终覆盖全部配置。
              </div>
            )}
            {schema.map(group => {
              const meta = groupMeta[group.key] || groupMeta.other;
              const Icon = meta.icon;
              const active = activeGroup === group.key;
              return (
                <button key={group.key} onClick={() => setActiveGroup(group.key)}
                  className={\`flex items-center gap-3 px-3 py-2.5 rounded-lg transition-colors text-sm font-medium \${active ? 'bg-[var(--yunque-surface-1)] border border-white/5' : 'hover:bg-white/5 border border-transparent'}\`}
                  style={{ color: active ? "var(--yunque-text)" : "var(--yunque-text-secondary)" }}>
                  <div className="flex h-6 w-6 items-center justify-center rounded-md" style={{ background: active ? \`color-mix(in srgb, \${meta.color} 15%, transparent)\` : "transparent" }}>
                    <Icon size={14} style={{ color: active ? meta.color : "var(--yunque-text-muted)" }} />
                  </div>
                  <span className="flex-1 text-left">{group.label_zh || group.label}</span>
                  {group.fields && group.fields.length > 0 && (
                    <span className="text-xs px-1.5 py-0.5 rounded-full" style={{ background: active ? "var(--yunque-bg)" : "var(--yunque-surface-1)", color: "var(--yunque-text-muted)" }}>
                      {group.fields.length}
                    </span>
                  )}
                </button>
              );
            })}
          </nav>`
);

fs.writeFileSync(path, code);
console.log("Replaced grid, tier, and sidebar in page.tsx");
