import { useCallback, useEffect, useState } from 'react'
import { invoke } from '@tauri-apps/api/core'
import { useDaemonSocket } from './hooks/useDaemonSocket'
import { Sidebar, type AppView } from './components/Sidebar'
import { FileList } from './components/FileList'
import { StatusBar } from './components/StatusBar'
import { ConnectScreen } from './components/ConnectScreen'
import { SettingsPage } from './components/SettingsPage'
import { useSyncStore } from './store/syncStore'
import { useSettingsStore } from './store/settingsStore'
import './app.css'

export type SetupState = 'checking' | 'needs_creds' | 'needs_token' | 'ready' | 'error'

export default function App() {
  const [setupState, setSetupState] = useState<SetupState>('checking')
  const [setupError, setSetupError] = useState('')
  const { connected } = useSyncStore()

  const checkSetup = async () => {
    try {
      const token = await invoke<boolean>('has_token')
      if (!token) {
        setSetupState('needs_token')
        return
      }

      // Start daemon and wait for health check
      try {
        await invoke('start_daemon')
      } catch (e) {
        console.error("Daemon start failed:", e)
        setSetupError(String(e))
        setSetupState('error')
        return
      }

      setSetupState('ready')
    } catch (e) {
      console.error(e)
      setSetupError(String(e))
      setSetupState('error')
    }
  }

  useEffect(() => {
    checkSetup()
  }, [])

  // Wait for WebSocket connection after setup is ready
  useEffect(() => {
    if (setupState === 'ready' && connected) {
      // All good, let the app render
    } else if (setupState === 'ready' && !connected) {
      // Still connecting, show a loading state
    }
  }, [setupState, connected])

  if (setupState !== 'ready') {
    return <ConnectScreen setupState={setupState} checkSetup={checkSetup} error={setupError} />
  }

  // Always render MainApp once setup is ready; it handles the connecting state internally
  return <MainApp onLoggedOut={() => { setSetupState('needs_token') }} />
}

function MainApp({ onLoggedOut }: { onLoggedOut: () => void }) {
  const { sendCommand } = useDaemonSocket()
  const { connected, error } = useSyncStore()
  const { setDriveInfo, setDriveInfoLoading, t } = useSettingsStore()
  const [currentView, setCurrentView] = useState<AppView>('files')

  const loadDriveInfo = useCallback(async () => {
    setDriveInfoLoading(true)
    try {
      const response = await fetch('http://localhost:7373/account')
      if (!response.ok) {
        setDriveInfo(null)
        return
      }

      const data = await response.json()
      const totalBytes = Number(data.total_bytes ?? data.storage_limit ?? 0)

      setDriveInfo({
        email: data.email ?? '',
        displayName: data.display_name ?? data.name ?? data.email ?? '',
        photoUrl: data.photo_url ?? data.picture ?? '',
        usedBytes: Number(data.used_bytes ?? data.storage_used ?? 0),
        totalBytes: totalBytes > 0 ? totalBytes : 15 * 1024 * 1024 * 1024,
      })
    } catch (e) {
      console.error('Failed to load Drive account info:', e)
      setDriveInfo(null)
    } finally {
      setDriveInfoLoading(false)
    }
  }, [setDriveInfo, setDriveInfoLoading])

  // Load drive info (user profile) from the daemon.
  useEffect(() => {
    if (connected) {
      void loadDriveInfo()
    }
  }, [connected, loadDriveInfo])

  const handleSignOut = async () => {
    const ok = await invoke<boolean>('confirm_dialog', {
      message: t('account_sign_out_confirm'),
      title: t('danger_sign_out_title'),
    }).catch(() => false)
    if (ok) {
      // Remove token by invoking logout (best-effort; redirect to login screen)
      try { await invoke('logout_google_drive') } catch (_) { /* command may not exist yet */ }
      onLoggedOut()
    }
  }

  // Show error screen if connection failed
  if (error && !connected) {
    return <ConnectScreen setupState="socket_error" checkSetup={() => window.location.reload()} error={error} />
  }

  return (
    <div className="app">
      <Sidebar
        sendCommand={sendCommand}
        currentView={currentView}
        onNavigate={setCurrentView}
      />
      <main className="main">
        {currentView === 'settings' ? (
          <SettingsPage
            onSignOut={handleSignOut}
            onRefreshAccount={loadDriveInfo}
            sendCommand={sendCommand}
          />
        ) : (
          <FileList sendCommand={sendCommand} />
        )}
      </main>
      <StatusBar connected={connected} />
    </div>
  )
}
