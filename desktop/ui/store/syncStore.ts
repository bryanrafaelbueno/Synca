import { create } from 'zustand'

export type FileStatus = 'synced' | 'syncing' | 'queued' | 'conflict' | 'error'

export interface FileEntry {
  local_path: string
  remote_id: string
  remote_name: string
  status: FileStatus
  last_sync: string
  local_md5: string
  remote_md5: string
  size: number
  is_dir?: boolean
  error?: string
}

export interface StatusSnapshot {
  files: FileEntry[] | null
  total_bytes: number
  total_files: number
  synced_files: number
  is_running: boolean
  last_updated: string
}

interface SyncStore {
  snapshot: StatusSnapshot | null
  connected: boolean
  error: string | null
  searchQuery: string
  /** Transient message from daemon (e.g. add_watch validation). */
  lastWsError: string | null

  setSnapshot: (snap: StatusSnapshot) => void
  setConnected: (v: boolean) => void
  setError: (e: string | null) => void
  setSearchQuery: (q: string) => void
  setLastWsError: (e: string | null) => void
}

const defaultSnapshot: StatusSnapshot = {
  files: [],
  total_bytes: 0,
  total_files: 0,
  synced_files: 0,
  is_running: false,
  last_updated: new Date().toISOString(),
}

export const useSyncStore = create<SyncStore>((set) => ({
  snapshot: defaultSnapshot,
  connected: false,
  error: null,
  searchQuery: '',
  lastWsError: null,

  setSnapshot: (snapshot) => set({ snapshot, lastWsError: null }),
  setConnected: (connected) => set({ connected }),
  setError: (error) => set({ error }),
  setSearchQuery: (searchQuery) => set({ searchQuery }),
  setLastWsError: (lastWsError) => set({ lastWsError }),
}))

// Derived selectors
export const selectFiles = (state: SyncStore) =>
  (state.snapshot?.files ?? []).filter((f) => {
    if (!state.searchQuery) return true;
    const query = state.searchQuery.toLowerCase().replace(/\\/g, '/');
    const path = f.local_path.toLowerCase().replace(/\\/g, '/');
    return path.includes(query);
  })

export const selectStats = (state: SyncStore) => ({
  totalBytes: state.snapshot?.total_bytes ?? 0,
  totalFiles: state.snapshot?.total_files ?? 0,
  syncedFiles: state.snapshot?.synced_files ?? 0,
  isRunning: state.snapshot?.is_running ?? false,
  lastUpdated: state.snapshot?.last_updated ?? null,
})
