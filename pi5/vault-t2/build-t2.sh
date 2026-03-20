#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

echo "Building vault-t2..."
go build -o vault-t2 ./cmd/vault-t2/

echo "Building vault-t2-fuse..."
go build -o vault-t2-fuse ./cmd/vault-t2-fuse/

echo "Building vault-t2-acl-update..."
go build -o vault-t2-acl-update ./cmd/vault-t2-acl-update/

echo "Creating symlinks..."
ln -sf vault-t2 t2-provision
ln -sf vault-t2 t2-get
ln -sf vault-t2 t2-set
ln -sf vault-t2 t2-list
ln -sf vault-t2 t2-delete

echo ""
echo "Built: vault-t2, vault-t2-fuse, vault-t2-acl-update, symlinks (t2-provision, t2-get, t2-set, t2-list, t2-delete)"
echo ""
echo "To install:"
echo "  sudo install -m 755 vault-t2 vault-t2-fuse /usr/local/bin/"
echo "  sudo install -m 755 -o root vault-t2-acl-update /usr/local/bin/"
echo "  sudo ln -sf vault-t2 /usr/local/bin/t2-provision"
echo "  sudo ln -sf vault-t2 /usr/local/bin/t2-get"
echo "  sudo ln -sf vault-t2 /usr/local/bin/t2-set"
echo "  sudo ln -sf vault-t2 /usr/local/bin/t2-list"
echo "  sudo ln -sf vault-t2 /usr/local/bin/t2-delete"
echo ""
echo "Config files needed (create manually):"
echo "  sudo mkdir -p /etc/vault-t2"
echo "  sudo cp acl.yaml.example /etc/vault-t2/acl.yaml   # then edit"
echo "  sudo chown root:root /etc/vault-t2/acl.yaml"
echo "  sudo chmod 644 /etc/vault-t2/acl.yaml"
