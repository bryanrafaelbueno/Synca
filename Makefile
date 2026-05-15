.PHONY: daemon daemon-windows daemon-run app-dev app-build \
        release-linux release-windows appimage-manual \
        dev build setup clean check-deps check-libs \
        check-webkit collect-webkit-deps check-appimage

# ── OS Detection ──────────────────────────────────────────────
ifeq ($(OS),Windows_NT)
    IS_WINDOWS := 1
    DAEMON_BIN := ../bin/synca-daemon-x86_64-pc-windows-msvc.exe
    KILL_DAEMON := taskkill /F /IM "synca-daemon*" 2>nul || exit 0
else
    IS_WINDOWS := 0
    DAEMON_BIN := ../bin/synca-daemon-x86_64-unknown-linux-gnu
    KILL_DAEMON := pkill -f synca-daemon || true

    # Linux-specific configuration
    LINUXDEPLOY ?= $(shell which linuxdeploy-x86_64.AppImage 2>/dev/null || which linuxdeploy 2>/dev/null || echo "$$HOME/tools/linuxdeploy/linuxdeploy-x86_64.AppImage")

    # Dynamic library resolution (cross-distro)
    HARFBUZZ := $(shell ldconfig -p | grep libharfbuzz.so.0 | head -n1 | awk '{print $$4}')
    FONTCONFIG := $(shell ldconfig -p | grep libfontconfig.so.1 | head -n1 | awk '{print $$4}')
    FREETYPE := $(shell ldconfig -p | grep libfreetype.so.6 | head -n1 | awk '{print $$4}')
    EXPAT := $(shell ldconfig -p | grep libexpat.so.1 | head -n1 | awk '{print $$4}')

    # WebKitGTK detection (para empacotamento portátil)
    WEBKITGTK_SO := $(shell ldconfig -p | grep 'libwebkit2gtk-4.1.so' | head -n1 | awk '{print $$4}')
    WEBKITGTK_LIBEXEC := $(shell for p in /usr/lib/x86_64-linux-gnu/webkit2gtk-4.1 /usr/lib/webkit2gtk-4.1 /usr/lib64/webkit2gtk-4.1 /usr/libexec/webkit2gtk-4.1; do [ -d "$$p" ] && echo "$$p" && break; done)
endif

# Export Go variables for cross-platform compatibility
export CGO_ENABLED=0

# ── Backend (Go daemon) ────────────────────────────────────────
daemon:
	@echo "Building synca daemon..."
	@node -e "const fs = require('fs'); fs.existsSync('.env') ? fs.copyFileSync('.env', 'daemon/internal/auth/.env.embedded') : fs.writeFileSync('daemon/internal/auth/.env.embedded', '');"
	cd daemon && go build -o $(DAEMON_BIN) ./cmd/synca
	@node -e "require('fs').writeFileSync('daemon/internal/auth/.env.embedded', '');"

daemon-windows: export GOOS=windows
daemon-windows: export GOARCH=amd64
daemon-windows:
	@echo "Building synca daemon (Windows)..."
	@node -e "const fs = require('fs'); fs.existsSync('.env') ? fs.copyFileSync('.env', 'daemon/internal/auth/.env.embedded') : fs.writeFileSync('daemon/internal/auth/.env.embedded', '');"
	cd daemon && go build -o ../bin/synca-daemon-x86_64-pc-windows-gnu.exe ./cmd/synca
	@node -e "require('fs').writeFileSync('daemon/internal/auth/.env.embedded', '');"

daemon-run: daemon
	bin/$(notdir $(DAEMON_BIN)) daemon

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
	cp desktop/src-tauri/target/release/bundle/deb/*/data/usr/share/applications/Synca.desktop \
	   desktop/src-tauri/target/release/bundle/appimage/Synca.AppDir/usr/share/applications/

	cp -r desktop/src-tauri/target/release/bundle/deb/*/data/usr/share/icons \
	   desktop/src-tauri/target/release/bundle/appimage/Synca.AppDir/usr/share/

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

release-windows: daemon
	@echo "Cleaning old NSIS bundles..."
	@node -e "const fs = require('fs'); fs.rmSync('desktop/src-tauri/target/release/bundle/nsis', {recursive:true, force:true});"
	
	@echo "Building Tauri app..."
	cd desktop && npm run tauri build
	
	@echo "Copying to releases/windows..."
	@node -e "const fs = require('fs'), path = require('path'); const d = 'releases/windows'; fs.mkdirSync(d, {recursive:true}); const s = 'desktop/src-tauri/target/release/bundle/nsis'; if(fs.existsSync(s)) { fs.readdirSync(s).filter(f=>f.endsWith('.exe')).forEach(f=>fs.copyFileSync(path.join(s,f), path.join(d,f))) }"

# ── Dev ───────────────────────────────────────────────────────
dev: daemon
	-$(KILL_DAEMON)
	cd desktop && npm run tauri dev

# ── Build ─────────────────────────────────────────────────────
build: daemon app-build

# ── Setup ─────────────────────────────────────────────────────
setup: check-deps
	cd desktop && npm install
	cd daemon && go mod tidy

# ── Clean ─────────────────────────────────────────────────────
clean:
	@node -e "const fs = require('fs'); ['bin', 'releases', 'desktop/node_modules', 'desktop/dist', 'desktop/src-tauri/target'].forEach(p => { if (fs.existsSync(p)) fs.rmSync(p, { recursive: true, force: true }); });"

# ── Dependency Check ───────────────────────────────────────────
check-deps:
	@node -e "const { execSync } = require('child_process'); try { execSync('go version'); execSync('rustc --version'); execSync('npm --version'); console.log('✅ Core dependencies OK'); } catch (e) { console.error('❌ Missing dependency: ' + e.message); process.exit(1); }"
