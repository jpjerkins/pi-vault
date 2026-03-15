# Pi5 Vault Authentication Proxy
# Runs on Windows laptop, provides YubiKey access to pi5 vault
# Requires: YubiKey Manager (ykman.exe in PATH)

$listener = New-Object System.Net.HttpListener
$listener.Prefixes.Add("http://localhost:3000/")

try {
    $listener.Start()
} catch {
    Write-Host "❌ Error: Could not start listener on port 3000" -ForegroundColor Red
    Write-Host "   Is another instance already running?" -ForegroundColor Yellow
    exit 1
}

Write-Host "═══════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host "  Pi5 Vault Authentication Proxy" -ForegroundColor Cyan
Write-Host "═══════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host ""
Write-Host "✓ Running on http://localhost:3000" -ForegroundColor Green
Write-Host "  YubiKey ready for authentication" -ForegroundColor White
Write-Host "  Press Ctrl+C to stop" -ForegroundColor Gray
Write-Host ""

# Verify ykman is available
try {
    $null = & ykman --version 2>&1
} catch {
    Write-Host "⚠ Warning: ykman command not found" -ForegroundColor Yellow
    Write-Host "  Install YubiKey Manager from: https://www.yubico.com/support/download/yubikey-manager/" -ForegroundColor Yellow
    Write-Host ""
}

try {
    while ($listener.IsListening) {
        # Use async GetContext with timeout so Ctrl+C can interrupt
        $contextTask = $listener.GetContextAsync()

        while (-not $contextTask.AsyncWaitHandle.WaitOne(200)) {
            # Check every 200ms to allow Ctrl+C to interrupt
        }

        $context = $contextTask.GetAwaiter().GetResult()
        $request = $context.Request
        $response = $context.Response
        $timestamp = Get-Date -Format 'HH:mm:ss'

        try {
            if ($request.HttpMethod -eq "POST" -and $request.Url.AbsolutePath -eq "/derive-key") {
                Write-Host "[$timestamp] " -NoNewline -ForegroundColor Gray
                Write-Host "🔐 Deriving session key from YubiKey..." -ForegroundColor Yellow

                # Fixed challenge for HMAC-SHA1 key derivation ("pi5-vault" in hex).
                # Both YubiKeys must be programmed with the same HMAC-SHA1 secret so that
                # either key produces the same output for this challenge.
                $vaultChallenge = "7069352d7661756c74"

                # Compute HMAC-SHA1 challenge-response from YubiKey slot 2
                $hmacHex = & ykman otp calculate 2 $vaultChallenge 2>&1

                # Check if command failed or returned an error object
                if ($LASTEXITCODE -ne 0 -or $hmacHex -is [System.Management.Automation.ErrorRecord]) {
                    throw "YubiKey error: Is slot 2 configured with HMAC-SHA1 challenge-response? Run: ykman otp info"
                }

                # Convert to string and trim
                $hmacHex = "$hmacHex".Trim()

                # SHA256 the HMAC output to produce a 32-byte AES-256 key
                $sha256 = [System.Security.Cryptography.SHA256]::Create()
                $keyBytes = $sha256.ComputeHash([System.Text.Encoding]::UTF8.GetBytes($hmacHex))
                $sessionKey = [Convert]::ToBase64String($keyBytes)

                Write-Host "[$timestamp] " -NoNewline -ForegroundColor Gray
                Write-Host "✓ Session key derived" -ForegroundColor Green

                $responseBody = @{
                    session_key = $sessionKey
                    expires_at  = [DateTime]::UtcNow.AddMinutes(30).ToString("o")
                } | ConvertTo-Json

                $buffer = [System.Text.Encoding]::UTF8.GetBytes($responseBody)
                $response.ContentLength64 = $buffer.Length
                $response.ContentType = "application/json"
                $response.StatusCode = 200
                $response.OutputStream.Write($buffer, 0, $buffer.Length)
            }
            elseif ($request.HttpMethod -eq "GET" -and $request.Url.AbsolutePath -eq "/health") {
                $healthBody = @{
                    status    = "running"
                    timestamp = [DateTime]::UtcNow.ToString("o")
                } | ConvertTo-Json

                $buffer = [System.Text.Encoding]::UTF8.GetBytes($healthBody)
                $response.ContentLength64 = $buffer.Length
                $response.ContentType = "application/json"
                $response.StatusCode = 200
                $response.OutputStream.Write($buffer, 0, $buffer.Length)

                Write-Host "[$timestamp] " -NoNewline -ForegroundColor Gray
                Write-Host "Health check" -ForegroundColor Gray
            }
            else {
                $response.StatusCode = 404
                Write-Host "[$timestamp] " -NoNewline -ForegroundColor Gray
                Write-Host "404 Not Found: $($request.HttpMethod) $($request.Url.AbsolutePath)" -ForegroundColor Red
            }
        }
        catch {
            Write-Host "[$timestamp] " -NoNewline -ForegroundColor Gray
            Write-Host "❌ Error: $_" -ForegroundColor Red

            $errorBody = @{error = $_.Exception.Message} | ConvertTo-Json
            $buffer = [System.Text.Encoding]::UTF8.GetBytes($errorBody)
            $response.ContentLength64 = $buffer.Length
            $response.ContentType = "application/json"
            $response.StatusCode = 500
            $response.OutputStream.Write($buffer, 0, $buffer.Length)
        }
        finally {
            $response.Close()
        }
    }
}
catch {
    # Ctrl+C or other interruption
    if ($_.Exception.Message -notlike "*operation was canceled*") {
        Write-Host "Error: $_" -ForegroundColor Red
    }
}
finally {
    $listener.Stop()
    Write-Host ""
    Write-Host "✓ Auth proxy stopped." -ForegroundColor Yellow
}
