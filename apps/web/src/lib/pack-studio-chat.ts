export type PackStudioPatchPlanSummary = {
  pack: {
    id: string;
    name: string;
    version: string;
  };
  goal: string;
  workspace: {
    id: string;
    path: string;
    originalSha256: string;
  };
  candidates: Array<{
    key: string;
    label: string;
    filePath: string;
    riskLevel: string;
    applyable: boolean;
    gates: string[];
    contentSummary?: {
      length: number;
      hash: string;
    };
  }>;
  displayText: string;
};

export type PackStudioWorkspaceRef = {
  id: string;
  path: string;
  originalSha256: string;
};

export type PackStudioPatchDraft = {
  pack: {
    id: string;
    name: string;
    version: string;
  };
  goal: string;
  workspace: PackStudioWorkspaceRef;
  filePath: string;
  content: string;
  reason: string;
  riskLevel: string;
  gates: string[];
  displayText: string;
};

export type PackStudioPatchDraftRequest = {
  pack: {
    id: string;
    name: string;
    version: string;
  };
  goal: string;
  workspace: PackStudioWorkspaceRef;
  target: {
    filePath: string;
    label: string;
    reason: string;
    riskLevel: string;
    gates: string[];
    contentSummary?: {
      length: number;
      hash: string;
    };
  };
  starterContentLength: number;
  expectedKind: string;
  displayText: string;
};

export type PackStudioBatchDraftRequest = {
  goal: string;
  rules: string[];
  batch?: {
    page: number;
    pageCount: number;
    total: number;
    pageSize: number;
  };
  packs: Array<{
    id: string;
    name: string;
    version: string;
    status: string;
    source: string;
    missing: string[];
    readiness: string;
    priority?: {
      level: string;
      label: string;
      reason: string;
    };
    risk?: {
      level: string;
      label: string;
      requiresAuthorization: boolean;
    };
    permissionSummary: string;
    delivery?: {
      level: string;
      label: string;
      description: string;
      nextStep: string;
    };
    polishGuidance?: {
      reason: string;
      firstEdit: string;
      verify: string;
      handoff: string;
    };
    studioUrl: string;
    packageUrl: string;
    sha256: string;
  }>;
  displayText: string;
};

type PackStudioPatchPlanCandidate = PackStudioPatchPlanSummary["candidates"][number];

function asRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object" && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null;
}

function stringValue(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function stringList(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string") : [];
}

function numberValue(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function contentSummary(value: unknown): { length: number; hash: string } | undefined {
  const record = asRecord(value);
  if (!record) return undefined;
  const length = typeof record.length === "number" ? record.length : 0;
  const hash = stringValue(record.hash);
  if (!length || !hash) return undefined;
  return { length, hash };
}

function deliverySummary(value: unknown): PackStudioBatchDraftRequest["packs"][number]["delivery"] | undefined {
  const record = asRecord(value);
  if (!record) return undefined;
  const level = stringValue(record.level);
  const label = stringValue(record.label);
  const description = stringValue(record.description);
  const nextStep = stringValue(record.next_step) || stringValue(record.nextStep);
  if (!level && !label && !description && !nextStep) return undefined;
  return { level, label, description, nextStep };
}

function riskSummary(value: unknown): PackStudioBatchDraftRequest["packs"][number]["risk"] | undefined {
  const record = asRecord(value);
  if (!record) return undefined;
  const level = stringValue(record.level);
  const label = stringValue(record.label);
  const requiresAuthorization = record.requires_authorization === true || record.requiresAuthorization === true;
  if (!level && !label && !requiresAuthorization) return undefined;
  return { level, label, requiresAuthorization };
}

function prioritySummary(value: unknown): PackStudioBatchDraftRequest["packs"][number]["priority"] | undefined {
  const record = asRecord(value);
  if (!record) return undefined;
  const level = stringValue(record.level);
  const label = stringValue(record.label);
  const reason = stringValue(record.reason);
  if (!level && !label && !reason) return undefined;
  return { level, label, reason };
}

function polishGuidance(value: unknown): PackStudioBatchDraftRequest["packs"][number]["polishGuidance"] | undefined {
  const record = asRecord(value);
  if (!record) return undefined;
  const reason = stringValue(record.reason);
  const firstEdit = stringValue(record.first_edit) || stringValue(record.firstEdit);
  const verify = stringValue(record.verify);
  const handoff = stringValue(record.handoff);
  if (!reason && !firstEdit && !verify && !handoff) return undefined;
  return { reason, firstEdit, verify, handoff };
}

function displayTextWithoutJsonBlocks(text: string, markers?: string | string[]): string {
  const markerList = Array.isArray(markers) ? markers : markers ? [markers] : [];
  const marker = markerList.find((item) => text.includes(item));
  const beforeMarker = marker ? text.slice(0, text.indexOf(marker)) : text;
  return beforeMarker
    .replace(/```json[\s\S]*?```/g, "")
    .trim();
}

export function packStudioWorkspaceMatches(
  imported: PackStudioWorkspaceRef | null | undefined,
  current: { workspace_id?: string; workspace_path?: string; original_sha256?: string } | null | undefined,
): boolean {
  if (!imported || !current) return false;
  const normalizePath = (value: string) => value.replace(/\\/g, "/").toLowerCase();
  return (
    imported.id === current.workspace_id ||
    normalizePath(imported.path) === normalizePath(current.workspace_path || "") ||
    (Boolean(imported.originalSha256) && imported.originalSha256 === current.original_sha256)
  );
}

function parseJsonBlocks(text: string): unknown[] {
  const blocks = [...text.matchAll(/```json\s*([\s\S]*?)```/g)];
  const parsed: unknown[] = [];
  for (const block of blocks) {
    try {
      parsed.push(JSON.parse(block[1]));
    } catch {
      continue;
    }
  }
  return parsed;
}

export function parsePackStudioPatchPlanPrompt(text?: string): PackStudioPatchPlanSummary | null {
  if (!text?.includes("yunque.pack_studio.patch_plan.v1")) return null;
  for (const parsed of parseJsonBlocks(text)) {
      const root = asRecord(parsed);
      if (!root || root.kind !== "yunque.pack_studio.patch_plan.v1") continue;
      const pack = asRecord(root.pack);
      const workspace = asRecord(root.workspace);
      if (!pack || !workspace) continue;
      const candidates = Array.isArray(root.candidates) ? root.candidates : [];
      const parsedCandidates: PackStudioPatchPlanCandidate[] = [];
      for (const item of candidates) {
        const candidate = asRecord(item);
        if (!candidate) continue;
        const summary = contentSummary(candidate.content_summary);
        parsedCandidates.push({
          key: stringValue(candidate.key),
          label: stringValue(candidate.label),
          filePath: stringValue(candidate.file_path),
          riskLevel: stringValue(candidate.risk_level),
          applyable: candidate.applyable === true,
          gates: stringList(candidate.gates),
          ...(summary ? { contentSummary: summary } : {}),
        });
      }
      return {
        pack: {
          id: stringValue(pack.id),
          name: stringValue(pack.name),
          version: stringValue(pack.version),
        },
        goal: stringValue(root.goal),
        workspace: {
          id: stringValue(workspace.id),
          path: stringValue(workspace.path),
          originalSha256: stringValue(workspace.original_sha256),
        },
        candidates: parsedCandidates,
        displayText: displayTextWithoutJsonBlocks(text, [
          "下面是能力包工坊已准备好的改包计划",
          "下面是 Pack Studio 已准备好的 Patch Plan",
        ]),
      };
  }
  return null;
}

export function parsePackStudioPatchDraftPrompt(text?: string): PackStudioPatchDraft | null {
  if (!text?.includes("yunque.pack_studio.patch_draft.v1")) return null;
  for (const parsed of parseJsonBlocks(text)) {
    const root = asRecord(parsed);
    if (!root || root.kind !== "yunque.pack_studio.patch_draft.v1") continue;
    const pack = asRecord(root.pack);
    const workspace = asRecord(root.workspace);
    if (!pack || !workspace) continue;
    const filePath = stringValue(root.file_path);
    const content = stringValue(root.content);
    if (!filePath || !content) continue;
    return {
      pack: {
        id: stringValue(pack.id),
        name: stringValue(pack.name),
        version: stringValue(pack.version),
      },
      goal: stringValue(root.goal),
      workspace: {
        id: stringValue(workspace.id),
        path: stringValue(workspace.path),
        originalSha256: stringValue(workspace.original_sha256),
      },
      filePath,
      content,
      reason: stringValue(root.reason),
      riskLevel: stringValue(root.risk_level),
      gates: stringList(root.gates),
      displayText: displayTextWithoutJsonBlocks(text),
    };
  }
  return null;
}

export function parsePackStudioPatchDraftRequestPrompt(text?: string): PackStudioPatchDraftRequest | null {
  if (!text?.includes("yunque.pack_studio.patch_draft_request.v1")) return null;
  for (const parsed of parseJsonBlocks(text)) {
    const root = asRecord(parsed);
    if (!root || root.kind !== "yunque.pack_studio.patch_draft_request.v1") continue;
    const pack = asRecord(root.pack);
    const workspace = asRecord(root.workspace);
    const target = asRecord(root.target);
    const expectedOutput = asRecord(root.expected_output);
    if (!pack || !workspace || !target) continue;
    const filePath = stringValue(target.file_path);
    if (!filePath) continue;
    const summary = contentSummary(target.content_summary);
    const starterContent = stringValue(root.starter_content);
    return {
      pack: {
        id: stringValue(pack.id),
        name: stringValue(pack.name),
        version: stringValue(pack.version),
      },
      goal: stringValue(root.goal),
      workspace: {
        id: stringValue(workspace.id),
        path: stringValue(workspace.path),
        originalSha256: stringValue(workspace.original_sha256),
      },
      target: {
        filePath,
        label: stringValue(target.label),
        reason: stringValue(target.reason),
        riskLevel: stringValue(target.risk_level),
        gates: stringList(target.gates),
        ...(summary ? { contentSummary: summary } : {}),
      },
      starterContentLength: starterContent.length,
      expectedKind: stringValue(expectedOutput?.kind),
      displayText: displayTextWithoutJsonBlocks(text, [
        "下面是能力包工坊的改包草稿请求",
        "下面是 Pack Studio 的 Patch Draft Request",
      ]),
    };
  }
  return null;
}

export function parsePackStudioBatchDraftRequestPrompt(text?: string): PackStudioBatchDraftRequest | null {
  if (!text?.includes("yunque.pack_studio.batch_draft_request.v1")) return null;
  for (const parsed of parseJsonBlocks(text)) {
    const root = asRecord(parsed);
    if (!root || root.kind !== "yunque.pack_studio.batch_draft_request.v1") continue;
    const packs = Array.isArray(root.packs) ? root.packs : null;
    if (!packs) continue;
    const batch = asRecord(root.batch);
    return {
      goal: stringValue(root.goal),
      rules: stringList(root.rules),
      ...(batch ? {
        batch: {
          page: numberValue(batch.page),
          pageCount: numberValue(batch.page_count),
          total: numberValue(batch.total),
          pageSize: numberValue(batch.page_size),
        },
      } : {}),
      packs: packs.map((item) => {
        const pack = asRecord(item) || {};
        const delivery = deliverySummary(pack.delivery);
        const risk = riskSummary(pack.risk);
        const priority = prioritySummary(pack.priority);
        const guidance = polishGuidance(pack.polish_guidance) || polishGuidance(pack.polishGuidance);
        return {
          id: stringValue(pack.id),
          name: stringValue(pack.name),
          version: stringValue(pack.version),
          status: stringValue(pack.status),
          source: stringValue(pack.source),
          missing: stringList(pack.missing),
          readiness: stringValue(pack.readiness),
          ...(priority ? { priority } : {}),
          ...(risk ? { risk } : {}),
          permissionSummary: stringValue(pack.permission_summary) || stringValue(pack.permissionSummary),
          ...(delivery ? { delivery } : {}),
          ...(guidance ? { polishGuidance: guidance } : {}),
          studioUrl: stringValue(pack.studio_url),
          packageUrl: stringValue(pack.package_url),
          sha256: stringValue(pack.sha256),
        };
      }).filter((pack) => pack.id || pack.name),
      displayText: displayTextWithoutJsonBlocks(text),
    };
  }
  return null;
}
