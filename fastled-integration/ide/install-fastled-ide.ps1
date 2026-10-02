param(
    [Parameter(Mandatory = $true)][string]$IdePath,
    [switch]$CheckOnly
)
# Applies the FastLED Pattern patch series (patches\*.patch, exported with
# git format-patch from feature/fastled) onto a PXLBLZ-IDE checkout that already
# carries the Art-Net external pixel output (src\engine\externalPixelOutput.ts).
#
# Idempotent: a checkout that already contains the series is left untouched.
# Uses git am --3way (commits, keeps authorship and messages) on a clean git
# checkout; never resets, stashes or deletes anything. On a conflict it stops,
# runs git am --abort and reports the patch that did not apply.
#
# Source files are only ever handled by git; this script itself reads files with
# [IO.File] (UTF-8 without BOM), never Get-Content/Set-Content.
$ErrorActionPreference = 'Stop'

function Fail([string]$message) { Write-Host ("FEHLER: " + $message) -ForegroundColor Red; exit 1 }
function Ok([string]$message) { Write-Host ("OK: " + $message) -ForegroundColor Green }
function ReadText([string]$path) { [IO.File]::ReadAllText($path, (New-Object Text.UTF8Encoding($false))) }
function Invoke-Git([string[]]$arguments) {
    $ErrorActionPreference = 'Continue'  # git writes progress to stderr; judge by exit code
    $output = & git.exe -C $IdePath @arguments 2>&1
    return @{ Code = $LASTEXITCODE; Text = (($output | ForEach-Object { "$_" }) -join "`n") }
}

$patchDir = Join-Path $PSScriptRoot 'patches'
$patches = @(Get-ChildItem -LiteralPath $patchDir -Filter '*.patch' | Sort-Object Name)
if ($patches.Count -eq 0) { Fail "Keine Patches in $patchDir gefunden." }
if (-not (Test-Path -LiteralPath $IdePath)) { Fail "IDE-Checkout nicht gefunden: $IdePath" }
$IdePath = (Resolve-Path -LiteralPath $IdePath).Path
if (-not (Get-Command git -ErrorAction SilentlyContinue)) { Fail 'git wurde nicht gefunden (PATH).' }

# Prerequisite: the Art-Net output branch (pxlblz-artnet-output).
$adapter = Join-Path $IdePath 'src\engine\externalPixelOutput.ts'
$preview = Join-Path $IdePath 'src\components\Preview.tsx'
if (-not (Test-Path -LiteralPath $adapter) -or -not (Test-Path -LiteralPath $preview) -or -not (ReadText $preview).Contains('createExternalPixelOutput')) {
    Fail 'Der Checkout enthaelt die Art-Net-Ausgabe (externalPixelOutput) nicht. Zuerst den Art-Net-Stand einspielen.'
}

# Already applied? Every file the series creates exists and the marker module is present.
$marker = Join-Path $IdePath 'src\engine\patternLanguage.ts'
$handle = Join-Path $IdePath 'src\engine\fastled\fastledPatternHandle.ts'
if ((Test-Path -LiteralPath $marker) -and (Test-Path -LiteralPath $handle) -and (ReadText $marker).Contains('@pxlblz-language')) {
    Ok "FastLED-Patterns sind in $IdePath bereits eingespielt - nichts zu tun."
    exit 0
}

$inside = Invoke-Git @('rev-parse', '--is-inside-work-tree')
if ($inside.Code -ne 0 -or $inside.Text.Trim() -ne 'true') { Fail "$IdePath ist kein git-Checkout." }
$status = Invoke-Git @('status', '--porcelain', '--untracked-files=no')
if ($status.Code -ne 0) { Fail $status.Text }
if ($status.Text.Trim() -ne '') { Fail "Der Checkout hat uncommittete Aenderungen. Bitte zuerst committen; es wird nichts verworfen.`n$($status.Text)" }
$amDir = Invoke-Git @('rev-parse', '--git-path', 'rebase-apply')
if ($amDir.Code -eq 0 -and (Test-Path -LiteralPath (Join-Path $IdePath $amDir.Text.Trim()))) { Fail 'Ein git am / rebase laeuft bereits in diesem Checkout.' }

if ($CheckOnly) { Ok "Voraussetzungen erfuellt; $($patches.Count) Patches wuerden eingespielt."; exit 0 }

$branch = (Invoke-Git @('rev-parse', '--abbrev-ref', 'HEAD')).Text.Trim()
$am = Invoke-Git (@('am', '--3way') + ($patches | ForEach-Object { $_.FullName }))
if ($am.Code -ne 0) {
    Write-Host $am.Text -ForegroundColor DarkGray
    $null = Invoke-Git @('am', '--abort')
    Fail 'Ein Patch liess sich nicht einspielen (Konflikt mit dem aktuellen IDE-Stand). git am wurde abgebrochen, der Checkout ist unveraendert. Siehe README.md "Nachziehen".'
}
Ok "$($patches.Count) FastLED-Commits auf '$branch' in $IdePath eingespielt."
Write-Host 'Danach: npm ci (falls noetig) und die IDE neu starten. Der FastLED-Compiler (fastled-integration, npm run service) muss laufen; der Launcher startet ihn.' -ForegroundColor DarkGray
