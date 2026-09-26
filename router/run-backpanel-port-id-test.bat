@echo off
setlocal
cd /d "%~dp0"
set "ROUTER=%~dp0..\bin\windows-x64\pxlblz-router.exe"
if not exist "%ROUTER%" set "ROUTER=%~dp0pxlblz-router.exe"
if not exist "%ROUTER%" goto :missing
echo PXLBLZ Router - BACK_PANEL_249 seven-port ID test
"%ROUTER%" --config config\routes.backpanel-port-id.json --pattern port-id --duration 12s
pause
exit /b %errorlevel%
:missing
echo ERROR: pxlblz-router.exe not found. Pull bin\windows-x64 or run build-windows.bat.
exit /b 1
