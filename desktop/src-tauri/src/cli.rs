//! When `synca` is invoked with CLI args (e.g. `synca connect google-drive`), forward to the
//! Go binary (`synca-daemon`). The Tauri UI is only used for a bare `synca` with no args.

use std::path::PathBuf;
use std::process::Command;

#[cfg(windows)]
const DAEMON_BIN: &str = "synca-daemon.exe";
#[cfg(not(windows))]
const DAEMON_BIN: &str = "synca-daemon";

fn synca_daemon_path() -> Option<PathBuf> {
    let exe = std::env::current_exe().ok()?;
    let dir = exe.parent()?;
    let sidecar = dir.join(DAEMON_BIN);
    if sidecar.is_file() {
        return Some(sidecar);
    }
    #[cfg(debug_assertions)]
    {
        let dev = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../bin/synca-daemon");
        if dev.is_file() {
            return Some(dev);
        }
    }
    None
}

/// If argv has CLI args, run `synca-daemon` with them (`exec` on Unix) or exit.
/// Returns only when there are no CLI args (launch GUI).
pub fn forward_to_daemon_if_cli() {
    let args: Vec<String> = std::env::args().collect();
    if args.len() <= 1 {
        return;
    }

    let Some(bin) = synca_daemon_path() else {
        eprintln!(
      "synca: could not find {DAEMON_BIN} next to this program (or ../../bin/synca-daemon in dev)."
    );
        eprintln!("synca: build the daemon with: make daemon");
        std::process::exit(1);
    };

    #[cfg(unix)]
    {
        use std::os::unix::process::CommandExt;
        let err = Command::new(&bin).args(&args[1..]).exec();
        eprintln!("synca: failed to run {}: {err}", bin.display());
        std::process::exit(1);
    }

    #[cfg(windows)]
    {
        match Command::new(&bin).args(&args[1..]).status() {
            Ok(status) => std::process::exit(status.code().unwrap_or(1)),
            Err(e) => {
                eprintln!("synca: failed to run {}: {e}", bin.display());
                std::process::exit(1);
            }
        }
    }
}
