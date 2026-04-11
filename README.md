<p align="center">
  <img src="./assets/SyncaWhite.png" alt="Synca Logo" width="480"/>
</p>

<p align="center">
  <b>Google Drive file synchronization for Linux and Windows</b><br/>
  Simple, lightweight, and open source — a free alternative to Insync
</p>

<p align="center">
  <img src="https://img.shields.io/badge/status-MVP-blue"/>
  <img src="https://img.shields.io/badge/license-MIT-green"/>
  <img src="https://img.shields.io/badge/go-1.22+-00ADD8"/>
  <img src="https://img.shields.io/badge/tauri-2.x-FFC131"/>
  <img src="https://img.shields.io/badge/platform-linux%20%7C%20windows-lightgrey"/>
</p>

---

## ✨ What is Synca?

**Synca** is a sync client that automatically connects your local folders to Google Drive.

It runs silently in the background (daemon), detects file changes, and keeps everything synchronized in real time.

💡 Ideal for anyone who wants something lightweight, open source, and without relying on paid solutions.

---

## 🚀 Features

- 🔄 Automatic synchronization (near real-time)
- 📂 Recursive folder monitoring
- ⚡ Low memory usage (Go + Tauri)
- 🔐 Secure Google login (OAuth2)
- 🧠 Intelligent conflict resolution
- 📡 Real-time communication (WebSocket)
- 🖥️ Lightweight native interface
- 🪟 Windows and Linux support

---

## 🧠 How it works


```
You save a file
        ↓
Synca detects the change
        ↓
Processes in background
        ↓
Uploads to Google Drive
        ↓
Interface updates automatically
```

---

## 📁 Project Structure

```
synca/
├── assets/                  # Logos and visual resources
├── bin/                     # Compiled daemon binaries
├── daemon/                  # Go backend (sync daemon)
│   ├── cmd/synca/           # CLI entrypoint
│   └── internal/            # Internal logic (watcher, sync, API)
├── desktop/                 # Desktop app (Tauri + React)
│   ├── ui/                  # React frontend (components, hooks, store)
│   ├── src-tauri/           # Tauri Rust backend (this name is required)
│   ├── index.html           # Vite entrypoint
│   ├── vite.config.ts       # Vite configuration
│   └── package.json         # Frontend dependencies
├── Makefile                 # Build, dev, and release commands
└── README.md
```

---

## 📦 Installation

### 1. Clone the project

```bash
git clone https://github.com/bryanrafaelbueno/synca
cd synca
make setup
```

---

### 2. Configure Google Drive

You will need to:

- Create a project on Google Cloud
- Enable the Google Drive API
- Download `credentials.json` to:

```
~/.config/synca/credentials.json
or
C:\Users\yourusername\.config\synca\credentials.json
```

---

### 3. Connect account

```bash
./bin/synca-daemon connect google-drive
```

👉 This will open the browser for login

---

### 4. Choose folder

```bash
./bin/synca-daemon watch ~/Documents
```

---

### 5. Start Synca

```bash
./bin/synca-daemon daemon
```

---

### 6. Open interface

```bash
make app-dev
```

---

## 🖥️ Interface

- Sync status
- File list
- Progress
- Connection state

*(add screenshots later)*

```
/assets/screenshot.png
```

---

## ⚙️ Configuration

File:

```
~/.config/synca/config.json
```

Example:

```json
{
  "watch_paths": ["/home/user/Documents"],
  "log_level": "info"
}
```

---

## ⚔️ File Conflicts

| Strategy   | Behavior                     |
|------------|------------------------------|
| KeepBoth   | Creates copy with timestamp  |
| NewerWins  | Keeps the most recent        |
| LocalWins  | Keeps the local version      |
| RemoteWins | Keeps the remote version     |

---

## 🛠️ CLI

```bash
synca daemon
synca connect google-drive
synca watch ~/folder
synca status
```

---

## 🧱 Stack

- Backend: Go
- Watcher: fsnotify (inotify)
- Frontend: Tauri + React
- State: Zustand
- Communication: WebSocket

---

## 🗺️ Roadmap

- [x] Automatic upload
- [x] Functional interface
- [x] Conflicts
- [ ] Full bidirectional sync
- [ ] System tray
- [ ] Multi-cloud (Rclone)

---

## 🤝 Contributing

```bash
git checkout -b feat/my-feature
git commit -m "feat: my feature"
git push
```

Pull requests are welcome 🚀

---

## 📄 License

MIT © Synca Contributors

---

## 💡 Note

Synca is still in MVP phase — bugs may occur, but it is already fully functional for real-world use.
