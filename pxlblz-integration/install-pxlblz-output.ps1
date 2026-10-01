param(
  [Parameter(Mandatory=$true)]
  [string]$PxlblzPath
)

$ErrorActionPreference = "Stop"

$root = (Resolve-Path $PxlblzPath).Path
$preview = Join-Path $root "src\components\Preview.tsx"
$engineDir = Join-Path $root "src\engine"
$moduleDest = Join-Path $engineDir "externalPixelOutput.ts"
$moduleSource = Join-Path $PSScriptRoot "src\externalPixelOutput.ts"

if (!(Test-Path $preview)) {
  throw "Preview.tsx not found: $preview"
}
if (!(Test-Path $moduleSource)) {
  throw "Integration module not found: $moduleSource"
}

$text = Get-Content $preview -Raw

$importAnchor = "import { createVirtualClock } from '@/engine/virtualClock'"
$layoutAnchor = "    const { mapPoints, pixelCount, draw } = layout"
$cleanupAnchor = "    return () => loop.stop()"

foreach ($anchor in @($importAnchor, $layoutAnchor, $cleanupAnchor)) {
  if (!$text.Contains($anchor)) {
    throw "Expected PXLBLZ source anchor not found. No changes made: $anchor"
  }
}

$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$backup = "$preview.pxout-backup-$stamp"
Copy-Item $preview $backup
Copy-Item $moduleSource $moduleDest -Force

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

Set-Content -Path $preview -Value $text -Encoding utf8

# Custom maps authored as a literal coordinate array get a fixed pixel count
# (= their point count), like PXLBLZ's stock literal maps. Without it a pattern
# keeps an old persisted count when such a map is selected and the router
# rejects every frame. Idempotent: skipped when already applied.
$fixedCountPatch = Join-Path $PSScriptRoot 'patches\custom-map-fixed-count.patch'
& git -C $PxlblzPath apply --reverse --check $fixedCountPatch 2>$null
if ($LASTEXITCODE -eq 0) {
  Write-Host "Patch already applied: custom-map-fixed-count"
} else {
  & git -C $PxlblzPath apply $fixedCountPatch
  if ($LASTEXITCODE -ne 0) { throw "Patch custom-map-fixed-count.patch does not apply to $PxlblzPath" }
  Write-Host "Patch applied: custom-map-fixed-count"
}

Write-Host ""
Write-Host "PXLBLZ external pixel output installed."
Write-Host "Backup: $backup"
Write-Host "Module: $moduleDest"
Write-Host ""
Write-Host "Start PXLBLZ normally, then open it with ?pxout=1 to enable hardware output."
