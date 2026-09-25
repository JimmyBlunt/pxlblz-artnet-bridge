@echo off
setlocal
cd /d "%~dp0"
echo PXLBLZ Router v0.3 - three-controller loopback (60/30/30 fps, blackout after 1 s)
start "Art-Net Listener" cmd /k "artnet-listener.exe"
timeout /t 1 /nobreak >nul
start "PXLBLZ Router multi" cmd /k "pxlblz-router.exe --config config\routes.multi-loopback.json --input ws --duration 15s"
timeout /t 1 /nobreak >nul
pxlblz-frame-sender.exe --config config\routes.multi-loopback.json --ws 127.0.0.1:9980 --pattern rainbow --fps 60 --duration 8s
echo Sender stopped - router should now report STALE/blackout per controller.
pause
