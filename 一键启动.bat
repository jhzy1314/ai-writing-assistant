@echo off
chcp 65001 >nul 2>&1
cd /d "%~dp0"
setlocal enabledelayedexpansion

title AI辅助写作助手 · 一键启动向导

:: ══════════════════════════════════════════════════════════════
::  第 0 步：欢迎 + 自检
:: ══════════════════════════════════════════════════════════════
cls
echo.
echo   ╔══════════════════════════════════════════════════╗
echo   ║        AI辅助写作助手 · 一键启动              ║
echo   ║      多模型角色调度 AI 写作协作平台            ║
echo   ╚══════════════════════════════════════════════════╝
echo.
echo   正在检查运行环境…

:: --- 检查 PowerShell ---
where powershell >nul 2>&1
if errorlevel 1 (
    echo   [X] 未找到 PowerShell，请确认系统为 Windows 10 或更高版本
    pause
    exit /b 1
)
echo   [OK] PowerShell 就绪

:: --- 检查 server.exe ---
if not exist "server.exe" (
    echo   [X] 未找到 server.exe，请将本文件放在解压后的文件夹内运行
    pause
    exit /b 1
)
echo   [OK] server.exe 就绪

:: --- 检查端口占用 ---
netstat -ano 2>nul | findstr /R ":8081 .*LISTENING" >nul 2>&1
if not errorlevel 1 (
    echo   [!] 端口 8081 已被占用，正在释放…
    taskkill /F /IM server.exe >nul 2>&1
    timeout /t 2 /nobreak >nul
    echo   [OK] 已释放端口
)
echo   [OK] 端口 8081 可用

:: --- 自动重建：保证 server.exe 与源码同步（前端 embed，不重建不生效） ---
if exist "cmd\server\main.go" (
    where go >nul 2>&1
    if not errorlevel 1 (
        echo   [OK] 检测到源码，自动重建最新版…
        call go build -o server.exe ./cmd/server
        if errorlevel 1 (
            echo   [!] 构建失败，使用现有 server.exe
        ) else (
            echo   [OK] server.exe 已更新到最新
        )
    )
)

:: ══════════════════════════════════════════════════════════════
::  第 1 步：检查 API Key
:: ══════════════════════════════════════════════════════════════
set NEED_CONFIG=0

if not exist "configs\config.yaml" (
    set NEED_CONFIG=1
    goto :CONFIG_CHECK_DONE
)

:: 检查是否有有效 Key（排除占位符）
findstr /R "api_key:.*sk-" "configs\config.yaml" >nul 2>&1
if errorlevel 1 (
    set NEED_CONFIG=1
    goto :CONFIG_CHECK_DONE
)

findstr /R "sk-your\|sk-placeholder\|sk-test\|your-key-here\|changeme\|请替换\|填入你的" "configs\config.yaml" >nul 2>&1
if not errorlevel 1 (
    set NEED_CONFIG=1
)

:CONFIG_CHECK_DONE

if %NEED_CONFIG%==1 (
    echo   [!] 检测到 API Key 未配置
    echo.
    echo   +----------------------------------------------------------+
    echo   ^|  首次使用需要配置模型 API Key。                            ^|
    echo   ^|  推荐 DeepSeek (platform.deepseek.com) ，注册即送额度。    ^|
    echo   ^|  按任意键启动配置向导，填入 Key 后即可自动启动。           ^|
    echo   ^+----------------------------------------------------------+
    echo.
    pause >nul
    powershell -ExecutionPolicy Bypass -File "%~dp0setup.ps1" -NoLaunch
    if errorlevel 1 (
        echo.
        echo   [X] 配置失败。你可以稍后双击 setup.bat 重新配置。
        pause
        exit /b 1
    )
    echo.
    echo   [OK] 配置已保存，正在继续启动…
) else (
    echo   [OK] API Key 已配置
)

:: ══════════════════════════════════════════════════════════════
::  第 2 步：启动服务
:: ══════════════════════════════════════════════════════════════
echo.
echo   正在启动 AI辅助写作助手…
start "" "%~dp0server.exe"
timeout /t 3 /nobreak >nul

:: ══════════════════════════════════════════════════════════════
::  第 3 步：打开浏览器 + 新手引导
:: ══════════════════════════════════════════════════════════════
echo   正在打开浏览器…
start "" "http://localhost:8081/"

:: 如果是首次使用，同时打开操作指南
if exist "新手操作指南.html" (
    echo   正在打开新手操作指南…
    start "" "%~dp0新手操作指南.html"
)

echo.
echo   ==========================================
echo   AI辅助写作助手 已启动！
echo.
echo   · 写作界面：http://localhost:8081/
echo   · 新手指南：双击 新手操作指南.html
echo   · 重新配置：双击 setup.bat
echo   · 按 ? 键查看快捷键
echo   ==========================================
echo.
echo   关闭此窗口将停止服务。
pause >nul
taskkill /F /IM server.exe >nul 2>&1
