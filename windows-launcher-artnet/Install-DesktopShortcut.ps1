param(
    [Parameter(Mandatory=$true)][string]$WorkspaceRoot,
    [Parameter(Mandatory=$true)][string]$RouterBinDirectory,
    [string]$RouterConfig = 'config\routes.esp-test-172.json',
    [string]$InstallDirectory = (Join-Path $env:LOCALAPPDATA 'PXLBLZ-IDE-ArtNet'),
    [string]$DesktopDirectory = [Environment]::GetFolderPath('Desktop'),
    [switch]$NoShortcut
)
$ErrorActionPreference = 'Stop'
# Art-Net counterpart of windows-launcher/Install-DesktopShortcut.ps1 (fadecandy-v0.1.1).
# Installs scripts, the router build and its configs only. It does not touch cookies,
# databases, secrets, a running IDE or hardware. Start the resulting shortcut when ready.
# Separate install directory, browser profile and mutex from the Fadecandy launcher.
$workspace = (Resolve-Path -LiteralPath $WorkspaceRoot).Path
foreach ($relative in @('PXLBLZ-IDE-main\package.json', 'PXLBLZ-IDE-main\.dev.vars', 'PXLBLZ-IDE\src\engine\externalPixelOutput.ts')) {
    if (-not (Test-Path -LiteralPath (Join-Path $workspace $relative))) { throw "Workspace unvollstaendig: $relative. Siehe windows-launcher-artnet/README.md." }
}
$bin = (Resolve-Path -LiteralPath $RouterBinDirectory).Path
$routerSource = Join-Path $bin 'pxlblz-router.exe'
if (-not (Test-Path -LiteralPath $routerSource)) { throw "pxlblz-router.exe fehlt in $bin" }
$artnet = Join-Path $workspace 'artnet'
New-Item -ItemType Directory -Path (Join-Path $artnet 'config') -Force | Out-Null
Copy-Item -LiteralPath $routerSource -Destination (Join-Path $artnet 'pxlblz-router.exe') -Force
Copy-Item -Path (Join-Path $PSScriptRoot '..\router\config\*.json') -Destination (Join-Path $artnet 'config') -Force
if (-not (Test-Path -LiteralPath (Join-Path $artnet $RouterConfig))) { throw "Router-Konfiguration fehlt: $RouterConfig" }
$source = if (Test-Path -LiteralPath (Join-Path $bin 'SOURCE_COMMIT.txt')) { (Get-Content -LiteralPath (Join-Path $bin 'SOURCE_COMMIT.txt') -TotalCount 1) } else { 'unbekannt' }
Set-Content -LiteralPath (Join-Path $artnet 'ROUTER_SOURCE.txt') -Value $source -Encoding UTF8

New-Item -ItemType Directory -Path $InstallDirectory -Force | Out-Null
foreach ($file in @('Start-PXLBLZ-ArtNet.ps1','Open-Local-IDE.mjs','Restore-IDEWindow.mjs')) {
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot $file) -Destination (Join-Path $InstallDirectory $file) -Force
}
@{version='artnet-v0.3-dev'; workspaceRoot=$workspace; routerConfig=$RouterConfig} | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $InstallDirectory 'launcher-config.json') -Encoding UTF8
if (-not $NoShortcut) {
    New-Item -ItemType Directory -Path $DesktopDirectory -Force | Out-Null
    $shortcutPath = Join-Path $DesktopDirectory 'PXLBLZ-IDE - ArtNet.lnk'
    $shell = New-Object -ComObject WScript.Shell
    $shortcut = $shell.CreateShortcut($shortcutPath)
    $shortcut.TargetPath = "$env:SystemRoot\System32\WindowsPowerShell\v1.0\powershell.exe"
    $shortcut.Arguments = '-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "' + (Join-Path $InstallDirectory 'Start-PXLBLZ-ArtNet.ps1') + '"'
    $shortcut.WorkingDirectory = $InstallDirectory
    $shortcut.Description = 'PXLBLZ Art-Net v0.3: IDE, lokalen Login und Art-Net-Router pruefen und starten'
    $shortcut.WindowStyle = 7
    $shortcut.IconLocation = "$env:SystemRoot\System32\shell32.dll,22"
    $shortcut.Save()
    Write-Output "Desktop-Verknuepfung: $shortcutPath"
}
Write-Output "Installiert: $InstallDirectory (Router-Konfiguration: $RouterConfig)"
