import { useSyncStore, selectStats } from '../store/syncStore'
import { useSettingsStore } from '../store/settingsStore'
import brandLogo from '../../src-tauri/icons/icon.png'

export type AppView = 'files' | 'settings'

interface Props {
  sendCommand: (action: string, payload?: object) => void
  currentView: AppView
  onNavigate: (view: AppView) => void
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

function WarningIcon() {
  return (
    <svg
      className="inline-warning-icon"
      width="13"
      height="13"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden
    >
      <path d="m21.73 18-8-14a2 2 0 0 0-3.46 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3" />
      <path d="M12 9v4" />
      <path d="M12 17h.01" />
    </svg>
  )
}

export function Sidebar({ sendCommand, currentView, onNavigate }: Props) {
  const stats = useSyncStore(selectStats)
  const files = useSyncStore(state => state.snapshot?.files ?? [])
  const { t } = useSettingsStore()

  const syncPct = stats.totalFiles > 0
    ? Math.round((stats.syncedFiles / stats.totalFiles) * 100)
    : 0

  const hasDepthError = files.some(f => f.error && f.error.includes('100 nested folders'))

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
        <div className="section-label">{t('sidebar_storage')}</div>
        <div className="stat-card">
          <div className="stat-value">{formatBytes(stats.totalBytes)}</div>
          <div className="stat-label">{t('sidebar_synced')}</div>
        </div>
      </div>

      <div className="sidebar-section">
        <div className="section-label">{t('sidebar_progress')}</div>
        <div className="progress-block">
          <div className="progress-bar-bg">
            <div
              className="progress-bar-fill"
              style={{
                width: `${syncPct}%`,
                backgroundColor: hasDepthError ? '#e74c3c' : undefined
              }}
            />
          </div>
          <div className="progress-label">{stats.syncedFiles} / {stats.totalFiles} {t('sidebar_files_count')}</div>
          {hasDepthError && (
            <div className="sidebar-warning">
              <WarningIcon />
              <span>{t('sidebar_drive_limit')}</span>
            </div>
          )}
        </div>
      </div>

      <div className="sidebar-section">
        <div className="section-label">{t('sidebar_last_sync')}</div>
        <div className="last-sync">{formatTime(stats.lastUpdated)}</div>
      </div>

      <div className="sidebar-actions">
        <button
          className="btn-action"
          type="button"
          onClick={() => {
            import('@tauri-apps/api/core').then(({ invoke }) => invoke('restart_daemon'));
          }}
          title={t('sidebar_refresh_title')}
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
          {t('sidebar_refresh')}
        </button>

        <button
          className={`btn-action ${currentView === 'settings' ? 'active' : ''}`}
          type="button"
          onClick={() => onNavigate(currentView === 'settings' ? 'files' : 'settings')}
          title={t('nav_settings')}
        >
          <svg
            className="btn-action-icon"
            width="15"
            height="15"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden
          >
            <circle cx="12" cy="12" r="3" />
            <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
          </svg>
          {t('nav_settings')}
        </button>
      </div>

      <div className="sidebar-footer">
        <span className="footer-version">v0.3.2</span>
        <a
          className="footer-link"
          href="https://github.com/bryanrafaelbueno/synca"
          target="_blank"
          rel="noopener noreferrer"
        >
          {t('nav_github')} ↗
        </a>
      </div>
    </aside>
  )
}
