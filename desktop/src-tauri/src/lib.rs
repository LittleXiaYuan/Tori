use std::process::{Child, Command};
use std::sync::Mutex;
use std::time::{Duration, Instant};
use tauri::{AppHandle, Manager};

const BACKEND_PORT: u16 = 9090;
const HEALTH_TIMEOUT: Duration = Duration::from_secs(60);
const POLL_INTERVAL: Duration = Duration::from_millis(500);

struct BackendState {
    child: Mutex<Option<Child>>,
}

impl BackendState {
    /// Kill the managed Go sidecar if one is still running. Safe to call
    /// repeatedly — the second call is a no-op because `take()` empties the
    /// slot. Both `kill()` and `wait()` errors are swallowed intentionally
    /// because we are shutting down and there is no meaningful recovery.
    fn kill_child(&self) {
        if let Ok(mut guard) = self.child.lock() {
            if let Some(mut child) = guard.take() {
                eprintln!("[yunque-desktop] terminating backend pid={}", child.id());
                let _ = child.kill();
                let _ = child.wait();
            }
        } else {
            eprintln!("[yunque-desktop] backend state mutex was poisoned; skipping kill");
        }
    }
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
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
                    state.kill_child();
                }
            }
            _ => {}
        })
        .run(tauri::generate_context!())
        .expect("failed to run yunque desktop");
}

async fn launch_backend(handle: &AppHandle) {
    let data_dir = if let Ok(app_data) = handle.path().app_data_dir() {
        let d = app_data.join("data");
        let _ = std::fs::create_dir_all(&d);
        eprintln!("[yunque-desktop] data dir: {}", d.display());
        Some(d)
    } else {
        eprintln!("[yunque-desktop] could not resolve app data dir");
        None
    };

    let mut search_dirs = Vec::new();
    if let Ok(res_dir) = handle.path().resource_dir() {
        search_dirs.push(res_dir);
    }
    if let Ok(exe) = std::env::current_exe() {
        if let Some(d) = exe.parent() {
            search_dirs.push(d.to_path_buf());
        }
    }
    eprintln!(
        "[yunque-desktop] searching for backend in: {:?}",
        search_dirs
            .iter()
            .map(|d| d.display().to_string())
            .collect::<Vec<_>>()
    );

    let backend_path = search_dirs
        .iter()
        .flat_map(|d| vec![d.join("yunque-agent.exe"), d.join("yunque-agent")])
        .find(|p| p.exists());

    let mut sidecar_started = false;

    if let Some(ref bin_path) = backend_path {
        if bin_path.exists() {
            eprintln!("[yunque-desktop] launching backend: {}", bin_path.display());
            let mut cmd = Command::new(bin_path);
            cmd.env("OPEN_BROWSER", "false")
                .env("HIDE_CONSOLE", "true");
            if let Some(ref dd) = data_dir {
                cmd.env("YUNQUE_DATA_DIR", dd.to_string_lossy().to_string());
            }

            match cmd.spawn() {
                Ok(child) => {
                    eprintln!(
                        "[yunque-desktop] backend process started (pid={})",
                        child.id()
                    );
                    let state = handle.state::<BackendState>();
                    if let Ok(mut guard) = state.child.lock() {
                        *guard = Some(child);
                        sidecar_started = true;
                    } else {
                        eprintln!("[yunque-desktop] backend state mutex poisoned on spawn");
                    }
                }
                Err(e) => {
                    eprintln!("[yunque-desktop] failed to start backend: {e}");
                }
            }
        } else {
            eprintln!(
                "[yunque-desktop] backend binary not found at: {}",
                bin_path.display()
            );
        }
    }

    if !sidecar_started {
        eprintln!(
            "[yunque-desktop] waiting for external backend on port {BACKEND_PORT}..."
        );
    }

    let start = Instant::now();
    let mut backend_ready = false;
    loop {
        if start.elapsed() > HEALTH_TIMEOUT {
            eprintln!(
                "[yunque-desktop] backend health check timed out after {}s",
                HEALTH_TIMEOUT.as_secs()
            );
            break;
        }
        if let Ok(addr) = format!("127.0.0.1:{}", BACKEND_PORT).parse::<std::net::SocketAddr>() {
            if std::net::TcpStream::connect_timeout(&addr, Duration::from_millis(200)).is_ok() {
                tokio::time::sleep(Duration::from_millis(300)).await;
                backend_ready = true;
                eprintln!(
                    "[yunque-desktop] backend ready on port {BACKEND_PORT} ({:.1}s)",
                    start.elapsed().as_secs_f64()
                );
                break;
            }
        }
        tokio::time::sleep(POLL_INTERVAL).await;
    }

    if backend_ready {
        if let Some(window) = handle.get_webview_window("main") {
            let url = format!("http://localhost:{}", BACKEND_PORT);
            let _ = window.eval(&format!("window.location.replace('{url}')"));
        }
    } else if sidecar_started {
        // We started a sidecar but it never came up — don't leave a zombie
        // Go process fighting for port 9090 while the user sees the timeout
        // screen. The HTML loader page will display a user-visible error.
        eprintln!("[yunque-desktop] killing unhealthy sidecar to avoid orphan process");
        let state = handle.state::<BackendState>();
        state.kill_child();
    }
}
