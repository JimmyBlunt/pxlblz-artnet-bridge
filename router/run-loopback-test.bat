@echo off
cd /d %~dp0
start "PXLBLZ Art-Net Listener" cmd /k "%~dp0artnet-listener.exe --bind 127.0.0.1:6454"
timeout /t 1 /nobreak >nul
pxlblz-router.exe --config config\routes.loopback.json --pattern rainbow --duration 10s
pause
