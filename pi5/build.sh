#!/bin/bash
# Build script for pi5 vault binary

set -e

echo "Building vault binary for pi5..."

# Build the binary
go build -o vault vault.go

# Create symlinks for different commands
ln -sf vault vault-get
ln -sf vault vault-set
ln -sf vault vault-list
ln -sf vault vault-delete

echo "✓ Build complete!"
echo ""
echo "Binaries created:"
ls -lh vault vault-get vault-set vault-list vault-delete
echo ""
echo "To install system-wide:"
echo "  sudo install -m 755 vault* /usr/local/bin/"
echo ""
echo "To test locally:"
echo "  ./vault-get <secret-name>"
