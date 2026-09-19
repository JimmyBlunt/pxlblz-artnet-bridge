param(
  [Parameter(Mandatory=$true)]
  [string]$PxlblzPath,

  [ValidateSet("fadecandy","artnet","custom")]
  [string]$Target = "fadecandy",

  [string]$CustomUrl = ""
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

switch ($Target) {
  "fadecandy" { $outputUrl = "ws://127.0.0.1:9981/pixels" }
  "artnet"    { $outputUrl = "ws://127.0.0.1:9980/pixels" }
  "custom" {
    if ([string]::IsNullOrWhiteSpace($CustomUrl)) {
      throw "-CustomUrl is required when -Target custom is selected."
    }
    $outputUrl = $CustomUrl.Trim()
  }
}

$text = Get-Content $preview -Raw

$importAnchor = "import { createVirtualClock } from '@/engine/virtualClock'"
$layoutAnchor = "    const { mapPoints, pixelCount, draw } = layout"

foreach ($anchor in @($importAnchor, $layoutAnchor)) {
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
    "$importAnchor`nimport { createExternalPixelOutput } from '@/engine/externalPixelOutput'"
  )
}

if (!$text.Contains("pixelCountCap === null ? createExternalPixelOutput(pixelCount)")) {
  $text = $text.Replace(
    $layoutAnchor,
    "$layoutAnchor`n    const externalPixelOutput =`n      pixelCountCap === null ? createExternalPixelOutput(pixelCount) : null"
  )
}

if (!$text.Contains("const paintPacked = externalPixelOutput?.enabled")) {
  $paintPattern = '(?ms)(    const paint = \(pixels: \[number, number, number\]\[\], brightness: number, dimmed: boolean\) => \{.*?^    \})\r?\n\r?\n(    const loop = createRenderLoop\(\{)'
  $paintInsert = @'
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
'@
  $next = [regex]::Replace($text, $paintPattern, $paintInsert, 1)
  if ($next -eq $text) {
    throw "Paint integration anchor not found. Backup exists at $backup"
  }
  $text = $next
}

if (!$text.Contains("      paintPacked,")) {
  $paintArgPattern = '(?m)^      paint,\r?$'
  $next = [regex]::Replace($text, $paintArgPattern, "      paint,`n      paintPacked,", 1)
  if ($next -eq $text) {
    throw "Render-loop paint argument anchor not found. Backup exists at $backup"
  }
  $text = $next
}

if (!$text.Contains("externalPixelOutput?.close()")) {
  $cleanupPattern = '(?m)^    return \(\) => loop\.stop\(\)\r?$'
  $cleanupReplacement = "    return () => {`n      loop.stop()`n      externalPixelOutput?.close()`n    }"
  $next = [regex]::Replace($text, $cleanupPattern, $cleanupReplacement, 1)
  if ($next -eq $text) {
    throw "Render-loop cleanup anchor not found. Backup exists at $backup"
  }
  $text = $next
}

Set-Content -Path $preview -Value $text -Encoding utf8

$encodedUrl = [System.Uri]::EscapeDataString($outputUrl)

Write-Host ""
Write-Host "PXLBLZ external pixel output installed." -ForegroundColor Green
Write-Host "Target: $Target"
Write-Host "Output URL: $outputUrl"
Write-Host "Backup: $backup"
Write-Host "Module: $moduleDest"
Write-Host ""
Write-Host "Start PXLBLZ and append this query string to the local URL it prints:" -ForegroundColor Cyan
Write-Host "  ?pxout=1&pxoutUrl=$encodedUrl"
