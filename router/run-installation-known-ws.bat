@echo off
setlocal
cd /d "%~dp0"
echo PXLBLZ Router - known installation controllers
echo.
echo Targets:
echo   10.0.0.244  WS2812_NODE
echo   10.0.0.253  BACK_PANEL_249
echo   10.0.0.251  PANEL8_251
echo.
echo APA102 controller is intentionally NOT included until its route is confirmed.
echo.
pxlblz-router.exe --config config\routes.installation-known.json --input ws
