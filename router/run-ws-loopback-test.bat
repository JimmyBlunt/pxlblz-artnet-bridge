@echo off
setlocal
cd /d "%~dp0"
set "BINDIR=%~dp0..\bin\windows-x64"
set "ROUTER=%BINDIR%\pxlblz-router.exe"
set "LISTENER=%BINDIR%\artnet-listener.exe"
set "SENDER=%BINDIR%\pxlblz-frame-sender.exe"
if not exist "%ROUTER%" set "ROUTER=%~dp0pxlblz-router.exe"
if not exist "%LISTENER%" set "LISTENER=%~dp0artnet-listener.exe"
if not exist "%SENDER%" set "SENDER=%~dp0pxlblz-frame-sender.exe"
if not exist "%ROUTER%" goto :missing
if not exist "%LISTENER%" goto :missing
if not exist "%SENDER%" goto :missing
start "Art-Net Listener" cmd /k ""%LISTENER%" --bind 127.0.0.1:6454"
timeout /t 1 /nobreak >nul
start "PXLBLZ Router WS" cmd /k ""%ROUTER%" --config config\routes.loopback.json --input ws --duration 13s"
timeout /t 1 /nobreak >nul
"%SENDER%" --config config\routes.loopback.json --ws 127.0.0.1:9980 --pattern rainbow --fps 60 --duration 10s
pause
exit /b %errorlevel%
:missing
echo ERROR: required Windows binaries not found. Pull bin\windows-x64 or run build-windows.bat.
exit /b 1
