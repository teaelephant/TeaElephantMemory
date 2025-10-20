#!/bin/bash
# Script to restore FDB dependencies from git history for backfill tool

set -e

COMMIT="c00e5bb"
ROOT_DIR="$(git rev-parse --show-toplevel)"

echo "Restoring FDB dependencies from commit $COMMIT..."

# Backup current go.mod and go.sum
echo "→ Backing up current go.mod and go.sum..."
cp "$ROOT_DIR/go.mod" "$ROOT_DIR/go.mod.pg-only.bak" 2>/dev/null || true
cp "$ROOT_DIR/go.sum" "$ROOT_DIR/go.sum.pg-only.bak" 2>/dev/null || true

# Create directories
mkdir -p "$ROOT_DIR/pkg/fdbclient"
mkdir -p "$ROOT_DIR/common/key_value/key_builder"
mkdir -p "$ROOT_DIR/common/key_value/encoder"

# Restore fdbclient
echo "→ Restoring pkg/fdbclient..."
git show "$COMMIT:pkg/fdbclient/client.go" > "$ROOT_DIR/pkg/fdbclient/client.go"

# Restore key_builder
echo "→ Restoring common/key_value/key_builder..."
git show "$COMMIT:common/key_value/key_builder/builder.go" > "$ROOT_DIR/common/key_value/key_builder/builder.go"
git show "$COMMIT:common/key_value/key_builder/keys.go" > "$ROOT_DIR/common/key_value/key_builder/keys.go"

# Restore encoder
echo "→ Restoring common/key_value/encoder..."
git show "$COMMIT:common/key_value/encoder/encoder.go" > "$ROOT_DIR/common/key_value/encoder/encoder.go"

# Restore go.mod and go.sum with FDB dependencies
echo "→ Restoring go.mod and go.sum with FDB dependencies..."
git show "$COMMIT:go.mod" > "$ROOT_DIR/go.mod"
git show "$COMMIT:go.sum" > "$ROOT_DIR/go.sum"

cd "$ROOT_DIR"
go mod download

echo "✓ FDB dependencies restored successfully!"
echo ""
echo "Backup files created:"
echo "  - go.mod.pg-only.bak"
echo "  - go.sum.pg-only.bak"
echo ""
echo "Next steps:"
echo "1. Ensure FoundationDB client libraries are installed on your system"
echo "2. Set PG_DSN environment variable"
echo "3. Run: go run cmd/backfill/main.go"
echo ""
echo "After backfill is complete, restore original go.mod:"
echo "  mv go.mod.pg-only.bak go.mod"
echo "  mv go.sum.pg-only.bak go.sum"
echo "  rm -rf pkg/fdbclient common/key_value"
echo "  go mod tidy"
