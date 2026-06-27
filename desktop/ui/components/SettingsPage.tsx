import { useState, useRef, useEffect, type ReactNode } from 'react'
import { invoke } from '@tauri-apps/api/core'
import { enable, disable, isEnabled } from '@tauri-apps/plugin-autostart'
import { useSettingsStore, type Locale, type ProxyMode, type ProxyType } from '../store/settingsStore'
import { useSyncStore } from '../store/syncStore'
import type { TranslationKey } from '../locales/en'

// ── Helper ──────────────────────────────────────────────────────────────────

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

function applyTemplate(template: string, values: Record<string, string>): string {
  return Object.entries(values).reduce(
    (result, [key, value]) => result.replace(`{${key}}`, value),
    template
  )
}

// ── Sub-components ──────────────────────────────────────────────────────────

type IconName =
  | 'account'
  | 'language'
  | 'proxy'
  | 'ignored'
  | 'startup'
  | 'about'
  | 'warning'
  | 'ban'
  | 'monitor'
  | 'sliders'

function SettingsIcon({ name }: { name: IconName }) {
  const common = {
    width: 14,
    height: 14,
    viewBox: '0 0 24 24',
    fill: 'none',
    stroke: 'currentColor',
    strokeWidth: 2,
    strokeLinecap: 'round' as const,
    strokeLinejoin: 'round' as const,
    'aria-hidden': true,
  }

  switch (name) {
    case 'account':
      return <svg {...common}><path d="M20 21a8 8 0 0 0-16 0" /><circle cx="12" cy="7" r="4" /></svg>
    case 'language':
      return <svg {...common}><circle cx="12" cy="12" r="10" /><path d="M2 12h20" /><path d="M12 2a15.3 15.3 0 0 1 0 20" /><path d="M12 2a15.3 15.3 0 0 0 0 20" /></svg>
    case 'proxy':
      return <svg {...common}><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" /><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" /></svg>
    case 'ignored':
      return <svg {...common}><circle cx="12" cy="12" r="10" /><path d="m4.9 4.9 14.2 14.2" /></svg>
    case 'startup':
      return <svg {...common}><path d="M4.5 16.5c-1.5 1.26-2 4-2 4s2.74-.5 4-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 0 0-2.91-.09z" /><path d="m12 15-3-3a22 22 0 0 1 2-3.95A12.88 12.88 0 0 1 22 2c0 2.72-.78 7.5-6 11a22 22 0 0 1-4 2z" /><path d="M9 12H4s.55-3.03 2-4c1.62-1.08 5 0 5 0" /><path d="M12 15v5s3.03-.55 4-2c1.08-1.62 0-5 0-5" /></svg>
    case 'about':
      return <svg {...common}><circle cx="12" cy="12" r="10" /><path d="M12 16v-4" /><path d="M12 8h.01" /></svg>
    case 'warning':
      return <svg {...common}><path d="m21.73 18-8-14a2 2 0 0 0-3.46 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3" /><path d="M12 9v4" /><path d="M12 17h.01" /></svg>
    case 'ban':
      return <svg {...common}><circle cx="12" cy="12" r="10" /><path d="m4.9 4.9 14.2 14.2" /></svg>
    case 'monitor':
      return <svg {...common}><rect width="20" height="14" x="2" y="3" rx="2" /><path d="M8 21h8" /><path d="M12 17v4" /></svg>
    case 'sliders':
      return <svg {...common}><path d="M4 21v-7" /><path d="M4 10V3" /><path d="M12 21v-9" /><path d="M12 8V3" /><path d="M20 21v-5" /><path d="M20 12V3" /><path d="M2 14h4" /><path d="M10 8h4" /><path d="M18 16h4" /></svg>
  }
}

function SectionHeader({ icon, label }: { icon: IconName; label: string }) {
  return (
    <div className="cfg-section-header">
      <span className="cfg-section-icon"><SettingsIcon name={icon} /></span>
      <span className="cfg-section-label">{label}</span>
    </div>
  )
}

function Card({ children, className = '' }: { children: ReactNode; className?: string }) {
  return <div className={`cfg-card ${className}`}>{children}</div>
}

// ── Account Section ─────────────────────────────────────────────────────────

function AccountSection({ t, onSignOut }: { t: (k: TranslationKey) => string; onSignOut: () => void }) {
  const { driveInfo, driveInfoLoading } = useSettingsStore()
  const [avatarFailed, setAvatarFailed] = useState(false)

  useEffect(() => {
    setAvatarFailed(false)
  }, [driveInfo?.photoUrl])

  const usedPct = driveInfo
    ? Math.min(100, Math.round((driveInfo.usedBytes / driveInfo.totalBytes) * 100))
    : 0

  const freeBytes = driveInfo ? Math.max(0, driveInfo.totalBytes - driveInfo.usedBytes) : 0

  // Determine fill color
  const fillColor =
    usedPct > 90 ? '#e74c3c' : usedPct > 70 ? '#f39c12' : 'var(--accent)'

  return (
    <Card>
      {driveInfoLoading ? (
        <div className="cfg-account-loading">
          <div className="cfg-spinner" />
          <span>{t('account_loading')}</span>
        </div>
      ) : driveInfo ? (
        <div className="cfg-account">
          <div className="cfg-account-avatar-wrap">
            {driveInfo.photoUrl && !avatarFailed ? (
              <img
                src={driveInfo.photoUrl}
                alt={driveInfo.displayName}
                className="cfg-account-avatar"
                referrerPolicy="no-referrer"
                onError={() => setAvatarFailed(true)}
              />
            ) : (
              <div className="cfg-account-avatar-fallback">
                {(driveInfo.displayName || driveInfo.email || '?')[0].toUpperCase()}
              </div>
            )}
            <span className="cfg-account-avatar-badge" title={t('account_google_drive')} />
          </div>

          <div className="cfg-account-info">
            <div className="cfg-account-name">{driveInfo.displayName || driveInfo.email}</div>
            <div className="cfg-account-email">{driveInfo.email}</div>

            <div className="cfg-storage-bar-wrap">
              <div className="cfg-storage-bar-bg">
                <div
                  className="cfg-storage-bar-fill"
                  style={{ width: `${usedPct}%`, background: fillColor }}
                />
              </div>
              <div className="cfg-storage-text">
                <span>
                  {applyTemplate(t('account_storage_used_of'), {
                    used: formatBytes(driveInfo.usedBytes),
                    total: formatBytes(driveInfo.totalBytes),
                  })}
                </span>
                <span>
                  {applyTemplate(t('account_storage_free'), {
                    free: formatBytes(freeBytes),
                  })}
                </span>
              </div>
            </div>
          </div>
        </div>
      ) : (
        <div className="cfg-account-loading">
          <span className="cfg-account-avatar-fallback" style={{ width: 40, height: 40, fontSize: 20 }}>?</span>
          <span style={{ color: 'var(--text1)', fontSize: 13 }}>{t('account_unknown')}</span>
        </div>
      )}

      <div className="cfg-account-footer">
        <button className="cfg-btn-signout" onClick={onSignOut}>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
            <polyline points="16 17 21 12 16 7" />
            <line x1="21" y1="12" x2="9" y2="12" />
          </svg>
          {t('account_sign_out')}
        </button>
      </div>
    </Card>
  )
}

// ── Language Section ─────────────────────────────────────────────────────────

function LanguageSection({ t }: { t: (k: TranslationKey) => string }) {
  const { locale, setLocale } = useSettingsStore()

  const langs: { value: Locale; label: TranslationKey }[] = [
    { value: 'en', label: 'language_en' },
    { value: 'pt-BR', label: 'language_pt_br' },
  ]

  return (
    <Card>
      <div className="cfg-row">
        <div className="cfg-row-label">
          <div className="cfg-row-title">{t('language_label')}</div>
        </div>
        <div className="cfg-lang-pills">
          {langs.map(({ value, label }) => (
            <button
              key={value}
              className={`cfg-lang-pill ${locale === value ? 'active' : ''}`}
              onClick={() => setLocale(value)}
            >
              <span>{value === 'en' ? '🇺🇸' : '🇧🇷'}</span>
              {t(label)}
            </button>
          ))}
        </div>
      </div>
    </Card>
  )
}

// ── Proxy Section ────────────────────────────────────────────────────────────

function ProxySection({
  t,
  sendCommand,
}: {
  t: (k: TranslationKey) => string
  sendCommand: (action: string, payload?: object) => void
}) {
  const { proxy, setProxy } = useSettingsStore()
  const lastWsError = useSyncStore(state => state.lastWsError)
  const setLastWsError = useSyncStore(state => state.setLastWsError)
  const networkError = useSyncStore(state => state.snapshot?.network_error ?? '')
  const dismissedNetworkError = useSyncStore(state => state.dismissedNetworkError)
  const dismissNetworkError = useSyncStore(state => state.dismissNetworkError)
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    if (lastWsError || networkError) {
      setSaved(false)
    }
  }, [lastWsError, networkError])

  const modes: { value: ProxyMode; labelKey: TranslationKey; icon: IconName }[] = [
    { value: 'none', labelKey: 'proxy_mode_none', icon: 'ban' },
    { value: 'system', labelKey: 'proxy_mode_system', icon: 'monitor' },
    { value: 'manual', labelKey: 'proxy_mode_manual', icon: 'sliders' },
  ]
  const types: { value: ProxyType; labelKey: TranslationKey }[] = [
    { value: 'socks', labelKey: 'proxy_type_socks' },
    { value: 'http', labelKey: 'proxy_type_http' },
  ]

  const saveProxy = (nextProxy = proxy) => {
    sendCommand('set_proxy', { proxy: nextProxy })
    setSaved(true)
    setTimeout(() => setSaved(false), 2000)
  }

  const handleModeChange = (mode: ProxyMode) => {
    const nextProxy = { ...proxy, mode }
    setProxy(nextProxy)
    if (mode !== 'manual') {
      saveProxy(nextProxy)
    }
  }

  const handleTypeChange = (type: ProxyType) => {
    const defaultPort = type === 'socks' ? '1080' : '8080'
    const currentPortIsDefault = proxy.port === '' || proxy.port === '1080' || proxy.port === '8080'
    setProxy({
      type,
      port: currentPortIsDefault ? defaultPort : proxy.port,
    })
  }

  return (
    <Card>
      <div className="cfg-proxy-modes">
        {modes.map(({ value, labelKey, icon }) => (
          <button
            key={value}
            className={`cfg-proxy-mode-btn ${proxy.mode === value ? 'active' : ''}`}
            onClick={() => handleModeChange(value)}
          >
            <span className="cfg-proxy-mode-icon"><SettingsIcon name={icon} /></span>
            {t(labelKey)}
          </button>
        ))}
      </div>

      {lastWsError && (
        <div className="folder-action-banner" role="status">
          {lastWsError}
          <button type="button" className="folder-action-dismiss" onClick={() => setLastWsError(null)} aria-label={t('files_cancel')}>
            ×
          </button>
        </div>
      )}

      {networkError && dismissedNetworkError !== networkError && (
        <div className="folder-action-banner proxy-error-banner" role="alert">
          <span>
            <strong>{t('proxy_error_title')}</strong> {networkError}
          </span>
          <button type="button" className="folder-action-dismiss" onClick={() => dismissNetworkError(networkError)} aria-label={t('files_cancel')}>
            ×
          </button>
        </div>
      )}

      {proxy.mode === 'manual' && (
        <div className="cfg-proxy-fields">
          <div className="cfg-input-group cfg-input-group-column">
            <label className="cfg-input-label">{t('proxy_type')}</label>
            <div className="cfg-proxy-modes cfg-proxy-types">
              {types.map(({ value, labelKey }) => (
                <button
                  key={value}
                  className={`cfg-proxy-mode-btn ${proxy.type === value ? 'active' : ''}`}
                  onClick={() => handleTypeChange(value)}
                >
                  {t(labelKey)}
                </button>
              ))}
            </div>
          </div>
          <div className="cfg-input-group cfg-input-group-row">
            <div className="cfg-input-item cfg-input-grow">
              <label className="cfg-input-label">{t('proxy_host')}</label>
              <input
                className="cfg-input"
                type="text"
                placeholder="127.0.0.1"
                value={proxy.host}
                onChange={(e) => setProxy({ host: e.target.value })}
              />
            </div>
            <div className="cfg-input-item" style={{ width: 90 }}>
              <label className="cfg-input-label">{t('proxy_port')}</label>
              <input
                className="cfg-input"
                type="number"
                placeholder="8080"
                value={proxy.port}
                onChange={(e) => setProxy({ port: e.target.value })}
              />
            </div>
          </div>
          <div className="cfg-input-group cfg-input-group-row">
            <div className="cfg-input-item cfg-input-grow">
              <label className="cfg-input-label">{t('proxy_username')}</label>
              <input
                className="cfg-input"
                type="text"
                value={proxy.username}
                onChange={(e) => setProxy({ username: e.target.value })}
              />
            </div>
            <div className="cfg-input-item cfg-input-grow">
              <label className="cfg-input-label">{t('proxy_password')}</label>
              <input
                className="cfg-input"
                type="password"
                value={proxy.password}
                onChange={(e) => setProxy({ password: e.target.value })}
              />
            </div>
          </div>
          {proxy.type === 'socks' && (
            <div className="cfg-row cfg-proxy-cert-row">
              <div className="cfg-row-label">
                <div className="cfg-row-title">{t('proxy_ignore_certificates')}</div>
                <div className="cfg-row-desc">{t('proxy_ignore_certificates_desc')}</div>
              </div>
              <label className="switch">
                <input
                  type="checkbox"
                  checked={Boolean(proxy.insecure_skip_verify)}
                  onChange={(e) => setProxy({ insecure_skip_verify: e.target.checked })}
                />
                <span className="slider round" />
              </label>
            </div>
          )}
          <button className="cfg-btn-save" onClick={() => saveProxy()}>
            {saved ? (
              <>
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                  <polyline points="20 6 9 17 4 12" />
                </svg>
                {t('proxy_saved')}
              </>
            ) : (
              t('proxy_save')
            )}
          </button>
        </div>
      )}
    </Card>
  )
}

// ── Ignored Folders Section ─────────────────────────────────────────────────

function IgnoredFoldersSection({
  t,
  sendCommand,
}: {
  t: (k: TranslationKey) => string
  sendCommand: (action: string, payload?: object) => void
}) {
  const { ignoredFolders, setIgnoredFolderPatterns } = useSettingsStore()
  const [input, setInput] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)

  const updateIgnoredFolders = (patterns: string[]) => {
    const normalized = Array.from(
      new Set(patterns.map((pattern) => pattern.trim()).filter(Boolean))
    )
    setIgnoredFolderPatterns(normalized)
    sendCommand('set_ignored_folders', { ignored_folders: normalized })
  }

  const handleAdd = () => {
    const val = input.trim()
    if (!val) return
    updateIgnoredFolders([...ignoredFolders.map((folder) => folder.pattern), val])
    setInput('')
    inputRef.current?.focus()
  }

  return (
    <Card>
      <p className="cfg-desc">{t('ignored_folders_desc')}</p>

      <div className="cfg-ignored-list">
        {ignoredFolders.length === 0 ? (
          <div className="cfg-ignored-empty">{t('ignored_folders_empty')}</div>
        ) : (
          ignoredFolders.map((f) => (
            <div key={f.id} className="cfg-ignored-chip">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
                <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
              </svg>
              <span className="cfg-ignored-name">{f.pattern}</span>
              <button
                className="cfg-ignored-remove"
                onClick={() => updateIgnoredFolders(
                  ignoredFolders
                    .filter((folder) => folder.id !== f.id)
                    .map((folder) => folder.pattern)
                )}
                title={t('files_remove_ignored_title')}
              >
                ×
              </button>
            </div>
          ))
        )}
      </div>

      <div className="cfg-ignored-add-row">
        <input
          ref={inputRef}
          className="cfg-input"
          placeholder={t('ignored_folders_placeholder')}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
        />
        <button className="cfg-btn-add" onClick={handleAdd} disabled={!input.trim()}>
          {t('ignored_folders_add')}
        </button>
      </div>
    </Card>
  )
}

// ── Startup Section ──────────────────────────────────────────────────────────

function StartupSection({ t }: { t: (k: TranslationKey) => string }) {
  const [isAutostart, setIsAutostart] = useState(false)
  const [canAutostart, setCanAutostart] = useState(true)

  useEffect(() => {
    isEnabled().then(setIsAutostart).catch(console.error)
    invoke<boolean>('can_autostart').then(setCanAutostart).catch(console.error)
  }, [])

  const toggle = async () => {
    try {
      if (isAutostart) {
        await disable()
        setIsAutostart(false)
      } else {
        await enable()
        setIsAutostart(true)
      }
    } catch (e) {
      console.error(e)
    }
  }

  if (!canAutostart) return null

  return (
    <Card>
      <div className="cfg-row">
        <div className="cfg-row-label">
          <div className="cfg-row-title">{t('startup_on_boot')}</div>
          <div className="cfg-row-desc">{t('startup_on_boot_desc')}</div>
        </div>
        <label className="switch">
          <input type="checkbox" checked={isAutostart} onChange={toggle} />
          <span className="slider round" />
        </label>
      </div>
    </Card>
  )
}

// ── About Section ────────────────────────────────────────────────────────────

function AboutSection({ t }: { t: (k: TranslationKey) => string }) {
  return (
    <Card>
      <div className="cfg-about-rows">
        <div className="cfg-about-row">
          <span className="cfg-about-key">{t('about_version')}</span>
          <span className="cfg-about-val">v0.4.0</span>
        </div>
        <div className="cfg-about-row">
          <span className="cfg-about-key">{t('about_license')}</span>
          <span className="cfg-about-val">MIT</span>
        </div>
        <div className="cfg-about-row">
          <span className="cfg-about-key">GitHub</span>
          <a
            className="cfg-about-link"
            href="https://github.com/bryanrafaelbueno/synca"
            target="_blank"
            rel="noopener noreferrer"
          >
            {t('about_github')} ↗
          </a>
        </div>
      </div>
    </Card>
  )
}

// ── Main SettingsPage ────────────────────────────────────────────────────────

interface SettingsPageProps {
  onSignOut: () => void
  onRefreshAccount: () => void
  sendCommand: (action: string, payload?: object) => void
}

export function SettingsPage({ onSignOut, onRefreshAccount, sendCommand }: SettingsPageProps) {
  const { t } = useSettingsStore()

  useEffect(() => {
    void onRefreshAccount()
  }, [onRefreshAccount])

  const sections: { icon: IconName; key: TranslationKey; content: ReactNode }[] = [
    {
      icon: 'account',
      key: 'settings_account',
      content: <AccountSection t={t} onSignOut={onSignOut} />,
    },
    {
      icon: 'language',
      key: 'settings_language',
      content: <LanguageSection t={t} />,
    },
    {
      icon: 'proxy',
      key: 'settings_proxy',
      content: <ProxySection t={t} sendCommand={sendCommand} />,
    },
    {
      icon: 'ignored',
      key: 'settings_ignored_folders',
      content: <IgnoredFoldersSection t={t} sendCommand={sendCommand} />,
    },
    {
      icon: 'startup',
      key: 'settings_startup',
      content: <StartupSection t={t} />,
    },
    {
      icon: 'about',
      key: 'settings_about',
      content: <AboutSection t={t} />,
    },
  ]

  return (
    <div className="cfg-page">
      <div className="cfg-header">
        <h1 className="cfg-title">{t('settings_title')}</h1>
      </div>

      <div className="cfg-body">
        {sections.map(({ icon, key, content }) => (
          <section key={key} className="cfg-section">
            <SectionHeader icon={icon} label={t(key)} />
            {content}
          </section>
        ))}

        {/* Danger zone */}
        <section className="cfg-section">
          <SectionHeader icon="warning" label={t('danger_zone')} />
          <Card className="cfg-card-danger">
            <div className="cfg-danger-row">
              <div>
                <div className="cfg-row-title">{t('danger_sign_out_title')}</div>
                <div className="cfg-row-desc">{t('danger_sign_out_desc')}</div>
              </div>
              <button className="cfg-btn-danger" onClick={onSignOut}>
                {t('account_sign_out')}
              </button>
            </div>
          </Card>
        </section>
      </div>
    </div>
  )
}
