@echo off
setlocal
cd /d "%~dp0"
echo.
echo ============================================================
echo  PXLBLZ Art-Net - known installation preflight
echo ============================================================
echo.
echo 1. Validate and print all routes
pxlblz-router.exe --config config\routes.installation-known.json --list-routes
if errorlevel 1 goto :fail
echo.
echo Route configuration validation PASS.
echo.
echo 2. Optional transport dry-run:
echo    Start PXLBLZ with ?pxout=1 in another window, then run:
echo.
echo    pxlblz-router.exe --config config\routes.installation-known.json --input ws --dry-run
echo.
echo No hardware traffic is sent by the command above.
exit /b 0
:fail
echo.
echo PRE-FLIGHT FAILED.
exit /b 1
