@echo off
setlocal
cd /d "%~dp0"
echo PXLBLZ Router - known installation dry-run
echo No UDP packets will be sent to hardware.
pxlblz-router.exe --config config\routes.installation-known.json --input ws --dry-run
