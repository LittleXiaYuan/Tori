const fs = require('fs');

const pagePath = 'c:/Code/AI/云雀/yunque-agent/apps/web/src/app/settings/page.tsx';
let code = fs.readFileSync(pagePath, 'utf8');

// 1. Move integrationGroups
const integrationGroupsRegex = /const integrationGroups = useMemo\(\(\) => \{[\s\S]*?\}, \[currentGroup\]\);\s*/;
const match = code.match(integrationGroupsRegex);
if (match) {
  code = code.replace(integrationGroupsRegex, '');
  
  const currentGroupRegex = /(const currentGroup = [^\n]*\n)/;
  code = code.replace(currentGroupRegex, `$1\n  ${match[0]}\n`);
}

// 2. Fix missing Modal imports
code = code.replace(
  /import \{ Card, Modal, Tabs, Tab,\s*Button, Spinner, TextField, Input, Label,\s*Select, ListBox, Chip, Separator,\s*\} from "@heroui\/react";/m,
  `import { Card, Modal, ModalContent, ModalHeader, ModalBody, ModalFooter, Tabs, Tab, Button, Spinner, TextField, Input, Label, Select, ListBox, Chip, Separator } from "@heroui/react";`
);

fs.writeFileSync(pagePath, code);
console.log("Fixed types");
