@echo off
setlocal
cd /d "%~dp0"
set "ROUTER=%~dp0..\bin\windows-x64\pxlblz-router.exe"
if not exist "%ROUTER%" set "ROUTER=%~dp0pxlblz-router.exe"
if not exist "%ROUTER%" goto :missing
echo.
echo ============================================================
echo  PXLBLZ Art-Net - known installation preflight
echo ============================================================
echo.
echo 1. Validate and print all routes
"%ROUTER%" --config config\routes.installation-known.json --list-routes
if errorlevel 1 goto :fail
echo.
echo Route configuration validation PASS.
echo.
echo 2. Optional transport dry-run:
echo    Start PXLBLZ with ?pxout=1 in another window, then run:
echo.
echo    run-installation-known-dry-run.bat
echo.
echo No hardware traffic is sent by the command above.
exit /b 0
:missing
echo ERROR: pxlblz-router.exe not found. Pull bin\windows-x64 or run build-windows.bat.
exit /b 1
:fail
echo.
echo PRE-FLIGHT FAILED.
exit /b 1
