import { useSettingsStore } from '../store/settingsStore'
import { useSyncStore } from '../store/syncStore'

interface Props {
  connected: boolean
}

export function StatusBar({ connected }: Props) {
  const { t } = useSettingsStore()
  const networkError = useSyncStore(state => state.snapshot?.network_error ?? '')
  const dismissedNetworkError = useSyncStore(state => state.dismissedNetworkError)
  const showNetworkError = networkError && dismissedNetworkError !== networkError

  return (
    <div className="status-bar">
      <div className={`status-dot ${connected ? 'online' : 'offline'}`} />
      <span className="status-text">
        {connected ? t('status_daemon_connected') : t('status_daemon_disconnected')}
      </span>
      {showNetworkError && (
        <span className="status-proxy-error" title={networkError}>
          {t('proxy_error_status')}
        </span>
      )}
      <span className="status-addr">localhost:7373</span>
    </div>
  )
}
