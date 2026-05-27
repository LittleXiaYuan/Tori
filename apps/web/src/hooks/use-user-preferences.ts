"use client";

import { useState, useEffect, useCallback } from "react";
import {
  loadPreferences,
  savePreferences,
  updatePreferences as updatePreferencesCore,
  resetPreferences as resetPreferencesCore,
  addRecentPage,
  addRecentItem,
  toggleFavorite,
  addQuickAction,
  removeQuickAction,
  toggleSidebarCollapsed,
  toggleGroupExpanded,
  pinItem,
  unpinItem,
  addCommandToHistory,
  clearCommandHistory,
  type UserPreferences,
  type RecentItem,
  type QuickAction,
} from "@/lib/user-preferences";

const STORAGE_KEY = "yunque_user_preferences";

/**
 * React hook for managing user preferences with automatic sync
 */
export function useUserPreferences() {
  const [preferences, setPreferences] = useState<UserPreferences>(() => loadPreferences());

  // Listen for storage changes (cross-tab sync)
  useEffect(() => {
    const handleStorageChange = (e: StorageEvent) => {
      if (e.key === STORAGE_KEY && e.newValue) {
        try {
          const updated = JSON.parse(e.newValue) as UserPreferences;
          setPreferences(updated);
        } catch (error) {
          console.error("[useUserPreferences] Failed to parse storage event:", error);
        }
      }
    };

    window.addEventListener("storage", handleStorageChange);
    return () => window.removeEventListener("storage", handleStorageChange);
  }, []);

  // Update preferences and local state
  const updatePreferences = useCallback(
    <K extends keyof UserPreferences>(section: K, updates: Partial<UserPreferences[K]>) => {
      const updated = updatePreferencesCore(section, updates);
      setPreferences(updated);
      return updated;
    },
    []
  );

  // Reset preferences
  const resetPreferences = useCallback(() => {
    const reset = resetPreferencesCore();
    setPreferences(reset);
    return reset;
  }, []);

  // Navigation helpers
  const navigation = {
    addRecentPage: useCallback((path: string, label: string) => {
      addRecentPage(path, label);
      setPreferences(loadPreferences());
    }, []),

    toggleSidebarCollapsed: useCallback(() => {
      const collapsed = toggleSidebarCollapsed();
      setPreferences(loadPreferences());
      return collapsed;
    }, []),

    toggleGroupExpanded: useCallback((groupId: string) => {
      const expanded = toggleGroupExpanded(groupId);
      setPreferences(loadPreferences());
      return expanded;
    }, []),

    pinItem: useCallback((itemId: string) => {
      pinItem(itemId);
      setPreferences(loadPreferences());
    }, []),

    unpinItem: useCallback((itemId: string) => {
      unpinItem(itemId);
      setPreferences(loadPreferences());
    }, []),
  };

  // Workflow helpers
  const workflow = {
    addRecentItem: useCallback((type: "projects" | "tasks" | "skills", item: Omit<RecentItem, "timestamp">) => {
      addRecentItem(type, item);
      setPreferences(loadPreferences());
    }, []),

    toggleFavorite: useCallback((type: "skills" | "workflows", id: string) => {
      const isFavorite = toggleFavorite(type, id);
      setPreferences(loadPreferences());
      return isFavorite;
    }, []),

    addQuickAction: useCallback((action: QuickAction) => {
      addQuickAction(action);
      setPreferences(loadPreferences());
    }, []),

    removeQuickAction: useCallback((actionId: string) => {
      removeQuickAction(actionId);
      setPreferences(loadPreferences());
    }, []),
  };

  // Interface helpers
  const interface_ = {
    addCommandToHistory: useCallback((command: string) => {
      addCommandToHistory(command);
      setPreferences(loadPreferences());
    }, []),

    clearCommandHistory: useCallback(() => {
      clearCommandHistory();
      setPreferences(loadPreferences());
    }, []),
  };

  return {
    preferences,
    updatePreferences,
    resetPreferences,
    navigation,
    workflow,
    interface: interface_,
  };
}

/**
 * Hook for a specific preference section
 */
export function usePreferenceSection<K extends keyof UserPreferences>(section: K) {
  const { preferences, updatePreferences } = useUserPreferences();

  const updateSection = useCallback(
    (updates: Partial<UserPreferences[K]>) => {
      return updatePreferences(section, updates);
    },
    [section, updatePreferences]
  );

  return {
    preferences: preferences[section],
    updatePreferences: updateSection,
  };
}

/**
 * Hook for navigation preferences
 */
export function useNavigationPreferences() {
  const { preferences, navigation } = useUserPreferences();
  return {
    ...preferences.navigation,
    ...navigation,
  };
}

/**
 * Hook for workflow preferences
 */
export function useWorkflowPreferences() {
  const { preferences, workflow } = useUserPreferences();
  return {
    ...preferences.workflow,
    ...workflow,
  };
}

/**
 * Hook for interface preferences
 */
export function useInterfacePreferences() {
  const { preferences, updatePreferences, interface: interface_ } = useUserPreferences();
  return {
    ...preferences.interface,
    ...interface_,
    updateInterface: (updates: Partial<UserPreferences["interface"]>) =>
      updatePreferences("interface", updates),
  };
}

/**
 * Hook for behavioral preferences
 */
export function useBehavioralPreferences() {
  const { preferences, updatePreferences } = useUserPreferences();
  return {
    ...preferences.behavioral,
    updateBehavioral: (updates: Partial<UserPreferences["behavioral"]>) =>
      updatePreferences("behavioral", updates),
  };
}
