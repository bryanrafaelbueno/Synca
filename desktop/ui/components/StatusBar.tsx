interface Props {
  connected: boolean
}

export function StatusBar({ connected }: Props) {
  return (
    <div className="status-bar">
      <div className={`status-dot ${connected ? 'online' : 'offline'}`} />
      <span className="status-text">
        {connected ? 'Daemon conectado' : 'Daemon desconectado'}
      </span>
      <span className="status-addr">localhost:7373</span>
    </div>
  )
}
