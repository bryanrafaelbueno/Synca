import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import en, { type TranslationKey } from '../locales/en'
import ptBR from '../locales/pt-BR'

// ── Types ───────────────────────────────────────────────────────────────────

export type Locale = 'en' | 'pt-BR'
export type ProxyMode = 'none' | 'system' | 'manual'
export type ProxyType = 'socks' | 'http'

export interface ProxySettings {
  mode: ProxyMode
  type: ProxyType
  host: string
  port: string
  username: string
  password: string
  insecure_skip_verify: boolean
}

export interface IgnoredFolder {
  id: string
  pattern: string
}

export interface DriveInfo {
  email: string
  displayName: string
  photoUrl: string
  usedBytes: number
  totalBytes: number
}

// ── Translations map ────────────────────────────────────────────────────────

const translations: Record<Locale, Record<TranslationKey, string>> = {
  en: en as Record<TranslationKey, string>,
  'pt-BR': ptBR,
}

// ── Store ───────────────────────────────────────────────────────────────────

interface SettingsStore {
  locale: Locale
  proxy: ProxySettings
  ignoredFolders: IgnoredFolder[]
  driveInfo: DriveInfo | null
  driveInfoLoading: boolean

  // Actions
  setLocale: (locale: Locale) => void
  setProxy: (proxy: Partial<ProxySettings>) => void
  addIgnoredFolder: (pattern: string) => void
  removeIgnoredFolder: (id: string) => void
  setIgnoredFolderPatterns: (patterns: string[]) => void
  setDriveInfo: (info: DriveInfo | null) => void
  setDriveInfoLoading: (loading: boolean) => void

  // Translation helper
  t: (key: TranslationKey) => string
}

export const useSettingsStore = create<SettingsStore>()(
  persist(
    (set, get) => ({
      locale: 'en',
      proxy: {
        mode: 'none',
        type: 'socks',
        host: '',
        port: '1080',
        username: '',
        password: '',
        insecure_skip_verify: false,
      },
      ignoredFolders: [
        { id: '1', pattern: 'node_modules' },
        { id: '2', pattern: '.git' },
      ],
      driveInfo: null,
      driveInfoLoading: false,

      setLocale: (locale) => set({ locale }),
      setProxy: (partial) =>
        set((state) => {
          const proxy = { ...state.proxy, ...partial }
          return {
            proxy: {
              ...proxy,
              insecure_skip_verify: Boolean(proxy.insecure_skip_verify),
            },
          }
        }),
      addIgnoredFolder: (pattern) =>
        set((state) => ({
          ignoredFolders: [
            ...state.ignoredFolders,
            { id: crypto.randomUUID(), pattern: pattern.trim() },
          ],
        })),
      removeIgnoredFolder: (id) =>
        set((state) => ({
          ignoredFolders: state.ignoredFolders.filter((f) => f.id !== id),
        })),
      setIgnoredFolderPatterns: (patterns) =>
        set({
          ignoredFolders: patterns
            .map((pattern) => pattern.trim())
            .filter(Boolean)
            .map((pattern) => ({ id: pattern, pattern })),
        }),
      setDriveInfo: (info) => set({ driveInfo: info }),
      setDriveInfoLoading: (loading) => set({ driveInfoLoading: loading }),

      t: (key) => {
        const locale = get().locale
        return translations[locale]?.[key] ?? translations['en'][key] ?? key
      },
    }),
    {
      name: 'synca-settings',
      partialize: (state) => ({
        locale: state.locale,
        proxy: state.proxy,
        ignoredFolders: state.ignoredFolders,
      }),
    }
  )
)
