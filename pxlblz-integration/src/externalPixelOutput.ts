/**
 * Experimental PXLBLZ -> pxlblz-router pixel output.
 *
 * Opt-in only:
 *   ?pxout=1
 *
 * Wire format:
 *   one WebSocket binary message = one complete RGB888 frame
 *   ws://127.0.0.1:9980/pixels
 *
 * Real-time rule:
 *   never queue old video frames in the browser. If the WebSocket send buffer is
 *   already busy, skip this render frame. The native router has its own LatestFrame
 *   handoff as a second latency guard.
 */

export interface ExternalPixelOutput {
  readonly enabled: boolean
  readonly url: string
  sendPacked(frame: Float64Array): void
  close(): void
}

const DEFAULT_URL = 'ws://127.0.0.1:9980/pixels'
const RECONNECT_MS = 500
const SESSION_ENABLED_KEY = 'pxlblz:pxout:enabled'
const SESSION_URL_KEY = 'pxlblz:pxout:url'

function storage(): Storage | null {
  if (typeof window === 'undefined') return null
  try {
    return window.sessionStorage ?? null
  } catch {
    return null
  }
}

function queryEnabled(): boolean {
  if (typeof window === 'undefined') return false
  const value = new URLSearchParams(window.location.search).get('pxout')?.trim().toLowerCase()
  const store = storage()
  if (value === '1' || value === 'true' || value === 'on') {
    try { store?.setItem(SESSION_ENABLED_KEY, '1') } catch { /* storage is optional */ }
    return true
  }
  if (value === '0' || value === 'false' || value === 'off') {
    try { store?.removeItem(SESSION_ENABLED_KEY) } catch { /* storage is optional */ }
    return false
  }
  try {
    return store?.getItem(SESSION_ENABLED_KEY) === '1'
  } catch {
    return false
  }
}

function queryUrl(): string {
  if (typeof window === 'undefined') return DEFAULT_URL
  const store = storage()
  const raw = new URLSearchParams(window.location.search).get('pxoutUrl')?.trim()
  if (raw) {
    try { store?.setItem(SESSION_URL_KEY, raw) } catch { /* storage is optional */ }
    return raw
  }
  try {
    return store?.getItem(SESSION_URL_KEY)?.trim() || DEFAULT_URL
  } catch {
    return DEFAULT_URL
  }
}

function clampByte(v: number): number {
  if (!Number.isFinite(v) || v <= 0) return 0
  if (v >= 1) return 255
  return Math.round(v * 255)
}

export function createExternalPixelOutput(pixelCount: number): ExternalPixelOutput {
  const enabled = queryEnabled()
  const url = queryUrl()

  if (!enabled) {
    return {
      enabled: false,
      url,
      sendPacked: () => undefined,
      close: () => undefined,
    }
  }

  const expectedValues = pixelCount * 3
  const rgb = new Uint8Array(expectedValues)
  let ws: WebSocket | null = null
  let reconnectTimer: number | null = null
  let closed = false
  let warnedLength = false
  let skippedBusy = 0
  let sent = 0
  let lastLog = performance.now()

  const scheduleReconnect = () => {
    if (closed || reconnectTimer !== null) return
    reconnectTimer = window.setTimeout(() => {
      reconnectTimer = null
      connect()
    }, RECONNECT_MS)
  }

  const connect = () => {
    if (closed) return
    if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) return

    try {
      const next = new WebSocket(url)
      next.binaryType = 'arraybuffer'
      ws = next

      next.onopen = () => {
        console.info(`[pxout] connected ${url} (${pixelCount} pixels / ${rgb.byteLength} bytes)`)
      }

      next.onerror = () => {
        // onclose normally follows; keep the console noise low during router restarts.
      }

      next.onclose = () => {
        if (ws === next) ws = null
        scheduleReconnect()
      }
    } catch (err) {
      console.warn('[pxout] WebSocket connect failed', err)
      ws = null
      scheduleReconnect()
    }
  }

  connect()

  return {
    enabled: true,
    url,

    sendPacked(frame: Float64Array) {
      if (frame.length !== expectedValues) {
        if (!warnedLength) {
          warnedLength = true
          console.warn(`[pxout] frame length ${frame.length} != expected ${expectedValues}; output skipped`)
        }
        return
      }

      const socket = ws
      if (!socket || socket.readyState !== WebSocket.OPEN) {
        connect()
        return
      }

      // Browser WebSocket is TCP. Do not allow it to become a hidden frame FIFO.
      // One full frame already waiting is enough reason to drop this newer render.
      if (socket.bufferedAmount >= rgb.byteLength) {
        skippedBusy++
        return
      }

      for (let i = 0; i < expectedValues; i++) {
        rgb[i] = clampByte(frame[i])
      }

      socket.send(rgb)
      sent++

      const now = performance.now()
      if (now - lastLog >= 5000) {
        if (skippedBusy > 0) {
          console.info(`[pxout] sent=${sent} skipped-browser-backpressure=${skippedBusy}`)
        }
        sent = 0
        skippedBusy = 0
        lastLog = now
      }
    },

    close() {
      closed = true
      if (reconnectTimer !== null) {
        window.clearTimeout(reconnectTimer)
        reconnectTimer = null
      }
      const socket = ws
      ws = null
      if (socket && socket.readyState < WebSocket.CLOSING) socket.close()
    },
  }
}
