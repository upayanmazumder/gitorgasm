#!/usr/bin/env bash
# Fails if a Secret value in deploy/argocd is not SOPS-encrypted.
set -euo pipefail
cd "$(dirname "$0")"
status=0
for file in *.sops.yaml; do
  if ! grep -q '^sops:' "$file"; then
    echo "$file: no sops metadata" >&2
    status=1
  fi
  awk -v file="$file" '
    /^---/ { block = 0 }
    /^(data|stringData):/ { block = 1; next }
    /^[^ ]/ { block = 0 }
    block && /^ +[^ #]/ && $0 !~ /ENC\[/ { print file ":" NR ": value is not encrypted"; bad = 1 }
    END { exit bad }
  ' "$file" >&2 || status=1
done
exit "$status"
