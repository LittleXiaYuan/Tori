const fs = require('fs');

const pagePath = 'c:/Code/AI/云雀/yunque-agent/apps/web/src/app/settings/page.tsx';
let pageCode = fs.readFileSync(pagePath, 'utf8');
pageCode = pageCode.replace(/<Card className="(.*?)" shadow="none" radius="lg">/g, '<Card className="$1">');
fs.writeFileSync(pagePath, pageCode);

try {
  const emptyStatePath = 'c:/Code/AI/云雀/yunque-agent/apps/web/src/components/chat/chat-empty-state.tsx';
  let emptyState = fs.readFileSync(emptyStatePath, 'utf8');
  emptyState = emptyState.replace(/ radius="full"/g, '');
  fs.writeFileSync(emptyStatePath, emptyState);
} catch (e) {}

console.log("Fixed remaining TS errors");
