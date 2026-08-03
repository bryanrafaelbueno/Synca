package auth

import (
	"errors"
	"os/exec"
	"runtime"

	"github.com/rs/zerolog/log"
)

// openBrowser opens url in the user's default browser. It returns an error
// instead of failing silently so the OAuth flow can surface the failure and
// offer the URL for manual sign-in. On Linux it falls back through common
// launchers because xdg-open may be missing or broken in sandboxed (AppImage)
// environments; portal-based desktops need DISPLAY/WAYLAND_DISPLAY and
// XDG_RUNTIME_DIR in the process environment.
func openBrowser(url string) error {
	var candidates []string
	switch runtime.GOOS {
	case "linux":
		candidates = []string{"xdg-open", "x-www-browser", "sensible-browser"}
	case "darwin":
		candidates = []string{"open"}
	case "windows":
		candidates = []string{"rundll32"}
	default:
		return errors.New("no browser launcher for this platform")
	}

	var lastErr error
	for _, launcher := range candidates {
		args := []string{url}
		if runtime.GOOS == "windows" && launcher == "rundll32" {
			args = []string{"url.dll,FileProtocolHandler", url}
		}
		cmd := exec.Command(launcher, args...)
		if err := cmd.Start(); err != nil {
			lastErr = err
			log.Warn().Str("launcher", launcher).Err(err).Msg("Failed to start browser launcher")
			continue
		}
		go func(launcher string) {
			if err := cmd.Wait(); err != nil {
				log.Warn().Str("launcher", launcher).Err(err).Msg("Browser launcher exited with an error")
			}
		}(launcher)
		return nil
	}
	return lastErr
}
