// Yunque Browser Connector — Background Service Worker
// Architecture: Local WebSocket ↔ CDP (Chrome DevTools Protocol)

const CDP_VERSION = "1.3";
const RECONNECT_DELAY = 3000;
const SESSION_TIMEOUT = 60000;
const DEFAULT_WS_URL = "ws://localhost:9090/ws/browser";
const RUNTIME_STATE_KEY = "yunque_runtime_state";
const MAX_OUTBOUND_QUEUE = 100;

// ─── State ───────────────────────────────────────────
let ws = null;
let wsUrl = DEFAULT_WS_URL;
let reconnectTimer = null;
let outboundQueue = [];
let lastBroadcastSignature = "";
let sessions = new Map();   // tabId → { target, lastUsed }
let connected = false;
let runtimeSession = {
  id: null,
  status: "idle",
  currentTabId: null,
  currentUrl: "",
  title: "",
  lastAction: "",
  takeover: false,
  updatedAt: 0,
};

function buildWebSocketURL(baseUrl, apiKey) {
  try {
    const url = new URL(baseUrl || DEFAULT_WS_URL);
    if (apiKey && !url.searchParams.get("key") && !url.searchParams.get("api_key")) {
      url.searchParams.set("key", apiKey);
    }
    return url.toString();
  } catch (_) {
    return baseUrl || DEFAULT_WS_URL;
  }
}

// ─── WebSocket Connection ────────────────────────────
function connect() {
  if (ws && ws.readyState <= 1) return;

  chrome.storage.local.get(["yunque_ws_url", "yunque_api_key"], (r) => {
    wsUrl = r.yunque_ws_url || DEFAULT_WS_URL;
    const socketUrl = buildWebSocketURL(wsUrl, r.yunque_api_key || "");
    log("info", `Connecting to ${socketUrl.replace(/([?&](?:key|api_key)=)[^&]+/, "$1***")}`);

    let socket;
    try {
      socket = new WebSocket(socketUrl);
      ws = socket;
    } catch (e) {
      log("error", `WebSocket create failed: ${e.message}`);
      scheduleReconnect();
      return;
    }

    socket.onopen = () => {
      if (ws !== socket) return;
      connected = true;
      log("info", "WebSocket connected");
      clearTimeout(reconnectTimer);
      sendToBackend({ type: "hello", version: chrome.runtime.getManifest().version }, { queueIfOffline: false });
      flushOutboundQueue();
      restoreBadgeFromState();
      broadcastRuntimeState({ force: true });
    };

    socket.onmessage = async (evt) => {
      if (ws !== socket) return;
      try {
        const msg = JSON.parse(evt.data);
        await handleCommand(msg);
      } catch (e) {
        log("error", `Message handling error: ${e.message}`);
        sendToBackend({ type: "error", error: e.message, requestId: null });
      }
    };

    socket.onclose = () => {
      if (ws !== socket) return;
      connected = false;
      ws = null;
      log("warn", "WebSocket closed");
      restoreBadgeFromState();
      broadcastRuntimeState({ force: true });
      scheduleReconnect();
    };

    socket.onerror = () => {
      if (ws !== socket) return;
      log("error", "WebSocket error");
      updateBadge("ERR", "#F44336");
      persistRuntimeState();
      broadcastRuntimeState({ force: true });
    };
  });
}

function scheduleReconnect() {
  clearTimeout(reconnectTimer);
  reconnectTimer = setTimeout(connect, RECONNECT_DELAY);
}

function sendToBackend(msg, options = {}) {
  const { queueIfOffline = true, fromQueue = false } = options;
  if (ws && ws.readyState === WebSocket.OPEN) {
    try {
      ws.send(JSON.stringify(msg));
      return true;
    } catch (e) {
      log("warn", `Send failed: ${e.message}`);
    }
  }
  if (queueIfOffline && !fromQueue) {
    enqueueOutboundMessage(msg);
  }
  return false;
}

function nextSessionId() {
  return `browser-${Date.now()}`;
}

function getBadgeState() {
  if (takeover.active) return { text: "USR", color: "#f59e0b" };
  if (connected) return { text: "ON", color: "#4CAF50" };
  return { text: "OFF", color: "#999" };
}

function persistRuntimeState() {
  chrome.storage.local.set({
    [RUNTIME_STATE_KEY]: {
      runtimeSession,
      takeover,
      connected,
      wsUrl,
      savedAt: Date.now(),
    },
  }, () => void chrome.runtime.lastError);
}

function runtimeStateSignature() {
  return JSON.stringify(getRuntimeState());
}

function restoreBadgeFromState() {
  const badge = getBadgeState();
  updateBadge(badge.text, badge.color);
}

function restoreRuntimeState() {
  return new Promise((resolve) => {
    chrome.storage.local.get([RUNTIME_STATE_KEY], async (res) => {
      const saved = res?.[RUNTIME_STATE_KEY];
      if (saved?.runtimeSession) {
        runtimeSession = {
          ...runtimeSession,
          ...saved.runtimeSession,
        };
      }
      if (saved?.takeover) {
        takeover = {
          ...takeover,
          ...saved.takeover,
        };
      }
      if (saved?.wsUrl) wsUrl = saved.wsUrl;

      if (runtimeSession.currentTabId) {
        const snapshot = await getTabSnapshot(runtimeSession.currentTabId);
        if (snapshot) {
          runtimeSession.currentUrl = snapshot.url || runtimeSession.currentUrl;
          runtimeSession.title = snapshot.title || runtimeSession.title;
        } else {
          runtimeSession.currentTabId = null;
          runtimeSession.currentUrl = "";
          runtimeSession.title = "";
          runtimeSession.status = takeover.active ? "takeover" : "idle";
        }
      }

      restoreBadgeFromState();
      broadcastRuntimeState({ force: true });
      resolve();
    });
  });
}

async function getTabSnapshot(tabId) {
  try {
    if (!tabId) {
      const [activeTab] = await chrome.tabs.query({ active: true, currentWindow: true });
      if (!activeTab) return null;
      tabId = activeTab.id;
    }
    const tab = await chrome.tabs.get(tabId);
    return {
      tabId: tab.id,
      url: tab.url || "",
      title: tab.title || "",
    };
  } catch (_) {
    return null;
  }
}

async function updateRuntimeSession(patch = {}, options = {}) {
  const { forceBroadcast = false } = options;
  if (!runtimeSession.id) runtimeSession.id = nextSessionId();
  const tabInfo = await getTabSnapshot(patch.currentTabId || runtimeSession.currentTabId);
  const nextSession = {
    ...runtimeSession,
    ...patch,
    currentTabId: patch.currentTabId ?? (tabInfo?.tabId ?? runtimeSession.currentTabId),
    currentUrl: patch.currentUrl ?? (tabInfo?.url ?? runtimeSession.currentUrl),
    title: patch.title ?? (tabInfo?.title ?? runtimeSession.title),
  };
  const changed = JSON.stringify(runtimeSession) !== JSON.stringify(nextSession);
  runtimeSession = {
    ...nextSession,
    updatedAt: changed ? Date.now() : runtimeSession.updatedAt,
  };
  persistRuntimeState();
  broadcastRuntimeState({ force: forceBroadcast });
  return runtimeSession;
}

function getRuntimeState() {
  return {
    connected,
    wsUrl,
    sessions: sessions.size,
    takeover: takeover.active,
    runtimeSession: {
      ...runtimeSession,
      takeover: takeover.active,
    },
  };
}

function broadcastRuntimeState(options = {}) {
  const { force = false } = options;
  const state = getRuntimeState();
  const signature = JSON.stringify(state);
  if (!force && signature === lastBroadcastSignature) return;
  lastBroadcastSignature = signature;
  const payload = { type: "bridge_state_update", state };
  chrome.tabs.query({}, (tabs) => {
    for (const tab of tabs) {
      if (!tab.id) continue;
      chrome.tabs.sendMessage(tab.id, payload, () => void chrome.runtime.lastError);
    }
  });
}

// ─── Takeover State ─────────────────────────────────
let takeover = { active: false, reason: "" };

// ─── Command Dispatcher ──────────────────────────────
async function handleCommand(msg) {
  const { requestId, action } = msg;
  if (!action) return;

  await updateRuntimeSession({
    status: action.type === "session_status" ? runtimeSession.status : "running",
    lastAction: action.type,
    currentTabId: action.tabId || runtimeSession.currentTabId,
  });

  // Block commands during user takeover (except session_status to resume)
  if (takeover.active && action.type !== "session_status") {
    sendToBackend({ type: "action_result", requestId, ok: false, error: "user takeover active — AI paused" });
    return;
  }

  try {
    let result;
    switch (action.type) {
      case "browser_navigate":
        result = await doNavigate(action);
        break;
      case "browser_click":
        result = await doClick(action);
        break;
      case "browser_input":
        result = await doInput(action);
        break;
      case "browser_scroll":
        result = await doScroll(action);
        break;
      case "browser_press_key":
        result = await doPressKey(action);
        break;
      case "browser_screenshot":
      case "browser_view":
        result = await doScreenshot(action);
        break;
      case "browser_get_content":
        result = await doGetContent(action);
        break;
      case "browser_get_structured_content":
        result = await doGetStructuredContent(action);
        break;
      case "browser_move_mouse":
        result = await doMoveMouse(action);
        break;
      case "browser_mark_elements":
        result = await doMarkElements(action);
        break;
      case "browser_unmark_elements":
        result = await doUnmarkElements(action);
        break;
      case "browser_get_elements":
        result = await doGetElements(action);
        break;
      case "browser_list_tabs":
        result = await doListTabs(action);
        break;
      case "browser_switch_tab":
        result = await doSwitchTab(action);
        break;
      case "browser_new_tab":
        result = await doNewTab(action);
        break;
      case "browser_close_tab":
        result = await doCloseTab(action);
        break;
      case "session_status":
        result = await doSessionStatus(action);
        break;
      default:
        result = { ok: false, error: `Unknown action: ${action.type}` };
    }
    await updateRuntimeSession({
      status: result && result.ok === false ? "error" : (takeover.active ? "takeover" : "running"),
      currentUrl: result?.url || runtimeSession.currentUrl,
      title: result?.title || runtimeSession.title,
    });
    sendToBackend({ type: "action_result", requestId, ...result });
  } catch (e) {
    await updateRuntimeSession({ status: "error" });
    sendToBackend({ type: "action_result", requestId, ok: false, error: e.message });
  }
}

// ─── CDP Session Management ──────────────────────────
async function getOrCreateSession(tabId) {
  if (!tabId) {
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
    if (!tab) throw new Error("No active tab");
    tabId = tab.id;
  }

  let session = sessions.get(tabId);
  if (session) {
    session.lastUsed = Date.now();
    return { tabId, session };
  }

  const target = { tabId };
  await new Promise((resolve, reject) => {
    chrome.debugger.attach(target, CDP_VERSION, () => {
      if (chrome.runtime.lastError) {
        reject(new Error(chrome.runtime.lastError.message));
        return;
      }
      resolve();
    });
  });

  await cdpSend(target, "Page.enable");

  session = { target, lastUsed: Date.now() };
  sessions.set(tabId, session);
  log("info", `CDP session attached to tab ${tabId}`);
  return { tabId, session };
}

async function cdpSend(target, method, params) {
  return new Promise((resolve, reject) => {
    chrome.debugger.sendCommand(target, method, params || {}, (result) => {
      if (chrome.runtime.lastError) {
        reject(new Error(chrome.runtime.lastError.message));
        return;
      }
      resolve(result);
    });
  });
}

async function detachSession(tabId) {
  const session = sessions.get(tabId);
  if (!session) return;
  try {
    await new Promise((resolve) => {
      chrome.debugger.detach(session.target, () => {
        if (chrome.runtime.lastError) log("warn", `Detach warning: ${chrome.runtime.lastError.message}`);
        resolve();
      });
    });
  } catch (_) {}
  sessions.delete(tabId);
  log("info", `CDP session detached from tab ${tabId}`);
}

// Cleanup stale sessions
setInterval(() => {
  const now = Date.now();
  for (const [tabId, session] of sessions) {
    if (now - session.lastUsed > SESSION_TIMEOUT) {
      detachSession(tabId);
    }
  }
}, 30000);

// Handle debugger detach events
chrome.debugger.onDetach.addListener((source, reason) => {
  if (source.tabId) {
    sessions.delete(source.tabId);
    log("warn", `Debugger detached from tab ${source.tabId}: ${reason}`);
  }
});

// ─── Browser Actions ─────────────────────────────────

async function doNavigate(action) {
  const { tabId } = await getOrCreateSession(action.tabId);
  await chrome.tabs.update(tabId, { url: action.url });
  await waitForLoad(tabId);
  const screenshot = await captureScreenshot(tabId);
  return { ok: true, url: action.url, screenshot };
}

async function doClick(action) {
  const { tabId, session } = await getOrCreateSession(action.tabId);
  const { target } = action;

  // Highlight before clicking
  try {
    if (target.strategy === "byIndex") {
      await chrome.tabs.sendMessage(tabId, { type: "yunque_highlight", index: target.index });
    } else if (target.strategy === "bySelector") {
      await chrome.tabs.sendMessage(tabId, { type: "yunque_highlight", selector: target.selector });
    } else if (target.strategy === "byCoordinates") {
      await chrome.tabs.sendMessage(tabId, { type: "yunque_highlight", x: target.coordinateX, y: target.coordinateY });
    }
    await sleep(200);
  } catch (_) {}

  if (target.strategy === "byCoordinates") {
    const { coordinateX: x, coordinateY: y } = target;
    await dispatchMouseEvent(session.target, "mousePressed", x, y);
    await dispatchMouseEvent(session.target, "mouseReleased", x, y);
  } else if (target.strategy === "bySelector") {
    await cdpSend(session.target, "Runtime.evaluate", {
      expression: `document.querySelector(${JSON.stringify(target.selector)})?.click()`,
      awaitPromise: true,
    });
  } else if (target.strategy === "byIndex") {
    await chrome.tabs.sendMessage(tabId, {
      type: "yunque_click_index",
      index: target.index,
    });
  }

  await sleep(300);
  const screenshot = await captureScreenshot(tabId);
  return { ok: true, screenshot };
}

async function doInput(action) {
  const { tabId, session } = await getOrCreateSession(action.tabId);
  const { target, text, pressEnter } = action;

  // Highlight before input
  try {
    if (target?.strategy === "bySelector") {
      await chrome.tabs.sendMessage(tabId, { type: "yunque_highlight", selector: target.selector });
    } else if (target?.strategy === "byCoordinates") {
      await chrome.tabs.sendMessage(tabId, { type: "yunque_highlight", x: target.coordinateX, y: target.coordinateY });
    }
    await sleep(150);
  } catch (_) {}

  if (target?.strategy === "bySelector" && target.selector) {
    await cdpSend(session.target, "Runtime.evaluate", {
      expression: `(() => {
        const el = document.querySelector(${JSON.stringify(target.selector)});
        if (el) { el.focus(); el.value = ${JSON.stringify(text)}; el.dispatchEvent(new Event('input', {bubbles:true})); }
      })()`,
    });
  } else if (target?.strategy === "byCoordinates") {
    await dispatchMouseEvent(session.target, "mousePressed", target.coordinateX, target.coordinateY);
    await dispatchMouseEvent(session.target, "mouseReleased", target.coordinateX, target.coordinateY);
    await sleep(100);
    await cdpSend(session.target, "Input.insertText", { text });
  } else {
    await cdpSend(session.target, "Input.insertText", { text });
  }

  if (pressEnter) {
    await cdpSend(session.target, "Input.dispatchKeyEvent", {
      type: "keyDown", key: "Enter", code: "Enter", windowsVirtualKeyCode: 13,
    });
    await cdpSend(session.target, "Input.dispatchKeyEvent", {
      type: "keyUp", key: "Enter", code: "Enter", windowsVirtualKeyCode: 13,
    });
  }

  await sleep(200);
  const screenshot = await captureScreenshot(tabId);
  return { ok: true, screenshot };
}

async function doScroll(action) {
  const { tabId, session } = await getOrCreateSession(action.tabId);
  const { direction, toEnd, coordinateX, coordinateY } = action;

  if (toEnd) {
    const scrollExpr = direction === "down" ? "window.scrollTo(0, document.body.scrollHeight)"
      : direction === "up" ? "window.scrollTo(0, 0)"
      : direction === "right" ? "window.scrollTo(document.body.scrollWidth, 0)"
      : "window.scrollTo(0, 0)";
    await cdpSend(session.target, "Runtime.evaluate", { expression: scrollExpr });
  } else {
    const deltaX = direction === "left" ? -400 : direction === "right" ? 400 : 0;
    const deltaY = direction === "up" ? -600 : direction === "down" ? 600 : 0;
    const x = coordinateX || 400;
    const y = coordinateY || 400;
    await cdpSend(session.target, "Input.dispatchMouseEvent", {
      type: "mouseWheel", x, y, deltaX, deltaY,
    });
  }

  await sleep(300);
  const screenshot = await captureScreenshot(tabId);
  return { ok: true, screenshot };
}

async function doPressKey(action) {
  const { session, tabId } = await getOrCreateSession(action.tabId);
  const keys = action.key.split("+");
  const modifiers = { Control: 0, Shift: 0, Alt: 0, Meta: 0 };
  let mainKey = keys[keys.length - 1];

  for (const k of keys.slice(0, -1)) {
    const mod = k.trim();
    if (mod in modifiers || mod === "Cmd" || mod === "Command") {
      const mapped = (mod === "Cmd" || mod === "Command") ? "Meta" : mod;
      await cdpSend(session.target, "Input.dispatchKeyEvent", {
        type: "keyDown", key: mapped, code: `${mapped}Left`,
      });
    }
  }

  const keyMap = { Enter: 13, Escape: 27, Tab: 9, Backspace: 8, Delete: 46, Space: 32 };
  await cdpSend(session.target, "Input.dispatchKeyEvent", {
    type: "keyDown", key: mainKey, code: mainKey,
    windowsVirtualKeyCode: keyMap[mainKey] || mainKey.charCodeAt(0),
  });
  await cdpSend(session.target, "Input.dispatchKeyEvent", {
    type: "keyUp", key: mainKey, code: mainKey,
  });

  for (const k of keys.slice(0, -1).reverse()) {
    const mod = k.trim();
    const mapped = (mod === "Cmd" || mod === "Command") ? "Meta" : mod;
    if (mapped in modifiers) {
      await cdpSend(session.target, "Input.dispatchKeyEvent", {
        type: "keyUp", key: mapped, code: `${mapped}Left`,
      });
    }
  }

  return { ok: true };
}

async function doScreenshot(action) {
  const { tabId } = await getOrCreateSession(action.tabId);
  const screenshot = await captureScreenshot(tabId);
  return { ok: true, screenshot };
}

async function doGetContent(action) {
  const { tabId } = await getOrCreateSession(action.tabId);
  const result = await chrome.tabs.sendMessage(tabId, { type: "yunque_get_content" });
  return { ok: true, content: result?.content || "", title: result?.title || "" };
}

async function doMoveMouse(action) {
  const { session } = await getOrCreateSession(action.tabId);
  await cdpSend(session.target, "Input.dispatchMouseEvent", {
    type: "mouseMoved", x: action.coordinateX, y: action.coordinateY,
  });
  return { ok: true };
}

async function doGetStructuredContent(action) {
  const { tabId } = await getOrCreateSession(action.tabId);
  const result = await chrome.tabs.sendMessage(tabId, { type: "yunque_get_structured_content" });
  return { ok: true, ...result };
}

// ─── Element Marking (Eye System) ────────────────────

async function doMarkElements(action) {
  const { tabId } = await getOrCreateSession(action.tabId);
  await chrome.tabs.sendMessage(tabId, { type: "yunque_show_markers" });
  const screenshot = await captureScreenshot(tabId);
  const { elements, total } = await chrome.tabs.sendMessage(tabId, { type: "yunque_get_elements" });
  return { ok: true, screenshot, elements, total };
}

async function doUnmarkElements(action) {
  const { tabId } = await getOrCreateSession(action.tabId);
  await chrome.tabs.sendMessage(tabId, { type: "yunque_hide_markers" });
  return { ok: true };
}

async function doGetElements(action) {
  const { tabId } = await getOrCreateSession(action.tabId);
  const { elements, total } = await chrome.tabs.sendMessage(tabId, { type: "yunque_get_elements" });
  return { ok: true, elements, total };
}

// ─── Multi-Tab Management ────────────────────────────

async function doListTabs() {
  const tabs = await chrome.tabs.query({ currentWindow: true });
  return {
    ok: true,
    tabs: tabs.map((t) => ({
      id: t.id,
      title: t.title,
      url: t.url,
      active: t.active,
      index: t.index,
    })),
  };
}

async function doSwitchTab(action) {
  const tabId = action.tabId;
  if (!tabId) return { ok: false, error: "tabId is required" };
  await chrome.tabs.update(tabId, { active: true });
  await sleep(300);
  const screenshot = await captureScreenshot(tabId);
  return { ok: true, tabId, screenshot };
}

async function doNewTab(action) {
  const tab = await chrome.tabs.create({ url: action.url || "about:blank", active: true });
  if (action.url) await waitForLoad(tab.id);
  const screenshot = await captureScreenshot(tab.id);
  return { ok: true, tabId: tab.id, screenshot };
}

async function doCloseTab(action) {
  const tabId = action.tabId;
  if (!tabId) return { ok: false, error: "tabId is required" };
  await detachSession(tabId);
  await chrome.tabs.remove(tabId);
  return { ok: true };
}

// ─── User Takeover ───────────────────────────────────

async function doSessionStatus(action) {
  const { status, sessionTitle, tabId } = action;

  if (status === "paused" || status === "take_over") {
    log("info", `User takeover activated: ${sessionTitle || "User takeover"}`);
    return syncTakeoverState(status, sessionTitle || "User takeover", tabId, true);
  }
  if (status === "resumed" || status === "running") {
    log("info", "User takeover ended, AI resumed");
    return syncTakeoverState(status, sessionTitle || "", tabId, true);
  }
  if (status === "stopped" && tabId) {
    await detachSession(tabId);
  }

  applyTakeoverState(false, "");
  await updateRuntimeSession({
    status: status === "stopped" ? "idle" : "running",
    title: sessionTitle || runtimeSession.title,
    currentTabId: tabId || runtimeSession.currentTabId,
  });
  sendToBackend({ type: "session_status", status, sessionTitle, takeover: takeover.active });
  return { ok: true, takeover: takeover.active, state: getRuntimeState() };
}

// ─── CDP Helpers ─────────────────────────────────────

async function captureScreenshot(tabId) {
  const { session } = await getOrCreateSession(tabId);
  try {
    const { data } = await cdpSend(session.target, "Page.captureScreenshot", {
      format: "jpeg",
      quality: 70,
      fromSurface: true,
    });
    return `data:image/jpeg;base64,${data}`;
  } catch (e) {
    log("warn", `Screenshot failed for tab ${tabId}: ${e.message}`);
    return null;
  }
}

async function dispatchMouseEvent(target, type, x, y, button = "left", clickCount = 1) {
  await cdpSend(target, "Input.dispatchMouseEvent", {
    type, x, y, button, clickCount,
  });
}

async function waitForLoad(tabId, timeout = 10000) {
  return new Promise((resolve) => {
    const timer = setTimeout(() => { cleanup(); resolve(); }, timeout);
    function onUpdated(id, info) {
      if (id === tabId && info.status === "complete") { cleanup(); resolve(); }
    }
    function cleanup() {
      clearTimeout(timer);
      chrome.tabs.onUpdated.removeListener(onUpdated);
    }
    chrome.tabs.onUpdated.addListener(onUpdated);
  });
}

// ─── Utility ─────────────────────────────────────────

function sleep(ms) { return new Promise((r) => setTimeout(r, ms)); }

function log(level, msg) {
  const prefix = "[YunqueBrowser]";
  if (level === "error") console.error(prefix, msg);
  else if (level === "warn") console.warn(prefix, msg);
  else console.log(prefix, msg);
}

function updateBadge(text, color) {
  chrome.action.setBadgeText({ text });
  chrome.action.setBadgeBackgroundColor({ color });
}

function applyTakeoverState(active, reason = "") {
  takeover.active = active;
  takeover.reason = active ? (reason || "User takeover") : "";
  if (active) {
    updateBadge("USR", "#f59e0b");
  } else {
    const badge = getBadgeState();
    updateBadge(badge.text, badge.color);
  }
}

async function syncTakeoverState(status, reason = "", tabId = null, notifyBackend = true) {
  const active = status === "paused" || status === "take_over";
  applyTakeoverState(active, reason);
  await updateRuntimeSession({
    status: active ? "takeover" : (status === "stopped" ? "idle" : "running"),
    title: reason || runtimeSession.title,
    currentTabId: tabId || runtimeSession.currentTabId,
  });
  if (notifyBackend) {
    sendToBackend({ type: "session_status", status, sessionTitle: reason, takeover: takeover.active });
  }
  return { ok: true, takeover: takeover.active, state: getRuntimeState() };
}


// ─── Lifecycle ───────────────────────────────────────

chrome.runtime.onInstalled.addListener(() => {
  log("info", "Yunque Browser Connector installed");
  connect();
});

chrome.runtime.onStartup.addListener(() => {
  connect();
});

chrome.tabs.onActivated.addListener(async ({ tabId }) => {
  await updateRuntimeSession({
    currentTabId: tabId,
    status: takeover.active ? "takeover" : runtimeSession.status,
  });
});

chrome.tabs.onUpdated.addListener(async (tabId, changeInfo, tab) => {
  if (runtimeSession.currentTabId && runtimeSession.currentTabId !== tabId) return;
  if (!changeInfo.url && !changeInfo.title && changeInfo.status !== "complete") return;
  await updateRuntimeSession({
    currentTabId: tabId,
    currentUrl: changeInfo.url || tab?.url || runtimeSession.currentUrl,
    title: changeInfo.title || tab?.title || runtimeSession.title,
    status: takeover.active ? "takeover" : runtimeSession.status,
  });
});

chrome.tabs.onRemoved.addListener(async (tabId) => {
  await detachSession(tabId);
  if (runtimeSession.currentTabId !== tabId) return;
  const fallbackTab = await getTabSnapshot();
  runtimeSession = {
    ...runtimeSession,
    currentTabId: fallbackTab?.tabId || null,
    currentUrl: fallbackTab?.url || "",
    title: fallbackTab?.title || "",
    status: takeover.active ? "takeover" : (fallbackTab ? "running" : "idle"),
    updatedAt: Date.now(),
  };
  persistRuntimeState();
  broadcastRuntimeState({ force: true });
});

// Also try connecting immediately when the service worker wakes up
restoreRuntimeState().finally(connect);

// Handle messages from popup
chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (msg.type === "get_status") {
    chrome.storage.local.get(["yunque_api_key"], (r) => {
      sendResponse({
        ...getRuntimeState(),
        apiKey: r.yunque_api_key || "",
      });
    });
    return true;
  }
  if (msg.type === "bridge_get_state") {
    sendResponse({ ok: true, state: getRuntimeState() });
    return true;
  }
  if (msg.type === "bridge_switch_to_tab") {
    const tabId = msg.tabId || runtimeSession.currentTabId;
    if (!tabId) {
      sendResponse({ ok: false, error: "no tab available" });
      return true;
    }
    chrome.tabs.update(tabId, { active: true }, async () => {
      const tab = await chrome.tabs.get(tabId).catch(() => null);
      if (tab?.windowId) chrome.windows.update(tab.windowId, { focused: true });
      await updateRuntimeSession({ currentTabId: tabId });
      sendResponse({ ok: true });
    });
    return true;
  }
  if (msg.type === "bridge_takeover") {
    (async () => {
      const result = await syncTakeoverState("take_over", msg.reason || "User takeover", msg.tabId || runtimeSession.currentTabId, true);
      sendResponse(result);
    })().catch((e) => sendResponse({ ok: false, error: e.message }));
    return true;
  }
  if (msg.type === "bridge_resume") {
    (async () => {
      const result = await syncTakeoverState("resumed", "", msg.tabId || runtimeSession.currentTabId, true);
      sendResponse(result);
    })().catch((e) => sendResponse({ ok: false, error: e.message }));
    return true;
  }
  if (msg.type === "resume_takeover") {
    (async () => {
      const result = await syncTakeoverState("resumed", "", msg.tabId || runtimeSession.currentTabId, true);
      sendResponse({ ok: result.ok, state: result.state });
    })().catch((e) => sendResponse({ ok: false, error: e.message }));
    return true;
  }
  if (msg.type === "set_ws_url") {
    wsUrl = msg.url;
    chrome.storage.local.set({
      yunque_ws_url: msg.url,
      yunque_api_key: msg.apiKey || "",
    });
    if (ws) ws.close();
    connect();
    sendResponse({ ok: true });
    return true;
  }
  if (msg.type === "reconnect") {
    if (ws) ws.close();
    connect();
    sendResponse({ ok: true });
    return true;
  }
});
