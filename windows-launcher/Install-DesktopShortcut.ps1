param(
    [Parameter(Mandatory=$true)][string]$WorkspaceRoot,
    [string]$InstallDirectory = (Join-Path $env:LOCALAPPDATA 'PXLBLZ-IDE'),
    [string]$DesktopDirectory = [Environment]::GetFolderPath('Desktop'),
    [switch]$NoShortcut
)
$ErrorActionPreference = 'Stop'
# Installs scripts and configuration only. It does not touch cookies, databases,
# secrets, the running IDE, or hardware. Start the resulting shortcut when ready.
$workspace = (Resolve-Path -LiteralPath $WorkspaceRoot).Path
foreach ($relative in @('PXLBLZ-IDE-main\package.json', 'PXLBLZ-IDE\src\engine\externalPixelOutput.ts', '2-FadeCandy-\pxlblz-artnet-bridge\fadecandy\config\l3d-8x8x8.json')) {
    if (-not (Test-Path -LiteralPath (Join-Path $workspace $relative))) { throw "Workspace unvollstaendig: $relative. Siehe windows-launcher/README.md." }
}
$bridgeSource = Join-Path $PSScriptRoot '..\bin\windows-x64\pxlblz-fadecandy-0.1.1.exe'
$bridgeTarget = Join-Path $workspace '2-FadeCandy-\pxlblz-fadecandy-0.1.1.exe'
if (-not (Test-Path -LiteralPath $bridgeTarget)) {
    Copy-Item -LiteralPath $bridgeSource -Destination $bridgeTarget
} elseif ((Get-FileHash -LiteralPath $bridgeTarget).Hash -ne (Get-FileHash -LiteralPath $bridgeSource).Hash) {
    throw 'Die vorhandene Bridge 0.1.1 hat einen anderen Hash. Sie wurde nicht ueberschrieben.'
}
$fcTarget = Join-Path $workspace '2-FadeCandy-\fcserver.exe'
$fcSource = Join-Path $PSScriptRoot '..\fadecandy\bin\fcserver.exe'
if (-not (Test-Path -LiteralPath $fcTarget)) {
    if (-not (Test-Path -LiteralPath $fcSource)) { throw 'fcserver fehlt. Zuerst fadecandy/install-fcserver.ps1 ausfuehren.' }
    Copy-Item -LiteralPath $fcSource -Destination $fcTarget
}
$manifest = Get-Content -LiteralPath (Join-Path $PSScriptRoot '..\pxlblz-integration\snapshots\fadecandy-v0.1.1\manifest.json') -Raw | ConvertFrom-Json
if ((Get-FileHash -LiteralPath $fcTarget).Hash -ne $manifest.fcserver.sha256) { throw 'fcserver entspricht nicht der gesicherten Version.' }
New-Item -ItemType Directory -Path $InstallDirectory -Force | Out-Null
foreach ($file in @('Start-PXLBLZ-Fadecandy.ps1','Start-PXLBLZ-IDE.ps1','Open-Local-IDE.mjs','Check-Fadecandy.mjs','Restore-IDEWindow.mjs')) {
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot $file) -Destination (Join-Path $InstallDirectory $file) -Force
}
@{version='fadecandy-v0.1.1'; workspaceRoot=$workspace} | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $InstallDirectory 'launcher-config.json') -Encoding UTF8
if (-not $NoShortcut) {
    New-Item -ItemType Directory -Path $DesktopDirectory -Force | Out-Null
    $shortcutPath = Join-Path $DesktopDirectory 'PXLBLZ-IDE - Fadecandy.lnk'
    $shell = New-Object -ComObject WScript.Shell
    $shortcut = $shell.CreateShortcut($shortcutPath)
    $shortcut.TargetPath = "$env:SystemRoot\System32\WindowsPowerShell\v1.0\powershell.exe"
    $shortcut.Arguments = '-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "' + (Join-Path $InstallDirectory 'Start-PXLBLZ-Fadecandy.ps1') + '"'
    $shortcut.WorkingDirectory = $InstallDirectory
    $shortcut.Description = 'PXLBLZ Fadecandy v0.1.1: IDE, lokalen Login, USB-Controller und Bridge pruefen und starten'
    $shortcut.WindowStyle = 7
    $shortcut.IconLocation = "$env:SystemRoot\System32\shell32.dll,22"
    $shortcut.Save()
    Write-Output "Desktop-Verknuepfung: $shortcutPath"
}
Write-Output "Installiert: $InstallDirectory"
