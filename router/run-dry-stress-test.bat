@echo off
cd /d %~dp0
pxlblz-router.exe --config config\routes.loopback.json --pattern rainbow --duration 10s --dry-run
pause
