@echo off
setlocal
cd /d "%~dp0"
set "VC=%~dp0..\bin\windows-x64\pxlblz-virtual-controller.exe"
if not exist "%VC%" set "VC=%~dp0pxlblz-virtual-controller.exe"
if not exist "%VC%" goto :missing

echo.
echo ============================================================
echo  PXLBLZ Virtual BACK_PANEL_249 Controller
echo ============================================================
echo.
echo UDP Art-Net: 127.0.0.1:6454
echo Visualizer:  http://127.0.0.1:9982/
echo Profile:     7 outputs / 4105 LEDs / 29 universes
echo Protocol:    Teensy runtime_receiver + run-policy emulator
echo.
start "" "http://127.0.0.1:9982/"
"%VC%" --config config\routes.backpanel-virtual.json --target-ip 127.0.0.1 --listen 127.0.0.1:6454 --web 127.0.0.1:9982
exit /b %errorlevel%

:missing
echo ERROR: pxlblz-virtual-controller.exe not found.
echo Pull bin\windows-x64 or run build-windows.bat.
exit /b 1
