param(
  [Parameter(Mandatory=$true)]
  [string]$PxlblzPath
)

$ErrorActionPreference = "Stop"

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

$text = Get-Content $preview -Raw
$deckText = Get-Content $previewDeck -Raw

$importAnchor = "import { createVirtualClock } from '@/engine/virtualClock'"
$layoutAnchor = "    const { mapPoints, pixelCount, draw } = layout"
$cleanupAnchor = "    return () => loop.stop()"
$deckImportAnchor = "import { CompactBrightness } from './CompactBrightness'"
$deckButtonAnchor = "      <CompactBrightness onSpace={toggle} value={brightness} onChange={v => { setBrightness(v); writeCascadedOverride('brightness', v) }} />"

foreach ($anchor in @($importAnchor, $layoutAnchor, $cleanupAnchor)) {
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
    "$importAnchor`r`nimport { createExternalPixelOutput } from '@/engine/externalPixelOutput'"
  )
}

if (!$text.Contains("pixelCountCap === null ? createExternalPixelOutput(pixelCount)")) {
  $text = $text.Replace(
    $layoutAnchor,
    "$layoutAnchor`r`n    const externalPixelOutput =`r`n      pixelCountCap === null ? createExternalPixelOutput(pixelCount) : null"
  )
}

$oldPaintEnd = @'
      captureRef.current.afterPaint(canvasRef.current)
    }

    const loop = createRenderLoop({
'@

$newPaintEnd = @'
      captureRef.current.afterPaint(canvasRef.current)
    }

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

    const loop = createRenderLoop({
'@

if (!$text.Contains("const paintPacked = externalPixelOutput?.enabled")) {
  if (!$text.Contains($oldPaintEnd)) {
    throw "Paint integration anchor not found. Backup exists at $backup"
  }
  $text = $text.Replace($oldPaintEnd, $newPaintEnd)
}

if (!$text.Contains("      paintPacked,")) {
  $text = $text.Replace(
    "      paint,`r`n      onError:",
    "      paint,`r`n      paintPacked,`r`n      onError:"
  )
}

if ($text.Contains($cleanupAnchor)) {
  $text = $text.Replace(
    $cleanupAnchor,
    "    return () => {`r`n      loop.stop()`r`n      externalPixelOutput?.close()`r`n    }"
  )
}

$text = $text.TrimEnd("`r", "`n") + "`r`n"

if (!$deckText.Contains("ExternalPixelOutputToggle")) {
  $deckText = $deckText.Replace(
    $deckImportAnchor,
    "$deckImportAnchor`r`nimport { ExternalPixelOutputToggle } from './ExternalPixelOutputToggle'"
  )
}

if (!$deckText.Contains("<ExternalPixelOutputToggle />")) {
  $deckText = $deckText.Replace(
    $deckButtonAnchor,
    "      <ExternalPixelOutputToggle />`r`n$deckButtonAnchor"
  )
}

$deckText = $deckText.TrimEnd("`r", "`n") + "`r`n"
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($preview, $text, $utf8NoBom)
[System.IO.File]::WriteAllText($previewDeck, $deckText, $utf8NoBom)

Write-Host ""
Write-Host "PXLBLZ external pixel output installed."
Write-Host "Preview backup: $backup"
Write-Host "PreviewDeck backup: $deckBackup"
Write-Host "Module: $moduleDest"
Write-Host "Toggle: $toggleDest"
Write-Host ""
Write-Host "Start PXLBLZ normally. Use the OUT button in the Preview header to enable/disable external output."
Write-Host "The legacy ?pxout=1 / ?pxout=0 URL controls remain available."
