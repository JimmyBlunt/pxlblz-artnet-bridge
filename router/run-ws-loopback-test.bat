@echo off
setlocal
cd /d "%~dp0"
start "Art-Net Listener" cmd /k "artnet-listener.exe"
timeout /t 1 /nobreak >nul
start "PXLBLZ Router WS" cmd /k "pxlblz-router.exe --config config\routes.loopback.json --input ws --duration 13s"
timeout /t 1 /nobreak >nul
pxlblz-frame-sender.exe --config config\routes.loopback.json --ws 127.0.0.1:9980 --pattern rainbow --fps 60 --duration 10s
pause
