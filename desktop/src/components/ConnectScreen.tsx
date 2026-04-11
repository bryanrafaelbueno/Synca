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
      console.error("Falha fatal:", e);
      alert("Falha no login: " + e);
    } finally {
      setIsLoggingIn(false);
    }
  };

  const handleUploadCredentials = async () => {
    try {
      const selected = await open({
        multiple: false,
        filters: [{ name: 'JSON Config', extensions: ['json'] }]
      });
      if (selected && typeof selected === 'string') {
        await invoke('save_credentials', { sourcePath: selected });
        alert("Credenciais salvas com sucesso!");
        checkSetup();
      }
    } catch (e) {
      console.error(e);
      alert("Erro ao salvar arquivo: " + e);
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
           {setupState === 'checking' ? "Iniciando Synca..." : 
            setupState === 'needs_creds' ? "Configuração Inicial" : 
            setupState === 'needs_token' ? "Falta Autenticar" : "Conexão Pendente"}
        </h2>
        
        <p className="connect-msg">
           {setupState === 'checking' ? "Procurando configurações salvas..." :
            setupState === 'needs_creds' ? "Você precisa fazer upload do arquivo credentials.json do Google Cloud." : 
            setupState === 'needs_token' ? "Faça login no Google Drive para autorizar o Synca." : error}
        </p>
        
        <div style={{ marginTop: '20px', display: 'flex', flexDirection: 'column', gap: '10px' }}>
          {setupState === 'needs_creds' && (
            <button className="btn-connect" onClick={handleUploadCredentials}>
              Fazer Upload (credentials.json)
            </button>
          )}

          {setupState === 'needs_token' && (
            <button className="btn-connect" onClick={handleLogin} disabled={isLoggingIn}>
              {isLoggingIn ? "Autenticando na Web..." : "Logar no Google Drive"}
            </button>
          )}

          {(setupState === 'error' || setupState === 'socket_error') && (
            <button
              className="btn-connect"
              onClick={checkSetup}
              style={{ opacity: 0.8, backgroundColor: 'transparent', border: '1px solid currentColor' }}
            >
              Recarregar
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
