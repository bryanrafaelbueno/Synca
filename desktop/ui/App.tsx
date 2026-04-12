import { useEffect, useState } from 'react'
import { invoke } from '@tauri-apps/api/core'
import { useDaemonSocket } from './hooks/useDaemonSocket'
import { Sidebar } from './components/Sidebar'
import { FileList } from './components/FileList'
import { StatusBar } from './components/StatusBar'
import { ConnectScreen } from './components/ConnectScreen'
import { useSyncStore } from './store/syncStore'
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
  return <MainApp />
}

function MainApp() {
  const { sendCommand } = useDaemonSocket()
  const { connected, error } = useSyncStore()

  // Show error screen if connection failed
  if (error && !connected) {
    return <ConnectScreen setupState="socket_error" checkSetup={() => window.location.reload()} error={error} />
  }

  return (
    <div className="app">
      <Sidebar sendCommand={sendCommand} />
      <main className="main">
        <FileList sendCommand={sendCommand} />
      </main>
      <StatusBar connected={connected} />
    </div>
  )
}
