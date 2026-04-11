// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod cli;

use tauri_plugin_shell::ShellExt;
use tauri::Manager;

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

#[tauri::command]
fn has_credentials(app: tauri::AppHandle) -> bool {
    if let Ok(mut path) = app.path().home_dir() {
        path.push(".config");
        path.push("synca");
        path.push("credentials.json");
        return path.exists();
    }
    false
}

#[tauri::command]
fn save_credentials(app: tauri::AppHandle, source_path: String) -> Result<(), String> {
    let mut dest = app.path().home_dir().map_err(|e| e.to_string())?;
    dest.push(".config");
    dest.push("synca");
    if !dest.exists() {
        std::fs::create_dir_all(&dest).map_err(|e| e.to_string())?;
    }
    dest.push("credentials.json");
    std::fs::copy(&source_path, &dest).map_err(|e| e.to_string())?;
    Ok(())
}

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
                Err(e) => Err(format!("Falha ao spawnar sidecar (daemon): {}", e))
            }
        }
        Err(e) => Err(format!("Falha ao instanciar sidecar (daemon): {}", e))
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
        .invoke_handler(tauri::generate_handler![
            login_google_drive, 
            has_credentials, 
            save_credentials, 
            has_token, 
            start_daemon,
            restart_daemon
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
