// Frame-by-frame comparison: native FastLED example (leds[] per loop) vs. its
// Pixelblaze-language port executed by the PXLBLZ-IDE engine (Fast + Precise).
// usage: tsx compare-demo.ts <port.js> <native.leds.bin> <numLeds> <frames>
import { readFileSync } from 'node:fs'
import { bundle } from '../../PXLBLZ-IDE/src/engine/bundle'
import { loadPattern } from '../../PXLBLZ-IDE/src/engine/loadPattern'
import { createFxShim, createShim, planeShimConfig } from '../../PXLBLZ-IDE/src/engine/shim'

const [portFile, nativeFile, nStr, fStr] = process.argv.slice(2)
const N = Number(nStr), F = Number(fStr)
const native = readFileSync(nativeFile)
const { code, fxCode, metadata } = bundle(readFileSync(portFile, 'utf8'), {})

// Same byte conversion as the IDE's Art-Net output adapter (externalPixelOutput.ts)
const toByte = (v: number) => (v <= 0 ? 0 : v >= 1 ? 255 : Math.round(v * 255))

for (const mode of ['fast', 'precise'] as const) {
  const cfg = { ...planeShimConfig({ rows: 1, cols: N }), getVirtualTime: () => 0 }
  const shim = mode === 'precise' ? createFxShim(cfg) : createShim(cfg)
  const h = loadPattern(mode === 'precise' ? fxCode : code, metadata, shim.builtins)
  const packed = new Float64Array(N * 3)
  let badFrames = 0, badBytes = 0, firstBad = -1, maxDiff = 0
  for (let f = 0; f < F; f++) {
    h.beforeRender(shim.encodeScalar(1000 / 60))
    for (let i = 0; i < N; i++) {
      h.render(shim.encodeScalar(i))
      shim.writeCapturedPixel(packed, i * 3)
    }
    let frameBad = false
    for (let k = 0; k < N * 3; k++) {
      const got = toByte(packed[k]), want = native[f * N * 3 + k]
      if (got !== want) { badBytes++; frameBad = true; maxDiff = Math.max(maxDiff, Math.abs(got - want)) }
    }
    if (frameBad) { badFrames++; if (firstBad < 0) firstBad = f }
  }
  console.log(`${mode.padEnd(7)} frames=${F} leds=${N} mismatchFrames=${badFrames} mismatchBytes=${badBytes}/${F * N * 3} maxByteDiff=${maxDiff} firstMismatchFrame=${firstBad}`)
}
