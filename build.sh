#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

for command_name in go node npm; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    printf 'Error: %s is required.\n' "$command_name" >&2
    exit 1
  fi
done
NODE_RAW="$(node --version)"
NODE_VERSION="${NODE_RAW#v}"
if [[ ! "$NODE_VERSION" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+) ]]; then
  printf 'Error: Node.js >= 22.22.2 is required.\nInstalled: %s\n' "$NODE_VERSION" >&2
  exit 1
fi
NODE_MAJOR="${BASH_REMATCH[1]}" NODE_MINOR="${BASH_REMATCH[2]}" NODE_PATCH="${BASH_REMATCH[3]}"
if (( NODE_MAJOR < 22 || (NODE_MAJOR == 22 && NODE_MINOR < 22) || (NODE_MAJOR == 22 && NODE_MINOR == 22 && NODE_PATCH < 2) )); then
  printf 'Error: Node.js >= 22.22.2 is required.\nInstalled: %s\n' "$NODE_VERSION" >&2
  exit 1
fi

cd "$ROOT_DIR/frontend"
if [[ ! -d node_modules ]]; then
  npm ci
fi
npm run build
cd "$ROOT_DIR"
go test ./...
mkdir -p dist
go build -o dist/yomirelay ./cmd/yomirelay
printf 'Built: %s\n' "$ROOT_DIR/dist/yomirelay"
