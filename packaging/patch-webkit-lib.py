#!/usr/bin/env python3
"""
patch-webkit-lib.py

Substitui o path hardcoded /usr/lib/x86_64-linux-gnu/webkit2gtk-4.1
na libwebkit2gtk-4.1.so.0 por /tmp/.synca-webkit-N (que o AppRun
cria como symlink para os binários empacotados).

Uso: python3 patch-webkit-lib.py <caminho-da-lib>
"""

import sys
import os

OLD_PATH = b"/usr/lib/x86_64-linux-gnu/webkit2gtk-4.1"
NEW_PATH = b"/tmp/.synca-webkit-N"

# Pad com null bytes para manter tamanho idêntico
NEW_PATH_PADDED = NEW_PATH + b"\x00" * (len(OLD_PATH) - len(NEW_PATH))

def main():
    if len(sys.argv) < 2:
        print(f"Uso: {sys.argv[0]} <caminho-da-libwebkit2gtk-4.1.so.0>")
        sys.exit(1)

    lib_path = sys.argv[1]

    if not os.path.isfile(lib_path):
        print(f"ERRO: arquivo não encontrado: {lib_path}")
        sys.exit(1)

    # Ler arquivo binário
    with open(lib_path, "rb") as f:
        data = f.read()

    # Contar ocorrências
    count = data.count(OLD_PATH)
    if count == 0:
        print("  ⚠️  Nenhuma ocorrência do path original encontrada (já foi patcheada?)")
        return

    print(f"  Encontradas {count} ocorrência(s) do path hardcoded")

    # Substituir
    data = data.replace(OLD_PATH, NEW_PATH_PADDED)

    # Escrever de volta
    with open(lib_path, "wb") as f:
        f.write(data)

    # Verificar
    with open(lib_path, "rb") as f:
        verify_data = f.read()

    remaining = verify_data.count(OLD_PATH)
    if remaining == 0:
        print(f"  ✅ Patch aplicado! {count} ocorrência(s) substituídas por /tmp/.synca-webkit-N")
    else:
        print(f"  ⚠️  Ainda restam {remaining} ocorrência(s)")
        sys.exit(1)

if __name__ == "__main__":
    main()
