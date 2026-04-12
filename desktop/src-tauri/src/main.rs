// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod cli;

use tauri_plugin_shell::ShellExt;
use tauri::{Manager, menu::{MenuBuilder, MenuItemBuilder}, tray::TrayIconBuilder, WindowEvent};

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
#[tauri::command]
fn has_token(app: tauri::AppHandle) -> bool {
    if let Ok(mut path) = app.path().home_dir() {
        path.push(".config");
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
fn start_daemon(app: tauri::AppHandle, state: tauri::State<'_, DaemonState>) -> Result<(), String> {
    let mut child_guard = state.0.lock().unwrap();
    
    if let Some(child) = child_guard.take() {
        let _ = child.kill();
    }
    
    match app.shell().sidecar("synca-daemon") {
        Ok(sidecar) => {
            match sidecar.args(["daemon"]).spawn() {
                Ok((_rx, child)) => {
                    *child_guard = Some(child);
                    Ok(())
                }
                Err(e) => Err(format!("Failed to spawn sidecar (daemon): {}", e))
            }
        }
        Err(e) => Err(format!("Failed to instantiate sidecar (daemon): {}", e))
    }
}

#[tauri::command]
fn restart_daemon(app: tauri::AppHandle, state: tauri::State<'_, DaemonState>) -> Result<(), String> {
    start_daemon(app, state)
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
            restart_daemon
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
