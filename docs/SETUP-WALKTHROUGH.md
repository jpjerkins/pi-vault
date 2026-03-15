# Pi5 Vault - Complete Setup Walkthrough

This guide walks you through setting up Pi5 Vault from scratch with your new YubiKeys.

## Prerequisites

**Windows Laptop:**
- YubiKey Manager installed
- PowerShell or Go compiler
- SSH client (built into Windows 10+)

**Raspberry Pi 5:**
- Go compiler installed
- SSH server running
- Access to `/mnt/data` directory

---

## Step 1: Install YubiKey Manager on Windows

1. Download YubiKey Manager from:
   https://www.yubico.com/support/download/yubikey-manager/

2. Install it (this gives you the `ykman` command)

3. Verify installation:
   ```powershell
   ykman --version
   ```

   Should show something like: `YubiKey Manager (ykman) version: 5.x.x`

---

## Step 2: Program Your Primary YubiKey

**Insert your first YubiKey into your laptop.**

### Option A: Auto-generate a Secret (Recommended)

This lets the YubiKey generate a random secret for you:

```powershell
ykman oath accounts add "Pi5 Vault" --oath-type TOTP --touch-required
```

You'll see:
- `Enter a secret key (base32):` - Just press Enter to auto-generate
- The YubiKey will blink - touch it to confirm

### Option B: Use Your Own Secret

If you want to use a specific secret (for programming multiple YubiKeys with the same credential):

```powershell
ykman oath accounts add "Pi5 Vault" --oath-type TOTP --touch-required
```

When prompted `Enter a secret key (base32):`, enter a base32-encoded secret.

**IMPORTANT: Save this secret somewhere secure if you want to program your backup YubiKey with the same credential!**

### Verify It Works

```powershell
ykman oath accounts code "Pi5 Vault"
```

Touch your YubiKey when it blinks. You should see a 6-digit code like:
```
Pi5 Vault    123456
```

---

## Step 3: Program Your Backup YubiKey (Optional but Recommended)

**Remove the first YubiKey, insert your second YubiKey.**

You have two options:

### Option A: Same Credential (Both Keys Work Identically)

Use the **same secret** you used for the first YubiKey:

```powershell
ykman oath accounts add "Pi5 Vault" --oath-type TOTP --touch-required
# Enter the SAME secret you used before
```

**Benefit:** Either YubiKey can decrypt your secrets interchangeably.

### Option B: Different Credential (Separate Keys)

Generate a new secret for the backup:

```powershell
ykman oath accounts add "Pi5 Vault Backup" --oath-type TOTP --touch-required
# Auto-generate a new secret
```

**Note:** If you do this, you'll need to modify the code to support multiple credentials. For simplicity, use Option A.

### Verify the Backup

```powershell
ykman oath accounts code "Pi5 Vault"
```

If you used the same secret (Option A), both YubiKeys should generate **the same code** at the same time window.

---

## Step 4: Build the Vault Binary on Pi5

**SSH into your Pi5:**

```bash
ssh pi5.local
```

**Navigate to the project and build:**

```bash
cd /path/to/pi5-vault/pi5
./build.sh
```

You should see output like:
```
Building vault binary...
Creating symlinks...
Done! Install with: sudo install -m 755 vault* /usr/local/bin/
```

**Install the binaries:**

```bash
sudo install -m 755 vault* /usr/local/bin/
```

**Create the secrets directory:**

```bash
sudo mkdir -p /mnt/data/secrets
sudo chown $USER:$USER /mnt/data/secrets
chmod 700 /mnt/data/secrets
```

**Verify installation:**

```bash
vault-list
```

Should complete without errors (empty output is normal - no secrets yet).

---

## Step 5: Set Up Windows Auth Proxy

You have two options. Choose ONE:

### Option A: PowerShell Version (Easier, No Build Required)

**On your Windows laptop, in PowerShell:**

```powershell
cd "C:\Local-only PARA\1 Projects\pi5-vault\windows\powershell"
.\vault-auth-proxy.ps1
```

You should see:
```
═══════════════════════════════════════════════════════
  Pi5 Vault Authentication Proxy
═══════════════════════════════════════════════════════

✓ Running on http://localhost:3000
  YubiKey ready for authentication
  Press Ctrl+C to stop
```

**Keep this window open!**

### Option B: Go Version (Cross-Platform)

**Build the Go auth proxy:**

```powershell
cd "C:\Local-only PARA\1 Projects\pi5-vault\windows\go"
.\build.ps1
```

**Run it:**

```powershell
.\vault-auth-proxy.exe
```

Same banner should appear. **Keep this running!**

---

## Step 6: Set Up SSH Tunnel from Pi5

The Pi5 needs to reach your laptop's auth proxy (running on `localhost:3000`).

**In a NEW terminal/SSH session on your Pi5:**

```bash
# Replace YOUR_LAPTOP_IP with your Windows laptop's IP address
# Or use hostname if DNS works
ssh -R 3000:localhost:3000 YOUR_LAPTOP_USERNAME@YOUR_LAPTOP_IP
```

Example:
```bash
ssh -R 3000:localhost:3000 philj@192.168.1.100
```

**What this does:**
- Creates a reverse tunnel
- Pi5's `localhost:3000` → Your laptop's `localhost:3000`
- The vault can now talk to the auth proxy

**Keep this SSH session alive** while using the vault!

---

## Step 7: Test the Complete System

**In your Pi5 SSH session (the first one, not the tunnel):**

### Test 1: Store a Secret

```bash
echo "my_test_password_123" | vault-set test_secret
```

**What happens:**
1. Pi5 vault requests session key from auth proxy (via SSH tunnel)
2. Your Windows laptop shows: `🔐 Touch YubiKey to derive session key...`
3. **Touch your YubiKey when it blinks**
4. Laptop derives session key and sends it back
5. Pi5 encrypts and stores the secret

You should see:
```
Requesting session key from laptop YubiKey...
✓ Session key cached (valid for 30 minutes)
✓ Secret encrypted and stored
```

### Test 2: Retrieve the Secret

```bash
vault-get test_secret
```

**What happens:**
- Uses cached session key (no YubiKey touch needed!)
- Decrypts and returns the secret

Output:
```
my_test_password_123
```

### Test 3: List Secrets

```bash
vault-list
```

Output:
```
test_secret
```

### Test 4: Delete the Test Secret

```bash
vault-delete test_secret
```

Output:
```
✓ Secret deleted
```

### Test 5: Session Key Caching

Try setting another secret immediately:

```bash
echo "another_secret" | vault-set cached_test
```

This time you should **NOT** be prompted to touch your YubiKey - it uses the cached session key from the first touch (valid for 30 minutes).

---

## Step 8: Check the Audit Log

**On Pi5:**

```bash
cat /mnt/data/secrets/.audit.log
```

You should see JSON entries for all operations:

```json
{"timestamp":"2026-03-14T10:30:00Z","action":"set","secret":"test_secret","success":true}
{"timestamp":"2026-03-14T10:30:15Z","action":"get","secret":"test_secret","success":true}
{"timestamp":"2026-03-14T10:30:30Z","action":"delete","secret":"test_secret","success":true}
```

---

## Troubleshooting

### "YubiKey error" on Windows

**Check YubiKey is inserted and ykman works:**

```powershell
ykman oath accounts list
```

Should show: `Pi5 Vault`

**Test TOTP generation:**

```powershell
ykman oath accounts code "Pi5 Vault"
```

Touch when prompted.

### "cannot reach auth proxy" on Pi5

**Check SSH tunnel is running:**

```bash
# On Pi5
curl http://localhost:3000/health
```

Should return:
```json
{"status":"running","timestamp":"..."}
```

If not, restart the SSH tunnel (Step 6).

### "decryption failed (wrong key or corrupted data)"

This means the session key used to encrypt is different from the key used to decrypt.

**Common causes:**
1. Time drift between Windows laptop and Pi5
   - **Fix:** Sync time on both machines
2. YubiKey TOTP secret changed
   - **Fix:** Re-program YubiKey or delete and re-create secrets
3. Backup YubiKey has different secret
   - **Fix:** Use the same YubiKey or program with same secret

### Auth proxy won't start on Windows

**"Could not start listener on port 3000"**

Port 3000 is already in use.

**Fix:**
```powershell
# Find what's using port 3000
netstat -ano | findstr :3000

# Kill that process or use a different port
```

To use a different port, edit the auth proxy and vault.go to use (e.g.) port 3001.

---

## Advanced: Persistent SSH Tunnel (AutoSSH)

Instead of manually maintaining the SSH tunnel, use `autossh`:

**On Pi5:**

```bash
sudo apt install autossh
```

**Create systemd service:**

```bash
sudo nano /etc/systemd/system/vault-tunnel.service
```

Content:
```ini
[Unit]
Description=Pi5 Vault SSH Tunnel to Laptop
After=network.target

[Service]
Type=simple
User=YOUR_USERNAME
ExecStart=/usr/bin/autossh -M 0 -N -R 3000:localhost:3000 YOUR_LAPTOP_USER@YOUR_LAPTOP_IP -o ServerAliveInterval=30 -o ServerAliveCountMax=3
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

**Enable and start:**

```bash
sudo systemctl daemon-reload
sudo systemctl enable vault-tunnel
sudo systemctl start vault-tunnel
```

**Check status:**

```bash
sudo systemctl status vault-tunnel
```

Now the tunnel auto-reconnects if it drops!

---

## Security Best Practices

1. **Store your TOTP secret securely** (if you want to program multiple YubiKeys)
2. **Never commit secrets** to git (they're encrypted, but still - don't)
3. **Protect your backup YubiKey** in a separate location
4. **Review audit logs** periodically: `cat /mnt/data/secrets/.audit.log`
5. **Use SSH key authentication** for the tunnel (not passwords)
6. **Keep session keys cached** only as long as needed (default 30min is reasonable)

---

## What's Next?

Now that the core system is working, you can:

1. **Integrate with apps** - Apps on Pi5 can call `vault-get` to retrieve secrets
2. **Add more secrets** - Use `vault-set` for DB passwords, API keys, etc.
3. **Set up the HTTP API** (future) - For web apps
4. **Build the recovery system** (future) - Passphrase-based backup
5. **Create a web UI** (future) - Manage secrets from browser

---

## Quick Reference Commands

**Windows (Auth Proxy):**
```powershell
# PowerShell version
.\vault-auth-proxy.ps1

# Go version
.\vault-auth-proxy.exe
```

**Pi5 (Vault Operations):**
```bash
# Set secret (from stdin)
echo "value" | vault-set secret_name

# Set secret (from argument)
vault-set secret_name "value"

# Get secret
vault-get secret_name

# List all secrets
vault-list

# Delete secret
vault-delete secret_name

# View audit log
cat /mnt/data/secrets/.audit.log
```

**SSH Tunnel:**
```bash
# Manual (keep running)
ssh -R 3000:localhost:3000 user@laptop

# Or use autossh (auto-reconnect)
autossh -M 0 -N -R 3000:localhost:3000 user@laptop -o ServerAliveInterval=30
```

---

## Need Help?

- Check audit log: `/mnt/data/secrets/.audit.log`
- Check session cache: `ls -la /mnt/data/secrets/.session_*`
- Test auth proxy: `curl http://localhost:3000/health`
- Verify YubiKey: `ykman oath accounts code "Pi5 Vault"`

Good luck! 🔐
