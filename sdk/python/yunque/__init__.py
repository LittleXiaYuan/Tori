"""
Yunque Agent Plugin SDK
=======================

The official Python SDK for writing Yunque Agent plugins.
Provides access to all agent capabilities through a simple, Chrome-extension-like API.

Usage in a plugin handler script:

    import yunque

    # Call the LLM
    reply = yunque.llm("Summarize this text", text)

    # Search the web
    results = yunque.search("latest AI news", limit=5)

    # Send a message to a channel
    yunque.send("telegram", chat_id, "Hello from plugin!")

    # Read/write plugin-private memory
    yunque.memory.set("last_run", "2024-03-24")
    val = yunque.memory.get("last_run")

    # Access the knowledge base
    docs = yunque.knowledge.search("quantum computing")

    # Schedule a cron job
    yunque.cron.add("0 8 * * *", "morning_digest")

Environment variables (injected by the agent runtime):
    YUNQUE_API_BASE     - Agent API base URL (default: http://localhost:9090)
    YUNQUE_PLUGIN_TOKEN - Plugin-scoped API token (permissions limited by manifest)
    YUNQUE_PLUGIN_NAME  - Plugin identifier
    YUNQUE_PLUGIN_DIR   - Plugin directory path
"""

import json
import os
import urllib.request
import urllib.error
from typing import Any, Optional

__version__ = "0.1.0"

# ── Configuration ──

_API_BASE = os.environ.get("YUNQUE_API_BASE", "http://localhost:9090")
_TOKEN = os.environ.get("YUNQUE_PLUGIN_TOKEN", "")
_PLUGIN_NAME = os.environ.get("YUNQUE_PLUGIN_NAME", os.environ.get("PLUGIN_NAME", ""))
_PLUGIN_DIR = os.environ.get("YUNQUE_PLUGIN_DIR", os.environ.get("PLUGIN_DIR", ""))


def _api_call(method: str, path: str, body: Any = None, timeout: int = 30) -> dict:
    """Make an authenticated API call to the agent."""
    url = f"{_API_BASE}{path}"
    data = None
    if body is not None:
        data = json.dumps(body).encode("utf-8")

    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if _TOKEN:
        req.add_header("Authorization", f"Bearer {_TOKEN}")
    req.add_header("X-Plugin-Name", _PLUGIN_NAME)

    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        error_body = e.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"Yunque API error {e.code}: {error_body}") from e
    except urllib.error.URLError as e:
        raise RuntimeError(f"Yunque API connection error: {e.reason}") from e


# ── LLM ──

def llm(prompt: str, user_input: str = "", model: str = "", temperature: float = 0.7) -> str:
    """Call the agent's LLM with a system prompt and user input.

    Args:
        prompt: System prompt or instruction.
        user_input: User message (the main input to process).
        model: Optional model override (e.g. "gpt-4o", "claude-3").
        temperature: Creativity level (0-1).

    Returns:
        The LLM's response text.
    """
    body = {
        "messages": [
            {"role": "system", "content": prompt},
            {"role": "user", "content": user_input},
        ],
        "temperature": temperature,
    }
    if model:
        body["model"] = model
    resp = _api_call("POST", "/v1/plugin-api/llm", body)
    return resp.get("reply", "")


def chat(messages: list[dict], temperature: float = 0.7, model: str = "") -> str:
    """Multi-turn chat with the LLM.

    Args:
        messages: List of {"role": "system"|"user"|"assistant", "content": "..."}.
        temperature: Creativity level.
        model: Optional model override.

    Returns:
        The assistant's response text.
    """
    body = {"messages": messages, "temperature": temperature}
    if model:
        body["model"] = model
    resp = _api_call("POST", "/v1/plugin-api/llm", body)
    return resp.get("reply", "")


# ── Web Search ──

def search(query: str, limit: int = 5) -> list[dict]:
    """Search the web using the agent's configured search providers.

    Returns:
        List of {"title": str, "url": str, "snippet": str}.
    """
    resp = _api_call("POST", "/v1/plugin-api/search", {"query": query, "limit": limit})
    return resp.get("results", [])


# ── Channel Messaging ──

def send(channel_type: str, target: str, content: str, format: str = "markdown") -> bool:
    """Send a message through a channel (Telegram, Feishu, Discord, etc.).

    Args:
        channel_type: "telegram", "feishu", "discord", "slack", etc.
        target: Chat ID or user ID.
        content: Message content.
        format: "text", "markdown", or "html".

    Returns:
        True if sent successfully.
    """
    resp = _api_call("POST", "/v1/plugin-api/send", {
        "channel": channel_type,
        "target": target,
        "content": content,
        "format": format,
    })
    return resp.get("ok", False)


# ── Plugin Memory (private namespace) ──

class _MemoryNamespace:
    """Plugin-private key-value memory store."""

    def get(self, key: str, default: str = "") -> str:
        """Get a value from plugin memory."""
        resp = _api_call("POST", "/v1/plugin-api/memory/get", {"key": key})
        return resp.get("value", default)

    def set(self, key: str, value: str) -> None:
        """Set a value in plugin memory."""
        _api_call("POST", "/v1/plugin-api/memory/set", {"key": key, "value": value})

    def delete(self, key: str) -> None:
        """Delete a key from plugin memory."""
        _api_call("POST", "/v1/plugin-api/memory/delete", {"key": key})

    def list(self, prefix: str = "") -> dict[str, str]:
        """List all keys (optionally filtered by prefix)."""
        resp = _api_call("POST", "/v1/plugin-api/memory/list", {"prefix": prefix})
        return resp.get("entries", {})

    def search(self, query: str, limit: int = 10) -> list[str]:
        """Search plugin memory by content."""
        resp = _api_call("POST", "/v1/plugin-api/memory/search", {
            "query": query, "limit": limit,
        })
        return resp.get("results", [])


memory = _MemoryNamespace()


# ── Agent Memory (shared, requires memory.read/write permission) ──

class _AgentMemory:
    """Access the agent's shared memory system."""

    def search(self, query: str, top_k: int = 5) -> str:
        """Search the agent's combined memory (short+mid+long+graph+editable)."""
        resp = _api_call("POST", "/v1/plugin-api/agent-memory/search", {
            "query": query, "top_k": top_k,
        })
        return resp.get("context", "")

    def add(self, fact: str, source: str = "") -> None:
        """Add a fact to the agent's mid-term memory."""
        _api_call("POST", "/v1/plugin-api/agent-memory/add", {
            "fact": fact, "source": source or _PLUGIN_NAME,
        })


agent_memory = _AgentMemory()


# ── Knowledge Base ──

class _Knowledge:
    """Access the agent's RAG knowledge base."""

    def search(self, query: str, limit: int = 5) -> list[dict]:
        """Search the knowledge base."""
        resp = _api_call("POST", "/v1/plugin-api/knowledge/search", {
            "query": query, "limit": limit,
        })
        return resp.get("results", [])

    def ingest(self, content: str, source: str = "", filename: str = "") -> dict:
        """Ingest text content into the knowledge base."""
        resp = _api_call("POST", "/v1/plugin-api/knowledge/ingest", {
            "content": content,
            "source": source or _PLUGIN_NAME,
            "filename": filename,
        })
        return resp


knowledge = _Knowledge()


# ── Knowledge Base (/v1/knowledge) ──

class _KnowledgeBaseNamespace:
    """Lightweight helpers for the host RAG knowledge base under /v1/knowledge/*."""

    def stats(self) -> dict:
        return _api_call("GET", "/v1/knowledge/stats")

    def sources(self) -> dict:
        return _api_call("GET", "/v1/knowledge/sources")

    def search(self, query: str | dict, *, limit: int = 10, file: str = "", lang: str = "") -> dict:
        from urllib.parse import urlencode
        if isinstance(query, dict):
            q = query.get("query") or query.get("q") or ""
            limit = int(query.get("limit") or query.get("n") or limit)
            file = query.get("file") or file
            lang = query.get("lang") or lang
        else:
            q = query
        params = {"q": q}
        if limit > 0:
            params["n"] = str(limit)
        if file:
            params["file"] = file
        if lang:
            params["lang"] = lang
        return _api_call("GET", f"/v1/knowledge/search?{urlencode(params)}")

    def ingest(self, content: str | dict, *, name: str = "", trigger: str = "") -> dict:
        if isinstance(content, dict):
            body = dict(content)
        else:
            body = {"content": content}
            if name:
                body["name"] = name
            if trigger:
                body["trigger"] = trigger
        return _api_call("POST", "/v1/knowledge/ingest", body)

    def update_source(self, source_id: str | dict, *, name: str = "", trigger: str = "", content: str = "") -> dict:
        if isinstance(source_id, dict):
            body = dict(source_id)
        else:
            body = {"id": source_id}
            if name:
                body["name"] = name
            if trigger:
                body["trigger"] = trigger
            if content:
                body["content"] = content
        return _api_call("POST", "/v1/knowledge/source/update", body)

    def delete_source(self, source_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("DELETE", f"/v1/knowledge/source?{urlencode({'id': source_id})}")

    def import_url(self, url: str | dict, *, name: str = "", crawl_children: bool = False, max_pages: int = 0) -> dict:
        if isinstance(url, dict):
            body = dict(url)
        else:
            body = {"url": url}
            if name:
                body["name"] = name
            if crawl_children:
                body["crawl_children"] = crawl_children
            if max_pages > 0:
                body["max_pages"] = max_pages
        return _api_call("POST", "/v1/knowledge/import-url", body)

    def import_repo(self, path: str | dict, *, max_files: int = 0) -> dict:
        if isinstance(path, dict):
            body = dict(path)
        else:
            body = {"path": path}
            if max_files > 0:
                body["max_files"] = max_files
        return _api_call("POST", "/v1/knowledge/import-repo", body)


knowledge_base = _KnowledgeBaseNamespace()



# ── LoRA Training and Evolution (/v1/lora) ──

class _LoRANamespace:
    """Lightweight helpers for host local-brain LoRA lifecycle under /v1/lora/*."""

    def status(self) -> dict:
        return _api_call("GET", "/v1/lora/status")

    def history(self) -> dict:
        return _api_call("GET", "/v1/lora/history")

    def summary(self) -> dict:
        return _api_call("GET", "/v1/lora/summary")

    def preview(self, tenant_id: str = "") -> dict:
        from urllib.parse import urlencode
        suffix = f"?{urlencode({'tenant_id': tenant_id})}" if tenant_id else ""
        return _api_call("GET", f"/v1/lora/preview{suffix}")

    def trigger(self, tenant_id: str | dict = "") -> dict:
        body = dict(tenant_id) if isinstance(tenant_id, dict) else ({"tenant_id": tenant_id} if tenant_id else {})
        return _api_call("POST", "/v1/lora/trigger", body)

    def rollback(self) -> dict:
        return _api_call("POST", "/v1/lora/rollback", {})

    def evolution(self) -> dict:
        return _api_call("GET", "/v1/lora/evolution")

    def config(self) -> dict:
        return _api_call("GET", "/v1/lora/config")

    def update_config(self, config: dict) -> dict:
        return _api_call("PUT", "/v1/lora/config", config)


lora = _LoRANamespace()



# ── Workflow Orchestration (/v1/workflows) ──

class _WorkflowsNamespace:
    """Lightweight helpers for host DAG workflow orchestration under /v1/workflows*."""

    def list(self) -> dict:
        return _api_call("GET", "/v1/workflows")

    def get(self, workflow_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("GET", f"/v1/workflows?{urlencode({'id': workflow_id})}")

    def save(self, definition: dict) -> dict:
        return _api_call("POST", "/v1/workflows", definition)

    def delete(self, workflow_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("DELETE", f"/v1/workflows?{urlencode({'id': workflow_id})}")

    def run(self, definition_id: str | dict, variables: Optional[dict] = None) -> dict:
        if isinstance(definition_id, dict):
            body = dict(definition_id)
        else:
            body = {"definition_id": definition_id}
            if variables is not None:
                body["variables"] = variables
        return _api_call("POST", "/v1/workflows/run", body)

    def instances(self) -> dict:
        return _api_call("GET", "/v1/workflows/instances")

    def get_instance(self, instance_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("GET", f"/v1/workflows/instances?{urlencode({'id': instance_id})}")

    def cancel(self, instance_id: str | dict) -> dict:
        body = dict(instance_id) if isinstance(instance_id, dict) else {"instance_id": instance_id}
        return _api_call("POST", "/v1/workflows/cancel", body)


workflows = _WorkflowsNamespace()


# ── Connectors (/api/connectors) ──

class _ConnectorsNamespace:
    """Lightweight helpers for connector catalog, auth, and action execution."""

    def list(self) -> dict:
        return _api_call("GET", "/api/connectors")

    def detail(self, connector_id: str) -> dict:
        from urllib.parse import quote
        return _api_call("GET", f"/api/connectors/detail?id={quote(connector_id)}")

    def connect(self, connector_id: str, token: str = "", api_key: str = "") -> dict:
        return _api_call("POST", "/api/connectors/connect", {
            "connector_id": connector_id,
            "token": token,
            "api_key": api_key,
        })

    def disconnect(self, connector_id: str) -> dict:
        return _api_call("POST", "/api/connectors/disconnect", {"connector_id": connector_id})

    def execute(self, connector_id: str, action_id: str, params: Optional[dict] = None) -> dict:
        return _api_call("POST", "/api/connectors/execute", {
            "connector_id": connector_id,
            "action_id": action_id,
            "params": params or {},
        })


connectors = _ConnectorsNamespace()


# ── Notify (/api/notify) ──

class _NotifyNamespace:
    """Lightweight helpers for notification channels, tests, and share dispatch."""

    def channels(self) -> dict:
        return _api_call("GET", "/api/notify/channels")

    def add_channel(self, channel: dict) -> dict:
        return _api_call("POST", "/api/notify/add", channel)

    def remove_channel(self, channel_id: str) -> dict:
        from urllib.parse import quote
        return _api_call("POST", f"/api/notify/remove?id={quote(channel_id)}")

    def toggle_channel(self, channel_id: str, enabled: bool) -> dict:
        return _api_call("POST", "/api/notify/toggle", {"id": channel_id, "enabled": enabled})

    def test_channel(self, channel_id: str) -> dict:
        return _api_call("POST", "/api/notify/test", {"id": channel_id})

    def share(self, channel_id: str | dict, title: str = "", message: str = "", session_id: str = "", task_id: str = "", url: str = "", files: Optional[list] = None) -> dict:
        if isinstance(channel_id, dict):
            body = dict(channel_id)
        else:
            body = {
                "channel_id": channel_id,
                "title": title,
                "message": message,
                "session_id": session_id,
                "task_id": task_id,
                "url": url,
                "files": files or [],
            }
        return _api_call("POST", "/api/notify/share", body)


notify = _NotifyNamespace()


# ── MCP Dispatch (/v1/workers, /v1/dispatch) ──

class _DispatchNamespace:
    """Lightweight helpers for external MCP worker dispatch and queue control."""

    def workers(self) -> dict:
        return _api_call("GET", "/v1/workers")

    def worker(self, worker_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("GET", f"/v1/workers/detail?{urlencode({'id': worker_id})}")

    def remove_worker(self, worker_id: str) -> dict:
        return _api_call("POST", "/v1/workers/remove", {"id": worker_id})

    def queue(self) -> dict:
        return _api_call("GET", "/v1/dispatch/queue")

    def enqueue(self, task_id: str | dict, *, capabilities: Optional[list[str]] = None, priority: int = 0) -> dict:
        if isinstance(task_id, dict):
            body = dict(task_id)
        else:
            body = {"task_id": task_id}
            if capabilities is not None:
                body["capabilities"] = capabilities
            if priority:
                body["priority"] = priority
        return _api_call("POST", "/v1/dispatch/enqueue", body)

    def worker_config(self, type: str = "") -> dict:
        from urllib.parse import urlencode
        suffix = f"?{urlencode({'type': type})}" if type else ""
        return _api_call("GET", f"/v1/workers/config{suffix}")


dispatch = _DispatchNamespace()


# ── IDE Worker Orchestrator (/v1/orchestrator) ──

class _OrchestratorNamespace:
    """Lightweight helpers for IDE worker daemon status, sessions, events, and policy."""

    def status(self) -> dict:
        return _api_call("GET", "/v1/orchestrator/status")

    def toggle(self, action: str) -> dict:
        return _api_call("POST", "/v1/orchestrator/toggle", {"action": action})

    def sessions(self) -> dict:
        return _api_call("GET", "/v1/orchestrator/sessions")

    def detect_ides(self) -> dict:
        return _api_call("GET", "/v1/orchestrator/detect")

    def events(self, limit: int = 0) -> dict:
        from urllib.parse import urlencode
        suffix = f"?{urlencode({'limit': str(limit)})}" if limit > 0 else ""
        return _api_call("GET", f"/v1/orchestrator/events{suffix}")

    def task_timeline(self, task_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("GET", f"/v1/orchestrator/events/task?{urlencode({'task_id': task_id})}")

    def policy(self) -> dict:
        return _api_call("GET", "/v1/orchestrator/policy")

    def update_policy(self, policy: dict) -> dict:
        return _api_call("PUT", "/v1/orchestrator/policy", policy)

    def add_adapter(self, config: dict) -> dict:
        return _api_call("POST", "/v1/orchestrator/adapters/add", config)


orchestrator = _OrchestratorNamespace()



# ── Conversations (/v1/conversations) ──

class _ConversationsNamespace:
    """Lightweight helpers for conversation sessions, messages, metadata, and replay."""

    def list(self, *, archived: bool = False) -> dict:
        suffix = "?archived=true" if archived else ""
        return _api_call("GET", f"/v1/conversations{suffix}")

    def messages(self, session_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("GET", f"/v1/conversations/messages?{urlencode({'session_id': session_id})}")

    def delete_messages(self, session_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("DELETE", f"/v1/conversations/messages?{urlencode({'session_id': session_id})}")

    def manage(self, session_id: str, **updates) -> dict:
        return _api_call("PUT", "/v1/conversations/manage", {"session_id": session_id, **updates})

    def rename(self, session_id: str, name: str) -> dict:
        return self.manage(session_id, name=name)

    def pin(self, session_id: str, pinned: bool = True) -> dict:
        return self.manage(session_id, pinned=pinned)

    def archive(self, session_id: str, archive: bool = True) -> dict:
        return self.manage(session_id, archive=archive)

    def replay(self, session_id: str, *, raw: bool = False, limit: Optional[int] = None, offset: Optional[int] = None) -> dict:
        from urllib.parse import urlencode
        params = {"session_id": session_id}
        if raw:
            params["raw"] = "true"
        if limit is not None:
            params["limit"] = limit
        if offset is not None:
            params["offset"] = offset
        return _api_call("GET", f"/v1/conversations/replay?{urlencode(params)}")


conversations = _ConversationsNamespace()










# ── Persona identity, skills, and presets (/v1/persona*) ──



# ── Reactions and sticker send (/v1/react, /v1/sticker/send) ──

class _ReactionsNamespace:
    """Lightweight helpers for emoji reactions and sticker sending."""

    def react(self, channel_type: str, target: str, message_id: str, emoji: str = "") -> dict:
        body = {"channel_type": channel_type, "target": target, "message_id": message_id}
        if emoji:
            body["emoji"] = emoji
        return _api_call("POST", "/v1/react", body)

    def send_sticker(self, channel_type: str, target: str, package_id: str = "", sticker_id: str = "", file_id: str = "", emoji: str = "", platform: str = "") -> dict:
        body = {"channel_type": channel_type, "target": target}
        for key, value in {"package_id": package_id, "sticker_id": sticker_id, "file_id": file_id, "emoji": emoji, "platform": platform}.items():
            if value:
                body[key] = value
        return _api_call("POST", "/v1/sticker/send", body)


reactions = _ReactionsNamespace()

# ── User instructions (/v1/instructions) ──

class _InstructionsNamespace:
    """Lightweight helpers for user instruction list, create, update, delete, and reorder."""

    def list(self, category: str = "") -> dict:
        from urllib.parse import urlencode

        suffix = f"?{urlencode({'category': category})}" if category else ""
        return _api_call("GET", f"/v1/instructions{suffix}")

    def create(self, instruction: dict) -> dict:
        return _api_call("POST", "/v1/instructions", instruction)

    def update(self, instruction: dict) -> dict:
        return _api_call("PUT", "/v1/instructions", instruction)

    def delete(self, instruction_id: str) -> dict:
        from urllib.parse import urlencode

        return _api_call("DELETE", f"/v1/instructions?{urlencode({'id': instruction_id})}")

    def reorder(self, ids: list[str]) -> dict:
        return _api_call("POST", "/v1/instructions/reorder", {"ids": ids})


instructions = _InstructionsNamespace()

# ── Emotion runtime (/v1/emotion) ──

class _EmotionNamespace:
    """Lightweight helpers for emotion history and sticker mappings."""

    def history(self, session_id: str = "", limit: int = 0, from_time: str = "", to_time: str = "") -> dict:
        from urllib.parse import urlencode

        query = {k: v for k, v in {
            "session_id": session_id or None,
            "limit": limit or None,
            "from": from_time or None,
            "to": to_time or None,
        }.items() if v is not None}
        suffix = f"?{urlencode(query)}" if query else ""
        return _api_call("GET", f"/v1/emotion/history{suffix}")

    def stickers(self) -> dict:
        return _api_call("GET", "/v1/emotion/stickers")

    def register_stickers(self, platform: str, emotion: str, stickers: list[dict]) -> dict:
        return _api_call("PUT", "/v1/emotion/stickers", {"platform": platform, "emotion": emotion, "stickers": stickers})

    def clear_stickers(self, platform: str, emotion: str) -> dict:
        return _api_call("DELETE", "/v1/emotion/stickers", {"platform": platform, "emotion": emotion})


emotion = _EmotionNamespace()

class _PersonaNamespace:
    """Lightweight helpers for persona identity, skills, and preset management."""

    def get(self) -> dict:
        return _api_call("GET", "/v1/persona")

    def update(self, identity: str = "", soul: str = "") -> dict:
        body: dict[str, str] = {}
        if identity:
            body["identity"] = identity
        if soul:
            body["soul"] = soul
        return _api_call("PUT", "/v1/persona", body)

    def skills(self) -> dict:
        return _api_call("GET", "/v1/persona/skills")

    def add_skill(self, name: str, description: str = "", content: str = "") -> dict:
        return _api_call("POST", "/v1/persona/skills", {"name": name, "description": description, "content": content})

    def delete_skill(self, name: str) -> dict:
        return _api_call("DELETE", "/v1/persona/skills", {"name": name})

    def presets(self) -> dict:
        return _api_call("GET", "/v1/persona/presets")

    def switch_preset(self, preset_id: str) -> dict:
        return _api_call("POST", "/v1/persona/presets", {"id": preset_id})

    def add_custom_preset(self, preset: dict) -> dict:
        return _api_call("POST", "/v1/persona/presets/custom", preset)

    def delete_custom_preset(self, preset_id: str) -> dict:
        return _api_call("DELETE", "/v1/persona/presets/custom", {"id": preset_id})

    def update_preset_features(self, preset_id: str, features: dict[str, bool]) -> dict:
        return _api_call("PUT", "/v1/persona/presets/features", {"id": preset_id, "features": features})


persona = _PersonaNamespace()

# ── Trust governance (/api/trust, /api/review, /api/skillgrow) ──

class _TrustNamespace:
    """Lightweight helpers for trust scores, review gate status, and skill growth patterns."""

    def scores(self) -> dict:
        return _api_call("GET", "/api/trust/scores")

    def reset(self, slug: str) -> dict:
        return _api_call("POST", "/api/trust/reset", {"slug": slug})

    def grant(self, slug: str) -> dict:
        return _api_call("POST", "/api/trust/grant", {"slug": slug})

    def grant_all(self) -> dict:
        return self.grant("*")

    def review_status(self) -> dict:
        return _api_call("GET", "/api/review/status")

    def skillgrow_patterns(self) -> dict:
        return _api_call("GET", "/api/skillgrow/patterns")


trust = _TrustNamespace()

# ── Self-iteration governance (/api/iterate/*) ──

class _IterateNamespace:
    """Lightweight helpers for self-iteration proposal review and manual cycles."""

    def proposals(self, status: str = "") -> dict:
        from urllib.parse import urlencode

        query = {"status": status} if status else {}
        suffix = f"?{urlencode(query)}" if query else ""
        return _api_call("GET", f"/api/iterate/proposals{suffix}")

    def pending_proposals(self) -> dict:
        return self.proposals(status="pending")

    def approve(self, proposal_id: str) -> dict:
        return _api_call("POST", "/api/iterate/approve", {"id": proposal_id})

    def reject(self, proposal_id: str) -> dict:
        return _api_call("POST", "/api/iterate/reject", {"id": proposal_id})

    def trigger(self) -> dict:
        return _api_call("POST", "/api/iterate/trigger", {})

    def status(self) -> dict:
        return _api_call("GET", "/api/iterate/status")


iterate = _IterateNamespace()

# ── Audit chain and trail (/v1/audit, /api/audit) ──

class _AuditNamespace:
    """Lightweight helpers for Merkle audit-chain and task audit-trail reads."""

    def tail(self, n: int = 0, type: str = "", actor: str = "") -> dict:
        from urllib.parse import urlencode
        query = {k: v for k, v in {"n": n or None, "type": type or None, "actor": actor or None}.items() if v is not None}
        suffix = f"?{urlencode(query)}" if query else ""
        return _api_call("GET", f"/v1/audit/tail{suffix}")

    def verify(self) -> dict:
        return _api_call("GET", "/v1/audit/verify")

    def stats(self) -> dict:
        return _api_call("GET", "/v1/audit/stats")

    def trail(self, date: str = "", type: str = "") -> dict:
        from urllib.parse import urlencode
        query = {k: v for k, v in {"date": date or None, "type": type or None}.items() if v is not None}
        suffix = f"?{urlencode(query)}" if query else ""
        return _api_call("GET", f"/api/audit/trail{suffix}")


audit = _AuditNamespace()

# ── Tools process execution (/v1/tools/*) ──

class _ToolsNamespace:
    """Lightweight helpers for controlled tool process execution sessions."""

    def exec(self, command: str, cwd: str = "", background: bool = False, timeout_ms: int = 0, yield_ms: int = 0, env: Optional[list[str]] = None) -> dict:
        return _api_call("POST", "/v1/tools/exec", {"Command": command, "Cwd": cwd, "Background": background, "TimeoutMs": timeout_ms, "YieldMs": yield_ms, "Env": env or []})

    def list(self) -> dict:
        return _api_call("GET", "/v1/tools/list")

    def poll(self, session_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("GET", f"/v1/tools/poll?{urlencode({'id': session_id})}")

    def kill(self, session_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("POST", f"/v1/tools/kill?{urlencode({'id': session_id})}")


tools = _ToolsNamespace()

# ── Subagents (/v1/subagent) ──

class _SubagentsNamespace:
    """Lightweight helpers for subagent listing, spawning, messaging, and deletion."""

    def list(self, parent_id: str = "") -> dict:
        if parent_id:
            from urllib.parse import urlencode
            return _api_call("GET", f"/v1/subagent?{urlencode({'parent_id': parent_id})}")
        return _api_call("GET", "/v1/subagent")

    def get(self, subagent_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("GET", f"/v1/subagent?{urlencode({'id': subagent_id})}")

    def spawn(self, name: str, parent_id: str = "", description: str = "", skills: Optional[list[str]] = None) -> dict:
        return _api_call("POST", "/v1/subagent", {"parent_id": parent_id, "name": name, "description": description, "skills": skills or []})

    def destroy(self, subagent_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("DELETE", f"/v1/subagent?{urlencode({'id': subagent_id})}")

    def append_messages(self, subagent_id: str, messages: list[dict]) -> dict:
        return _api_call("POST", "/v1/subagent/message", {"id": subagent_id, "messages": messages})


subagents = _SubagentsNamespace()

# ── Runtime Queue and Events (/v1/sessions/queue, /v1/events/stream) ──

class _RuntimeNamespace:
    """Lightweight helpers for session queue inspection/cancellation and runtime events."""

    def queues(self) -> dict:
        return _api_call("GET", "/v1/sessions/queue")

    def session_queue(self, session_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("GET", f"/v1/sessions/queue?{urlencode({'id': session_id})}")

    def cancel_queued_task(self, session_id: str, task_id: str) -> dict:
        return _api_call("POST", "/v1/sessions/queue/cancel", {"session_id": session_id, "task_id": task_id})

    def events_url(self) -> str:
        return f"{_API_BASE}/v1/events/stream"

    def event_headers(self) -> dict:
        headers = {"Accept": "text/event-stream", "X-Plugin-Name": _PLUGIN_NAME}
        if _TOKEN:
            headers["Authorization"] = f"Bearer {_TOKEN}"
        return headers


runtime = _RuntimeNamespace()

# ── Browser Automation (/v1/browser, /api/browser/ext) ──

class _BrowserNamespace:
    """Lightweight helpers for browser extension automation, capture, and OPP decisions."""

    def status(self) -> dict:
        return _api_call("GET", "/v1/browser/status")

    def config(self) -> dict:
        return _api_call("GET", "/v1/browser/config")

    def navigate(self, url: str) -> dict:
        return _api_call("POST", "/v1/browser/navigate", {"url": url})

    def screenshot(self) -> dict:
        return _api_call("GET", "/v1/browser/screenshot")

    def latest_screenshot(self) -> dict:
        return _api_call("GET", "/v1/browser/screenshot/latest")

    def ocr(self) -> dict:
        return _api_call("POST", "/v1/browser/ocr", {})

    def opp_pending(self) -> dict:
        return _api_call("GET", "/v1/browser/opp/pending")

    def opp_decide(self, decision: str, *, problem_id: str = "", id: str = "") -> dict:
        body = {"decision": decision}
        if problem_id:
            body["problem_id"] = problem_id
        if id:
            body["id"] = id
        return _api_call("POST", "/v1/browser/opp/decide", body)

    def extension_status(self) -> dict:
        return _api_call("GET", "/api/browser/ext/status")

    def extension_session(self) -> dict:
        return _api_call("POST", "/api/browser/ext/session", {})

    def extension_action(self, action: dict) -> dict:
        return _api_call("POST", "/api/browser/ext/action", action)

    def scenarios(self) -> dict:
        return _api_call("GET", "/api/browser/ext/scenarios")

    def run_scenario(self, scenario_id: str) -> dict:
        return _api_call("POST", "/api/browser/ext/scenarios/run", {"scenario_id": scenario_id})


browser = _BrowserNamespace()

# ── Files (/api/files) ──

class _FilesNamespace:
    """Lightweight helpers for agent output file listing, previews, and downloads."""

    def list(self, path: str = "") -> dict:
        from urllib.parse import urlencode
        suffix = f"?{urlencode({'path': path})}" if path else ""
        return _api_call("GET", f"/api/files{suffix}")

    def preview(self, path: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("GET", f"/api/files/preview?{urlencode({'path': path})}")

    def download(self, path: str):
        from urllib.parse import urlencode
        return _api_call("GET", f"/api/files/download?{urlencode({'path': path})}")


files = _FilesNamespace()

# ── RBAC (/v1/rbac) ──

class _RBACNamespace:
    """Lightweight helpers for role-based access control roles, bindings, and checks."""

    def roles(self) -> dict:
        return _api_call("GET", "/v1/rbac/roles")

    def create_role(self, role: dict) -> dict:
        return _api_call("POST", "/v1/rbac/roles", role)

    def delete_role(self, role_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("DELETE", f"/v1/rbac/roles?{urlencode({'id': role_id})}")

    def assign_role(self, subject_id: str, role_id: str, tenant_id: str = "") -> dict:
        body = {"subject_id": subject_id, "role_id": role_id}
        if tenant_id:
            body["tenant_id"] = tenant_id
        return _api_call("POST", "/v1/rbac/assign", body)

    def revoke_role(self, subject_id: str, role_id: str, tenant_id: str = "") -> dict:
        body = {"subject_id": subject_id, "role_id": role_id}
        if tenant_id:
            body["tenant_id"] = tenant_id
        return _api_call("POST", "/v1/rbac/revoke", body)

    def check(self, resource: str, action: str, *, subject_id: str = "", tenant_id: str = "") -> dict:
        body = {"resource": resource, "action": action}
        if subject_id:
            body["subject_id"] = subject_id
        if tenant_id:
            body["tenant_id"] = tenant_id
        return _api_call("POST", "/v1/rbac/check", body)

    def my_roles(self) -> dict:
        return _api_call("GET", "/v1/rbac/my-roles")


rbac = _RBACNamespace()


# ── Permissions facade (/v1/rbac/check, /v1/rbac/my-roles) ──

class _PermissionsNamespace:
    """Lightweight permission-check facade over the RBAC slice."""

    def check(self, resource: str, action: str, *, subject_id: str = "", tenant_id: str = "") -> dict:
        return rbac.check(resource, action, subject_id=subject_id, tenant_id=tenant_id)

    def my_roles(self) -> dict:
        return rbac.my_roles()


permissions = _PermissionsNamespace()

# ── Approvals (/v1/approvals) ──

class _ApprovalsNamespace:
    """Lightweight helpers for human-in-the-loop approval queues and rules."""

    def list(self, *, status: str = "", history: bool = False) -> dict:
        from urllib.parse import urlencode
        params = {}
        if status != "":
            params["status"] = status
        if history:
            params["history"] = "true"
        suffix = f"?{urlencode(params)}" if params else ""
        return _api_call("GET", f"/v1/approvals{suffix}")

    def pending(self) -> dict:
        return self.list(status="pending")

    def history(self, status: str = "") -> dict:
        return self.list(status=status, history=True)

    def approve(self, approval_id: str) -> dict:
        return _api_call("POST", "/v1/approvals/approve", {"id": approval_id})

    def deny(self, approval_id: str, reason: str = "") -> dict:
        body = {"id": approval_id}
        if reason:
            body["reason"] = reason
        return _api_call("POST", "/v1/approvals/deny", body)

    def decide(self, approval_id: str, decision: str) -> dict:
        return _api_call("POST", "/v1/approvals/decide", {"id": approval_id, "decision": decision})

    def rules(self) -> dict:
        return _api_call("GET", "/v1/approvals/rules")

    def add_rule(self, rule: dict) -> dict:
        return _api_call("POST", "/v1/approvals/rules", rule)

    def delete_rule(self, rule_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("DELETE", f"/v1/approvals/rules?{urlencode({'id': rule_id})}")


approvals = _ApprovalsNamespace()

# ── Conversation Forks (/v1/fork) ──

class _ForkNamespace:
    """Lightweight helpers for conversation root forks, branches, and branch lists."""

    def root(self, session_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("GET", f"/v1/fork?{urlencode({'session_id': session_id})}")

    def get(self, fork_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("GET", f"/v1/fork?{urlencode({'id': fork_id})}")

    def create(self, session_id: str | dict, messages: Optional[list[dict]] = None) -> dict:
        if isinstance(session_id, dict):
            body = dict(session_id)
        else:
            body = {"session_id": session_id}
            if messages is not None:
                body["messages"] = messages
        return _api_call("POST", "/v1/fork", body)

    def remove(self, fork_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("DELETE", f"/v1/fork?{urlencode({'id': fork_id})}")

    def branch(self, fork_id: str | dict, at_index: int = 0, label: str = "") -> dict:
        if isinstance(fork_id, dict):
            body = dict(fork_id)
        else:
            body = {"fork_id": fork_id, "at_index": at_index}
            if label:
                body["label"] = label
        return _api_call("POST", "/v1/fork/branch", body)

    def list(self, session_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("GET", f"/v1/fork/list?{urlencode({'session_id': session_id})}")


fork = _ForkNamespace()


# ── Providers / Models (/api/providers, /v1/models) ──

class _ProvidersNamespace:
    """Lightweight helpers for LLM provider registry, runtime mode, models, local discovery, and breaker reset."""

    def models(self) -> dict:
        return _api_call("GET", "/v1/models")

    def add_model(self, model: dict) -> dict:
        return _api_call("POST", "/v1/models", model)

    def delete_model(self, model_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("DELETE", f"/v1/models?{urlencode({'id': model_id})}")

    def list(self) -> dict:
        return _api_call("GET", "/api/providers")

    def test(self, provider_id: str) -> dict:
        return _api_call("POST", "/api/providers/test", {"id": provider_id})

    def enable(self, provider_id: str) -> dict:
        return _api_call("POST", "/api/providers/enable", {"id": provider_id})

    def disable(self, provider_id: str) -> dict:
        return _api_call("POST", "/api/providers/disable", {"id": provider_id})

    def switch_model(self, provider_id: str, model: str) -> dict:
        return _api_call("POST", "/api/providers/switch-model", {"id": provider_id, "model": model})

    def set_session(self, session_id: str, provider_id: str = "") -> dict:
        return _api_call("POST", "/api/providers/session", {"session_id": session_id, "provider_id": provider_id})

    def mode(self) -> dict:
        return _api_call("GET", "/api/providers/mode")

    def set_mode(self, mode: str) -> dict:
        return _api_call("POST", "/api/providers/mode", {"mode": mode})

    def presets(self) -> dict:
        return _api_call("GET", "/api/providers/presets")

    def register(self, config: dict) -> dict:
        return _api_call("POST", "/api/providers/register", config)

    def delete(self, provider_id: str) -> dict:
        return _api_call("POST", "/api/providers/delete", {"id": provider_id})

    def discover_local(self, base_url: str | dict) -> dict:
        body = dict(base_url) if isinstance(base_url, dict) else {"base_url": base_url}
        return _api_call("POST", "/api/providers/local/discover", body)

    def register_local(self, base_url: str | dict, *, model: str = "", tier: str = "", backend: str = "") -> dict:
        if isinstance(base_url, dict):
            body = dict(base_url)
        else:
            body = {"base_url": base_url}
            if model:
                body["model"] = model
            if tier:
                body["tier"] = tier
            if backend:
                body["backend"] = backend
        return _api_call("POST", "/api/providers/local/register", body)

    def discover_tori(self, auto_register: bool = False) -> dict:
        suffix = "?auto_register=true" if auto_register else ""
        return _api_call("POST", f"/api/providers/tori/discover{suffix}")

    def exec(self) -> dict:
        return _api_call("GET", "/api/providers/exec")

    def set_exec(self, provider_id: str) -> dict:
        return _api_call("POST", "/api/providers/exec", {"provider_id": provider_id})

    def reset_breakers(self) -> dict:
        return _api_call("POST", "/api/breaker/reset")


providers = _ProvidersNamespace()


# ── Cognis / Cognitive Kernel (/v1/cognis) ──

class _CognisNamespace:
    """Lightweight helpers for Cogni registry, traces, experience, evolution, and federation."""

    def list(self) -> dict:
        return _api_call("GET", "/v1/cognis")

    def create(self, declaration: dict) -> dict:
        return _api_call("POST", "/v1/cognis", declaration)

    def get(self, cogni_id: str) -> dict:
        return _api_call("GET", f"/v1/cognis/{cogni_id}")

    def remove(self, cogni_id: str) -> dict:
        return _api_call("DELETE", f"/v1/cognis/{cogni_id}")

    def enable(self, cogni_id: str) -> dict:
        return _api_call("POST", f"/v1/cognis/{cogni_id}/enable")

    def disable(self, cogni_id: str) -> dict:
        return _api_call("POST", f"/v1/cognis/{cogni_id}/disable")

    def reload(self) -> dict:
        return _api_call("POST", "/v1/cognis/reload")

    def traces(self, limit: Optional[int] = None) -> dict:
        from urllib.parse import urlencode
        suffix = f"?{urlencode({'limit': limit})}" if limit is not None else ""
        return _api_call("GET", f"/v1/cognis/traces{suffix}")

    def trace(self, cogni_id: str, limit: Optional[int] = None) -> dict:
        from urllib.parse import urlencode
        suffix = f"?{urlencode({'limit': limit})}" if limit is not None else ""
        return _api_call("GET", f"/v1/cognis/{cogni_id}/trace{suffix}")

    def stats(self) -> dict:
        return _api_call("GET", "/v1/cognis/stats")

    def health(self, cogni_id: str = "") -> dict:
        path = f"/v1/cognis/{cogni_id}/health" if cogni_id else "/v1/cognis/health"
        return _api_call("GET", path)

    def verify(self, cogni_id: str = "") -> dict:
        path = f"/v1/cognis/{cogni_id}/verify" if cogni_id else "/v1/cognis/verify"
        return _api_call("GET", path)

    def alerts(self) -> dict:
        return _api_call("GET", "/v1/cognis/alerts")

    def scan_alerts(self) -> dict:
        return _api_call("POST", "/v1/cognis/alerts/scan")

    def generate(self, request: dict) -> dict:
        return _api_call("POST", "/v1/cognis/generate", request)

    def export_bundle(self) -> dict:
        return _api_call("GET", "/v1/cognis/export")

    def import_bundle(self, bundle: dict) -> dict:
        return _api_call("POST", "/v1/cognis/import", bundle)

    def workflows(self, cogni_id: str) -> dict:
        return _api_call("GET", f"/v1/cognis/{cogni_id}/workflows")

    def run_workflow(self, cogni_id: str, workflow: str, request: Optional[dict] = None) -> dict:
        return _api_call("POST", f"/v1/cognis/{cogni_id}/workflow/{workflow}", request or {})

    def experience(self, cogni_id: str) -> dict:
        return _api_call("GET", f"/v1/cognis/{cogni_id}/experience")

    def record_experience(self, cogni_id: str, record_type: str, data: dict) -> dict:
        return _api_call("POST", f"/v1/cognis/{cogni_id}/experience/record", {"type": record_type, "data": data})

    def confirm_experience_pattern(self, cogni_id: str, pattern_id: str) -> dict:
        return _api_call("POST", f"/v1/cognis/{cogni_id}/experience/patterns/{pattern_id}/confirm")

    def evolve(self, cogni_id: str, request: Optional[dict] = None) -> dict:
        return _api_call("POST", f"/v1/cognis/{cogni_id}/evolve", request or {})

    def evolution(self, cogni_id: str = "") -> dict:
        path = f"/v1/cognis/{cogni_id}/evolution" if cogni_id else "/v1/cognis/evolution"
        return _api_call("GET", path)

    def federation(self) -> dict:
        return _api_call("GET", "/v1/cognis/federation")

    def federation_peers(self) -> dict:
        return _api_call("GET", "/v1/cognis/federation/peers")

    def discover_federation(self, request: Optional[dict] = None) -> dict:
        return _api_call("POST", "/v1/cognis/federation/discover", request or {})

    def expose(self, cogni_id: str) -> dict:
        return _api_call("POST", f"/v1/cognis/{cogni_id}/expose")

    def unexpose(self, cogni_id: str) -> dict:
        return _api_call("POST", f"/v1/cognis/{cogni_id}/unexpose")

    def economics(self) -> dict:
        return _api_call("GET", "/v1/cognis/economics")


cognis = _CognisNamespace()


# ── Execution Trace / Audit Replay (/v1/trace) ──

class _TraceNamespace:
    """Lightweight helpers for recent, trace-id, and task-id execution trace reads."""

    def recent(self, limit: Optional[int] = None, raw: bool = False) -> dict:
        from urllib.parse import urlencode
        query = {}
        if limit is not None:
            query["limit"] = limit
        if raw:
            query["raw"] = "true"
        suffix = f"?{urlencode(query)}" if query else ""
        return _api_call("GET", f"/v1/trace/recent{suffix}")

    def by_trace_id(self, trace_id: str, raw: bool = False) -> dict:
        from urllib.parse import quote, urlencode
        suffix = f"?{urlencode({'raw': 'true'})}" if raw else ""
        return _api_call("GET", f"/v1/trace/{quote(trace_id, safe='')}{suffix}")

    def by_task_id(self, task_id: str, raw: bool = False) -> dict:
        from urllib.parse import quote, urlencode
        suffix = f"?{urlencode({'raw': 'true'})}" if raw else ""
        return _api_call("GET", f"/v1/trace/task/{quote(task_id, safe='')}{suffix}")


trace = _TraceNamespace()


# ── Proactive Heartbeat Lifecycle (/v1/heartbeat) ──

class _HeartbeatNamespace:
    """Lightweight helpers for proactive lifecycle heartbeat status, control, trigger, and logs."""

    def status(self) -> dict:
        return _api_call("GET", "/v1/heartbeat")

    def update(self, enabled: Optional[bool] = None, interval_minutes: Optional[int] = None) -> dict:
        body: dict[str, Any] = {}
        if enabled is not None:
            body["enabled"] = enabled
        if interval_minutes is not None:
            body["interval_minutes"] = interval_minutes
        return _api_call("PUT", "/v1/heartbeat", body)

    def trigger(self) -> dict:
        return _api_call("POST", "/v1/heartbeat/trigger", {})

    def logs(self, limit: Optional[int] = None) -> list[dict]:
        from urllib.parse import urlencode
        suffix = f"?{urlencode({'limit': limit})}" if limit is not None else ""
        result = _api_call("GET", f"/v1/heartbeat/logs{suffix}")
        return result if isinstance(result, list) else []


heartbeat = _HeartbeatNamespace()




# ── Events SSE Stream (/v1/events/stream) ──

class _EventsNamespace:
    """Lightweight helpers for Server-Sent Events stream parsing and raw connections."""

    def stream_url(self) -> str:
        return f"{_API_BASE}/v1/events/stream"

    def headers(self) -> dict:
        headers = {"Accept": "text/event-stream", "X-Plugin-Name": _PLUGIN_NAME}
        if _TOKEN:
            headers["Authorization"] = f"Bearer {_TOKEN}"
        return headers

    def parse(self, text: str) -> list[dict]:
        events: list[dict] = []
        for raw in text.replace("\r\n", "\n").split("\n\n"):
            if not raw.strip():
                continue
            event = "message"
            data: list[str] = []
            event_id = ""
            retry: Optional[int] = None
            for line in raw.split("\n"):
                if not line or line.startswith(":"):
                    continue
                field, _, value = line.partition(":")
                value = value[1:] if value.startswith(" ") else value
                if field == "event":
                    event = value
                elif field == "data":
                    data.append(value)
                elif field == "id":
                    event_id = value
                elif field == "retry":
                    try:
                        retry = int(value)
                    except ValueError:
                        retry = None
            if event == "message" and not data and not event_id and retry is None:
                continue
            payload: Any = "\n".join(data) if data else None
            if isinstance(payload, str):
                try:
                    payload = json.loads(payload)
                except json.JSONDecodeError:
                    pass
            item = {"event": event, "data": payload, "raw": raw}
            if event_id:
                item["id"] = event_id
            if retry is not None:
                item["retry"] = retry
            events.append(item)
        return events


events = _EventsNamespace()

# ── Reverie Proactive Thought Loop (/v1/reverie) ──

class _ReverieNamespace:
    """Lightweight helpers for Reverie journal, stats, config, think, actions, and targets."""

    def journal(self, category: Optional[str] = None, min_significance: Optional[float] = None, delivered: Optional[bool] = None, limit: Optional[int] = None, offset: Optional[int] = None) -> dict:
        from urllib.parse import urlencode
        query: dict[str, Any] = {}
        if category:
            query["category"] = category
        if min_significance is not None:
            query["min_significance"] = min_significance
        if delivered is not None:
            query["delivered"] = str(delivered).lower()
        if limit is not None:
            query["limit"] = limit
        if offset is not None:
            query["offset"] = offset
        suffix = f"?{urlencode(query)}" if query else ""
        return _api_call("GET", f"/v1/reverie/journal{suffix}")

    def stats(self) -> dict:
        return _api_call("GET", "/v1/reverie/stats")

    def config(self) -> dict:
        return _api_call("GET", "/v1/reverie/config")

    def update_config(self, config: dict) -> dict:
        return _api_call("PUT", "/v1/reverie/config", config)

    def think(self, event_type: str = "", trigger: str = "") -> dict:
        body: dict[str, Any] = {}
        if event_type:
            body["event_type"] = event_type
        if trigger:
            body["trigger"] = trigger
        return _api_call("POST", "/v1/reverie/think", body)

    def delete_thought(self, thought_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("DELETE", f"/v1/reverie/thought?{urlencode({'id': thought_id})}")

    def actions(self) -> dict:
        return _api_call("GET", "/v1/reverie/actions")

    def targets(self) -> dict:
        return _api_call("GET", "/v1/reverie/targets")


reverie = _ReverieNamespace()

# ── Realtime WebSocket Chat (/v1/ws) ──

class _RealtimeNamespace:
    """Lightweight helpers for /v1/ws URL construction and ping/chat messages."""

    def ws_url(self, *, token: str = "", api_key: str = "", query: Optional[dict] = None) -> str:
        from urllib.parse import urlencode, urlparse, urlunparse

        parsed = urlparse(_API_BASE.rstrip("/") + "/v1/ws")
        scheme = {"http": "ws", "https": "wss"}.get(parsed.scheme, parsed.scheme)
        if scheme not in ("ws", "wss"):
            raise ValueError(f"Unsupported realtime base URL protocol: {parsed.scheme}")
        params = {str(k): v for k, v in (query or {}).items() if v is not None and v != ""}
        if not any(k in params for k in ("key", "api_key", "token", "access_token")):
            selected_api_key = api_key or os.environ.get("YUNQUE_API_KEY", "")
            selected_token = token or _TOKEN
            if selected_api_key:
                params["api_key"] = selected_api_key
            elif selected_token:
                params["access_token"] = selected_token
        return urlunparse((scheme, parsed.netloc, parsed.path, "", urlencode(params), ""))

    def ping(self, **extra) -> dict:
        return {"type": "ping", **extra}

    def chat(self, content: str, *, session: str = "", **extra) -> dict:
        message = {"type": "chat", "content": content, **extra}
        if session:
            message["session"] = session
        return message

    def serialize(self, message: dict) -> str:
        return json.dumps(message, ensure_ascii=False)

    def parse(self, data: str | bytes) -> dict:
        if isinstance(data, bytes):
            data = data.decode("utf-8")
        parsed = json.loads(data)
        if not isinstance(parsed, dict):
            raise ValueError("Realtime message must be an object")
        return parsed


realtime = _RealtimeNamespace()

# ── Chat Runtime (/v1/chat, /v1/chat/stream, /v1/chat/agentic) ──

class _ChatNamespace:
    """Lightweight helpers for basic, streaming, and agentic chat endpoints."""

    def send(self, messages: list[dict], **extra) -> dict:
        return _api_call("POST", "/v1/chat", {"messages": messages, **extra})

    def agentic(self, messages: list[dict], **extra) -> dict:
        return _api_call("POST", "/v1/chat/agentic", {"messages": messages, **extra})

    def stream_url(self) -> str:
        return f"{_API_BASE}/v1/chat/stream"

    def stream_request(self, messages: list[dict], **extra) -> dict:
        return {"messages": messages, "stream": True, **extra}

    def parse_stream(self, text: str) -> list[dict]:
        parsed_events = events.parse(text)
        out: list[dict] = []
        for event in parsed_events:
            raw = event.get("raw_data", event.get("data"))
            if raw == "[DONE]":
                continue
            item = {"event": event.get("event", "message"), "raw": raw}
            data = event.get("data")
            if isinstance(data, dict):
                item["data"] = data
                if data.get("type") == "delta" or "content" in data:
                    item["kind"] = "delta"
                    item["content"] = data.get("content", "")
                elif data.get("type") == "error" or "error" in data:
                    item["kind"] = "error"
                    item["message"] = data.get("error") or data.get("message") or str(raw)
                else:
                    item["kind"] = item["event"]
            else:
                item["data"] = data
                item["kind"] = item["event"] or "raw"
            out.append(item)
        return out


chat_sdk = _ChatNamespace()

# ── Cost / Usage / Quota (/v1/cost, /v1/usage, /v1/quota) ──

class _CostNamespace:
    """Lightweight helpers for cost summary, budget, task cost, history, alerts, usage, and quota."""

    def summary(self) -> dict:
        return _api_call("GET", "/v1/cost/summary")

    def set_budget(self, budget: dict) -> dict:
        return _api_call("POST", "/v1/cost/budget", budget)

    def task(self, task_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("GET", f"/v1/cost/task?{urlencode({'id': task_id})}")

    def task_timeline(self, task_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("GET", f"/v1/cost/task/timeline?{urlencode({'id': task_id})}")

    def breakdown(self) -> dict:
        return _api_call("GET", "/v1/cost/breakdown")

    def history(self, **query) -> dict:
        from urllib.parse import urlencode
        params = {k: v for k, v in query.items() if v is not None and v != ""}
        suffix = f"?{urlencode(params)}" if params else ""
        return _api_call("GET", f"/v1/cost/history{suffix}")

    def alerts(self) -> dict:
        return _api_call("GET", "/v1/cost/alerts")

    def usage(self) -> dict:
        return _api_call("GET", "/v1/usage")

    def set_quota(self, quota: dict, tenant_id: str = "") -> dict:
        body = dict(quota) if "quota" in quota else {"quota": quota}
        if tenant_id:
            body["tenant_id"] = tenant_id
        return _api_call("POST", "/v1/quota", body)


cost = _CostNamespace()


# ── Skill Market (/v1/market) ──

class _SkillMarketNamespace:
    """Lightweight helpers for skill marketplace search, ranking, and stats."""

    def search(self, query: str = "") -> dict:
        from urllib.parse import urlencode
        suffix = f"?{urlencode({'q': query})}" if query else ""
        return _api_call("GET", f"/v1/market/search{suffix}")

    def top(self, *, n: int = 0, by: str = "") -> dict:
        from urllib.parse import urlencode
        params = {}
        if n > 0:
            params["n"] = str(n)
        if by:
            params["by"] = by
        suffix = f"?{urlencode(params)}" if params else ""
        return _api_call("GET", f"/v1/market/top{suffix}")

    def stats(self) -> dict:
        return _api_call("GET", "/v1/market/stats")


market = _SkillMarketNamespace()


# ── Projects (/v1/projects) ──

class _ProjectsNamespace:
    """Lightweight helpers for project workspace CRUD under /v1/projects*."""

    def list(self) -> dict:
        return _api_call("GET", "/v1/projects")

    def create(self, name: str | dict, repo_path: str = "", *, repo_url: str = "", description: str = "", default_caps: Optional[list[str]] = None, meta: Optional[dict[str, str]] = None) -> dict:
        if isinstance(name, dict):
            body = dict(name)
        else:
            body = {"name": name, "repo_path": repo_path}
            if repo_url:
                body["repo_url"] = repo_url
            if description:
                body["description"] = description
            if default_caps is not None:
                body["default_caps"] = default_caps
            if meta is not None:
                body["meta"] = meta
        return _api_call("POST", "/v1/projects", body)

    def detail(self, project_id: str) -> dict:
        from urllib.parse import urlencode
        return _api_call("GET", f"/v1/projects/detail?{urlencode({'id': project_id})}")

    def update(self, project_id: str, patch: dict) -> dict:
        from urllib.parse import urlencode
        return _api_call("PUT", f"/v1/projects/detail?{urlencode({'id': project_id})}", patch)

    def remove(self, project_id: str) -> dict:
        return _api_call("POST", "/v1/projects/remove", {"id": project_id})


projects = _ProjectsNamespace()


# ── Cron / Scheduling ──

class _Cron:
    """Schedule periodic tasks."""

    def add(self, expr: str, name: str, message: str = "") -> dict:
        """Add a cron job.

        Args:
            expr: Cron expression (e.g. "0 8 * * *" for daily at 8am).
            name: Job name.
            message: What the agent should do when triggered.

        Returns:
            {"id": str, "status": "created"}.
        """
        resp = _api_call("POST", "/v1/plugin-api/cron/add", {
            "expression": expr,
            "name": f"{_PLUGIN_NAME}:{name}",
            "message": message,
        })
        return resp

    def remove(self, job_id: str) -> bool:
        """Remove a cron job."""
        resp = _api_call("POST", "/v1/plugin-api/cron/remove", {"id": job_id})
        return resp.get("ok", False)

    def list(self) -> list[dict]:
        """List all cron jobs created by this plugin."""
        resp = _api_call("GET", f"/v1/plugin-api/cron/list?plugin={_PLUGIN_NAME}")
        return resp.get("jobs", [])


cron = _Cron()


# ── Cron System (/v1/cron) ──

class _CronSystemNamespace:
    """Lightweight helpers for host scheduled tasks under /v1/cron/*."""

    def list(self) -> dict:
        """List host cron jobs."""
        return _api_call("GET", "/v1/cron/list")

    def add(self, name: str, schedule: dict, payload: dict) -> dict:
        """Add a host cron job using the /v1/cron/add shape."""
        return _api_call("POST", "/v1/cron/add", {
            "name": name,
            "schedule": schedule,
            "payload": payload,
        })

    def remove(self, job_id: str) -> dict:
        """Remove a host cron job by id."""
        from urllib.parse import urlencode
        return _api_call("POST", f"/v1/cron/remove?{urlencode({'id': job_id})}")

    def run(self, job_id: str) -> dict:
        """Run a host cron job immediately."""
        from urllib.parse import urlencode
        return _api_call("POST", f"/v1/cron/run?{urlencode({'id': job_id})}")


cron_system = _CronSystemNamespace()


# ── Memory Kernel (/v1/memory) ──

class _MemoryCoreNamespace:
    """Lightweight helpers for the host recall memory layer under /v1/memory/*."""

    def stats(self) -> dict:
        """Return host memory counters and layer statistics."""
        return _api_call("GET", "/v1/memory/stats")

    def search(self, query: str | dict, *, limit: int = 10, layer: str = "") -> dict:
        """Search host recall memory. Pass either a query string or a complete body."""
        if isinstance(query, dict):
            body = dict(query)
        else:
            body = {"query": query, "limit": limit}
            if layer:
                body["layer"] = layer
        return _api_call("POST", "/v1/memory/search", body)

    def add(self, value: str | dict = "", *, key: str = "", layer: str = "mid",
            source: str = "", tags: Optional[list[str]] = None) -> dict:
        """Add a fact to host recall memory using the /v1/memory/add shape."""
        if isinstance(value, dict):
            body = dict(value)
            if "value" not in body and "content" in body:
                body["value"] = body["content"]
        else:
            body = {"value": value, "layer": layer}
            if key:
                body["key"] = key
            if source:
                body["source"] = source
            if tags:
                body["tags"] = tags
        return _api_call("POST", "/v1/memory/add", body)

    def remember(self, content: str, *, layer: str = "mid", source: str = "",
                 tags: Optional[list[str]] = None) -> dict:
        """Compact alias for adding a text fact to host recall memory."""
        return self.add(content, layer=layer, source=source, tags=tags)

    def compact(self, *, target_count: int = 0, decay_days: int = 0) -> dict:
        """Run memory compaction with optional target/decay hints."""
        body = {}
        if target_count > 0:
            body["target_count"] = target_count
        if decay_days > 0:
            body["decay_days"] = decay_days
        return _api_call("POST", "/v1/memory/compact", body)


memory_core = _MemoryCoreNamespace()


# ── Knowledge Graph (/v1/graph) ──

class _GraphNamespace:
    """Lightweight helpers for the host knowledge graph under /v1/graph/*."""

    def entities(self, query: str = "") -> dict:
        """List or search graph entities."""
        from urllib.parse import urlencode
        suffix = f"?{urlencode({'q': query})}" if query else ""
        return _api_call("GET", f"/v1/graph/entities{suffix}")

    def put_entity(self, entity: dict) -> dict:
        """Create or update a graph entity."""
        return _api_call("POST", "/v1/graph/entities", entity)

    def delete_entity(self, entity_id: str) -> dict:
        """Delete a graph entity by id."""
        from urllib.parse import urlencode
        return _api_call("DELETE", f"/v1/graph/entities?{urlencode({'id': entity_id})}")

    def relations(self, entity_id: str = "") -> dict:
        """List all relations or relations for one entity."""
        from urllib.parse import urlencode
        suffix = f"?{urlencode({'entity_id': entity_id})}" if entity_id else ""
        return _api_call("GET", f"/v1/graph/relations{suffix}")

    def put_relation(self, relation: dict) -> dict:
        """Create or update a graph relation."""
        return _api_call("POST", "/v1/graph/relations", relation)

    def context_by_entity_id(self, entity_id: str) -> dict:
        """Return context and neighbors for an entity id."""
        from urllib.parse import urlencode
        return _api_call("GET", f"/v1/graph/context?{urlencode({'entity_id': entity_id})}")

    def context_by_name(self, name: str) -> dict:
        """Return context and neighbors for an entity name."""
        from urllib.parse import urlencode
        return _api_call("GET", f"/v1/graph/context?{urlencode({'name': name})}")

    def stats(self) -> dict:
        """Return graph entity/relation counters."""
        return _api_call("GET", "/v1/graph/stats")


graph = _GraphNamespace()


# ── Reflection Experience ──

class _ReflectNamespace:
    """Lightweight helpers for the agent reflection/experience layer.

    Reflection is exposed as a small SDK surface so external scripts can reuse
    lessons and strategy hints without importing platform internals.
    """

    def experiences(self, *, q: str = "", source: str = "", category: str = "",
                    outcome: str = "", tag: str = "", limit: int = 0) -> dict:
        """List captured reflection experiences with optional filters."""
        return _api_call("GET", f"/v1/reflect/experiences{_reflect_query(q, source, category, outcome, tag, limit)}")

    def stats(self, *, source: str = "", category: str = "",
              outcome: str = "", tag: str = "") -> dict:
        """Return reflection experience counters for the same filter set."""
        return _api_call("GET", f"/v1/reflect/experiences{_reflect_query('', source, category, outcome, tag, 0, stats=True)}")

    def strategies(self, *, q: str = "", source: str = "", category: str = "",
                   outcome: str = "", tag: str = "", limit: int = 0) -> str:
        """Return compiled strategy hints derived from reflection experiences."""
        resp = _api_call("GET", f"/v1/reflect/strategies{_reflect_query(q, source, category, outcome, tag, limit)}")
        return resp.get("strategies", "")


def _reflect_query(q: str = "", source: str = "", category: str = "",
                   outcome: str = "", tag: str = "", limit: int = 0,
                   stats: bool = False) -> str:
    from urllib.parse import urlencode

    params = {}
    if q:
        params["q"] = q
    if source:
        params["source"] = source
    if category:
        params["category"] = category
    if outcome:
        params["outcome"] = outcome
    if tag:
        params["tag"] = tag
    if limit > 0:
        params["limit"] = str(limit)
    if stats:
        params["stats"] = "true"
    return f"?{urlencode(params)}" if params else ""


reflect = _ReflectNamespace()


# ── Mission Parse ──

class _MissionsNamespace:
    """Lightweight helpers for natural-language mission parsing.

    Mission parsing is exposed as a small SDK slice so external pages,
    plugins, CLIs, and automation scripts can turn user intent into a typed
    task/workflow/cron/trigger draft without importing platform internals.
    """

    def parse(self, description: str) -> dict:
        """Parse a natural-language mission description into a structured draft."""
        return _api_call("POST", "/v1/missions/parse", {"description": description})


missions = _MissionsNamespace()


# ── Prompt Scheduler ──

class _SchedulerNamespace:
    """Lightweight helpers for prompt-based recurring scheduler jobs."""

    def jobs(self) -> dict:
        """List scheduler jobs from /v1/scheduler/jobs."""
        return _api_call("GET", "/v1/scheduler/jobs")

    def add(self, name: str, prompt: str, interval: str) -> dict:
        """Add a recurring prompt job. Interval uses Go duration strings such as '1h'."""
        return _api_call("POST", "/v1/scheduler/add", {
            "name": name,
            "prompt": prompt,
            "interval": interval,
        })

    def remove(self, job_id: str) -> dict:
        """Remove a scheduler job by id."""
        return _api_call("POST", "/v1/scheduler/remove", {"id": job_id})


scheduler = _SchedulerNamespace()


# ── Trigger Automation ──

class _TriggersNamespace:
    """Lightweight helpers for Triggers v2 automation definitions and events."""

    def list(self, *, tenant_id: str = "", type: str = "", status: str = "") -> dict:
        """List Triggers v2 definitions with optional tenant/type/status filters."""
        from urllib.parse import urlencode

        params = {}
        if tenant_id:
            params["tenant_id"] = tenant_id
        if type:
            params["type"] = type
        if status:
            params["status"] = status
        query = f"?{urlencode(params)}" if params else ""
        return _api_call("GET", f"/v1/triggers/v2{query}")

    def get(self, trigger_id: str) -> dict:
        """Get one Triggers v2 definition by id."""
        from urllib.parse import urlencode
        return _api_call("GET", f"/v1/triggers/v2?{urlencode({'id': trigger_id})}")

    def create(self, definition: dict) -> dict:
        """Create a Triggers v2 definition."""
        return _api_call("POST", "/v1/triggers/v2", definition)

    def update(self, definition: dict) -> dict:
        """Update a Triggers v2 definition."""
        return _api_call("PUT", "/v1/triggers/v2", definition)

    def delete(self, trigger_id: str) -> dict:
        """Delete a Triggers v2 definition by id."""
        from urllib.parse import urlencode
        return _api_call("DELETE", f"/v1/triggers/v2?{urlencode({'id': trigger_id})}")

    def emit(self, event: str | dict, *, text: str = "", data: Optional[dict] = None, timestamp: str = "") -> dict:
        """Emit a trigger event. Pass either an event string or a complete payload dict."""
        if isinstance(event, dict):
            payload = dict(event)
        else:
            payload = {"event": event}
            if text:
                payload["text"] = text
            if data is not None:
                payload["data"] = data
            if timestamp:
                payload["timestamp"] = timestamp
        return _api_call("POST", "/v1/triggers/v2/emit", payload)

    def runs(self, *, trigger_id: str = "", limit: int = 0) -> dict:
        """List recent trigger runs."""
        return _api_call("GET", f"/v1/triggers/v2/runs{_trigger_history_query(trigger_id, limit)}")

    def events(self, *, trigger_id: str = "", limit: int = 0) -> dict:
        """List recent trigger events."""
        return _api_call("GET", f"/v1/triggers/v2/events{_trigger_history_query(trigger_id, limit)}")


def _trigger_history_query(trigger_id: str = "", limit: int = 0) -> str:
    from urllib.parse import urlencode

    params = {}
    if trigger_id:
        params["trigger_id"] = trigger_id
    if limit > 0:
        params["limit"] = str(limit)
    return f"?{urlencode(params)}" if params else ""


triggers = _TriggersNamespace()


# ── System Extension Registration ──
# These let plugins ADD new system-level capabilities to the agent.
# Like Magisk modules or Chrome extensions — you're extending the platform itself.

def register_provider(id: str, base_url: str, model: str, *,
                      api_keys: list[str] = None, tier: str = "",
                      provider_type: str = "chat") -> dict:
    """Register a new LLM provider (Ollama, vLLM, Claude, etc.).

    The provider must serve an OpenAI-compatible API.

    Example:
        yunque.register_provider("ollama", "http://localhost:11434/v1", "llama3")
    """
    body = {"id": id, "base_url": base_url, "model": model, "type": provider_type}
    if api_keys:
        body["api_keys"] = api_keys
    if tier:
        body["tier"] = tier
    return _api_call("POST", "/v1/plugin-api/register/provider", body)


def register_channel(name: str, webhook_url: str, send_endpoint: str, *,
                     display_name: str = "", config: dict = None) -> dict:
    """Register a new messaging channel adapter (Matrix, IRC, custom webhook, etc.).

    Args:
        name: Channel type identifier (e.g. "matrix").
        webhook_url: Your plugin's endpoint for receiving messages from the channel.
        send_endpoint: Your plugin's endpoint for the agent to send messages through.
    """
    body = {"name": name, "webhook_url": webhook_url, "send_endpoint": send_endpoint}
    if display_name:
        body["display_name"] = display_name
    if config:
        body["config_json"] = json.dumps(config)
    return _api_call("POST", "/v1/plugin-api/register/channel", body)


def register_search(name: str, base_url: str, *, api_key: str = "",
                    search_path: str = "/search") -> dict:
    """Register a new web search engine."""
    return _api_call("POST", "/v1/plugin-api/register/search", {
        "name": name, "base_url": base_url, "api_key": api_key,
        "search_path": search_path,
    })


def register_guardrail(name: str, description: str, *, phase: str = "both",
                       keywords: list[str] = None, patterns: list[str] = None) -> dict:
    """Register a new safety guardrail rule.

    Args:
        phase: "input" (check user messages), "output" (check agent replies), "both".
        keywords: Block messages containing these keywords.
        patterns: Block messages matching these regex patterns.
    """
    body = {"name": name, "description": description, "phase": phase}
    if keywords:
        body["keywords"] = keywords
    if patterns:
        body["patterns"] = patterns
    return _api_call("POST", "/v1/plugin-api/register/guardrail", body)


def register_embedding(name: str, base_url: str, model: str, *,
                       api_key: str = "", dimensions: int = 0) -> dict:
    """Register a new vector embedding provider."""
    body = {"name": name, "base_url": base_url, "model": model}
    if api_key:
        body["api_key"] = api_key
    if dimensions > 0:
        body["dimensions"] = dimensions
    return _api_call("POST", "/v1/plugin-api/register/embedding", body)


def register_speech(name: str, speech_type: str, base_url: str, *,
                    model: str = "", voice: str = "", api_key: str = "") -> dict:
    """Register a new TTS or STT engine.

    Args:
        speech_type: "tts" (text-to-speech) or "stt" (speech-to-text).
    """
    body = {"name": name, "type": speech_type, "base_url": base_url}
    if model:
        body["model"] = model
    if voice:
        body["voice"] = voice
    if api_key:
        body["api_key"] = api_key
    return _api_call("POST", "/v1/plugin-api/register/speech", body)


def list_extensions() -> list[dict]:
    """List all plugin-contributed system extensions."""
    resp = _api_call("GET", "/v1/plugin-api/extensions")
    return resp.get("extensions", [])


# ── System Info ──

def version() -> dict:
    """Get the agent's version info."""
    return _api_call("GET", "/v1/version")


def info() -> dict:
    """Get plugin runtime info."""
    return {
        "plugin_name": _PLUGIN_NAME,
        "plugin_dir": _PLUGIN_DIR,
        "api_base": _API_BASE,
        "sdk_version": __version__,
        "authenticated": bool(_TOKEN),
    }

# ── State Kernel (lightweight state layer) ──

class _StateNamespace:
    """Typed-enough helpers for the agent State Kernel.

    This namespace is intentionally small: external plugins, Python scripts, and
    sidecars can read/write the state layer without importing the generated full
    OpenAPI client or platform internals.
    """

    def snapshot(self) -> dict:
        """Return the full State Kernel snapshot from /v1/state."""
        return _api_call("GET", "/v1/state")

    def actions(self) -> list[dict]:
        """Return recent action records from the State Kernel snapshot."""
        return self.snapshot().get("recent_actions") or []

    def capabilities(self) -> dict:
        """Return capability summary from the State Kernel snapshot."""
        return self.snapshot().get("capabilities") or {}

    def goals(self) -> list[dict]:
        """List goals tracked by the State Kernel."""
        result = _api_call("GET", "/v1/state/goals")
        return result if isinstance(result, list) else []

    def save_goal(self, goal: dict) -> dict:
        """Create or update a State Kernel goal."""
        return _api_call("POST", "/v1/state/goals", goal)

    def focus(self) -> str:
        """Return the current focus string."""
        return _api_call("GET", "/v1/state/focus").get("focus", "")

    def resources(self) -> list[dict]:
        """List active resources tracked by the State Kernel."""
        result = _api_call("GET", "/v1/state/resources")
        return result if isinstance(result, list) else []


state = _StateNamespace()

# ── Agent Kit bundle ──

class AgentKit:
    """Small bundle of common SDK-first Yunque surfaces.

    Use this when an external Python script, plugin, or sidecar wants the State
    Kernel, Reflection Experience, Mission Parse, Scheduler, Cron System, Triggers, and Plugin API
    Runtime helpers from one object without depending on a generated full
    OpenAPI client. The namespace objects are the same lightweight module-level
    helpers, so this remains a zero-dependency incremental package.
    """

    def __init__(self):
        self.state = state
        self.reflect = reflect
        self.missions = missions
        self.scheduler = scheduler
        self.cron_system = cron_system
        self.triggers = triggers
        self.memory_core = memory_core
        self.graph = graph
        self.knowledge_base = knowledge_base
        self.lora = lora
        self.workflows = workflows
        self.connectors = connectors
        self.notify = notify
        self.projects = projects
        self.market = market
        self.dispatch = dispatch
        self.orchestrator = orchestrator
        self.fork = fork
        self.cost = cost
        self.providers = providers
        self.cognis = cognis
        self.trace = trace
        self.heartbeat = heartbeat
        self.events = events
        self.reverie = reverie
        self.realtime = realtime
        self.chat = chat_sdk
        self.conversations = conversations
        self.approvals = approvals
        self.rbac = rbac
        self.files = files
        self.browser = browser
        self.runtime = runtime
        self.subagents = subagents
        self.tools = tools
        self.audit = audit
        self.trust = trust
        self.iterate = iterate
        self.persona = persona
        self.emotion = emotion
        self.instructions = instructions
        self.reactions = reactions
        self.permissions = permissions
        self.plugin = plugin
        self.memory = memory
        self.agent_memory = agent_memory
        self.knowledge = knowledge
        self.cron = cron


class _PluginRuntimeNamespace:
    """Grouped Plugin API Runtime helpers for AgentKit users."""

    def llm(self, prompt: str, user_input: str = "", model: str = "", temperature: float = 0.7) -> str:
        return llm(prompt, user_input, model, temperature)

    def chat(self, messages: list[dict], temperature: float = 0.7, model: str = "") -> str:
        return chat(messages, temperature, model)

    def search(self, query: str, limit: int = 5) -> list[dict]:
        return search(query, limit)

    def send(self, channel_type: str, target: str, content: str, format: str = "markdown") -> bool:
        return send(channel_type, target, content, format)

    def register_provider(self, *args, **kwargs) -> dict:
        return register_provider(*args, **kwargs)

    def register_channel(self, *args, **kwargs) -> dict:
        return register_channel(*args, **kwargs)

    def register_search(self, *args, **kwargs) -> dict:
        return register_search(*args, **kwargs)

    def register_guardrail(self, *args, **kwargs) -> dict:
        return register_guardrail(*args, **kwargs)

    def register_embedding(self, *args, **kwargs) -> dict:
        return register_embedding(*args, **kwargs)

    def register_speech(self, *args, **kwargs) -> dict:
        return register_speech(*args, **kwargs)

    def list_extensions(self) -> list[dict]:
        return list_extensions()


plugin = _PluginRuntimeNamespace()

def create_agent_kit() -> AgentKit:
    """Return a lightweight bundle of state, reflection, and plugin runtime helpers."""
    return AgentKit()

