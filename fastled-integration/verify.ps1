# Rebuilds what is needed and runs the golden-frame verification (writes RESULTS.md).
# Usage: .\verify.ps1 [-Frames 1200] [-Only "Fire2012,Blink"]
param([int]$Frames = 1200, [string]$Only = '')
$args2 = @('tests/verify.mjs', '--frames', $Frames)
if ($Only) { $args2 += @('--only', $Only) }
Push-Location $PSScriptRoot
try { & node @args2; exit $LASTEXITCODE } finally { Pop-Location }
