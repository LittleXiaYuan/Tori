const fs = require('fs');
let code = fs.readFileSync('c:/Code/AI/云雀/yunque-agent/apps/web/src/app/packs/page.tsx', 'utf8');
code = code.replace(/import \{ ([^\}]+) \} from \"@heroui\/react\"/, (m, p1) => {
  if (!p1.includes('Accordion')) return `import { Accordion, ${p1} } from "@heroui/react"`;
  return m;
});
code = code.replace(/variant=\"flat\"/g, 'variant="soft"');
code = code.replace(/ radius=\"(sm|md|lg|full)\"/g, '');
code = code.replace(/<Disclosure/g, '<Accordion');
code = code.replace(/<\/Disclosure/g, '</Accordion');
code = code.replace(/<Accordion\.Heading>/g, '<Accordion.Item><Accordion.Trigger className="w-full flex justify-between mt-3 px-3 py-2 rounded-lg bg-white/5 border border-white/5 text-sm hover:bg-white/10 transition-colors cursor-pointer">');
code = code.replace(/<Button slot=\"trigger\" [^>]+>([^<]+)<Accordion\.Indicator \/><\/Button>/g, '$1<Accordion.Indicator />');
code = code.replace(/<\/Accordion\.Heading>/g, '</Accordion.Trigger>');
code = code.replace(/<Accordion\.Content>/g, '<Accordion.Panel>');
code = code.replace(/<\/Accordion\.Content>/g, '</Accordion.Panel></Accordion.Item>');
fs.writeFileSync('c:/Code/AI/云雀/yunque-agent/apps/web/src/app/packs/page.tsx', code);
console.log('Fixed page.tsx');
