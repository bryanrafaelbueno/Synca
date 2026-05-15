import { useState } from 'react'
import { ask } from '@tauri-apps/plugin-dialog'
import { useSyncStore, selectFiles, FileEntry, FileStatus, SyncMode } from '../store/syncStore'

const SYNC_MODE_LABELS: Record<SyncMode, { label: string; icon: string; desc: string }> = {
  two_way:       { label: 'Two-Way',      icon: '⇅', desc: 'Sync both directions' },
  upload_only:   { label: 'Upload Only',  icon: '↑', desc: 'Local → Drive only' },
  download_only: { label: 'Download Only',icon: '↓', desc: 'Drive → Local only' },
}

function SyncModeSelector({ value, onChange }: { value: SyncMode; onChange: (m: SyncMode) => void }) {
  return (
    <div className="sync-mode-selector">
      {(Object.keys(SYNC_MODE_LABELS) as SyncMode[]).map(mode => (
        <button
          key={mode}
          type="button"
          className={`sync-mode-btn ${value === mode ? 'active' : ''}`}
          onClick={() => onChange(mode)}
          title={SYNC_MODE_LABELS[mode].desc}
        >
          <span className="sync-mode-icon">{SYNC_MODE_LABELS[mode].icon}</span>
          <span>{SYNC_MODE_LABELS[mode].label}</span>
        </button>
      ))}
    </div>
  )
}

function SyncModeBadge({ mode }: { mode: SyncMode }) {
  const info = SYNC_MODE_LABELS[mode]
  if (!info) return null
  return (
    <span className={`sync-mode-badge sync-mode-badge-${mode}`} title={info.desc}>
      {info.icon} {info.label}
    </span>
  )
}

interface FileListProps {
  sendCommand: (action: string, payload?: object) => void
}

type TreeNode = {
  name: string;
  path: string;
  localPath: string;
  isFolder: boolean;
  isWatchRoot?: boolean;
  children: Record<string, TreeNode>;
  file?: FileEntry;
};

async function pickWatchFolder(): Promise<string | null> {
  // Try Rust-side dialog first (bypasses JS capability issues)
  try {
    const { invoke } = await import('@tauri-apps/api/core');
    console.log('[pickWatchFolder] Calling Rust dialog...');
    const result: string | null = await invoke('pick_folder_dialog');
    console.log('[pickWatchFolder] Rust dialog result:', result);
    if (result) return result;
    return null;
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    console.error('[pickWatchFolder] Rust dialog failed:', msg);
  }

  // Fallback: JS dialog plugin
  try {
    const dialog = await import('@tauri-apps/plugin-dialog');
    const selected = await dialog.open({
      directory: true,
      multiple: false,
      title: 'Choose folder to sync',
    }) as string | string[] | null;
    if (!selected) return null;
    if (typeof selected === 'string') return selected;
    if (Array.isArray(selected) && selected.length > 0) return selected[0];
    return null;
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    console.error('[pickWatchFolder] JS dialog also failed:', msg);
  }

  // Last resort: manual path entry
  const raw = window.prompt(
    'Native folder picker is not available.\n\n' +
    'Please enter the absolute folder path to sync:\n\n' +
    'Linux:  /home/user/Documents\n' +
    'Windows: C:\\Users\\user\\Documents'
  );
  return raw?.trim() || null;
}

const STATUS_LABELS: Record<FileStatus, string> = {
  synced: 'synced',
  initializing: 'initializing…',
  uploading: 'uploading…',
  verifying: 'verifying…',
  finalizing: 'finalizing…',
  queued: 'queued',
  conflict: 'conflict',
  error: 'error',
}

function StatusPill({ status }: { status: FileStatus }) {
  const isProgressing = status === 'initializing' || status === 'uploading' || status === 'verifying' || status === 'finalizing';
  return (
    <span className={`pill pill-${status}`}>
      {isProgressing && <span className="pill-spinner" />}
      {STATUS_LABELS[status]}
    </span>
  )
}

function FileIcon({ path }: { path: string }) {
  const ext = path.split('.').pop()?.toLowerCase() ?? ''
  const iconColor: Record<string, string> = {
    pdf: '#E85D24',
    doc: '#185FA5', docx: '#185FA5',
    xls: '#3B6D11', xlsx: '#3B6D11',
    ppt: '#993C1D', pptx: '#993C1D',
    jpg: '#854F0B', jpeg: '#854F0B', png: '#854F0B', gif: '#854F0B',
    mp4: '#533AB7', mov: '#533AB7', mkv: '#533AB7',
    zip: '#5F5E5A', tar: '#5F5E5A', gz: '#5F5E5A',
    md: '#185FA5', txt: '#444441',
    js: '#BA7517', ts: '#185FA5',
    py: '#3B6D11', go: '#0F6E56',
  }
  const color = iconColor[ext] ?? '#888780'
  return (
    <div className="file-icon" style={{ background: color + '18' }}>
      <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
        <rect x="2" y="1" width="10" height="12" rx="1.5" stroke={color} strokeWidth="1"/>
        <path d="M4 5h6M4 7.5h6M4 10h4" stroke={color} strokeWidth="1" strokeLinecap="round"/>
      </svg>
    </div>
  )
}

function TreeGuides({ depth }: { depth: number }) {
  const guides = [];
  for (let i = 0; i < depth; i++) {
    const left = 20 + i * 16 + 7;
    guides.push(<div key={i} className="tree-guide" style={{ left }} />);
  }
  return <>{guides}</>;
}

function FileRow({ entry, depth = 0 }: { entry: FileEntry, depth?: number }) {
  const fileName = entry.local_path.split(/[/\\]/).pop() ?? entry.local_path

  const lastSync = entry.last_sync
    ? new Date(entry.last_sync).toLocaleString('en-US', {
        day: '2-digit', month: '2-digit',
        hour: '2-digit', minute: '2-digit',
      })
    : '—'

  return (
    <div 
      className={`file-row ${entry.status === 'conflict' ? 'file-row-conflict' : ''}`}
      style={{ paddingLeft: 20 + depth * 16 }}
    >
      <TreeGuides depth={depth} />
      <FileIcon path={entry.local_path} />
      <div className="file-info">
        <div className="file-name">{fileName}</div>
        {entry.error && <div className="file-error">{entry.error}</div>}
      </div>
      <div className="file-meta">
        <div className="file-sync-time">{lastSync}</div>
        <StatusPill status={entry.status} />
      </div>
    </div>
  )
}

function TreeNodeView({ node, depth = 0, sendCommand }: { node: TreeNode, depth?: number, sendCommand: (action: string, payload?: object) => void }) {
  const [isOpen, setIsOpen] = useState(true);
  const [editingMode, setEditingMode] = useState(false);
  const watchModes = useSyncStore(state => state.snapshot?.watch_path_modes ?? {});

  const handleRemove = async (path: string) => {
    if (!path) return;
    
    let confirmed = false;
    try {
      const { invoke } = await import('@tauri-apps/api/core');
      confirmed = await invoke('confirm_dialog', {
        title: 'Remove Folder from Sync',
        message: `Are you sure you want to stop syncing this folder?\n\nThis will REMOVE all files from Google Drive, but keep your local files untouched.`
      });
    } catch (err) {
      console.warn('[handleRemove] Rust dialog failed, falling back to JS:', err);
      confirmed = await ask(
        `Are you sure you want to stop syncing this folder?\n\nThis will REMOVE all files from Google Drive, but keep your local files untouched.`,
        { title: 'Remove Folder from Sync', kind: 'warning' }
      );
    }

    if (confirmed) {
      sendCommand('remove_watch', { path });
    }
  };

  const handleModeChange = (mode: SyncMode) => {
    sendCommand('update_watch', { path: node.localPath, mode });
    setEditingMode(false);
  };

  if (!node.isFolder) {
    return <FileRow entry={node.file!} depth={depth} />;
  }

  const currentMode = (node.isWatchRoot ? watchModes[node.localPath] : undefined) as SyncMode | undefined;

  return (
    <div className="tree-node">
      <div 
        className="tree-folder-row" 
        style={{ paddingLeft: 20 + depth * 16 }}
        onClick={() => setIsOpen(!isOpen)}
      >
        <TreeGuides depth={depth} />
        <span className="tree-chevron">{isOpen ? '▼' : '▶'}</span>
        <svg className="folder-icon" width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
          <path d="M10 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"/>
        </svg>
        <span className="tree-folder-name">{node.name}</span>
        <div className="file-meta" style={{ marginLeft: 'auto' }}>
          {node.file && node.file.status !== 'synced' && (
            <>
              {node.file.error && <span className="file-error" style={{ marginRight: 8, fontSize: 11 }}>{node.file.error}</span>}
              <StatusPill status={node.file.status} />
            </>
          )}
          {node.isWatchRoot && currentMode && (
            <span
              className="sync-mode-badge-wrap"
              onClick={(e) => { e.stopPropagation(); setEditingMode(!editingMode); }}
              title="Click to change sync mode"
            >
              <SyncModeBadge mode={currentMode} />
            </span>
          )}
          {node.isWatchRoot && (
            <button 
              className="btn-remove-root"
              onClick={(e) => {
                e.stopPropagation();
                handleRemove(node.localPath);
              }}
              title="Remove from sync (deletes from Drive)"
            >
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M3 6h18M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M10 11v6M14 11v6" />
              </svg>
            </button>
          )}
        </div>
      </div>
      {editingMode && node.isWatchRoot && currentMode && (
        <div className="sync-mode-edit-row" onClick={(e) => e.stopPropagation()}>
          <SyncModeSelector value={currentMode} onChange={handleModeChange} />
        </div>
      )}
      {isOpen && (
        <div className="tree-children">
          {Object.values(node.children)
            .sort((a, b) => {
              if (a.isFolder === b.isFolder) return a.name.localeCompare(b.name);
              return a.isFolder ? -1 : 1;
            })
            .map(child => (
              <TreeNodeView key={child.path} node={child} depth={depth + 1} sendCommand={sendCommand} />
          ))}
        </div>
      )}
    </div>
  );
}

export function FileList({ sendCommand }: FileListProps) {
  const files = useSyncStore(selectFiles)
  const { searchQuery, setSearchQuery, connected, lastWsError, setLastWsError } = useSyncStore()
  const [pendingFolder, setPendingFolder] = useState<string | null>(null)
  const [pendingMode, setPendingMode] = useState<SyncMode>('two_way')

  const onAddWatchFolder = async () => {
    if (!connected) return
    setLastWsError(null)
    const path = await pickWatchFolder()
    if (!path) return
    setPendingFolder(path)
    setPendingMode('two_way')
  }

  const confirmAddFolder = () => {
    if (!pendingFolder) return
    sendCommand('add_watch', { path: pendingFolder, mode: pendingMode })
    setPendingFolder(null)
  }

  const counts = {
    conflict: files.filter(f => f.status === 'conflict').length,
    error: files.filter(f => f.status === 'error').length,
  }

  const watchPaths = useSyncStore(state => state.snapshot?.watch_paths ?? []);

  const rootNode: TreeNode = { name: 'root', path: '', localPath: '', isFolder: true, children: {} };

  files.forEach(f => {
    const isWindows = f.local_path.includes('\\');
    const sep = isWindows ? '\\' : '/';
    const parts = f.local_path.split(/[/\\]/).filter(Boolean);
    let current = rootNode;
    
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i];
      const isFile = !f.is_dir && (i === parts.length - 1);
      // Construct a consistent internal tree path using forward slash
      const nodePath = '/' + parts.slice(0, i + 1).join('/');
      
      let localPathPrefix = parts.slice(0, i + 1).join(sep);
      if (!isWindows) {
          localPathPrefix = '/' + localPathPrefix;
      } else if (localPathPrefix.length === 2 && localPathPrefix[1] === ':') {
          localPathPrefix += '\\';
      }
      
      if (!current.children[part]) {
        current.children[part] = {
          name: part,
          path: nodePath,
          localPath: localPathPrefix,
          isFolder: !isFile,
          children: {}
        };
      }
      
      if (watchPaths.includes(localPathPrefix)) {
        current.children[part].isWatchRoot = true;
      }
      
      if (isFile) {
        current.children[part].file = f;
        current.children[part].isFolder = false;
      } else if (f.is_dir && i === parts.length - 1) {
        current.children[part].file = f;
      }
      current = current.children[part];
    }
  });

  let displayRoots = Object.values(rootNode.children);
  while (displayRoots.length === 1 && displayRoots[0].isFolder) {
    const single = displayRoots[0];
    if (single.isWatchRoot) break;
    const hasDirectFiles = Object.values(single.children).some(c => !c.isFolder);
    if (hasDirectFiles) break;
    displayRoots = Object.values(single.children);
  }

  let displayContent;
  if (searchQuery.trim()) {
    displayContent = files.map(f => <FileRow key={f.local_path} entry={f} />);
  } else {
    displayContent = displayRoots
      .sort((a, b) => {
        if (a.isFolder === b.isFolder) return a.name.localeCompare(b.name);
        return a.isFolder ? -1 : 1;
      })
      .map(node => <TreeNodeView key={node.path} node={node} sendCommand={sendCommand} />);
  }

  return (
    <div className="file-list">
      <div className="file-list-header">
        <div className="header-title">
          <span>Files</span>
          {counts.conflict > 0 && (
            <span className="badge-warn">{counts.conflict} conflict{counts.conflict > 1 ? 's' : ''}</span>
          )}
          {counts.error > 0 && (
            <span className="badge-error">{counts.error} error{counts.error > 1 ? 's' : ''}</span>
          )}
        </div>
        <div className="header-tools">
          <div className="search-wrap">
            <svg className="search-icon" width="13" height="13" viewBox="0 0 13 13" fill="none">
              <circle cx="5.5" cy="5.5" r="4" stroke="currentColor" strokeWidth="1.2"/>
              <path d="M9 9L12 12" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round"/>
            </svg>
            <input
              className="search-input"
              type="text"
              placeholder="Search files…"
              value={searchQuery}
              onChange={e => setSearchQuery(e.target.value)}
            />
          </div>
          <button
            type="button"
            className="btn-add-watch"
            title="Add folder to sync"
            disabled={!connected}
            onClick={() => void onAddWatchFolder()}
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
              <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
              <line x1="12" y1="11" x2="12" y2="17" />
              <line x1="9" y1="14" x2="15" y2="14" />
            </svg>
            <span className="btn-add-watch-label">Folder</span>
          </button>
        </div>
      </div>

      {lastWsError && (
        <div className="folder-action-banner" role="status">
          {lastWsError}
          <button type="button" className="folder-action-dismiss" onClick={() => setLastWsError(null)} aria-label="Close">
            ×
          </button>
        </div>
      )}

      <div className="file-list-body">
        {!connected && (
          <div className="empty-state">
            <div className="empty-icon">⏳</div>
            <div className="empty-title">Connecting to daemon…</div>
            <div className="empty-sub">Please wait</div>
          </div>
        )}

        {connected && files.length === 0 && (
          <div className="empty-state">
            <div className="empty-icon">📂</div>
            <div className="empty-title">No files found</div>
            <div className="empty-sub">
              {searchQuery
                ? 'Try a different search term'
                : 'Click Folder next to the search bar or use synca watch ~/folder in the terminal'}
            </div>
          </div>
        )}

        {connected && files.length > 0 && displayContent}
      </div>

      {/* ── Add-folder mode selector modal ── */}
      {pendingFolder && (
        <div className="add-folder-modal-overlay" onClick={() => setPendingFolder(null)}>
          <div className="add-folder-modal" onClick={(e) => e.stopPropagation()}>
            <h3 className="add-folder-modal-title">Choose Sync Mode</h3>
            <p className="add-folder-modal-path" title={pendingFolder}>
              {pendingFolder.split(/[/\\]/).pop()}
            </p>
            <SyncModeSelector value={pendingMode} onChange={setPendingMode} />
            <div className="add-folder-modal-actions">
              <button type="button" className="btn-modal-cancel" onClick={() => setPendingFolder(null)}>Cancel</button>
              <button type="button" className="btn-modal-confirm" onClick={confirmAddFolder}>Add Folder</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
