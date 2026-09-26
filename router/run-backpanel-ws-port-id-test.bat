@echo off
setlocal
cd /d "%~dp0"
set "BINDIR=%~dp0..\bin\windows-x64"
set "ROUTER=%BINDIR%\pxlblz-router.exe"
set "SENDER=%BINDIR%\pxlblz-frame-sender.exe"
if not exist "%ROUTER%" set "ROUTER=%~dp0pxlblz-router.exe"
if not exist "%SENDER%" set "SENDER=%~dp0pxlblz-frame-sender.exe"
if not exist "%ROUTER%" goto :missing
if not exist "%SENDER%" goto :missing
echo PXLBLZ Router - BACK_PANEL_249 external WebSocket frame test
start "PXLBLZ Router WS - BACK_PANEL" cmd /k ""%ROUTER%" --config config\routes.backpanel-port-id.json --input ws --duration 15s"
timeout /t 1 /nobreak >nul
"%SENDER%" --config config\routes.backpanel-port-id.json --ws 127.0.0.1:9980 --pattern port-id --fps 60 --duration 12s
pause
exit /b %errorlevel%
:missing
echo ERROR: required Windows binaries not found. Pull bin\windows-x64 or run build-windows.bat.
exit /b 1
