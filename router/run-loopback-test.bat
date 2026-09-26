@echo off
setlocal
cd /d "%~dp0"
set "BINDIR=%~dp0..\bin\windows-x64"
set "ROUTER=%BINDIR%\pxlblz-router.exe"
set "LISTENER=%BINDIR%\artnet-listener.exe"
if not exist "%ROUTER%" set "ROUTER=%~dp0pxlblz-router.exe"
if not exist "%LISTENER%" set "LISTENER=%~dp0artnet-listener.exe"
if not exist "%ROUTER%" goto :missing
if not exist "%LISTENER%" goto :missing
start "PXLBLZ Art-Net Listener" cmd /k ""%LISTENER%" --bind 127.0.0.1:6454"
timeout /t 1 /nobreak >nul
"%ROUTER%" --config config\routes.loopback.json --pattern rainbow --duration 10s
pause
exit /b %errorlevel%
:missing
echo ERROR: required Windows binaries not found. Pull bin\windows-x64 or run build-windows.bat.
exit /b 1
