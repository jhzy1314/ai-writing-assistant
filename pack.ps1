$ErrorActionPreference = "Stop"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $scriptDir

$now = Get-Date -Format "yyyyMMdd"
$zipName = "AI-Novel-Studio_${now}.zip"
$buildDir = Join-Path $env:TEMP "ai-novel-pack"

# Config template with placeholder key (triggers setup wizard on first run)
$configLines = @(
    '# AI Novel Studio config',
    '# The setup wizard will auto-popup on first launch.',
    '',
    'server:',
    '  listen_addr: "0.0.0.0"',
    '  port: 8081',
    '  sqlite_path: "data/ai-novel.db"',
    '  log_dir: "log"',
    '  auth_password: ""',
    '',
    'quotas:',
    '  daily_call_limit: 500',
    '  daily_token_limit: 2000000',
    '  per_request_token_limit: 0',
    '  light_input_char_limit: 500',
    '  max_iterations: 3',
    '  rate_limit_per_minute: 20',
    '  max_concurrent: 5',
    '  warn_ratio: 0.8',
    '',
    'models:',
    '  - name: "deepseek-v4-pro"',
    '    vendor: "DeepSeek"',
    '    api_endpoint: "https://api.deepseek.com"',
    '    api_key: "sk-your-api-key-here"',
    '    status: "active"',
    '    daily_limit: 0',
    '',
    'role_models:',
    '  thinker: ["deepseek-v4-pro"]',
    '  worker: ["deepseek-v4-pro"]',
    '  verifier: ["deepseek-v4-pro"]',
    '  helper: ["deepseek-v4-pro"]',
    ''
)

Write-Host ""
Write-Host "  AI Novel Studio - Packager" -ForegroundColor Cyan
Write-Host "  ===========================" -ForegroundColor Cyan
Write-Host ""

# Step 1: Find source files
# Use wildcard patterns to locate files regardless of encoding
$srcExe   = Get-ChildItem $scriptDir -Name "server.exe" -File | Select-Object -First 1
$srcBat1  = Get-ChildItem $scriptDir -Name "*YiJian*" -File | Select-Object -First 1
$srcBat2  = Get-ChildItem $scriptDir -Name "setup.bat" -File | Select-Object -First 1
$srcPs1   = Get-ChildItem $scriptDir -Name "setup.ps1" -File | Select-Object -First 1
$srcGuide = Get-ChildItem $scriptDir -Name "*Guide*" -File | Where-Object { $_.EndsWith(".html") } | Select-Object -First 1
if (-not $srcGuide) { $srcGuide = Get-ChildItem $scriptDir -Name "*ZhiNan*" -File | Where-Object { $_.EndsWith(".html") } | Select-Object -First 1 }
if (-not $srcGuide) { $srcGuide = Get-ChildItem $scriptDir -Name "*zn*" -File | Where-Object { $_.EndsWith(".html") } | Select-Object -First 1 }

# Backup fallback
if (-not $srcBat1) {
    $srcBat1 = Get-ChildItem $scriptDir -Name "*.bat" -File | Where-Object { $_ -notlike "*run*" -and $_ -notlike "*setup*" } | Select-Object -First 1
}
if (-not $srcGuide) {
    $srcGuide = Get-ChildItem $scriptDir -Name "*.html" -File | Where-Object { $_ -notlike "index*" -and $_ -notlike "AI_Novel*" -and $_ -notlike "github*" } | Select-Object -First 1
}

Write-Host "  Source files found:"
Write-Host "    exe  = $srcExe"
Write-Host "    bat1 = $srcBat1"
Write-Host "    bat2 = $srcBat2"
Write-Host "    ps1  = $srcPs1"
Write-Host "    guide= $srcGuide"

$missing = @()
if (-not $srcExe)   { $missing += "server.exe" }
if (-not $srcBat1)  { $missing += "*YiJian*.bat (launcher)" }
if (-not $srcBat2)  { $missing += "setup.bat" }
if (-not $srcPs1)   { $missing += "setup.ps1" }
if (-not $srcGuide) { $missing += "*Guide*.html (beginner guide)" }

if ($missing.Count -gt 0) {
    Write-Host "  [X] Missing: $($missing -join ', ')" -ForegroundColor Red
    exit 1
}
Write-Host "  [OK] All required files present" -ForegroundColor Green

# Step 2: Create output directory
if (Test-Path $buildDir) { Remove-Item $buildDir -Recurse -Force }
$outDir = Join-Path $buildDir "AI-Novel-Studio"
$null = New-Item -ItemType Directory -Path $outDir -Force
$null = New-Item -ItemType Directory -Path (Join-Path $outDir "configs") -Force
$null = New-Item -ItemType Directory -Path (Join-Path $outDir "data") -Force
$null = New-Item -ItemType Directory -Path (Join-Path $outDir "log") -Force
Write-Host "  [OK] Output dir created" -ForegroundColor Green

# Step 3: Copy files
$copyList = @(
    @{S=$srcExe;   D="server.exe"},
    @{S=$srcBat1;  D="AI-YiJianQiDong.bat"},
    @{S=$srcBat2;  D="setup.bat"},
    @{S=$srcPs1;   D="setup.ps1"},
    @{S=$srcGuide; D="Beginner-Guide.html"}
)

foreach ($cp in $copyList) {
    $src = Join-Path $scriptDir $cp.S
    $dst = Join-Path $outDir $cp.D
    Copy-Item $src $dst -Force
    Write-Host "    + $($cp.D)" -ForegroundColor Gray
}

# Generate placeholder config (will trigger setup wizard)
$configPath = Join-Path $outDir "configs\config.yaml"
$configContent = $configLines -join "`n"
[System.IO.File]::WriteAllText($configPath, $configContent, [System.Text.UTF8Encoding]::new($false))
Write-Host "    + configs/config.yaml (placeholder)" -ForegroundColor Gray

# Keep empty dirs
"" | Out-File (Join-Path $outDir "data\.keep") -Encoding UTF8
"" | Out-File (Join-Path $outDir "log\.keep") -Encoding UTF8
Write-Host "  [OK] All files copied" -ForegroundColor Green

# Step 4: Create ZIP
$zipPath = Join-Path $scriptDir $zipName
if (Test-Path $zipPath) { Remove-Item $zipPath -Force }

Add-Type -AssemblyName System.IO.Compression.FileSystem
[System.IO.Compression.ZipFile]::CreateFromDirectory($outDir, $zipPath,
    [System.IO.Compression.CompressionLevel]::Optimal, $false)

$zipSize = [math]::Round((Get-Item $zipPath).Length / 1MB, 1)

Write-Host ""
Write-Host "  ==================================" -ForegroundColor Green
Write-Host "  Package complete!" -ForegroundColor Green
Write-Host "  File : $zipName ($($zipSize) MB)" -ForegroundColor White
Write-Host "  ==================================" -ForegroundColor Green
Write-Host ""
Write-Host "  To share this with a friend:" -ForegroundColor Cyan
Write-Host "   1. Send them the ZIP file"
Write-Host "   2. They unzip to any folder"
Write-Host "   3. Double-click AI-YiJianQiDong.bat"
Write-Host "   4. Enter API Key when prompted"
Write-Host "   5. Browser opens -> start writing!"
Write-Host ""

# Step 5: Cleanup
Remove-Item $buildDir -Recurse -Force -ErrorAction SilentlyContinue
Write-Host "  [OK] Temp files cleaned" -ForegroundColor Gray
