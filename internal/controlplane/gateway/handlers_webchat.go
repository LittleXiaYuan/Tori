package gateway

import (
	"fmt"
	"net/http"
	"strings"
)

// handleWebChatWidget serves the embeddable WebChat widget.
// GET /v1/webchat/widget.js — returns the self-contained widget script.
func (g *Gateway) handleWebChatWidget(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	// Allow cross-origin embedding
	origin := r.Header.Get("Origin")
	if origin != "" {
		if g.isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
	}
	fmt.Fprint(w, webChatWidgetJS)
}

// isAllowedOrigin checks if origin is allowed by CORS config.
func (g *Gateway) isAllowedOrigin(origin string) bool {
	for _, a := range g.allowedOrigins {
		if a == "*" || strings.EqualFold(a, origin) {
			return true
		}
	}
	return false
}

// webChatWidgetJS is the self-contained embeddable webchat widget.
// Usage: <script src="https://your-tori-server/v1/webchat/widget.js" data-api-key="YOUR_KEY"></script>
const webChatWidgetJS = `
(function() {
  'use strict';

  // ── Configuration ──
  var script = document.currentScript;
  var apiKey = script.getAttribute('data-api-key') || '';
  var apiBase = script.getAttribute('data-api-base') || script.src.replace(/\/v1\/webchat\/widget\.js.*/, '');
  var title = script.getAttribute('data-title') || 'Tori Assistant';
  var placeholder = script.getAttribute('data-placeholder') || '输入消息...';
  var position = script.getAttribute('data-position') || 'bottom-right';
  var theme = script.getAttribute('data-theme') || 'light';
  var tenantID = script.getAttribute('data-tenant-id') || 'default';

  // ── Styles ──
  var css = document.createElement('style');
  css.textContent = [
    '#tori-webchat-toggle {',
    '  position: fixed;',
    '  width: 56px; height: 56px;',
    '  border-radius: 50%;',
    '  background: #4F46E5;',
    '  color: white;',
    '  border: none;',
    '  cursor: pointer;',
    '  box-shadow: 0 4px 12px rgba(0,0,0,0.2);',
    '  z-index: 99998;',
    '  display: flex; align-items: center; justify-content: center;',
    '  font-size: 24px;',
    '  transition: transform 0.2s;',
    '}',
    '#tori-webchat-toggle:hover { transform: scale(1.1); }',
    position === 'bottom-left' ? '#tori-webchat-toggle { bottom: 20px; left: 20px; }' : '#tori-webchat-toggle { bottom: 20px; right: 20px; }',
    '',
    '#tori-webchat-container {',
    '  position: fixed;',
    '  width: 380px; height: 520px;',
    '  border-radius: 12px;',
    '  box-shadow: 0 8px 32px rgba(0,0,0,0.15);',
    '  z-index: 99999;',
    '  display: none;',
    '  flex-direction: column;',
    '  overflow: hidden;',
    '  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;',
    '  font-size: 14px;',
    '}',
    position === 'bottom-left' ? '#tori-webchat-container { bottom: 86px; left: 20px; }' : '#tori-webchat-container { bottom: 86px; right: 20px; }',
    '#tori-webchat-container.open { display: flex; }',
    '',
    theme === 'dark' ? '#tori-webchat-container { background: #1e1e2e; color: #cdd6f4; }' : '#tori-webchat-container { background: #ffffff; color: #1e293b; }',
    '',
    '#tori-wc-header {',
    '  padding: 14px 16px;',
    '  font-weight: 600;',
    '  font-size: 15px;',
    '  display: flex; align-items: center; justify-content: space-between;',
    '}',
    theme === 'dark' ? '#tori-wc-header { background: #313244; color: #cdd6f4; }' : '#tori-wc-header { background: #4F46E5; color: white; }',
    '#tori-wc-header button { background: none; border: none; color: inherit; cursor: pointer; font-size: 18px; padding: 0 4px; }',
    '',
    '#tori-wc-messages {',
    '  flex: 1;',
    '  overflow-y: auto;',
    '  padding: 12px;',
    '  scroll-behavior: smooth;',
    '}',
    '#tori-wc-messages::-webkit-scrollbar { width: 4px; }',
    '#tori-wc-messages::-webkit-scrollbar-thumb { background: #94a3b8; border-radius: 2px; }',
    '',
    '.tori-wc-msg {',
    '  margin-bottom: 10px;',
    '  max-width: 85%;',
    '  padding: 8px 12px;',
    '  border-radius: 12px;',
    '  line-height: 1.5;',
    '  white-space: pre-wrap;',
    '  word-break: break-word;',
    '}',
    '.tori-wc-msg.user {',
    '  margin-left: auto;',
    '  border-bottom-right-radius: 4px;',
    '}',
    theme === 'dark' ? '.tori-wc-msg.user { background: #585b70; color: #cdd6f4; }' : '.tori-wc-msg.user { background: #4F46E5; color: white; }',
    '.tori-wc-msg.bot {',
    '  margin-right: auto;',
    '  border-bottom-left-radius: 4px;',
    '}',
    theme === 'dark' ? '.tori-wc-msg.bot { background: #313244; color: #cdd6f4; }' : '.tori-wc-msg.bot { background: #f1f5f9; color: #1e293b; }',
    '.tori-wc-msg.typing::after { content: "..."; animation: tori-dots 1.2s infinite; }',
    '@keyframes tori-dots { 0%,20% { content: "."; } 40% { content: ".."; } 60%,100% { content: "..."; } }',
    '',
    '#tori-wc-input-row {',
    '  display: flex;',
    '  padding: 10px 12px;',
    '  gap: 8px;',
    '}',
    theme === 'dark' ? '#tori-wc-input-row { border-top: 1px solid #45475a; }' : '#tori-wc-input-row { border-top: 1px solid #e2e8f0; }',
    '#tori-wc-input {',
    '  flex: 1;',
    '  border: 1px solid;',
    '  border-radius: 8px;',
    '  padding: 8px 12px;',
    '  font-size: 14px;',
    '  outline: none;',
    '  resize: none;',
    '  min-height: 36px;',
    '  max-height: 80px;',
    '  font-family: inherit;',
    '}',
    theme === 'dark' ? '#tori-wc-input { background: #313244; color: #cdd6f4; border-color: #585b70; }' : '#tori-wc-input { background: white; color: #1e293b; border-color: #cbd5e1; }',
    '#tori-wc-input:focus { border-color: #4F46E5; }',
    '#tori-wc-send {',
    '  background: #4F46E5;',
    '  color: white;',
    '  border: none;',
    '  border-radius: 8px;',
    '  padding: 8px 14px;',
    '  cursor: pointer;',
    '  font-size: 14px;',
    '  white-space: nowrap;',
    '}',
    '#tori-wc-send:disabled { opacity: 0.5; cursor: not-allowed; }',
    '#tori-wc-send:hover:not(:disabled) { background: #4338ca; }',
    '',
    '@media (max-width: 480px) {',
    '  #tori-webchat-container { width: calc(100vw - 20px); height: calc(100vh - 100px); left: 10px; right: 10px; bottom: 80px; }',
    '}'
  ].join('\n');
  document.head.appendChild(css);

  // ── DOM ──
  var toggle = document.createElement('button');
  toggle.id = 'tori-webchat-toggle';
  toggle.innerHTML = '&#128172;';
  toggle.title = title;

  var container = document.createElement('div');
  container.id = 'tori-webchat-container';
  container.innerHTML = [
    '<div id="tori-wc-header">',
    '  <span>' + escapeHTML(title) + '</span>',
    '  <button id="tori-wc-close" title="关闭">&times;</button>',
    '</div>',
    '<div id="tori-wc-messages"></div>',
    '<div id="tori-wc-input-row">',
    '  <textarea id="tori-wc-input" rows="1" placeholder="' + escapeHTML(placeholder) + '"></textarea>',
    '  <button id="tori-wc-send">发送</button>',
    '</div>'
  ].join('');

  document.body.appendChild(toggle);
  document.body.appendChild(container);

  var msgArea = document.getElementById('tori-wc-messages');
  var input = document.getElementById('tori-wc-input');
  var sendBtn = document.getElementById('tori-wc-send');
  var isOpen = false;
  var sessionID = 'wc_' + Math.random().toString(36).substring(2, 10);
  var sending = false;

  // ── Toggle ──
  toggle.addEventListener('click', function() {
    isOpen = !isOpen;
    container.classList.toggle('open', isOpen);
    if (isOpen) input.focus();
  });

  document.getElementById('tori-wc-close').addEventListener('click', function() {
    isOpen = false;
    container.classList.remove('open');
  });

  // ── Send ──
  function sendMessage() {
    var text = input.value.trim();
    if (!text || sending) return;
    sending = true;
    sendBtn.disabled = true;

    addMessage(text, 'user');
    input.value = '';
    autoResize();

    var typing = addMessage('', 'bot typing');

    fetch(apiBase + '/v1/chat', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-API-Key': apiKey,
        'X-Tenant-ID': tenantID
      },
      body: JSON.stringify({
        message: text,
        session_id: sessionID,
        stream: false
      })
    })
    .then(function(r) { return r.json(); })
    .then(function(data) {
      typing.remove();
      addMessage(data.reply || data.error || '无响应', 'bot');
    })
    .catch(function(err) {
      typing.remove();
      addMessage('连接错误: ' + err.message, 'bot');
    })
    .finally(function() {
      sending = false;
      sendBtn.disabled = false;
      input.focus();
    });
  }

  sendBtn.addEventListener('click', sendMessage);
  input.addEventListener('keydown', function(e) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  });

  // Auto-resize textarea
  function autoResize() {
    input.style.height = 'auto';
    input.style.height = Math.min(input.scrollHeight, 80) + 'px';
  }
  input.addEventListener('input', autoResize);

  // ── Helpers ──
  function addMessage(text, cls) {
    var el = document.createElement('div');
    el.className = 'tori-wc-msg ' + cls;
    if (text) el.textContent = text;
    msgArea.appendChild(el);
    msgArea.scrollTop = msgArea.scrollHeight;
    return el;
  }

  function escapeHTML(s) {
    var div = document.createElement('div');
    div.appendChild(document.createTextNode(s));
    return div.innerHTML;
  }
})();
`
