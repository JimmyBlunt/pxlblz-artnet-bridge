# Starter window for PXLBLZ-IDE~FastLED: one row per component, started in order.
# Each component runs in its own PowerShell process (Invoke-LauncherComponent.ps1), so the
# window stays responsive and a failing component shows its message instead of hanging.
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
[System.Windows.Forms.Application]::EnableVisualStyles()

$psExe = "$env:SystemRoot\System32\WindowsPowerShell\v1.0\powershell.exe"
$componentScript = Join-Path $PSScriptRoot 'Invoke-LauncherComponent.ps1'
$components = @(
    @{ Key = 'Git';      Label = 'Workspace / Git' },
    @{ Key = 'Api';      Label = 'API + lokale Daten (5174)' },
    @{ Key = 'Ide';      Label = 'FastLED-IDE (5175)' },
    @{ Key = 'Compiler'; Label = 'FastLED-Compiler (9996)' },
    @{ Key = 'Router';   Label = 'Art-Net-Router (9980)' },
    @{ Key = 'Browser';  Label = 'Browser + lokaler Login' }
)
$mutex = New-Object System.Threading.Mutex($false, 'Local\PXLBLZ-FastLED-Launcher')
if (-not $mutex.WaitOne(0)) {
    [void][System.Windows.Forms.MessageBox]::Show('Der FastLED-Starter ist bereits geoeffnet.', 'PXLBLZ-IDE~FastLED')
    exit 0
}
try { Add-Content -LiteralPath (Join-Path $PSScriptRoot 'launch-history.log') -Value ("{0}  Fenster geoeffnet" -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss')) -Encoding UTF8 } catch {}

$form = New-Object System.Windows.Forms.Form
$form.Text = 'PXLBLZ-IDE~FastLED'
$form.StartPosition = 'CenterScreen'
$form.Size = New-Object System.Drawing.Size(760, 430)
$form.MinimumSize = New-Object System.Drawing.Size(640, 380)
$form.Font = New-Object System.Drawing.Font('Segoe UI', 9)
try { $form.Icon = [System.Drawing.Icon]::ExtractAssociatedIcon("$env:SystemRoot\System32\shell32.dll") } catch {}

$title = New-Object System.Windows.Forms.Label
$title.Text = 'PXLBLZ-IDE mit FastLED - Ausgabe ueber den Art-Net-Router'
$title.Font = New-Object System.Drawing.Font('Segoe UI', 11, [System.Drawing.FontStyle]::Bold)
$title.AutoSize = $true
$title.Location = New-Object System.Drawing.Point(14, 12)
$form.Controls.Add($title)

$list = New-Object System.Windows.Forms.ListView
$list.View = 'Details'
$list.FullRowSelect = $true
$list.HeaderStyle = 'Nonclickable'
$list.Location = New-Object System.Drawing.Point(14, 44)
$list.Size = New-Object System.Drawing.Size(716, 250)
$list.Anchor = 'Top,Left,Right,Bottom'
[void]$list.Columns.Add('Komponente', 190)
[void]$list.Columns.Add('Status', 90)
[void]$list.Columns.Add('Meldung', 420)
foreach ($c in $components) {
    $item = New-Object System.Windows.Forms.ListViewItem($c.Label)
    [void]$item.SubItems.Add('-')
    [void]$item.SubItems.Add('')
    [void]$list.Items.Add($item)
}
$form.Controls.Add($list)

$status = New-Object System.Windows.Forms.Label
$status.AutoSize = $false
$status.Location = New-Object System.Drawing.Point(14, 300)
$status.Size = New-Object System.Drawing.Size(716, 22)
$status.Anchor = 'Left,Right,Bottom'
$status.Text = 'Bereit.'
$form.Controls.Add($status)

function New-Button([string]$text, [int]$x) {
    $b = New-Object System.Windows.Forms.Button
    $b.Text = $text
    $b.Location = New-Object System.Drawing.Point($x, 330)
    $b.Size = New-Object System.Drawing.Size(130, 32)
    $b.Anchor = 'Left,Bottom'
    $form.Controls.Add($b)
    return $b
}
$startButton = New-Button 'Starten' 14
$checkButton = New-Button 'Nur pruefen' 152
$freeButton = New-Button '5175 freigeben' 290
$logsButton = New-Button 'Logs oeffnen' 428
$settingsButton = New-Button 'Router-Einstellungen' 566
$startButton.Font = New-Object System.Drawing.Font('Segoe UI', 9, [System.Drawing.FontStyle]::Bold)
$form.AcceptButton = $startButton

$colors = @{ OK = [System.Drawing.Color]::FromArgb(0, 128, 0); FEHLER = [System.Drawing.Color]::FromArgb(190, 0, 0); WARTEN = [System.Drawing.Color]::FromArgb(160, 110, 0); AUS = [System.Drawing.Color]::Gray }
function Set-Row([int]$index, [string]$state, [string]$message) {
    $item = $list.Items[$index]
    $item.SubItems[1].Text = $state
    $item.SubItems[2].Text = $message
    $key = if ($colors.ContainsKey($state)) { $state } else { 'WARTEN' }
    $item.ForeColor = $colors[$key]
    [System.Windows.Forms.Application]::DoEvents()
}
function Set-Busy([bool]$busy) {
    foreach ($b in $startButton, $checkButton, $freeButton) { $b.Enabled = -not $busy }
    $form.Cursor = if ($busy) { [System.Windows.Forms.Cursors]::WaitCursor } else { [System.Windows.Forms.Cursors]::Default }
}

# Runs one component without blocking the UI thread; returns @{Code; Message}.
function Invoke-Component([string]$key, [string]$action) {
    $out = [IO.Path]::GetTempFileName()
    $p = Start-Process -FilePath $psExe -ArgumentList @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', ('"' + $componentScript + '"'), '-Component', $key, '-Action', $action) -WindowStyle Hidden -RedirectStandardOutput $out -PassThru
    $null = $p.Handle   # Windows PowerShell 5.1 only keeps ExitCode if the handle was opened while running
    while (-not $p.HasExited) { [System.Windows.Forms.Application]::DoEvents(); Start-Sleep -Milliseconds 100 }
    $p.WaitForExit()
    $lines = @(Get-Content -LiteralPath $out -Encoding UTF8 -ErrorAction SilentlyContinue | Where-Object { $_.Trim() })
    Remove-Item -LiteralPath $out -ErrorAction SilentlyContinue
    $message = if ($lines.Count) { $lines[-1] } else { '(keine Ausgabe)' }
    return @{ Code = $p.ExitCode; Message = $message; All = $lines }
}

function Run-All([string]$mode) {
    Set-Busy $true
    $ok = $true
    try {
        for ($i = 0; $i -lt $components.Count; $i++) { Set-Row $i '-' '' }
        for ($i = 0; $i -lt $components.Count; $i++) {
            $key = $components[$i].Key
            $action = if ($mode -eq 'Check') { 'Check' } else { 'Start' }
            Set-Row $i 'WARTEN' ($(if ($action -eq 'Start') { 'wird gestartet ...' } else { 'wird geprueft ...' }))
            $status.Text = "$($components[$i].Label) ..."
            $r = Invoke-Component $key $action
            $text = ($r.Message -replace '^(OK|FEHLER): ', '')
            if ($r.Code -eq 0) {
                Set-Row $i 'OK' $text
            } else {
                $ok = $false
                Set-Row $i 'FEHLER' $text
                if ($r.Code -eq 2 -or $key -in 'Api', 'Ide') {
                    for ($j = $i + 1; $j -lt $components.Count; $j++) { Set-Row $j 'AUS' 'uebersprungen' }
                    break
                }
            }
        }
        if ($ok) {
            $status.Text = if ($mode -eq 'Check') { 'Pruefung erfolgreich.' } else { 'Fertig: Studio ist offen. Dieses Fenster kann geschlossen werden.' }
        } else {
            $status.Text = 'Es gab Probleme - Details in der Liste bzw. unter "Logs oeffnen".'
        }
        Add-Content -LiteralPath (Join-Path $PSScriptRoot 'launch-history.log') -Value ("{0}  {1}: {2}" -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $mode, $(if ($ok) { 'OK' } else { 'Fehler' })) -Encoding UTF8
    } finally {
        Set-Busy $false
    }
}

$startButton.Add_Click({ Run-All 'Start' })
$checkButton.Add_Click({ Run-All 'Check' })
$logsButton.Add_Click({ Start-Process explorer.exe $PSScriptRoot })
$settingsButton.Add_Click({ Start-Process 'http://127.0.0.1:9988/' })
$freeButton.Add_Click({
    # Stops only a Vite dev server of another PXLBLZ-IDE checkout on 5175 (e.g. the Art-Net starter),
    # after confirmation. Never touches data, the API on 5174 or unrelated programs.
    $owners = @(Get-NetTCPConnection -State Listen -LocalPort 5175 -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess -Unique)
    if (-not $owners.Count) { [void][System.Windows.Forms.MessageBox]::Show('Port 5175 ist frei.', 'PXLBLZ-IDE~FastLED'); return }
    try { $id = Invoke-RestMethod 'http://localhost:5175/__identity' -TimeoutSec 4 } catch { $id = $null }
    if (-not $id -or $id.project -ne 'pxlblz-ide') { [void][System.Windows.Forms.MessageBox]::Show('Port 5175 gehoert keinem PXLBLZ-IDE-Server. Es wird nichts beendet.', 'PXLBLZ-IDE~FastLED'); return }
    $answer = [System.Windows.Forms.MessageBox]::Show("Auf Port 5175 laeuft die IDE aus`n$($id.worktree)`n`nDiesen IDE-Server beenden? (Daten bleiben unveraendert; ungespeicherte Aenderungen in einem offenen Browser-Tab dorthin speichern!)", 'PXLBLZ-IDE~FastLED', 'YesNo', 'Question')
    if ($answer -ne 'Yes') { return }
    foreach ($ownerPid in $owners) {
        $proc = Get-CimInstance Win32_Process -Filter "ProcessId = $ownerPid"
        if ($proc.Name -eq 'node.exe' -and $proc.CommandLine -like '*vite*') { Stop-Process -Id $ownerPid -Confirm:$false }
    }
    $status.Text = 'Port 5175 freigegeben. Jetzt "Starten".'
})

$form.Add_Shown({ $form.Activate(); Run-All 'Start' })
try { [void]$form.ShowDialog() } finally { $mutex.ReleaseMutex(); $mutex.Dispose() }
