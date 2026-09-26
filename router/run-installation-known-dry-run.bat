@echo off
setlocal
cd /d "%~dp0"
set "ROUTER=%~dp0..\bin\windows-x64\pxlblz-router.exe"
if not exist "%ROUTER%" set "ROUTER=%~dp0pxlblz-router.exe"
if not exist "%ROUTER%" goto :missing
echo PXLBLZ Router - known installation dry-run
echo No UDP packets will be sent to hardware.
"%ROUTER%" --config config\routes.installation-known.json --input ws --dry-run
exit /b %errorlevel%
:missing
echo ERROR: pxlblz-router.exe not found. Pull bin\windows-x64 or run build-windows.bat.
exit /b 1
