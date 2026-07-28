$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $scriptDir

$port = 8081
$url = "http://localhost:$port"

$portInUse = netstat -ano 2>$null | Select-String ":$port .*LISTENING"
if ($portInUse) {
    Write-Host "[Skip] Server already running" -ForegroundColor Yellow
} else {
    if (-not (Test-Path "server.exe")) {
        Write-Host "[Error] server.exe not found" -ForegroundColor Red
        Read-Host "Press Enter to exit"
        exit 1
    }
    Write-Host "[Start] Launching server..." -ForegroundColor Green
    Start-Process -FilePath ".\server.exe" -WindowStyle Hidden

    Write-Host "[Wait] Waiting for server..." -ForegroundColor Green
    $maxWait = 30
    for ($i = 0; $i -lt $maxWait; $i++) {
        Start-Sleep -Seconds 1
        try {
            $null = Invoke-WebRequest -Uri "$url/api/v1/novels" -UseBasicParsing -TimeoutSec 2
            break
        } catch {
            if ($i -eq $maxWait - 1) {
                Write-Host "[Error] Server startup timeout" -ForegroundColor Red
                Read-Host "Press Enter to exit"
                exit 1
            }
        }
    }
}

Write-Host "[Open] Opening browser..." -ForegroundColor Green
Start-Process $url

Write-Host "[Ready] AI Novel Studio is running!" -ForegroundColor Cyan
Write-Host "URL: $url" -ForegroundColor White
Start-Sleep -Seconds 2
