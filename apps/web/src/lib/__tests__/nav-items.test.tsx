import { describe, expect, it } from "vitest";
import { DEFAULT_NAV_ITEM_IDS, NAV_ITEMS, filterNavItemsByProfile } from "@/lib/nav-items";

describe("nav capability layering", () => {
  it("keeps easy mode limited to the main product path", () => {
    const easyItems = filterNavItemsByProfile(NAV_ITEMS, "easy");
    const easyIds = new Set(easyItems.map((item) => item.id));

    expect(easyIds).toEqual(DEFAULT_NAV_ITEM_IDS);
    expect(easyItems).toHaveLength(DEFAULT_NAV_ITEM_IDS.size);
    expect(easyItems.some((item) => item.layer === "lab" || item.layer === "control-plane")).toBe(false);
    expect(easyItems.every((item) => item.layer === "core" || item.id === "nav-packs")).toBe(true);
  });

  it("requires all default-visible items to be declared explicitly", () => {
    const defaultVisibleIds = NAV_ITEMS.filter((item) => item.defaultVisible).map((item) => item.id);
    expect(new Set(defaultVisibleIds)).toEqual(DEFAULT_NAV_ITEM_IDS);
  });

  it("keeps full mode discoverable without changing the source list", () => {
    expect(filterNavItemsByProfile(NAV_ITEMS, "full")).toEqual(NAV_ITEMS);
  });

  it("keeps packs as the default extension entry and demotes skills/plugins to advanced surfaces", () => {
    const packs = NAV_ITEMS.find((item) => item.id === "nav-packs");
    const skills = NAV_ITEMS.find((item) => item.id === "nav-skills");
    const plugins = NAV_ITEMS.find((item) => item.id === "nav-plugins");

    expect(packs?.layer).toBe("pack");
    expect(packs?.defaultVisible).toBe(true);
    expect(skills?.layer).toBe("lab");
    expect(skills?.defaultVisible).toBeUndefined();
    expect(plugins?.layer).toBe("control-plane");
    expect(plugins?.defaultVisible).toBeUndefined();
  });
});
