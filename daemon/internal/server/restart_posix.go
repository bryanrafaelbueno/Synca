//go:build !windows

package server

import (
	"os"
	"path/filepath"
	"syscall"
)

// restartProcessAsDaemon replaces this process with the same binary running the daemon subcommand.
// Same PID on Linux (execve), so systemd does not see a stop/start; also works without systemd (e.g. make dev).
func restartProcessAsDaemon() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, e := filepath.EvalSymlinks(exe); e == nil {
		exe = resolved
	}
	name := filepath.Base(exe)
	argv := []string{name, "daemon"}
	return syscall.Exec(exe, argv, os.Environ())
}
