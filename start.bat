@echo off
setlocal EnableExtensions DisableDelayedExpansion
chcp 936 >nul

title Kugo Music Converter v0.2.2

echo ======================================
echo   Kugo Music Converter v0.2.2
echo   Support KGG/KGM/KGMA/VPR/NCM
echo ======================================
echo.

set "ROOT=%~dp0"
set "EXE=%ROOT%backend\bin\kugo-converter.exe"
set "FFMPEG=%ROOT%tools\ffmpeg.exe"

if not exist "%EXE%" (
    echo [ERROR] Missing kugo-converter.exe
    echo [ERROR] Expected path: %EXE%
    pause
    exit /b 1
)

if not exist "%FFMPEG%" (
    echo [WARN] Missing ffmpeg.exe
    echo [WARN] Expected path: %FFMPEG%
)

call :free_port_8080
if errorlevel 1 exit /b 1

echo [INFO] Starting backend service...
start "" /b cmd /c "timeout /t 2 /nobreak >nul & start http://localhost:8080"
"%EXE%" --addr :8080 --ffmpeg "%FFMPEG%"
set "EXIT_CODE=%ERRORLEVEL%"

echo.
echo [INFO] Service stopped. Exit code: %EXIT_CODE%
pause
exit /b %EXIT_CODE%

:free_port_8080
set "BUSY="
for /f "tokens=5" %%A in ('netstat -ano ^| findstr ":8080 " ^| findstr "LISTENING"') do (
    set "BUSY=1"
    echo [WARN] Port 8080 occupied. Killing PID %%A ...
    taskkill /F /PID %%A >nul 2>&1
)

if not defined BUSY goto :eof

timeout /t 1 /nobreak >nul
for /f "tokens=5" %%A in ('netstat -ano ^| findstr ":8080 " ^| findstr "LISTENING"') do (
    echo [ERROR] Port 8080 is still occupied by PID %%A
    echo [ERROR] Please stop that process and retry.
    exit /b 1
)

echo [INFO] Port 8080 released.
goto :eof
