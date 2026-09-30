param([switch]$CheckOnly, [switch]$NoOpen)
$ErrorActionPreference = 'Stop'
# Art-Net counterpart of windows-launcher/Start-PXLBLZ-Fadecandy.ps1 (tag fadecandy-v0.1.1):
# same two-IDE layout, same local login, the v0.3 Art-Net router instead of fcserver + bridge.
# Machine-specific paths come from launcher-config.json; browser state stays local.
$configPath = Join-Path $PSScriptRoot 'launcher-config.json'
if (-not (Test-Path -LiteralPath $configPath)) { throw 'Bitte zuerst Install-DesktopShortcut.ps1 ausfuehren.' }
$launcherConfig = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
$root = $launcherConfig.workspaceRoot
if (-not $root -or -not (Test-Path -LiteralPath $root)) { throw 'Der konfigurierte Workspace wurde nicht gefunden.' }
$main = Join-Path $root 'PXLBLZ-IDE-main'
$output = Join-Path $root 'PXLBLZ-IDE'
$artnet = Join-Path $root 'artnet'
$routerExe = Join-Path $artnet 'pxlblz-router.exe'
$routerConfig = Join-Path $artnet $launcherConfig.routerConfig
$wsPort = 9980
$logDir = $PSScriptRoot
$statusFile = Join-Path $logDir 'artnet-start-status.txt'
$mutex = New-Object System.Threading.Mutex($false, 'Local\PXLBLZ-ArtNet-Launcher')
$locked = $false
$report = New-Object System.Collections.Generic.List[string]
function Note([string]$message) { $report.Add($message); $report | Set-Content -LiteralPath $statusFile -Encoding UTF8; Write-Output $message }
function Listener([int]$port) {
    @(Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue | Where-Object { $_.LocalPort -eq $port })
}
function Identity([int]$port, [string]$folder) {
    try {
        $id = Invoke-RestMethod "http://localhost:$port/__identity" -TimeoutSec 4
        return $id.project -eq 'pxlblz-ide' -and $id.worktree.TrimEnd('\','/') -eq $folder.TrimEnd('\','/')
    } catch { return $false }
}
function Wait-Port([int]$port, $process) {
    $deadline = (Get-Date).AddSeconds(60)
    do {
        if (Listener $port) { return }
        if ($process.HasExited) { throw "Dienst auf Port $port wurde beendet. Details in $logDir." }
        Start-Sleep -Milliseconds 500
    } while ((Get-Date) -lt $deadline)
    throw "Dienst auf Port $port ist nicht bereit. Details in $logDir."
}
function Ensure-IDE([int]$port, [string]$folder, [string]$logName, [string]$proxy) {
    if (Listener $port) {
        if (-not (Identity $port $folder)) { throw "Port $port gehoert nicht zur erwarteten IDE-Version in $folder. Es wurde kein Prozess beendet." }
    } else {
        if ($CheckOnly) { throw "IDE auf Port $port ist nicht gestartet." }
        $vite = Join-Path $folder 'node_modules\vite\bin\vite.js'
        if (-not (Test-Path -LiteralPath $vite)) { throw "Vite fehlt: $vite (npm ci ausfuehren)" }
        $oldProxy = $env:VITE_API_PROXY_TARGET
        $oldPersistence = $env:VITE_CF_PERSIST_STATE
        $oldBase = $env:VITE_BASE_PATH
        $oldPort = $env:VITE_PORT
        try {
            $env:VITE_API_PROXY_TARGET = $proxy
            $env:VITE_CF_PERSIST_STATE = $null
            $env:VITE_BASE_PATH = '/PXLBLZ-IDE/'
            $env:VITE_PORT = "$port"
            $process = Start-Process -FilePath $node -ArgumentList ('"' + $vite + '" --host localhost --port ' + $port + ' --strictPort') -WorkingDirectory $folder -WindowStyle Hidden -RedirectStandardOutput (Join-Path $logDir "$logName.log") -RedirectStandardError (Join-Path $logDir "$logName-error.log") -PassThru
        } finally {
            $env:VITE_API_PROXY_TARGET = $oldProxy
            $env:VITE_CF_PERSIST_STATE = $oldPersistence
            $env:VITE_BASE_PATH = $oldBase
            $env:VITE_PORT = $oldPort
        }
        Wait-Port $port $process
        if (-not (Identity $port $folder)) { throw "IDE auf Port $port meldet eine falsche Version oder ist nicht bereit." }
    }
    Note "OK: IDE $port - $folder"
}
try {
    $locked = $mutex.WaitOne(0)
    if (-not $locked) { exit 0 }
    $node = (Get-Command node.exe -ErrorAction Stop).Source
    $version = & $node --version
    if ([int]($version.TrimStart('v').Split('.')[0]) -lt 24) { throw 'Dieser lokale Starter braucht Node.js 24 oder neuer.' }
    $adapter = Join-Path $output 'src\engine\externalPixelOutput.ts'
    $preview = Join-Path $output 'src\components\Preview.tsx'
    if (-not (Test-Path -LiteralPath $adapter) -or -not (Get-Content -LiteralPath $preview -Raw).Contains('createExternalPixelOutput')) { throw 'Die vorbereitete IDE-Version mit externer RGB-Ausgabe fehlt.' }
    Ensure-IDE 5174 $main 'server' ''
    Ensure-IDE 5175 $output 'output-ui' 'http://localhost:5174'
    $served = Invoke-WebRequest 'http://localhost:5175/PXLBLZ-IDE/src/components/Preview.tsx' -UseBasicParsing -TimeoutSec 60
    if (-not $served.Content.Contains('createExternalPixelOutput')) { throw 'Die laufende IDE liefert den Art-Net-Ausgabeadapter nicht aus.' }
    Note 'OK: Ausgelieferte IDE enthaelt den RGB-Ausgabeadapter.'

    if (-not (Test-Path -LiteralPath $routerExe)) { throw "Art-Net-Router fehlt: $routerExe" }
    if (-not (Test-Path -LiteralPath $routerConfig)) { throw "Router-Konfiguration fehlt: $routerConfig" }
    & $routerExe --config $routerConfig --list-routes | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Router-Konfiguration ist ungueltig: $routerConfig" }
    $routerListener = Listener $wsPort
    if (-not $routerListener) {
        if ($CheckOnly) { throw "Art-Net-Router auf Port $wsPort ist nicht gestartet." }
        $router = Start-Process -FilePath $routerExe -ArgumentList ('--config "' + $routerConfig + '" --input ws --ws-listen 127.0.0.1:' + $wsPort) -WorkingDirectory $artnet -WindowStyle Hidden -RedirectStandardOutput (Join-Path $logDir 'artnet-router.log') -RedirectStandardError (Join-Path $logDir 'artnet-router-error.log') -PassThru
        Wait-Port $wsPort $router
        $routerListener = Listener $wsPort
    }
    foreach ($ownerId in ($routerListener.OwningProcess | Select-Object -Unique)) {
        $owner = Get-CimInstance Win32_Process -Filter "ProcessId = $ownerId"
        if ($owner.ExecutablePath -ne $routerExe -or -not $owner.CommandLine.Contains($routerConfig) -or $owner.CommandLine -match '--dry-run|--input\s+pattern') { throw "Port $wsPort gehoert nicht zum erwarteten Art-Net-Router mit $routerConfig (PID $ownerId). Der bestehende Prozess bleibt erhalten." }
    }
    Note "OK: Art-Net-Router, WebSocket $wsPort, $($launcherConfig.routerConfig)"

    $loginHelper = Join-Path $logDir 'Open-Local-IDE.mjs'
    if ($CheckOnly -or $NoOpen) {
        & $node $loginHelper $main --check
    } else {
        $browserProcesses = @(Get-CimInstance Win32_Process | Where-Object { $_.Name -in 'chrome.exe','msedge.exe' })
        $profile = Join-Path $logDir 'BrowserProfile'
        $ids = @($browserProcesses | Where-Object { $_.CommandLine -like "*$profile*" } | Select-Object -ExpandProperty ProcessId)
        do {
            $oldCount = $ids.Count
            $ids = @($ids + @($browserProcesses | Where-Object { $_.ParentProcessId -in $ids } | Select-Object -ExpandProperty ProcessId) | Select-Object -Unique)
        } while ($ids.Count -gt $oldCount)
        $active = @(Get-NetTCPConnection -State Established -ErrorAction SilentlyContinue | Where-Object { $_.RemotePort -eq $wsPort -and $_.OwningProcess -in $ids })
        if ($active.Count -gt 0) {
            & $node (Join-Path $logDir 'Restore-IDEWindow.mjs') *> (Join-Path $logDir 'restore-window.log')
            if ($LASTEXITCODE -ne 0) { throw "Browserfenster konnte nicht geoeffnet werden. Siehe $logDir\restore-window.log" }
            Note 'Das vorhandene Browserprofil wurde geoeffnet; kein zweiter Ausgabesender gestartet.'
        } else {
            & $node $loginHelper $main *> (Join-Path $logDir 'local-login.log')
            if ($LASTEXITCODE -ne 0) { throw "Lokaler Login fehlgeschlagen. Siehe $logDir\local-login.log" }
            Note 'Studio mit lokalem Login und aktivierter Art-Net-Ausgabe geoeffnet.'
        }
    }
    if ($LASTEXITCODE -ne 0) { throw 'Die lokale Anmeldepruefung ist fehlgeschlagen.' }
    Note 'Art-Net-Startpruefung erfolgreich.'
} catch {
    $report.Add('FEHLER: ' + $_.Exception.Message)
    $report | Set-Content -LiteralPath $statusFile -Encoding UTF8
    if (-not $CheckOnly) {
        $notice = New-Object -ComObject WScript.Shell
        [void]$notice.Popup($_.Exception.Message, 15, 'PXLBLZ-IDE - ArtNet', 16)
    }
    Write-Error $_ -ErrorAction Continue
    exit 1
} finally {
    $report | Set-Content -LiteralPath $statusFile -Encoding UTF8
    if ($locked) { $mutex.ReleaseMutex() }
    $mutex.Dispose()
}
