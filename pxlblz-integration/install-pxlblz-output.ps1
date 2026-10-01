param(
  [Parameter(Mandatory=$true)]
  [string]$PxlblzPath,

  # Output daemon the printed Studio URL points at. The adapter itself is the
  # same for every target; the target only selects ?pxoutUrl=.
  [ValidateSet("artnet","fadecandy","custom")]
  [string]$Target = "artnet",

  [string]$CustomUrl = ""
)

$ErrorActionPreference = "Stop"

# Windows PowerShell 5.1: Get-Content -Raw reads BOM-less files as ANSI and
# Set-Content -Encoding utf8 writes a BOM, which turns UTF-8 characters in the
# PXLBLZ sources into mojibake. Always read and write UTF-8 without BOM.
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
function Read-Utf8([string]$Path) { [System.IO.File]::ReadAllText($Path, $utf8NoBom) }
function Write-Utf8([string]$Path, [string]$Text) { [System.IO.File]::WriteAllText($Path, $Text, $utf8NoBom) }

$root = (Resolve-Path $PxlblzPath).Path
$preview = Join-Path $root "src\components\Preview.tsx"
$engineDir = Join-Path $root "src\engine"
$componentsDir = Join-Path $root "src\components"
$moduleDest = Join-Path $engineDir "externalPixelOutput.ts"
$moduleSource = Join-Path $PSScriptRoot "src\externalPixelOutput.ts"
$toggleDest = Join-Path $componentsDir "ExternalPixelOutputToggle.tsx"
$toggleSource = Join-Path $PSScriptRoot "src\ExternalPixelOutputToggle.tsx"
$previewDeck = Join-Path $componentsDir "PreviewDeck.tsx"

if (!(Test-Path $preview)) {
  throw "Preview.tsx not found: $preview"
}
if (!(Test-Path $moduleSource)) {
  throw "Integration module not found: $moduleSource"
}
if (!(Test-Path $toggleSource)) {
  throw "Integration toggle component not found: $toggleSource"
}
if (!(Test-Path $previewDeck)) {
  throw "PreviewDeck.tsx not found: $previewDeck"
}

switch ($Target) {
  "artnet"    { $outputUrl = "ws://127.0.0.1:9980/pixels" }
  "fadecandy" { $outputUrl = "ws://127.0.0.1:9981/pixels" }
  "custom" {
    if ([string]::IsNullOrWhiteSpace($CustomUrl)) {
      throw "-CustomUrl is required when -Target custom is selected."
    }
    $outputUrl = $CustomUrl.Trim()
  }
}

$text = Read-Utf8 $preview
$deckText = Read-Utf8 $previewDeck
# Keep each file's own line endings for inserted lines.
$nl = if ($text.Contains("`r`n")) { "`r`n" } else { "`n" }
$deckNl = if ($deckText.Contains("`r`n")) { "`r`n" } else { "`n" }

$importAnchor = "import { createVirtualClock } from '@/engine/virtualClock'"
$layoutAnchor = "    const { mapPoints, pixelCount, draw } = layout"
$deckImportAnchor = "import { CompactBrightness } from './CompactBrightness'"
$deckButtonAnchor = "      <CompactBrightness onSpace={toggle} value={brightness} onChange={v => { setBrightness(v); writeCascadedOverride('brightness', v) }} />"

# Only anchors that survive an installation are checked up front, so the
# installer can be re-run on an already patched checkout.
foreach ($anchor in @($importAnchor, $layoutAnchor)) {
  if (!$text.Contains($anchor)) {
    throw "Expected PXLBLZ source anchor not found. No changes made: $anchor"
  }
}
foreach ($anchor in @($deckImportAnchor, $deckButtonAnchor)) {
  if (!$deckText.Contains($anchor)) {
    throw "Expected PreviewDeck source anchor not found. No changes made: $anchor"
  }
}

$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$backup = "$preview.pxout-backup-$stamp"
$deckBackup = "$previewDeck.pxout-backup-$stamp"
Copy-Item $preview $backup
Copy-Item $previewDeck $deckBackup
Copy-Item $moduleSource $moduleDest -Force
Copy-Item $toggleSource $toggleDest -Force

if (!$text.Contains("createExternalPixelOutput")) {
  $text = $text.Replace(
    $importAnchor,
    "$importAnchor${nl}import { createExternalPixelOutput } from '@/engine/externalPixelOutput'"
  )
}

if (!$text.Contains("pixelCountCap === null ? createExternalPixelOutput(pixelCount)")) {
  $text = $text.Replace(
    $layoutAnchor,
    "$layoutAnchor${nl}    const externalPixelOutput =${nl}      pixelCountCap === null ? createExternalPixelOutput(pixelCount) : null"
  )
}

if (!$text.Contains("const paintPacked = externalPixelOutput?.enabled")) {
  $paintPattern = '(?ms)(    const paint = \(pixels: \[number, number, number\]\[\], brightness: number, dimmed: boolean\) => \{.*?^    \})\r?\n\r?\n(    const loop = createRenderLoop\(\{)'
  $paintInsert = (@'
$1

    const paintPacked = externalPixelOutput?.enabled
      ? (frame: Float64Array, brightness: number, dimmed: boolean) => {
          if (positions3D) {
            const view = useCameraStore.getState()
            renderer.setCamera(captureCameraRef.current ?? view.camera)
            renderer.setZoom(view.zoom)
          }
          renderer.paint(frame, brightness, dimmed)
          captureRef.current.afterPaint(canvasRef.current)
          externalPixelOutput.sendPacked(frame)
        }
      : undefined

$2
'@ -replace "`r?`n", $nl).TrimEnd("`r", "`n")
  $next = [regex]::Replace($text, $paintPattern, $paintInsert, 1)
  if ($next -eq $text) {
    throw "Paint integration anchor not found. Backup exists at $backup"
  }
  $text = $next
}

if (!$text.Contains("      paintPacked,")) {
  $paintArgPattern = '(?m)^      paint,(?=\r?$)'
  $next = [regex]::Replace($text, $paintArgPattern, "      paint,${nl}      paintPacked,", 1)
  if ($next -eq $text) {
    throw "Render-loop paint argument anchor not found. Backup exists at $backup"
  }
  $text = $next
}

if (!$text.Contains("externalPixelOutput?.close()")) {
  $cleanupPattern = '(?m)^    return \(\) => loop\.stop\(\)(?=\r?$)'
  $cleanupReplacement = "    return () => {${nl}      loop.stop()${nl}      externalPixelOutput?.close()${nl}    }"
  $next = [regex]::Replace($text, $cleanupPattern, $cleanupReplacement, 1)
  if ($next -eq $text) {
    throw "Render-loop cleanup anchor not found. Backup exists at $backup"
  }
  $text = $next
}

$text = $text.TrimEnd("`r", "`n") + $nl

if (!$deckText.Contains("ExternalPixelOutputToggle")) {
  $deckText = $deckText.Replace(
    $deckImportAnchor,
    "$deckImportAnchor${deckNl}import { ExternalPixelOutputToggle } from './ExternalPixelOutputToggle'"
  )
}

if (!$deckText.Contains("<ExternalPixelOutputToggle />")) {
  $deckText = $deckText.Replace(
    $deckButtonAnchor,
    "      <ExternalPixelOutputToggle />${deckNl}$deckButtonAnchor"
  )
}

$deckText = $deckText.TrimEnd("`r", "`n") + $deckNl
Write-Utf8 $preview $text
Write-Utf8 $previewDeck $deckText

# Custom maps authored as a literal coordinate array get a fixed pixel count
# (= their point count), like PXLBLZ's stock literal maps. Without it a pattern
# keeps an old persisted count when such a map is selected and the router
# rejects every frame. Idempotent: skipped when already applied.
$fixedCountPatch = Join-Path $PSScriptRoot 'patches\custom-map-fixed-count.patch'
# Windows PowerShell 5.1 turns native stderr into a terminating error while
# $ErrorActionPreference is Stop, so run git with Continue and check exit codes.
$savedEap = $ErrorActionPreference
$ErrorActionPreference = "Continue"
try {
  & git -C $PxlblzPath apply --reverse --check $fixedCountPatch 2>$null
  $alreadyApplied = ($LASTEXITCODE -eq 0)
  if (!$alreadyApplied) {
    & git -C $PxlblzPath apply $fixedCountPatch
    $applyExit = $LASTEXITCODE
  }
} finally {
  $ErrorActionPreference = $savedEap
}
if ($alreadyApplied) {
  Write-Host "Patch already applied: custom-map-fixed-count"
} elseif ($applyExit -ne 0) {
  throw "Patch custom-map-fixed-count.patch does not apply to $PxlblzPath"
} else {
  Write-Host "Patch applied: custom-map-fixed-count"
}

$encodedUrl = [System.Uri]::EscapeDataString($outputUrl)

Write-Host ""
Write-Host "PXLBLZ external pixel output installed." -ForegroundColor Green
Write-Host "Target: $Target"
Write-Host "Output URL: $outputUrl"
Write-Host "Preview backup: $backup"
Write-Host "PreviewDeck backup: $deckBackup"
Write-Host "Module: $moduleDest"
Write-Host "Toggle: $toggleDest"
Write-Host ""
Write-Host "Start PXLBLZ and append this query string to the local Studio URL:" -ForegroundColor Cyan
Write-Host "  ?pxout=1&pxoutUrl=$encodedUrl"
Write-Host "The OUT button in the Preview header then switches external output on/off for that tab;"
Write-Host "?pxout=0 disables it explicitly."
