@echo off
cd /d "%~dp0"
setlocal enabledelayedexpansion

title AI辅助写作助手 - One Click Setup

:: ================================================================
::  AI辅助写作助手 - One Click Launcher
::  Double-click this file to start everything.
:: ================================================================

cls
echo.
echo   +-------------------------------------------------------+
echo   ^|     AI辅助写作助手 - Multi-Agent Novel Writing     ^|
echo   ^|           One-Click Launcher                        ^|
echo   +-------------------------------------------------------+
echo.
echo   Checking environment...

:: --- Check PowerShell ---
where powershell >nul 2>&1
if errorlevel 1 (
    echo   [X] PowerShell not found. Requires Windows 10 or later.
    pause
    exit /b 1
)
echo   [OK] PowerShell ready

:: --- Check server.exe ---
if not exist "server.exe" (
    echo   [X] server.exe not found.
    echo   Please ensure this file is in the unzipped folder.
    pause
    exit /b 1
)
echo   [OK] server.exe found

:: --- Check port ---
netstat -ano 2>nul | findstr /R ":8081 .*LISTENING" >nul 2>&1
if not errorlevel 1 (
    echo   [!] Port 8081 busy, releasing...
    taskkill /F /IM server.exe >nul 2>&1
    timeout /t 2 /nobreak >nul
)
echo   [OK] Port 8081 available

:: --- Auto rebuild: keep server.exe in sync with source code ---
:: (dev folder with source + Go toolchain => always build the latest version
::  before starting; released zip without source => skip and use bundled exe)
if exist "cmd\server\main.go" (
    where go >nul 2>&1
    if not errorlevel 1 (
        echo   [OK] Source + Go toolchain detected, rebuilding latest...
        call go build -o server.exe ./cmd/server
        if errorlevel 1 (
            echo   [!] Build failed, will start existing server.exe
        ) else (
            echo   [OK] Build finished, server.exe is up to date
        )
    ) else (
        echo   [!] Go not found, starting existing server.exe (may be outdated)
    )
)

:: ================================================================
::  Step 1: Check / Setup API Key
:: ================================================================
set NEED_CONFIG=0

if not exist "configs\config.yaml" (
    set NEED_CONFIG=1
    goto :CONFIG_DONE
)

findstr /R "api_key:.*sk-" "configs\config.yaml" >nul 2>&1
if errorlevel 1 (
    set NEED_CONFIG=1
    goto :CONFIG_DONE
)

findstr /R "sk-your\|sk-placeholder\|sk-test\|your-key-here\|changeme" "configs\config.yaml" >nul 2>&1
if not errorlevel 1 set NEED_CONFIG=1

:CONFIG_DONE

if %NEED_CONFIG%==1 (
    echo   [!] API Key not configured yet.
    echo.
    echo   +-------------------------------------------------------+
    echo   ^|  First time setup: you need a model API Key.         ^|
    echo   ^|                                                       ^|
    echo   ^|  Recommended: DeepSeek (free credits on signup)       ^|
    echo   ^|  1. Go to https://platform.deepseek.com              ^|
    echo   ^|  2. Create an API Key (starts with sk-)              ^|
    echo   ^|  3. Come back and paste it below                     ^|
    echo   ^+-------------------------------------------------------+
    echo.
    pause
    powershell -ExecutionPolicy Bypass -File "%~dp0setup.ps1" -NoLaunch
    if errorlevel 1 (
        echo.
        echo   [X] Setup cancelled or failed.
        echo   You can run setup.bat later to try again.
        pause
        exit /b 1
    )
    echo.
    echo   [OK] Config saved. Starting server...
) else (
    echo   [OK] API Key configured
)

:: ================================================================
::  Step 2: Start Server
:: ================================================================
echo.
echo   Starting AI辅助写作助手...
start "" "%~dp0server.exe"
timeout /t 4 /nobreak >nul

:: ================================================================
::  Step 3: Open Browser + Guide
:: ================================================================
echo   Opening browser...
start "" "http://localhost:8081/"

:: Open beginner guide (try both English and Chinese filenames)
if exist "Beginner-Guide.html" (
    echo   Opening beginner guide...
    start "" "%~dp0Beginner-Guide.html"
) else if exist "新手操作指南.html" (
    start "" "新手操作指南.html"
)

echo.
echo   ===========================================
echo   AI辅助写作助手 is running!
echo.
echo   Writing app : http://localhost:8081/
echo   Beginner guide : open Beginner-Guide.html
echo   Reconfigure   : double-click setup.bat
echo.
echo   Press ? in the app for keyboard shortcuts.
echo   ===========================================
echo.
echo   Close this window to stop the server.
pause >nul
taskkill /F /IM server.exe >nul 2>&1
