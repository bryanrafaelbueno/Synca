.PHONY: daemon app dev build release-linux release-windows clean setup setup-creds

# ── Backend (Go daemon) ────────────────────────────────────────
daemon:
	@echo "Building synca daemon..."
	cd daemon && go build -o ../bin/synca-daemon-x86_64-unknown-linux-gnu ./cmd/synca

daemon-windows:
	@echo "Building synca daemon for Windows..."
	cd daemon && GOOS=windows GOARCH=amd64 go build -o ../bin/synca-daemon-x86_64-pc-windows-gnu.exe ./cmd/synca

daemon-run: daemon
	./bin/synca-daemon-x86_64-unknown-linux-gnu daemon

# ── Frontend (Tauri + React) ───────────────────────────────────
app-dev:
	@echo "Starting Tauri dev mode..."
	cd desktop && npm run tauri dev

app-build:
	@echo "Building Tauri app..."
	cd desktop && npm run tauri build

# ── Releases Exports ───────────────────────────────────────────
release-linux: daemon
	@echo "Building Linux release artifacts..."
	cd desktop && CARGO_HOME=$$(pwd)/.cargo-home CARGO_TARGET_DIR=$$(pwd)/src-tauri/target npm run tauri build -- --bundles deb,appimage
	@echo "Exporting Linux releases to root..."
	mkdir -p releases/linux
	cp -r desktop/src-tauri/target/release/bundle/deb/* releases/linux/ || true
	cp -r desktop/src-tauri/target/release/bundle/appimage/* releases/linux/ || true
	@echo "Release Linux artifacts exported to ./releases/linux/"

release-windows: daemon-windows
	@echo "Building Windows release artifacts..."
	cd desktop && CARGO_HOME=$$(pwd)/.cargo-home CARGO_TARGET_DIR=$$(pwd)/src-tauri/target npm run tauri build -- --target x86_64-pc-windows-gnu
	@echo "Exporting Windows releases to root..."
	mkdir -p releases/windows
	cp -r desktop/src-tauri/target/x86_64-pc-windows-gnu/release/bundle/nsis/*.exe releases/windows/ || true
	@echo "Release Windows artifacts exported to ./releases/windows/"

# ── Dev: run app ───────────────────────────────────────────────
dev: daemon
	@echo "Initial cleanup (killing zombie daemons if any)..."
	-pkill -f synca-daemon || true
	@echo "Starting Tauri app (Daemon is now started natively by Sidecar)..."
	cd desktop && npm run tauri dev

# ── Full build ─────────────────────────────────────────────────
build: daemon app-build
	@echo "Build complete. App bundle: desktop/src-tauri/target/release/bundle"

# ── Setup ──────────────────────────────────────────────────────
setup:
	@echo "Installing frontend dependencies..."
	cd desktop && npm install
	@echo "Downloading Go modules..."
	cd daemon && go mod tidy

# ── Credentials setup helper ───────────────────────────────────
setup-creds:
	@echo "Creating config directory..."
	mkdir -p ~/.config/synca
	@echo ""
	@echo "Next steps:"
	@echo "  1. Go to https://console.cloud.google.com"
	@echo "  2. Create a project and enable Google Drive API"
	@echo "  3. Create OAuth 2.0 credentials (Desktop app)"
	@echo "  4. Download credentials.json → ~/.config/synca/credentials.json"
	@echo "  5. Run: make daemon && ./bin/synca-daemon-x86_64-unknown-linux-gnu connect google-drive"

clean:
	rm -rf bin/ releases/
	cd desktop && rm -rf node_modules dist src-tauri/target
