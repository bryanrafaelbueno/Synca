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
	StatusSynced   FileStatus = iota
	StatusSyncing
	StatusQueued
	StatusConflict
	StatusError
)

func (s FileStatus) String() string {
	switch s {
	case StatusSynced:
		return "synced"
	case StatusSyncing:
		return "syncing"
	case StatusQueued:
		return "queued"
	case StatusConflict:
		return "conflict"
	default:
		return "error"
	}
}

// FileEntry tracks a file's sync state.
type FileEntry struct {
	LocalPath    string     `json:"local_path"`
	RemoteID     string     `json:"remote_id"`
	RemoteName   string     `json:"remote_name"`
	Status       FileStatus `json:"status"`
	LastSync     time.Time  `json:"last_sync"`
	LocalMD5     string     `json:"local_md5"`
	RemoteMD5    string     `json:"remote_md5"`
	Size         int64      `json:"size"`
	ErrorMsg     string     `json:"error,omitempty"`
}

// StatusSnapshot is what the UI reads.
type StatusSnapshot struct {
	Files       []*FileEntry `json:"files"`
	TotalBytes  int64        `json:"total_bytes"`
	TotalFiles  int          `json:"total_files"`
	SyncedFiles int          `json:"synced_files"`
	IsRunning   bool         `json:"is_running"`
	LastUpdated time.Time    `json:"last_updated"`
}

// Engine is the core sync orchestrator.
type Engine struct {
	cfg      *config.Config
	watcher  *watcher.Watcher
	drive    *drive.Client
	resolver *conflicts.Detector

	pathsMu sync.RWMutex // cfg.WatchPaths: coordinate with AddWatchRoot

	mu      sync.RWMutex
	files   map[string]*FileEntry // keyed by localPath
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

	// Keep folder structure in Drive when local directories are created.
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		if event.Kind == watcher.EventCreate {
			if _, err := e.ensureRemoteFolderTree(ctx, path); err != nil {
				log.Error().Err(err).Str("path", path).Msg("Failed to sync folder")
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

	// NOW mark as syncing (after dedup check passed)
	e.setStatus(localPath, StatusSyncing, "")

	remoteName := filepath.Base(localPath)
	parentID, err := e.resolveRemoteParentID(ctx, localPath)
	if err != nil {
		e.setStatus(localPath, StatusError, err.Error())
		return
	}

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

	// Check for conflict
	if remoteFile != nil {
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

	result, err := e.drive.UploadFile(ctx, localPath, remoteName, parentID, remoteID)
	if err != nil {
		log.Error().Err(err).Str("file", localPath).Msg("Upload failed")
		e.setStatus(localPath, StatusError, err.Error())
		return
	}

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

	// Upload any local files not yet in Drive (throttled to avoid flooding the queue)
	e.pathsMu.RLock()
	roots := slices.Clone(e.cfg.WatchPaths)
	e.pathsMu.RUnlock()
	var paths []string
	for _, watchPath := range roots {
		_ = filepath.WalkDir(watchPath, func(path string, d os.DirEntry, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if path != watchPath {
					if _, err := e.ensureRemoteFolderTree(ctx, path); err != nil {
						log.Error().Err(err).Str("path", path).Msg("Failed to sync folder during full sync")
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
	remoteFiles, err := e.drive.ListFiles(ctx, "")
	if err != nil {
		return
	}
	e.mu.Lock()
	for _, f := range remoteFiles {
		e.remoteCache[f.Name] = f
	}
	e.mu.Unlock()
}

func (e *Engine) setStatus(localPath string, status FileStatus, errMsg string) {
	e.mu.Lock()
	entry := e.files[localPath]
	if entry == nil {
		entry = &FileEntry{LocalPath: localPath}
		e.files[localPath] = entry
	}
	entry.Status = status
	entry.ErrorMsg = errMsg
	e.mu.Unlock()
	e.broadcast()
}

func (e *Engine) markQueuedIfNotExists(localPath string) {
	e.mu.Lock()
	entry := e.files[localPath]
	if entry == nil {
		entry = &FileEntry{LocalPath: localPath, Status: StatusQueued}
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
		IsRunning:   true,
		LastUpdated: time.Now(),
	}
	for _, entry := range e.files {
		snap.Files = append(snap.Files, entry)
		snap.TotalFiles++
		if entry.Status == StatusSynced {
			snap.SyncedFiles++
		}
		snap.TotalBytes += entry.Size // use cached size instead of os.Stat
	}
	return snap
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
		if parentDir == watchRoot {
			return "", nil // Drive root
		}
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

	relDir, err := filepath.Rel(matchedRoot, localDir)
	if err != nil || relDir == "." {
		return "", nil
	}

	parts := strings.Split(relDir, string(filepath.Separator))
	parentID := ""
	curPath := matchedRoot

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

// AddWatchRoot persists a new watch path, registers it with fsnotify, and enqueues an initial index.
func (e *Engine) AddWatchRoot(ctx context.Context, raw string) error {
	e.pathsMu.Lock()
	before := len(e.cfg.WatchPaths)
	e.cfg.AddWatchPath(raw)
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

	log.Info().Str("path", root).Msg("Folder added to sync (UI)")
	go e.indexNewWatchRoot(ctx, root)
	return nil
}

func (e *Engine) indexNewWatchRoot(ctx context.Context, watchPath string) {
	var paths []string
	_ = filepath.WalkDir(watchPath, func(path string, d os.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != watchPath {
				if _, err := e.ensureRemoteFolderTree(ctx, path); err != nil {
					log.Error().Err(err).Str("path", path).Msg("Failed to sync folder structure")
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
