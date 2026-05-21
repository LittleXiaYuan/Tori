/** Lightweight Cognition SDK schema-catalog helpers for frontend/plugin consumers. */
export const cognitionSchemaNames = [
  "pack-manifest",
  "pack-bundle",
  "pack-bundle-summary",
  "pack-bundle-digest-check",
  "pack-bundle-diff",
  "pack-bundle-review",
  "pack-bundle-apply-plan",
  "pack-bundle-apply-actions",
  "pack-bundle-apply-action-kinds",
  "pack-bundle-apply-checklist",
  "pack-bundle-apply-checklist-summary",
  "feedback-proposal",
] as const;

export type CognitionSchemaName = (typeof cognitionSchemaNames)[number];
export type JsonSchemaDocument = Record<string, unknown> & { $id?: string; title?: string };
export type CognitionSchemaInfo = { name: CognitionSchemaName; title: string; schema: string; description?: string };
export type CognitionSchemaCatalogEntry = CognitionSchemaInfo & { schema_document?: JsonSchemaDocument };
export type CognitionSchemaArtifact = CognitionSchemaInfo & { file: string };
export type CognitionSchemaArtifactCheck = { name: CognitionSchemaName; file: string; expected: string; actual?: string; match: boolean; error?: string };

const schemaNameSet = new Set<string>(cognitionSchemaNames);

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function asSchemaName(value: unknown): CognitionSchemaName | undefined {
  return typeof value === "string" && schemaNameSet.has(value) ? value as CognitionSchemaName : undefined;
}

function compactString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() ? value.trim() : undefined;
}

export function isCognitionSchemaName(value: string): value is CognitionSchemaName {
  return schemaNameSet.has(value);
}

export function cognitionSchemaFileName(name: CognitionSchemaName): string {
  return `${name}.schema.json`;
}

export function normalizeCognitionSchemaCatalog(input: unknown): CognitionSchemaCatalogEntry[] {
  if (!Array.isArray(input)) throw new Error("Cognition schema catalog must be an array");
  return input.map((entry, index) => {
    if (!isRecord(entry)) throw new Error(`Cognition schema catalog entry ${index} must be an object`);
    const name = asSchemaName(entry.name);
    const title = compactString(entry.title);
    const schema = compactString(entry.schema);
    if (!name) throw new Error(`Cognition schema catalog entry ${index} has unknown schema name`);
    if (!title) throw new Error(`Cognition schema catalog entry ${name} is missing title`);
    if (!schema) throw new Error(`Cognition schema catalog entry ${name} is missing schema id`);
    return {
      name,
      title,
      schema,
      description: compactString(entry.description),
      schema_document: isRecord(entry.schema_document) ? entry.schema_document as JsonSchemaDocument : undefined,
    };
  });
}

export function findCognitionSchema(catalog: readonly CognitionSchemaCatalogEntry[], name: CognitionSchemaName): CognitionSchemaCatalogEntry | undefined {
  return catalog.find((entry) => entry.name === name);
}

export function buildCognitionSchemaArtifacts(catalog: readonly CognitionSchemaInfo[]): CognitionSchemaArtifact[] {
  return catalog.map((entry) => ({ ...entry, file: cognitionSchemaFileName(entry.name) }));
}

export function verifyCognitionSchemaArtifacts(artifacts: readonly CognitionSchemaArtifact[], documents: Record<string, JsonSchemaDocument>): CognitionSchemaArtifactCheck[] {
  return artifacts.map((artifact) => {
    const document = documents[artifact.file];
    const actual = compactString(document?.$id);
    if (!document) return { name: artifact.name, file: artifact.file, expected: artifact.schema, match: false, error: "missing schema document" };
    if (!actual) return { name: artifact.name, file: artifact.file, expected: artifact.schema, match: false, error: "schema document missing $id" };
    return { name: artifact.name, file: artifact.file, expected: artifact.schema, actual, match: actual === artifact.schema, error: actual === artifact.schema ? undefined : "schema id mismatch" };
  });
}
