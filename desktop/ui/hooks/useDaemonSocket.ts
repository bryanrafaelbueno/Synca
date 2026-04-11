import { useEffect, useRef, useCallback } from 'react'
import { useSyncStore, type StatusSnapshot } from '../store/syncStore'

const WS_URL = 'ws://localhost:7373/ws'
const RECONNECT_DELAY_MS = 1500
const MAX_RECONNECT_ATTEMPTS = 40

export function useDaemonSocket() {
  const ws = useRef<WebSocket | null>(null)
  const reconnectTimer = useRef<ReturnType<typeof setTimeout>>()
  const attempts = useRef(0)

  const { setSnapshot, setConnected, setError, setLastWsError } = useSyncStore()

  const connect = useCallback(() => {
    if (ws.current?.readyState === WebSocket.OPEN) return

    try {
      const socket = new WebSocket(WS_URL)
      ws.current = socket

      socket.onopen = () => {
        attempts.current = 0
        setConnected(true)
        setError(null)
        // Request immediate status
        socket.send(JSON.stringify({ action: 'get_status' }))
      }

      socket.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data) as Record<string, unknown>
          if (typeof data.error === 'string' && data.error.length > 0) {
            setLastWsError(data.error)
            return
          }
          if (!Array.isArray(data.files)) {
            return
          }
          setSnapshot(data as unknown as StatusSnapshot)
        } catch (e) {
          console.error('Failed to parse daemon message:', e)
        }
      }

      socket.onclose = () => {
        setConnected(false)
        if (attempts.current < MAX_RECONNECT_ATTEMPTS) {
          attempts.current++
          reconnectTimer.current = setTimeout(connect, RECONNECT_DELAY_MS)
        } else {
          setError('Cannot connect to Synca daemon. Is it running?')
        }
      }

      socket.onerror = () => {
        setConnected(false)
      }
    } catch (e) {
      setError('Failed to connect to daemon')
    }
  }, [setSnapshot, setConnected, setError, setLastWsError])

  useEffect(() => {
    connect()
    return () => {
      clearTimeout(reconnectTimer.current)
      ws.current?.close()
    }
  }, [connect])

  const sendCommand = useCallback((action: string, payload?: object) => {
    if (action === 'restart_daemon') {
      attempts.current = 0
    }
    if (ws.current?.readyState === WebSocket.OPEN) {
      ws.current.send(JSON.stringify({ action, ...payload }))
    }
  }, [])

  return { sendCommand }
}
