//go:build windows

package server

import (
	"os"
	"os/exec"
	"syscall"
)

func restartProcessAsDaemon() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "daemon")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	// DETACHED_PROCESS (0x00000008) starts the process without an attached console window,
	// letting it run fully detached in the background.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000008,
	}
	return cmd.Start()
}
