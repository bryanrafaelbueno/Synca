//go:build windows

package server

import "errors"

func restartProcessAsDaemon() error {
	return errors.New("re-exec not supported on Windows")
}
