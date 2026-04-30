use std::process::{Child, Command};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Mutex;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use serde::{Deserialize, Serialize};
use tauri::{AppHandle, Emitter, Manager, State, Theme, WebviewUrl, WebviewWindow};
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpStream as TokioTcpStream;
use tauri_plugin_clipboard_manager::ClipboardExt;
use tauri_plugin_global_shortcut::{GlobalShortcutExt, Shortcut, ShortcutState};

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
    for (_label, win) in handle.webview_windows() {
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
        if let Ok(json) = serde_json::to_string_pretty(&*items) {
            let _ = std::fs::write(path, json);
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
        if let Ok(json) = serde_json::to_string(&pos) {
            let _ = std::fs::write(path, json);
        }
    }

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
    let port = resolve_backend_port();

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

    let url = format!("http://127.0.0.1:{port}/floating-panel");
    let (x, y) = panel_position_from_ball(&handle);

    let builder = tauri::WebviewWindowBuilder::new(
        &handle,
        "floating-panel",
        WebviewUrl::External(url.parse().unwrap()),
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

fn position_panel_near_ball(handle: &AppHandle, panel: &tauri::WebviewWindow) {
    if let Some(ball) = handle.get_webview_window("floating-ball") {
        if let Ok(pos) = ball.outer_position() {
            let _ = panel.set_position(tauri::PhysicalPosition::new(
                pos.x - 270,
                pos.y - 460,
            ));
        }
    }
}

fn panel_position_from_ball(handle: &AppHandle) -> (f64, f64) {
    if let Some(ball) = handle.get_webview_window("floating-ball") {
        if let Ok(pos) = ball.outer_position() {
            return ((pos.x - 270) as f64, (pos.y - 460) as f64);
        }
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
    if let Some(main) = handle.get_webview_window("main") {
        let js = format!(
            "document.dispatchEvent(new CustomEvent('yunque:quick-send', {{ detail: {} }}));",
            serde_json::to_string(&text).unwrap_or_default()
        );
        let _ = main.eval(&js);
        let _ = main.show();
        let _ = main.set_focus();
    }
    if let Some(panel) = handle.get_webview_window("floating-panel") {
        let _ = panel.hide();
    }
}

fn create_floating_ball(handle: &AppHandle, port: u16) {
    let url = format!("http://127.0.0.1:{port}/floating-ball");

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
        WebviewUrl::External(url.parse().unwrap()),
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

fn register_selection_shortcut(handle: &AppHandle) {
    let shortcut: Shortcut = "Alt+Y".parse().unwrap();
    let handle2 = handle.clone();
    if let Err(e) = handle.global_shortcut().on_shortcut(shortcut, move |_app, _sc, event| {
        if event.state != ShortcutState::Pressed {
            return;
        }
        log::info!("global shortcut Alt+Y triggered");
        let h = handle2.clone();
        tauri::async_runtime::spawn(async move {
            let old_clip = h.clipboard().read_text().unwrap_or_default();
            simulate_ctrl_c();
            tokio::time::sleep(Duration::from_millis(150)).await;
            let new_clip = h.clipboard().read_text().unwrap_or_default();
            let text = if new_clip != old_clip && !new_clip.trim().is_empty() {
                new_clip.trim().to_string()
            } else if !old_clip.trim().is_empty() {
                old_clip.trim().to_string()
            } else {
                return;
            };
            if text.len() < 2 {
                return;
            }

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

            show_floating_panel(&h);
        });
    }) {
        log::error!("failed to register global shortcut: {e}");
    }
}

fn show_floating_panel(handle: &AppHandle) {
    if let Some(panel) = handle.get_webview_window("floating-panel") {
        position_panel_near_ball(handle, &panel);
        let _ = panel.show();
        let _ = panel.set_focus();
    } else {
        toggle_floating_panel(handle.clone());
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
            toggle_floating_panel,
            get_floating_items,
            get_floating_count,
            add_floating_item,
            remove_floating_item,
            clear_floating_items,
            floating_send_to_chat,
            apply_window_theme,
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
            if let Some(main) = handle.get_webview_window("main") {
                #[cfg(target_os = "macos")]
                {
                    let _ = main.set_decorations(true);
                    let _ = main.set_title_bar_style(tauri::TitleBarStyle::Overlay);
                }
                apply_window_appearance(&main, "dark");
            }

            register_selection_shortcut(&handle);
            tauri::async_runtime::spawn(async move {
                launch_backend(&handle).await;
            });
            Ok(())
        })
        .on_window_event(|window, event| match event {
            tauri::WindowEvent::CloseRequested { .. } | tauri::WindowEvent::Destroyed => {
                if window.label() == "main" {
                    if let Some(ball) = window.app_handle().get_webview_window("floating-ball") {
                        if let Ok(pos) = ball.outer_position() {
                            if let Some(fs) = window.try_state::<FloatingState>() {
                                fs.save_ball_pos(pos.x as f64, pos.y as f64);
                            }
                        }
                    }
                    if let Some(state) = window.try_state::<BackendState>() {
                        state.graceful_kill();
                    }
                }
            }
            tauri::WindowEvent::Focused(false) => {
                if window.label() == "selection-popup" {
                    let _ = window.hide();
                }
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
