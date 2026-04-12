// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod cli;

use tauri_plugin_shell::ShellExt;
use tauri_plugin_dialog::DialogExt;
use tauri::{Manager, menu::{MenuBuilder, MenuItemBuilder}, tray::TrayIconBuilder, WindowEvent};

/// Detecta se está rodando como AppImage
fn is_appimage() -> bool {
    std::env::var("APPIMAGE").is_ok() || std::env::var("APPDIR").is_ok()
}

#[tauri::command]
async fn login_google_drive(app: tauri::AppHandle) -> Result<String, String> {
    let sidecar = app.shell().sidecar("synca-daemon").map_err(|e| e.to_string())?;
    let out = sidecar.args(["connect", "google-drive"]).output().await.map_err(|e| e.to_string())?;
    if out.status.success() {
        Ok("Login OK".into())
    } else {
        Err(String::from_utf8_lossy(&out.stderr).into_owned())
    }
}

// Simplified setup: only check for token.json
// Must use the SAME path that the Go daemon uses (os.UserConfigDir + "synca")
#[tauri::command]
fn has_token(_app: tauri::AppHandle) -> bool {
    // Go's os.UserConfigDir() returns:
    //   Linux:   $HOME/.config
    //   Windows: %APPDATA%
    //   macOS:   $HOME/Library/Application Support
    let config_base = {
        #[cfg(target_os = "windows")]
        {
            std::env::var("APPDATA").ok()
        }
        #[cfg(target_os = "macos")]
        {
            std::env::var("HOME").map(|h| format!("{}/Library/Application Support", h)).ok()
        }
        #[cfg(not(any(target_os = "windows", target_os = "macos")))]
        {
            std::env::var("XDG_CONFIG_HOME")
                .ok()
                .or_else(|| std::env::var("HOME").ok().map(|h| format!("{}/.config", h)))
        }
    };

    if let Some(base) = config_base {
        let mut path = std::path::PathBuf::from(base);
        path.push("synca");
        path.push("token.json");
        return path.exists();
    }
    false
}

use std::sync::Mutex;
use tauri_plugin_shell::process::CommandChild;

struct DaemonState(Mutex<Option<CommandChild>>);

#[tauri::command]
fn is_appimage_cmd() -> bool {
    is_appimage()
}

/// Folder picker implemented on the Rust side to avoid JS capability issues
#[tauri::command]
async fn pick_folder_dialog(app: tauri::AppHandle) -> Result<Option<String>, String> {
    let (tx, rx) = std::sync::mpsc::channel();
    app.dialog()
        .file()
        .set_title("Choose folder to sync")
        .pick_folder(move |path: Option<tauri_plugin_dialog::FilePath>| {
            use tauri_plugin_dialog::FilePath;
            let result = path.and_then(|p| match p {
                FilePath::Path(pb) => pb.to_str().map(|s| s.to_string()),
                FilePath::Url(u) => Some(u.to_string()),
            });
            let _ = tx.send(result);
        });
    Ok(rx.recv().map_err(|e| format!("Dialog channel error: {e}"))?)
}

#[tauri::command]
async fn start_daemon(app: tauri::AppHandle, state: tauri::State<'_, DaemonState>) -> Result<(), String> {
    // Kill existing daemon if running
    {
        let mut child_guard = state.0.lock().unwrap();
        if let Some(child) = child_guard.take() {
            let _ = child.kill();
        }
    } // Lock released here

    match app.shell().sidecar("synca-daemon") {
        Ok(sidecar) => {
            eprintln!("Starting synca-daemon sidecar...");

            // On Linux (especially AppImage), clear env to prevent LD_LIBRARY_PATH conflicts
            // On Windows, keep the default environment (removing SystemRoot, WINDIR, etc. breaks Go)
            #[cfg(target_os = "linux")]
            let sidecar_cmd = {
                let mut cmd = sidecar
                    .args(["daemon"])
                    .env_clear();
                // Only set env vars if they exist in the parent process.
                // Passing empty strings is worse than not setting the var at all,
                // because Go's os.UserConfigDir() will fallback to $HOME/.config.
                if let Ok(val) = std::env::var("HOME") {
                    cmd = cmd.env("HOME", val);
                }
                if let Ok(val) = std::env::var("PATH") {
                    cmd = cmd.env("PATH", val);
                }
                if let Ok(val) = std::env::var("XDG_CONFIG_HOME") {
                    cmd = cmd.env("XDG_CONFIG_HOME", val);
                }
                if let Ok(val) = std::env::var("DISPLAY") {
                    cmd = cmd.env("DISPLAY", val);
                }
                if let Ok(val) = std::env::var("WAYLAND_DISPLAY") {
                    cmd = cmd.env("WAYLAND_DISPLAY", val);
                }
                if let Ok(val) = std::env::var("XDG_RUNTIME_DIR") {
                    cmd = cmd.env("XDG_RUNTIME_DIR", val);
                }
                if let Ok(val) = std::env::var("GOOGLE_APPLICATION_CREDENTIALS") {
                    cmd = cmd.env("GOOGLE_APPLICATION_CREDENTIALS", val);
                }
                if let Ok(val) = std::env::var("XAUTHORITY") {
                    cmd = cmd.env("XAUTHORITY", val);
                }
                if let Ok(val) = std::env::var("DBUS_SESSION_BUS_ADDRESS") {
                    cmd = cmd.env("DBUS_SESSION_BUS_ADDRESS", val);
                }
                cmd
            };

            #[cfg(not(target_os = "linux"))]
            let sidecar_cmd = sidecar.args(["daemon"]);

            match sidecar_cmd.spawn() {
                Ok((mut rx, child)) => {
                    // Spawn a task to read stderr
                    let app_clone = app.clone();
                    tauri::async_runtime::spawn(async move {
                        while let Some(event) = rx.recv().await {
                            match event {
                                tauri_plugin_shell::process::CommandEvent::Stderr(bytes) => {
                                    let text = String::from_utf8_lossy(&bytes);
                                    eprintln!("[daemon stderr] {}", text.trim());
                                }
                                tauri_plugin_shell::process::CommandEvent::Stdout(bytes) => {
                                    let text = String::from_utf8_lossy(&bytes);
                                    eprintln!("[daemon stdout] {}", text.trim());
                                }
                                tauri_plugin_shell::process::CommandEvent::Terminated(payload) => {
                                    eprintln!("[daemon] terminated with code {:?}, signal {:?}", payload.code, payload.signal);
                                    // Clear the state since daemon died
                                    let state = app_clone.state::<DaemonState>();
                                    let mut guard = state.0.lock().unwrap();
                                    guard.take();
                                }
                                _ => {}
                            }
                        }
                    });

                    // Store child in state
                    {
                        let mut child_guard = state.0.lock().unwrap();
                        *child_guard = Some(child);
                    } // Lock released here

                    // Wait for daemon to be ready via health check
                    const MAX_RETRIES: u32 = 30;
                    const RETRY_DELAY: std::time::Duration = std::time::Duration::from_millis(500);

                    for i in 0..MAX_RETRIES {
                        tokio::time::sleep(RETRY_DELAY).await;
                        if let Ok(resp) = reqwest::get("http://localhost:7373/health").await {
                            if resp.status().is_success() {
                                eprintln!("Daemon health check passed");
                                return Ok(());
                            }
                        }
                        if i == 5 {
                            eprintln!("Waiting for daemon to be ready... (attempt {}/{})", i + 1, MAX_RETRIES);
                        }
                    }

                    // After all retries, check if daemon process is still alive
                    let guard = state.0.lock().unwrap();
                    if guard.is_none() {
                        eprintln!("Daemon process died during startup");
                        return Err("Daemon process died during startup".to_string());
                    }

                    eprintln!("Daemon health check timed out");
                    Err("Daemon failed to start: health check timed out".to_string())
                }
                Err(e) => {
                    eprintln!("Failed to spawn sidecar: {}", e);
                    Err(format!("Failed to spawn sidecar (daemon): {}", e))
                }
            }
        }
        Err(e) => {
            eprintln!("Failed to instantiate sidecar: {}", e);
            Err(format!("Failed to instantiate sidecar (daemon): {}", e))
        }
    }
}

#[tauri::command]
async fn restart_daemon(app: tauri::AppHandle, state: tauri::State<'_, DaemonState>) -> Result<(), String> {
    start_daemon(app, state).await
}

fn main() {
    cli::forward_to_daemon_if_cli();

    tauri::Builder::default()
        .manage(DaemonState(Mutex::new(None)))
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_autostart::init(tauri_plugin_autostart::MacosLauncher::LaunchAgent, None))
        .setup(|app| {
            // 1. Create Tray Menu Items
            let quit_i = MenuItemBuilder::with_id("quit", "Quit Synca").build(app)?;
            let show_i = MenuItemBuilder::with_id("show", "Show App").build(app)?;

            // 2. Build the Tray Menu
            let menu = MenuBuilder::new(app)
                .item(&show_i)
                .separator()
                .item(&quit_i)
                .build()?;

            // 3. Setup Tray Icon
            let _tray = TrayIconBuilder::new()
                .icon(app.default_window_icon().unwrap().clone())
                .menu(&menu)
                .show_menu_on_left_click(false)
                .on_menu_event(|app, event| {
                    match event.id.as_ref() {
                        "quit" => {
                            app.exit(0);
                        }
                        "show" => {
                            if let Some(window) = app.get_webview_window("main") {
                                let _ = window.show();
                                let _ = window.set_focus();
                            }
                        }
                        _ => {}
                    }
                })
                .build(app)?;

            Ok(())
        })
        .on_window_event(|window, event| {
            if let WindowEvent::CloseRequested { api, .. } = event {
                // Minimize to tray instead of closing
                api.prevent_close();
                let _ = window.hide();
            }
        })
        .invoke_handler(tauri::generate_handler![
            login_google_drive,
            has_token,
            start_daemon,
            restart_daemon,
            is_appimage_cmd,
            pick_folder_dialog
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
