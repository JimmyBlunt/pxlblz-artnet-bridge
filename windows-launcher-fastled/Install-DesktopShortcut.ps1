param(
    [Parameter(Mandatory=$true)][string]$WorkspaceRoot,
    # IDE checkout (folder name under the workspace) with the FastLED integration.
    [string]$IdeFolder = 'PXLBLZ-IDE-fastled',
    # Folder of fastled-integration (compile service with node_modules). Default: <workspace>\fastled-work.
    [string]$FastLedIntegrationPath,
    # Router config used when the starter has to start the Art-Net router (relative to <workspace>\artnet).
    [string]$RouterConfig,
    [string]$OutputUrl = 'ws://127.0.0.1:9980/pixels',
    # Default on E: next to the workspace: the browser profile grows and drive C: is often full.
    [string]$InstallDirectory,
    [string]$DesktopDirectory = [Environment]::GetFolderPath('Desktop'),
    [switch]$NoShortcut
)
$ErrorActionPreference = 'Stop'
# Installs only scripts, launcher-config.json and shortcuts. It does not touch the router
# binary/configs (the Art-Net installer owns them), databases, secrets or running services.
# Re-running keeps earlier choices that are not given again; the browser profile is kept.
$workspace = (Resolve-Path -LiteralPath $WorkspaceRoot).Path
if (-not $InstallDirectory) { $InstallDirectory = Join-Path $workspace 'launcher-fastled' }
$configPath = Join-Path $InstallDirectory 'launcher-config.json'
$previous = if (Test-Path -LiteralPath $configPath) { Get-Content -LiteralPath $configPath -Raw -Encoding UTF8 | ConvertFrom-Json } else { $null }
if (-not $PSBoundParameters.ContainsKey('RouterConfig')) {
    $RouterConfig = if ($previous.routerConfig) { $previous.routerConfig } else {
        $artnetConfig = Join-Path $env:LOCALAPPDATA 'PXLBLZ-IDE-ArtNet\launcher-config.json'
        if (Test-Path -LiteralPath $artnetConfig) { (Get-Content -LiteralPath $artnetConfig -Raw -Encoding UTF8 | ConvertFrom-Json).routerConfig } else { 'config\routes.installation-live.json' }
    }
}
if (-not $PSBoundParameters.ContainsKey('FastLedIntegrationPath')) {
    $FastLedIntegrationPath = if ($previous.fastledIntegrationPath) { $previous.fastledIntegrationPath } else { Join-Path $workspace 'fastled-work' }
}
foreach ($required in @((Join-Path $workspace 'PXLBLZ-IDE-main\.dev.vars'), (Join-Path $workspace "$IdeFolder\package.json"), (Join-Path $FastLedIntegrationPath 'service\server.mjs'), (Join-Path $workspace "artnet\$RouterConfig"))) {
    if (-not (Test-Path -LiteralPath $required)) { throw "Fehlt: $required" }
}

New-Item -ItemType Directory -Path $InstallDirectory -Force | Out-Null
foreach ($file in 'Start-PXLBLZ-FastLED.ps1','Show-PXLBLZ-Launcher.ps1','Invoke-LauncherComponent.ps1','Open-Local-IDE.mjs','Restore-IDEWindow.mjs') {
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot $file) -Destination (Join-Path $InstallDirectory $file) -Force
}
$config = [ordered]@{
    version = 'fastled-v0.1'
    workspaceRoot = $workspace
    ideFolder = $IdeFolder
    fastledIntegrationPath = $FastLedIntegrationPath
    routerConfig = $RouterConfig
    outputUrl = $OutputUrl
}
[IO.File]::WriteAllText($configPath, ($config | ConvertTo-Json), (New-Object Text.UTF8Encoding($false)))
Write-Output "Installiert: $InstallDirectory"

if (-not $NoShortcut) {
    $shell = New-Object -ComObject WScript.Shell
    $startMenu = Join-Path ([Environment]::GetFolderPath('Programs')) 'PXLBLZ-IDE FastLED'
    foreach ($dir in @($DesktopDirectory, $startMenu)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
        $shortcutPath = Join-Path $dir 'PXLBLZ-IDE - FastLED.lnk'
        $shortcut = $shell.CreateShortcut($shortcutPath)
        $shortcut.TargetPath = "$env:SystemRoot\System32\WindowsPowerShell\v1.0\powershell.exe"
        $shortcut.Arguments = '-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "' + (Join-Path $InstallDirectory 'Start-PXLBLZ-FastLED.ps1') + '"'
        $shortcut.WorkingDirectory = $InstallDirectory
        $shortcut.Description = 'PXLBLZ-IDE~FastLED: IDE mit FastLED, Compiler, Art-Net-Router und lokalem Login starten'
        $shortcut.WindowStyle = 7
        $shortcut.IconLocation = "$env:SystemRoot\System32\shell32.dll,43"
        $shortcut.Save()
        Write-Output "Verknuepfung: $shortcutPath"
    }
}
