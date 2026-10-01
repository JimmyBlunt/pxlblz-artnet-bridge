$ErrorActionPreference = 'Stop'
$bridgeExe = Join-Path $PSScriptRoot 'pxlblz-fadecandy-0.1.1.exe'
$bridgeConfig = Join-Path $PSScriptRoot 'pxlblz-artnet-bridge\fadecandy\config\l3d-8x8x8.json'
$listener = Get-NetTCPConnection -LocalPort 9981 -State Listen -ErrorAction SilentlyContinue
if ($listener) {
    throw "Port 9981 is already in use by PID $($listener.OwningProcess). Stop the previous Fadecandy bridge before starting this version."
}
& $bridgeExe --config $bridgeConfig
