@echo off
cd /d "%~dp0"
echo ============================================
echo   AI辅助写作助手 · API Key 配置向导
echo ============================================
echo.
powershell -ExecutionPolicy Bypass -File "%~dp0setup.ps1"
pause
