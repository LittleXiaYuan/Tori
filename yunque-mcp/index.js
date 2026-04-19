#!/usr/bin/env node

/**
 * yunque-mcp — stdio MCP bridge to Yunque dispatch server.
 *
 * Usage:
 *   npx yunque-mcp --server http://localhost:8765
 *
 * This tool acts as a stdio-based MCP proxy: it reads JSON-RPC requests
 * from stdin, forwards them to the Yunque HTTP MCP endpoint, and writes
 * responses to stdout. This allows any MCP client that supports stdio
 * transport (Cursor, Claude Code, etc.) to connect to Yunque.
 */

import { createInterface } from "readline";

const args = process.argv.slice(2);
let serverUrl = "http://localhost:8765/mcp/v1";

for (let i = 0; i < args.length; i++) {
  if (args[i] === "--server" && args[i + 1]) {
    serverUrl = args[i + 1].replace(/\/$/, "");
    if (!serverUrl.endsWith("/mcp/v1")) {
      serverUrl += "/mcp/v1";
    }
    i++;
  } else if (args[i] === "--help" || args[i] === "-h") {
    process.stderr.write(`yunque-mcp — Yunque MCP Bridge

Usage:
  npx yunque-mcp [options]

Options:
  --server <url>  Yunque server URL (default: http://localhost:8765)
  --help, -h      Show this help

MCP Config Example (Cursor / Claude Code):
  {
    "mcpServers": {
      "yunque": {
        "command": "npx",
        "args": ["yunque-mcp", "--server", "http://your-server:8765"]
      }
    }
  }
`);
    process.exit(0);
  }
}

process.stderr.write(`[yunque-mcp] connecting to ${serverUrl}\n`);

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
  process.stderr.write("[yunque-mcp] stdin closed, exiting\n");
  process.exit(0);
});

async function forwardToServer(request) {
  try {
    const res = await fetch(serverUrl, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(request),
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
    return {
      jsonrpc: "2.0",
      id: request.id ?? null,
      error: { code: -32603, message: `connection failed: ${err.message}` },
    };
  }
}
