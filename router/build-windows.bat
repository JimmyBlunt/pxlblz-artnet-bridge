@echo off
setlocal
if not exist dist\windows mkdir dist\windows
set GOOS=windows
set GOARCH=amd64
go test ./...
if errorlevel 1 exit /b 1
go build -trimpath -ldflags "-s -w" -o dist\windows\pxlblz-router.exe .\cmd\pxlblz-router
if errorlevel 1 exit /b 1
go build -trimpath -ldflags "-s -w" -o dist\windows\artnet-listener.exe .\cmd\artnet-listener
if errorlevel 1 exit /b 1
go build -trimpath -ldflags "-s -w" -o dist\windows\pxlblz-frame-sender.exe .\cmd\pxlblz-frame-sender
if errorlevel 1 exit /b 1
copy /Y dist\windows\pxlblz-router.exe pxlblz-router.exe >nul
copy /Y dist\windows\artnet-listener.exe artnet-listener.exe >nul
copy /Y dist\windows\artnet-probe.exe artnet-probe.exe >nul
copy /Y dist\windows\pxlblz-frame-sender.exe pxlblz-frame-sender.exe >nul
echo Windows x64 binaries built.
