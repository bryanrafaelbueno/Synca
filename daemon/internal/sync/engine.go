package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/synca/daemon/internal/config"
	"github.com/synca/daemon/internal/conflicts"
	"github.com/synca/daemon/internal/drive"
	"github.com/synca/daemon/internal/watcher"
)

// FileStatus represents the sync state of a file.
type FileStatus int

const (
	StatusQueued FileStatus = iota
	StatusInitializing
	StatusUploading
	StatusVerifying
	StatusFinalizing
	StatusSynced
	StatusConflict
	StatusError
)

func (s FileStatus) String() string {
	switch s {
	case StatusSynced:
		return "synced"
	case StatusInitializing:
		return "initializing"
	case StatusUploading:
		return "uploading"
	case StatusVerifying:
		return "verifying"
	case StatusFinalizing:
		return "finalizing"
	case StatusQueued:
		return "queued"
	case StatusConflict:
		return "conflict"
	default:
		return "error"
	}
}

func (s FileStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *FileStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	switch str {
	case "synced":
		*s = StatusSynced
	case "initializing":
		*s = StatusInitializing
	case "uploading":
		*s = StatusUploading
	case "verifying":
		*s = StatusVerifying
	case "finalizing":
		*s = StatusFinalizing
	case "queued":
		*s = StatusQueued
	case "conflict":
		*s = StatusConflict
	default:
		*s = StatusError
	}
	return nil
}

// FileEntry tracks a file's sync state.
type FileEntry struct {
	LocalPath  string     `json:"local_path"`
	RemoteID   string     `json:"remote_id"`
	RemoteName string     `json:"remote_name"`
	Status     FileStatus `json:"status"`
	LastSync   time.Time  `json:"last_sync"`
	LocalMD5   string     `json:"local_md5"`
	RemoteMD5  string     `json:"remote_md5"`
	Size       int64      `json:"size"`
	IsDir      bool       `json:"is_dir"`
	ErrorMsg   string     `json:"error,omitempty"`
}

// StatusSnapshot is what the UI reads.
type StatusSnapshot struct {
	Files          []*FileEntry      `json:"files"`
	WatchPaths     []string          `json:"watch_paths"`
	WatchPathModes map[string]string `json:"watch_path_modes"`
	TotalBytes     int64             `json:"total_bytes"`
	TotalFiles     int               `json:"total_files"`
	SyncedFiles    int               `json:"synced_files"`
	IsRunning      bool              `json:"is_running"`
	LastUpdated    time.Time         `json:"last_updated"`
}

// Engine is the core sync orchestrator.
type Engine struct {
	cfg      *config.Config
	watcher  *watcher.Watcher
	drive    *drive.Client
	resolver *conflicts.Detector

	pathsMu sync.RWMutex // cfg.WatchPaths: coordinate with AddWatchRoot

	mu    sync.RWMutex
	files map[string]*FileEntry // keyed by localPath
	// remote state cache: remoteName → File
	remoteCache map[string]*drive.File
	// local directory path → remote folder ID
	folderCache map[string]string

	// Broadcast channel: UI subscribes to this
	Updates chan StatusSnapshot

	// work queue
	queue chan workItem

	// Throttled broadcast: coalesce rapid state changes
	broadcastDirty int32

	// Persistent state file path
	stateFile string
}

type workItem struct {
	event watcher.FileEvent
}

const (
	workerCount  = 4
	pollInterval = 30 * time.Second
)

func NewEngine(cfg *config.Config) (*Engine, error) {
	w, err := watcher.New()
	if err != nil {
		return nil, err
	}

	// Derive state file path from config dir (same dir as config.json)
	stateFile := filepath.Join(filepath.Dir(cfg.TokenFile), "sync_state.json")

	e := &Engine{
		cfg:         cfg,
		watcher:     w,
		files:       make(map[string]*FileEntry),
		remoteCache: make(map[string]*drive.File),
		folderCache: make(map[string]string),
		Updates:     make(chan StatusSnapshot, 8),
		queue:       make(chan workItem, 512),
		stateFile:   stateFile,
	}

	e.resolver = conflicts.NewDetector(
		cfg.ConflictDir,
		conflicts.StrategyKeepBoth,
	)

	// Load persisted sync state from previous session
	e.loadState()

	// Start throttled broadcast loop (coalesces rapid updates)
	go e.broadcastThrottle()

	return e, nil
}

func (e *Engine) Run(ctx context.Context) error {
	// Init Drive client
	driveClient, err := drive.NewClient(ctx)
	if err != nil {
		return err
	}
	e.drive = driveClient

	// Watch all configured paths
	e.pathsMu.RLock()
	roots := slices.Clone(e.cfg.WatchPaths)
	e.pathsMu.RUnlock()
	for _, path := range roots {
		if err := e.watcher.Add(path); err != nil {
			log.Warn().Str("path", path).Err(err).Msg("Cannot watch path")
		} else {
			log.Info().Str("path", path).Msg("Watching path")
		}
	}

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.worker(ctx)
		}()
	}

	// Initial full sync
	go e.fullSync(ctx)

	// Poll Drive for remote changes
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			close(e.queue)
			wg.Wait()
			e.saveState()
			return nil

		case event, ok := <-e.watcher.Events:
			if !ok {
				continue
			}
			e.queue <- workItem{event: event}

		case err := <-e.watcher.Errors:
			log.Error().Err(err).Msg("Watcher error")

		case <-ticker.C:
			go e.pollRemote(ctx)
			go e.saveState()
		}
	}
}

func (e *Engine) worker(ctx context.Context) {
	for item := range e.queue {
		if ctx.Err() != nil {
			return
		}
		e.handleEvent(ctx, item.event)
	}
}

func (e *Engine) handleEvent(ctx context.Context, event watcher.FileEvent) {
	path := event.Path

	// ── Mode gate: skip local events if mode does not process them ──
	mode := e.modeForPath(path)
	if !mode.ShouldProcessLocalEvents() {
		return
	}

	// Keep folder structure in Drive when local directories are created.
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		if event.Kind == watcher.EventCreate && mode.AllowsUpload() {
			e.setStatusDir(path, StatusInitializing, "")
			if _, err := e.ensureRemoteFolderTree(ctx, path); err != nil {
				log.Error().Err(err).Str("path", path).Msg("Failed to sync folder")
				errMsg := err.Error()
				if strings.Contains(errMsg, "403") || strings.Contains(errMsg, "400") {
					errMsg = "Path too deep: Drive limit is 100 nested folders"
				}
				e.setStatusDir(path, StatusError, errMsg)
			} else {
				e.setStatusDir(path, StatusSynced, "")
			}
		}
		return
	}
	if isTempFile(path) {
		return
	}

	switch event.Kind {
	case watcher.EventCreate, watcher.EventWrite:
		e.uploadFile(ctx, path)
	case watcher.EventRemove, watcher.EventRename:
		e.removeRemoteFile(ctx, path)
	}
}

func (e *Engine) uploadFile(ctx context.Context, localPath string) {
	// Compute local MD5 BEFORE setting status (avoid creating bare entry that defeats dedup)
	localMD5, err := conflicts.MD5File(localPath)
	if err != nil {
		e.setStatus(localPath, StatusError, err.Error())
		return
	}

	// Get file size for caching
	var fileSize int64
	if fi, err := os.Stat(localPath); err == nil {
		fileSize = fi.Size()
	}

	e.mu.RLock()
	entry := e.files[localPath]
	e.mu.RUnlock()

	// Skip if local content unchanged from last successful sync
	if entry != nil && entry.LocalMD5 == localMD5 && entry.Status == StatusSynced {
		return
	}

	// NOW mark as initializing (after dedup check passed)
	e.setStatus(localPath, StatusInitializing, "")

	remoteName := filepath.Base(localPath)
	parentID, err := e.resolveRemoteParentID(ctx, localPath)
	if err != nil {
		e.setStatus(localPath, StatusError, err.Error())
		return
	}

	e.setStatus(localPath, StatusUploading, "")
	remoteID := ""
	var lastSync time.Time
	if entry != nil {
		remoteID = entry.RemoteID
		lastSync = entry.LastSync
	}

	var remoteFile *drive.File
	if remoteID == "" {
		// Actively search Drive if the file already exists but was not in local memory (e.g. restart)
		remoteFile, _ = e.drive.GetFileByNameInFolder(ctx, remoteName, parentID)
		if remoteFile != nil {
			remoteID = remoteFile.ID
		}
	} else {
		e.mu.RLock()
		remoteFile = e.remoteCache[remoteName]
		e.mu.RUnlock()
	}

	// If remote file exists and MD5 matches local, skip upload (covers both restart and unchanged cases)
	if remoteFile != nil && remoteFile.MD5 == localMD5 {
		e.mu.Lock()
		e.files[localPath] = &FileEntry{
			LocalPath:  localPath,
			RemoteID:   remoteFile.ID,
			RemoteName: remoteFile.Name,
			Status:     StatusSynced,
			LastSync:   time.Now(),
			LocalMD5:   localMD5,
			RemoteMD5:  remoteFile.MD5,
			Size:       fileSize,
		}
		e.mu.Unlock()
		e.broadcast()
		return
	}

	// Check for conflict (only in TwoWay mode)
	if remoteFile != nil && e.modeForPath(localPath).AllowsConflictResolution() {
		fi, _ := os.Stat(localPath)
		localModTime := time.Time{}
		if fi != nil {
			localModTime = fi.ModTime()
		}
		if e.resolver.HasConflict(localPath, localModTime, remoteFile.ModTime, lastSync) {
			c := &conflicts.Conflict{
				LocalPath:     localPath,
				RemoteName:    remoteName,
				LocalModTime:  localModTime,
				RemoteModTime: remoteFile.ModTime,
				LocalMD5:      localMD5,
				RemoteMD5:     remoteFile.MD5,
			}
			uploadPath, localReplaced, err := e.resolver.Resolve(c)
			if err != nil {
				e.setStatus(localPath, StatusConflict, err.Error())
				return
			}
			if localReplaced {
				e.setStatus(localPath, StatusConflict, "remote version downloaded")
				return
			}
			if uploadPath != localPath {
				go e.uploadFile(ctx, uploadPath)
			}
			e.setStatus(localPath, StatusConflict, "conflict copy created")
		}
	}

	e.setStatus(localPath, StatusUploading, "")
	result, err := e.drive.UploadFile(ctx, localPath, remoteName, parentID, remoteID)
	if err != nil {
		log.Error().Err(err).Str("file", localPath).Msg("Upload failed")
		e.setStatus(localPath, StatusError, err.Error())
		return
	}

	e.setStatus(localPath, StatusVerifying, "")
	// (MD5 verification is done via drive.UploadFile return values)

	e.setStatus(localPath, StatusFinalizing, "")
	e.mu.Lock()
	e.files[localPath] = &FileEntry{
		LocalPath:  localPath,
		RemoteID:   result.ID,
		RemoteName: result.Name,
		Status:     StatusSynced,
		LastSync:   time.Now(),
		LocalMD5:   localMD5,
		RemoteMD5:  result.MD5,
		Size:       fileSize,
	}
	e.mu.Unlock()

	log.Info().Str("file", localPath).Msg("Uploaded successfully")
	e.broadcast()
}

func (e *Engine) removeRemoteFile(ctx context.Context, localPath string) {
	e.mu.RLock()
	entry := e.files[localPath]
	e.mu.RUnlock()

	if entry == nil || entry.RemoteID == "" {
		return
	}

	if err := e.drive.DeleteFile(ctx, entry.RemoteID); err != nil {
		log.Error().Err(err).Str("file", localPath).Msg("Remote delete failed")
		return
	}

	e.mu.Lock()
	delete(e.files, localPath)
	e.mu.Unlock()

	log.Info().Str("file", localPath).Msg("Deleted from Drive")
	e.broadcast()
}

func (e *Engine) fullSync(ctx context.Context) {
	log.Info().Msg("Starting full sync...")
	remoteFiles, err := e.drive.ListFiles(ctx, "")
	if err != nil {
		log.Error().Err(err).Msg("Full sync: failed to list Drive files")
		return
	}

	e.mu.Lock()
	for _, f := range remoteFiles {
		e.remoteCache[f.Name] = f
	}
	e.mu.Unlock()

	// Process each watch root according to its mode
	e.pathsMu.RLock()
	roots := slices.Clone(e.cfg.WatchPaths)
	e.pathsMu.RUnlock()
	var paths []string
	for _, watchPath := range roots {
		mode := e.modeForPath(watchPath)

		// Always ensure the root folder exists in state so the UI displays it even if empty
		e.setStatusDir(watchPath, StatusSynced, "")

		// ── Upload side: scan local files (TwoWay + UploadOnly) ──
		if mode.AllowsUpload() {
			_ = filepath.WalkDir(watchPath, func(path string, d os.DirEntry, err error) error {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if err != nil {
					return nil
				}
				if d.IsDir() {
					if path != watchPath {
						e.setStatusDir(path, StatusInitializing, "")
						if _, err := e.ensureRemoteFolderTree(ctx, path); err != nil {
							log.Error().Err(err).Str("path", path).Msg("Failed to sync folder during full sync")
							errMsg := err.Error()
							if strings.Contains(errMsg, "403") || strings.Contains(errMsg, "400") {
								errMsg = "Path too deep: Drive limit is 100 nested folders"
							}
							e.setStatusDir(path, StatusError, errMsg)
						} else {
							e.setStatusDir(path, StatusSynced, "")
						}
					}
					return nil
				}
				if isTempFile(path) {
					return nil
				}
				paths = append(paths, path)
				e.markQueuedIfNotExists(path)
				return nil
			})
		}

		// ── Download side: fetch remote files (TwoWay + DownloadOnly) ──
		if mode.AllowsDownload() {
			e.downloadRemoteFiles(ctx, watchPath)
		}
	}

	count := 0
	for _, path := range paths {
		select {
		case e.queue <- workItem{event: watcher.FileEvent{Path: path, Kind: watcher.EventWrite}}:
			count++
			if count%50 == 0 {
				time.Sleep(100 * time.Millisecond) // let workers drain
			}
		case <-ctx.Done():
			return
		}
	}

	log.Info().Int("remote_files", len(remoteFiles)).Int("local_enqueued", count).Msg("Full sync complete")
	e.broadcast()
}

func (e *Engine) pollRemote(ctx context.Context) {
	// Download new/changed remote files for roots that allow downloads
	e.pathsMu.RLock()
	roots := slices.Clone(e.cfg.WatchPaths)
	e.pathsMu.RUnlock()
	for _, root := range roots {
		mode := e.modeForPath(root)
		if mode.ShouldProcessRemoteEvents() {
			e.downloadRemoteFiles(ctx, root)
		}
	}
}

// downloadRemoteFiles fetches files from Drive that are missing or outdated locally.
// It walks the remote folder hierarchy corresponding to watchRoot.
func (e *Engine) downloadRemoteFiles(ctx context.Context, watchRoot string) {
	// Find the remote folder ID that matches this watch root
	e.mu.RLock()
	remoteFolderID, hasCached := e.folderCache[watchRoot]
	e.mu.RUnlock()

	if !hasCached {
		id, err := e.ensureRemoteFolderTree(ctx, watchRoot)
		if err != nil || id == "" {
			return
		}
		remoteFolderID = id
	}

	remoteFiles, err := e.drive.ListFiles(ctx, remoteFolderID)
	if err != nil {
		log.Error().Err(err).Str("root", watchRoot).Msg("Failed to list remote files for download")
		return
	}

	for _, rf := range remoteFiles {
		if ctx.Err() != nil {
			return
		}
		// Skip folders for now (recursive download is a future enhancement)
		if rf.MimeType == "application/vnd.google-apps.folder" {
			continue
		}

		localPath := filepath.Join(watchRoot, rf.Name)

		// Check if local copy is up to date
		e.mu.RLock()
		entry := e.files[localPath]
		e.mu.RUnlock()

		if entry != nil && entry.RemoteMD5 == rf.MD5 && entry.Status == StatusSynced {
			continue // already in sync
		}

		// Check local file MD5
		if localMD5, err := conflicts.MD5File(localPath); err == nil && localMD5 == rf.MD5 {
			// Local file matches remote, just update state
			e.mu.Lock()
			e.files[localPath] = &FileEntry{
				LocalPath:  localPath,
				RemoteID:   rf.ID,
				RemoteName: rf.Name,
				Status:     StatusSynced,
				LastSync:   time.Now(),
				LocalMD5:   localMD5,
				RemoteMD5:  rf.MD5,
				Size:       rf.Size,
			}
			e.mu.Unlock()
			e.broadcast()
			continue
		}

		// Download the file
		log.Info().Str("file", rf.Name).Str("dest", localPath).Msg("Downloading from Drive")
		e.setStatus(localPath, StatusInitializing, "")

		if err := e.drive.DownloadFile(ctx, rf.ID, localPath); err != nil {
			log.Error().Err(err).Str("file", rf.Name).Msg("Download failed")
			e.setStatus(localPath, StatusError, err.Error())
			continue
		}

		var fileSize int64
		if fi, err := os.Stat(localPath); err == nil {
			fileSize = fi.Size()
		}
		localMD5, _ := conflicts.MD5File(localPath)

		e.mu.Lock()
		e.files[localPath] = &FileEntry{
			LocalPath:  localPath,
			RemoteID:   rf.ID,
			RemoteName: rf.Name,
			Status:     StatusSynced,
			LastSync:   time.Now(),
			LocalMD5:   localMD5,
			RemoteMD5:  rf.MD5,
			Size:       fileSize,
		}
		e.mu.Unlock()

		log.Info().Str("file", rf.Name).Msg("Downloaded successfully")
		e.broadcast()
	}

	// Handle remote deletions (sync local state to match remote)
	remoteMap := make(map[string]bool)
	for _, rf := range remoteFiles {
		remoteMap[rf.Name] = true
	}

	e.mu.RLock()
	var localFilesToDelete []string
	for localPath, entry := range e.files {
		if filepath.Dir(localPath) == watchRoot {
			if !remoteMap[entry.RemoteName] && entry.Status == StatusSynced && entry.RemoteID != "" {
				localFilesToDelete = append(localFilesToDelete, localPath)
			}
		}
	}
	e.mu.RUnlock()

	for _, localPath := range localFilesToDelete {
		log.Info().Str("file", localPath).Msg("Deleting local file (removed from Drive)")
		e.mu.Lock()
		delete(e.files, localPath)
		e.mu.Unlock()
		os.Remove(localPath)
		e.broadcast()
	}
}

func (e *Engine) setStatus(localPath string, status FileStatus, errMsg string) {
	e.setStatusInternal(localPath, status, errMsg, false)
}

func (e *Engine) setStatusDir(localPath string, status FileStatus, errMsg string) {
	e.setStatusInternal(localPath, status, errMsg, true)
}

func (e *Engine) setStatusInternal(localPath string, status FileStatus, errMsg string, isDir bool) {
	e.mu.Lock()
	entry, ok := e.files[localPath]
	var oldStatus string
	if ok {
		oldStatus = entry.Status.String()
	} else {
		oldStatus = "new"
		entry = &FileEntry{LocalPath: localPath, IsDir: isDir}
		e.files[localPath] = entry
	}

	if entry.Status != status || entry.ErrorMsg != errMsg {
		if entry.Status != status {
			log.Info().
				Str("path", localPath).
				Str("from", oldStatus).
				Str("to", status.String()).
				Msg("File state transition")
		}
		entry.Status = status
		entry.ErrorMsg = errMsg
		entry.IsDir = isDir
		e.mu.Unlock()
		e.broadcast()
	} else {
		e.mu.Unlock()
	}
}

func (e *Engine) markQueuedIfNotExists(localPath string) {
	e.markQueuedIfNotExistsInternal(localPath, false)
}

func (e *Engine) markQueuedIfNotExistsInternal(localPath string, isDir bool) {
	e.mu.Lock()
	entry := e.files[localPath]
	if entry == nil {
		entry = &FileEntry{LocalPath: localPath, Status: StatusQueued, IsDir: isDir}
		e.files[localPath] = entry
	}
	e.mu.Unlock()
	e.broadcast()
}

func (e *Engine) broadcast() {
	atomic.StoreInt32(&e.broadcastDirty, 1)
}

// broadcastThrottle coalesces rapid state changes into periodic snapshots.
func (e *Engine) broadcastThrottle() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		if atomic.CompareAndSwapInt32(&e.broadcastDirty, 1, 0) {
			snap := e.Snapshot()
			select {
			case e.Updates <- snap:
			default:
			}
		}
	}
}

func (e *Engine) Snapshot() StatusSnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()

	snap := StatusSnapshot{
		IsRunning:      true,
		LastUpdated:    time.Now(),
		WatchPathModes: make(map[string]string),
	}
	for _, entry := range e.files {
		snap.Files = append(snap.Files, entry)
		snap.TotalFiles++
		if entry.Status == StatusSynced {
			snap.SyncedFiles++
		}
		snap.TotalBytes += entry.Size // use cached size instead of os.Stat
	}

	e.pathsMu.RLock()
	snap.WatchPaths = slices.Clone(e.cfg.WatchPaths)
	for _, wp := range e.cfg.WatchPaths {
		snap.WatchPathModes[wp] = e.cfg.GetWatchPathMode(wp).String()
	}
	e.pathsMu.RUnlock()

	return snap
}

// modeForPath resolves the sync mode for the watch root that contains the given path.
func (e *Engine) modeForPath(filePath string) config.SyncMode {
	e.pathsMu.RLock()
	defer e.pathsMu.RUnlock()
	for _, root := range e.cfg.WatchPaths {
		rel, err := filepath.Rel(root, filePath)
		if err != nil {
			continue
		}
		if rel == "." || !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return e.cfg.GetWatchPathMode(root)
		}
	}
	return config.ModeTwoWay
}

func isTempFile(path string) bool {
	base := filepath.Base(path)
	if len(base) == 0 {
		return false
	}
	// Skip hidden files, temp editors, swap files
	if base[0] == '.' {
		return true
	}
	ext := filepath.Ext(base)
	switch ext {
	case ".swp", ".swx", ".tmp", ".part", ".crdownload":
		return true
	}
	// Vim/Emacs temp
	if base[len(base)-1] == '~' {
		return true
	}
	return false
}

func (e *Engine) resolveRemoteParentID(ctx context.Context, localPath string) (string, error) {
	e.pathsMu.RLock()
	roots := slices.Clone(e.cfg.WatchPaths)
	e.pathsMu.RUnlock()
	for _, watchRoot := range roots {
		rel, err := filepath.Rel(watchRoot, localPath)
		if err != nil {
			continue
		}
		if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		parentDir := filepath.Dir(localPath)
		return e.ensureRemoteFolderTree(ctx, parentDir)
	}
	return "", nil
}

func (e *Engine) ensureRemoteFolderTree(ctx context.Context, localDir string) (string, error) {
	e.mu.RLock()
	if id, ok := e.folderCache[localDir]; ok {
		e.mu.RUnlock()
		return id, nil
	}
	e.mu.RUnlock()

	var matchedRoot string
	e.pathsMu.RLock()
	roots := slices.Clone(e.cfg.WatchPaths)
	e.pathsMu.RUnlock()
	for _, root := range roots {
		rel, err := filepath.Rel(root, localDir)
		if err != nil {
			continue
		}
		if rel == "." || !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			matchedRoot = root
			break
		}
	}
	if matchedRoot == "" {
		return "", nil
	}

	relDir, err := filepath.Rel(filepath.Dir(matchedRoot), localDir)
	if err != nil {
		return "", nil
	}

	parts := strings.Split(relDir, string(filepath.Separator))

	// Pre-flight check: Google Drive limits folder nesting to 100 levels.
	if len(parts) >= 100 {
		return "", fmt.Errorf("Path too deep: Drive limit is 100 nested folders")
	}

	parentID := ""
	curPath := filepath.Dir(matchedRoot)

	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}

		curPath = filepath.Join(curPath, part)

		e.mu.RLock()
		if id, ok := e.folderCache[curPath]; ok {
			e.mu.RUnlock()
			parentID = id
			continue
		}
		e.mu.RUnlock()

		folderID, err := e.drive.GetOrCreateFolder(ctx, part, parentID)
		if err != nil {
			return "", err
		}

		e.mu.Lock()
		e.folderCache[curPath] = folderID
		e.mu.Unlock()
		parentID = folderID
	}

	return parentID, nil
}

// AddWatchRoot persists a new watch path with the default mode (TwoWay).
func (e *Engine) AddWatchRoot(ctx context.Context, raw string) error {
	return e.AddWatchRootWithMode(ctx, raw, config.ModeTwoWay)
}

// AddWatchRootWithMode persists a new watch path with the given sync mode,
// registers it with fsnotify, and enqueues an initial index.
func (e *Engine) AddWatchRootWithMode(ctx context.Context, raw string, mode config.SyncMode) error {
	e.pathsMu.Lock()
	before := len(e.cfg.WatchPaths)
	e.cfg.AddWatchPathWithMode(raw, mode)
	if len(e.cfg.WatchPaths) == before {
		e.pathsMu.Unlock()
		return fmt.Errorf("This folder is already in the sync list")
	}
	root := e.cfg.WatchPaths[len(e.cfg.WatchPaths)-1]

	if fi, err := os.Stat(root); err != nil {
		e.cfg.RemoveWatchPath(root)
		e.pathsMu.Unlock()
		return fmt.Errorf("Could not access the folder: %w", err)
	} else if !fi.IsDir() {
		e.cfg.RemoveWatchPath(root)
		e.pathsMu.Unlock()
		return fmt.Errorf("Please select a folder (directory)")
	}

	if err := e.cfg.Save(); err != nil {
		e.cfg.RemoveWatchPath(root)
		e.pathsMu.Unlock()
		return err
	}
	if err := e.watcher.Add(root); err != nil {
		e.cfg.RemoveWatchPath(root)
		_ = e.cfg.Save()
		e.pathsMu.Unlock()
		return fmt.Errorf("Could not watch the folder: %w", err)
	}
	e.pathsMu.Unlock()

	log.Info().Str("path", root).Str("mode", mode.String()).Msg("Folder added to sync (UI)")
	go e.indexNewWatchRoot(ctx, root)
	return nil
}

// UpdateWatchMode changes the sync mode for an existing watch path.
// The new mode takes effect immediately for future events.
func (e *Engine) UpdateWatchMode(path string, mode config.SyncMode) error {
	e.pathsMu.Lock()
	defer e.pathsMu.Unlock()

	found := false
	for _, p := range e.cfg.WatchPaths {
		if p == path {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("folder is not in the sync list")
	}

	e.cfg.SetWatchPathMode(path, mode)
	if err := e.cfg.Save(); err != nil {
		return err
	}

	log.Info().Str("path", path).Str("mode", mode.String()).Msg("Sync mode updated")
	e.broadcast()
	return nil
}

// RemoveWatchRoot removes a watch path, stops monitoring it, and deletes remote files.
func (e *Engine) RemoveWatchRoot(ctx context.Context, path string) error {
	e.pathsMu.Lock()
	mode := e.cfg.GetWatchPathMode(path)
	e.cfg.RemoveWatchPath(path)
	if err := e.cfg.Save(); err != nil {
		e.pathsMu.Unlock()
		return err
	}
	_ = e.watcher.Remove(path)
	e.pathsMu.Unlock()

	log.Info().Str("path", path).Msg("Removing folder from sync")

	// 1. Identify remote folder ID to delete from Drive
	e.mu.RLock()
	remoteID, hasRemote := e.folderCache[path]
	e.mu.RUnlock()

	if hasRemote && remoteID != "" && mode.AllowsUpload() {
		log.Info().Str("path", path).Str("remoteID", remoteID).Msg("Deleting folder from Google Drive")
		if err := e.drive.DeleteFile(ctx, remoteID); err != nil {
			log.Error().Err(err).Str("path", path).Msg("Failed to delete folder from Google Drive")
			// We continue anyway to clean up local state
		}
	}

	// 2. Clean up internal state (files and folderCache)
	e.mu.Lock()
	for localPath := range e.files {
		if localPath == path || strings.HasPrefix(localPath, path+string(os.PathSeparator)) {
			delete(e.files, localPath)
		}
	}
	for localPath := range e.folderCache {
		if localPath == path || strings.HasPrefix(localPath, path+string(os.PathSeparator)) {
			delete(e.folderCache, localPath)
		}
	}
	e.mu.Unlock()

	e.saveState()
	e.broadcast()

	return nil
}

func (e *Engine) indexNewWatchRoot(ctx context.Context, watchPath string) {
	mode := e.modeForPath(watchPath)

	// Always ensure the root folder exists in state so the UI displays it even if empty
	e.setStatusDir(watchPath, StatusSynced, "")

	// ── DownloadOnly: skip local scan, download remote files instead ──
	if !mode.AllowsUpload() {
		log.Info().Str("path", watchPath).Str("mode", mode.String()).Msg("Indexing folder (download only)")
		if mode.AllowsDownload() {
			e.downloadRemoteFiles(ctx, watchPath)
		}
		return
	}

	var filePaths []string
	var dirPaths []string

	// Phase 1: Scan all files and directories first (collect paths)
	_ = filepath.WalkDir(watchPath, func(path string, d os.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return nil
		}
		if isTempFile(path) {
			return nil
		}
		if d.IsDir() {
			dirPaths = append(dirPaths, path)
		} else {
			filePaths = append(filePaths, path)
		}
		return nil
	})

	// Phase 2: Create folder structure on Drive and update total count in UI
	for _, path := range dirPaths {
		e.setStatusDir(path, StatusInitializing, "")
		if _, err := e.ensureRemoteFolderTree(ctx, path); err != nil {
			log.Error().Err(err).Str("path", path).Msg("Failed to sync folder structure")
			errMsg := err.Error()
			if strings.Contains(errMsg, "403") || strings.Contains(errMsg, "400") {
				errMsg = "Path too deep: Drive limit is 100 nested folders"
			}
			e.setStatusDir(path, StatusError, errMsg)
		} else {
			e.setStatusDir(path, StatusSynced, "")
		}
	}

	for _, path := range filePaths {
		e.markQueuedIfNotExists(path)
	}

	// Immediate broadcast so the UI shows the full total count before uploads start
	e.broadcast()

	// Phase 3: Start the actual sync process
	count := 0
	for _, path := range filePaths {
		select {
		case e.queue <- workItem{event: watcher.FileEvent{Path: path, Kind: watcher.EventWrite}}:
			count++
			if count%50 == 0 {
				time.Sleep(100 * time.Millisecond) // let workers drain
			}
		case <-ctx.Done():
			return
		}
	}

	// Phase 4: Also download remote files if TwoWay
	if mode.AllowsDownload() {
		e.downloadRemoteFiles(ctx, watchPath)
	}

	log.Info().Int("files_enqueued", count).Str("path", watchPath).Msg("Folder indexing complete")
	e.broadcast()
}

// loadState restores the files map from disk (previous session).
func (e *Engine) loadState() {
	data, err := os.ReadFile(e.stateFile)
	if err != nil {
		return // no previous state, start fresh
	}
	var entries map[string]*FileEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		log.Warn().Err(err).Msg("Failed to parse sync state file, starting fresh")
		return
	}
	e.mu.Lock()
	for path, entry := range entries {
		// Only restore entries that were successfully synced
		if entry.Status == StatusSynced && entry.RemoteID != "" {
			e.files[path] = entry
		}
	}
	e.mu.Unlock()
	log.Info().Int("restored_files", len(e.files)).Msg("Loaded sync state from disk")
}

// saveState persists the files map to disk for resume on restart.
func (e *Engine) saveState() {
	e.mu.RLock()
	data, err := json.Marshal(e.files)
	e.mu.RUnlock()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to marshal sync state")
		return
	}
	if err := os.WriteFile(e.stateFile, data, 0600); err != nil {
		log.Warn().Err(err).Msg("Failed to save sync state")
	}
}
