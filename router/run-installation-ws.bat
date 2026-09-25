@echo off
setlocal
cd /d "%~dp0"
echo PXLBLZ Router v0.3 - FULL installation (.244 / .251 / .253) with live WebSocket input
echo .244 and .251 routes are NOT yet hardware verified - see docs\V0.3_MULTI_CONTROLLER.md
echo Status JSON: http://127.0.0.1:9981/status
pxlblz-router.exe --config config\routes.installation-full.json --input ws
pause
