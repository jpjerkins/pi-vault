# Security FAQ

Common security questions about Pi5 Vault.

## Core Security Model

### Q: What secrets are stored on pi5?

**A:** Only encrypted secrets. The encryption keys are NOT on pi5.

**On pi5:**
- ✅ Encrypted secrets (`.enc` files)
- ✅ Cached session key (30min TTL, expires automatically)
- ✅ Audit log

**NOT on pi5:**
- ❌ TOTP seed (master secret)
- ❌ YubiKey secrets
- ❌ Master password
- ❌ Any way to derive session keys

**Why this matters:** A root attacker on pi5 finds only encrypted data. After 30 minutes, the cached session key expires and they cannot derive new ones without your YubiKey.

### Q: If someone has root access to my pi5, can they decrypt my secrets?

**A:** Only if they catch the cached session key within the 30-minute window AND access secrets during that time.

**Attack timeline:**
```
T=0:00   Attacker gains root access on pi5
T=0:01   Finds encrypted secrets - useless without key
T=0:02   Finds cached session key (if you accessed vault recently)
T=0:03   Can decrypt secrets using cached key
T=30:00  Cached session key expires
T=30:01  Attacker locked out - cannot derive new keys
```

**What they need to maintain access:**
- Your YubiKey (physical possession)
- Your laptop (to run auth proxy)
- Active SSH tunnel to pi5
- All three simultaneously

**Bottom line:** Root access gives brief window (up to 30min), not permanent access.

### Q: What happens if someone steals my laptop?

**A:** They still need your YubiKey.

**What they have:**
- Your laptop (can run auth proxy)
- Potentially: SSH keys to pi5

**What they don't have:**
- YubiKey (you can remove it and keep it separately)

**Protection:**
1. Don't leave YubiKey plugged in when not using vault
2. Store YubiKey separately from laptop when traveling
3. Use SSH key passphrase (so stolen laptop can't auto-SSH to pi5)

## Google Authenticator

### Q: Why is Google Authenticator mentioned? Doesn't that mean secrets on my phone?

**A:** No. Google Authenticator is ONLY for disaster recovery, not daily use.

**What Google Auth IS used for:**
- ✅ Programming new YubiKeys if both are lost/destroyed
- That's it. Period.

**What Google Auth is NOT used for:**
- ❌ Daily vault access
- ❌ Accessing vault from phone
- ❌ Fallback when away from laptop
- ❌ Anything on pi5

**Where Google Auth lives:**
- On your phone (in your possession)
- Never synced to pi5
- Never used to access vault directly

**How disaster recovery works:**
```
1. Both YubiKeys lost (house fire, etc.)
2. Open Google Authenticator on phone
3. Buy new YubiKey
4. Program new YubiKey with seed from Google Auth:
   ykman oath accounts add "Pi5 Vault" <seed>
5. New YubiKey works - back in business
```

**The TOTP seed from Google Auth is used to program a NEW YubiKey on your laptop, not to access the vault.**

### Q: Does Google Authenticator weaken security?

**A:** Slightly, but it doesn't compromise the pi5 security model.

**Risk assessment:**

**Scenario: Phone stolen**
```
Attacker has: TOTP seed in Google Auth
Still needs:
  - SSH access to pi5 (your SSH key or password)
  - Your laptop (to run auth proxy)
  - Active SSH tunnel

Without all three, TOTP seed alone is useless
```

**Scenario: Root on pi5**
```
Attacker has: Root on pi5
Can find: Encrypted secrets
Cannot find: TOTP seed (not on pi5, only on your devices)

Google Auth doesn't change this - seed still not on pi5
```

**Trade-off:**
- ⚠️ Adds small risk (phone theft = TOTP seed exposure)
- ✅ Faster disaster recovery (no waiting for printed doc)
- ✅ Still doesn't put seed on pi5 (core security maintained)

**Mitigation:**
- Strong phone lock screen (biometric + PIN)
- Phone encryption enabled
- Keep backup YubiKey in safe (primary recovery method)

### Q: Should I use Google Authenticator?

**A:** Depends on your threat model.

**Use Google Auth if:**
- ✅ You want fastest disaster recovery
- ✅ Your phone has strong security (biometric lock, encryption)
- ✅ You accept small additional risk for convenience

**Skip Google Auth if:**
- ✅ You want absolute maximum security
- ✅ You're okay with printed recovery doc as only backup
- ✅ You're uncomfortable with TOTP seed on phone

**Remember:** It's optional. The vault works perfectly without it.

## YubiKey Security

### Q: What if someone steals my YubiKey?

**A:** They still need your laptop + SSH access to pi5.

**Attack requirements:**
```
Stolen YubiKey alone: Useless
  Needs: Your laptop (to run auth proxy)
  Needs: SSH access to pi5

Stolen YubiKey + your laptop: Partial threat
  Still needs: SSH key passphrase (if you use one)
  OR: Your pi5 password (if you use password auth)

Protection: Remove YubiKey when not in use
```

**If YubiKey is stolen:**
1. Use backup YubiKey from safe
2. Change SSH keys on pi5
3. Monitor audit log for unauthorized access
4. Consider re-encrypting secrets with new TOTP seed

### Q: Can I use my YubiKey for other things too?

**A:** Yes! The OATH TOTP slot is separate from other functions.

**YubiKey has multiple functions:**
- FIDO2/U2F (web authentication)
- PIV (smart card)
- OATH TOTP (what we use)
- Static password
- Challenge-response

**Each function is independent.** Using OATH for pi5-vault doesn't affect other uses.

### Q: What if I lose both YubiKeys?

**A:** Use recovery methods:

**Recovery options (in order):**

1. **Google Authenticator (if configured):**
   - Open Google Auth on phone
   - Buy new YubiKey
   - Program with seed: `ykman oath accounts add "Pi5 Vault" <seed>`
   - Works immediately

2. **Printed recovery document (if created):**
   - Retrieve from safe
   - Buy new YubiKey
   - Program with seed from document
   - Works immediately

3. **No recovery method:**
   - **Locked out permanently**
   - Secrets unrecoverable (intentional security design)
   - This is why backup YubiKey is critical

## Network Security

### Q: Can the vault be accessed over the internet?

**A:** No. Only via SSH tunnel from your laptop.

**Access requirements:**
- Must SSH from laptop (where YubiKey is)
- Auth proxy must be running on that laptop
- SSH tunnel auto-forwards auth proxy port

**Cannot access from:**
- ❌ Web browser directly to pi5
- ❌ Phone app
- ❌ Another computer without SSH tunnel
- ❌ The internet (no exposed ports)

**This is intentional:** Limits attack surface to SSH + local auth proxy.

### Q: What if SSH tunnel is hijacked?

**A:** Attacker could intercept session key derivation.

**Risk:**
- Session keys transmitted over SSH tunnel
- If tunnel is compromised (MITM), attacker might capture session key
- Session key valid for 30 minutes

**Mitigation:**
1. SSH uses strong encryption (if configured properly)
2. Verify SSH host keys (prevents MITM)
3. Use SSH certificates if in high-security environment
4. Monitor audit log for unexpected access
5. Keep session TTL short (30min default)

## Operational Security

### Q: Can I leave auth proxy running all the time?

**A:** Yes, but remove YubiKey when not using vault.

**Recommended practice:**
```
Working with vault:
  - Start auth proxy
  - Insert YubiKey
  - SSH to pi5, access secrets
  - Remove YubiKey
  - Leave auth proxy running (harmless without YubiKey)

Done for the day:
  - Close SSH session
  - Stop auth proxy (or leave running)
  - Store YubiKey separately from laptop
```

**Why remove YubiKey:** Even with auth proxy running, without YubiKey it can't derive keys.

### Q: What's in the audit log?

**A:** All vault access attempts.

**Log format:**
```json
{"timestamp":"2026-03-12T10:30:15Z","action":"get","secret":"db_password","success":true}
{"timestamp":"2026-03-12T10:31:00Z","action":"set","secret":"api_key","success":true}
{"timestamp":"2026-03-12T10:32:00Z","action":"get","secret":"github_token","success":false,"error":"permission denied"}
```

**What's logged:**
- Every secret access (get, set, delete)
- Timestamp
- Success/failure
- Error messages

**What's NOT logged:**
- Secret values (never in plaintext)
- Session keys
- TOTP codes

**Use for:**
- Detecting unauthorized access
- Compliance auditing
- Troubleshooting access issues

### Q: How often should I rotate secrets?

**A:** Depends on the secret and your security policy.

**Rotation strategy:**

**High-frequency (monthly):**
- API keys for expensive services
- Database passwords for production
- Service account tokens

**Medium-frequency (quarterly):**
- Internal service credentials
- Development API keys

**Low-frequency (yearly):**
- Recovery passphrases
- TOTP seeds (requires re-programming YubiKeys)

**Process:**
```bash
# Rotate a secret
echo "new_secret_value" | vault-set db_password

# Update services using the secret
# (depends on your deployment process)

# Verify new secret works
vault-get db_password

# Old secret automatically overwritten
```

### Q: Can I have different vaults for different environments?

**A:** Not with current implementation, but you can simulate it.

**Current limitation:** One vault per pi5 (one `/mnt/data/secrets/` directory).

**Workarounds:**

**Option 1: Naming convention**
```bash
vault-set prod_db_password "secret1"
vault-set staging_db_password "secret2"
vault-set dev_db_password "secret3"
```

**Option 2: Multiple pi5 instances**
- Different pi5 for prod/staging
- Each has its own vault
- Different YubiKey TOTP accounts ("Pi5 Vault Prod", "Pi5 Vault Staging")

**Option 3: Hybrid approach**
- Pi5 vault for production secrets
- Traditional secrets (SOPS, ansible-vault) for dev/staging

## Comparison to Other Solutions

### Q: Why not use HashiCorp Vault?

**A:** Too complex for single-node setup, doesn't provide YubiKey tap security.

**HashiCorp Vault:**
- ✅ Industry standard, many features
- ⚠️ Complex setup (requires Consul/etcd for HA)
- ⚠️ Heavy resource usage
- ⚠️ Auto-unseal (no physical auth factor like YubiKey tap)

**This vault:**
- ✅ Simple (single binary)
- ✅ Physical security (YubiKey required)
- ✅ Perfect for single-node (pi5)
- ⚠️ Less features (but you might not need them)

### Q: Why not use password manager (Bitwarden, 1Password)?

**A:** Designed for human use, not service automation.

**Password managers:**
- ✅ Great for human passwords
- ✅ Web UI, mobile apps
- ⚠️ Manual copy-paste (not programmable)
- ⚠️ Not designed for service-to-service secrets

**This vault:**
- ✅ Programmable (CLI, scripts, APIs)
- ✅ Designed for service secrets
- ✅ Audit logging
- ⚠️ No UI (command-line only)

### Q: Why not use SOPS or age?

**A:** No runtime authentication, secrets in plaintext after decrypt.

**SOPS/age:**
- ✅ Simple file encryption
- ✅ Git-friendly
- ⚠️ No runtime service (one-time decrypt)
- ⚠️ Decrypted secrets stay in plaintext
- ⚠️ No physical auth factor

**This vault:**
- ✅ Runtime service (secrets always encrypted on disk)
- ✅ Physical auth factor (YubiKey tap)
- ✅ Session-based access (keys expire)
- ⚠️ More complex setup

## Getting Help

**If you have security concerns:**

1. Review this FAQ
2. Read full design doc (linked in CLAUDE.md)
3. Check threat model in README.md
4. Examine source code (it's simple Go, ~500 lines)

**If you find a security issue:**

1. Don't use the vault for production until resolved
2. Document the issue clearly
3. Consider if it's acceptable risk for your threat model

**Remember:** No security system is perfect. This vault protects against specific threats (disk theft, pi5 root compromise) while accepting others (laptop+YubiKey theft). Choose tools that match your threat model.
