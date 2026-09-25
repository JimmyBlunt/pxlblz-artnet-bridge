@echo off
setlocal
cd /d "%~dp0"
echo PXLBLZ Router v0.3 - BACK_PANEL_249 panel-walk visual verification (G9)
echo.
echo Watch the white head: it must walk P1 - P2 - P3 - P4 - P5 - P6 - P7.
echo Bright port-coloured pixels mark the electrical START of every lane.
echo On Panel 6 the head must continue seamlessly from P6 (yellow) into P7 (white).
echo Ctrl-C to stop.
echo.
pxlblz-router.exe --config config\routes.backpanel-all.json --pattern panel-walk --walk-speed 4
pause
