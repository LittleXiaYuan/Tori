export function recoveryGroupKeyForTarget(category: string | undefined, href: string | undefined): string | undefined {
  const normalized = (category || "").toLowerCase();
  let groupCategory = normalized;
  const target = (href || "").trim();

  if (normalized === "browser") groupCategory = "connector";
  if (normalized === "model") groupCategory = "provider";
  if (!groupCategory && target) {
    if (target.startsWith("/settings/providers")) groupCategory = "provider";
    else if (target.startsWith("/settings/connectors") || target.startsWith("/packs/browser")) groupCategory = "connector";
    else if (target.startsWith("/packs/computer-use")) groupCategory = "sandbox";
    else if (target.startsWith("/approvals")) groupCategory = "approval";
    else if (target.startsWith("/skills")) groupCategory = "skill";
    else if (target.startsWith("/tools")) groupCategory = "tool";
    else if (target.includes("#dependency-view")) groupCategory = "dependency";
  }

  return groupCategory && target ? `${groupCategory}|${target}` : undefined;
}
