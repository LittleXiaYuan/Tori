use std::process::{Child, Command};
use std::sync::Mutex;
use std::time::{Duration, Instant};

use serde::Serialize;
use tauri::{AppHandle, Emitter, Manager};
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpStream as TokioTcpStream;

// ── Tunables ────────────────────────────────────────────────────────────────
const DEFAULT_BACKEND_PORT: u16 = 9090;
const HEALTH_TIMEOUT: Duration = Duration::from_secs(60);
const POLL_INTERVAL: Duration = Duration::from_millis(500);
const HTTP_PROBE_TIMEOUT: Duration = Duration::from_millis(1_500);
const PROGRESS_TICK: Duration = Duration::from_millis(500);

/// How long we give the Go sidecar to finish its own graceful shutdown
/// (DB flush, HTTP server Shutdown, plugin cleanup) before we SIGKILL it.
/// The Go side uses GracefulShutdownTimeout = 15s in cmd/agent/constants.go,
/// so 20s here comfortably covers that plus a little extra slack.
const GRACEFUL_TIMEOUT: Duration = Duration::from_secs(20);
const GRACEFUL_POLL: Duration = Duration::from_millis(200);

/// Windows CreateProcess flag: start the child in its own process group so
/// that GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT) targets only the sidecar
/// and not this Tauri host.
#[cfg(windows)]
const CREATE_NEW_PROCESS_GROUP: u32 = 0x0000_0200;

// Event names — kept in one place so the loader HTML and main frontend
// can rely on a stable contract.
const EVT_STATUS: &str = "backend:status";
const EVT_READY: &str = "backend:ready";
const EVT_ERROR: &str = "backend:error";

// ── Event payloads ──────────────────────────────────────────────────────────
#[derive(Serialize, Clone)]
struct StatusPayload<'a> {
    /// "searching" | "spawning" | "waiting" | "probing"
    phase: &'a str,
    message: &'a str,
    elapsed_ms: u128,
    /// 0..=100 — suggested bar fill for the loader UI.
    progress: u8,
    port: u16,
}

#[derive(Serialize, Clone)]
struct ReadyPayload {
    port: u16,
    elapsed_ms: u128,
}

#[derive(Serialize, Clone)]
struct ErrorPayload<'a> {
    message: &'a str,
    port: u16,
}

// ── Sidecar lifecycle ───────────────────────────────────────────────────────
struct BackendState {
    child: Mutex<Option<Child>>,
}

impl BackendState {
    /// Politely ask the sidecar to shut down, then fall back to SIGKILL if
    /// it overruns the grace window. Safe to call repeatedly — `take()`
    /// empties the slot so subsequent calls are no-ops.
    ///
    /// NOTE: This runs from the sync `on_window_event` callback, so the
    /// inner wait uses `std::thread::sleep`. Do NOT call `graceful_kill`
    /// from a tokio task without `spawn_blocking`; the blocking sleep
    /// would stall the tokio runtime.
    fn graceful_kill(&self) {
        let mut guard = lock_or_recover(&self.child);
        let Some(mut child) = guard.take() else {
            return;
        };
        let pid = child.id();
        log::info!("requesting graceful shutdown (pid={pid})");

        let signaled = send_graceful_signal(pid);

        if signaled && wait_with_timeout(&mut child, GRACEFUL_TIMEOUT) {
            log::info!("backend exited gracefully (pid={pid})");
            return;
        }

        log::warn!(
            "graceful shutdown {} — forcing kill (pid={pid})",
            if signaled { "timed out" } else { "unavailable" }
        );
        let _ = child.kill();
        let _ = child.wait();
    }
}

/// Acquire a Mutex guard, recovering from poisoning so a panic on the
/// spawn path can never strand the Go child process.
fn lock_or_recover<T>(m: &Mutex<T>) -> std::sync::MutexGuard<'_, T> {
    match m.lock() {
        Ok(g) => g,
        Err(poisoned) => {
            log::error!("backend state mutex poisoned; recovering");
            poisoned.into_inner()
        }
    }
}

// ── Entry point ─────────────────────────────────────────────────────────────
#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        // File + stderr logging. On Windows release builds there is no
        // console, so the file target is what you grep when users report
        // "it won't open". Log dir resolves to the platform app-log path.
        .plugin(
            tauri_plugin_log::Builder::new()
                .level(log::LevelFilter::Info)
                .targets([
                    tauri_plugin_log::Target::new(tauri_plugin_log::TargetKind::Stderr),
                    tauri_plugin_log::Target::new(tauri_plugin_log::TargetKind::LogDir {
                        file_name: Some("desktop".into()),
                    }),
                ])
                .build(),
        )
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_process::init())
        .manage(BackendState {
            child: Mutex::new(None),
        })
        .setup(|app| {
            let handle = app.handle().clone();
            tauri::async_runtime::spawn(async move {
                launch_backend(&handle).await;
            });
            Ok(())
        })
        .on_window_event(|window, event| match event {
            // macOS typically fires CloseRequested but not Destroyed when a
            // user hits the red traffic-light button — listen to both so the
            // Go sidecar never outlives its host window.
            tauri::WindowEvent::CloseRequested { .. } | tauri::WindowEvent::Destroyed => {
                if let Some(state) = window.try_state::<BackendState>() {
                    state.graceful_kill();
                }
            }
            _ => {}
        })
        .run(tauri::generate_context!())
        .expect("failed to run yunque desktop");
}

// ── Main launcher ───────────────────────────────────────────────────────────
async fn launch_backend(handle: &AppHandle) {
    let port = resolve_backend_port();
    let start = Instant::now();

    emit_status(handle, "searching", "定位后端二进制…", 2, port, start);

    let data_dir = if let Ok(app_data) = handle.path().app_data_dir() {
        let d = app_data.join("data");
        let _ = std::fs::create_dir_all(&d);
        log::info!("data dir: {}", d.display());
        Some(d)
    } else {
        log::warn!("could not resolve app data dir");
        None
    };

    // Tauri v2 `externalBin` drops the `-<triple>` suffix when bundling,
    // so the binary lands next to the main executable on Windows/Linux and
    // inside `Contents/MacOS/` on macOS. In dev the Tauri CLI ALSO drops
    // the suffix. That makes a single canonical filename per OS sufficient.
    let bin_name: &str = if cfg!(windows) {
        "yunque-agent.exe"
    } else {
        "yunque-agent"
    };

    let search_dirs = candidate_dirs(handle);
    log::info!(
        "looking for {bin_name} in: {:?}",
        search_dirs
            .iter()
            .map(|d| d.display().to_string())
            .collect::<Vec<_>>()
    );

    let backend_path = search_dirs
        .iter()
        .map(|d| d.join(bin_name))
        .find(|p| p.exists());

    let mut sidecar_started = false;

    match backend_path {
        Some(ref bin_path) => {
            emit_status(handle, "spawning", "启动 Go 后端…", 8, port, start);
            log::info!("launching backend: {}", bin_path.display());
            let mut cmd = Command::new(bin_path);
            cmd.env("OPEN_BROWSER", "false")
                .env("HIDE_CONSOLE", "true");
            if let Some(ref dd) = data_dir {
                cmd.env("YUNQUE_DATA_DIR", dd.to_string_lossy().to_string());
            }

            // Desktop is a local-only app: force loopback binding so the
            // backend's fail-closed "production-like" heuristic doesn't reject
            // startup because of a weak/empty JWT_SECRET on first run. Respect
            // an operator override if AGENT_ADDR is already in the parent env.
            if std::env::var_os("AGENT_ADDR").is_none() {
                cmd.env("AGENT_ADDR", format!("127.0.0.1:{port}"));
            }
            // Tag the process so Go-side warnings can tell this is the GUI
            // wrapper rather than a headless server.
            cmd.env("YUNQUE_LAUNCHER", "tauri-desktop");

            // On Windows the sidecar MUST live in its own process group so
            // GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT) does not also kill us.
            #[cfg(windows)]
            {
                use std::os::windows::process::CommandExt;
                cmd.creation_flags(CREATE_NEW_PROCESS_GROUP);
            }

            match cmd.spawn() {
                Ok(child) => {
                    log::info!("backend process started (pid={})", child.id());
                    let state = handle.state::<BackendState>();
                    *lock_or_recover(&state.child) = Some(child);
                    sidecar_started = true;
                }
                Err(e) => {
                    let msg = format!("failed to start backend: {e}");
                    log::error!("{msg}");
                    emit_error(handle, &msg, port);
                    return;
                }
            }
        }
        None => {
            emit_status(
                handle,
                "waiting",
                "未找到内嵌二进制，等待外部后端…",
                5,
                port,
                start,
            );
            log::warn!("sidecar binary not found; waiting for external backend on port {port}");
        }
    }

    emit_status(handle, "probing", "等待健康信号…", 15, port, start);

    let backend_ready = wait_for_healthy(handle, port, start).await;

    if backend_ready {
        log::info!(
            "backend healthy on port {port} ({:.1}s) — loader page will auto-navigate",
            start.elapsed().as_secs_f64()
        );
        if let Err(e) = handle.emit(
            EVT_READY,
            ReadyPayload {
                port,
                elapsed_ms: start.elapsed().as_millis(),
            },
        ) {
            log::debug!("emit {EVT_READY} failed: {e}");
        }
    } else if sidecar_started {
        // We started a sidecar but it never came up — don't leave a zombie
        // Go process fighting for the port while the user sees the timeout
        // screen.
        log::warn!("killing unhealthy sidecar to avoid orphan process");
        emit_error(handle, "后端启动超时，正在回收进程", port);
        let state = handle.state::<BackendState>();
        state.graceful_kill();
    } else {
        emit_error(handle, "未检测到后端服务", port);
    }
}

/// Parse `AGENT_ADDR` (e.g. `":9090"`, `"0.0.0.0:9090"`, `"[::]:9090"`,
/// `"localhost:9090"`). Falls back to `DEFAULT_BACKEND_PORT` on any
/// parse error — we prefer a live probe against the default over refusing
/// to launch.
fn resolve_backend_port() -> u16 {
    let Ok(raw) = std::env::var("AGENT_ADDR") else {
        return DEFAULT_BACKEND_PORT;
    };
    let trimmed = raw.trim();
    if trimmed.is_empty() {
        return DEFAULT_BACKEND_PORT;
    }
    // Last `:` segment is the port in every supported form, incl. IPv6.
    let tail = trimmed.rsplit(':').next().unwrap_or("");
    match tail.parse::<u16>() {
        Ok(p) if p != 0 => p,
        _ => {
            log::warn!(
                "AGENT_ADDR={raw:?} unparseable, falling back to {DEFAULT_BACKEND_PORT}"
            );
            DEFAULT_BACKEND_PORT
        }
    }
}

/// Build the ordered list of directories where the sidecar binary might
/// live, covering dev, Tauri v2 bundle, and macOS app-bundle layouts.
fn candidate_dirs(handle: &AppHandle) -> Vec<std::path::PathBuf> {
    let mut dirs: Vec<std::path::PathBuf> = Vec::new();
    if let Ok(exe) = std::env::current_exe() {
        if let Some(d) = exe.parent() {
            dirs.push(d.to_path_buf());
            // macOS app bundle: exe lives in Contents/MacOS, resources
            // sometimes land in Contents/Resources — probe both.
            if let Some(contents) = d.parent() {
                dirs.push(contents.join("Resources"));
            }
        }
    }
    if let Ok(res_dir) = handle.path().resource_dir() {
        dirs.push(res_dir.clone());
        // Some Tauri versions keep externalBin's parent folder in the
        // bundle; probe `resource_dir/binaries/` as a defensive fallback.
        dirs.push(res_dir.join("binaries"));
    }
    dirs
}

/// Poll the backend until it serves a 2xx on `/healthz`, or the global
/// deadline elapses. Emits `backend:status` progress ticks at roughly
/// `PROGRESS_TICK`, so the UI stays lively even during long Go init
/// (plugin warmup, embedding caches, migrations, etc.).
async fn wait_for_healthy(handle: &AppHandle, port: u16, start: Instant) -> bool {
    // Prime `last_tick` so the first iteration emits immediately.
    let mut last_tick = Instant::now()
        .checked_sub(PROGRESS_TICK)
        .unwrap_or_else(Instant::now);
    loop {
        let elapsed = start.elapsed();
        if elapsed > HEALTH_TIMEOUT {
            log::warn!(
                "backend health check timed out after {}s",
                HEALTH_TIMEOUT.as_secs()
            );
            return false;
        }

        if probe_healthz(port).await {
            // Brief settle so Go finishes registering any late routes
            // before the user navigates to a page that fetches them.
            tokio::time::sleep(Duration::from_millis(150)).await;
            return true;
        }

        // Non-spammy progress: emit at most every ~500ms. Map elapsed to a
        // 15..90 progress window so the loader keeps crawling forward.
        if last_tick.elapsed() >= PROGRESS_TICK {
            let ratio = elapsed.as_secs_f64() / HEALTH_TIMEOUT.as_secs_f64();
            let pct = (ratio * 75.0 + 15.0).clamp(15.0, 90.0) as u8;
            emit_status(handle, "probing", "等待健康信号…", pct, port, start);
            last_tick = Instant::now();
        }

        tokio::time::sleep(POLL_INTERVAL).await;
    }
}

/// Minimal async HTTP/1.1 GET /healthz probe. Returns true on `2xx`.
/// We deliberately avoid pulling in `reqwest` just for one request: the
/// status line is all we need, and Go's /healthz returns `200 OK` with a
/// tiny body.
async fn probe_healthz(port: u16) -> bool {
    let addr = format!("127.0.0.1:{port}");
    let stream = match tokio::time::timeout(HTTP_PROBE_TIMEOUT, TokioTcpStream::connect(&addr))
        .await
    {
        Ok(Ok(s)) => s,
        _ => return false,
    };

    let probe = async move {
        let mut s = stream;
        s.write_all(b"GET /healthz HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n")
            .await?;
        s.flush().await?;
        let mut buf = [0u8; 64];
        let n = s.read(&mut buf).await?;
        Ok::<_, std::io::Error>((buf, n))
    };

    match tokio::time::timeout(HTTP_PROBE_TIMEOUT, probe).await {
        // "HTTP/1.1 2xx " — we only need the 3 status code bytes.
        Ok(Ok((buf, n))) if n >= 12 => matches!(&buf[9..12], b"200" | b"204"),
        _ => false,
    }
}

fn emit_status(
    handle: &AppHandle,
    phase: &str,
    message: &str,
    progress: u8,
    port: u16,
    start: Instant,
) {
    let payload = StatusPayload {
        phase,
        message,
        elapsed_ms: start.elapsed().as_millis(),
        progress,
        port,
    };
    if let Err(e) = handle.emit(EVT_STATUS, payload) {
        log::debug!("emit {EVT_STATUS} failed: {e}");
    }
}

fn emit_error(handle: &AppHandle, message: &str, port: u16) {
    if let Err(e) = handle.emit(EVT_ERROR, ErrorPayload { message, port }) {
        log::debug!("emit {EVT_ERROR} failed: {e}");
    }
}

// ── Platform signal helpers ─────────────────────────────────────────────────
// Kept to a tiny platform gate rather than a full cross-platform signal
// crate, since we only need one gesture: "please shut down".

#[cfg(windows)]
fn send_graceful_signal(pid: u32) -> bool {
    use windows_sys::Win32::System::Console::{GenerateConsoleCtrlEvent, CTRL_BREAK_EVENT};
    // Safety: GenerateConsoleCtrlEvent is thread-safe and takes an integer
    // process group id. The child was spawned with CREATE_NEW_PROCESS_GROUP,
    // so its pid is also its process group id.
    let ok = unsafe { GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, pid) };
    if ok == 0 {
        log::warn!(
            "GenerateConsoleCtrlEvent failed: {}",
            std::io::Error::last_os_error()
        );
        return false;
    }
    true
}

#[cfg(unix)]
fn send_graceful_signal(pid: u32) -> bool {
    // Safety: libc::kill is trivially safe; we're only sending SIGTERM.
    let rc = unsafe { libc::kill(pid as libc::pid_t, libc::SIGTERM) };
    if rc != 0 {
        log::warn!("kill(SIGTERM) failed: {}", std::io::Error::last_os_error());
        return false;
    }
    true
}

#[cfg(not(any(windows, unix)))]
fn send_graceful_signal(_pid: u32) -> bool {
    false
}

/// Poll `child.try_wait()` until it exits or `timeout` elapses.
/// Returns `true` if the child exited within the window.
///
/// Synchronous sleep is intentional — see note on `graceful_kill`.
fn wait_with_timeout(child: &mut Child, timeout: Duration) -> bool {
    let deadline = Instant::now() + timeout;
    loop {
        match child.try_wait() {
            Ok(Some(_status)) => return true,
            Ok(None) => {
                if Instant::now() >= deadline {
                    return false;
                }
                std::thread::sleep(GRACEFUL_POLL);
            }
            Err(e) => {
                log::warn!("try_wait error: {e}");
                return false;
            }
        }
    }
}
