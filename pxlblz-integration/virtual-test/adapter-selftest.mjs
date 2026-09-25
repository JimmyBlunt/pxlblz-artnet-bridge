import assert from 'node:assert/strict'
import { createExternalPixelOutput } from './build/externalPixelOutput.js'

class FakeWebSocket {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3
  static instances = []

  constructor(url) {
    this.url = url
    this.readyState = FakeWebSocket.CONNECTING
    this.bufferedAmount = 0
    this.binaryType = ''
    this.sent = []
    FakeWebSocket.instances.push(this)
    queueMicrotask(() => {
      this.readyState = FakeWebSocket.OPEN
      this.onopen?.()
    })
  }

  send(payload) {
    this.sent.push(new Uint8Array(payload))
  }

  close() {
    this.readyState = FakeWebSocket.CLOSED
    this.onclose?.()
  }
}

class MemoryStorage {
  constructor() { this.values = new Map() }
  getItem(key) { return this.values.has(key) ? this.values.get(key) : null }
  setItem(key, value) { this.values.set(key, String(value)) }
  removeItem(key) { this.values.delete(key) }
}

globalThis.WebSocket = FakeWebSocket
globalThis.window = {
  location: { search: '?pxout=1' },
  sessionStorage: new MemoryStorage(),
  setTimeout: globalThis.setTimeout.bind(globalThis),
  clearTimeout: globalThis.clearTimeout.bind(globalThis),
}

const out = createExternalPixelOutput(2)
assert.equal(out.enabled, true)
assert.equal(out.url, 'ws://127.0.0.1:9980/pixels')
assert.deepEqual(out.stats(), {
  sent: 0,
  skippedBackpressure: 0,
  notConnected: 0,
  wrongSize: 0,
  connectAttempts: 1,
})

await new Promise(resolve => setTimeout(resolve, 0))
const ws = FakeWebSocket.instances.at(-1)
assert.ok(ws)
assert.equal(ws.binaryType, 'arraybuffer')

out.sendPacked(new Float64Array([-1, 0, 0.5, 1, 2, Number.NaN]))
assert.equal(ws.sent.length, 1)
assert.deepEqual([...ws.sent[0]], [0, 0, 128, 255, 255, 0])
assert.equal(out.stats().sent, 1)

out.sendPacked(new Float64Array([1, 1, 1]))
assert.equal(ws.sent.length, 1)
assert.equal(out.stats().wrongSize, 1)

ws.bufferedAmount = 6
out.sendPacked(new Float64Array([1, 0, 0, 0, 1, 0]))
assert.equal(ws.sent.length, 1)
assert.equal(out.stats().skippedBackpressure, 1)
ws.bufferedAmount = 0

out.close()
assert.equal(ws.readyState, FakeWebSocket.CLOSED)

window.location.search = ''
const persisted = createExternalPixelOutput(2)
assert.equal(persisted.enabled, true)
await new Promise(resolve => setTimeout(resolve, 0))
persisted.close()

window.location.search = '?pxout=0'
const before = FakeWebSocket.instances.length
const disabled = createExternalPixelOutput(2)
assert.equal(disabled.enabled, false)
disabled.sendPacked(new Float64Array(6))
disabled.close()
assert.equal(FakeWebSocket.instances.length, before)
assert.deepEqual(disabled.stats(), {
  sent: 0,
  skippedBackpressure: 0,
  notConnected: 0,
  wrongSize: 0,
  connectAttempts: 0,
})

window.location.search = ''
assert.equal(createExternalPixelOutput(2).enabled, false)

console.log('ADAPTER_SELFTEST_PASS conversion=ok wrong-size=drop backpressure=drop counters=ok session-persist=ok explicit-off=ok')
