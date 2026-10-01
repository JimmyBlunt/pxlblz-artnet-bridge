param([switch]$CheckOnly, [switch]$NoOpen)
$ErrorActionPreference = 'Stop'
# Machine-specific paths are installed separately; browser state stays local.
$configPath = Join-Path $PSScriptRoot 'launcher-config.json'
if (-not (Test-Path -LiteralPath $configPath)) { throw 'Bitte zuerst Install-DesktopShortcut.ps1 ausfuehren.' }
$launcherConfig = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
$root = $launcherConfig.workspaceRoot
if (-not $root -or -not (Test-Path -LiteralPath $root)) { throw 'Der konfigurierte Workspace wurde nicht gefunden.' }
$main = Join-Path $root 'PXLBLZ-IDE-main'
$output = Join-Path $root 'PXLBLZ-IDE'
$cube = Join-Path $root '2-FadeCandy-'
$logDir = $PSScriptRoot
$mutex = New-Object System.Threading.Mutex($false, 'Local\PXLBLZ-Fadecandy-Launcher')
$locked = $false
$report = New-Object System.Collections.Generic.List[string]
function Note([string]$message) { $report.Add($message); $report | Set-Content -LiteralPath (Join-Path $logDir 'fadecandy-start-status.txt') -Encoding UTF8; Write-Output $message }
function Listener([int]$port) {
    @(Get-NetTCPConnection -State Listen -ErrorAction Stop | Where-Object { $_.LocalPort -eq $port })
}
function Identity([int]$port, [string]$folder) {
    try {
        $id = Invoke-RestMethod "http://localhost:$port/__identity" -TimeoutSec 4
        return $id.project -eq 'pxlblz-ide' -and $id.worktree.TrimEnd('\','/') -eq $folder.TrimEnd('\','/')
    } catch { return $false }
}
function Wait-Port([int]$port, $process) {
    $deadline = (Get-Date).AddSeconds(30)
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
        if (-not (Test-Path -LiteralPath $vite)) { throw "Vite fehlt: $vite" }
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
    $served = Invoke-WebRequest 'http://localhost:5175/PXLBLZ-IDE/src/components/Preview.tsx' -UseBasicParsing -TimeoutSec 30
    if (-not $served.Content.Contains('createExternalPixelOutput')) { throw 'Die laufende IDE liefert den Fadecandy-Ausgabeadapter nicht aus.' }
    Note 'OK: Ausgelieferte IDE enthaelt den RGB-Ausgabeadapter.'
    if (-not (Listener 7890)) {
        if ($CheckOnly) { throw 'fcserver auf Port 7890 ist nicht gestartet.' }
        $fc = Start-Process -FilePath (Join-Path $cube 'fcserver.exe') -ArgumentList ('"' + (Join-Path $cube 'pxlblz-artnet-bridge\fadecandy\config\fcserver-l3d.json') + '"') -WorkingDirectory $cube -WindowStyle Hidden -RedirectStandardOutput (Join-Path $logDir 'fcserver.log') -RedirectStandardError (Join-Path $logDir 'fcserver-error.log') -PassThru
        Wait-Port 7890 $fc
    }
    & $node (Join-Path $logDir 'Check-Fadecandy.mjs') $output
    if ($LASTEXITCODE -ne 0) { throw 'fcserver meldet keinen angeschlossenen Fadecandy-Controller. USB-Verbindung pruefen.' }
    Note 'OK: fcserver und angeschlossener Fadecandy-Controller.'
    $bridgeExe = Join-Path $cube 'pxlblz-fadecandy-0.1.1.exe'
    $bridgeConfig = Join-Path $cube 'pxlblz-artnet-bridge\fadecandy\config\l3d-8x8x8.json'
    $config = Get-Content -LiteralPath $bridgeConfig -Raw | ConvertFrom-Json
    if ($config.input.ws_listen -ne '127.0.0.1:9981' -or $config.input.pixel_count -ne 512 -or $config.fadecandy.address -ne '127.0.0.1:7890' -or ($config.mapping.dimensions -join ',') -ne '8,8,8') { throw 'Die L3D-Konfiguration passt nicht zum erwarteten 8x8x8-Cube.' }
    $bridgeListener = Listener 9981
    if (-not $bridgeListener) {
        if ($CheckOnly) { throw 'Fadecandy-Bridge auf Port 9981 ist nicht gestartet.' }
        $bridge = Start-Process -FilePath $bridgeExe -ArgumentList ('--config "' + $bridgeConfig + '"') -WorkingDirectory $cube -WindowStyle Hidden -RedirectStandardOutput (Join-Path $logDir 'fadecandy.log') -RedirectStandardError (Join-Path $logDir 'fadecandy-error.log') -PassThru
        Wait-Port 9981 $bridge
        $bridgeListener = Listener 9981
    }
    foreach ($ownerId in ($bridgeListener.OwningProcess | Select-Object -Unique)) {
        $owner = Get-CimInstance Win32_Process -Filter "ProcessId = $ownerId"
        if ($owner.ExecutablePath -ne $bridgeExe -or -not $owner.CommandLine.Contains($bridgeConfig) -or $owner.CommandLine -match '--dry-run|--input\s+pattern') { throw "Port 9981 gehoert nicht zur erwarteten Fadecandy-Bridge 0.1.1 mit L3D-Konfiguration (PID $ownerId). Der bestehende Prozess bleibt erhalten." }
    }
    Note 'OK: Fadecandy-Bridge 0.1.1, L3D 8x8x8, WebSocket 9981 -> OPC 7890.'
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
        $active = @(Get-NetTCPConnection -State Established -ErrorAction SilentlyContinue | Where-Object { $_.RemotePort -eq 9981 -and $_.OwningProcess -in $ids })
        if ($active.Count -gt 0) {
            # Let the browser reveal its own profile; a temporary helper tab
            # closes itself without loading another output-enabled IDE.
            & $node (Join-Path $logDir 'Restore-IDEWindow.mjs') *> (Join-Path $logDir 'restore-window.log')
            if ($LASTEXITCODE -ne 0) { throw "Browserfenster konnte nicht geoeffnet werden. Siehe $logDir\restore-window.log" }
            Note 'Das vorhandene Browserprofil wurde geoeffnet; kein zweiter Ausgabesender gestartet.'
        } else {
            & $node $loginHelper $main *> (Join-Path $logDir 'local-login.log')
            if ($LASTEXITCODE -ne 0) { throw "Lokaler Login fehlgeschlagen. Siehe $logDir\local-login.log" }
            Note 'Studio mit lokalem Login und aktivierter Fadecandy-Ausgabe geoeffnet.'
        }
    }
    if ($LASTEXITCODE -ne 0) { throw 'Die lokale Anmeldepruefung ist fehlgeschlagen.' }
    Note 'Fadecandy-Startpruefung erfolgreich.'
} catch {
    $report.Add('FEHLER: ' + $_.Exception.Message)
    $report | Set-Content -LiteralPath (Join-Path $logDir 'fadecandy-start-status.txt') -Encoding UTF8
    if (-not $CheckOnly) {
        # Bound the dialog lifetime so an unseen error cannot lock future starts.
        $notice = New-Object -ComObject WScript.Shell
        [void]$notice.Popup($_.Exception.Message, 15, 'PXLBLZ-IDE - Fadecandy', 16)
    }
    Write-Error $_ -ErrorAction Continue
    exit 1
} finally {
    $report | Set-Content -LiteralPath (Join-Path $logDir 'fadecandy-start-status.txt') -Encoding UTF8
    if ($locked) { $mutex.ReleaseMutex() }
    $mutex.Dispose()
}
