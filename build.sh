#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

mkdir -p cmd/server/web
cp web/admin.html cmd/server/web/
cp web/reader.html cmd/server/web/

echo "➡️ go build ./cmd/server"
go build -o storybook ./cmd/server

if [ ! -f storybook ]; then
  echo "❌ build failed"
  exit 1
fi

echo "✅ build ok: ./storybook"
