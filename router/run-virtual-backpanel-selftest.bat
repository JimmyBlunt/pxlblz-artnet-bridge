@echo off
setlocal
cd /d "%~dp0"
set "BINDIR=%~dp0..\bin\windows-x64"
set "VC=%BINDIR%\pxlblz-virtual-controller.exe"
set "ROUTER=%BINDIR%\pxlblz-router.exe"
set "SENDER=%BINDIR%\pxlblz-frame-sender.exe"
if not exist "%VC%" goto :missing
if not exist "%ROUTER%" goto :missing
if not exist "%SENDER%" goto :missing

set "SUMMARY=%TEMP%\pxlblz-virtual-backpanel-summary.json"
if exist "%SUMMARY%" del "%SUMMARY%"

start "Virtual BACK_PANEL" /b "%VC%" --config config\routes.backpanel-all.json --target-ip 10.0.0.253 --listen 127.0.0.1:6454 --web 127.0.0.1:9982 --duration 9s --summary-json "%SUMMARY%"
timeout /t 1 /nobreak >nul
start "Router to Virtual BACK_PANEL" /b "%ROUTER%" --config config\routes.backpanel-virtual.json --input ws --duration 7s
timeout /t 1 /nobreak >nul
"%SENDER%" --config config\routes.backpanel-virtual.json --ws 127.0.0.1:9980 --pattern port-id --fps 60 --duration 5s
timeout /t 4 /nobreak >nul

echo.
if exist "%SUMMARY%" (
  type "%SUMMARY%"
  echo.
  echo Virtual BACK_PANEL summary written to:
  echo %SUMMARY%
) else (
  echo ERROR: summary file was not created.
  exit /b 1
)
pause
exit /b 0

:missing
echo ERROR: required Windows binaries not found in bin\windows-x64.
exit /b 1
