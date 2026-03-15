# Build script for Windows auth proxy (Go version)

Write-Host "Building vault-auth-proxy.exe for Windows..." -ForegroundColor Cyan

$env:GOOS = "windows"
$env:GOARCH = "amd64"

go build -o vault-auth-proxy.exe vault-auth-proxy.go

if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ Build complete!" -ForegroundColor Green
    Write-Host ""
    Write-Host "Binary created: vault-auth-proxy.exe"
    Get-Item vault-auth-proxy.exe | Select-Object Name, Length, LastWriteTime
    Write-Host ""
    Write-Host "To run:" -ForegroundColor Yellow
    Write-Host "  .\vault-auth-proxy.exe"
} else {
    Write-Host "❌ Build failed" -ForegroundColor Red
    exit 1
}
