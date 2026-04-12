#!/usr/bin/env bash
# ============================================================
# collect-webkit-deps.sh
# Coleta TODAS as dependências do WebKitGTK do sistema hospedeiro
# e as copia para o AppDir
# ============================================================
set -euo pipefail

APPDIR="$1"
if [ -z "$APPDIR" ]; then
    echo "Uso: $0 <AppDir path>"
    exit 1
fi

if [ ! -d "$APPDIR/usr" ]; then
    echo "Erro: $APPDIR/usr não existe. Execute após criar o AppDir."
    exit 1
fi

LIB_DIR="$APPDIR/usr/lib"
LIBEXEC_DIR="$APPDIR/usr/libexec"
mkdir -p "$LIB_DIR"
mkdir -p "$LIBEXEC_DIR/webkit2gtk-4.1"

# ============================================================
# 1. Identificar os binários auxiliares do WebKitGTK
# ============================================================
echo ">>> Procurando binários auxiliares do WebKitGTK..."

# Possíveis paths (varia por distro)
WEBKIT_BIN_PATHS=(
    "/usr/lib/x86_64-linux-gnu/webkit2gtk-4.1"
    "/usr/lib/webkit2gtk-4.1"
    "/usr/lib64/webkit2gtk-4.1"
    "/usr/libexec/webkit2gtk-4.1"
)

WEBKIT_SRC_DIR=""
for path in "${WEBKIT_BIN_PATHS[@]}"; do
    if [ -d "$path" ]; then
        WEBKIT_SRC_DIR="$path"
        echo "    Encontrado em: $path"
        break
    fi
done

if [ -z "$WEBKIT_SRC_DIR" ]; then
    echo "ERRO: Não foi possível encontrar os binários do WebKitGTK no sistema."
    echo "Instale: sudo apt install libwebkit2gtk-4.1-0 (Ubuntu/Debian)"
    echo "         sudo pacman -S webkit2gtk-4.1 (Arch)"
    exit 1
fi

# Copiar binários auxiliares
echo ">>> Copiando binários auxiliares do WebKitGTK..."
for bin in WebKitNetworkProcess WebKitWebProcess WebKitDatabaseProcess; do
    if [ -f "$WEBKIT_SRC_DIR/$bin" ]; then
        cp -v "$WEBKIT_SRC_DIR/$bin" "$LIBEXEC_DIR/webkit2gtk-4.1/"
        chmod +x "$LIBEXEC_DIR/webkit2gtk-4.1/$bin"
        echo "    Copiado: $bin"
    fi
done

# ============================================================
# 2. Coletar dependências recursivas do WebKitGTK
# ============================================================
echo ">>> Coletando dependências do WebKitGTK..."

# Função para coletar libs recursivamente
collect_lib_deps() {
    local lib_path="$1"
    local visited="$2"
    
    # Evitar loops
    if echo "$visited" | grep -q "$lib_path"; then
        return
    fi
    visited="$visited $lib_path"
    
    # Resolver symlinks
    local real_path
    real_path=$(readlink -f "$lib_path")
    
    # Copiar a lib
    if [ -f "$real_path" ]; then
        local basename
        basename=$(basename "$real_path")
        if [ ! -f "$LIB_DIR/$basename" ]; then
            cp -v "$real_path" "$LIB_DIR/"
            echo "    Lib: $basename"
        fi
        
        # Coletar dependências desta lib
        local deps
        deps=$(ldd "$real_path" 2>/dev/null | grep "=>" | awk '{print $3}' || true)
        for dep in $deps; do
            if [ -f "$dep" ]; then
                # Filtrar apenas libs que não são do sistema base
                case "$dep" in
                    */libc.so*|*/libm.so*|*/libpthread.so*|*/libdl.so*|*/librt.so*|*/libutil.so*|*/ld-linux*|*/libgcc_s.so*|*/libstdc++.so*|*/libz.so*|*/libgomp.so*)
                        # libs glibc/gcc de baixo nível - geralmente não empacotar
                        continue
                        ;;
                    *)
                        collect_lib_deps "$dep" "$visited"
                        ;;
                esac
            fi
        done
    fi
}

# Localizar a lib principal do WebKitGTK
WEBKIT_LIBS=(
    "libwebkit2gtk-4.1.so"
    "libjavascriptcoregtk-4.1.so"
)

for lib_name in "${WEBKIT_LIBS[@]}"; do
    lib_path=$(ldconfig -p 2>/dev/null | grep "$lib_name" | head -1 | awk '{print $NF}' || true)
    if [ -n "$lib_path" ] && [ -f "$lib_path" ]; then
        echo ">>> Coletando deps de: $lib_path"
        collect_lib_deps "$lib_path" ""
    fi
done

# ============================================================
# 3. Coletar GStreamer plugins (necessários para media)
# ============================================================
echo ">>> Coletando plugins GStreamer..."

GST_SRC_DIR=""
GST_POSSIBLE_PATHS=(
    "/usr/lib/x86_64-linux-gnu/gstreamer-1.0"
    "/usr/lib/gstreamer-1.0"
    "/usr/lib64/gstreamer-1.0"
)

for path in "${GST_POSSIBLE_PATHS[@]}"; do
    if [ -d "$path" ]; then
        GST_SRC_DIR="$path"
        break
    fi
done

if [ -n "$GST_SRC_DIR" ]; then
    mkdir -p "$LIB_DIR/gstreamer-1.0"
    # Copiar apenas plugins essenciais para manter o AppImage leve
    for plugin in libgstcoreelements.so libgstplayback.so libgstaudioresample.so libgstvolume.so; do
        if [ -f "$GST_SRC_DIR/$plugin" ]; then
            cp -v "$GST_SRC_DIR/$plugin" "$LIB_DIR/gstreamer-1.0/"
        fi
    done
fi

# ============================================================
# 4. Coletar schemas GIO/Glib
# ============================================================
echo ">>> Coletando schemas GIO..."

GIO_SCHEMAS_SRC=""
SCHEMA_PATHS=(
    "/usr/share/glib-2.0/schemas"
    "/usr/local/share/glib-2.0/schemas"
)

for path in "${SCHEMA_PATHS[@]}"; do
    if [ -d "$path" ]; then
        GIO_SCHEMAS_SRC="$path"
        break
    fi
done

if [ -n "$GIO_SCHEMAS_SRC" ]; then
    mkdir -p "$APPDIR/usr/share/glib-2.0/schemas"
    cp "$GIO_SCHEMAS_SRC"/*.xml "$APPDIR/usr/share/glib-2.0/schemas/" 2>/dev/null || true
    cp "$GIO_SCHEMAS_SRC"/gschemas.compiled "$APPDIR/usr/share/glib-2.0/schemas/" 2>/dev/null || true
fi

# ============================================================
# 5. Corrigir RPATH em todos os binários/libs empacotados
# ============================================================
echo ">>> Corrigindo RPATH nos binários empacotados..."

if command -v patchelf &> /dev/null; then
    # Corrigir binários auxiliares WebKit
    for bin in "$LIBEXEC_DIR"/webkit2gtk-4.1/*; do
        if [ -f "$bin" ] && [ -x "$bin" ]; then
            echo "    Patching RPATH: $(basename $bin)"
            patchelf --set-rpath '$ORIGIN/../../lib' "$bin" 2>/dev/null || true
        fi
    done
else
    echo "    AVISO: patchelf não encontrado. RPATH não foi corrigido."
    echo "    Instale: sudo apt install patchelf"
fi

echo ""
echo "============================================================"
echo " Coleta concluída!"
echo "============================================================"
echo "Libs em: $LIB_DIR"
echo "Binários WebKit em: $LIBEXEC_DIR/webkit2gtk-4.1"
echo ""
echo "Verifique com:"
echo "  ldd $LIBEXEC_DIR/webkit2gtk-4.1/WebKitNetworkProcess"
echo "  ldd $LIBEXEC_DIR/webkit2gtk-4.1/WebKitWebProcess"
