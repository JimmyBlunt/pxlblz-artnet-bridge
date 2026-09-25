import { createExternalPixelOutput } from './build/externalPixelOutput.js'

const args = new Map()
for (let i = 2; i < process.argv.length; i += 2) {
  const key = process.argv[i]
  const value = process.argv[i + 1]
  if (!key?.startsWith('--') || value === undefined) throw new Error(`Bad argument near ${key ?? '<end>'}`)
  args.set(key.slice(2), value)
}

const pixels = Number(args.get('pixels') ?? '8000')
const fps = Number(args.get('fps') ?? '60')
const seconds = Number(args.get('seconds') ?? '5')
const url = args.get('url') ?? 'ws://127.0.0.1:9980/pixels'
const mode = args.get('mode') ?? 'dynamic'

if (!Number.isInteger(pixels) || pixels <= 0) throw new Error('--pixels must be a positive integer')
if (!Number.isFinite(fps) || fps <= 0 || fps > 240) throw new Error('--fps must be >0 and <=240')
if (!Number.isFinite(seconds) || seconds <= 0) throw new Error('--seconds must be >0')
if (mode !== 'dynamic' && mode !== 'static') throw new Error('--mode must be dynamic or static')

// Minimal browser surface used by externalPixelOutput.ts. Node 22 supplies the
// standards-compatible WebSocket and performance objects.
globalThis.window = {
  location: { search: `?pxout=1&pxoutUrl=${encodeURIComponent(url)}` },
  sessionStorage: {
    values: new Map(),
    getItem(key) { return this.values.has(key) ? this.values.get(key) : null },
    setItem(key, value) { this.values.set(key, String(value)) },
    removeItem(key) { this.values.delete(key) },
  },
  setTimeout: globalThis.setTimeout.bind(globalThis),
  clearTimeout: globalThis.clearTimeout.bind(globalThis),
}

const output = createExternalPixelOutput(pixels)
if (!output.enabled) throw new Error('external output unexpectedly disabled')

const frame = new Float64Array(pixels * 3)
const periodMs = 1000 / fps
const totalFrames = Math.max(1, Math.round(fps * seconds))
let produced = 0

function fillBaseFrame() {
  for (let p = 0; p < pixels; p++) {
    const o = p * 3
    frame[o] = (p % 257) / 256
    frame[o + 1] = ((p >> 4) % 193) / 192
    frame[o + 2] = (p % 251) / 250
  }
}

function updateDynamicFrame(frameNo) {
  const phase = (frameNo % 256) / 255
  for (let p = 0; p < pixels; p++) {
    const o = p * 3
    const x = (p % 257) / 256
    frame[o] = (x + phase) % 1
    frame[o + 1] = ((p >> 4) % 193) / 192
    frame[o + 2] = ((p + frameNo * 3) % 251) / 250
  }
}

fillBaseFrame()
const started = performance.now()

// Deadline-based pacing avoids cumulative setInterval drift. If the process is
// briefly late, the next deadline remains anchored to the original start time.
while (produced < totalFrames) {
  const due = started + produced * periodMs
  const now = performance.now()
  const waitMs = due - now
  if (waitMs > 1) {
    await new Promise(resolve => setTimeout(resolve, Math.max(0, waitMs - 0.5)))
    continue
  }
  if (mode === 'dynamic') updateDynamicFrame(produced)
  else {
    // Make each static transport frame observably distinct without paying for
    // a second full-frame generator pass.
    frame[0] = (produced & 255) / 255
  }
  output.sendPacked(frame)
  produced++
}

await new Promise(resolve => setTimeout(resolve, 150))
const stats = output.stats()
output.close()
const elapsed = (performance.now() - started) / 1000
console.log(
  `VIRTUAL_PXLBLZ_DONE pixels=${pixels} produced=${produced} elapsed=${elapsed.toFixed(3)} avg_fps=${(produced / elapsed).toFixed(2)} mode=${mode}`
  + ` sent=${stats.sent} skipped=${stats.skippedBackpressure} not_connected=${stats.notConnected} wrong_size=${stats.wrongSize} connect_attempts=${stats.connectAttempts}`
)
