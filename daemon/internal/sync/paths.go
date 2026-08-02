package sync

import (
	"fmt"
	"path/filepath"
	"strings"
)

// RelWithin reports whether path is contained in root and returns the
// relative path from root. The root itself yields (".", true).
//
// Containment is proven with filepath.Rel plus explicit "." / ".."
// rejection rather than a path-prefix string comparison, so traversal,
// absolute paths, and separators embedded in names cannot escape the root.
// Windows volume and case-collision semantics additionally require native
// platform tests; this helper is deliberately platform-aware.
func RelWithin(root, path string) (string, bool) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

// ResolveRoot returns the first watch root containing path.
// Resolution is order-first, matching the historical engine behavior;
// nested roots therefore resolve to the earlier entry.
func ResolveRoot(roots []string, path string) (string, bool) {
	for _, root := range roots {
		if _, ok := RelWithin(root, path); ok {
			return root, true
		}
	}
	return "", false
}

// SafeJoin joins a remote-derived name into root and proves the result is
// still inside root. Remote names are untrusted data: Drive permits almost
// any byte sequence, and a hostile or corrupt name must never map outside
// the watch root.
//
// The name is rejected when empty, "." or "..", when it embeds the "/"
// separator (Drive forbids "/" in names; rejecting it also blocks the
// cross-platform separator hazard and traversal attempts), and when it
// collides with a Windows reserved device name. Names containing "\"
// remain literal components on Unix (backslash is not a separator there);
// on Windows the platform join treats them as separators, and the final
// RelWithin proof keeps every result inside root either way.
func SafeJoin(root, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty remote name")
	}
	if name == "." || name == ".." {
		return "", fmt.Errorf("unsafe remote name %q", name)
	}
	if strings.ContainsRune(name, '/') {
		return "", fmt.Errorf("unsafe remote name %q", name)
	}
	if isWindowsReservedName(name) {
		return "", fmt.Errorf("remote name %q is reserved on Windows", name)
	}
	joined := filepath.Join(root, name)
	if rel, ok := RelWithin(root, joined); !ok || rel == "." {
		return "", fmt.Errorf("remote name %q escapes watch root", name)
	}
	return joined, nil
}

// isWindowsReservedName reports whether name collides with a Windows
// reserved device name (CON, PRN, AUX, NUL, COM1-9, LPT1-9), with or
// without extension, case-insensitively. COM10+ and COM1x are ordinary
// names and are not reserved.
func isWindowsReservedName(name string) bool {
	base := name
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	base = strings.ToUpper(base)
	if strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT") {
		suffix := base[3:]
		return len(suffix) == 1 && suffix[0] >= '1' && suffix[0] <= '9'
	}
	switch base {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	return false
}

// IsTempFile reports whether the base name looks like an editor swap file,
// hidden file, or partial download that must never be synchronized.
func IsTempFile(path string) bool {
	base := filepath.Base(path)
	if len(base) == 0 {
		return false
	}
	// Skip hidden files, temp editors, swap files
	if base[0] == '.' {
		return true
	}
	switch filepath.Ext(base) {
	case ".swp", ".swx", ".tmp", ".part", ".crdownload":
		return true
	}
	// Vim/Emacs temp
	return base[len(base)-1] == '~'
}

// DriveErrorMessage maps known Drive API failure text to a user-safe
// explanation, leaving every other message untouched.
func DriveErrorMessage(errMsg string) string {
	if strings.Contains(errMsg, "403") || strings.Contains(errMsg, "400") {
		return "path too deep: Drive limit is 100 nested folders"
	}
	return errMsg
}
