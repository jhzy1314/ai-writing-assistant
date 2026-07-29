@echo off
cd /d "%~dp0"
echo [1/3] Stopping old server...
taskkill /F /IM server.exe >nul 2>&1
timeout /t 1 /nobreak >nul

echo [2/3] Starting new server...
start "" "%~dp0server.exe"
timeout /t 3 /nobreak >nul

echo [3/3] Opening browser with fresh cache...
start "" "http://localhost:8081/?v=%random%%random%"
echo Ready.
