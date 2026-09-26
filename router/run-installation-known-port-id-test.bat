@echo off
setlocal
cd /d "%~dp0"
set "ROUTER=%~dp0..\bin\windows-x64\pxlblz-router.exe"
if not exist "%ROUTER%" set "ROUTER=%~dp0pxlblz-router.exe"
if not exist "%ROUTER%" goto :missing
echo.
echo ============================================================
echo  PXLBLZ Art-Net - known installation safe port-ID test
echo ============================================================
echo.
echo This WILL send Art-Net to:
echo   10.0.0.244  WS2812_NODE
echo   10.0.0.253  BACK_PANEL_249
echo   10.0.0.251  PANEL8_251
echo.
echo APA102 is not included because its route is not confirmed yet.
echo Only the small route ID bar / moving marker is lit per output.
echo.
pause
"%ROUTER%" --config config\routes.installation-known.json --pattern port-id --duration 15s
echo.
echo Expected nominal aggregate TX at 30 FPS:
echo   44 universes/frame
echo   1320 Art-Net packets/s
echo   0 send errors
echo.
pause
exit /b %errorlevel%
:missing
echo ERROR: pxlblz-router.exe not found. Pull bin\windows-x64 or run build-windows.bat.
exit /b 1
