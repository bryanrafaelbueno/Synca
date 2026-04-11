<p align="center">
  <img src="./assets/SyncaWhite.png" alt="Synca Logo" width="480"/>
</p>

<p align="center">
  <b>Sincronização de arquivos com o Google Drive no Linux e Windows</b><br/>
  Simples, leve e open source — uma alternativa gratuita ao Insync
</p>

<p align="center">
  <img src="https://img.shields.io/badge/status-MVP-blue"/>
  <img src="https://img.shields.io/badge/license-MIT-green"/>
  <img src="https://img.shields.io/badge/go-1.22+-00ADD8"/>
  <img src="https://img.shields.io/badge/tauri-2.x-FFC131"/>
  <img src="https://img.shields.io/badge/platform-linux%20%7C%20windows-lightgrey"/>
</p>

---

## ✨ O que é o Synca?

O **Synca** é um cliente de sincronização que conecta suas pastas locais ao Google Drive automaticamente.

Ele roda silenciosamente no fundo (daemon), detecta mudanças nos arquivos e mantém tudo sincronizado em tempo real.

💡 Ideal para quem quer algo leve, open source e sem depender de soluções pagas.

---

## 🚀 Features

- 🔄 Sincronização automática (quase em tempo real)
- 📂 Monitoramento recursivo de pastas
- ⚡ Baixo consumo de memória (Go + Tauri)
- 🔐 Login seguro com Google (OAuth2)
- 🧠 Resolução inteligente de conflitos
- 📡 Comunicação em tempo real (WebSocket)
- 🖥️ Interface leve e nativa
- 🪟 Suporte a Windows e Linux

---

## 🧠 Como funciona


```
Você salva um arquivo
        ↓
Synca detecta a mudança
        ↓
Processa em background
        ↓
Envia para o Google Drive
        ↓
Interface atualiza automaticamente
```

---

## 📦 Instalação

### 1. Clone o projeto

```bash
git clone https://github.com/bryanrafaelbueno/synca
cd synca
make setup
```

---

### 2. Configure o Google Drive

Você vai precisar:

- Criar um projeto no Google Cloud
- Ativar a Google Drive API
- Baixar o `credentials.json` para:

```
~/.config/synca/credentials.json
ou
C:\Users\seuusuario\.config\synca\credentials.json
```

---

### 3. Conectar conta

```bash
./bin/synca-daemon connect google-drive
```

👉 Vai abrir o navegador para login

---

### 4. Escolher pasta

```bash
./bin/synca-daemon watch ~/Documentos
```

---

### 5. Iniciar Synca

```bash
./bin/synca-daemon daemon
```

---

### 6. Abrir interface

```bash
make app-dev
```

---

## 🖥️ Interface

- Status da sincronização
- Lista de arquivos
- Progresso
- Estado da conexão

*(adicione screenshots depois)*

```
/assets/screenshot.png
```

---

## ⚙️ Configuração

Arquivo:

```
~/.config/synca/config.json
```

Exemplo:

```json
{
  "watch_paths": ["/home/user/Documentos"],
  "log_level": "info"
}
```

---

## ⚔️ Conflitos de arquivo

| Estratégia | Comportamento |
|-----------|--------------|
| KeepBoth | Cria cópia com timestamp |
| NewerWins | Mantém o mais recente |
| LocalWins | Mantém o local |
| RemoteWins | Mantém o remoto |

---

## 🛠️ CLI

```bash
synca daemon
synca connect google-drive
synca watch ~/pasta
synca status
```

---

## 🧱 Stack

- Backend: Go
- Watcher: fsnotify (inotify)
- Frontend: Tauri + React
- Estado: Zustand
- Comunicação: WebSocket

---

## 🗺️ Roadmap

- [x] Upload automático
- [x] Interface funcional
- [x] Conflitos
- [ ] Sync bidirecional completo
- [ ] System tray
- [ ] Multi-cloud (Rclone)

---

## 🤝 Contribuindo

```bash
git checkout -b feat/minha-feature
git commit -m "feat: minha feature"
git push
```

Pull requests são bem-vindos 🚀

---

## 📄 Licença

MIT © Synca Contributors

---

## 💡 Observação

Synca ainda está em fase MVP ,  bugs podem acontecer, mas já é totalmente funcional para uso real.
