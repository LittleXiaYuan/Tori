import { describe, it, expect, beforeEach, afterEach } from "@jest/globals";
import {
  loadPreferences,
  savePreferences,
  updatePreferences,
  resetPreferences,
  addRecentPage,
  toggleSidebarCollapsed,
  toggleGroupExpanded,
  addRecentItem,
  toggleFavorite,
  addQuickAction,
  removeQuickAction,
  DEFAULT_PREFERENCES,
  type UserPreferences,
} from "../user-preferences";

describe("User Preferences", () => {
  const STORAGE_KEY = "yunque_user_preferences";

  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    localStorage.clear();
  });

  describe("loadPreferences", () => {
    it("should return default preferences when nothing is stored", () => {
      const prefs = loadPreferences();
      expect(prefs).toEqual(DEFAULT_PREFERENCES);
    });

    it("should load stored preferences", () => {
      const custom: UserPreferences = {
        ...DEFAULT_PREFERENCES,
        interface: {
          ...DEFAULT_PREFERENCES.interface,
          profileMode: "full",
        },
      };
      localStorage.setItem(STORAGE_KEY, JSON.stringify(custom));

      const prefs = loadPreferences();
      expect(prefs.interface.profileMode).toBe("full");
    });

    it("should merge with defaults for missing fields", () => {
      const partial = {
        version: 1,
        lastUpdated: Date.now(),
        navigation: {
          sidebarCollapsed: true,
          expandedGroups: ["work"],
          pinnedItems: [],
          recentPages: [],
        },
      };
      localStorage.setItem(STORAGE_KEY, JSON.stringify(partial));

      const prefs = loadPreferences();
      expect(prefs.navigation.sidebarCollapsed).toBe(true);
      expect(prefs.workflow).toBeDefined();
      expect(prefs.interface).toBeDefined();
      expect(prefs.behavioral).toBeDefined();
    });
  });

  describe("savePreferences", () => {
    it("should save preferences to localStorage", () => {
      const prefs = { ...DEFAULT_PREFERENCES };
      savePreferences(prefs);

      const stored = localStorage.getItem(STORAGE_KEY);
      expect(stored).toBeTruthy();
      const parsed = JSON.parse(stored!);
      expect(parsed.version).toBe(1);
    });

    it("should update lastUpdated timestamp", () => {
      const before = Date.now();
      savePreferences(DEFAULT_PREFERENCES);
      const after = Date.now();

      const stored = JSON.parse(localStorage.getItem(STORAGE_KEY)!);
      expect(stored.lastUpdated).toBeGreaterThanOrEqual(before);
      expect(stored.lastUpdated).toBeLessThanOrEqual(after);
    });
  });

  describe("updatePreferences", () => {
    it("should update a specific section", () => {
      const updated = updatePreferences("interface", { profileMode: "full" });
      expect(updated.interface.profileMode).toBe("full");

      const stored = loadPreferences();
      expect(stored.interface.profileMode).toBe("full");
    });

    it("should preserve other sections", () => {
      updatePreferences("navigation", { sidebarCollapsed: true });
      const prefs = loadPreferences();

      expect(prefs.navigation.sidebarCollapsed).toBe(true);
      expect(prefs.interface).toEqual(DEFAULT_PREFERENCES.interface);
      expect(prefs.workflow).toEqual(DEFAULT_PREFERENCES.workflow);
    });
  });

  describe("resetPreferences", () => {
    it("should reset to defaults", () => {
      updatePreferences("interface", { profileMode: "full" });
      updatePreferences("navigation", { sidebarCollapsed: true });

      const reset = resetPreferences();
      expect(reset).toEqual(DEFAULT_PREFERENCES);

      const stored = loadPreferences();
      expect(stored.interface.profileMode).toBe("easy");
      expect(stored.navigation.sidebarCollapsed).toBe(false);
    });
  });

  describe("Navigation helpers", () => {
    it("should add recent pages", () => {
      addRecentPage("/chat", "对话");
      addRecentPage("/settings", "设置");

      const prefs = loadPreferences();
      expect(prefs.navigation.recentPages).toHaveLength(2);
      expect(prefs.navigation.recentPages[0].path).toBe("/settings");
      expect(prefs.navigation.recentPages[1].path).toBe("/chat");
    });

    it("should limit recent pages to 20", () => {
      for (let i = 0; i < 25; i++) {
        addRecentPage(`/page${i}`, `Page ${i}`);
      }

      const prefs = loadPreferences();
      expect(prefs.navigation.recentPages).toHaveLength(20);
    });

    it("should toggle sidebar collapsed", () => {
      const collapsed1 = toggleSidebarCollapsed();
      expect(collapsed1).toBe(true);

      const collapsed2 = toggleSidebarCollapsed();
      expect(collapsed2).toBe(false);
    });

    it("should toggle group expanded", () => {
      const expanded1 = toggleGroupExpanded("work");
      expect(expanded1).toContain("work");

      const expanded2 = toggleGroupExpanded("work");
      expect(expanded2).not.toContain("work");
    });
  });

  describe("Workflow helpers", () => {
    it("should add recent items", () => {
      addRecentItem("projects", {
        id: "proj1",
        name: "Project 1",
        type: "project",
      });

      const prefs = loadPreferences();
      expect(prefs.workflow.recentProjects).toHaveLength(1);
      expect(prefs.workflow.recentProjects[0].id).toBe("proj1");
    });

    it("should limit recent items to 10", () => {
      for (let i = 0; i < 15; i++) {
        addRecentItem("tasks", {
          id: `task${i}`,
          name: `Task ${i}`,
          type: "task",
        });
      }

      const prefs = loadPreferences();
      expect(prefs.workflow.recentTasks).toHaveLength(10);
    });

    it("should toggle favorites", () => {
      const isFav1 = toggleFavorite("skills", "skill1");
      expect(isFav1).toBe(true);

      const prefs1 = loadPreferences();
      expect(prefs1.workflow.favoriteSkills).toContain("skill1");

      const isFav2 = toggleFavorite("skills", "skill1");
      expect(isFav2).toBe(false);

      const prefs2 = loadPreferences();
      expect(prefs2.workflow.favoriteSkills).not.toContain("skill1");
    });

    it("should add and remove quick actions", () => {
      addQuickAction({
        id: "action1",
        label: "Action 1",
        action: "do-something",
        order: 1,
      });

      const prefs1 = loadPreferences();
      expect(prefs1.workflow.quickActions).toHaveLength(1);

      removeQuickAction("action1");

      const prefs2 = loadPreferences();
      expect(prefs2.workflow.quickActions).toHaveLength(0);
    });
  });
});
