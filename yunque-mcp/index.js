#!/usr/bin/env node

/**
 * @cloudtori/yunque-bridge — stdio MCP bridge to Yunque dispatch server.
 *
 * Usage:
 *   npx @cloudtori/yunque-bridge --server http://localhost:9090
 *
 * This tool acts as a stdio-based MCP proxy: it reads JSON-RPC requests
 * from stdin, forwards them to the Yunque HTTP MCP endpoint, and writes
 * responses to stdout. This allows any MCP client that supports stdio
 * transport (Cursor, Claude Code, etc.) to connect to Yunque.
 */

import { createInterface } from "readline";

const args = process.argv.slice(2);
let serverUrl = process.env.YUNQUE_SERVER || process.env.YUNQUE_MCP_URL || "";
let authToken = process.env.YUNQUE_TOKEN || process.env.YUNQUE_API_KEY || "";

for (let i = 0; i < args.length; i++) {
  if ((args[i] === "--server" || args[i] === "-s") && args[i + 1]) {
    serverUrl = args[i + 1];
    i++;
  } else if ((args[i] === "--token" || args[i] === "-t") && args[i + 1]) {
    authToken = args[i + 1];
    i++;
  } else if (args[i] === "--version" || args[i] === "-v" || args[i] === "-V") {
    const { readFileSync } = await import("fs");
    const { fileURLToPath } = await import("url");
    const { dirname, join } = await import("path");
    const pkg = JSON.parse(readFileSync(join(dirname(fileURLToPath(import.meta.url)), "package.json"), "utf8"));
    process.stdout.write(`${pkg.version}\n`);
    process.exit(0);
  } else if (args[i] === "--help" || args[i] === "-h") {
    process.stderr.write(`yunque-bridge — Yunque MCP Bridge

Usage:
  npx @cloudtori/yunque-bridge [options]

Options:
  --server, -s <url>    Yunque server URL
  --token, -t <token>   Auth token for remote servers
  --version, -v         Show version
  --help, -h            Show this help

Environment Variables:
  YUNQUE_SERVER          Server URL (same as --server)
  YUNQUE_MCP_URL         Server URL (alternative)
  YUNQUE_TOKEN           Auth token (same as --token)
  YUNQUE_API_KEY         Auth token (alternative)

The server URL is resolved in this order:
  1. --server flag
  2. YUNQUE_SERVER env var
  3. YUNQUE_MCP_URL env var
  4. Auto-discover local instance (http://localhost:9090)

MCP Config Examples:

  Local:
  {
    "mcpServers": {
      "yunque": { "command": "npx", "args": ["@cloudtori/yunque-bridge"] }
    }
  }

  Remote:
  {
    "mcpServers": {
      "yunque": {
        "command": "npx",
        "args": ["@cloudtori/yunque-bridge", "-s", "http://my-server:8765", "-t", "my-token"]
      }
    }
  }

  With env var:
  {
    "mcpServers": {
      "yunque": {
        "command": "npx",
        "args": ["@cloudtori/yunque-bridge"],
        "env": { "YUNQUE_SERVER": "http://my-server:8765", "YUNQUE_TOKEN": "xxx" }
      }
    }
  }
`);
    process.exit(0);
  }
}

// Normalize URL
if (serverUrl) {
  serverUrl = serverUrl.replace(/\/$/, "");
  if (!serverUrl.endsWith("/mcp/v1")) serverUrl += "/mcp/v1";
} else {
  // Auto-discover: try localhost
  serverUrl = await autoDiscover();
}

process.stderr.write(`[yunque-bridge] server: ${serverUrl}\n`);
if (authToken) process.stderr.write(`[yunque-bridge] auth: token configured\n`);

async function autoDiscover() {
  const candidates = [
    "http://localhost:9090/mcp/v1",
    "http://127.0.0.1:9090/mcp/v1",
    "http://localhost:8765/mcp/v1",
  ];
  for (const url of candidates) {
    try {
      const res = await fetch(url, { method: "GET", signal: AbortSignal.timeout(2000) });
      if (res.ok) {
        process.stderr.write(`[yunque-bridge] auto-discovered: ${url}\n`);
        return url;
      }
    } catch { /* try next */ }
  }
  process.stderr.write(`[yunque-bridge] no local instance found, using default\n`);
  return "http://localhost:9090/mcp/v1";
}

const rl = createInterface({ input: process.stdin, terminal: false });
let buffer = "";

rl.on("line", async (line) => {
  line = line.trim();
  if (!line) return;

  try {
    const request = JSON.parse(line);
    const response = await forwardToServer(request);
    if (response !== null) {
      process.stdout.write(JSON.stringify(response) + "\n");
    }
  } catch (err) {
    const errorResponse = {
      jsonrpc: "2.0",
      id: null,
      error: { code: -32603, message: `bridge error: ${err.message}` },
    };
    process.stdout.write(JSON.stringify(errorResponse) + "\n");
  }
});

rl.on("close", () => {
  process.stderr.write("[yunque-bridge] stdin closed, exiting\n");
  process.exit(0);
});

async function forwardToServer(request, retries = 2) {
  try {
    const headers = { "Content-Type": "application/json" };
    if (authToken) {
      headers["Authorization"] = `Bearer ${authToken}`;
    }
    const res = await fetch(serverUrl, {
      method: "POST",
      headers,
      body: JSON.stringify(request),
      signal: AbortSignal.timeout(30000),
    });

    if (!res.ok) {
      const text = await res.text();
      return {
        jsonrpc: "2.0",
        id: request.id ?? null,
        error: { code: -32603, message: `server returned ${res.status}: ${text}` },
      };
    }

    const contentType = res.headers.get("content-type") || "";

    if (contentType.includes("application/json")) {
      return await res.json();
    }

    // Notification (no response expected)
    if (res.status === 204 || request.method?.startsWith("notifications/")) {
      return null;
    }

    const text = await res.text();
    if (text.trim()) {
      try {
        return JSON.parse(text);
      } catch {
        return {
          jsonrpc: "2.0",
          id: request.id ?? null,
          error: { code: -32603, message: `invalid server response: ${text.substring(0, 200)}` },
        };
      }
    }
    return null;
  } catch (err) {
    const cause = err.cause ? ` (${err.cause.code || err.cause.message || err.cause})` : "";
    process.stderr.write(`[yunque-bridge] fetch error: ${err.message}${cause}\n`);
    if (retries > 0 && !err.name?.includes("Abort")) {
      process.stderr.write(`[yunque-bridge] retrying... (${retries} left)\n`);
      await new Promise(r => setTimeout(r, 500));
      return forwardToServer(request, retries - 1);
    }
    return {
      jsonrpc: "2.0",
      id: request.id ?? null,
      error: { code: -32603, message: `connection failed: ${err.message}${cause}` },
    };
  }
}
