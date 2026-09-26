@echo off
setlocal
cd /d "%~dp0"
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
pxlblz-router.exe --config config\routes.installation-known.json --pattern port-id --duration 15s
echo.
echo Expected nominal aggregate TX at 30 FPS:
echo   44 universes/frame
echo   1320 Art-Net packets/s
echo   0 send errors
echo.
pause
