// Yunque Browser Connector — Background Service Worker
// Architecture: Local WebSocket ↔ CDP (Chrome DevTools Protocol)

const CDP_VERSION = "1.3";
const RECONNECT_DELAY_MIN = 2000;
const RECONNECT_DELAY_MAX = 60000;
const NO_CREDENTIAL_RECONNECT_DELAY = 30000;
const SESSION_TIMEOUT = 120000;
const CDP_RETRY_COUNT = 2;
const CDP_RETRY_DELAY = 500;
const DEFAULT_WS_URL = "ws://localhost:9090/ws/browser";
const DEFAULT_TORI_API_BASE = "http://localhost:9090";
const RUNTIME_STATE_KEY = "yunque_runtime_state";
const MAX_OUTBOUND_QUEUE = 100;

// ─── State ───────────────────────────────────────────
let ws = null;
let wsUrl = DEFAULT_WS_URL;
let reconnectTimer = null;
let reconnectAttempts = 0;
let outboundQueue = [];
let lastBroadcastSignature = "";
let sessions = new Map();   // tabId → { target, lastUsed }
let connected = false;
let lastConnectionError = "";
let agentTabGroupId = null;
const AGENT_GROUP_COLOR = "blue";
let tabGroupsSupported = null;
let taskAnimationTimer = null;
const TASK_EMOJIS = ["👆","🖐️","👋","👍","🖖","🫰","✌","🤚","🤟","👉","🤞","👇","☝","🤙","👈","✊","🤘"];
const TASK_EMOJI_DONE = "✅";
const TASK_EMOJI_WAIT = "⌛️";
const TASK_EMOJI_INTERVAL = 1000;
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

function buildWebSocketURL(baseUrl) {
  try {
    const url = new URL(baseUrl || DEFAULT_WS_URL);
    return url.toString();
  } catch (_) {
    return baseUrl || DEFAULT_WS_URL;
  }
}

function buildSessionURL(baseUrl) {
  try {
    const ws = new URL(baseUrl || DEFAULT_WS_URL);
    const protocol = ws.protocol === "wss:" ? "https:" : "http:";
    return `${protocol}//${ws.host}/api/browser/ext/session`;
  } catch (_) {
    return "http://localhost:9090/api/browser/ext/session";
  }
}

function buildToriURL(baseUrl, path) {
  const normalized = (baseUrl || DEFAULT_TORI_API_BASE).trim().replace(/\/$/, "");
  return `${normalized}${path}`;
}

function storageGet(keys) {
  return new Promise((resolve) => chrome.storage.local.get(keys, resolve));
}

function storageSet(values) {
  return new Promise((resolve, reject) => {
    chrome.storage.local.set(values, () => {
      if (chrome.runtime.lastError) reject(new Error(chrome.runtime.lastError.message));
      else resolve();
    });
  });
}

function storageRemove(keys) {
  return new Promise((resolve) => chrome.storage.local.remove(keys, resolve));
}

async function sha256Hex(text) {
  const encoded = new TextEncoder().encode(text);
  const digest = await crypto.subtle.digest("SHA-256", encoded);
  return Array.from(new Uint8Array(digest)).map((b) => b.toString(16).padStart(2, "0")).join("");
}

async function sha256Base64Url(text) {
  const encoded = new TextEncoder().encode(text);
  const digest = await crypto.subtle.digest("SHA-256", encoded);
  const bytes = new Uint8Array(digest);
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}

function randomToken(length = 48) {
  const bytes = new Uint8Array(length);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (b) => (b % 36).toString(36)).join("");
}

function toFormBody(data) {
  const params = new URLSearchParams();
  Object.entries(data).forEach(([key, value]) => {
    if (value !== undefined && value !== null) params.set(key, String(value));
  });
  return params.toString();
}

async function fetchJSON(url, options = {}) {
  const res = await fetch(url, options);
  const raw = await res.text();
  let data = null;
  try {
    data = raw ? JSON.parse(raw) : null;
  } catch (_) {
    data = null;
  }
  if (!res.ok) {
    throw new Error((data && (data.message || data.error)) || `${res.status} ${raw}`);
  }
  return data;
}

async function getStoredToriState() {
  const data = await storageGet(["yunque_tori_api_base", "yunque_tori_oauth", "yunque_extension_token"]);
  return {
    apiBase: data.yunque_tori_api_base || DEFAULT_TORI_API_BASE,
    oauth: data.yunque_tori_oauth || null,
    extensionToken: data.yunque_extension_token || "",
  };
}

async function refreshToriAccessTokenIfNeeded(force = false) {
  const state = await getStoredToriState();
  if (!state.oauth?.refreshToken) return state;
  const expiresAt = state.oauth.expiresAt || 0;
  if (!force && Date.now() < expiresAt - 60_000) return state;
  const tokenData = await fetchJSON(buildToriURL(state.apiBase, "/oauth/token"), {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: toFormBody({
      grant_type: "refresh_token",
      client_id: "yunque-browser-extension",
      refresh_token: state.oauth.refreshToken,
    }),
  });
  state.oauth = {
    ...state.oauth,
    accessToken: tokenData.access_token,
    refreshToken: tokenData.refresh_token || state.oauth.refreshToken,
    expiresAt: Date.now() + (Number(tokenData.expires_in || 3600) * 1000),
    scope: tokenData.scope || state.oauth.scope || "browser:connect",
  };
  await storageSet({ yunque_tori_oauth: state.oauth, yunque_tori_api_base: state.apiBase });
  return state;
}

async function fetchToriUserInfo(state) {
  return fetchJSON(buildToriURL(state.apiBase, "/oauth/userinfo"), {
    headers: { Authorization: `Bearer ${state.oauth.accessToken}` },
  });
}

async function createExtensionGrant(state) {
  const payload = await fetchJSON(buildToriURL(state.apiBase, "/api/extension/grants"), {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${state.oauth.accessToken}`,
    },
    body: JSON.stringify({
      name: "Yunque Browser Connector",
      scope: "browser:connect",
      expires_in_days: 30,
    }),
  });
  return payload?.data || payload;
}

async function connectTori(apiBase) {
  const verifier = randomToken(64);
  const challenge = await sha256Base64Url(verifier);
  const state = randomToken(24);
  const redirectURI = chrome.identity.getRedirectURL("tori");
  const authorizeURL = new URL(buildToriURL(apiBase, "/oauth/authorize"));
  authorizeURL.searchParams.set("client_id", "yunque-browser-extension");
  authorizeURL.searchParams.set("redirect_uri", redirectURI);
  authorizeURL.searchParams.set("response_type", "code");
  authorizeURL.searchParams.set("scope", "browser:connect");
  authorizeURL.searchParams.set("state", state);
  authorizeURL.searchParams.set("code_challenge", challenge);
  authorizeURL.searchParams.set("code_challenge_method", "S256");

  const callbackURL = await chrome.identity.launchWebAuthFlow({
    url: authorizeURL.toString(),
    interactive: true,
  });
  if (!callbackURL) throw new Error("OAuth login was cancelled");
  const callback = new URL(callbackURL);
  if (callback.searchParams.get("state") !== state) throw new Error("OAuth state mismatch");
  const code = callback.searchParams.get("code");
  if (!code) throw new Error(callback.searchParams.get("error") || "OAuth authorization failed");

  const tokenData = await fetchJSON(buildToriURL(apiBase, "/oauth/token"), {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: toFormBody({
      grant_type: "authorization_code",
      client_id: "yunque-browser-extension",
      code,
      redirect_uri: redirectURI,
      code_verifier: verifier,
    }),
  });

  const oauthState = {
    accessToken: tokenData.access_token,
    refreshToken: tokenData.refresh_token || "",
    expiresAt: Date.now() + (Number(tokenData.expires_in || 3600) * 1000),
    scope: tokenData.scope || "browser:connect",
  };
  await storageSet({ yunque_tori_api_base: apiBase, yunque_tori_oauth: oauthState });

  const freshState = await refreshToriAccessTokenIfNeeded();
  const profile = await fetchToriUserInfo(freshState);
  const grant = await createExtensionGrant(freshState);
  await storageSet({
    yunque_extension_token: grant.token,
    yunque_tori_oauth: {
      ...freshState.oauth,
      profile,
      grantId: grant.id,
      grantName: grant.name,
      extensionScope: grant.scope,
    },
  });
  return { profile, grant };
}

async function disconnectTori() {
  const state = await getStoredToriState();
  try {
    if (state.oauth?.accessToken && state.oauth?.grantId) {
      await fetch(buildToriURL(state.apiBase, `/api/extension/grants/${state.oauth.grantId}`), {
        method: "DELETE",
        headers: { Authorization: `Bearer ${state.oauth.accessToken}` },
      });
    }
    if (state.oauth?.refreshToken) {
      await fetch(buildToriURL(state.apiBase, "/oauth/revoke"), {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: toFormBody({ token: state.oauth.refreshToken }),
      });
    }
  } catch (e) {
    log("warn", `Tori disconnect cleanup failed: ${e.message}`);
  }
  await storageRemove(["yunque_tori_oauth", "yunque_extension_token"]);
}

async function resolveSessionCredential() {
  const data = await storageGet(["yunque_api_key", "yunque_extension_token", "yunque_jwt"]);
  const apiKey = (data.yunque_api_key || "").trim();
  const extensionToken = (data.yunque_extension_token || "").trim();
  const jwt = (data.yunque_jwt || "").trim();
  if (apiKey) return { type: "api_key", value: apiKey, label: "manual" };
  if (extensionToken) return { type: "bearer", value: extensionToken, label: "tori" };
  if (jwt) return { type: "bearer", value: jwt, label: "jwt" };
  return { type: "none", value: "", label: "anonymous" };
}

async function fetchBrowserSession(baseUrl, credential) {
  const headers = { "Content-Type": "application/json" };
  if (credential?.type === "api_key" && credential.value) headers["X-API-Key"] = credential.value;
  if (credential?.type === "bearer" && credential.value) headers["Authorization"] = `Bearer ${credential.value}`;
  const res = await fetch(buildSessionURL(baseUrl), {
    method: "POST",
    headers,
    body: "{}",
  });
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`Session request failed: ${res.status} ${body}`);
  }
  return res.json();
}

function connect() {
  if (ws && ws.readyState <= 1) return;

  chrome.storage.local.get(["yunque_ws_url"], async (r) => {
    wsUrl = r.yunque_ws_url || DEFAULT_WS_URL;
    let session;
    let socketUrl = buildWebSocketURL(wsUrl);
    try {
      const credential = await resolveSessionCredential();
      if (credential.type === "none") {
        lastConnectionError =
          "缺少凭证：请在扩展弹窗填写 Yunque API Key（云雀 WebUI → 设置），或通过 Tori 登录获取连接器令牌。未携带凭证时无法连接 /ws/browser。";
        log("error", lastConnectionError);
        scheduleReconnect(NO_CREDENTIAL_RECONNECT_DELAY);
        broadcastRuntimeState({ force: true });
        return;
      }
      session = await fetchBrowserSession(wsUrl, credential);
      const url = new URL(session.ws_url || socketUrl);
      url.searchParams.set("ticket", session.ticket);
      url.searchParams.set("nonce", session.nonce);
      socketUrl = url.toString();
      lastConnectionError = "";
    } catch (e) {
      lastConnectionError = e.message;
      log("error", `Session bootstrap failed: ${e.message}`);
      scheduleReconnect();
      broadcastRuntimeState({ force: true });
      return;
    }
    log("info", `Connecting to ${socketUrl.replace(/([?&](?:ticket|nonce)=)[^&]+/g, "$1***")}`);

    let socket;
    try {
      socket = new WebSocket(socketUrl);
      ws = socket;
    } catch (e) {
      log("error", `WebSocket create failed: ${e.message}`);
      scheduleReconnect();
      return;
    }

    let challengeDone = !session?.ticket;

    socket.onopen = () => {
      if (ws !== socket) return;
      connected = true;
      lastConnectionError = "";
      reconnectAttempts = 0;
      log("info", "WebSocket connected");
      clearTimeout(reconnectTimer);
      if (challengeDone) {
        sendToBackend({ type: "hello", version: chrome.runtime.getManifest().version }, { queueIfOffline: false });
        flushOutboundQueue();
      }
      restoreBadgeFromState();
      broadcastRuntimeState({ force: true });
    };

    socket.onmessage = async (evt) => {
      if (ws !== socket) return;
      try {
        const msg = JSON.parse(evt.data);
        if (msg?.type === "challenge") {
          if (!session?.ticket || !msg.challenge || !msg.nonce) {
            throw new Error("Missing challenge context");
          }
          const proof = await sha256Hex(`${session.ticket}:${msg.nonce}:${msg.challenge}`);
          sendToBackend({ type: "challenge_response", proof }, { queueIfOffline: false });
          challengeDone = true;
          sendToBackend({ type: "hello", version: chrome.runtime.getManifest().version }, { queueIfOffline: false });
          flushOutboundQueue();
          return;
        }
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
      lastConnectionError = "WebSocket error";
      log("error", "WebSocket error");
      updateBadge("ERR", "#F44336");
      persistRuntimeState();
      broadcastRuntimeState({ force: true });
    };
  });
}

function scheduleReconnect(delayMs) {
  clearTimeout(reconnectTimer);
  if (delayMs == null) {
    const backoff = Math.min(RECONNECT_DELAY_MIN * Math.pow(1.5, reconnectAttempts), RECONNECT_DELAY_MAX);
    const jitter = backoff * (0.5 + Math.random() * 0.5);
    delayMs = Math.round(jitter);
    reconnectAttempts++;
  }
  log("info", `Reconnecting in ${Math.round(delayMs / 1000)}s (attempt ${reconnectAttempts})`);
  reconnectTimer = setTimeout(connect, delayMs);
}

chrome.storage.onChanged.addListener((changes, areaName) => {
  if (areaName !== "local") return;
  if (!changes.yunque_api_key && !changes.yunque_extension_token && !changes.yunque_ws_url && !changes.yunque_jwt) return;
  clearTimeout(reconnectTimer);
  if (ws) {
    try {
      ws.close();
    } catch (_) {}
  }
  connect();
});

function enqueueOutboundMessage(msg) {
  if (outboundQueue.length >= MAX_OUTBOUND_QUEUE) outboundQueue.shift();
  outboundQueue.push(msg);
}

function flushOutboundQueue() {
  while (outboundQueue.length > 0) {
    const msg = outboundQueue.shift();
    sendToBackend(msg, { queueIfOffline: false, fromQueue: true });
  }
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
    lastConnectionError,
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
  const targetTabId = runtimeSession.currentTabId;
  chrome.tabs.query({}, (tabs) => {
    for (const tab of tabs) {
      if (!tab.id) continue;
      const isTarget = tab.id === targetTabId;
      chrome.tabs.sendMessage(tab.id, {
        type: "bridge_state_update",
        state,
        isTargetTab: isTarget,
      }, () => void chrome.runtime.lastError);
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
    if (runtimeSession.currentTabId) {
      tabId = runtimeSession.currentTabId;
    } else {
      const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
      if (!tab) throw new Error("No active tab");
      if (isProtectedTab(tab)) throw new Error("Cannot attach debugger to the Yunque WebUI tab. Use browser_navigate to open a target URL first.");
      tabId = tab.id;
    }
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

async function cdpSendOnce(target, method, params) {
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

async function cdpSend(target, method, params) {
  for (let attempt = 0; attempt <= CDP_RETRY_COUNT; attempt++) {
    try {
      return await cdpSendOnce(target, method, params);
    } catch (e) {
      const isRetryable = e.message.includes("not attached") || e.message.includes("Target closed") || e.message.includes("Cannot find context");
      if (attempt < CDP_RETRY_COUNT && isRetryable) {
        log("warn", `CDP retry ${attempt + 1}/${CDP_RETRY_COUNT}: ${method} — ${e.message}`);
        await sleep(CDP_RETRY_DELAY * (attempt + 1));
        try {
          const tabId = target.tabId;
          if (tabId) {
            sessions.delete(tabId);
            await new Promise((resolve, reject) => {
              chrome.debugger.attach(target, CDP_VERSION, () => {
                if (chrome.runtime.lastError) reject(new Error(chrome.runtime.lastError.message));
                else resolve();
              });
            });
            await cdpSendOnce(target, "Page.enable");
            sessions.set(tabId, { target, lastUsed: Date.now() });
          }
        } catch (reattachErr) {
          log("warn", `CDP reattach failed: ${reattachErr.message}`);
        }
        continue;
      }
      throw e;
    }
  }
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

// ─── Tab Group Isolation ─────────────────────────────

let _agentGroupSeedTabId = null;

function isTabGroupsSupported() {
  if (tabGroupsSupported !== null) return tabGroupsSupported;
  tabGroupsSupported = typeof chrome.tabs !== "undefined"
    && typeof chrome.tabs.group === "function"
    && typeof chrome.tabGroups !== "undefined";
  return tabGroupsSupported;
}

async function ensureAgentTabGroup(title) {
  if (!isTabGroupsSupported()) return null;

  if (agentTabGroupId != null) {
    try {
      const group = await chrome.tabGroups.get(agentTabGroupId);
      if (group) {
        if (title) await chrome.tabGroups.update(agentTabGroupId, { title }).catch(() => {});
        return agentTabGroupId;
      }
    } catch (_) {
      agentTabGroupId = null;
    }
  }

  try {
    const tab = await chrome.tabs.create({ url: "about:blank", active: false });
    _agentGroupSeedTabId = tab.id;
    const groupId = await chrome.tabs.group({ tabIds: [tab.id] });
    await chrome.tabGroups.update(groupId, {
      title: title || "Yunque AI",
      color: AGENT_GROUP_COLOR,
      collapsed: false,
    });
    const pinnedTabs = await chrome.tabs.query({ pinned: true });
    await chrome.tabGroups.move(groupId, { index: pinnedTabs.length });
    agentTabGroupId = groupId;
    return groupId;
  } catch (e) {
    log("warn", `TabGroups not supported, switching to simple tab mode: ${e.message}`);
    tabGroupsSupported = false;
    return null;
  }
}

// ─── Task Status Animation ───────────────────────────

function startTaskAnimation(title) {
  stopTaskAnimation();
  if (!isTabGroupsSupported() || agentTabGroupId == null) return;
  let idx = 0;
  const tick = async () => {
    if (agentTabGroupId == null) { stopTaskAnimation(); return; }
    const emoji = TASK_EMOJIS[idx % TASK_EMOJIS.length];
    try { await chrome.tabGroups.update(agentTabGroupId, { title: `${emoji} ${title}` }); } catch (_) {}
    idx++;
  };
  tick();
  taskAnimationTimer = setInterval(tick, TASK_EMOJI_INTERVAL);
}

function stopTaskAnimation() {
  if (taskAnimationTimer) {
    clearInterval(taskAnimationTimer);
    taskAnimationTimer = null;
  }
}

async function setTaskDone(title) {
  stopTaskAnimation();
  if (!isTabGroupsSupported() || agentTabGroupId == null) return;
  try { await chrome.tabGroups.update(agentTabGroupId, { title: `${TASK_EMOJI_DONE} ${title}` }); } catch (_) {}
}

async function setTaskWaiting(title) {
  stopTaskAnimation();
  if (!isTabGroupsSupported() || agentTabGroupId == null) return;
  try { await chrome.tabGroups.update(agentTabGroupId, { title: `${TASK_EMOJI_WAIT} ${title}` }); } catch (_) {}
}

async function cleanupSeedTab() {
  if (_agentGroupSeedTabId != null) {
    try { await chrome.tabs.remove(_agentGroupSeedTabId); } catch (_) {}
    _agentGroupSeedTabId = null;
  }
}

async function addTabToAgentGroup(tabId, title) {
  const groupId = await ensureAgentTabGroup(title);
  if (groupId == null) {
    // TabGroups not supported — move tab to leftmost position instead
    try {
      const pinnedTabs = await chrome.tabs.query({ pinned: true });
      await chrome.tabs.move(tabId, { index: pinnedTabs.length });
    } catch (_) {}
    return;
  }
  try {
    await chrome.tabs.group({ tabIds: [tabId], groupId });
    await cleanupSeedTab();
  } catch (e) {
    log("warn", `Failed to add tab ${tabId} to agent group: ${e.message}`);
  }
}

// ─── Browser Actions ─────────────────────────────────

function isProtectedTab(tab) {
  if (!tab || !tab.url) return false;
  try {
    const u = new URL(tab.url);
    return u.hostname === "localhost" || u.hostname === "127.0.0.1" || u.hostname === "0.0.0.0";
  } catch (_) { return false; }
}

async function doNavigate(action) {
  let tabId = null;

  if (agentTabGroupId != null) {
    const groupTabs = await chrome.tabs.query({ groupId: agentTabGroupId }).catch(() => []);
    const existingTab = groupTabs.find((t) => t.url && t.url !== "about:blank" && !isProtectedTab(t));
    if (existingTab) tabId = existingTab.id;
  }

  if (tabId) {
    const current = await chrome.tabs.get(tabId).catch(() => null);
    if (current && isProtectedTab(current)) {
      tabId = null;
    }
  }

  if (tabId) {
    await chrome.tabs.update(tabId, { url: action.url, active: true });
  } else {
    const tab = await chrome.tabs.create({ url: action.url, active: true });
    tabId = tab.id;
    await addTabToAgentGroup(tabId, runtimeSession.title || "Yunque AI");
  }

  await getOrCreateSession(tabId);
  await waitForLoad(tabId);
  runtimeSession.currentTabId = tabId;
  const screenshot = await captureScreenshot(tabId);
  return { ok: true, url: action.url, tabId, screenshot };
}

async function doClick(action) {
  const { tabId, session } = await getOrCreateSession(action.tabId);
  const { target } = action;

  try {
    if (target.strategy === "byIndex") {
      await sendTabMessage(tabId, { type: "yunque_highlight", index: target.index });
    } else if (target.strategy === "bySelector") {
      await sendTabMessage(tabId, { type: "yunque_highlight", selector: target.selector });
    } else if (target.strategy === "byCoordinates") {
      await sendTabMessage(tabId, { type: "yunque_highlight", x: target.coordinateX, y: target.coordinateY });
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
    await sendTabMessage(tabId, {
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

  try {
    if (target?.strategy === "bySelector") {
      await sendTabMessage(tabId, { type: "yunque_highlight", selector: target.selector });
    } else if (target?.strategy === "byCoordinates") {
      await sendTabMessage(tabId, { type: "yunque_highlight", x: target.coordinateX, y: target.coordinateY });
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
  const result = await sendTabMessage(tabId, { type: "yunque_get_content" });
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
  const result = await sendTabMessage(tabId, { type: "yunque_get_structured_content" });
  return { ok: true, ...result };
}

// ─── Element Marking (Eye System) ────────────────────

async function doMarkElements(action) {
  const { tabId } = await getOrCreateSession(action.tabId);
  await sendTabMessage(tabId, { type: "yunque_show_markers" });
  const screenshot = await captureScreenshot(tabId);
  const { elements, total } = await sendTabMessage(tabId, { type: "yunque_get_elements" });
  return { ok: true, screenshot, elements, total };
}

async function doUnmarkElements(action) {
  const { tabId } = await getOrCreateSession(action.tabId);
  await sendTabMessage(tabId, { type: "yunque_hide_markers" });
  return { ok: true };
}

async function doGetElements(action) {
  const { tabId } = await getOrCreateSession(action.tabId);
  const { elements, total } = await sendTabMessage(tabId, { type: "yunque_get_elements" });
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
  await addTabToAgentGroup(tab.id, runtimeSession.title);
  if (action.url) await waitForLoad(tab.id);
  const screenshot = await captureScreenshot(tab.id);
  return { ok: true, tabId: tab.id, screenshot };
}

async function doCloseTab(action) {
  const tabId = action.tabId;
  if (!tabId) return { ok: false, error: "tabId is required" };
  await detachSession(tabId);
  await chrome.tabs.remove(tabId);
  if (isTabGroupsSupported() && agentTabGroupId != null) {
    try {
      const tabs = await chrome.tabs.query({ groupId: agentTabGroupId });
      if (!tabs || tabs.length === 0) agentTabGroupId = null;
    } catch (_) { agentTabGroupId = null; }
  }
  return { ok: true };
}

// ─── User Takeover ───────────────────────────────────

async function doSessionStatus(action) {
  const { status, sessionTitle, tabId } = action;
  const label = sessionTitle || runtimeSession.title || "Yunque AI";

  if (status === "paused" || status === "take_over") {
    setTaskWaiting(label);
    if (isTabGroupsSupported() && agentTabGroupId != null) {
      try { await chrome.tabGroups.update(agentTabGroupId, { color: "yellow" }); } catch (_) {}
    }
    log("info", `User takeover activated: ${label}`);
    return syncTakeoverState(status, label, tabId, true);
  }
  if (status === "resumed" || status === "running") {
    startTaskAnimation(label);
    if (isTabGroupsSupported() && agentTabGroupId != null) {
      try { await chrome.tabGroups.update(agentTabGroupId, { color: AGENT_GROUP_COLOR }); } catch (_) {}
    }
    log("info", "User takeover ended, AI resumed");
    return syncTakeoverState(status, label, tabId, true);
  }
  if (status === "stopped") {
    await setTaskDone(label);
    if (tabId) await detachSession(tabId);
    if (isTabGroupsSupported() && agentTabGroupId != null) {
      try { await chrome.tabGroups.update(agentTabGroupId, { color: "grey" }); } catch (_) {}
    }
    agentTabGroupId = null;
  }
  if (status === "error") {
    stopTaskAnimation();
    if (isTabGroupsSupported() && agentTabGroupId != null) {
      try { await chrome.tabGroups.update(agentTabGroupId, { color: "red", title: `❌ ${label}` }); } catch (_) {}
    }
  }

  applyTakeoverState(false, "");
  await updateRuntimeSession({
    status: status === "stopped" ? "idle" : "running",
    title: label,
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

async function waitForLoad(tabId, timeout = 30000) {
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

async function sendTabMessage(tabId, msg, retries = 2) {
  for (let i = 0; i <= retries; i++) {
    try {
      return await chrome.tabs.sendMessage(tabId, msg);
    } catch (e) {
      if (i < retries && (e.message.includes("Receiving end does not exist") || e.message.includes("Could not establish connection"))) {
        log("warn", `Content script not ready (attempt ${i + 1}), injecting and retrying...`);
        try {
          await chrome.scripting.executeScript({ target: { tabId }, files: ["content.js"] });
        } catch (_) {}
        await sleep(300);
        continue;
      }
      throw e;
    }
  }
}

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
  if (isTabGroupsSupported() && agentTabGroupId != null) {
    const tab = await chrome.tabs.get(tabId).catch(() => null);
    if (!tab || tab.groupId !== agentTabGroupId) return;
  }
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
    chrome.storage.local.get(["yunque_api_key", "yunque_tori_api_base", "yunque_tori_oauth", "yunque_extension_token", "yunque_jwt"], (r) => {
      sendResponse({
        ...getRuntimeState(),
        apiKey: r.yunque_api_key || "",
        toriApiBase: r.yunque_tori_api_base || DEFAULT_TORI_API_BASE,
        extensionTokenPresent: !!r.yunque_extension_token,
        jwtPresent: !!r.yunque_jwt,
        tori: r.yunque_tori_oauth || null,
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
  if (msg.type === "bridge_stop_session") {
    (async () => {
      stopTaskAnimation();
      sendToBackend({ type: "session_status", status: "stopped" });
      applyTakeoverState(false, "");
      await updateRuntimeSession({ status: "idle" }, { forceBroadcast: true });
      if (isTabGroupsSupported() && agentTabGroupId != null) {
        try { await chrome.tabGroups.update(agentTabGroupId, { color: "grey" }); } catch (_) {}
        agentTabGroupId = null;
      }
      sendResponse({ ok: true });
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
  if (msg.type === "connect_tori") {
    (async () => {
      const apiBase = (msg.apiBase || DEFAULT_TORI_API_BASE).trim();
      const result = await connectTori(apiBase);
      sendResponse({ ok: true, ...result });
    })().catch((e) => sendResponse({ ok: false, error: e.message }));
    return true;
  }
  if (msg.type === "disconnect_tori") {
    (async () => {
      await disconnectTori();
      sendResponse({ ok: true });
    })().catch((e) => sendResponse({ ok: false, error: e.message }));
    return true;
  }
  if (msg.type === "set_ws_url") {
    wsUrl = msg.url;
    chrome.storage.local.set({
      yunque_ws_url: msg.url,
      yunque_api_key: msg.apiKey || "",
      ...(msg.toriApiBase ? { yunque_tori_api_base: msg.toriApiBase } : {}),
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
