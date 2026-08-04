@echo off
title AI辅助写作助手 启动器
cd /d "%~dp0"

echo.
echo  ============================================
echo    AI辅助写作助手 - 本地 AI 小说创作软件
echo  ============================================
echo.

rem ---- 检查 server.exe ----
if not exist "server.exe" (
    echo [错误] 未找到 server.exe，请确认压缩包完整解压。
    pause
    exit /b 1
)

rem ---- 防重复启动：检查端口 8081 是否已被本程序占用 ----
netstat -ano | findstr ":8081" | findstr "LISTENING" >nul 2>&1
if %errorlevel%==0 (
    echo [提示] 检测到 AI辅助写作助手 已在运行（端口 8081），
    echo        将直接打开浏览器，无需重复启动。
    start "" http://localhost:8081
    exit /b 0
)

echo [启动] 正在启动 AI辅助写作助手...
start "" "server.exe"

rem ---- 等待服务就绪（最多 20 秒）----
set /a tries=0
:waitloop
set /a tries+=1
if %tries% gtr 20 goto timeout
curl -s -o nul http://localhost:8081/ >nul 2>&1
if %errorlevel%==0 goto ready
timeout /t 1 /nobreak >nul
goto waitloop

:timeout
echo [警告] 服务启动较慢，请在浏览器手动打开 http://localhost:8081
start "" http://localhost:8081
goto end

:ready
echo [成功] 服务已就绪，正在打开浏览器...
start "" http://localhost:8081

:end
echo.
echo  提示：关闭本窗口不会停止服务。
echo        如需停止，请在任务管理器结束 server.exe 进程。
echo.
pause
