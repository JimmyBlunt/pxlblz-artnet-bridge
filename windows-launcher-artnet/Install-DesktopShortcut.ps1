param(
    [Parameter(Mandatory=$true)][string]$WorkspaceRoot,
    [Parameter(Mandatory=$true)][string]$RouterBinDirectory,
    # Router config the starter uses. Omitted on a re-install: the previous choice is kept.
    [string]$RouterConfig,
    # Folder of fastled-integration (FastLED compile service). Omitted: previous choice, else <workspace>astled-integration.
    [string]$FastLedIntegrationPath,
    [string]$InstallDirectory = (Join-Path $env:LOCALAPPDATA 'PXLBLZ-IDE-ArtNet'),
    [string]$DesktopDirectory = [Environment]::GetFolderPath('Desktop'),
    [switch]$NoShortcut,
    # Replace workspace configs that differ from the repository (old file goes to config\backups).
    [switch]$OverwriteConfigs
)
$ErrorActionPreference = 'Stop'
# Art-Net counterpart of windows-launcher/Install-DesktopShortcut.ps1 (fadecandy-v0.1.1).
# Installs scripts, the router build and its configs only. It does not touch cookies,
# databases, secrets, the running IDE or hardware. Start the resulting shortcut when ready.
# Separate install directory, browser profile and mutex from the Fadecandy launcher.
$workspace = (Resolve-Path -LiteralPath $WorkspaceRoot).Path
foreach ($relative in @('PXLBLZ-IDE-main\package.json', 'PXLBLZ-IDE-main\.dev.vars', 'PXLBLZ-IDE\src\engine\externalPixelOutput.ts')) {
    if (-not (Test-Path -LiteralPath (Join-Path $workspace $relative))) { throw "Workspace unvollstaendig: $relative. Siehe windows-launcher-artnet/README.md." }
}
$bin = (Resolve-Path -LiteralPath $RouterBinDirectory).Path
$routerSource = Join-Path $bin 'pxlblz-router.exe'
if (-not (Test-Path -LiteralPath $routerSource)) { throw "pxlblz-router.exe fehlt in $bin" }
$artnet = Join-Path $workspace 'artnet'
$configDir = Join-Path $artnet 'config'
New-Item -ItemType Directory -Path $configDir -Force | Out-Null

# Keep the router config choice of an earlier installation unless a new one is given.
$launcherConfigPath = Join-Path $InstallDirectory 'launcher-config.json'
if (-not $PSBoundParameters.ContainsKey('RouterConfig')) {
    $RouterConfig = 'config\routes.esp-test-172-8x8-12x6.json'
    if (Test-Path -LiteralPath $launcherConfigPath) {
        $previous = (Get-Content -LiteralPath $launcherConfigPath -Raw | ConvertFrom-Json).routerConfig
        if ($previous) { $RouterConfig = $previous }
    }
}

# FastLED compile service location (optional; the starter only warns if it is missing).
if (-not $PSBoundParameters.ContainsKey('FastLedIntegrationPath')) {
    $FastLedIntegrationPath = ''
    if (Test-Path -LiteralPath $launcherConfigPath) {
        $previousConfig = Get-Content -LiteralPath $launcherConfigPath -Raw | ConvertFrom-Json
        if ($previousConfig.PSObject.Properties['fastledIntegrationPath']) { $FastLedIntegrationPath = $previousConfig.fastledIntegrationPath }
    }
}

# A running router locks its exe. Stop only this workspace's router; the starter restarts it.
$routerTarget = Join-Path $artnet 'pxlblz-router.exe'
Get-CimInstance Win32_Process -Filter "Name='pxlblz-router.exe'" | Where-Object { $_.ExecutablePath -eq $routerTarget } | ForEach-Object {
    Stop-Process -Id $_.ProcessId -Confirm:$false
    Write-Output "Laufenden Router beendet (PID $($_.ProcessId)) - der Starter startet ihn neu."
    Start-Sleep -Milliseconds 500
}
Copy-Item -LiteralPath $routerSource -Destination $routerTarget -Force

# Configs: the workspace copy is the one the router and its web page edit.
# New repository configs are added; existing ones are never silently replaced.
foreach ($repoFile in Get-ChildItem -Path (Join-Path $PSScriptRoot '..\router\config') -Filter '*.json') {
    $target = Join-Path $configDir $repoFile.Name
    if (-not (Test-Path -LiteralPath $target)) {
        Copy-Item -LiteralPath $repoFile.FullName -Destination $target
        Write-Output "Config neu: $($repoFile.Name)"
    } elseif ((Get-FileHash -LiteralPath $target).Hash -ne (Get-FileHash -LiteralPath $repoFile.FullName).Hash) {
        if ($OverwriteConfigs) {
            $backupDir = Join-Path $configDir 'backups'
            New-Item -ItemType Directory -Path $backupDir -Force | Out-Null
            $backup = Join-Path $backupDir ($repoFile.BaseName + '.' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '.json')
            Copy-Item -LiteralPath $target -Destination $backup
            Copy-Item -LiteralPath $repoFile.FullName -Destination $target -Force
            Write-Output "Config ersetzt: $($repoFile.Name) (alte Fassung: $backup)"
        } else {
            Write-Output "Config behalten (weicht vom Repo ab): $($repoFile.Name)"
        }
    }
}
if (-not (Test-Path -LiteralPath (Join-Path $artnet $RouterConfig))) { throw "Router-Konfiguration fehlt: $RouterConfig" }
$source = if (Test-Path -LiteralPath (Join-Path $bin 'SOURCE_COMMIT.txt')) { (Get-Content -LiteralPath (Join-Path $bin 'SOURCE_COMMIT.txt') -TotalCount 1) } else { 'unbekannt' }
Set-Content -LiteralPath (Join-Path $artnet 'ROUTER_SOURCE.txt') -Value $source -Encoding UTF8

New-Item -ItemType Directory -Path $InstallDirectory -Force | Out-Null
foreach ($file in @('Start-PXLBLZ-ArtNet.ps1','Open-Local-IDE.mjs','Restore-IDEWindow.mjs')) {
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot $file) -Destination (Join-Path $InstallDirectory $file) -Force
}
@{version='artnet-v0.3-dev'; workspaceRoot=$workspace; routerConfig=$RouterConfig; fastledIntegrationPath=$FastLedIntegrationPath} | ConvertTo-Json | Set-Content -LiteralPath $launcherConfigPath -Encoding UTF8
if (-not $NoShortcut) {
    # Desktop and Start menu: the starter (visible progress window, closes itself
    # on success, stays open with the error otherwise) and the configuration page.
    $startMenu = Join-Path ([Environment]::GetFolderPath('Programs')) 'PXLBLZ-IDE ArtNet'
    foreach ($dir in @($DesktopDirectory, $startMenu)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
        $shortcutPath = Join-Path $dir 'PXLBLZ-IDE - ArtNet.lnk'
        $shell = New-Object -ComObject WScript.Shell
        $shortcut = $shell.CreateShortcut($shortcutPath)
        $shortcut.TargetPath = "$env:SystemRoot\System32\WindowsPowerShell\v1.0\powershell.exe"
        $shortcut.Arguments = '-NoProfile -ExecutionPolicy Bypass -File "' + (Join-Path $InstallDirectory 'Start-PXLBLZ-ArtNet.ps1') + '" -Interactive'
        $shortcut.WorkingDirectory = $InstallDirectory
        $shortcut.Description = 'PXLBLZ-IDE~ArtNet: IDE, lokalen Login und Art-Net-Router pruefen und starten'
        $shortcut.WindowStyle = 1
        $shortcut.IconLocation = "$env:SystemRoot\System32\shell32.dll,22"
        $shortcut.Save()
        Write-Output "Verknuepfung: $shortcutPath"
        $urlPath = Join-Path $dir 'PXLBLZ-ArtNet Einstellungen.url'
        Set-Content -LiteralPath $urlPath -Value "[InternetShortcut]`r`nURL=http://127.0.0.1:9988/`r`nIconFile=$env:SystemRoot\System32\shell32.dll`r`nIconIndex=21" -Encoding ASCII
        Write-Output "Verknuepfung: $urlPath"
    }
}
Write-Output "Installiert: $InstallDirectory (Router-Konfiguration: $RouterConfig)"
