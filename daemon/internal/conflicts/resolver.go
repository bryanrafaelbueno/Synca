package conflicts

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
)

// ConflictStrategy determines how conflicts are resolved.
type ConflictStrategy int

const (
	// StrategyKeepBoth creates a versioned copy of the conflicting file.
	StrategyKeepBoth ConflictStrategy = iota
	// StrategyNewerWins keeps the file with the later modification time.
	StrategyNewerWins
	// StrategyLocalWins always keeps the local version.
	StrategyLocalWins
	// StrategyRemoteWins always keeps the remote version.
	StrategyRemoteWins
)

// Conflict represents a detected conflict between local and remote files.
type Conflict struct {
	LocalPath    string
	RemoteName   string
	LocalModTime time.Time
	RemoteModTime time.Time
	LocalMD5     string
	RemoteMD5    string
}

// Detector checks whether a local file conflicts with a remote file.
type Detector struct {
	conflictDir string
	strategy    ConflictStrategy
}

func NewDetector(conflictDir string, strategy ConflictStrategy) *Detector {
	_ = os.MkdirAll(conflictDir, 0755)
	return &Detector{conflictDir: conflictDir, strategy: strategy}
}

// HasConflict returns true if the local file has changed since last sync
// AND the remote file has also changed (both sides modified).
func (d *Detector) HasConflict(localPath string, localModTime, remoteModTime, lastSyncTime time.Time) bool {
	if lastSyncTime.IsZero() {
		return false
	}
	localChanged := localModTime.After(lastSyncTime)
	remoteChanged := remoteModTime.After(lastSyncTime)
	return localChanged && remoteChanged
}

// Resolve handles a detected conflict according to the configured strategy.
// It returns the path that should be uploaded to Drive and whether the local
// file was replaced.
func (d *Detector) Resolve(c *Conflict) (uploadPath string, localReplaced bool, err error) {
	switch d.strategy {
	case StrategyKeepBoth:
		return d.keepBoth(c)
	case StrategyNewerWins:
		return d.newerWins(c)
	case StrategyLocalWins:
		return c.LocalPath, false, nil
	case StrategyRemoteWins:
		// Caller handles downloading the remote version
		return "", true, nil
	default:
		return d.keepBoth(c)
	}
}

// keepBoth renames the local file to a conflict copy and returns the original path.
// Example: notes.md → notes (conflicted copy 2024-01-15).md
func (d *Detector) keepBoth(c *Conflict) (string, bool, error) {
	ext := filepath.Ext(c.LocalPath)
	base := c.LocalPath[:len(c.LocalPath)-len(ext)]
	ts := time.Now().Format("2006-01-02 15-04-05")
	conflictPath := fmt.Sprintf("%s (conflicted copy %s)%s", base, ts, ext)

	log.Warn().
		Str("original", c.LocalPath).
		Str("conflict_copy", conflictPath).
		Msg("Conflict detected — keeping both versions")

	if err := copyFile(c.LocalPath, conflictPath); err != nil {
		return "", false, err
	}

	// Also save a copy to the dedicated conflicts dir for visibility
	conflictBackup := filepath.Join(d.conflictDir, filepath.Base(conflictPath))
	_ = copyFile(c.LocalPath, conflictBackup)

	return conflictPath, false, nil
}

func (d *Detector) newerWins(c *Conflict) (string, bool, error) {
	if c.LocalModTime.After(c.RemoteModTime) {
		log.Info().Str("file", c.LocalPath).Msg("Conflict: local is newer, keeping local")
		return c.LocalPath, false, nil
	}
	log.Info().Str("file", c.LocalPath).Msg("Conflict: remote is newer, will download remote")
	return "", true, nil
}

// MD5File computes the MD5 checksum of a file.
func MD5File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
