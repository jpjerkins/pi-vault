# Pi5 Vault Setup Checklist

Print this out or keep it open while you set up!

## Pre-Setup

- [ ] YubiKey Manager installed on Windows
- [ ] Both YubiKeys available
- [ ] SSH access to Pi5 working
- [ ] Go compiler on Pi5

---

## YubiKey Programming

### Primary YubiKey

- [ ] Insert primary YubiKey
- [ ] Run: `ykman oath accounts add "Pi5 Vault" --oath-type TOTP --touch-required`
- [ ] Press Enter to auto-generate secret (OR enter your own)
- [ ] Touch YubiKey when it blinks
- [ ] Test: `ykman oath accounts code "Pi5 Vault"` (should show 6-digit code)
- [ ] **IMPORTANT:** If you entered your own secret, save it securely for backup YubiKey!

### Backup YubiKey

- [ ] Remove primary YubiKey
- [ ] Insert backup YubiKey
- [ ] Run: `ykman oath accounts add "Pi5 Vault" --oath-type TOTP --touch-required`
- [ ] Enter THE SAME secret as primary (or press Enter if you want different)
- [ ] Touch YubiKey when it blinks
- [ ] Test: `ykman oath accounts code "Pi5 Vault"`
- [ ] Verify both keys generate same code (if using same secret)

---

## Pi5 Setup

- [ ] SSH to Pi5: `ssh pi5.local`
- [ ] Navigate to project: `cd /path/to/pi5-vault/pi5`
- [ ] Build: `./build.sh`
- [ ] Install: `sudo install -m 755 vault* /usr/local/bin/`
- [ ] Create secrets dir: `sudo mkdir -p /mnt/data/secrets`
- [ ] Set ownership: `sudo chown $USER:$USER /mnt/data/secrets`
- [ ] Set permissions: `chmod 700 /mnt/data/secrets`
- [ ] Test: `vault-list` (should run without error)

---

## Windows Auth Proxy

### Option A: PowerShell (Easier)

- [ ] Open PowerShell
- [ ] Navigate: `cd "C:\Local-only PARA\1 Projects\pi5-vault\windows\powershell"`
- [ ] Run: `.\vault-auth-proxy.ps1`
- [ ] Verify banner shows "Running on http://localhost:3000"
- [ ] **KEEP THIS WINDOW OPEN**

### Option B: Go (Alternative)

- [ ] Open PowerShell
- [ ] Navigate: `cd "C:\Local-only PARA\1 Projects\pi5-vault\windows\go"`
- [ ] Build: `.\build.ps1`
- [ ] Run: `.\vault-auth-proxy.exe`
- [ ] Verify banner shows
- [ ] **KEEP THIS WINDOW OPEN**

---

## SSH Tunnel

- [ ] Open NEW SSH session to Pi5
- [ ] Find your Windows laptop IP: `ipconfig` (on Windows)
- [ ] Run tunnel: `ssh -R 3000:localhost:3000 YOUR_USER@YOUR_LAPTOP_IP`
- [ ] Enter password/authenticate
- [ ] **KEEP THIS SESSION OPEN**
- [ ] Test from Pi5: `curl http://localhost:3000/health` (should return JSON)

---

## Testing

### Test 1: Store Secret

- [ ] On Pi5: `echo "test123" | vault-set test_secret`
- [ ] You should see "Touch YubiKey" on Windows
- [ ] Touch YubiKey when it blinks
- [ ] Verify: "✓ Session key cached"
- [ ] Verify: "✓ Secret encrypted and stored"

### Test 2: Retrieve Secret

- [ ] On Pi5: `vault-get test_secret`
- [ ] Should immediately return: `test123`
- [ ] No YubiKey touch needed (cached!)

### Test 3: List Secrets

- [ ] On Pi5: `vault-list`
- [ ] Should show: `test_secret`

### Test 4: Delete Secret

- [ ] On Pi5: `vault-delete test_secret`
- [ ] Verify: "✓ Secret deleted"

### Test 5: Cached Session

- [ ] On Pi5: `echo "cached" | vault-set another_test`
- [ ] Should NOT prompt for YubiKey touch
- [ ] Uses cached session key

### Test 6: Audit Log

- [ ] On Pi5: `cat /mnt/data/secrets/.audit.log`
- [ ] Should show JSON entries for all operations

---

## Cleanup Test Secrets

- [ ] `vault-delete test_secret` (if not already deleted)
- [ ] `vault-delete another_test`
- [ ] `vault-delete cached_test`
- [ ] `vault-list` (should be empty)

---

## Optional: Persistent SSH Tunnel

- [ ] Install autossh: `sudo apt install autossh`
- [ ] Create systemd service (see SETUP-WALKTHROUGH.md)
- [ ] Enable: `sudo systemctl enable vault-tunnel`
- [ ] Start: `sudo systemctl start vault-tunnel`
- [ ] Check: `sudo systemctl status vault-tunnel`

---

## You're Done! 🎉

Your vault is ready to use. Next steps:

- [ ] Store real secrets: `vault-set db_password`, `vault-set api_key`, etc.
- [ ] Integrate with apps on Pi5
- [ ] Review audit logs periodically
- [ ] Store backup YubiKey in safe location

---

## Quick Troubleshooting

**Problem:** "cannot reach auth proxy"
- Is auth proxy running on Windows?
- Is SSH tunnel active?
- Test: `curl http://localhost:3000/health` on Pi5

**Problem:** "YubiKey error"
- Is YubiKey inserted?
- Test: `ykman oath accounts list` on Windows
- Does it show "Pi5 Vault"?

**Problem:** "decryption failed"
- Different YubiKey secret used?
- Time drift between Windows and Pi5?
- Run: `sudo ntpdate pool.ntp.org` on Pi5

**Problem:** Port 3000 in use
- Find process: `netstat -ano | findstr :3000` on Windows
- Kill it or change port in both vault.go and auth proxy

---

See `SETUP-WALKTHROUGH.md` for detailed instructions!
