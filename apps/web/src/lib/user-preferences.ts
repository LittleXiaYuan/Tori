/**
 * User Preferences Storage System
 *
 * Provides a unified interface for storing and retrieving user preferences,
 * similar to OpenClaw's personalization features.
 *
 * Features:
 * - Interface preferences (theme, layout, navigation state)
 * - Workflow preferences (recent items, favorites, shortcuts)
 * - Behavioral preferences (auto-save, notifications)
 * - Cross-device sync support (optional backend integration)
 */

import { ProfileMode } from "./profile-mode";

// ============================================================================
// Type Definitions
// ============================================================================

export interface NavigationPreferences {
  sidebarCollapsed: boolean;
  expandedGroups: string[];
  pinnedItems: string[];
  recentPages: RecentPage[];
}

export interface RecentPage {
  path: string;
  label: string;
  timestamp: number;
}

export interface WorkflowPreferences {
  recentProjects: RecentItem[];
  recentTasks: RecentItem[];
  recentSkills: RecentItem[];
  favoriteSkills: string[];
  favoriteWorkflows: string[];
  quickActions: QuickAction[];
}

export interface RecentItem {
  id: string;
  name: string;
  type: string;
  timestamp: number;
  metadata?: Record<string, unknown>;
}

export interface QuickAction {
  id: string;
  label: string;
  action: string;
  icon?: string;
  order: number;
}

export interface InterfacePreferences {
  profileMode: ProfileMode;
  chatDensity: "cozy" | "compact";
  fontSize: "small" | "default" | "large";
  showOnboarding: boolean;
  zenModeEnabled: boolean;
  floatingWidgetEnabled: boolean;
  commandPaletteHistory: string[];
}

export interface BehavioralPreferences {
  autoSaveEnabled: boolean;
  notificationsEnabled: boolean;
  soundEnabled: boolean;
  analyticsEnabled: boolean;
  telemetryEnabled: boolean;
}

export interface UserPreferences {
  version: number;
  lastUpdated: number;
  navigation: NavigationPreferences;
  workflow: WorkflowPreferences;
  interface: InterfacePreferences;
  behavioral: BehavioralPreferences;
}

// ============================================================================
// Default Values
// ============================================================================

export const DEFAULT_PREFERENCES: UserPreferences = {
  version: 1,
  lastUpdated: Date.now(),
  navigation: {
    sidebarCollapsed: false,
    expandedGroups: ["work"],
    pinnedItems: [],
    recentPages: [],
  },
  workflow: {
    recentProjects: [],
    recentTasks: [],
    recentSkills: [],
    favoriteSkills: [],
    favoriteWorkflows: [],
    quickActions: [],
  },
  interface: {
    profileMode: "easy",
    chatDensity: "cozy",
    fontSize: "default",
    showOnboarding: true,
    zenModeEnabled: false,
    floatingWidgetEnabled: true,
    commandPaletteHistory: [],
  },
  behavioral: {
    autoSaveEnabled: true,
    notificationsEnabled: true,
    soundEnabled: false,
    analyticsEnabled: true,
    telemetryEnabled: false,
  },
};

// ============================================================================
// Storage Keys
// ============================================================================

const STORAGE_KEY = "yunque_user_preferences";
const STORAGE_VERSION = 1;

// ============================================================================
// Core Functions
// ============================================================================

/**
 * Load user preferences from localStorage
 */
export function loadPreferences(): UserPreferences {
  if (typeof window === "undefined") {
    return DEFAULT_PREFERENCES;
  }

  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (!stored) {
      return DEFAULT_PREFERENCES;
    }

    const parsed = JSON.parse(stored) as UserPreferences;

    // Version migration
    if (parsed.version !== STORAGE_VERSION) {
      return migratePreferences(parsed);
    }

    // Merge with defaults to handle new fields
    return deepMerge(DEFAULT_PREFERENCES, parsed);
  } catch (error) {
    console.error("[Preferences] Failed to load preferences:", error);
    return DEFAULT_PREFERENCES;
  }
}

/**
 * Save user preferences to localStorage
 */
export function savePreferences(preferences: UserPreferences): void {
  if (typeof window === "undefined") {
    return;
  }

  try {
    const toSave: UserPreferences = {
      ...preferences,
      version: STORAGE_VERSION,
      lastUpdated: Date.now(),
    };

    localStorage.setItem(STORAGE_KEY, JSON.stringify(toSave));

    // Dispatch event for cross-tab sync
    window.dispatchEvent(
      new StorageEvent("storage", {
        key: STORAGE_KEY,
        newValue: JSON.stringify(toSave),
      })
    );
  } catch (error) {
    console.error("[Preferences] Failed to save preferences:", error);
  }
}

/**
 * Update a specific section of preferences
 */
export function updatePreferences<K extends keyof UserPreferences>(
  section: K,
  updates: Partial<UserPreferences[K]>
): UserPreferences {
  const current = loadPreferences();
  const updated: UserPreferences = {
    ...current,
    [section]: {
      ...(current[section] as unknown as Record<string, unknown>),
      ...(updates as unknown as Record<string, unknown>),
    },
  };
  savePreferences(updated);
  return updated;
}

/**
 * Reset preferences to defaults
 */
export function resetPreferences(): UserPreferences {
  savePreferences(DEFAULT_PREFERENCES);
  return DEFAULT_PREFERENCES;
}

/**
 * Export preferences as JSON
 */
export function exportPreferences(): string {
  const prefs = loadPreferences();
  return JSON.stringify(prefs, null, 2);
}

/**
 * Import preferences from JSON
 */
export function importPreferences(json: string): UserPreferences {
  try {
    const parsed = JSON.parse(json) as UserPreferences;
    const migrated = migratePreferences(parsed);
    savePreferences(migrated);
    return migrated;
  } catch (error) {
    console.error("[Preferences] Failed to import preferences:", error);
    throw new Error("Invalid preferences JSON");
  }
}

// ============================================================================
// Navigation Helpers
// ============================================================================

export function addRecentPage(path: string, label: string): void {
  const prefs = loadPreferences();
  const recent = prefs.navigation.recentPages.filter((p) => p.path !== path);
  recent.unshift({ path, label, timestamp: Date.now() });

  // Keep only last 20 pages
  if (recent.length > 20) {
    recent.splice(20);
  }

  updatePreferences("navigation", { recentPages: recent });
}

export function toggleSidebarCollapsed(): boolean {
  const prefs = loadPreferences();
  const collapsed = !prefs.navigation.sidebarCollapsed;
  updatePreferences("navigation", { sidebarCollapsed: collapsed });
  return collapsed;
}

export function toggleGroupExpanded(groupId: string): string[] {
  const prefs = loadPreferences();
  const expanded = [...prefs.navigation.expandedGroups];
  const index = expanded.indexOf(groupId);

  if (index >= 0) {
    expanded.splice(index, 1);
  } else {
    expanded.push(groupId);
  }

  updatePreferences("navigation", { expandedGroups: expanded });
  return expanded;
}

export function pinItem(itemId: string): void {
  const prefs = loadPreferences();
  if (!prefs.navigation.pinnedItems.includes(itemId)) {
    const pinned = [...prefs.navigation.pinnedItems, itemId];
    updatePreferences("navigation", { pinnedItems: pinned });
  }
}

export function unpinItem(itemId: string): void {
  const prefs = loadPreferences();
  const pinned = prefs.navigation.pinnedItems.filter((id) => id !== itemId);
  updatePreferences("navigation", { pinnedItems: pinned });
}

// ============================================================================
// Workflow Helpers
// ============================================================================

export function addRecentItem(
  type: "projects" | "tasks" | "skills",
  item: Omit<RecentItem, "timestamp">
): void {
  const prefs = loadPreferences();
  const key = `recent${type.charAt(0).toUpperCase() + type.slice(1)}` as keyof WorkflowPreferences;
  const recent = (prefs.workflow[key] as RecentItem[]).filter((i) => i.id !== item.id);

  recent.unshift({ ...item, timestamp: Date.now() });

  // Keep only last 10 items
  if (recent.length > 10) {
    recent.splice(10);
  }

  updatePreferences("workflow", { [key]: recent });
}

export function toggleFavorite(type: "skills" | "workflows", id: string): boolean {
  const prefs = loadPreferences();
  const key = `favorite${type.charAt(0).toUpperCase() + type.slice(1)}` as keyof WorkflowPreferences;
  const favorites = [...(prefs.workflow[key] as string[])];
  const index = favorites.indexOf(id);

  if (index >= 0) {
    favorites.splice(index, 1);
    updatePreferences("workflow", { [key]: favorites });
    return false;
  } else {
    favorites.push(id);
    updatePreferences("workflow", { [key]: favorites });
    return true;
  }
}

export function addQuickAction(action: QuickAction): void {
  const prefs = loadPreferences();
  const actions = prefs.workflow.quickActions.filter((a) => a.id !== action.id);
  actions.push(action);
  actions.sort((a, b) => a.order - b.order);
  updatePreferences("workflow", { quickActions: actions });
}

export function removeQuickAction(actionId: string): void {
  const prefs = loadPreferences();
  const actions = prefs.workflow.quickActions.filter((a) => a.id !== actionId);
  updatePreferences("workflow", { quickActions: actions });
}

// ============================================================================
// Interface Helpers
// ============================================================================

export function addCommandToHistory(command: string): void {
  const prefs = loadPreferences();
  const history = prefs.interface.commandPaletteHistory.filter((c) => c !== command);
  history.unshift(command);

  // Keep only last 50 commands
  if (history.length > 50) {
    history.splice(50);
  }

  updatePreferences("interface", { commandPaletteHistory: history });
}

export function clearCommandHistory(): void {
  updatePreferences("interface", { commandPaletteHistory: [] });
}

// ============================================================================
// Utility Functions
// ============================================================================

// Constrain to `object` (not `Record<string, unknown>`) so that `interface`
// types like UserPreferences — which lack an implicit index signature — still
// satisfy the bound. The body operates on a Record view internally.
function deepMerge<T extends object>(target: T, source: Partial<T>): T {
  const result: Record<string, unknown> = { ...(target as Record<string, unknown>) };
  const src = source as Record<string, unknown>;

  for (const key in src) {
    if (Object.prototype.hasOwnProperty.call(src, key)) {
      const sourceValue = src[key];
      const targetValue = result[key];

      if (
        sourceValue &&
        typeof sourceValue === "object" &&
        !Array.isArray(sourceValue) &&
        targetValue &&
        typeof targetValue === "object" &&
        !Array.isArray(targetValue)
      ) {
        result[key] = deepMerge(
          targetValue as Record<string, unknown>,
          sourceValue as Record<string, unknown>
        );
      } else if (sourceValue !== undefined) {
        result[key] = sourceValue;
      }
    }
  }

  return result as T;
}

function migratePreferences(old: UserPreferences): UserPreferences {
  // Future version migrations will go here
  // For now, just merge with defaults
  return deepMerge(DEFAULT_PREFERENCES, old);
}

// ============================================================================
// React Hook (optional)
// ============================================================================

export function usePreferences() {
  if (typeof window === "undefined") {
    return {
      preferences: DEFAULT_PREFERENCES,
      updatePreferences,
      resetPreferences,
    };
  }

  return {
    preferences: loadPreferences(),
    updatePreferences,
    resetPreferences,
  };
}
