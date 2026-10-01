$ErrorActionPreference = 'Stop'

$FadecandyDir = $PSScriptRoot
$BinDir = Join-Path $FadecandyDir 'bin'
$Fcserver = Join-Path $BinDir 'fcserver.exe'
$Config = Join-Path $FadecandyDir 'config\fcserver-l3d.json'

# Pinned to the source revision used when this installer was authored.
$FcserverUrl = 'https://raw.githubusercontent.com/rewolff/fadecandy/de7f94a052579b3cea1dc81e17231be469f1603a/bin/fcserver.exe'
$ExpectedSize = 310286

New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

Write-Host 'Downloading Fadecandy fcserver.exe...'
Invoke-WebRequest -Uri $FcserverUrl -OutFile $Fcserver

$size = (Get-Item $Fcserver).Length
if ($size -ne $ExpectedSize) {
    throw "Unexpected fcserver.exe size: $size bytes (expected $ExpectedSize)."
}

Write-Host ''
Write-Host 'Installed:' -ForegroundColor Green
Write-Host "  $Fcserver"
Write-Host 'Config:'
Write-Host "  $Config"
Write-Host ''
Write-Host 'Start the server with:' -ForegroundColor Cyan
Write-Host "  & '$Fcserver' '$Config'"
Write-Host ''
Write-Host 'Then leave that window open and run fadecandy-probe.exe from bin\windows-x64.'
