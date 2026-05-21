/** Local-only Cognition SDK package manifest helpers for frontends, plugins, and automation scripts. */
export type CognitionSdkCapability = { name: string; symbols: string[] };
export type CognitionSdkCliTool = { name: string; file: string; commands: string[]; flags: string[]; schemas?: string[] };
export type CognitionSdkGoPackage = { importPath: string; implementationFiles: string[] };
export type CognitionSdkPackageManifest = {
  version: number;
  domain: "cognisdk-package";
  description: string;
  principles: string[];
  goPackage: CognitionSdkGoPackage;
  capabilities: CognitionSdkCapability[];
  cli: CognitionSdkCliTool[];
  tests: string[];
  docs: string[];
};

export type CognitionSdkPackageSummary = {
  domain: "cognisdk-package";
  capabilityCount: number;
  cliToolCount: number;
  commandCount: number;
  schemaCount: number;
  testCount: number;
  docCount: number;
  goPackage: string;
};

export type CognitionSdkEntrypoint = {
  kind: "go" | "cli" | "test" | "doc";
  name: string;
  path: string;
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function stringValue(value: unknown, label: string): string {
  if (typeof value !== "string" || !value.trim()) throw new Error(`Cognition SDK package manifest ${label} must be a non-empty string`);
  return value.trim();
}

function stringArray(value: unknown, label: string): string[] {
  if (!Array.isArray(value)) throw new Error(`Cognition SDK package manifest ${label} must be an array`);
  return value.map((item, index) => stringValue(item, `${label}[${index}]`));
}

function normalizeGoPackage(value: unknown): CognitionSdkGoPackage {
  if (!isRecord(value)) throw new Error("Cognition SDK package manifest goPackage must be an object");
  const goPackage = {
    importPath: stringValue(value.importPath, "goPackage.importPath"),
    implementationFiles: stringArray(value.implementationFiles, "goPackage.implementationFiles"),
  };
  if (!goPackage.importPath.includes("pkg/cognisdk")) throw new Error("Cognition SDK package manifest goPackage.importPath must point at pkg/cognisdk");
  return goPackage;
}

function normalizeCapabilities(value: unknown): CognitionSdkCapability[] {
  if (!Array.isArray(value)) throw new Error("Cognition SDK package manifest capabilities must be an array");
  return value.map((item, index) => {
    if (!isRecord(item)) throw new Error(`Cognition SDK package manifest capabilities[${index}] must be an object`);
    return { name: stringValue(item.name, `capabilities[${index}].name`), symbols: stringArray(item.symbols, `capabilities[${index}].symbols`) };
  });
}

function normalizeCliTools(value: unknown): CognitionSdkCliTool[] {
  if (!Array.isArray(value)) throw new Error("Cognition SDK package manifest cli must be an array");
  return value.map((item, index) => {
    if (!isRecord(item)) throw new Error(`Cognition SDK package manifest cli[${index}] must be an object`);
    return {
      name: stringValue(item.name, `cli[${index}].name`),
      file: stringValue(item.file, `cli[${index}].file`),
      commands: stringArray(item.commands, `cli[${index}].commands`),
      flags: stringArray(item.flags, `cli[${index}].flags`),
      schemas: item.schemas === undefined ? undefined : stringArray(item.schemas, `cli[${index}].schemas`),
    };
  });
}

export function normalizeCognitionSdkPackageManifest(input: unknown): CognitionSdkPackageManifest {
  if (!isRecord(input)) throw new Error("Cognition SDK package manifest must be an object");
  const version = typeof input.version === "number" ? input.version : Number.NaN;
  if (!Number.isInteger(version) || version < 1) throw new Error("Cognition SDK package manifest version must be a positive integer");
  const domain = stringValue(input.domain, "domain");
  if (domain !== "cognisdk-package") throw new Error("Cognition SDK package manifest domain must be cognisdk-package");
  return {
    version,
    domain,
    description: stringValue(input.description, "description"),
    principles: stringArray(input.principles, "principles"),
    goPackage: normalizeGoPackage(input.goPackage),
    capabilities: normalizeCapabilities(input.capabilities),
    cli: normalizeCliTools(input.cli),
    tests: stringArray(input.tests, "tests"),
    docs: stringArray(input.docs, "docs"),
  };
}

export function summarizeCognitionSdkPackageManifest(manifest: CognitionSdkPackageManifest): CognitionSdkPackageSummary {
  const schemaNames = new Set(manifest.cli.flatMap((tool) => tool.schemas ?? []));
  return {
    domain: manifest.domain,
    capabilityCount: manifest.capabilities.length,
    cliToolCount: manifest.cli.length,
    commandCount: manifest.cli.reduce((sum, tool) => sum + tool.commands.length, 0),
    schemaCount: schemaNames.size,
    testCount: manifest.tests.length,
    docCount: manifest.docs.length,
    goPackage: manifest.goPackage.importPath,
  };
}

export function listCognitionSdkPackageEntrypoints(manifest: CognitionSdkPackageManifest): CognitionSdkEntrypoint[] {
  return [
    ...manifest.goPackage.implementationFiles.map((path) => ({ kind: "go" as const, name: path.split("/").pop() || path, path })),
    ...manifest.cli.map((tool) => ({ kind: "cli" as const, name: tool.name, path: tool.file })),
    ...manifest.tests.map((path) => ({ kind: "test" as const, name: path.split("/").pop() || path, path })),
    ...manifest.docs.map((path) => ({ kind: "doc" as const, name: path.split("/").pop() || path, path })),
  ];
}

export function findCognitionSdkCapability(manifest: CognitionSdkPackageManifest, name: string): CognitionSdkCapability | undefined {
  return manifest.capabilities.find((capability) => capability.name === name);
}
