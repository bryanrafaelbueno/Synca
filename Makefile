.PHONY: daemon daemon-windows daemon-run app-dev app-build \
        release-linux release-windows appimage-manual \
        dev build setup clean check-deps check-libs \
        check-webkit collect-webkit-deps check-appimage

# ── Configuration ─────────────────────────────────────────────
LINUXDEPLOY ?= $(shell which linuxdeploy-x86_64.AppImage 2>/dev/null || which linuxdeploy 2>/dev/null || echo "$$HOME/tools/linuxdeploy/linuxdeploy-x86_64.AppImage")

# Dynamic library resolution (cross-distro)
HARFBUZZ := $(shell ldconfig -p | grep libharfbuzz.so.0 | head -n1 | awk '{print $$4}')
FONTCONFIG := $(shell ldconfig -p | grep libfontconfig.so.1 | head -n1 | awk '{print $$4}')
FREETYPE := $(shell ldconfig -p | grep libfreetype.so.6 | head -n1 | awk '{print $$4}')
EXPAT := $(shell ldconfig -p | grep libexpat.so.1 | head -n1 | awk '{print $$4}')

# WebKitGTK detection (para empacotamento portátil)
WEBKITGTK_SO := $(shell ldconfig -p | grep 'libwebkit2gtk-4.1.so' | head -n1 | awk '{print $$4}')
WEBKITGTK_LIBEXEC := $(shell for p in /usr/lib/x86_64-linux-gnu/webkit2gtk-4.1 /usr/lib/webkit2gtk-4.1 /usr/lib64/webkit2gtk-4.1 /usr/libexec/webkit2gtk-4.1; do [ -d "$$p" ] && echo "$$p" && break; done)

# ── Backend (Go daemon) ────────────────────────────────────────
daemon:
	@echo "Building synca daemon..."
	cp .env daemon/internal/auth/.env.embedded || touch daemon/internal/auth/.env.embedded
	cd daemon && CGO_ENABLED=0 go build -o ../bin/synca-daemon-x86_64-unknown-linux-gnu ./cmd/synca
	rm -f daemon/internal/auth/.env.embedded && touch daemon/internal/auth/.env.embedded

daemon-windows:
	@echo "Building synca daemon (Windows)..."
	cp .env daemon/internal/auth/.env.embedded || touch daemon/internal/auth/.env.embedded
	cd daemon && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o ../bin/synca-daemon-x86_64-pc-windows-gnu.exe ./cmd/synca
	rm -f daemon/internal/auth/.env.embedded && touch daemon/internal/auth/.env.embedded

daemon-run: daemon
	./bin/synca-daemon-x86_64-unknown-linux-gnu daemon

# ── Frontend ──────────────────────────────────────────────────
app-dev:
	cd desktop && npm run tauri dev

app-build:
	cd desktop && npm run tauri build

# ── Validate libs ─────────────────────────────────────────────
check-libs:
	@echo "Checking required shared libraries..."
	@test -f "$(HARFBUZZ)" || (echo "❌ libharfbuzz not found"; exit 1)
	@test -f "$(FONTCONFIG)" || (echo "❌ libfontconfig not found"; exit 1)
	@test -f "$(FREETYPE)" || (echo "❌ libfreetype not found"; exit 1)
	@test -f "$(EXPAT)" || (echo "❌ libexpat not found"; exit 1)
	@echo "✅ All required libs found"

# ── Validate WebKitGTK libs ───────────────────────────────────
check-webkit:
	@echo "Checking WebKitGTK dependencies..."
	@test -n "$(WEBKITGTK_SO)" || (echo "❌ libwebkit2gtk-4.1 not found. Install libwebkit2gtk-4.1-dev"; exit 1)
	@test -n "$(WEBKITGTK_LIBEXEC)" || (echo "❌ WebKitGTK libexec dir not found"; exit 1)
	@test -f "$(WEBKITGTK_LIBEXEC)/WebKitNetworkProcess" || (echo "❌ WebKitNetworkProcess not found"; exit 1)
	@test -f "$(WEBKITGTK_LIBEXEC)/WebKitWebProcess" || (echo "❌ WebKitWebProcess not found"; exit 1)
	@echo "✅ WebKitGTK components found"

# ── Collect WebKitGTK deps into AppDir ────────────────────────
collect-webkit-deps: check-webkit
	@echo "Collecting WebKitGTK dependencies into AppDir..."
	@bash packaging/collect-webkit-deps.sh "desktop/src-tauri/target/release/bundle/appimage/Synca.AppDir"
	@echo "Patching hardcoded paths in libwebkit2gtk-4.1.so.0..."
	@python3 packaging/patch-webkit-lib.py "desktop/src-tauri/target/release/bundle/appimage/Synca.AppDir/usr/lib/libwebkit2gtk-4.1.so.0"

# ── AppImage Manual Build ─────────────────────────────────────
appimage-manual: check-libs
	@echo "Preparing AppDir..."

	mkdir -p desktop/src-tauri/target/release/bundle/appimage/Synca.AppDir/usr/bin

	# Clean bin
	rm -f desktop/src-tauri/target/release/bundle/appimage/Synca.AppDir/usr/bin/*

	# Copy binaries
	cp desktop/src-tauri/target/release/synca \
	   desktop/src-tauri/target/release/bundle/appimage/Synca.AppDir/usr/bin/

	cp bin/synca-daemon-x86_64-unknown-linux-gnu \
	   desktop/src-tauri/target/release/bundle/appimage/Synca.AppDir/usr/bin/synca-daemon

	# Metadata
	mkdir -p desktop/src-tauri/target/release/bundle/appimage/Synca.AppDir/usr/share/applications
	mkdir -p desktop/src-tauri/target/release/bundle/appimage/Synca.AppDir/usr/share/icons/hicolor/2048x2048/apps

	cp desktop/src-tauri/target/release/bundle/deb/*/data/usr/share/applications/Synca.desktop \
	   desktop/src-tauri/target/release/bundle/appimage/Synca.AppDir/usr/share/applications/

	cp desktop/src-tauri/target/release/bundle/deb/*/data/usr/share/icons/hicolor/2048x2048/apps/synca.png \
	   desktop/src-tauri/target/release/bundle/appimage/Synca.AppDir/usr/share/icons/hicolor/2048x2048/apps/

	@echo "Collecting WebKitGTK dependencies..."
	$(MAKE) collect-webkit-deps

	@echo "Installing custom AppRun..."
	cp packaging/AppRun desktop/src-tauri/target/release/bundle/appimage/Synca.AppDir/AppRun
	chmod +x desktop/src-tauri/target/release/bundle/appimage/Synca.AppDir/AppRun

	@echo "Building AppImage..."

	cd desktop/src-tauri/target/release/bundle/appimage && \
	export NO_STRIP=1 && \
	$(LINUXDEPLOY) \
		--appdir Synca.AppDir \
		--executable Synca.AppDir/usr/bin/synca \
		--executable Synca.AppDir/usr/bin/synca-daemon \
		--library $(HARFBUZZ) \
		--library $(FONTCONFIG) \
		--library $(FREETYPE) \
		--library $(EXPAT) \
		--output appimage

	@echo "✅ AppImage built!"

# ── Verify AppImage ───────────────────────────────────────────
check-appimage:
	@echo "Checking AppImage..."
	@test -f releases/linux/Synca-x86_64.AppImage || (echo "❌ AppImage not found"; exit 1)

	rm -rf squashfs-root
	releases/linux/Synca-x86_64.AppImage --appimage-extract > /dev/null

	@echo "--- ldd check (synca) ---"
	@LD_LIBRARY_PATH=squashfs-root/usr/lib ldd squashfs-root/usr/bin/synca | grep "not found" || echo "✅ OK"

	@echo "--- ldd check (WebKitNetworkProcess) ---"
	@if [ -f squashfs-root/usr/libexec/webkit2gtk-4.1/WebKitNetworkProcess ]; then \
		LD_LIBRARY_PATH=squashfs-root/usr/lib ldd squashfs-root/usr/libexec/webkit2gtk-4.1/WebKitNetworkProcess | grep "not found" || echo "✅ OK"; \
	elif [ -f squashfs-root/usr/bin/webkit2gtk-4.1/WebKitNetworkProcess ]; then \
		LD_LIBRARY_PATH=squashfs-root/usr/lib ldd squashfs-root/usr/bin/webkit2gtk-4.1/WebKitNetworkProcess | grep "not found" || echo "✅ OK"; \
	else \
		echo "⚠️  WebKitNetworkProcess não encontrado no AppImage!"; \
	fi

	@echo "--- ldd check (WebKitWebProcess) ---"
	@if [ -f squashfs-root/usr/libexec/webkit2gtk-4.1/WebKitWebProcess ]; then \
		LD_LIBRARY_PATH=squashfs-root/usr/lib ldd squashfs-root/usr/libexec/webkit2gtk-4.1/WebKitWebProcess | grep "not found" || echo "✅ OK"; \
	elif [ -f squashfs-root/usr/bin/webkit2gtk-4.1/WebKitWebProcess ]; then \
		LD_LIBRARY_PATH=squashfs-root/usr/lib ldd squashfs-root/usr/bin/webkit2gtk-4.1/WebKitWebProcess | grep "not found" || echo "✅ OK"; \
	else \
		echo "⚠️  WebKitWebProcess não encontrado no AppImage!"; \
	fi

	@echo "--- strings check (paths hardcoded) ---"
	@strings squashfs-root/usr/libexec/webkit2gtk-4.1/WebKitNetworkProcess 2>/dev/null | grep -E '/usr/lib/' | grep -v 'usr/lib' | head -5 || echo "✅ No hardcoded Ubuntu paths"

	rm -rf squashfs-root

# ── Releases ──────────────────────────────────────────────────
release-linux: daemon
	@echo "Building Linux release..."

	rm -rf desktop/src-tauri/target/release/bundle/deb/*
	rm -rf desktop/src-tauri/target/release/bundle/appimage/*

	cd desktop && \
	CARGO_HOME=$$(pwd)/.cargo-home \
	CARGO_TARGET_DIR=$$(pwd)/src-tauri/target \
	npm run tauri build -- --bundles deb

	$(MAKE) appimage-manual

	mkdir -p releases/linux
	rm -rf releases/linux/*

	cp -r desktop/src-tauri/target/release/bundle/deb/* releases/linux/ 2>/dev/null || true
	cp -r desktop/src-tauri/target/release/bundle/appimage/*.AppImage releases/linux/ 2>/dev/null || true

	@echo "✅ Linux release ready"

release-windows: daemon-windows
	cd desktop && \
	CARGO_HOME=$$(pwd)/.cargo-home \
	CARGO_TARGET_DIR=$$(pwd)/src-tauri/target \
	npm run tauri build -- --target x86_64-pc-windows-gnu

	mkdir -p releases/windows
	cp desktop/src-tauri/target/x86_64-pc-windows-gnu/release/bundle/nsis/*.exe releases/windows/ || true

# ── Dev ───────────────────────────────────────────────────────
dev: daemon
	-pkill -f synca-daemon || true
	cd desktop && npm run tauri dev

# ── Build ─────────────────────────────────────────────────────
build: daemon app-build

# ── Setup ─────────────────────────────────────────────────────
setup: check-deps
	cd desktop && npm install
	cd daemon && go mod tidy

# ── Clean ─────────────────────────────────────────────────────
clean:
	rm -rf bin/ releases/
	cd desktop && rm -rf node_modules dist src-tauri/target

# ── Dependency Check ───────────────────────────────────────────
check-deps:
	@go version >/dev/null 2>&1 || (echo "❌ Go missing"; exit 1)
	@rustc --version >/dev/null 2>&1 || (echo "❌ Rust missing"; exit 1)
	@npm --version >/dev/null 2>&1 || (echo "❌ npm missing"; exit 1)
	@echo "✅ Core dependencies OK"
