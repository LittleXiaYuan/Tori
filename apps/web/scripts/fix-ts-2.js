const fs = require('fs');

const pagePath = 'c:/Code/AI/云雀/yunque-agent/apps/web/src/app/settings/page.tsx';
let pageCode = fs.readFileSync(pagePath, 'utf8');

// The first script didn't add Card properly because it tried to replace 'import { Tabs, Tab, Button'.
// Let's just append it to the heroui imports or create a new one.
if (!pageCode.includes('Card } from "@heroui/react"')) {
  pageCode = pageCode.replace(
    /import \{([^}]+)\} from "@heroui\/react";/,
    (match, p1) => {
      if (p1.includes('Card')) return match;
      return `import { Card, ${p1} } from "@heroui/react";`;
    }
  );
}
fs.writeFileSync(pagePath, pageCode);

// chat-empty-state.tsx
try {
  const emptyStatePath = 'c:/Code/AI/云雀/yunque-agent/apps/web/src/components/chat/chat-empty-state.tsx';
  let emptyState = fs.readFileSync(emptyStatePath, 'utf8');
  emptyState = emptyState.replace(/variant="soft"/g, 'variant="ghost"');
  fs.writeFileSync(emptyStatePath, emptyState);
} catch (e) {}

console.log("Fixed Card import");
