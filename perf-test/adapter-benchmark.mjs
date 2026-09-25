import fs from 'node:fs'
import { pathToFileURL } from 'node:url'

function arg(name, fallback) {
  const i = process.argv.indexOf(`--${name}`)
  return i >= 0 ? process.argv[i + 1] : fallback
}

const modulePath = arg('module')
const pixels = Number(arg('pixels', '8186'))
const frames = Number(arg('frames', pixels > 20000 ? '300' : '1000'))
const warmup = Number(arg('warmup', '100'))
if (!modulePath) throw new Error('--module is required')

class MemoryStorage {
  constructor() { this.values = new Map() }
  getItem(k) { return this.values.has(k) ? this.values.get(k) : null }
  setItem(k,v) { this.values.set(k, String(v)) }
  removeItem(k) { this.values.delete(k) }
}

class NullWebSocket {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3
  constructor() {
    this.readyState = NullWebSocket.OPEN
    this.bufferedAmount = 0
    this.binaryType = 'arraybuffer'
  }
  send() {}
  close() { this.readyState = NullWebSocket.CLOSED }
}

globalThis.WebSocket = NullWebSocket
globalThis.window = {
  location: { search: '?pxout=1' },
  sessionStorage: new MemoryStorage(),
  setTimeout: globalThis.setTimeout.bind(globalThis),
  clearTimeout: globalThis.clearTimeout.bind(globalThis),
}

const { createExternalPixelOutput } = await import(pathToFileURL(modulePath).href + `?bench=${Date.now()}`)
const output = createExternalPixelOutput(pixels)
const frame = new Float64Array(pixels * 3)
for (let i = 0; i < frame.length; i++) frame[i] = (i % 1021) / 1020

for (let i = 0; i < warmup; i++) {
  frame[0] = (i & 255) / 255
  output.sendPacked(frame)
}

const start = performance.now()
for (let i = 0; i < frames; i++) {
  frame[0] = (i & 255) / 255
  output.sendPacked(frame)
}
const elapsedMs = performance.now() - start
output.close()

const rgbBytes = pixels * 3
const inputBytes = pixels * 3 * 8
const report = {
  pixels,
  frames,
  elapsedMs,
  msPerFrame: elapsedMs / frames,
  framesPerSecond: frames * 1000 / elapsedMs,
  rgbOutputMBps: (frames * rgbBytes) / (elapsedMs / 1000) / 1_000_000,
  floatInputMBps: (frames * inputBytes) / (elapsedMs / 1000) / 1_000_000,
}
console.log('ADAPTER_BENCHMARK_JSON ' + JSON.stringify(report))
