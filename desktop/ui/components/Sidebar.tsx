import { useSyncStore, selectStats } from '../store/syncStore'
import { useState, useEffect } from 'react'
import { enable, disable, isEnabled } from '@tauri-apps/plugin-autostart'
import { invoke } from '@tauri-apps/api/core'
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
  const files = useSyncStore(state => state.snapshot?.files ?? [])
  const [isAutostart, setIsAutostart] = useState(false)
  const [isAppImage, setIsAppImage] = useState(false)

  const syncPct = stats.totalFiles > 0
    ? Math.round((stats.syncedFiles / stats.totalFiles) * 100)
    : 0

  const hasDepthError = files.some(f => f.error && f.error.includes('100 nested folders'))

  useEffect(() => {
    const checkStatus = () => {
      isEnabled().then(setIsAutostart).catch(console.error)
      invoke<boolean>('is_appimage_cmd').then(setIsAppImage).catch(console.error)
    }

    // Check initially
    checkStatus()

    // Re-check every time the window gains focus (e.g., restored from tray)
    window.addEventListener('focus', checkStatus)
    return () => window.removeEventListener('focus', checkStatus)
  }, [])

  const toggleAutostart = async () => {
    try {
      if (isAutostart) {
        await disable()
        setIsAutostart(false)
      } else {
        await enable()
        setIsAutostart(true)
      }
    } catch (e) {
      console.error('Failed to toggle autostart:', e)
    }
  }

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
            <div
              className="progress-bar-fill"
              style={{
                width: `${syncPct}%`,
                backgroundColor: hasDepthError ? '#e74c3c' : undefined
              }}
            />
          </div>
          <div className="progress-label">{stats.syncedFiles} / {stats.totalFiles} files</div>
          {hasDepthError && (
            <div style={{ color: '#e74c3c', fontSize: '11px', marginTop: '6px', lineHeight: 1.3 }}>
              ⚠️ Drive limit: Max 100 nested folders reached.
            </div>
          )}
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

      {!isAppImage && (
        <div className="sidebar-section">
          <div className="section-label">Settings</div>
          <div className="settings-row">
            <span className="settings-text">Start on Boot</span>
            <label className="switch">
              <input
                type="checkbox"
                checked={isAutostart}
                onChange={toggleAutostart}
              />
              <span className="slider round"></span>
            </label>
          </div>
        </div>
      )}

      <div className="sidebar-footer">
        <span className="footer-version">v0.3.1</span>
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
