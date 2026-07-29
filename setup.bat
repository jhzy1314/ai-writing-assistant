@echo off
cd /d "%~dp0"
echo ============================================
echo   AI Novel Studio · API Key 配置向导
echo ============================================
echo.
powershell -ExecutionPolicy Bypass -File "%~dp0setup.ps1"
pause
