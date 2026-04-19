# @cloudtori/yunque-bridge

Yunque Bridge — Connect any MCP-compatible IDE to [Yunque Agent](https://github.com/nichuanfang/yunque-agent) central orchestration.

A lightweight stdio-based MCP proxy that bridges IDE tools (Cursor, Claude Code, Windsurf, etc.) to the Yunque dispatch server, letting Yunque act as the central brain to coordinate coding tasks.

## Install & Run

```bash
npx @cloudtori/yunque-bridge
```

Or install globally:

```bash
npm i -g @cloudtori/yunque-bridge
yunque-bridge
```

## IDE Configuration

### Cursor / Claude Code / Windsurf

Add to your MCP config (`mcp.json` or equivalent):

**Local (auto-discover):**

```json
{
  "mcpServers": {
    "yunque": {
      "command": "npx",
      "args": ["@cloudtori/yunque-bridge"]
    }
  }
}
```

**Remote server:**

```json
{
  "mcpServers": {
    "yunque": {
      "command": "npx",
      "args": ["@cloudtori/yunque-bridge", "-s", "http://my-server:9090", "-t", "my-token"]
    }
  }
}
```

**With environment variables:**

```json
{
  "mcpServers": {
    "yunque": {
      "command": "npx",
      "args": ["@cloudtori/yunque-bridge"],
      "env": {
        "YUNQUE_SERVER": "http://my-server:9090",
        "YUNQUE_TOKEN": "my-token"
      }
    }
  }
}
```

## Options

| Flag | Env Var | Description |
|------|---------|-------------|
| `--server, -s` | `YUNQUE_SERVER` / `YUNQUE_MCP_URL` | Yunque server URL |
| `--token, -t` | `YUNQUE_TOKEN` / `YUNQUE_API_KEY` | Auth token |
| `--version, -v` | | Show version |
| `--help, -h` | | Show help |

## How It Works

```
IDE (stdin) → yunque-bridge → HTTP POST → Yunque /mcp/v1 → response → IDE (stdout)
```

The bridge reads JSON-RPC requests from stdin, forwards them to the Yunque HTTP MCP endpoint, and writes responses back to stdout. This allows any MCP client that supports stdio transport to connect to Yunque.

## Server Discovery

When no `--server` is specified, the bridge auto-discovers a local Yunque instance by probing:

1. `http://localhost:9090/mcp/v1`
2. `http://127.0.0.1:9090/mcp/v1`
3. `http://localhost:8765/mcp/v1`

## Requirements

- Node.js >= 18.0.0
- A running Yunque Agent instance

## License

MIT
