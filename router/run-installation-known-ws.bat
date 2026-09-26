@echo off
setlocal
cd /d "%~dp0"
set "ROUTER=%~dp0..\bin\windows-x64\pxlblz-router.exe"
if not exist "%ROUTER%" set "ROUTER=%~dp0pxlblz-router.exe"
if not exist "%ROUTER%" goto :missing
echo PXLBLZ Router - known installation controllers
echo.
echo Targets:
echo   10.0.0.244  WS2812_NODE
echo   10.0.0.253  BACK_PANEL_249
echo   10.0.0.251  PANEL8_251
echo.
echo APA102 controller is intentionally NOT included until its route is confirmed.
echo.
"%ROUTER%" --config config\routes.installation-known.json --input ws
exit /b %errorlevel%
:missing
echo ERROR: pxlblz-router.exe not found. Pull bin\windows-x64 or run build-windows.bat.
exit /b 1
