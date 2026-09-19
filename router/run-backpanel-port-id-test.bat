@echo off
setlocal
cd /d "%~dp0"
echo PXLBLZ Router - BACK_PANEL_249 seven-port ID test
pxlblz-router.exe --config config\routes.backpanel-port-id.json --pattern port-id --duration 12s
pause
