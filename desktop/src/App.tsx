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

  const checkSetup = async () => {
    try {
      const creds = await invoke<boolean>('has_credentials')
      if (!creds) {
        setSetupState('needs_creds')
        return
      }
      const token = await invoke<boolean>('has_token')
      if (!token) {
        setSetupState('needs_token')
        return
      }
      
      try {
        await invoke('start_daemon')
      } catch (e) {
        console.warn("Daemon start result:", e)
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

  if (setupState !== 'ready') {
    return <ConnectScreen setupState={setupState} checkSetup={checkSetup} error={setupError} />
  }

  return <MainApp />
}

function MainApp() {
  const { sendCommand } = useDaemonSocket()
  const { connected, error } = useSyncStore()

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
