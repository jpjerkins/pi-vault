# vault-t2 Installation Guide

Complete first-time setup for the Tier 2 hardware-sealed vault on pi5.local.

---

## Prerequisites

- Tier 1 vault operational (`vault-get`, `vault-set` working with YubiKey)
- `/etc/fuse.conf` contains `user_allow_other` (required for Docker access)
- `/mnt/data` mounted and writable by `philj`

Check fuse.conf:
```bash
grep user_allow_other /etc/fuse.conf || echo "MISSING — add 'user_allow_other' to /etc/fuse.conf"
```

---

## Step 1 — Build and install binaries

```bash
cd ~/dev/pi-vault/pi5/vault-t2
./build-t2.sh

# vault-t2 and vault-t2-fuse run as philj
sudo install -m 755 vault-t2 vault-t2-fuse /usr/local/bin/

# vault-t2-acl-update must be root-owned (run via sudo)
sudo install -m 755 -o root vault-t2-acl-update /usr/local/bin/

# Symlinks for vault-t2 subcommands
sudo ln -sf vault-t2 /usr/local/bin/t2-provision
sudo ln -sf vault-t2 /usr/local/bin/t2-get
sudo ln -sf vault-t2 /usr/local/bin/t2-set
sudo ln -sf vault-t2 /usr/local/bin/t2-list
sudo ln -sf vault-t2 /usr/local/bin/t2-delete
```

---

## Step 2 — Create the data directory

```bash
sudo mkdir -p /mnt/data/vault-t2
sudo chown philj:philj /mnt/data/vault-t2
chmod 700 /mnt/data/vault-t2
```

---

## Step 3 — Provision the tier-2 seed

This generates a random 32-byte seed, seals it against the Pi's hardware
fingerprint, and prints the seed for safe storage in Tier 1.

```bash
t2-provision --generate
# Output: "Store this seed in Tier 1: <base64_seed>"
```

Store the seed in Tier 1 immediately (YubiKey tap required):
```bash
vault-set tier2_seed <base64_seed>
```

---

## Step 4 — Store secrets

```bash
echo "my_postgres_password" | t2-set db_password
echo "sk-abc123..."         | t2-set api_key_openai

t2-list   # verify
```

---

## Step 5 — Configure the ACL

Create `/etc/vault-t2/acl.yaml` listing which UIDs may read each secret.
Use UIDs from the reserved range **50000–50099** (one per service).

```bash
sudo mkdir -p /etc/vault-t2

# Generate from registry.yaml if vault_uid/vault_secrets are declared:
dcm secrets acl-generate | sudo vault-t2-acl-update

# Or write manually:
sudo tee /etc/vault-t2/acl.yaml <<'EOF'
db_password:
  - 50001   # postgres service
api_key_openai:
  - 50002   # myapp service
EOF
sudo chown root:root /etc/vault-t2/acl.yaml
sudo chmod 644 /etc/vault-t2/acl.yaml
```

See `acl.yaml.example` for format documentation.

---

## Step 6 — Configure envfiles (optional)

Only needed for containers that require environment variables rather than
secret files (i.e. the `env_file:` docker-compose pattern).

```bash
sudo cp ~/dev/pi-vault/pi5/vault-t2/envfiles.yaml.example /etc/vault-t2/envfiles.yaml
sudo $EDITOR /etc/vault-t2/envfiles.yaml   # edit to match your services
sudo chown root:root /etc/vault-t2/envfiles.yaml
sudo chmod 644 /etc/vault-t2/envfiles.yaml
```

See `envfiles.yaml.example` for format documentation.

---

## Step 7 — Install tmpfiles and systemd unit

The tmpfiles rule recreates `/run/vault-t2-fs` after every reboot (since
`/run` is a tmpfs that is wiped on boot).

```bash
sudo cp ~/dev/pi-vault/pi5/vault-t2/vault-t2.tmpfiles.conf /etc/tmpfiles.d/vault-t2.conf
sudo systemd-tmpfiles --create /etc/tmpfiles.d/vault-t2.conf

sudo cp ~/dev/pi-vault/pi5/vault-t2/vault-t2-fuse.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now vault-t2-fuse
```

---

## Step 8 — Verify

```bash
# Service is running
systemctl status vault-t2-fuse

# Mount is up and lists secrets
ls /run/vault-t2-fs/

# Read a secret directly (bypasses FUSE, uses CLI)
t2-get db_password

# Read a secret through the FUSE mount as the correct UID
sudo -u '#50001' cat /run/vault-t2-fs/db_password

# Read should be denied for philj (not in ACL)
cat /run/vault-t2-fs/db_password   # expect: Permission denied

# Check audit log
tail -5 /mnt/data/vault-t2/.audit.log
```

---

## Updating secrets

```bash
# Add or update a secret (CLI — bypasses FUSE, no UID check)
echo "new_value" | t2-set db_password

# The FUSE mount serves the new value immediately on next read
# (FOPEN_DIRECT_IO — no kernel caching)
```

---

## Updating the ACL

```bash
# Edit registry.yaml vault_uid/vault_secrets, then:
dcm secrets acl-generate | sudo vault-t2-acl-update
sudo systemctl restart vault-t2-fuse
```

---

## Recovery after OS reinstall

```bash
# 1. Reinstall OS, rebuild and install binaries (Steps 1–2 above)

# 2. Retrieve seed from Tier 1 (YubiKey tap required)
vault-get tier2_seed

# 3. Re-seal against the new hardware fingerprint
echo "<base64_seed>" | t2-provision

# 4. Existing .enc files in /mnt/data/vault-t2/ are immediately readable
t2-list
```

---

## Docker compose integration

### Secret file pattern (`_FILE` env vars)

```yaml
services:
  postgres:
    image: postgres:16
    user: "50001:50001"
    volumes:
      - /run/vault-t2-fs/db_password:/run/secrets/db_password:ro
    environment:
      POSTGRES_PASSWORD_FILE: /run/secrets/db_password
```

### Envfile pattern (environment variables)

```yaml
services:
  myapp:
    image: myapp:latest
    user: "50002:50002"
    env_file: /run/vault-t2-fs/envfiles/myapp
```

Both patterns require the service's container UID to be listed in the ACL
(secret file pattern) or `envfiles.yaml` (envfile pattern).
