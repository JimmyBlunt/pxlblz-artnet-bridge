@echo off
cd /d %~dp0
echo Start artnet-listener.exe in Terminal 1.
echo Then run pxlblz-router.exe --config config\routes.loopback.json --pattern rainbow --duration 10s
echo Expected: U0-U47 and about 2880 ArtDmx packets/sec at 60 FPS.
