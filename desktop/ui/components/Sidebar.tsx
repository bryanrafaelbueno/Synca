import { useSyncStore, selectStats } from '../store/syncStore'
import brandLogo from '../../src-tauri/icons/icon.png'

interface Props {
  sendCommand: (action: string, payload?: object) => void
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

function formatTime(iso: string | null): string {
  if (!iso) return '—'
  const d = new Date(iso)
  return d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

export function Sidebar({ sendCommand }: Props) {
  const stats = useSyncStore(selectStats)
  const syncPct = stats.totalFiles > 0
    ? Math.round((stats.syncedFiles / stats.totalFiles) * 100)
    : 0

  return (
    <aside className="sidebar">
      <div className="sidebar-brand">
        <div className="brand-icon">
          <img src={brandLogo} alt="" width={28} height={28} className="brand-icon-img" />
        </div>
        <span className="brand-name">Synca</span>
        <span className={`brand-dot ${stats.isRunning ? 'running' : 'stopped'}`} />
      </div>

      <div className="sidebar-section">
        <div className="section-label">Storage</div>
        <div className="stat-card">
          <div className="stat-value">{formatBytes(stats.totalBytes)}</div>
          <div className="stat-label">synced</div>
        </div>
      </div>

      <div className="sidebar-section">
        <div className="section-label">Progress</div>
        <div className="progress-block">
          <div className="progress-bar-bg">
            <div className="progress-bar-fill" style={{ width: `${syncPct}%` }} />
          </div>
          <div className="progress-label">{stats.syncedFiles} / {stats.totalFiles} files</div>
        </div>
      </div>

      <div className="sidebar-section">
        <div className="section-label">Last sync</div>
        <div className="last-sync">{formatTime(stats.lastUpdated)}</div>
      </div>

      <div className="sidebar-actions">
        <button
          className="btn-action"
          type="button"
          onClick={() => {
            import('@tauri-apps/api/core').then(({ invoke }) => invoke('restart_daemon'));
          }}
          title="Restart the daemon (sidecar) instantly via Tauri"
        >
          <svg
            className="btn-action-icon"
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden
          >
            <path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8" />
            <path d="M21 3v5h-5" />
            <path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16" />
            <path d="M3 21v-5h5" />
          </svg>
          Refresh
        </button>
      </div>

      <div className="sidebar-footer">
        <span className="footer-version">v0.2.0-mvp</span>
        <a
          className="footer-link"
          href="https://github.com/bryanrafaelbueno/synca"
          target="_blank"
          rel="noopener noreferrer"
        >
          GitHub ↗
        </a>
      </div>
    </aside>
  )
}
