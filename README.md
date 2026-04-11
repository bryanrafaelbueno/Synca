# Synca — Cloud sync para Linux 🔄

> Cliente de sincronização open source para Linux. Alternativa gratuita ao Insync.

![Status](https://img.shields.io/badge/status-MVP-blue)
![License](https://img.shields.io/badge/license-MIT-green)
![Go](https://img.shields.io/badge/go-1.22+-00ADD8)
![Tauri](https://img.shields.io/badge/tauri-2.x-FFC131)

---

## Stack

| Camada | Tecnologia | Por quê |
|--------|-----------|---------|
| Daemon | Go 1.22 | Performance, baixo uso de memória, binário único |
| File watching | fsnotify | Nativo no kernel (inotify no Linux) |
| Drive API | google.golang.org/api | Client oficial, suporte a upload incremental |
| Auth | OAuth2 (golang.org/x/oauth2) | Fluxo seguro, token persistido localmente |
| UI | Tauri + React + TypeScript | App nativo leve (~5MB), sem Electron |
| Estado | Zustand | Simples, sem boilerplate |
| IPC | WebSocket (gorilla/websocket) | Comunicação em tempo real daemon ↔ UI |

---

## Arquitetura

```
synca/
├── daemon/                       # Go backend
│   ├── cmd/synca/main.go         # CLI (daemon, connect, watch, status)
│   └── internal/
│       ├── auth/                 # OAuth2 Google Drive
│       │   ├── oauth.go          # Fluxo de autenticação + token storage
│       │   └── browser.go        # Abertura do browser cross-platform
│       ├── config/               # Configuração JSON (~/.config/synca/)
│       ├── watcher/              # fsnotify com debounce + watch recursivo
│       ├── drive/                # Google Drive API client
│       ├── conflicts/            # Detecção e resolução de conflitos
│       ├── sync/                 # Engine principal (workers + queue)
│       └── server/               # WebSocket server + REST /status
│
├── desktop/                      # Tauri frontend (React + TS)
│   └── src/
│       ├── App.tsx
│       ├── app.css
│       ├── hooks/
│       │   └── useDaemonSocket.ts # WebSocket com auto-reconnect
│       ├── store/
│       │   └── syncStore.ts       # Estado global (Zustand)
│       └── components/
│           ├── Sidebar.tsx        # Stats, progresso, controles
│           ├── FileList.tsx       # Lista de arquivos + status pills
│           ├── StatusBar.tsx      # Barra inferior de conexão
│           └── ConnectScreen.tsx  # Tela de erro/setup
│
└── releases/                     # Instaladores (Gerados por make release-*)
```

### Fluxo de sincronização

```
Arquivo modificado
      │
      ▼
  fsnotify (inotify)
      │
      ▼
  Debounce 500ms ──► skip (burst de writes)
      │
      ▼
  WorkQueue (buffered chan, 512)
      │
      ▼
  Worker Pool (4 goroutines)
      │
      ├── MD5 local == MD5 remoto? ──► skip
      │
      ├── Conflito? (ambos modificados desde última sync)
      │     ├── StrategyKeepBoth → cria cópia com timestamp
      │     ├── StrategyNewerWins → mantém o mais recente
      │     └── StrategyLocalWins / RemoteWins
      │
      ▼
  Drive API — upload incremental
      │
      ▼
  Atualiza estado + broadcast WebSocket → UI
```

---

## Setup

### Pré-requisitos

- Go 1.22+
- Node.js 18+ e npm
- Rust (stable) + Linux WebKitGTK runtime dependencies
- Conta Google com Drive

### 1. Clone e instale dependências

```bash
git clone https://github.com/synca/synca
cd synca
make setup
```

### 2. Configure credenciais Google

```bash
make setup-creds
```

Siga as instruções:
1. Acesse [Google Cloud Console](https://console.cloud.google.com)
2. Crie um projeto e ative a **Google Drive API**
3. Crie credenciais **OAuth 2.0** → tipo **Desktop app**
4. Baixe `credentials.json` → `~/.config/synca/credentials.json`

### 3. Compile o daemon

```bash
make daemon
```

### 4. Autentique com o Google Drive

```bash
./bin/synca-daemon connect google-drive
# Abre o browser para autorização OAuth2
# Token salvo em ~/.config/synca/token.json
```

### 5. Configure a pasta a sincronizar

```bash
./bin/synca-daemon watch ~/Documentos
# Adiciona o caminho em ~/.config/synca/config.json
```

### 6. Inicie o daemon

```bash
./bin/synca-daemon daemon
# WebSocket disponível em ws://localhost:7373/ws
# REST status em http://localhost:7373/status
```

### 7. Abra a interface

```bash
make app-dev
# ou para build de produção:
make build
```

### 8. Gerar release Linux e Windows

**Linux:**

```bash
make release-linux
```
Artefatos generados na raiz do projeto em: `releases/linux/` (Contém pacotes `.deb` e executáveis `AppImage`).

**Windows:**

```bash
make release-windows
```
Artefatos generados na raiz do projeto em: `releases/windows/` (Contém instaladores nativos `.msi` e `.nsis`). 

*Nota: Para que o instalador do sidecar seja gerado perfeitamente no Linux apontando para Windows, sua máquina deve ter suporte à cross-compilation (mingw-w64 ativo em rust).*
---

## CLI Reference

```bash
synca daemon                    # Inicia o daemon de sincronização
synca connect google-drive      # Autentica via OAuth2
synca watch ~/pasta             # Adiciona pasta para monitorar
synca status                    # Mostra status do daemon em execução
```

---

## Configuração

Arquivo: `~/.config/synca/config.json`

```json
{
  "ws_addr": "localhost:7373",
  "watch_paths": ["/home/user/Documentos"],
  "token_file": "/home/user/.config/synca/token.json",
  "cred_file": "/home/user/.config/synca/credentials.json",
  "conflict_dir": "/home/user/.config/synca/conflicts",
  "log_level": "info"
}
```

---

## Estratégias de conflito

| Estratégia | Comportamento |
|-----------|---------------|
| `KeepBoth` (padrão) | Cria cópia local com timestamp: `arquivo (conflicted copy 2024-01-15).md` |
| `NewerWins` | Mantém a versão com data de modificação mais recente |
| `LocalWins` | Sempre mantém a versão local |
| `RemoteWins` | Sempre baixa a versão remota |

---

## Roadmap MVP → v1.0

- [x] Auth OAuth2 Google Drive
- [x] Watch recursivo de pastas (fsnotify + debounce)
- [x] Upload automático ao Drive
- [x] Detecção e resolução de conflitos
- [x] WebSocket server para UI em tempo real
- [x] Interface Tauri com status dos arquivos
- [ ] Download / sync bidirecional completo
- [ ] Indicador de progresso por arquivo (multipart upload)
- [ ] System tray com menu rápido
- [ ] Suporte a múltiplas pastas na UI
- [ ] Exclusões por padrão (.gitignore syntax)
- [ ] Suporte a Rclone (OneDrive, Dropbox, S3)
- [ ] Auto-start no login (systemd unit)
- [ ] Notificações do sistema

---

## Contribuindo

Pull requests são bem-vindos! Para mudanças grandes, abra uma issue primeiro.

```bash
# Fork → clone → branch
git checkout -b feat/minha-feature
# Código, testes, commit
git push origin feat/minha-feature
# Abra PR no GitHub
```

---

## Licença

MIT © Synca Contributors
