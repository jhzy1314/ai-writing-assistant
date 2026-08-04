@echo off
cd /d "%~dp0"
echo [1/3] Stopping old server...
taskkill /F /IM server.exe >nul 2>&1
timeout /t 1 /nobreak >nul

echo [1.5/3] Rebuilding latest version (if source + Go available)...
if exist "cmd\server\main.go" (
    where go >nul 2>&1
    if not errorlevel 1 (
        call go build -o server.exe ./cmd/server
        if errorlevel 1 (
            echo   [!] Build failed, using existing server.exe
        ) else (
            echo   [OK] server.exe is up to date
        )
    ) else (
        echo   [!] Go not found, using existing server.exe
    )
)

echo [2/3] Starting server...
start "" "%~dp0server.exe"
timeout /t 3 /nobreak >nul

echo [3/3] Opening browser...
start "" "http://localhost:8081/"
echo Ready.
