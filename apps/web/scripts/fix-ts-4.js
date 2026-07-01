const fs = require('fs');

try {
  const packsPagePath = 'c:/Code/AI/云雀/yunque-agent/apps/web/src/app/packs/page.tsx';
  let packsPageCode = fs.readFileSync(packsPagePath, 'utf8');
  packsPageCode = packsPageCode.replace(/ref=\{filterTriggerRef\}/g, '');
  packsPageCode = packsPageCode.replace(/ref=\{filterDialogRef\}/g, '');
  fs.writeFileSync(packsPagePath, packsPageCode);
} catch (e) {}

try {
  const emptyStatePath = 'c:/Code/AI/云雀/yunque-agent/apps/web/src/components/chat/chat-empty-state.tsx';
  let emptyState = fs.readFileSync(emptyStatePath, 'utf8');
  emptyState = emptyState.replace(/ title=\{prompt\}/g, '');
  fs.writeFileSync(emptyStatePath, emptyState);
} catch (e) {}

console.log("Fixed remaining minor TS errors in packs/page.tsx and chat-empty-state.tsx");
