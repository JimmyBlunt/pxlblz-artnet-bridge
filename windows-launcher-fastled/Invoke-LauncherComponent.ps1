param(
    [Parameter(Mandatory=$true)][ValidateSet('Git','Api','Ide','Compiler','Router','Browser')][string]$Component,
    [ValidateSet('Check','Start')][string]$Action = 'Check'
)
# One launcher component of PXLBLZ-IDE~FastLED (same model as the Fadecandy starter).
# Exit codes: 0 = OK, 1 = problem (message on stdout), 2 = fatal for the rest (Git: workspace missing).
# The last stdout line is the status shown in the starter window. Nothing here deletes data or
# stops a process it did not start itself.
$ErrorActionPreference = 'Stop'
$logDir = $PSScriptRoot
$configPath = Join-Path $PSScriptRoot 'launcher-config.json'

function Listener([int]$port) {
    @(Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue | Where-Object { $_.LocalPort -eq $port })
}
function Identity([int]$port) {
    try { return Invoke-RestMethod "http://localhost:$port/__identity" -TimeoutSec 4 } catch { return $null }
}
function SameFolder([string]$a, [string]$b) {
    return [IO.Path]::GetFullPath($a).TrimEnd('\') -ieq [IO.Path]::GetFullPath($b).TrimEnd('\')
}
function Wait-Port([int]$port, $process, [int]$seconds = 60) {
    $deadline = (Get-Date).AddSeconds($seconds)
    do {
        if (Listener $port) { return }
        if ($process.HasExited) { throw "Dienst auf Port $port wurde beendet (Logs: $logDir)." }
        Start-Sleep -Milliseconds 500
    } while ((Get-Date) -lt $deadline)
    throw "Dienst auf Port $port ist nach $seconds s nicht bereit (Logs: $logDir)."
}
function Start-Vite([int]$port, [string]$folder, [string]$logName, [string]$proxy) {
    $vite = Join-Path $folder 'node_modules\vite\bin\vite.js'
    if (-not (Test-Path -LiteralPath $vite)) { throw "Vite fehlt in $folder (dort einmal: npm ci)" }
    $saved = @{}
    foreach ($name in 'VITE_API_PROXY_TARGET','VITE_CF_PERSIST_STATE','VITE_BASE_PATH','VITE_PORT') { $saved[$name] = [Environment]::GetEnvironmentVariable($name) }
    try {
        $env:VITE_API_PROXY_TARGET = $proxy
        $env:VITE_CF_PERSIST_STATE = $null
        $env:VITE_BASE_PATH = '/PXLBLZ-IDE/'
        $env:VITE_PORT = "$port"
        $process = Start-Process -FilePath $node -ArgumentList ('"' + $vite + '" --host localhost --port ' + $port + ' --strictPort') -WorkingDirectory $folder -WindowStyle Hidden -RedirectStandardOutput (Join-Path $logDir "$logName.log") -RedirectStandardError (Join-Path $logDir "$logName-error.log") -PassThru
    } finally {
        foreach ($name in $saved.Keys) { [Environment]::SetEnvironmentVariable($name, $saved[$name]) }
    }
    Wait-Port $port $process 90
}

try {
    if (-not (Test-Path -LiteralPath $configPath)) { Write-Output 'Starter nicht installiert (Install-DesktopShortcut.ps1 ausfuehren).'; exit 2 }
    $config = Get-Content -LiteralPath $configPath -Raw -Encoding UTF8 | ConvertFrom-Json
    $root = $config.workspaceRoot
    $main = Join-Path $root 'PXLBLZ-IDE-main'
    $ide = Join-Path $root $config.ideFolder
    $artnet = Join-Path $root 'artnet'
    $routerExe = Join-Path $artnet 'pxlblz-router.exe'
    $routerConfig = Join-Path $artnet $config.routerConfig
    $wsPort = 9980
    $fastledPort = 9996
    $node = (Get-Command node.exe -ErrorAction Stop).Source

    switch ($Component) {
        'Git' {
            foreach ($folder in @($main, $ide)) {
                if (-not (Test-Path -LiteralPath (Join-Path $folder 'package.json'))) { Write-Output "Workspace fehlt: $folder"; exit 2 }
            }
            if (-not (Test-Path -LiteralPath (Join-Path $main '.dev.vars'))) { Write-Output "Lokale Zugangsdaten fehlen: $main\.dev.vars"; exit 2 }
            $branch = (& git -C $ide rev-parse --abbrev-ref HEAD 2>$null)
            $head = (& git -C $ide log -1 --format=%h 2>$null)
            $marker = Join-Path $ide 'src\engine\patternLanguage.ts'
            if (-not (Test-Path -LiteralPath $marker)) { Write-Output "In $ide fehlt die FastLED-Integration (Branch: $branch)."; exit 2 }
            $dirty = @(& git -C $ide status --porcelain --untracked-files=no 2>$null).Count
            $note = if ($dirty -gt 0) { ", $dirty geaenderte Dateien" } else { '' }
            Write-Output "OK: $($config.ideFolder) auf $branch @ $head$note"
        }
        'Api' {
            if (Listener 5174) {
                $id = Identity 5174
                if (-not $id -or $id.project -ne 'pxlblz-ide' -or -not (SameFolder $id.worktree $main)) { throw 'Port 5174 gehoert nicht zu PXLBLZ-IDE-main. Es wurde kein Prozess beendet.' }
            } elseif ($Action -eq 'Start') {
                Start-Vite 5174 $main 'server' ''
            } else { throw 'API/Daten (Port 5174) laeuft nicht.' }
            Write-Output 'OK: API und lokale Daten auf Port 5174 (PXLBLZ-IDE-main)'
        }
        'Ide' {
            if (Listener 5175) {
                $id = Identity 5175
                if ($id -and $id.project -eq 'pxlblz-ide' -and -not (SameFolder $id.worktree $ide)) {
                    throw "Auf Port 5175 laeuft eine andere IDE ($($id.worktree)), z. B. vom Art-Net-Starter. Im Starter '5175 freigeben' waehlen."
                }
                if (-not $id -or -not (SameFolder $id.worktree $ide)) { throw 'Port 5175 ist von einem anderen Programm belegt. Es wurde kein Prozess beendet.' }
            } elseif ($Action -eq 'Start') {
                Start-Vite 5175 $ide 'ide-fastled' 'http://localhost:5174'
            } else { throw 'FastLED-IDE (Port 5175) laeuft nicht.' }
            $served = Invoke-WebRequest 'http://localhost:5175/PXLBLZ-IDE/src/components/Preview.tsx' -UseBasicParsing -TimeoutSec 90
            if (-not $served.Content.Contains('createExternalPixelOutput')) { throw 'Die laufende IDE liefert den Ausgabeadapter nicht aus.' }
            Write-Output "OK: FastLED-IDE auf Port 5175 ($($config.ideFolder))"
        }
        'Compiler' {
            $dir = $config.fastledIntegrationPath
            $healthy = { try { $h = Invoke-RestMethod "http://127.0.0.1:$fastledPort/health" -TimeoutSec 3; [bool]($h.ok -and $h.abiVersion) } catch { $false } }
            if (Listener $fastledPort) {
                if (-not (& $healthy)) { throw "Port $fastledPort ist belegt, antwortet aber nicht als FastLED-Compiler." }
            } elseif ($Action -eq 'Start') {
                $server = Join-Path $dir 'service\server.mjs'
                if (-not (Test-Path -LiteralPath $server)) { throw "server.mjs fehlt in $dir" }
                if (-not (Test-Path -LiteralPath (Join-Path $dir 'node_modules'))) { throw "node_modules fehlt in $dir (dort einmal: npm run setup)" }
                $process = Start-Process -FilePath $node -ArgumentList ('"' + $server + '" --port ' + $fastledPort) -WorkingDirectory $dir -WindowStyle Hidden -RedirectStandardOutput (Join-Path $logDir 'fastled-service.log') -RedirectStandardError (Join-Path $logDir 'fastled-service-error.log') -PassThru
                $deadline = (Get-Date).AddSeconds(45)
                do {
                    Start-Sleep -Milliseconds 500
                    if ($process.HasExited) { throw "FastLED-Compiler wurde beendet (fastled-service-error.log)." }
                } while (-not (& $healthy) -and (Get-Date) -lt $deadline)
                if (-not (& $healthy)) { throw 'FastLED-Compiler antwortet nicht.' }
            } else { throw "FastLED-Compiler (Port $fastledPort) laeuft nicht." }
            $v = Invoke-RestMethod "http://127.0.0.1:$fastledPort/version" -TimeoutSec 3
            $fl = if ($v.fastled.version) { $v.fastled.version } else { 'FastLED' }
            Write-Output "OK: FastLED-Compiler auf Port $fastledPort ($fl, zig $($v.zig))"
        }
        'Router' {
            if (-not (Test-Path -LiteralPath $routerExe)) { throw "Art-Net-Router fehlt: $routerExe (Art-Net-Starter einmal installieren)" }
            if (-not (Test-Path -LiteralPath $routerConfig)) { throw "Router-Konfiguration fehlt: $routerConfig" }
            $listener = Listener $wsPort
            if (-not $listener) {
                if ($Action -ne 'Start') { throw "Art-Net-Router (Port $wsPort) laeuft nicht." }
                & $routerExe --config $routerConfig --list-routes | Out-Null
                if ($LASTEXITCODE -ne 0) { throw "Router-Konfiguration ist ungueltig: $($config.routerConfig)" }
                $router = Start-Process -FilePath $routerExe -ArgumentList ('--config "' + $routerConfig + '" --input ws --ws-listen 127.0.0.1:' + $wsPort) -WorkingDirectory $artnet -WindowStyle Hidden -RedirectStandardOutput (Join-Path $logDir 'artnet-router.log') -RedirectStandardError (Join-Path $logDir 'artnet-router-error.log') -PassThru
                Wait-Port $wsPort $router
                $listener = Listener $wsPort
            }
            foreach ($ownerId in ($listener.OwningProcess | Select-Object -Unique)) {
                $owner = Get-CimInstance Win32_Process -Filter "ProcessId = $ownerId"
                if (-not (SameFolder $owner.ExecutablePath $routerExe)) { throw "Port $wsPort gehoert nicht zum Art-Net-Router (PID $ownerId). Es wurde kein Prozess beendet." }
                if (-not $owner.CommandLine.Contains($routerConfig)) { Write-Output "Hinweis: Router laeuft mit anderer Konfiguration (PID $ownerId)." }
            }
            Write-Output "OK: Art-Net-Router auf Port $wsPort ($($config.routerConfig)), Einstellungen http://127.0.0.1:9988/"
        }
        'Browser' {
            $helper = Join-Path $PSScriptRoot 'Open-Local-IDE.mjs'
            if ($Action -eq 'Check') {
                # cmd /c keeps node's stderr out of PowerShell 5.1's error stream (it would throw).
                $checkLog = Join-Path $logDir 'local-login-check.log'
                & cmd.exe /d /c "`"$node`" `"$helper`" --config `"$configPath`" --check > `"$checkLog`" 2>&1"
                $code = $LASTEXITCODE
                if ($code -ne 0) {
                    $lines = @(Get-Content -LiteralPath $checkLog -Encoding UTF8 | Where-Object { $_ -match 'Error|ECONNREFUSED|fehl|nicht' })
                    $reason = if ($lines -match 'ECONNREFUSED') { 'IDE/API nicht erreichbar' } elseif ($lines.Count) { $lines[0].Trim() } else { 'siehe local-login-check.log' }
                    throw "Lokaler Login: $reason"
                }
                Write-Output 'OK: lokaler Login (ohne Google/GitHub) bestaetigt'
            } else {
                $profile = Join-Path $PSScriptRoot 'BrowserProfile'
                $browserProcesses = @(Get-CimInstance Win32_Process | Where-Object { $_.Name -in 'chrome.exe','msedge.exe' })
                $ids = @($browserProcesses | Where-Object { $_.CommandLine -like "*$profile*" } | Select-Object -ExpandProperty ProcessId)
                do {
                    $before = $ids.Count
                    $ids = @($ids + @($browserProcesses | Where-Object { $_.ParentProcessId -in $ids } | Select-Object -ExpandProperty ProcessId) | Select-Object -Unique)
                } while ($ids.Count -gt $before)
                $senders = @(Get-NetTCPConnection -State Established -ErrorAction SilentlyContinue | Where-Object { $_.RemotePort -eq $wsPort })
                $mine = @($senders | Where-Object { $_.OwningProcess -in $ids })
                if ($mine.Count -gt 0) {
                    & cmd.exe /d /c "`"$node`" `"$(Join-Path $PSScriptRoot 'Restore-IDEWindow.mjs')`" > `"$(Join-Path $logDir 'restore-window.log')`" 2>&1"
                    if ($LASTEXITCODE -ne 0) { throw 'Vorhandenes Browserfenster konnte nicht geoeffnet werden (restore-window.log).' }
                    Write-Output 'OK: vorhandenes FastLED-Browserfenster geoeffnet (kein zweiter Sender)'
                } else {
                    & cmd.exe /d /c "`"$node`" `"$helper`" --config `"$configPath`" > `"$(Join-Path $logDir 'local-login.log')`" 2>&1"
                    if ($LASTEXITCODE -ne 0) { throw 'Lokaler Login fehlgeschlagen (local-login.log).' }
                    $others = $senders.Count - $mine.Count
                    $hint = if ($others -gt 0) { " - Achtung: ein anderes Fenster sendet bereits an den Router" } else { '' }
                    Write-Output "OK: Studio mit lokalem Login und Art-Net-Ausgabe geoeffnet$hint"
                }
            }
        }
    }
    exit 0
} catch {
    Write-Output ('FEHLER: ' + $_.Exception.Message)
    exit 1
}
