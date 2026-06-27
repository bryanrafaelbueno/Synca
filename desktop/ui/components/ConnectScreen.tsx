import { invoke } from '@tauri-apps/api/core';
import { useState } from 'react';
import type { SetupState } from '../App';
import { useSettingsStore } from '../store/settingsStore';

interface ConnectScreenProps {
  setupState: SetupState | 'socket_error' | 'connecting'
  error: string
  checkSetup: () => void
}

export function ConnectScreen({ setupState, error, checkSetup }: ConnectScreenProps) {
  const [isLoggingIn, setIsLoggingIn] = useState(false);
  const { t } = useSettingsStore();

  const handleLogin = async () => {
    setIsLoggingIn(true);
    try {
      await invoke('login_google_drive');
      checkSetup(); 
    } catch (e) {
      console.error("Fatal error:", e);
      alert(t('connect_login_failed') + e);
    } finally {
      setIsLoggingIn(false);
    }
  };


  return (
    <div className="connect-screen">
      <div className="connect-card">
        <div className="connect-icon">
          <svg width="32" height="32" viewBox="0 0 32 32" fill="none">
            <path d="M16 4L26 14H20V22H12V14H6L16 4Z" fill="currentColor" opacity="0.9"/>
            <path d="M16 28L6 18H12V10H20V18H26L16 28Z" fill="currentColor" opacity="0.3"/>
          </svg>
        </div>
        <h2 className="connect-title">
           {setupState === 'checking' ? t('connect_starting_title') :
            setupState === 'connecting' ? t('connect_daemon_title') :
            setupState === 'needs_creds' ? t('connect_initial_setup_title') :
            setupState === 'needs_token' ? t('connect_auth_required_title') : t('connect_pending_title')}
        </h2>

        <p className="connect-msg">
           {setupState === 'checking' ? t('connect_saved_settings') :
            setupState === 'connecting' ? t('connect_waiting_daemon') :
            setupState === 'needs_token' ? t('connect_authorize') : error}
        </p>

        <div style={{ marginTop: '20px', display: 'flex', flexDirection: 'column', gap: '10px' }}>

          {setupState === 'connecting' && (
            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', justifyContent: 'center' }}>
              <svg width="20" height="20" viewBox="0 0 20 20" style={{ animation: 'spin 1s linear infinite' }}>
                <circle cx="10" cy="10" r="8" fill="none" stroke="currentColor" strokeWidth="2" strokeDasharray="40 10" opacity="0.5"/>
              </svg>
              <span>{t('connect_starting_daemon')}</span>
            </div>
          )}

          {setupState === 'needs_token' && (
            <button className="btn-connect" onClick={handleLogin} disabled={isLoggingIn}>
              {isLoggingIn ? t('connect_authenticating') : t('connect_login')}
            </button>
          )}

          {(setupState === 'error' || setupState === 'socket_error') && (
            <button
              className="btn-connect"
              onClick={checkSetup}
              style={{ opacity: 0.8, backgroundColor: 'transparent', border: '1px solid currentColor' }}
            >
              {t('connect_reload')}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
