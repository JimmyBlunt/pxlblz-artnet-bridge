@echo off
setlocal
cd /d "%~dp0"
echo PXLBLZ Router v0.2.0 - BACK_PANEL_249 external WebSocket frame test
start "PXLBLZ Router WS - BACK_PANEL" cmd /k "pxlblz-router.exe --config config\routes.backpanel-port-id.json --input ws --duration 15s"
timeout /t 1 /nobreak >nul
pxlblz-frame-sender.exe --config config\routes.backpanel-port-id.json --ws 127.0.0.1:9980 --pattern port-id --fps 60 --duration 12s
pause
