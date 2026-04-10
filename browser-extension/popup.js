const els = {
  statusDot: document.getElementById("statusDot"),
  statusText: document.getElementById("statusText"),
  connectionMeta: document.getElementById("connectionMeta"),
  errorText: document.getElementById("errorText"),
  sessionCount: document.getElementById("sessionCount"),
  wsUrl: document.getElementById("wsUrl"),
  apiKey: document.getElementById("apiKey"),
  toriApiBase: document.getElementById("toriApiBase"),
  toriDetails: document.getElementById("toriDetails"),
  accountCard: document.getElementById("accountCard"),
  accountName: document.getElementById("accountName"),
  accountMeta: document.getElementById("accountMeta"),
  connectBtn: document.getElementById("connectBtn"),
  reconnectBtn: document.getElementById("reconnectBtn"),
  connectToriBtn: document.getElementById("connectToriBtn"),
  disconnectToriBtn: document.getElementById("disconnectToriBtn"),
  markBtn: document.getElementById("markBtn"),
  resumeBtn: document.getElementById("resumeBtn"),
  tabList: document.getElementById("tabList"),
  credentialBanner: document.getElementById("credentialBanner"),
  apiKeyDetails: document.getElementById("apiKeyDetails"),
  importFromTabBtn: document.getElementById("importFromTabBtn"),
};

/** @type {Record<string, unknown>} */
let lastStatus = {};

function sendMessage(message) {
  return new Promise((resolve) => chrome.runtime.sendMessage(message, resolve));
}

function setBusy(button, busy, label) {
  button.disabled = busy;
  if (label) button.textContent = label;
}

function credentialOk() {
  const key = (els.apiKey?.value || "").trim() || (lastStatus.apiKey || "").trim();
  return !!key || !!lastStatus.extensionTokenPresent || !!lastStatus.jwtPresent;
}

function updateCredentialUI() {
  const ok = credentialOk();
  if (els.credentialBanner) els.credentialBanner.style.display = ok ? "none" : "block";
  if (els.reconnectBtn) els.reconnectBtn.disabled = !ok;
  if (els.apiKeyDetails) els.apiKeyDetails.open = !ok;
}

function renderStatus(status) {
  lastStatus = status || {};
  const dotClass = status.takeover ? "warn" : status.connected ? "on" : "off";
  els.statusDot.className = `dot ${dotClass}`;
  els.statusText.textContent = status.takeover ? "接管" : status.connected ? "已连接" : "离线";
  els.connectionMeta.textContent = `桥接：${status.wsUrl || "ws://localhost:9090/ws/browser"}`;
  els.sessionCount.textContent = status.sessions ? `${status.sessions} 个会话` : "无会话";
  els.wsUrl.value = status.wsUrl || "";
  els.apiKey.value = status.apiKey || "";
  els.toriApiBase.value = status.toriApiBase || "http://localhost:3000";

  const profile = status.tori?.profile;
  const hasTori = !!profile;
  els.accountCard.style.display = hasTori ? "block" : "none";
  if (hasTori) {
    els.accountName.textContent = profile.username || profile.email || "已登录";
    const grant = status.tori?.grantName || "Extension token ready";
    const scope = status.tori?.extensionScope || "browser:connect";
    els.accountMeta.textContent = `${grant} - ${scope}`;
    if (els.toriDetails) els.toriDetails.open = true;
  }

  if (status.lastConnectionError) {
    els.errorText.style.display = "block";
    els.errorText.textContent = status.lastConnectionError;
  } else {
    els.errorText.style.display = "none";
    els.errorText.textContent = "";
  }
  updateCredentialUI();
}

async function updateStatus() {
  const status = await sendMessage({ type: "get_status" });
  if (status) renderStatus(status);
}

async function loadTabs() {
  const tabs = await chrome.tabs.query({ currentWindow: true });
  els.tabList.innerHTML = "";
  for (const tab of tabs) {
    const item = document.createElement("div");
    item.className = `tab-item${tab.active ? " active" : ""}`;
    item.innerHTML = `<span class="dot ${tab.active ? "on" : "off"}"></span><span class="tab-title" title="${tab.url || ""}">${tab.title || "Untitled"}</span>`;
    item.addEventListener("click", async () => {
      await chrome.tabs.update(tab.id, { active: true });
      setTimeout(loadTabs, 150);
    });
    els.tabList.appendChild(item);
  }
}

els.connectToriBtn.addEventListener("click", async () => {
  setBusy(els.connectToriBtn, true, "登录中...");
  const result = await sendMessage({ type: "connect_tori", apiBase: els.toriApiBase.value.trim() });
  setBusy(els.connectToriBtn, false, "登录 Tori");
  if (!result?.ok) {
    els.errorText.style.display = "block";
    els.errorText.textContent = result?.error || "Tori connection failed";
    return;
  }
  await updateStatus();
});

els.disconnectToriBtn.addEventListener("click", async () => {
  setBusy(els.disconnectToriBtn, true, "退出中...");
  await sendMessage({ type: "disconnect_tori" });
  setBusy(els.disconnectToriBtn, false, "退出");
  await updateStatus();
});

els.apiKey?.addEventListener("input", () => updateCredentialUI());

els.importFromTabBtn?.addEventListener("click", async () => {
  setBusy(els.importFromTabBtn, true, "读取中...");
  try {
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
    if (!tab?.id || !tab.url) throw new Error("没有可用标签页");
    const u = new URL(tab.url);
    if (u.hostname !== "localhost" && u.hostname !== "127.0.0.1") {
      throw new Error("请先切换到本机云雀页面（localhost 或 127.0.0.1）");
    }
    const injected = await chrome.scripting.executeScript({
      target: { tabId: tab.id },
      func: () => ({
        apiKey: localStorage.getItem("yunque_api_key") || "",
        jwt: localStorage.getItem("yunque_token") || "",
      }),
    });
    const result = injected[0]?.result || {};
    const apiKey = (result.apiKey || "").trim();
    const jwt = (result.jwt || "").trim();
    if (!apiKey && !jwt) {
      throw new Error("当前页面未找到登录信息：请在云雀 Web 登录（或保存 API Key）后重试。");
    }
    if (apiKey) {
      await chrome.storage.local.set({ yunque_api_key: apiKey, yunque_jwt: "" });
    } else {
      await chrome.storage.local.set({ yunque_jwt: jwt, yunque_api_key: "" });
    }
    await sendMessage({ type: "reconnect" });
    await updateStatus();
    els.errorText.style.display = "none";
    els.errorText.textContent = "";
  } catch (e) {
    els.errorText.style.display = "block";
    els.errorText.textContent = e?.message || String(e);
  } finally {
    setBusy(els.importFromTabBtn, false, "从云雀页面导入");
  }
});

els.connectBtn.addEventListener("click", async () => {
  if (!credentialOk()) {
    els.errorText.style.display = "block";
    els.errorText.textContent = "请先填写 API Key，或先完成 Tori 登录以获取 extension token。";
    return;
  }
  setBusy(els.connectBtn, true, "保存中...");
  await sendMessage({
    type: "set_ws_url",
    url: els.wsUrl.value.trim(),
    apiKey: els.apiKey.value.trim(),
    toriApiBase: els.toriApiBase.value.trim(),
  });
  setBusy(els.connectBtn, false, "保存并连接");
  setTimeout(updateStatus, 300);
});

els.reconnectBtn.addEventListener("click", async () => {
  setBusy(els.reconnectBtn, true, "重连中...");
  await sendMessage({ type: "reconnect" });
  setBusy(els.reconnectBtn, false, "重连");
  setTimeout(updateStatus, 300);
});

els.markBtn.addEventListener("click", async () => {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (!tab?.id) return;
  const res = await chrome.tabs.sendMessage(tab.id, { type: "yunque_show_markers" });
  els.markBtn.textContent = res?.ok ? `Marked ${res.count}` : "Mark";
  setTimeout(() => { els.markBtn.textContent = "Mark"; }, 1200);
});

els.resumeBtn.addEventListener("click", async () => {
  await sendMessage({ type: "resume_takeover" });
  setTimeout(updateStatus, 200);
});

updateStatus();
loadTabs();
