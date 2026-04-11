.PHONY: daemon daemon-windows daemon-run app-dev app-build \
        release-linux release-windows appimage-manual \
        dev build setup setup-creds clean

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

# ── AppImage Manual Build ──────────────────────────────────────
appimage-manual:
	@echo "Preparing AppDir for manual AppImage build..."
	mkdir -p desktop/src-tauri/target/release/bundle/appimage/Synca.AppDir/usr/bin

	# Ensure clean bin directory to avoid corrupted files
	rm -f desktop/src-tauri/target/release/bundle/appimage/Synca.AppDir/usr/bin/*

	# Copy binaries (main app and sidecar)
	cp desktop/src-tauri/target/release/synca desktop/src-tauri/target/release/bundle/appimage/Synca.AppDir/usr/bin/
	cp bin/synca-daemon-x86_64-unknown-linux-gnu desktop/src-tauri/target/release/bundle/appimage/Synca.AppDir/usr/bin/synca-daemon

	# Copy metadata (desktop file and icon) from the built DEB
	mkdir -p desktop/src-tauri/target/release/bundle/appimage/Synca.AppDir/usr/share/applications
	mkdir -p desktop/src-tauri/target/release/bundle/appimage/Synca.AppDir/usr/share/icons/hicolor/2048x2048/apps
	cp desktop/src-tauri/target/release/bundle/deb/*/data/usr/share/applications/Synca.desktop desktop/src-tauri/target/release/bundle/appimage/Synca.AppDir/usr/share/applications/
	cp desktop/src-tauri/target/release/bundle/deb/*/data/usr/share/icons/hicolor/2048x2048/apps/synca.png desktop/src-tauri/target/release/bundle/appimage/Synca.AppDir/usr/share/icons/hicolor/2048x2048/apps/

	@echo "Building AppImage manually with linuxdeploy..."

	cd desktop/src-tauri/target/release/bundle/appimage && \
	export NO_STRIP=1 && \
	$$HOME/tools/linuxdeploy/linuxdeploy-x86_64.AppImage \
		--appdir Synca.AppDir \
		--executable Synca.AppDir/usr/bin/synca \
		--output appimage

	@echo "AppImage built successfully!"

# ── Releases (Linux) ───────────────────────────────────────────
release-linux: daemon
	@echo "Building Linux release artifacts..."
	rm -rf desktop/src-tauri/target/release/bundle/deb/*
	rm -rf desktop/src-tauri/target/release/bundle/appimage/*

	# Build only DEB with Tauri (stable)
	cd desktop && \
	CARGO_HOME=$$(pwd)/.cargo-home \
	CARGO_TARGET_DIR=$$(pwd)/src-tauri/target \
	npm run tauri build -- --bundles deb

	# Build AppImage manually (reliable)
	$(MAKE) appimage-manual

	@echo "Exporting Linux releases to root..."
	mkdir -p releases/linux
	rm -rf releases/linux/*

	# Copy DEB
	cp -r desktop/src-tauri/target/release/bundle/deb/* releases/linux/ 2>/dev/null || true

	# Copy AppImage (manual build)
	cp -r desktop/src-tauri/target/release/bundle/appimage/*.AppImage releases/linux/ 2>/dev/null || true

	@echo "Release Linux artifacts exported to ./releases/linux/"

# ── Releases (Windows) ─────────────────────────────────────────
release-windows: daemon-windows
	@echo "Building Windows release artifacts..."
	rm -rf desktop/src-tauri/target/x86_64-pc-windows-gnu/release/bundle/nsis/*
	cd desktop && \
	CARGO_HOME=$$(pwd)/.cargo-home \
	CARGO_TARGET_DIR=$$(pwd)/src-tauri/target \
	npm run tauri build -- --target x86_64-pc-windows-gnu

	@echo "Exporting Windows releases to root..."
	mkdir -p releases/windows
	rm -rf releases/windows/*

	cp -r desktop/src-tauri/target/x86_64-pc-windows-gnu/release/bundle/nsis/*.exe releases/windows/ || true

	@echo "Release Windows artifacts exported to ./releases/windows/"

# ── Dev ────────────────────────────────────────────────────────
dev: daemon
	@echo "Cleaning zombie daemons..."
	-pkill -f synca-daemon || true

	@echo "Starting Tauri app..."
	cd desktop && npm run tauri dev

# ── Full build ─────────────────────────────────────────────────
build: daemon app-build
	@echo "Build complete."

# ── Setup ──────────────────────────────────────────────────────
setup:
	@echo "Installing frontend dependencies..."
	cd desktop && npm install

	@echo "Downloading Go modules..."
	cd daemon && go mod tidy

# ── Credentials helper ─────────────────────────────────────────
setup-creds:
	@echo "Creating config directory..."
	mkdir -p ~/.config/synca
	@echo ""
	@echo "Next steps:"
	@echo "  1. Go to https://console.cloud.google.com"
	@echo "  2. Create a project and enable Google Drive API"
	@echo "  3. Create OAuth 2.0 credentials (Desktop app)"
	@echo "  4. Download credentials.json → ~/.config/synca/credentials.json"
	@echo "  5. Run:"
	@echo "     make daemon && ./bin/synca-daemon-x86_64-unknown-linux-gnu connect google-drive"

# ── Clean ──────────────────────────────────────────────────────
clean:
	rm -rf bin/ releases/
	cd desktop && rm -rf node_modules dist src-tauri/target