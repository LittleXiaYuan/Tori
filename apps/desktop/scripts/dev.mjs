#!/usr/bin/env node
/**
 * One-command desktop dev launcher.
 *
 * Why this exists: the desktop dev stack is three processes —
 *   next dev (:3001, in apps/web)  →  tauri dev  →  Go sidecar (:9090)
 * tauri.conf.json has no `beforeDevCommand`, so `tauri dev` only WAITS for the
 * frontend on :3001; nobody actually starts it. Forgetting to start next dev
 * in a separate terminal is the #1 cause of "Waiting for frontend dev server"
 * hangs, and repeated manual kill/restart cycles leave zombie next-dev (node)
 * processes and stale :3001 / :9090 listeners that fight each other and produce
 * intermittent, baffling failures.
 *
 * This launcher owns the whole lifecycle:
 *   1. preflight — free :3001 and :9090, killing any stale dev process holding them
 *   2. start next dev (apps/web) and wait until :3001 actually accepts connections
 *   3. run `tauri dev` (which itself spawns + reaps the Go sidecar)
 *   4. on Ctrl+C / exit — tear next dev down so no zombie survives
 *
 * The Go sidecar's own lifecycle is handled by the Rust side (src-tauri/lib.rs
 * spawns it with AGENT_ADDR=127.0.0.1:<port> and reaps it on Drop), so we only
 * have to manage next dev here.
 *
 * Usage: npm run dev:all   (from apps/desktop)
 */
import { spawn, execSync } from "child_process";
import path from "path";
import { fileURLToPath } from "url";
import net from "net";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const DESKTOP_DIR = path.resolve(__dirname, "..");
const REPO_ROOT = path.resolve(DESKTOP_DIR, "..", "..");
const WEB_DIR = path.join(REPO_ROOT, "apps", "web");
const NEXT_PORT = 3001;
const BACKEND_PORT = 9090;
const isWin = process.platform === "win32";

const c = {
  info: (m) => console.log(`\x1b[36m[dev]\x1b[0m ${m}`),
  warn: (m) => console.warn(`\x1b[33m[dev]\x1b[0m ${m}`),
  err: (m) => console.error(`\x1b[31m[dev]\x1b[0m ${m}`),
};

// ── preflight: free ports / kill stale dev processes ─────────────────────────

function pidsOnPort(port) {
  try {
    if (isWin) {
      const out = execSync("netstat -ano -p tcp", { encoding: "utf8" });
      const pids = new Set();
      for (const line of out.split(/\r?\n/)) {
        const m = line.match(/\s\S+:(\d+)\s+\S+\s+LISTENING\s+(\d+)/);
        if (m && Number(m[1]) === port) pids.add(m[2]);
      }
      return [...pids];
    }
    const out = execSync(`lsof -ti tcp:${port} -s tcp:LISTEN`, { encoding: "utf8" });
    return out.split(/\s+/).filter(Boolean);
  } catch {
    return [];
  }
}

function killPid(pid) {
  try {
    if (isWin) execSync(`taskkill /PID ${pid} /T /F`, { stdio: "ignore" });
    else process.kill(Number(pid), "SIGKILL");
  } catch {
    /* already gone */
  }
}

/**
 * Kill stale instances by process name. This MUST run before freeing ports:
 * while the desktop app is alive it keeps respawning the Go sidecar and holds
 * an OS lock on the sidecar binary. Only freeing the port leaves the app
 * running — it refills the port and keeps the binary locked, which then makes
 * `tauri-build` fail with PermissionDenied (Windows file lock). So we kill the
 * app (and any orphan sidecar) by name first, then free ports as a backstop.
 */
function killByName(names) {
  for (const name of names) {
    try {
      if (isWin) execSync(`taskkill /IM ${name} /T /F`, { stdio: "ignore" });
      else execSync(`pkill -f ${name}`, { stdio: "ignore" });
    } catch {
      /* none running */
    }
  }
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

function freePort(port, label) {
  const pids = pidsOnPort(port);
  if (pids.length === 0) return;
  c.warn(`port ${port} (${label}) held by pid(s) ${pids.join(", ")} — killing stale process`);
  pids.forEach(killPid);
}

function waitForPort(port, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  return new Promise((resolve, reject) => {
    const tryOnce = () => {
      const sock = net.connect({ host: "127.0.0.1", port }, () => {
        sock.destroy();
        resolve();
      });
      sock.on("error", () => {
        sock.destroy();
        if (Date.now() > deadline) reject(new Error(`timeout waiting for :${port}`));
        else setTimeout(tryOnce, 400);
      });
    };
    tryOnce();
  });
}

// ── lifecycle ────────────────────────────────────────────────────────────────

const children = [];
let shuttingDown = false;

function shutdown(code) {
  if (shuttingDown) return;
  shuttingDown = true;
  for (const child of children) {
    if (!child || child.killed || child.exitCode !== null) continue;
    try {
      if (isWin && child.pid) execSync(`taskkill /PID ${child.pid} /T /F`, { stdio: "ignore" });
      else child.kill("SIGTERM");
    } catch {
      /* already gone */
    }
  }
  process.exit(code ?? 0);
}

process.on("SIGINT", () => shutdown(0));
process.on("SIGTERM", () => shutdown(0));

async function main() {
  c.info("preflight: clearing stale desktop / sidecar processes and freeing ports");
  killByName(isWin ? ["yunque-desktop.exe", "yunque-agent.exe"] : ["yunque-desktop", "yunque-agent"]);
  freePort(NEXT_PORT, "next dev");
  freePort(BACKEND_PORT, "go sidecar");
  // Give Windows a moment to release the file lock on the sidecar binary so the
  // upcoming copy-sidecar / tauri-build step can overwrite and read it.
  await sleep(700);

  c.info(`starting next dev on :${NEXT_PORT} (apps/web)`);
  const next = spawn("npm", ["run", "dev"], {
    cwd: WEB_DIR,
    stdio: "inherit",
    shell: true,
  });
  children.push(next);
  next.on("exit", (code) => {
    if (!shuttingDown) {
      c.err(`next dev exited (code ${code}) — tearing down`);
      shutdown(code ?? 1);
    }
  });

  try {
    await waitForPort(NEXT_PORT, 90_000);
  } catch (e) {
    c.err(`frontend never came up on :${NEXT_PORT}: ${e.message}`);
    shutdown(1);
    return;
  }
  c.info(`frontend ready on :${NEXT_PORT} — launching tauri dev`);

  // copy-sidecar first (mirrors the existing `predev` hook), then tauri dev.
  const tauri = spawn("npm", ["run", "dev"], {
    cwd: DESKTOP_DIR,
    stdio: "inherit",
    shell: true,
  });
  children.push(tauri);
  tauri.on("exit", (code) => {
    c.info(`tauri dev exited (code ${code})`);
    shutdown(code ?? 0);
  });
}

main().catch((e) => {
  c.err(e.stack || String(e));
  shutdown(1);
});
