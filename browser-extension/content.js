// Yunque Browser Connector — Content Script
// Injected into every page for DOM interaction, element marking, and content extraction.

(function () {
  "use strict";

  const MARKER_ATTR = "data-yunque-idx";
  const OVERLAY_CLASS = "yunque-marker-overlay";
  const HIGHLIGHT_CLASS = "yunque-action-highlight";
  let markersVisible = false;

  function emitBridgeEvent(type, payload) {
    window.postMessage({ source: "yunque-bridge", type, payload }, "*");
  }

  async function requestBridgeState() {
    return new Promise((resolve) => {
      chrome.runtime.sendMessage({ type: "bridge_get_state" }, (res) => {
        resolve(res?.state || null);
      });
    });
  }

  async function handleBridgeRequest(data) {
    switch (data.type) {
      case "bridge/ping": {
        const state = await requestBridgeState();
        emitBridgeEvent("bridge/ready", { state });
        break;
      }
      case "bridge/get-state": {
        const state = await requestBridgeState();
        emitBridgeEvent("bridge/state", { state, requestId: data.requestId || null });
        break;
      }
      case "bridge/switch-to-tab": {
        chrome.runtime.sendMessage({ type: "bridge_switch_to_tab", tabId: data.tabId }, (res) => {
          emitBridgeEvent("bridge/action-result", { requestId: data.requestId || null, action: data.type, result: res });
        });
        break;
      }
      case "bridge/takeover": {
        chrome.runtime.sendMessage({ type: "bridge_takeover", reason: data.reason || "User takeover" }, (res) => {
          emitBridgeEvent("bridge/action-result", { requestId: data.requestId || null, action: data.type, result: res });
        });
        break;
      }
      case "bridge/resume": {
        chrome.runtime.sendMessage({ type: "bridge_resume" }, (res) => {
          emitBridgeEvent("bridge/action-result", { requestId: data.requestId || null, action: data.type, result: res });
        });
        break;
      }
      default:
        break;
    }
  }

  window.addEventListener("message", (event) => {
    if (event.source !== window) return;
    const data = event.data;
    if (!data || data.source !== "yunque-app") return;
    handleBridgeRequest(data).catch((err) => {
      emitBridgeEvent("bridge/error", { requestId: data.requestId || null, error: err?.message || String(err) });
    });
  });

  // ─── Styles ────────────────────────────────────────

  function injectStyles() {
    if (document.getElementById("yunque-ext-styles")) return;
    const style = document.createElement("style");
    style.id = "yunque-ext-styles";
    style.textContent = `
      .${OVERLAY_CLASS} {
        position: absolute;
        pointer-events: none;
        z-index: 2147483646;
        border: 2px solid rgba(59,130,246,0.7);
        border-radius: 3px;
        background: rgba(59,130,246,0.06);
        transition: opacity 0.15s;
      }
      .${OVERLAY_CLASS}::after {
        content: attr(data-label);
        position: absolute;
        top: -16px;
        left: -1px;
        font: bold 10px/14px monospace;
        color: #fff;
        background: rgba(59,130,246,0.85);
        padding: 0 4px;
        border-radius: 2px 2px 0 0;
        white-space: nowrap;
      }
      .${HIGHLIGHT_CLASS} {
        outline: 3px solid #f59e0b !important;
        outline-offset: 2px;
        box-shadow: 0 0 12px rgba(245,158,11,0.5) !important;
        transition: outline 0.2s, box-shadow 0.2s;
      }
      @keyframes yunque-pulse {
        0%, 100% { outline-color: #f59e0b; box-shadow: 0 0 8px rgba(245,158,11,0.4); }
        50% { outline-color: #ef4444; box-shadow: 0 0 16px rgba(239,68,68,0.6); }
      }
      .yunque-pulse { animation: yunque-pulse 0.6s ease-in-out 2; }
    `;
    document.head.appendChild(style);
  }

  // ─── Interactive Element Detection ─────────────────

  function getInteractiveElements() {
    const selectors = [
      "a[href]", "button", "input", "textarea", "select",
      "[role=button]", "[role=link]", "[role=tab]", "[role=menuitem]",
      "[role=checkbox]", "[role=radio]", "[role=switch]", "[role=combobox]",
      "[onclick]", "[tabindex]:not([tabindex='-1'])", "[contenteditable=true]",
      "summary", "details > summary", "label[for]",
    ];
    const all = document.querySelectorAll(selectors.join(","));
    return Array.from(all).filter((el) => {
      const rect = el.getBoundingClientRect();
      if (rect.width < 4 || rect.height < 4) return false;
      if (rect.bottom < 0 || rect.top > window.innerHeight + 200) return false;
      if (rect.right < 0 || rect.left > window.innerWidth + 200) return false;
      const style = window.getComputedStyle(el);
      if (style.display === "none" || style.visibility === "hidden") return false;
      if (parseFloat(style.opacity) < 0.1) return false;
      if (el.disabled) return false;
      return true;
    });
  }

  // ─── Element Markers (Eye System) ──────────────────

  function showMarkers() {
    removeMarkers();
    injectStyles();
    const elements = getInteractiveElements();
    elements.forEach((el, i) => {
      el.setAttribute(MARKER_ATTR, String(i));
      const rect = el.getBoundingClientRect();
      const marker = document.createElement("div");
      marker.className = OVERLAY_CLASS;
      marker.setAttribute("data-label", String(i));
      marker.style.top = `${rect.top + window.scrollY}px`;
      marker.style.left = `${rect.left + window.scrollX}px`;
      marker.style.width = `${rect.width}px`;
      marker.style.height = `${rect.height}px`;
      document.body.appendChild(marker);
    });
    markersVisible = true;
    return elements.length;
  }

  function removeMarkers() {
    document.querySelectorAll(`.${OVERLAY_CLASS}`).forEach((m) => m.remove());
    document.querySelectorAll(`[${MARKER_ATTR}]`).forEach((el) => el.removeAttribute(MARKER_ATTR));
    markersVisible = false;
  }

  // ─── Action Highlight ──────────────────────────────

  function highlightElement(el, duration = 1200) {
    if (!el) return;
    injectStyles();
    el.classList.add(HIGHLIGHT_CLASS, "yunque-pulse");
    el.scrollIntoView({ behavior: "smooth", block: "center" });
    setTimeout(() => {
      el.classList.remove(HIGHLIGHT_CLASS, "yunque-pulse");
    }, duration);
  }

  function highlightByIndex(index) {
    const el = getInteractiveElements()[index];
    if (el) highlightElement(el);
    return !!el;
  }

  function highlightBySelector(selector) {
    const el = document.querySelector(selector);
    if (el) highlightElement(el);
    return !!el;
  }

  function highlightByCoords(x, y) {
    const el = document.elementFromPoint(x, y);
    if (el) highlightElement(el);
    return !!el;
  }

  // ─── Content Extraction ────────────────────────────

  function extractPageContent() {
    const clone = document.cloneNode(true);
    for (const sel of ["script", "style", "noscript", "svg", "iframe", "nav", "footer", "header"]) {
      clone.querySelectorAll(sel).forEach((el) => el.remove());
    }

    const article = clone.querySelector("article, [role=main], main, #content, .content, .post-content, .entry-content");
    const root = article || clone.body;
    if (!root) return "";

    return domToMarkdown(root).slice(0, 50000);
  }

  function domToMarkdown(node) {
    const blocks = [];
    const walker = document.createTreeWalker(node, NodeFilter.SHOW_ELEMENT | NodeFilter.SHOW_TEXT, null);
    let current;

    while ((current = walker.nextNode())) {
      if (current.nodeType === Node.TEXT_NODE) {
        const text = current.textContent.trim();
        if (text) blocks.push(text);
        continue;
      }

      const tag = current.tagName?.toLowerCase();
      if (!tag) continue;

      const text = current.textContent?.trim() || "";

      if (tag === "h1") blocks.push(`\n# ${text}\n`);
      else if (tag === "h2") blocks.push(`\n## ${text}\n`);
      else if (tag === "h3") blocks.push(`\n### ${text}\n`);
      else if (tag === "h4") blocks.push(`\n#### ${text}\n`);
      else if (tag === "h5" || tag === "h6") blocks.push(`\n##### ${text}\n`);
      else if (tag === "p") blocks.push(`\n${text}\n`);
      else if (tag === "li") blocks.push(`- ${text}`);
      else if (tag === "blockquote") blocks.push(`> ${text}`);
      else if (tag === "pre" || tag === "code") blocks.push(`\`\`\`\n${text}\n\`\`\``);
      else if (tag === "a" && current.href) blocks.push(`[${text}](${current.href})`);
      else if (tag === "img" && current.src) blocks.push(`![${current.alt || "image"}](${current.src})`);
      else if (tag === "br") blocks.push("\n");
      else if (tag === "hr") blocks.push("\n---\n");
    }

    return blocks.join("\n").replace(/\n{3,}/g, "\n\n").trim();
  }

  function extractStructuredContent() {
    const title = document.title;
    const url = window.location.href;
    const meta = {};
    document.querySelectorAll('meta[name], meta[property]').forEach((m) => {
      const key = m.getAttribute("name") || m.getAttribute("property");
      if (key) meta[key] = m.content || "";
    });

    const headings = Array.from(document.querySelectorAll("h1,h2,h3")).map((h) => ({
      level: parseInt(h.tagName[1]),
      text: h.textContent.trim().slice(0, 200),
    }));

    const links = Array.from(document.querySelectorAll("a[href]")).slice(0, 50).map((a) => ({
      text: a.textContent.trim().slice(0, 80),
      href: a.href,
    })).filter((l) => l.text && l.href.startsWith("http"));

    const images = Array.from(document.querySelectorAll("img[src]")).slice(0, 20).map((img) => ({
      alt: img.alt || "",
      src: img.src,
      width: img.naturalWidth,
      height: img.naturalHeight,
    }));

    return { title, url, meta, headings, links, images, content: extractPageContent() };
  }

  // ─── Message Handler ───────────────────────────────

  chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
    try {
      switch (msg.type) {
        case "yunque_get_content":
          sendResponse({
            title: document.title,
            url: window.location.href,
            content: extractPageContent(),
          });
          return true;

        case "yunque_get_structured_content":
          sendResponse(extractStructuredContent());
          return true;

        case "yunque_click_index": {
          const el = getInteractiveElements()[msg.index];
          if (el) {
            highlightElement(el);
            setTimeout(() => el.click(), 300);
            sendResponse({ ok: true });
          } else {
            sendResponse({ ok: false, error: "Element not found at index " + msg.index });
          }
          return true;
        }

        case "yunque_get_elements": {
          const elements = getInteractiveElements().map((el, i) => ({
            index: i,
            tag: el.tagName.toLowerCase(),
            text: (el.textContent || "").trim().slice(0, 80),
            type: el.type || "",
            role: el.getAttribute("role") || "",
            href: el.href || "",
            ariaLabel: el.getAttribute("aria-label") || "",
            placeholder: el.placeholder || "",
            rect: (() => {
              const r = el.getBoundingClientRect();
              return { x: Math.round(r.x), y: Math.round(r.y), w: Math.round(r.width), h: Math.round(r.height) };
            })(),
          }));
          sendResponse({ elements, total: elements.length });
          return true;
        }

        case "yunque_show_markers":
          sendResponse({ ok: true, count: showMarkers() });
          return true;

        case "yunque_hide_markers":
          removeMarkers();
          sendResponse({ ok: true });
          return true;

        case "yunque_highlight": {
          let found = false;
          if (msg.index != null) found = highlightByIndex(msg.index);
          else if (msg.selector) found = highlightBySelector(msg.selector);
          else if (msg.x != null && msg.y != null) found = highlightByCoords(msg.x, msg.y);
          sendResponse({ ok: found });
          return true;
        }

        default:
          sendResponse({ ok: false, error: "unknown message type: " + msg.type });
          return true;
      }
    } catch (e) {
      sendResponse({ ok: false, error: e.message });
      return true;
    }
  });

  // ─── Operator Overlay (Manus-style) ────────────────

  const OVERLAY_ID = "yunque-operator-overlay";
  let overlayState = { visible: false, status: "idle", takeover: false };

  function injectOperatorStyles() {
    if (document.getElementById("yunque-operator-styles")) return;
    const s = document.createElement("style");
    s.id = "yunque-operator-styles";
    s.textContent = `
      #${OVERLAY_ID}-top {
        position: fixed; top: 0; left: 0; right: 0; z-index: 2147483647;
        height: 28px; display: flex; align-items: center; justify-content: space-between;
        padding: 0 12px;
        background: linear-gradient(90deg, rgba(30,27,75,0.88) 0%, rgba(49,46,129,0.88) 40%, rgba(67,56,202,0.88) 100%);
        backdrop-filter: blur(4px);
        color: #e0e7ff; font: 500 12px/28px -apple-system, "Segoe UI", sans-serif;
        box-shadow: 0 1px 4px rgba(0,0,0,0.2);
        transform: translateY(-100%); transition: transform 0.3s ease;
        user-select: none;
      }
      #${OVERLAY_ID}-top.yunque-overlay-show { transform: translateY(0); }
      #${OVERLAY_ID}-top .yunque-op-left { display: flex; align-items: center; gap: 8px; }
      #${OVERLAY_ID}-top .yunque-op-dot {
        width: 8px; height: 8px; border-radius: 50%; background: #34d399;
        animation: yunque-op-pulse 1.5s infinite;
      }
      #${OVERLAY_ID}-top .yunque-op-dot.paused { background: #fbbf24; animation: none; }
      #${OVERLAY_ID}-top .yunque-op-dot.error { background: #f87171; animation: none; }
      #${OVERLAY_ID}-top .yunque-op-cancel {
        background: #dc2626; color: #fff; border: none; border-radius: 4px;
        padding: 2px 12px; font: 500 12px/20px inherit; cursor: pointer;
        transition: background 0.15s;
      }
      #${OVERLAY_ID}-top .yunque-op-cancel:hover { background: #b91c1c; }
      @keyframes yunque-op-pulse {
        0%, 100% { opacity: 1; } 50% { opacity: 0.4; }
      }

      #${OVERLAY_ID}-bottom {
        position: fixed; bottom: 20px; left: 50%; transform: translateX(-50%) translateY(80px);
        z-index: 2147483647;
        display: flex; align-items: center; gap: 12px;
        padding: 8px 20px; border-radius: 24px;
        background: rgba(15, 23, 42, 0.92); backdrop-filter: blur(8px);
        color: #e2e8f0; font: 500 13px/1.4 -apple-system, "Segoe UI", sans-serif;
        box-shadow: 0 4px 20px rgba(0,0,0,0.4);
        transition: transform 0.3s ease, opacity 0.3s ease;
        opacity: 0; user-select: none;
      }
      #${OVERLAY_ID}-bottom.yunque-overlay-show { transform: translateX(-50%) translateY(0); opacity: 1; }
      #${OVERLAY_ID}-bottom .yunque-op-spinner {
        width: 16px; height: 16px; border: 2px solid #4f46e5;
        border-top-color: transparent; border-radius: 50%;
        animation: yunque-op-spin 0.8s linear infinite;
      }
      @keyframes yunque-op-spin { to { transform: rotate(360deg); } }
      #${OVERLAY_ID}-bottom .yunque-op-takeover {
        background: #4f46e5; color: #fff; border: none; border-radius: 16px;
        padding: 4px 16px; font: 600 12px/20px inherit; cursor: pointer;
        transition: background 0.15s;
      }
      #${OVERLAY_ID}-bottom .yunque-op-takeover:hover { background: #4338ca; }
      #${OVERLAY_ID}-bottom .yunque-op-takeover.active {
        background: #f59e0b; color: #1e1b4b;
      }
      #${OVERLAY_ID}-bottom .yunque-op-takeover.active:hover { background: #d97706; }
    `;
    document.head.appendChild(s);
  }

  function createOperatorOverlay() {
    if (document.getElementById(`${OVERLAY_ID}-top`)) return;
    injectOperatorStyles();

    const top = document.createElement("div");
    top.id = `${OVERLAY_ID}-top`;
    top.innerHTML = `
      <div class="yunque-op-left">
        <div class="yunque-op-dot"></div>
        <span class="yunque-op-text">Yunque AI 正在浏览此页面</span>
      </div>
      <button class="yunque-op-cancel">停止</button>
    `;
    document.body.appendChild(top);

    const bottom = document.createElement("div");
    bottom.id = `${OVERLAY_ID}-bottom`;
    bottom.innerHTML = `
      <div class="yunque-op-spinner"></div>
      <span class="yunque-op-status">Yunque is browsing...</span>
      <button class="yunque-op-takeover">Take over</button>
    `;
    document.body.appendChild(bottom);

    top.querySelector(".yunque-op-cancel").addEventListener("click", () => {
      chrome.runtime.sendMessage({ type: "bridge_stop_session" });
    });

    bottom.querySelector(".yunque-op-takeover").addEventListener("click", () => {
      const btn = bottom.querySelector(".yunque-op-takeover");
      if (overlayState.takeover) {
        chrome.runtime.sendMessage({ type: "bridge_resume" });
      } else {
        chrome.runtime.sendMessage({ type: "bridge_takeover", reason: "User takeover via overlay" });
      }
    });
  }

  function isLocalPage() {
    try {
      const h = window.location.hostname;
      return h === "localhost" || h === "127.0.0.1" || h === "0.0.0.0" || h === "";
    } catch (_) { return false; }
  }

  function updateOperatorOverlay(runtimeState) {
    if (!runtimeState) return;
    if (isLocalPage()) return;

    const session = runtimeState.runtimeSession;
    const isRunning = session?.status === "running";
    const isTakeover = runtimeState.takeover || session?.status === "takeover";
    const isError = session?.status === "error";
    const isActive = isRunning || isTakeover || isError;

    if (isActive && !overlayState.visible) {
      createOperatorOverlay();
      requestAnimationFrame(() => {
        const t = document.getElementById(`${OVERLAY_ID}-top`);
        const b = document.getElementById(`${OVERLAY_ID}-bottom`);
        if (t) t.classList.add("yunque-overlay-show");
        if (b) b.classList.add("yunque-overlay-show");
      });
      overlayState.visible = true;
    }

    if (!isActive && overlayState.visible) {
      const t = document.getElementById(`${OVERLAY_ID}-top`);
      const b = document.getElementById(`${OVERLAY_ID}-bottom`);
      if (t) t.classList.remove("yunque-overlay-show");
      if (b) b.classList.remove("yunque-overlay-show");
      overlayState.visible = false;
      return;
    }

    if (!isActive) return;

    const dot = document.querySelector(`#${OVERLAY_ID}-top .yunque-op-dot`);
    const text = document.querySelector(`#${OVERLAY_ID}-top .yunque-op-text`);
    const spinner = document.querySelector(`#${OVERLAY_ID}-bottom .yunque-op-spinner`);
    const status = document.querySelector(`#${OVERLAY_ID}-bottom .yunque-op-status`);
    const btn = document.querySelector(`#${OVERLAY_ID}-bottom .yunque-op-takeover`);

    if (dot) {
      dot.classList.toggle("paused", isTakeover);
      dot.classList.toggle("error", isError);
    }

    if (text) {
      if (isTakeover) text.textContent = "用户已接管浏览器";
      else if (isError) text.textContent = "Yunque AI 遇到错误";
      else text.textContent = `Yunque AI 正在浏览此页面`;
    }

    if (spinner) spinner.style.display = isTakeover ? "none" : "";

    if (status) {
      if (isTakeover) status.textContent = "已暂停 — 用户操作中";
      else status.textContent = "Yunque is browsing...";
    }

    if (btn) {
      btn.classList.toggle("active", isTakeover);
      btn.textContent = isTakeover ? "Resume AI" : "Take over";
    }

    overlayState.status = session?.status;
    overlayState.takeover = isTakeover;
  }

  // ─── State Listeners ──────────────────────────────

  chrome.runtime.onMessage.addListener((msg) => {
    if (msg?.type === "bridge_state_update") {
      emitBridgeEvent("bridge/state-update", { state: msg.state });
      updateOperatorOverlay(msg.state);
    }
  });

  requestBridgeState().then((state) => {
    emitBridgeEvent("bridge/ready", { state });
    if (state) updateOperatorOverlay(state);
  }).catch(() => {});

  injectStyles();
})();
