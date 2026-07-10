mod selection_accessibility;

use std::process::{Child, Command};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Mutex, OnceLock};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use serde::{Deserialize, Serialize};
use tauri::{AppHandle, Emitter, Manager, State, Theme, WebviewUrl, WebviewWindow};
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpStream as TokioTcpStream;
use tauri_plugin_clipboard_manager::ClipboardExt;
use tauri_plugin_global_shortcut::{GlobalShortcutExt, Shortcut, ShortcutState};

static HOOK_SUPPRESS_UNTIL_MS: AtomicU64 = AtomicU64::new(0);

// Global selection assistant (the drag-to-select popup that can appear over
// ANY app). Default OFF: a system-wide overlay with no off switch is a
// hostile default, so the WH_MOUSE_LL hook is not even installed until the
// user opts in via Settings. `SELECTION_ENABLED` gates the hook callback at
// runtime; `SELECTION_HOOK_INSTALLED` ensures we only install the OS hook +
// capture worker once (lazily, on first enable).
static SELECTION_ENABLED: AtomicBool = AtomicBool::new(false);
static SELECTION_HOOK_INSTALLED: AtomicBool = AtomicBool::new(false);

fn hook_now_ms() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis() as u64
}

fn hook_suppressed() -> bool {
    hook_now_ms() < HOOK_SUPPRESS_UNTIL_MS.load(Ordering::Relaxed)
}

fn suppress_global_hook(ms: u64) {
    HOOK_SUPPRESS_UNTIL_MS.store(hook_now_ms() + ms, Ordering::Relaxed);
}

fn selection_popup_visible(handle: &AppHandle) -> bool {
    handle
        .get_webview_window("selection-popup")
        .and_then(|w| w.is_visible().ok())
        .unwrap_or(false)
}

// ── Tunables ────────────────────────────────────────────────────────────────
const DEFAULT_BACKEND_PORT: u16 = 9090;
const DEFAULT_FRONTEND_PORT: u16 = 3001;
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

// Event names - kept in one place so the loader HTML and main frontend
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

/// Last-ditch safety net: if the Tauri host exits without going through
/// `on_window_event` (panic on a background thread, `process::exit`, a
/// crash in tao's event loop, the user `taskkill /F` on the wrapper,
/// etc.) the managed `BackendState` is still dropped during teardown of
/// the `tauri::App`. Without this Drop the Go sidecar would survive as
/// an orphan, keep holding port 9090, and refuse the next launch.
///
/// We avoid the full graceful-timeout dance here because Drop runs on a
/// path where the process is already going away — best effort SIGTERM
/// then SIGKILL keeps shutdown bounded.
impl Drop for BackendState {
    fn drop(&mut self) {
        // Safe to get_mut: Drop has exclusive access.
        let Ok(mut guard) = self.child.lock() else {
            // Mutex poisoned: still try to take the child via into_inner-ish
            // recovery so a panic on the spawn path can't strand the sidecar.
            log::error!("backend state mutex poisoned in Drop; attempting recovery");
            return;
        };
        let Some(mut child) = guard.take() else {
            return;
        };
        let pid = child.id();
        log::info!("BackendState::drop — reaping sidecar (pid={pid})");
        let _ = send_graceful_signal(pid);
        // Short bounded wait so Drop never blocks shutdown forever.
        if !wait_with_timeout(&mut child, Duration::from_secs(2)) {
            let _ = child.kill();
            let _ = child.wait();
        }
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

/// Write `bytes` to `path` atomically by writing into a sibling tempfile
/// then renaming. Prevents zero-byte / half-written JSON if the process
/// crashes mid-write or the OS power-cycles during persistence.
///
/// Rename is atomic on the same volume on every supported platform
/// (POSIX `rename(2)`, Windows `MoveFileExW(MOVEFILE_REPLACE_EXISTING)`).
/// The temp file lives next to the target so we never cross a filesystem
/// boundary.
fn atomic_write(path: &std::path::Path, bytes: &[u8]) -> std::io::Result<()> {
    use std::io::Write;

    let dir = path.parent().ok_or_else(|| {
        std::io::Error::new(
            std::io::ErrorKind::InvalidInput,
            "atomic_write: path has no parent",
        )
    })?;
    std::fs::create_dir_all(dir)?;

    let file_name = path
        .file_name()
        .and_then(|s| s.to_str())
        .unwrap_or("tmp");
    let nonce = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_nanos())
        .unwrap_or(0);
    let tmp = dir.join(format!(".{file_name}.{nonce}.tmp"));

    {
        let mut f = std::fs::File::create(&tmp)?;
        f.write_all(bytes)?;
        f.sync_all()?;
    }
    // rename is atomic on the same filesystem; on Windows this maps to
    // MoveFileExW with MOVEFILE_REPLACE_EXISTING in std since 1.5.
    if let Err(e) = std::fs::rename(&tmp, path) {
        // Best-effort cleanup so we don't leave .tmp turds behind.
        let _ = std::fs::remove_file(&tmp);
        return Err(e);
    }
    Ok(())
}

// ── Window appearance (per-platform liquid glass / Mica) ────────────────────
//
// 我们按平台原生材质语言走，不强求跨平台像素级一致：
//   - macOS  : NSVisualEffectView vibrancy (`Sidebar`)。Apple 的 Metal 合成器
//              本身就做高质量降饱和模糊，配合 set_theme 让 NSAppearance
//              自动跟随，呈现真·液态玻璃。
//   - Win 11 : Mica（不再使用 Acrylic）。Acrylic 的盒式模糊 + 噪点纹理在
//              花壁纸下会出现"颗粒+斑驳"，远不及 mac 的 vibrancy。Mica
//              不模糊，但壁纸采样后只贡献"色温"，应用主体保持稳定纯净，
//              是微软自家 Win11 设置/资源管理器/Edge/Teams 的统一语言，
//              在 Win 平台被视作"现代高级感"的代表。
//   - Win 10 : 不支持 Mica，set_effects 调用会被 Tauri 静默忽略，窗口
//              呈现 `transparent: false` 后的纯色 webview，由 CSS
//              `--yunque-bg` 兜底，效果与 Linux 分支一致。
//   - Linux  : 没有 compositor-level blur API，set_effects(None)
//              + CSS 纯色底，由内部 `.glass-card` 等 backdrop-filter
//              提供应用内的玻璃质感。
//
// 设计哲学：外层窗口背景稳定，liquid glass 在应用内部由 CSS backdrop-filter
// 统一承担（Linear / Cursor / Codex 同款思路），跨平台一致且不被用户壁纸
// 颜色冲击。Mica 无 tint 参数，所以 Windows 分支不再传 Color。
//
// 该 helper 是窗口级的，主窗口和悬浮面板共用同一逻辑。
fn apply_window_appearance(window: &WebviewWindow, theme: &str) {
    // 1. Tell the OS what colour scheme the window should advertise.
    //    On macOS this drives NSAppearance (so vibrancy auto-adapts);
    //    on Windows this drives the title-bar / scrollbar colour
    //    AND tells Mica which wallpaper-tint variant to apply.
    let theme_enum = if theme == "light" { Theme::Light } else { Theme::Dark };
    let _ = window.set_theme(Some(theme_enum));

    // 2. Apply the platform-appropriate window material.
    #[cfg(target_os = "macos")]
    {
        use tauri::window::{Effect, EffectState, EffectsBuilder};
        let _ = window.set_effects(
            EffectsBuilder::new()
                .effect(Effect::Sidebar)
                .state(EffectState::FollowsWindowActiveState)
                .radius(10.0)
                .build(),
        );
    }

    #[cfg(target_os = "windows")]
    {
        use tauri::window::{Effect, EffectsBuilder};
        // Mica：壁纸只贡献色温，主体不模糊不透出形状，避免 Acrylic
        // 在花壁纸下产生的颗粒/斑驳。light/dark 用不同变体让
        // 系统采样出对应明暗倾向的色温。Win10 不支持时调用静默失败，
        // 窗口落到 `transparent: false` 的纯色兜底。
        let effect = if theme == "light" {
            Effect::MicaLight
        } else {
            Effect::MicaDark
        };
        let _ = window.set_effects(
            EffectsBuilder::new()
                .effect(effect)
                .build(),
        );
    }

    #[cfg(not(any(target_os = "macos", target_os = "windows")))]
    {
        // Linux / other：没有 compositor 级模糊 API，由 CSS 纯色底兜底，
        // 应用内部的 `.glass-card` 等 backdrop-filter 承担玻璃质感。
        let _ = window.set_effects(None);
    }
}

/// Apply the in-app theme to every window we own.
///
/// Called from `setup` once on startup and from the `apply_window_theme`
/// command whenever the front-end flips presetTheme. Iterating over all
/// windows means the floating panel/ball pick up the new appearance even
/// if they were created lazily after the user signed in.
fn apply_appearance_all(handle: &AppHandle, theme: &str) {
    for (label, win) in handle.webview_windows() {
        // The selection popup must stay fully transparent (only the pill shows).
        // Applying Mica fills its whole window with a dark backdrop — the
        // "black block" around the toolbar.
        if label == "selection-popup" {
            continue;
        }
        apply_window_appearance(&win, theme);
    }
}

/// Tracks the most recent theme the front-end asked for. Lazily-created
/// windows (floating ball / floating panel) read this on construction so
/// they don't briefly render as a plain transparent rectangle before the
/// next theme sync arrives.
struct ThemeState {
    current: Mutex<String>,
}

impl ThemeState {
    fn new() -> Self {
        Self {
            current: Mutex::new("dark".into()),
        }
    }

    fn current(&self) -> String {
        lock_or_recover(&self.current).clone()
    }

    fn set(&self, v: &str) {
        *lock_or_recover(&self.current) = v.to_string();
    }
}

fn current_theme(handle: &AppHandle) -> String {
    handle
        .try_state::<ThemeState>()
        .map(|s| s.current())
        .unwrap_or_else(|| "dark".into())
}

#[tauri::command]
fn apply_window_theme(handle: AppHandle, theme: String) {
    let normalized = if theme == "light" { "light" } else { "dark" };
    if let Some(state) = handle.try_state::<ThemeState>() {
        state.set(normalized);
    }
    apply_appearance_all(&handle, normalized);
}

// ── Floating window (Feishu-style) ──────────────────────────────────────────

#[derive(Serialize, Deserialize, Clone, Debug)]
struct FloatingItem {
    id: String,
    text: String,
    timestamp: u64,
}

const MAX_FLOATING_ITEMS: usize = 100;

#[derive(Serialize, Deserialize, Clone, Debug, Default)]
struct BallPosition {
    x: f64,
    y: f64,
}

struct FloatingState {
    items: Mutex<Vec<FloatingItem>>,
    next_id: AtomicU64,
    data_path: Mutex<Option<std::path::PathBuf>>,
}

impl FloatingState {
    fn new() -> Self {
        Self {
            items: Mutex::new(Vec::new()),
            next_id: AtomicU64::new(1),
            data_path: Mutex::new(None),
        }
    }

    fn gen_id(&self) -> String {
        let n = self.next_id.fetch_add(1, Ordering::Relaxed);
        let ts = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or_default()
            .as_millis();
        format!("{ts:x}-{n:x}")
    }

    fn save_items(&self) {
        let guard = lock_or_recover(&self.data_path);
        let Some(dir) = guard.as_ref() else { return };
        let path = dir.join("floating_items.json");
        let items = lock_or_recover(&self.items);
        let json = match serde_json::to_string_pretty(&*items) {
            Ok(s) => s,
            Err(e) => {
                log::error!("serialise floating items failed: {e}");
                return;
            }
        };
        if let Err(e) = atomic_write(&path, json.as_bytes()) {
            log::error!("save floating items failed ({}): {e}", path.display());
        }
    }

    fn load_items(&self) {
        let guard = lock_or_recover(&self.data_path);
        let Some(dir) = guard.as_ref() else { return };
        let path = dir.join("floating_items.json");
        if let Ok(json) = std::fs::read_to_string(&path) {
            if let Ok(items) = serde_json::from_str::<Vec<FloatingItem>>(&json) {
                let mut store = lock_or_recover(&self.items);
                *store = items;
                let max_id = store.iter()
                    .filter_map(|i| i.id.split('-').last().and_then(|s| u64::from_str_radix(s, 16).ok()))
                    .max()
                    .unwrap_or(0);
                self.next_id.store(max_id + 1, Ordering::Relaxed);
                log::info!("loaded {} floating items from disk", store.len());
            }
        }
    }

    fn save_ball_pos(&self, x: f64, y: f64) {
        let guard = lock_or_recover(&self.data_path);
        let Some(dir) = guard.as_ref() else { return };
        let path = dir.join("floating_ball_pos.json");
        let pos = BallPosition { x, y };
        let json = match serde_json::to_string(&pos) {
            Ok(s) => s,
            Err(e) => {
                log::error!("serialise ball pos failed: {e}");
                return;
            }
        };
        if let Err(e) = atomic_write(&path, json.as_bytes()) {
            log::error!("save ball pos failed ({}): {e}", path.display());
        }
    }

    #[allow(dead_code)] // used by create_floating_ball (floating ball feature, off by default)
    fn load_ball_pos(&self) -> Option<BallPosition> {
        let guard = lock_or_recover(&self.data_path);
        let dir = guard.as_ref()?;
        let path = dir.join("floating_ball_pos.json");
        let json = std::fs::read_to_string(path).ok()?;
        serde_json::from_str(&json).ok()
    }
}

#[tauri::command]
fn toggle_floating_panel(handle: AppHandle) {
    let ui_port = resolve_frontend_port();

    if let Some(panel) = handle.get_webview_window("floating-panel") {
        if panel.is_visible().unwrap_or(false) {
            let _ = panel.hide();
        } else {
            position_panel_near_ball(&handle, &panel);
            let _ = panel.show();
            let _ = panel.set_focus();
        }
        return;
    }

    let url = format!("http://127.0.0.1:{ui_port}/floating-panel");
    let parsed_url = match url.parse() {
        Ok(u) => u,
        Err(e) => {
            log::error!("invalid floating-panel url {url:?}: {e}");
            return;
        }
    };
    let (x, y) = panel_position_from_ball(&handle);

    let builder = tauri::WebviewWindowBuilder::new(
        &handle,
        "floating-panel",
        WebviewUrl::External(parsed_url),
    )
    .title("")
    .inner_size(320.0, 460.0)
    .position(x, y)
    .decorations(false)
    .resizable(false)
    .always_on_top(true)
    .skip_taskbar(true)
    .transparent(true)
    .focused(true);

    match builder.build() {
        Ok(panel) => {
            log::info!("floating panel created");
            apply_window_appearance(&panel, &current_theme(&handle));
        }
        Err(e) => log::error!("failed to create floating panel: {e}"),
    }
}

/// Approximate panel size (matches builder.inner_size below).
/// Used to clamp the panel into the visible monitor area.
const PANEL_W: i32 = 320;
const PANEL_H: i32 = 460;
/// Keep a small inset so the panel doesn't sit flush with the screen edge.
const PANEL_EDGE_INSET: i32 = 8;

/// Compute a "near the ball" placement for the floating panel, clamped to
/// the ball's current monitor. The original `(ball.x - 270, ball.y - 460)`
/// math sent the panel to negative coordinates when the ball sat near the
/// top-left corner of the screen, hiding the panel off-screen on first
/// open. We now mirror around the ball's monitor instead of blindly
/// subtracting.
fn clamped_panel_pos(handle: &AppHandle) -> Option<(i32, i32)> {
    let ball = handle.get_webview_window("floating-ball")?;
    let pos = ball.outer_position().ok()?;
    // Prefer the monitor the ball is actually on; fall back to primary.
    let monitor = ball
        .current_monitor()
        .ok()
        .flatten()
        .or_else(|| handle.primary_monitor().ok().flatten())?;
    let m_pos = monitor.position();
    let m_size = monitor.size();
    let min_x = m_pos.x + PANEL_EDGE_INSET;
    let min_y = m_pos.y + PANEL_EDGE_INSET;
    let max_x = m_pos.x + (m_size.width as i32) - PANEL_W - PANEL_EDGE_INSET;
    let max_y = m_pos.y + (m_size.height as i32) - PANEL_H - PANEL_EDGE_INSET;
    let x = (pos.x - PANEL_W + 50).clamp(min_x, max_x.max(min_x));
    let y = (pos.y - PANEL_H).clamp(min_y, max_y.max(min_y));
    Some((x, y))
}

fn position_panel_near_ball(handle: &AppHandle, panel: &tauri::WebviewWindow) {
    if let Some((x, y)) = clamped_panel_pos(handle) {
        let _ = panel.set_position(tauri::PhysicalPosition::new(x, y));
    }
}

fn panel_position_from_ball(handle: &AppHandle) -> (f64, f64) {
    if let Some((x, y)) = clamped_panel_pos(handle) {
        return (x as f64, y as f64);
    }
    (400.0, 200.0)
}

#[tauri::command]
fn get_floating_items(state: State<FloatingState>) -> Vec<FloatingItem> {
    lock_or_recover(&state.items).clone()
}

#[tauri::command]
fn get_floating_count(state: State<FloatingState>) -> usize {
    lock_or_recover(&state.items).len()
}

#[tauri::command]
fn add_floating_item(
    text: String,
    state: State<FloatingState>,
    handle: AppHandle,
) -> FloatingItem {
    let item = FloatingItem {
        id: state.gen_id(),
        text,
        timestamp: SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or_default()
            .as_secs(),
    };
    {
        let mut items = lock_or_recover(&state.items);
        items.insert(0, item.clone());
        items.truncate(MAX_FLOATING_ITEMS);
    }
    state.save_items();
    let _ = handle.emit("yunque:floating-update", ());
    item
}

#[tauri::command]
fn remove_floating_item(id: String, state: State<FloatingState>, handle: AppHandle) {
    lock_or_recover(&state.items).retain(|i| i.id != id);
    state.save_items();
    let _ = handle.emit("yunque:floating-update", ());
}

#[tauri::command]
fn clear_floating_items(state: State<FloatingState>, handle: AppHandle) {
    lock_or_recover(&state.items).clear();
    state.save_items();
    let _ = handle.emit("yunque:floating-update", ());
}

#[tauri::command]
fn floating_send_to_chat(text: String, handle: AppHandle) {
    // Emit a typed Tauri event instead of injecting arbitrary JS via
    // `main.eval()`. Two reasons:
    //   1. eval() forces us to keep `script-src 'unsafe-eval'` in CSP if
    //      we ever want to allow webview-side `Function()` etc. By going
    //      through the event bus we keep CSP tight and still get the
    //      same delivery semantics.
    //   2. The previous JSON.stringify + format!() round-trip silently
    //      dropped messages when serialisation failed (`unwrap_or_default`).
    //      `emit_to` returns a Result so problems surface in logs.
    if let Some(main) = handle.get_webview_window("main") {
        if let Err(e) = main.emit("yunque:quick-send", &text) {
            log::warn!("emit yunque:quick-send to main failed: {e}");
        }
        let _ = main.show();
        let _ = main.set_focus();
    }
    if let Some(panel) = handle.get_webview_window("floating-panel") {
        let _ = panel.hide();
    }
    suppress_global_hook(180);
    hide_selection_popup_window(&handle);
}

#[allow(dead_code)] // floating ball is opt-in; kept for the settings toggle path
fn create_floating_ball(handle: &AppHandle, port: u16) {
    let url = format!("http://127.0.0.1:{port}/floating-ball");
    let parsed_url = match url.parse() {
        Ok(u) => u,
        Err(e) => {
            log::error!("invalid floating-ball url {url:?}: {e}");
            return;
        }
    };

    let state = handle.state::<FloatingState>();
    let (x, y) = if let Some(pos) = state.load_ball_pos() {
        (pos.x, pos.y)
    } else if let Ok(Some(monitor)) = handle.primary_monitor() {
        let size = monitor.size();
        let scale = monitor.scale_factor();
        (
            (size.width as f64 / scale - 80.0),
            (size.height as f64 / scale - 120.0),
        )
    } else {
        (1800.0, 900.0)
    };

    let builder = tauri::WebviewWindowBuilder::new(
        handle,
        "floating-ball",
        WebviewUrl::External(parsed_url),
    )
    .title("")
    .inner_size(56.0, 56.0)
    .position(x, y)
    .decorations(false)
    .resizable(false)
    .always_on_top(true)
    .skip_taskbar(true)
    .transparent(true)
    .focused(false);

    match builder.build() {
        Ok(ball) => {
            log::info!("floating ball created at ({x}, {y})");
            // The 56x56 ball renders as a circle in CSS, but we still set
            // window-level material so its drop-shadow / hover halo blend
            // with the desktop instead of clipping against opaque pixels.
            apply_window_appearance(&ball, &current_theme(handle));
        }
        Err(e) => log::error!("failed to create floating ball: {e}"),
    }
}

// ── Keyboard simulation ─────────────────────────────────────────────────────

#[cfg(windows)]
fn simulate_ctrl_c() {
    use windows_sys::Win32::UI::Input::KeyboardAndMouse::{
        SendInput, INPUT, INPUT_0, INPUT_KEYBOARD, KEYBDINPUT, KEYEVENTF_KEYUP, VK_CONTROL,
    };
    let vk_c: u16 = 0x43; // 'C'
    let inputs: [INPUT; 4] = [
        INPUT {
            r#type: INPUT_KEYBOARD,
            Anonymous: INPUT_0 {
                ki: KEYBDINPUT {
                    wVk: VK_CONTROL,
                    wScan: 0,
                    dwFlags: 0,
                    time: 0,
                    dwExtraInfo: 0,
                },
            },
        },
        INPUT {
            r#type: INPUT_KEYBOARD,
            Anonymous: INPUT_0 {
                ki: KEYBDINPUT {
                    wVk: vk_c,
                    wScan: 0,
                    dwFlags: 0,
                    time: 0,
                    dwExtraInfo: 0,
                },
            },
        },
        INPUT {
            r#type: INPUT_KEYBOARD,
            Anonymous: INPUT_0 {
                ki: KEYBDINPUT {
                    wVk: vk_c,
                    wScan: 0,
                    dwFlags: KEYEVENTF_KEYUP,
                    time: 0,
                    dwExtraInfo: 0,
                },
            },
        },
        INPUT {
            r#type: INPUT_KEYBOARD,
            Anonymous: INPUT_0 {
                ki: KEYBDINPUT {
                    wVk: VK_CONTROL,
                    wScan: 0,
                    dwFlags: KEYEVENTF_KEYUP,
                    time: 0,
                    dwExtraInfo: 0,
                },
            },
        },
    ];
    unsafe {
        SendInput(4, inputs.as_ptr(), std::mem::size_of::<INPUT>() as i32);
    }
}

#[cfg(not(windows))]
fn simulate_ctrl_c() {
    log::warn!("simulate_ctrl_c not implemented on this platform");
}

#[cfg(windows)]
fn focus_hwnd(hwnd: isize) {
    use windows_sys::Win32::UI::WindowsAndMessaging::SetForegroundWindow;
    if hwnd != 0 {
        unsafe {
            let _ = SetForegroundWindow(hwnd as _);
        }
    }
}

#[cfg(not(windows))]
fn focus_hwnd(_hwnd: isize) {}

#[cfg(windows)]
fn request_copy_from_window(hwnd: isize) {
    use windows_sys::Win32::UI::WindowsAndMessaging::SendMessageW;
    const WM_COPY: u32 = 0x0301;
    if hwnd != 0 {
        unsafe {
            SendMessageW(hwnd as _, WM_COPY, 0, 0);
        }
    }
}

#[cfg(not(windows))]
fn request_copy_from_window(_hwnd: isize) {}

#[cfg(windows)]
fn hwnd_copy_chain(leaf: isize) -> Vec<isize> {
    use windows_sys::Win32::UI::WindowsAndMessaging::GetParent;
    let mut chain = Vec::new();
    let mut cur = leaf;
    for _ in 0..8 {
        if cur == 0 {
            break;
        }
        chain.push(cur);
        cur = unsafe { GetParent(cur as _) as isize };
    }
    chain
}

#[cfg(not(windows))]
fn hwnd_copy_chain(leaf: isize) -> Vec<isize> {
    if leaf != 0 {
        vec![leaf]
    } else {
        vec![]
    }
}

fn read_clipboard_text(handle: &AppHandle) -> Option<String> {
    let text = handle
        .clipboard()
        .read_text()
        .unwrap_or_default()
        .trim()
        .to_string();
    if text.len() >= 2 {
        Some(text)
    } else {
        None
    }
}

/// Clipboard-based fallback used when the accessibility (UIA) read returns
/// nothing. Sends WM_COPY up the focused window's parent chain, then simulates
/// Ctrl+C as a last resort. Runs on the capture worker thread, never the input
/// hook thread, so its sleeps don't lag global input.
fn capture_via_clipboard(handle: &AppHandle, fg_hwnd: Option<isize>) -> Option<String> {
    let hwnds: Vec<isize> = fg_hwnd
        .filter(|h| *h != 0)
        .map(hwnd_copy_chain)
        .unwrap_or_default();

    for hwnd in hwnds {
        focus_hwnd(hwnd);
        request_copy_from_window(hwnd);
        std::thread::sleep(Duration::from_millis(24));
        if let Some(text) = read_clipboard_text(handle) {
            return Some(text);
        }
    }

    simulate_ctrl_c();
    std::thread::sleep(Duration::from_millis(30));
    read_clipboard_text(handle)
}

/// One-off capture for the Alt+Y shortcut (no persistent worker on this path).
/// Tries accessibility first, then clipboard.
fn capture_selection_oneoff(handle: &AppHandle, fg_hwnd: isize) -> Option<String> {
    if let Some(reader) = selection_accessibility::SelectionReader::new() {
        if let Some(text) = reader.read() {
            return Some(text);
        }
    }
    capture_via_clipboard(handle, Some(fg_hwnd))
}

#[cfg(windows)]
fn is_child_of(child: isize, parent: isize) -> bool {
    use windows_sys::Win32::UI::WindowsAndMessaging::IsChild;
    if child == 0 || parent == 0 {
        return false;
    }
    unsafe { IsChild(parent as _, child as _) != 0 }
}

#[cfg(windows)]
fn hwnd_belongs_to_yunque(handle: &AppHandle, target: isize) -> bool {
    if target == 0 {
        return false;
    }
    for label in ["main", "selection-popup", "floating-panel", "floating-ball"] {
        if let Some(win) = handle.get_webview_window(label) {
            if let Ok(own) = win.hwnd() {
                let own_hwnd = own.0 as isize;
                if target as usize == own_hwnd as usize || is_child_of(target, own_hwnd) {
                    return true;
                }
            }
        }
    }
    false
}

#[cfg(not(windows))]
fn hwnd_belongs_to_yunque(handle: &AppHandle, _target: isize) -> bool {
    for label in ["main", "selection-popup"] {
        if let Some(win) = handle.get_webview_window(label) {
            if win.is_focused().unwrap_or(false) {
                return true;
            }
        }
    }
    false
}

static GLOBAL_LISTENER_APP: OnceLock<AppHandle> = OnceLock::new();
static LAST_GLOBAL_TRIGGER_MS: AtomicU64 = AtomicU64::new(0);
/// Gesture requests (anchor_x, anchor_y, target_hwnd) handed to the long-lived
/// capture worker. Using a dedicated worker lets us build the UI Automation
/// client exactly once instead of paying CoCreateInstance per selection, which
/// was the dominant source of the "卡顿" lag.
static SELECTION_CAPTURE_TX: OnceLock<std::sync::mpsc::Sender<(i32, i32, isize)>> = OnceLock::new();

/// Spawn the persistent capture worker. It owns the UIA client and processes
/// one (coalesced) gesture at a time: accessibility read first, clipboard
/// fallback second, then shows the popup at the gesture anchor.
fn start_selection_capture_worker(handle: AppHandle) {
    let (tx, rx) = std::sync::mpsc::channel::<(i32, i32, isize)>();
    if SELECTION_CAPTURE_TX.set(tx).is_err() {
        return;
    }
    std::thread::Builder::new()
        .name("selection-capture".into())
        .spawn(move || {
            // Built once on this thread; reused for every read.
            let reader = selection_accessibility::SelectionReader::new();
            log::info!("selection capture worker ready (uia={})", reader.is_some());

            while let Ok(first) = rx.recv() {
                // Coalesce: if the user dragged several times while we were
                // busy, only honor the most recent gesture.
                let mut latest = first;
                while let Ok(newer) = rx.try_recv() {
                    latest = newer;
                }
                let (ax, ay, hwnd) = latest;

                let text = reader
                    .as_ref()
                    .and_then(|r| r.read())
                    .or_else(|| capture_via_clipboard(&handle, Some(hwnd)));

                if let Some(text) = text {
                    show_selection_popup_at(&handle, &text, Some((ax, ay)));
                }
            }
        })
        .ok();
}

#[cfg(windows)]
fn hwnd_at_screen_point(x: i32, y: i32) -> isize {
    use windows_sys::Win32::Foundation::POINT;
    use windows_sys::Win32::UI::WindowsAndMessaging::{GetForegroundWindow, WindowFromPoint};
    let pt = POINT { x, y };
    let hwnd = unsafe { WindowFromPoint(pt) as isize };
    if hwnd != 0 {
        return hwnd;
    }
    unsafe { GetForegroundWindow() as isize }
}

#[cfg(not(windows))]
fn hwnd_at_screen_point(_x: i32, _y: i32) -> isize {
    0
}

fn try_schedule_global_selection(anchor: (i32, i32), target_hwnd: isize) {
    let Some(handle) = GLOBAL_LISTENER_APP.get() else {
        return;
    };
    if hook_suppressed() || selection_popup_visible(handle) {
        return;
    }
    if hwnd_belongs_to_yunque(handle, target_hwnd) {
        return;
    }
    let now = hook_now_ms();
    let last = LAST_GLOBAL_TRIGGER_MS.load(Ordering::Relaxed);
    if now.saturating_sub(last) < 80 {
        return;
    }
    LAST_GLOBAL_TRIGGER_MS.store(now, Ordering::Relaxed);

    // Hand off to the persistent worker; never block the input hook thread.
    if let Some(tx) = SELECTION_CAPTURE_TX.get() {
        let _ = tx.send((anchor.0, anchor.1, target_hwnd));
    }
}

#[derive(Serialize, Deserialize, Default)]
struct SelectionPref {
    enabled: bool,
}

fn selection_pref_path(handle: &AppHandle) -> Option<std::path::PathBuf> {
    handle
        .path()
        .app_data_dir()
        .ok()
        .map(|d| d.join("data").join("selection_assistant.json"))
}

/// Read the persisted on/off state for the global selection assistant.
/// Missing / unreadable file means OFF — the privacy-respecting default.
fn read_selection_pref(handle: &AppHandle) -> bool {
    let Some(path) = selection_pref_path(handle) else {
        return false;
    };
    std::fs::read_to_string(&path)
        .ok()
        .and_then(|s| serde_json::from_str::<SelectionPref>(&s).ok())
        .map(|p| p.enabled)
        .unwrap_or(false)
}

fn write_selection_pref(handle: &AppHandle, enabled: bool) {
    let Some(path) = selection_pref_path(handle) else {
        return;
    };
    if let Some(parent) = path.parent() {
        let _ = std::fs::create_dir_all(parent);
    }
    match serde_json::to_string(&SelectionPref { enabled }) {
        Ok(json) => {
            if let Err(e) = std::fs::write(&path, json) {
                log::error!("write selection pref failed: {e}");
            }
        }
        Err(e) => log::error!("serialise selection pref failed: {e}"),
    }
}

/// Install the OS-level capture pipeline exactly once. Safe to call repeatedly;
/// only the first call (after the user opts in) actually starts the worker and
/// the WH_MOUSE_LL hook.
fn ensure_selection_capture_installed(handle: &AppHandle) {
    if SELECTION_HOOK_INSTALLED.swap(true, Ordering::SeqCst) {
        return;
    }
    start_selection_capture_worker(handle.clone());
    register_global_selection_listener(handle);
    prewarm_selection_popup(handle);
}

#[tauri::command]
fn get_selection_assistant_enabled() -> bool {
    SELECTION_ENABLED.load(Ordering::Relaxed)
}

#[tauri::command]
fn set_selection_assistant_enabled(app: AppHandle, enabled: bool) {
    SELECTION_ENABLED.store(enabled, Ordering::SeqCst);
    write_selection_pref(&app, enabled);
    if enabled {
        ensure_selection_capture_installed(&app);
    } else {
        // Hook stays in the chain but no-ops; clear any popup already showing.
        hide_selection_popup_window(&app);
    }
    log::info!("selection assistant {}", if enabled { "enabled" } else { "disabled" });
}


#[cfg(windows)]
static HOOK_DRAG_START: Mutex<Option<(i32, i32)>> = Mutex::new(None);
#[cfg(windows)]
static HOOK_LAST_UP: AtomicU64 = AtomicU64::new(0);
#[cfg(windows)]
static HOOK_CLICK_STREAK: AtomicU64 = AtomicU64::new(0);

/// Low-level mouse hook. The callback is intentionally a *no-op for mouse
/// movement* — it only does work on button down/up and returns immediately via
/// CallNextHookEx. This is the key to keeping the cursor smooth: unlike rdev
/// (which allocated an Event for every system-wide move), we never touch the
/// high-frequency move stream.
#[cfg(windows)]
unsafe extern "system" fn selection_mouse_proc(
    code: i32,
    wparam: windows_sys::Win32::Foundation::WPARAM,
    lparam: windows_sys::Win32::Foundation::LPARAM,
) -> windows_sys::Win32::Foundation::LRESULT {
    use windows_sys::Win32::UI::WindowsAndMessaging::{
        CallNextHookEx, HC_ACTION, MSLLHOOKSTRUCT, WM_LBUTTONDOWN, WM_LBUTTONUP,
    };

    // Runtime kill-switch: when the user has the selection assistant turned
    // off we leave the hook in the chain but do nothing, so toggling off is
    // instant and doesn't require tearing down the hook thread.
    if !SELECTION_ENABLED.load(Ordering::Relaxed) {
        return CallNextHookEx(std::ptr::null_mut(), code, wparam, lparam);
    }

    if code == HC_ACTION as i32 {
        match wparam as u32 {
            WM_LBUTTONDOWN => {
                let info = &*(lparam as *const MSLLHOOKSTRUCT);
                let now = hook_now_ms();
                if now.saturating_sub(HOOK_LAST_UP.load(Ordering::Relaxed)) < 450 {
                    HOOK_CLICK_STREAK.fetch_add(1, Ordering::Relaxed);
                } else {
                    HOOK_CLICK_STREAK.store(1, Ordering::Relaxed);
                }
                if let Ok(mut g) = HOOK_DRAG_START.lock() {
                    *g = Some((info.pt.x, info.pt.y));
                }
            }
            WM_LBUTTONUP => {
                let info = &*(lparam as *const MSLLHOOKSTRUCT);
                HOOK_LAST_UP.store(hook_now_ms(), Ordering::Relaxed);
                let start = HOOK_DRAG_START.lock().ok().and_then(|mut g| g.take());
                if let Some((sx, sy)) = start {
                    let (ax, ay) = (info.pt.x, info.pt.y);
                    // A real text selection is either a drag of a few px or a
                    // double-click word select. A plain (even slightly-held)
                    // single click is NOT a selection — it should *dismiss* a
                    // stale popup, not re-trigger capture. The old `held>=35ms`
                    // rule fired on virtually every click, so deselecting never
                    // closed the toolbar.
                    // Require a deliberate drag (≥8px) so accidental
                    // click-jitter no longer pops the toolbar mid-interaction.
                    let dragged = (ax - sx).abs() >= 8 || (ay - sy).abs() >= 8;
                    let dbl = HOOK_CLICK_STREAK.load(Ordering::Relaxed) >= 2;
                    let hwnd = hwnd_at_screen_point(ax, ay);
                    let on_yunque = GLOBAL_LISTENER_APP
                        .get()
                        .map(|h| hwnd_belongs_to_yunque(h, hwnd))
                        .unwrap_or(false);
                    if on_yunque {
                        // Click landed on our own popup/windows — let the web
                        // layer handle it (button actions self-dismiss).
                    } else if dragged || dbl {
                        try_schedule_global_selection((ax, ay), hwnd);
                    } else if let Some(h) = GLOBAL_LISTENER_APP.get() {
                        if selection_popup_visible(h) {
                            suppress_global_hook(250);
                            hide_selection_popup_window(h);
                        }
                    }
                }
            }
            _ => {}
        }
    }

    CallNextHookEx(std::ptr::null_mut(), code, wparam, lparam)
}

#[cfg(windows)]
fn register_global_selection_listener(handle: &AppHandle) {
    let _ = GLOBAL_LISTENER_APP.set(handle.clone());
    std::thread::Builder::new()
        .name("global-selection-hook".into())
        .spawn(|| unsafe {
            use windows_sys::Win32::UI::WindowsAndMessaging::{
                GetMessageW, SetWindowsHookExW, UnhookWindowsHookEx, MSG, WH_MOUSE_LL,
            };
            let hook =
                SetWindowsHookExW(WH_MOUSE_LL, Some(selection_mouse_proc), std::ptr::null_mut(), 0);
            if hook.is_null() {
                log::error!("SetWindowsHookExW(WH_MOUSE_LL) failed");
                return;
            }
            log::info!("global selection hook installed (WH_MOUSE_LL)");
            let mut msg: MSG = std::mem::zeroed();
            while GetMessageW(&mut msg, std::ptr::null_mut(), 0, 0) > 0 {}
            let _ = UnhookWindowsHookEx(hook);
        })
        .ok();
}

#[cfg(not(windows))]
fn register_global_selection_listener(handle: &AppHandle) {
    let _ = GLOBAL_LISTENER_APP.set(handle.clone());
    log::info!("global selection listener: Alt+Y only on this platform");
}

const SEL_POPUP_W: f64 = 340.0;
const SEL_POPUP_H: f64 = 38.0;

#[cfg(windows)]
fn cursor_physical_position() -> (i32, i32) {
    use windows_sys::Win32::Foundation::POINT;
    use windows_sys::Win32::UI::WindowsAndMessaging::GetCursorPos;
    let mut pt = POINT { x: 0, y: 0 };
    unsafe {
        GetCursorPos(&mut pt);
    }
    (pt.x, pt.y)
}

#[cfg(not(windows))]
fn cursor_physical_position() -> (i32, i32) {
    (0, 0)
}

fn selection_popup_origin_at(handle: &AppHandle, cx: i32, cy: i32) -> (i32, i32) {
    let w = SEL_POPUP_W as i32;
    let h = SEL_POPUP_H as i32;
    let mut x = cx - w / 2;
    let mut y = cy - h - 14;

    if let Ok(Some(monitor)) = handle.primary_monitor() {
        let pos = monitor.position();
        let size = monitor.size();
        let min_x = pos.x + 8;
        let min_y = pos.y + 8;
        let max_x = pos.x + size.width as i32 - w - 8;
        let max_y = pos.y + size.height as i32 - h - 8;
        x = x.clamp(min_x, max_x.max(min_x));
        y = y.clamp(min_y, max_y.max(min_y));
    }

    (x, y)
}

fn show_selection_popup(handle: &AppHandle, text: &str) {
    show_selection_popup_at(handle, text, None);
}

fn show_selection_popup_at(handle: &AppHandle, text: &str, anchor: Option<(i32, i32)>) {
    let ui_port = resolve_frontend_port();
    let (cx, cy) = anchor.unwrap_or_else(|| cursor_physical_position());
    let (x, y) = selection_popup_origin_at(handle, cx, cy);

    if let Some(popup) = handle.get_webview_window("selection-popup") {
        let _ = popup.set_position(tauri::PhysicalPosition::new(x, y));
        if let Err(e) = popup.emit("yunque:selection-text", text) {
            log::warn!("emit yunque:selection-text failed: {e}");
        }
        let _ = popup.show();
        return;
    }

    let url = format!(
        "http://127.0.0.1:{ui_port}/selection-popup.html?text={}",
        urlencoding::encode(text)
    );
    let parsed_url = match url.parse() {
        Ok(u) => u,
        Err(e) => {
            log::error!("invalid selection-popup url {url:?}: {e}");
            return;
        }
    };

    let builder = tauri::WebviewWindowBuilder::new(
        handle,
        "selection-popup",
        WebviewUrl::External(parsed_url),
    )
    .title("")
    .inner_size(SEL_POPUP_W, SEL_POPUP_H)
    .position(x as f64, y as f64)
    .decorations(false)
    .resizable(false)
    .always_on_top(true)
    .skip_taskbar(true)
    .transparent(true)
    .shadow(false)
    .focused(false)
    .accept_first_mouse(true);

    match builder.build() {
        Ok(popup) => {
            // Deliberately NOT calling apply_window_appearance: the popup stays
            // a bare transparent window so only the pill is visible (no Mica
            // backdrop = no black block). shadow(false) removes the DWM window
            // drop-shadow that otherwise paints a dark halo around it.
            let _ = popup.show();
            log::info!("selection popup created");
        }
        Err(e) => log::error!("failed to create selection popup: {e}"),
    }
}

fn hide_selection_popup_window(handle: &AppHandle) {
    LAST_GLOBAL_TRIGGER_MS.store(0, Ordering::Relaxed);
    if let Some(popup) = handle.get_webview_window("selection-popup") {
        let _ = popup.hide();
    }
}

/// Create the popup webview once at startup so the first real selection
/// doesn't pay window-creation + page-load latency.
fn prewarm_selection_popup(handle: &AppHandle) {
    if handle.get_webview_window("selection-popup").is_some() {
        return;
    }
    let ui_port = resolve_frontend_port();
    let url = format!("http://127.0.0.1:{ui_port}/selection-popup.html?text=");
    let parsed_url = match url.parse() {
        Ok(u) => u,
        Err(e) => {
            log::warn!("prewarm selection-popup url failed: {e}");
            return;
        }
    };
    let builder = tauri::WebviewWindowBuilder::new(
        handle,
        "selection-popup",
        WebviewUrl::External(parsed_url),
    )
    .title("")
    .inner_size(SEL_POPUP_W, SEL_POPUP_H)
    .position(-32000.0, -32000.0)
    .decorations(false)
    .resizable(false)
    .always_on_top(true)
    .skip_taskbar(true)
    .transparent(true)
    .shadow(false)
    .focused(false)
    .visible(false);

    match builder.build() {
        Ok(_popup) => {
            // No Mica — keep it transparent (see show_selection_popup_at).
            log::info!("selection popup prewarmed");
        }
        Err(e) => log::warn!("selection popup prewarm failed: {e}"),
    }
}

#[tauri::command]
fn hide_selection_popup(handle: AppHandle) {
    suppress_global_hook(180);
    hide_selection_popup_window(&handle);
}

#[tauri::command]
fn selection_popup_dismiss(handle: AppHandle) {
    suppress_global_hook(180);
    hide_selection_popup_window(&handle);
}

fn register_selection_shortcut(handle: &AppHandle) {
    // "Alt+Y" is a static literal — if this ever fails to parse, it's a
    // programmer error caught at first launch in dev, not a runtime
    // condition we should panic on in user's hands.
    let shortcut: Shortcut = match "Alt+Y".parse() {
        Ok(s) => s,
        Err(e) => {
            log::error!("failed to parse global shortcut spec: {e}");
            return;
        }
    };
    let handle2 = handle.clone();
    if let Err(e) = handle.global_shortcut().on_shortcut(shortcut, move |_app, _sc, event| {
        if event.state != ShortcutState::Pressed {
            return;
        }
        log::info!("global shortcut Alt+Y triggered");
        let h = handle2.clone();
        std::thread::spawn(move || {
            let fg = {
                #[cfg(windows)]
                {
                    use windows_sys::Win32::UI::WindowsAndMessaging::GetForegroundWindow;
                    unsafe { GetForegroundWindow() as isize }
                }
                #[cfg(not(windows))]
                {
                    0
                }
            };
            let Some(text) = capture_selection_oneoff(&h, fg) else {
                return;
            };

            let state = h.state::<FloatingState>();
            let item = FloatingItem {
                id: state.gen_id(),
                text: text.clone(),
                timestamp: SystemTime::now()
                    .duration_since(UNIX_EPOCH)
                    .unwrap_or_default()
                    .as_secs(),
            };
            {
                let mut items = lock_or_recover(&state.items);
                items.insert(0, item);
                items.truncate(MAX_FLOATING_ITEMS);
            }
            state.save_items();
            let _ = h.emit("yunque:floating-update", ());
            log::info!("added text to floating items ({} chars)", text.len());

            show_selection_popup(&h, &text);
        });
    }) {
        log::error!("failed to register global shortcut: {e}");
    }
}

#[allow(dead_code)]
fn show_floating_panel(handle: &AppHandle) {
    if let Some(panel) = handle.get_webview_window("floating-panel") {
        position_panel_near_ball(handle, &panel);
        let _ = panel.show();
        let _ = panel.set_focus();
    } else {
        toggle_floating_panel(handle.clone());
    }
}

// ── System tray ─────────────────────────────────────────────────────────────
//
// Bring the main window back from the tray. Covers all three "hidden" states:
// minimized (unminimize), hidden via close-to-tray (show), and not-focused
// (set_focus). Without this the user could close/minimize the window and have
// no way to summon the GUI again — the icon would sit dead in the tray.
fn show_main_window(handle: &AppHandle) {
    if let Some(main) = handle.get_webview_window("main") {
        let _ = main.unminimize();
        let _ = main.show();
        let _ = main.set_focus();
    }
}

/// Real quit path (tray menu "退出"). Closing the main window only hides it to
/// the tray to keep the agent resident, so the *only* way to actually stop the
/// backend + exit is here (plus the BackendState Drop safety net).
fn quit_app(handle: &AppHandle) {
    if let Some(state) = handle.try_state::<BackendState>() {
        state.graceful_kill();
    }
    handle.exit(0);
}

/// UI-invokable quit. The connection-guard splash offers this as an escape
/// hatch when the backend never comes up on first launch — without it a stuck
/// user's only exit is the tray menu, which is easy to miss on a frameless
/// window. Routes through `quit_app` so the sidecar is still reaped gracefully.
#[tauri::command]
fn quit_from_ui(handle: AppHandle) {
    log::info!("quit requested from UI (connection splash escape hatch)");
    quit_app(&handle);
}

/// Open the desktop log directory in the OS file browser. Paired with the
/// splash escape hatch so a user staring at a stuck "本地服务不可用" screen can
/// grab logs for a bug report instead of being dead-ended. Uses a per-OS
/// reveal command rather than the opener plugin to avoid widening capabilities.
#[tauri::command]
fn open_log_dir(handle: AppHandle) -> Result<(), String> {
    let dir = handle
        .path()
        .app_log_dir()
        .map_err(|e| format!("resolve log dir: {e}"))?;
    // Best-effort: the dir may not exist yet if logging never wrote a file.
    let _ = std::fs::create_dir_all(&dir);

    #[cfg(target_os = "windows")]
    let result = Command::new("explorer").arg(&dir).spawn();
    #[cfg(target_os = "macos")]
    let result = Command::new("open").arg(&dir).spawn();
    #[cfg(not(any(target_os = "windows", target_os = "macos")))]
    let result = Command::new("xdg-open").arg(&dir).spawn();

    result
        .map(|_| ())
        .map_err(|e| format!("open log dir {}: {e}", dir.display()))
}

/// Persist the floating-ball position from a window context. Shared by the
/// close-to-tray and real-teardown paths.
fn save_ball_pos_from(window: &tauri::Window) {
    if let Some(ball) = window.app_handle().get_webview_window("floating-ball") {
        if let Ok(pos) = ball.outer_position() {
            if let Some(fs) = window.try_state::<FloatingState>() {
                fs.save_ball_pos(pos.x as f64, pos.y as f64);
            }
        }
    }
}

// ── Entry point ─────────────────────────────────────────────────────────────
#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
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
        .plugin(tauri_plugin_global_shortcut::Builder::new().build())
        .plugin(tauri_plugin_clipboard_manager::init())
        .manage(BackendState {
            child: Mutex::new(None),
        })
        .manage(FloatingState::new())
        .manage(ThemeState::new())
        .invoke_handler(tauri::generate_handler![
            backend_port,
            toggle_floating_panel,
            get_floating_items,
            get_floating_count,
            add_floating_item,
            remove_floating_item,
            clear_floating_items,
            floating_send_to_chat,
            hide_selection_popup,
            selection_popup_dismiss,
            apply_window_theme,
            get_selection_assistant_enabled,
            set_selection_assistant_enabled,
            quit_from_ui,
            open_log_dir,
        ])
        .setup(|app| {
            let handle = app.handle().clone();

            if let Ok(app_data) = handle.path().app_data_dir() {
                let data_dir = app_data.join("data");
                let _ = std::fs::create_dir_all(&data_dir);
                let state = handle.state::<FloatingState>();
                *lock_or_recover(&state.data_path) = Some(data_dir);
                state.load_items();
            }

            // First-paint window material. The front-end will call
            // `apply_window_theme` again as soon as it boots, but doing it
            // here too prevents a flash where the window briefly shows
            // the raw webview before Mica/vibrancy kicks in. We default
            // to dark — matching the inline bootstrap script in layout.tsx.
            //
            // NOTE: window decorations / title-bar style are owned by
            // `tauri.conf.json` (decorations:false, hiddenTitle:true) so
            // every platform follows the same source of truth. The previous
            // macOS-only override that re-enabled decorations + Overlay
            // title bar contradicted the conf and produced an inconsistent
            // chrome on mac vs. Windows. If we ever want a mac-specific
            // overlay title bar back, do it through `windows[].titleBarStyle`
            // in tauri.conf.json rather than mutating the window after
            // creation.
            if let Some(main) = handle.get_webview_window("main") {
                apply_window_appearance(&main, "dark");
            }

            // System tray: keep the agent resident and re-summonable. Left-click
            // restores the main window; the menu offers explicit 显示/退出. This
            // is what makes "close/minimize then click the tray icon" actually
            // bring the GUI back instead of leaving a dead icon.
            {
                use tauri::menu::{Menu, MenuItem};
                use tauri::tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent};

                let show_i = MenuItem::with_id(app, "tray_show", "显示云雀", true, None::<&str>)?;
                let quit_i = MenuItem::with_id(app, "tray_quit", "退出", true, None::<&str>)?;
                let menu = Menu::with_items(app, &[&show_i, &quit_i])?;

                let mut builder = TrayIconBuilder::with_id("yunque-tray")
                    .tooltip("云雀 Agent")
                    .menu(&menu)
                    .show_menu_on_left_click(false)
                    .on_menu_event(|app, event| match event.id.as_ref() {
                        "tray_show" => show_main_window(app),
                        "tray_quit" => quit_app(app),
                        _ => {}
                    })
                    .on_tray_icon_event(|tray, event| {
                        if let TrayIconEvent::Click {
                            button: MouseButton::Left,
                            button_state: MouseButtonState::Up,
                            ..
                        } = event
                        {
                            show_main_window(tray.app_handle());
                        }
                    });
                if let Some(icon) = app.default_window_icon().cloned() {
                    builder = builder.icon(icon);
                }
                builder.build(app)?;
            }

            register_selection_shortcut(&handle);
            // Global drag-to-select assistant: OFF unless the user opted in.
            // When off we install nothing (no WH_MOUSE_LL hook, no worker, no
            // prewarmed popup) — Settings flips it on at runtime.
            let selection_enabled = read_selection_pref(&handle);
            SELECTION_ENABLED.store(selection_enabled, Ordering::SeqCst);
            if selection_enabled {
                let sel_handle = handle.clone();
                tauri::async_runtime::spawn(async move {
                    tokio::time::sleep(Duration::from_secs(3)).await;
                    ensure_selection_capture_installed(&sel_handle);
                });
            }
            tauri::async_runtime::spawn(async move {
                launch_backend(&handle).await;
            });
            Ok(())
        })
        .on_window_event(|window, event| match event {
            // Close = hide to tray (keep the agent resident + backend alive).
            // The real quit is the tray "退出" menu (-> quit_app) or teardown.
            tauri::WindowEvent::CloseRequested { api, .. } => {
                if window.label() == "main" {
                    save_ball_pos_from(window);
                    let _ = window.hide();
                    api.prevent_close();
                }
            }
            // Real teardown (tray quit / app.exit): persist + reap the sidecar.
            tauri::WindowEvent::Destroyed => {
                if window.label() == "main" {
                    save_ball_pos_from(window);
                    if let Some(state) = window.try_state::<BackendState>() {
                        state.graceful_kill();
                    }
                }
            }
            tauri::WindowEvent::Focused(false) => {
                if window.label() == "floating-panel" {
                    let _ = window.hide();
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

    // Pre-flight the port before spawning. This turns the old failure mode —
    // "stale sidecar holds :9090 → we spawn a second Go process that can't bind
    // → 60s spinner → generic timeout → relaunch still fails" — into an
    // immediate, correct outcome:
    if port_is_occupied(port).await {
        if probe_healthz(port).await {
            // A healthy Yunque backend is already here (e.g. a previous instance
            // still resident, or an operator-run headless server). Reuse it
            // instead of spawning a competing process that can't bind the port.
            log::info!("backend already healthy on port {port}; reusing existing instance");
            if let Err(e) = handle.emit(
                EVT_READY,
                ReadyPayload { port, elapsed_ms: start.elapsed().as_millis() },
            ) {
                log::debug!("emit {EVT_READY} failed: {e}");
            }
            return;
        }
        // Port is taken but not by a healthy backend — surface it now rather
        // than making the user wait out the full 60s health timeout.
        let msg = format!("端口 {port} 已被占用，且不是可用的云雀后端。请关闭占用该端口的程序（或已在运行的旧实例）后重试。");
        log::error!("port {port} occupied by a non-Yunque listener; aborting spawn");
        emit_error(handle, &msg, port);
        return;
    }

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
            let mut cmd = build_backend_command(bin_path, port, data_dir.as_deref());

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
        // Floating ball disabled by default — users can enable via settings.
        // create_floating_ball(handle, port);
        if let Err(e) = handle.emit(
            EVT_READY,
            ReadyPayload {
                port,
                elapsed_ms: start.elapsed().as_millis(),
            },
        ) {
            log::debug!("emit {EVT_READY} failed: {e}");
        }
        // Only watch a sidecar WE spawned. When we're reusing an external /
        // operator-run backend (sidecar_started == false), its lifetime isn't
        // ours to manage, so we don't monitor or restart it.
        if sidecar_started {
            if let Some(ref bin_path) = backend_path {
                spawn_backend_watchdog(handle.clone(), bin_path.clone(), port, data_dir.clone());
            }
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

/// Build the sidecar launch command with the exact env the desktop backend
/// needs. Shared by the initial launch and the watchdog's restart path so a
/// respawned process gets identical loopback binding / CORS / data-dir config —
/// a restart that silently dropped ALLOWED_ORIGINS would come back up but every
/// UI fetch would then be CORS-blocked, which is worse than staying down.
fn build_backend_command(bin_path: &std::path::Path, port: u16, data_dir: Option<&std::path::Path>) -> Command {
    let mut cmd = Command::new(bin_path);
    cmd.env("OPEN_BROWSER", "false").env("HIDE_CONSOLE", "true");
    if let Some(dd) = data_dir {
        cmd.env("YUNQUE_DATA_DIR", dd.to_string_lossy().to_string());
    }
    // Desktop is a local-only app: force loopback binding so the backend's
    // fail-closed "production-like" heuristic doesn't reject startup because of
    // a weak/empty JWT_SECRET on first run. Respect an operator override if
    // AGENT_ADDR is already in the parent env.
    if std::env::var_os("AGENT_ADDR").is_none() {
        cmd.env("AGENT_ADDR", format!("127.0.0.1:{port}"));
    }
    // Tag the process so Go-side warnings can tell this is the GUI wrapper.
    cmd.env("YUNQUE_LAUNCHER", "tauri-desktop");
    // The webview origin is tauri.localhost / tauri://localhost — NOT a loopback
    // host the backend auto-trusts. Whitelist it so the loopback-bound backend
    // accepts the shell's cross-origin fetches. Respect an operator override.
    if std::env::var_os("ALLOWED_ORIGINS").is_none() {
        cmd.env(
            "ALLOWED_ORIGINS",
            "http://tauri.localhost,https://tauri.localhost,tauri://localhost",
        );
    }
    // On Windows the sidecar MUST live in its own process group so
    // GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT) does not also kill us.
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        cmd.creation_flags(CREATE_NEW_PROCESS_GROUP);
    }
    cmd
}

/// How many times the watchdog will try to respawn a crashed sidecar before
/// giving up and asking the user to relaunch. Bounded so a hard crash-loop
/// (misconfig, corrupt DB) doesn't peg the CPU restarting forever.
const MAX_BACKEND_RESTARTS: u32 = 3;
/// Backoff between crash-restart attempts. Fixed rather than exponential — a
/// local process either comes back in a couple seconds or it's genuinely broken.
const RESTART_BACKOFF: Duration = Duration::from_secs(2);
/// How often the watchdog checks whether the sidecar is still alive.
const WATCHDOG_POLL: Duration = Duration::from_secs(2);

/// Watch the running sidecar and restart it if it dies unexpectedly.
///
/// Distinguishing an unexpected crash from our own deliberate shutdown is the
/// crux: `graceful_kill` / `Drop` both `take()` the child out of the shared
/// slot. So when the watchdog finds the slot empty, that's "we shut it down on
/// purpose" → stop watching, don't restart. Only a child that is *still in the
/// slot* but has *exited on its own* counts as a crash worth restarting.
fn spawn_backend_watchdog(
    handle: AppHandle,
    bin_path: std::path::PathBuf,
    port: u16,
    data_dir: Option<std::path::PathBuf>,
) {
    tauri::async_runtime::spawn(async move {
        let mut restarts: u32 = 0;
        loop {
            tokio::time::sleep(WATCHDOG_POLL).await;

            let Some(state) = handle.try_state::<BackendState>() else {
                return; // app tearing down
            };

            // Inspect the child without holding the lock across an await.
            let exited = {
                let mut guard = lock_or_recover(&state.child);
                match guard.as_mut() {
                    // Slot emptied by graceful_kill/Drop → intentional shutdown.
                    None => return,
                    Some(child) => match child.try_wait() {
                        Ok(Some(status)) => {
                            log::warn!("sidecar exited unexpectedly (status={status:?})");
                            // Drop the dead handle so a later graceful_kill is a no-op.
                            *guard = None;
                            true
                        }
                        Ok(None) => false, // still running
                        Err(e) => {
                            log::warn!("watchdog try_wait error: {e}");
                            false
                        }
                    },
                }
            };

            if !exited {
                continue;
            }

            if restarts >= MAX_BACKEND_RESTARTS {
                log::error!("sidecar crashed {restarts} times; giving up on auto-restart");
                emit_error(
                    &handle,
                    "本地后端多次异常退出，已停止自动重启。请查看日志或退出后重新启动应用。",
                    port,
                );
                return;
            }

            restarts += 1;
            log::info!("watchdog restarting sidecar (attempt {restarts}/{MAX_BACKEND_RESTARTS})");
            emit_error(&handle, "本地后端已退出，正在自动重启…", port);
            tokio::time::sleep(RESTART_BACKOFF).await;

            // A crash may have freed the port only after a TIME_WAIT delay; if
            // it's still held we can't rebind, so surface that instead of
            // spawning a doomed process.
            match build_backend_command(&bin_path, port, data_dir.as_deref()).spawn() {
                Ok(child) => {
                    log::info!("sidecar restarted (pid={})", child.id());
                    if let Some(state) = handle.try_state::<BackendState>() {
                        *lock_or_recover(&state.child) = Some(child);
                    }
                    // Re-confirm health so the UI can recover from its banner.
                    let restart_start = Instant::now();
                    if wait_for_healthy(&handle, port, restart_start).await {
                        log::info!("sidecar healthy again after restart");
                        restarts = 0; // healthy run resets the budget
                        if let Err(e) = handle.emit(
                            EVT_READY,
                            ReadyPayload { port, elapsed_ms: restart_start.elapsed().as_millis() },
                        ) {
                            log::debug!("emit {EVT_READY} after restart failed: {e}");
                        }
                    } else {
                        log::warn!("restarted sidecar never became healthy");
                    }
                }
                Err(e) => {
                    log::error!("watchdog failed to restart sidecar: {e}");
                    emit_error(&handle, &format!("本地后端重启失败：{e}"), port);
                }
            }
        }
    });
}

/// Backend port exposed to the webview so the frontend can target the live Go
/// sidecar even when AGENT_ADDR / auto-pick selects a non-default port. Keeps
/// the desktop dev (next dev on :3001 + sidecar on :PORT) from polling a dead
/// :9090 when the build-time NEXT_PUBLIC_API_BASE and the runtime port differ.
#[tauri::command]
fn backend_port() -> u16 {
    resolve_backend_port()
}

/// Parse `AGENT_ADDR` (e.g. `":9090"`, `"0.0.0.0:9090"`, `"[::]:9090"`,
/// `"localhost:9090"`). Falls back to `DEFAULT_BACKEND_PORT` on any
/// parse error — we prefer a live probe against the default over refusing
/// to launch.
/// Next.js / bundled UI port. Auxiliary webviews (selection popup, floating
/// panel) must load the frontend, NOT the Go API port.
fn resolve_frontend_port() -> u16 {
    if let Ok(raw) = std::env::var("YUNQUE_FRONTEND_PORT") {
        if let Ok(p) = raw.parse::<u16>() {
            if p != 0 {
                return p;
            }
        }
    }
    DEFAULT_FRONTEND_PORT
}

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

/// Can we open a TCP connection to `127.0.0.1:port`? A successful connect
/// means *something* is already listening there. Combined with `probe_healthz`
/// this lets `launch_backend` tell three cases apart before spending 60s on a
/// health wait:
///   - connect ok + healthz ok  → a live Yunque backend we can just reuse
///   - connect ok + healthz bad → the port is held by something else (or a
///                                half-dead sidecar) → fail fast with a clear msg
///   - connect fails            → the port is free → spawn normally
async fn port_is_occupied(port: u16) -> bool {
    let addr = format!("127.0.0.1:{port}");
    matches!(
        tokio::time::timeout(HTTP_PROBE_TIMEOUT, TokioTcpStream::connect(&addr)).await,
        Ok(Ok(_))
    )
}

/// Minimal async HTTP/1.1 GET /healthz probe. Returns true on `2xx`.
/// We deliberately avoid pulling in `reqwest` just for one request: the
/// status line is all we need, and Go's /healthz returns `200 OK` with a
/// tiny body.
///
/// IMPORTANT: a single `read()` does NOT guarantee we get the full status
/// line in one shot. TCP is a stream — the kernel may hand us 8 bytes now
/// and the next 4 bytes a millisecond later. The original implementation
/// returned `false` whenever the first read was short, causing spurious
/// "unhealthy" verdicts and (worse) `launch_backend` killing a perfectly
/// good sidecar after timeout. We now loop until we have the 12 status
/// bytes or the peer closes, whichever comes first.
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

        // Read until we have at least the 12-byte status line ("HTTP/1.1 2xx"),
        // or EOF / buffer fills. 64 bytes is far more than we need but keeps
        // the syscall count low on real responses.
        let mut buf = [0u8; 64];
        let mut filled = 0usize;
        while filled < 12 {
            match s.read(&mut buf[filled..]).await? {
                0 => break, // peer closed before sending a full status line
                n => filled += n,
            }
        }
        Ok::<_, std::io::Error>((buf, filled))
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
