const fs = require('fs');

const pagePath = 'c:/Code/AI/云雀/yunque-agent/apps/web/src/app/settings/page.tsx';
let code = fs.readFileSync(pagePath, 'utf8');

// 1. Add editingIntegration state
if (!code.includes('editingIntegration')) {
  code = code.replace(
    /const \[detectedDirs, setDetectedDirs\] = useState<any\[\]>\(\[\]\);/,
    `const [detectedDirs, setDetectedDirs] = useState<any[]>([]);\n  const [editingIntegration, setEditingIntegration] = useState<{name: string, fields: ConfigField[]} | null>(null);`
  );
}

// 2. Add integrationGroups memo
if (!code.includes('const integrationGroups = useMemo')) {
  code = code.replace(
    /const addDirToReadPaths = \(path: string\) => \{/,
    `const integrationGroups = useMemo(() => {
    if (!currentGroup || !currentGroup.key.includes("channel")) return null;
    const groups: Record<string, ConfigField[]> = {};
    for (const field of currentGroup.fields || []) {
      let prefix = "其他集成";
      const lbl = field.label_zh || field.label || field.key;
      if (lbl.includes("Telegram")) prefix = "Telegram 机器人";
      else if (lbl.includes("飞书")) prefix = "飞书企业自建应用";
      else if (lbl.includes("Discord")) prefix = "Discord 机器人";
      else if (lbl.includes("钉钉")) prefix = "钉钉企业内部机器人";
      else if (lbl.includes("微信") || lbl.includes("企微")) prefix = "微信/企微助手";
      else if (lbl.includes("Slack")) prefix = "Slack Bot";

      if (!groups[prefix]) groups[prefix] = [];
      groups[prefix].push(field);
    }
    return groups;
  }, [currentGroup]);\n\n  const addDirToReadPaths = (path: string) => {`
  );
}

// 3. Replace the `flex flex-col gap-5` field map with the conditional integration groups
const oldMapRegex = /<div className="flex flex-col gap-5">\s*\{currentGroup\.fields\?\.map\(field => \{[\s\S]*?\n\s*\}\)\}\s*<\/div>/;

const newMapContent = `{integrationGroups ? (
                  <div className="flex flex-col gap-4">
                    {Object.entries(integrationGroups).map(([name, fields]) => (
                      <Card key={name} className="flex flex-row justify-between items-center p-5 rounded-xl border border-white/5 bg-[var(--yunque-surface-1)] hover:border-white/10 transition-colors">
                        <div className="flex flex-col gap-1">
                          <span className="font-semibold text-base" style={{ color: "var(--yunque-text)" }}>{name}</span>
                          <span className="text-xs" style={{ color: "var(--yunque-text-secondary)" }}>{fields.length} 项配置</span>
                        </div>
                        <Button size="sm" variant="outline" onPress={() => setEditingIntegration({ name, fields })}>
                          配置
                        </Button>
                      </Card>
                    ))}
                  </div>
                ) : (
                  <div className="flex flex-col gap-5">
                    {currentGroup.fields?.map(field => {
                      const isPwd = field.type === "password" || field.sensitive
                        || field.key?.toLowerCase().includes("key")
                        || field.key?.toLowerCase().includes("secret");
                      const visible = showPwd.has(field.key);

                      if (field.type === "select" && field.options) {
                        return (
                          <div key={field.key} className="p-5 rounded-xl border border-white/5 bg-[var(--yunque-surface-1)] transition-colors hover:border-white/10">
                            <Select className="w-full"
                              placeholder="请选择"
                              selectedKey={values[field.key] || ""}
                              onSelectionChange={k => upd(field.key, String(k))}>
                              <Label>
                                {field.label_zh || field.label || field.key}
                                {field.required && <span style={{ color: "var(--yunque-danger)", marginLeft: 2 }}>*</span>}
                              </Label>
                              <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
                              <Select.Popover>
                                <ListBox>
                                  {field.options.map(opt => (
                                    <ListBox.Item key={opt} id={opt} textValue={opt}>{opt}<ListBox.ItemIndicator /></ListBox.Item>
                                  ))}
                                </ListBox>
                              </Select.Popover>
                            </Select>
                            {field.hint && <div className="mt-2 text-xs leading-relaxed" style={{ color: "var(--yunque-text-muted)" }}>{field.hint}</div>}
                          </div>
                        );
                      }

                      return (
                        <div key={field.key} className="p-5 rounded-xl border border-white/5 bg-[var(--yunque-surface-1)] transition-colors hover:border-white/10">
                          <TextField name={field.key}
                            type={isPwd && !visible ? "password" : "text"}
                            isRequired={field.required}
                            value={values[field.key] || ""}
                            onChange={v => upd(field.key, v)}>
                            <Label>
                              {field.label_zh || field.label || field.key}
                              {field.required && <span style={{ color: "var(--yunque-danger)", marginLeft: 2 }}>*</span>}
                            </Label>
                            <div style={{ position: "relative" }}>
                              <Input placeholder={field.placeholder || ""} />
                              {isPwd && (
                                <Button isIconOnly aria-label="切换密码可见" variant="ghost" size="sm"
                                  onPress={() => togglePwd(field.key)} style={{
                                    position: "absolute", right: 6, top: "50%", transform: "translateY(-50%)",
                                  }}>
                                  {visible ? <EyeOff size={13} /> : <Eye size={13} />}
                                </Button>
                              )}
                            </div>
                          </TextField>
                          {field.hint && <div className="mt-2 text-xs leading-relaxed" style={{ color: "var(--yunque-text-muted)" }}>{field.hint}</div>}
                        </div>
                      );
                    })}
                  </div>
                )}`;

if (oldMapRegex.test(code) && !code.includes('integrationGroups ?')) {
  code = code.replace(oldMapRegex, newMapContent);
}

// 4. Add Modal to the bottom
const modalCode = `      {/* Integration Edit Modal */}
      <Modal isOpen={!!editingIntegration} onOpenChange={(open) => !open && setEditingIntegration(null)} size="2xl">
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader className="flex flex-col gap-1">{editingIntegration?.name}</ModalHeader>
              <ModalBody className="py-4 flex flex-col gap-4">
                {editingIntegration?.fields.map(field => {
                  const isPwd = field.type === "password" || field.sensitive || field.key?.toLowerCase().includes("key") || field.key?.toLowerCase().includes("secret");
                  const visible = showPwd.has(field.key);
                  
                  if (field.type === "select" && field.options) {
                    return (
                      <div key={field.key} className="w-full">
                        <Select className="w-full"
                          placeholder="请选择"
                          selectedKey={values[field.key] || ""}
                          onSelectionChange={k => upd(field.key, String(k))}>
                          <Label>
                            {field.label_zh || field.label || field.key}
                            {field.required && <span style={{ color: "var(--yunque-danger)", marginLeft: 2 }}>*</span>}
                          </Label>
                          <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
                          <Select.Popover>
                            <ListBox>
                              {field.options.map(opt => (
                                <ListBox.Item key={opt} id={opt} textValue={opt}>{opt}<ListBox.ItemIndicator /></ListBox.Item>
                              ))}
                            </ListBox>
                          </Select.Popover>
                        </Select>
                        {field.hint && <div className="mt-1 text-xs text-[var(--yunque-text-muted)]">{field.hint}</div>}
                      </div>
                    );
                  }

                  return (
                    <div key={field.key} className="w-full">
                      <TextField name={field.key} type={isPwd && !visible ? "password" : "text"} isRequired={field.required} value={values[field.key] || ""} onChange={v => upd(field.key, v)}>
                        <Label>{field.label_zh || field.label || field.key}</Label>
                        <div style={{ position: "relative" }}>
                          <Input placeholder={field.placeholder || ""} />
                          {isPwd && (
                            <Button isIconOnly aria-label="切换密码可见" variant="ghost" size="sm" onPress={() => togglePwd(field.key)} style={{ position: "absolute", right: 6, top: "50%", transform: "translateY(-50%)" }}>
                              {visible ? <EyeOff size={13} /> : <Eye size={13} />}
                            </Button>
                          )}
                        </div>
                      </TextField>
                      {field.hint && <div className="mt-1 text-xs text-[var(--yunque-text-muted)]">{field.hint}</div>}
                    </div>
                  );
                })}
              </ModalBody>
              <ModalFooter>
                <Button variant="ghost" onPress={onClose}>取消</Button>
                <Button style={{ background: "var(--yunque-accent)", color: "#fff" }} isPending={saving} onPress={async () => {
                  await handleSaveConfig();
                  onClose();
                }}>保存配置</Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>
    </section>
  );
}`;

if (!code.includes('Integration Edit Modal')) {
  code = code.replace(/    <\/section>\s*  \);\s*\}\s*$/g, modalCode + '\n}\n');
}

// 5. Add HeroUI imports if missing
if (!code.includes('Modal, ModalContent, ModalHeader, ModalBody, ModalFooter')) {
  code = code.replace(
    /import \{ Button, Card, TextField, Label, Input, Chip, Select, ListBox \} from "@heroui\/react";/,
    `import { Button, Card, TextField, Label, Input, Chip, Select, ListBox, Modal, ModalContent, ModalHeader, ModalBody, ModalFooter } from "@heroui/react";`
  );
}

fs.writeFileSync(pagePath, code);
console.log("Refactoring complete");
