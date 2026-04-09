const els = {
  statusDot: document.getElementById("statusDot"),
  statusText: document.getElementById("statusText"),
  connectionMeta: document.getElementById("connectionMeta"),
  errorText: document.getElementById("errorText"),
  sessionCount: document.getElementById("sessionCount"),
  wsUrl: document.getElementById("wsUrl"),
  apiKey: document.getElementById("apiKey"),
  toriApiBase: document.getElementById("toriApiBase"),
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
};

function sendMessage(message) {
  return new Promise((resolve) => chrome.runtime.sendMessage(message, resolve));
}

function setBusy(button, busy, label) {
  button.disabled = busy;
  if (label) button.textContent = label;
}

function renderStatus(status) {
  const dotClass = status.takeover ? "warn" : status.connected ? "on" : "off";
  els.statusDot.className = `dot ${dotClass}`;
  els.statusText.textContent = status.takeover ? "Takeover" : status.connected ? "Connected" : "Offline";
  els.connectionMeta.textContent = `Bridge: ${status.wsUrl || "ws://localhost:9090/ws/browser"}`;
  els.sessionCount.textContent = status.sessions ? `${status.sessions} session${status.sessions > 1 ? "s" : ""}` : "No session";
  els.wsUrl.value = status.wsUrl || "";
  els.apiKey.value = status.apiKey || "";
  els.toriApiBase.value = status.toriApiBase || "http://localhost:3000";

  const profile = status.tori?.profile;
  const hasTori = !!profile;
  els.accountCard.style.display = hasTori ? "block" : "none";
  if (hasTori) {
    els.accountName.textContent = profile.username || profile.email || "Connected account";
    const grant = status.tori?.grantName || "Extension token ready";
    const scope = status.tori?.extensionScope || "browser:connect";
    els.accountMeta.textContent = `${grant} - ${scope}`;
  }

  if (status.lastConnectionError) {
    els.errorText.style.display = "block";
    els.errorText.textContent = status.lastConnectionError;
  } else {
    els.errorText.style.display = "none";
    els.errorText.textContent = "";
  }
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
  setBusy(els.connectToriBtn, true, "Connecting...");
  const result = await sendMessage({ type: "connect_tori", apiBase: els.toriApiBase.value.trim() });
  setBusy(els.connectToriBtn, false, "Connect Tori");
  if (!result?.ok) {
    els.errorText.style.display = "block";
    els.errorText.textContent = result?.error || "Tori connection failed";
    return;
  }
  await updateStatus();
});

els.disconnectToriBtn.addEventListener("click", async () => {
  setBusy(els.disconnectToriBtn, true, "Signing out...");
  await sendMessage({ type: "disconnect_tori" });
  setBusy(els.disconnectToriBtn, false, "Sign out");
  await updateStatus();
});

els.connectBtn.addEventListener("click", async () => {
  setBusy(els.connectBtn, true, "Saving...");
  await sendMessage({
    type: "set_ws_url",
    url: els.wsUrl.value.trim(),
    apiKey: els.apiKey.value.trim(),
    toriApiBase: els.toriApiBase.value.trim(),
  });
  setBusy(els.connectBtn, false, "Connect bridge");
  setTimeout(updateStatus, 300);
});

els.reconnectBtn.addEventListener("click", async () => {
  setBusy(els.reconnectBtn, true, "Reconnecting...");
  await sendMessage({ type: "reconnect" });
  setBusy(els.reconnectBtn, false, "Reconnect");
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
