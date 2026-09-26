@echo off
setlocal
cd /d "%~dp0"
set "BINDIR=%~dp0..\bin\windows-x64"
set "VC=%BINDIR%\pxlblz-virtual-controller.exe"
set "ROUTER=%BINDIR%\pxlblz-router.exe"
if not exist "%VC%" set "VC=%~dp0pxlblz-virtual-controller.exe"
if not exist "%ROUTER%" set "ROUTER=%~dp0pxlblz-router.exe"
if not exist "%VC%" goto :missing
if not exist "%ROUTER%" goto :missing

echo.
echo Starting virtual BACK_PANEL receiver...
start "PXLBLZ Virtual BACK_PANEL" cmd /k ""%VC%" --config config\routes.backpanel-virtual.json --target-ip 127.0.0.1 --listen 127.0.0.1:6454 --web 127.0.0.1:9982"
timeout /t 1 /nobreak >nul

echo Starting PXLBLZ router against the virtual receiver...
start "PXLBLZ Router to Virtual BACK_PANEL" cmd /k ""%ROUTER%" --config config\routes.backpanel-virtual.json --input ws"
timeout /t 1 /nobreak >nul

start "" "http://127.0.0.1:9982/"

echo.
echo Virtual hardware path is ready:
echo.
echo   PXLBLZ
echo     ^| WebSocket 127.0.0.1:9980
echo   pxlblz-router
echo     ^| Art-Net UDP 127.0.0.1:6454
echo   virtual Teensy receiver
echo     ^| browser visualization
echo   http://127.0.0.1:9982/
echo.
echo Enable OUT in PXLBLZ. No physical controller will receive traffic.
echo.
pause
exit /b 0

:missing
echo ERROR: required Windows binaries not found.
echo Pull bin\windows-x64 or run build-windows.bat.
exit /b 1
