const fs = require('fs');

const pagePath = 'c:/Code/AI/云雀/yunque-agent/apps/web/src/app/settings/page.tsx';
let code = fs.readFileSync(pagePath, 'utf8');

// Fix imports
code = code.replace(
  /ModalContent, ModalHeader, ModalBody, ModalFooter,\s*/g,
  ''
);

// Fix JSX tags
code = code.replace(/<ModalContent>/g, '<Modal.Content>');
code = code.replace(/<\/ModalContent>/g, '</Modal.Content>');
code = code.replace(/<ModalHeader/g, '<Modal.Header');
code = code.replace(/<\/ModalHeader>/g, '</Modal.Header>');
code = code.replace(/<ModalBody/g, '<Modal.Body');
code = code.replace(/<\/ModalBody>/g, '</Modal.Body>');
code = code.replace(/<ModalFooter>/g, '<Modal.Footer>');
code = code.replace(/<\/ModalFooter>/g, '</Modal.Footer>');

fs.writeFileSync(pagePath, code);

const modalPath = 'c:/Code/AI/云雀/yunque-agent/apps/web/src/components/cherry/settings-modal.tsx';
let mcode = fs.readFileSync(modalPath, 'utf8');
mcode = mcode.replace(/ModalContent, ModalHeader, ModalBody, ModalFooter,\s*/g, '');
mcode = mcode.replace(/<ModalContent>/g, '<Modal.Content>');
mcode = mcode.replace(/<\/ModalContent>/g, '</Modal.Content>');
mcode = mcode.replace(/<ModalHeader/g, '<Modal.Header');
mcode = mcode.replace(/<\/ModalHeader>/g, '</Modal.Header>');
mcode = mcode.replace(/<ModalBody/g, '<Modal.Body');
mcode = mcode.replace(/<\/ModalBody>/g, '</Modal.Body>');
mcode = mcode.replace(/<ModalFooter>/g, '<Modal.Footer>');
mcode = mcode.replace(/<\/ModalFooter>/g, '</Modal.Footer>');
// also fix Textarea
mcode = mcode.replace(/Textarea/g, 'TextArea');
fs.writeFileSync(modalPath, mcode);

console.log("Fixed HeroUI v3 component syntax.");
