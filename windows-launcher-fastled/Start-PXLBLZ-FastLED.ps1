param([switch]$CheckOnly, [switch]$NoOpen)
$ErrorActionPreference = 'Stop'
# Desktop entry of PXLBLZ-IDE~FastLED (same model as the Fadecandy starter):
# without switches the starter window opens; -CheckOnly / -NoOpen run headless for tests.
try {
    if ($CheckOnly -or $NoOpen) {
        $result = 0
        foreach ($component in 'Git','Api','Ide','Compiler','Router','Browser') {
            $action = if ($CheckOnly -or $component -eq 'Browser') { 'Check' } else { 'Start' }
            & "$env:SystemRoot\System32\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot 'Invoke-LauncherComponent.ps1') -Component $component -Action $action
            if ($LASTEXITCODE -ne 0) { $result = 1 }
            if ($component -eq 'Git' -and $LASTEXITCODE -eq 2) { break }
        }
        exit $result
    }
    & (Join-Path $PSScriptRoot 'Show-PXLBLZ-Launcher.ps1')
} catch {
    $_ | Out-String | Set-Content -LiteralPath (Join-Path $PSScriptRoot 'launcher-fatal.log') -Encoding UTF8
    Add-Type -AssemblyName System.Windows.Forms
    [void][System.Windows.Forms.MessageBox]::Show($_.Exception.Message, 'PXLBLZ-IDE~FastLED - Fehler')
    exit 1
}
