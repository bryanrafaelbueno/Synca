import { invoke } from '@tauri-apps/api/core';
import { open } from '@tauri-apps/plugin-dialog';
import { useState } from 'react';
import type { SetupState } from '../App';

interface ConnectScreenProps {
  setupState: SetupState | 'socket_error'
  error: string
  checkSetup: () => void
}

export function ConnectScreen({ setupState, error, checkSetup }: ConnectScreenProps) {
  const [isLoggingIn, setIsLoggingIn] = useState(false);

  const handleLogin = async () => {
    setIsLoggingIn(true);
    try {
      await invoke('login_google_drive');
      checkSetup(); 
    } catch (e) {
      console.error("Fatal error:", e);
      alert("Login failed: " + e);
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
           {setupState === 'checking' ? "Starting Synca..." : 
            setupState === 'needs_creds' ? "Initial Setup" : 
            setupState === 'needs_token' ? "Authentication Required" : "Connection Pending"}
        </h2>
        
        <p className="connect-msg">
           {setupState === 'checking' ? "Looking for saved settings..." :
            setupState === 'needs_token' ? "Log in to Google Drive to authorize Synca." : error}
        </p>
        
        <div style={{ marginTop: '20px', display: 'flex', flexDirection: 'column', gap: '10px' }}>

          {setupState === 'needs_token' && (
            <button className="btn-connect" onClick={handleLogin} disabled={isLoggingIn}>
              {isLoggingIn ? "Authenticating on the Web..." : "Log in to Google Drive"}
            </button>
          )}

          {(setupState === 'error' || setupState === 'socket_error') && (
            <button
              className="btn-connect"
              onClick={checkSetup}
              style={{ opacity: 0.8, backgroundColor: 'transparent', border: '1px solid currentColor' }}
            >
              Reload
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
