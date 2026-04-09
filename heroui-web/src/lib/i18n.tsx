"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

export type Locale = "zh" | "en";

const translations: Record<Locale, Record<string, string>> = {
  zh: {
    default: "??",
    "auth.loading": "????????...",
    "auth.login": "??",
    "auth.loginTitle": "???? Agent",
    "auth.loginSubtitle": "?????????????",
    "auth.setupTitle": "???????",
    "auth.setupSubtitle": "???????????????????",
    "auth.password": "??",
    "auth.passwordPlaceholder": "?????",
    "auth.confirmPassword": "????",
    "auth.confirmPasswordPlaceholder": "??????",
    "auth.remember": "????",
    "auth.submit": "??",
    "auth.setupSubmit": "????",
    "auth.submitting": "???...",
    "auth.passwordMismatch": "???????????",
    "auth.passwordTooShort": "??????? 8 ??",
    "auth.networkError": "?????????????????",
    "setup.title": "???????",
    "setup.subtitle": "????????????????????",
    "setup.step.detect": "????",
    "setup.step.model": "????",
    "setup.step.template": "????",
    "setup.step.done": "??",
    "setup.next": "???",
    "setup.back": "???",
    "setup.refresh": "????",
    "setup.detect.empty": "??????????",
    "setup.detect.system": "????",
    "setup.detect.runtime": "???",
    "setup.detect.providers": "?????????",
    "setup.detect.components": "????",
    "setup.detect.firstRun": "??????????????????????????",
    "setup.model.title": "???????",
    "setup.model.subtitle": "?? OpenAI ??????????????????",
    "setup.model.baseUrl": "API Base URL",
    "setup.model.apiKey": "API Key",
    "setup.model.name": "????",
    "setup.model.test": "????",
    "setup.model.testing": "???...",
    "setup.template.title": "??????",
    "setup.template.subtitle": "??????????????????",
    "setup.done.title": "?????",
    "setup.done.subtitle": "???????? .env?????????????",
    "setup.done.login": "????",
    "setup.done.settings": "????",
    "setup.install": "??",
    "setup.installing": "???...",
    "setup.installed": "???",
    "setup.notInstalled": "???",
    "setup.manual": "?????",
    "setup.connected": "??",
    "setup.unavailable": "????",
    "browser.runtime": "?????",
    "browser.takeover": "????",
    "browser.connected": "???",
    "browser.disconnected": "???",
    "browser.resume": "????",
    "browser.handoff": "?????",
    "browser.open": "??????",
    "browser.return": "???????",
    "browser.latest": "????",
    "browser.updated": "???",
    "browser.elements": "???",
    "browser.screenshot": "???",
    "browser.next": "?????",
    "slash.title": "????",
    "slash.subtitle": "?? / ???????????????????",
    "slash.insert": "???",
    "slash.usage": "??",
    "slash.usageDesc": "??????????????????",
    "slash.available": "?????",
    "slash.close": "??",
    "connector.title": "???",
    "connector.subtitle": "??????????????????? Agent ?????",
    "connector.search": "?????",
    "connector.browser": "?????",
    "connector.browserConnected": "????????",
    "connector.browserDisconnected": "?????????",
    "connector.connect": "??",
    "connector.disconnect": "????",
    "connector.actions": "????",
    "connector.status.connected": "???",
    "connector.status.connecting": "???",
    "connector.status.error": "????",
    "connector.status.disconnected": "???",
    "persona.title": "????",
    "persona.save": "??",
    "persona.saving": "???...",
    "persona.preset": "??",
    "persona.custom": "???",
    "persona.identity": "????",
    "persona.identityPlaceholder": "?? Bot ?????????...",
    "persona.soul": "????",
    "persona.soulPlaceholder": "?? Bot ??????????????...",
    "persona.skills": "??",
    "persona.addSkill": "????",
    "persona.skillName": "????",
    "persona.skillDesc": "????",
    "persona.skillContent": "?????Markdown?",
    "persona.noSkills": "???????",
    "persona.noDesc": "????",
    "persona.create": "??",
    "persona.cancel": "??",
  },
  en: {
    default: "Default",
    "auth.loading": "Checking session...",
    "auth.login": "Sign in",
    "auth.loginTitle": "Sign in to Yunque Agent",
    "auth.loginSubtitle": "Enter the admin password to open the workspace.",
    "auth.setupTitle": "Create admin password",
    "auth.setupSubtitle": "Create an admin password before using the workspace.",
    "auth.password": "Password",
    "auth.passwordPlaceholder": "Enter password",
    "auth.confirmPassword": "Confirm password",
    "auth.confirmPasswordPlaceholder": "Enter password again",
    "auth.remember": "Keep me signed in",
    "auth.submit": "Continue",
    "auth.setupSubmit": "Save password",
    "auth.submitting": "Processing...",
    "auth.passwordMismatch": "The two passwords do not match.",
    "auth.passwordTooShort": "Password must be at least 8 characters.",
    "auth.networkError": "Connection failed. Please verify the service is running.",
    "setup.title": "Yunque setup",
    "setup.subtitle": "Finish environment detection and model configuration before entering the workspace.",
    "setup.step.detect": "Environment",
    "setup.step.model": "Model",
    "setup.step.template": "Template",
    "setup.step.done": "Done",
    "setup.next": "Next",
    "setup.back": "Back",
    "setup.refresh": "Refresh",
    "setup.detect.empty": "No environment info yet.",
    "setup.detect.system": "System",
    "setup.detect.runtime": "Runtime",
    "setup.detect.providers": "Detected providers",
    "setup.detect.components": "Optional components",
    "setup.detect.firstRun": "This instance still looks like a first run. The next step will generate config files automatically.",
    "setup.model.title": "Configure model provider",
    "setup.model.subtitle": "OpenAI-compatible endpoints, local models, and self-hosted services all work here.",
    "setup.model.baseUrl": "API Base URL",
    "setup.model.apiKey": "API Key",
    "setup.model.name": "Model name",
    "setup.model.test": "Test connection",
    "setup.model.testing": "Testing...",
    "setup.template.title": "Choose a starter template",
    "setup.template.subtitle": "Pick the closest scenario and generate a practical default config.",
    "setup.done.title": "Configuration saved",
    "setup.done.subtitle": "Your environment has been written to .env. You can return to login now.",
    "setup.done.login": "Go to login",
    "setup.done.settings": "Open settings",
    "setup.install": "Install",
    "setup.installing": "Installing...",
    "setup.installed": "Installed",
    "setup.notInstalled": "Not installed",
    "setup.manual": "Manual install",
    "setup.connected": "Available",
    "setup.unavailable": "Unavailable",
    "browser.runtime": "Browser runtime",
    "browser.takeover": "Manual takeover",
    "browser.connected": "Connected",
    "browser.disconnected": "Disconnected",
    "browser.resume": "Resume run",
    "browser.handoff": "Hand over to you",
    "browser.open": "Open browser page",
    "browser.return": "Return to tab",
    "browser.latest": "Latest output",
    "browser.updated": "Updated",
    "browser.elements": "elements",
    "browser.screenshot": "Screenshot attached",
    "browser.next": "Use next command",
    "slash.title": "Slash commands",
    "slash.subtitle": "Type / to insert browser actions, connector actions, or built-in skills.",
    "slash.insert": "Will insert",
    "slash.usage": "Usage",
    "slash.usageDesc": "This only inserts a command. It does not rewrite your text into a prompt.",
    "slash.available": "commands available",
    "slash.close": "Close",
    "connector.title": "Connectors",
    "connector.subtitle": "Connect your browser, repos, email, and collaboration tools to the agent workflow.",
    "connector.search": "Search connectors",
    "connector.browser": "My browser",
    "connector.browserConnected": "Extension connected",
    "connector.browserDisconnected": "Extension not connected",
    "connector.connect": "Connect",
    "connector.disconnect": "Disconnect",
    "connector.actions": "Available actions",
    "connector.status.connected": "Connected",
    "connector.status.connecting": "Connecting",
    "connector.status.error": "Needs attention",
    "connector.status.disconnected": "Disconnected",
    "persona.title": "Persona",
    "persona.save": "Save",
    "persona.saving": "Saving...",
    "persona.preset": "Preset",
    "persona.custom": "Custom",
    "persona.identity": "Identity",
    "persona.identityPlaceholder": "Define the bot's identity, personality, and position...",
    "persona.soul": "Soul",
    "persona.soulPlaceholder": "Define the bot's values, communication style, and behavior rules...",
    "persona.skills": "Skills",
    "persona.addSkill": "Add skill",
    "persona.skillName": "Skill name",
    "persona.skillDesc": "Skill description",
    "persona.skillContent": "Skill content (Markdown)",
    "persona.noSkills": "No skills configured yet",
    "persona.noDesc": "No description",
    "persona.create": "Create",
    "persona.cancel": "Cancel",
  },
};

interface I18nContextType {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: string) => string;
}

const I18nContext = createContext<I18nContextType>({
  locale: "zh",
  setLocale: () => {},
  t: (key) => key,
});

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>("zh");

  useEffect(() => {
    const saved = typeof window !== "undefined" ? (localStorage.getItem("yunque_locale") as Locale | null) : null;
    if (saved === "zh" || saved === "en") {
      setLocaleState(saved);
      return;
    }
    if (typeof navigator !== "undefined") {
      setLocaleState(navigator.language.toLowerCase().startsWith("zh") ? "zh" : "en");
    }
  }, []);

  const setLocale = useCallback((nextLocale: Locale) => {
    setLocaleState(nextLocale);
    if (typeof window !== "undefined") {
      localStorage.setItem("yunque_locale", nextLocale);
      document.documentElement.lang = nextLocale;
    }
  }, []);

  useEffect(() => {
    if (typeof document !== "undefined") {
      document.documentElement.lang = locale;
    }
  }, [locale]);

  const t = useMemo(() => (key: string) => translations[locale]?.[key] || translations.en[key] || key, [locale]);

  return <I18nContext.Provider value={{ locale, setLocale, t }}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  return useContext(I18nContext);
}
