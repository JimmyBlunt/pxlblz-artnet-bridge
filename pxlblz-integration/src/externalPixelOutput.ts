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

export interface ExternalPixelOutputStats {
  sent: number
  skippedBackpressure: number
  notConnected: number
  wrongSize: number
  connectAttempts: number
}

export interface ExternalPixelOutput {
  readonly enabled: boolean
  readonly url: string
  sendPacked(frame: Float64Array): void
  stats(): ExternalPixelOutputStats
  close(): void
}

const DEFAULT_URL = 'ws://127.0.0.1:9980/pixels'
const RECONNECT_MS = 500
const SESSION_ENABLED_KEY = 'pxlblz:pxout:enabled'
const SESSION_URL_KEY = 'pxlblz:pxout:url'

function emptyStats(): ExternalPixelOutputStats {
  return {
    sent: 0,
    skippedBackpressure: 0,
    notConnected: 0,
    wrongSize: 0,
    connectAttempts: 0,
  }
}

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


export function externalPixelOutputPreference(): boolean {
  return queryEnabled()
}

export function externalPixelOutputUrlPreference(): string {
  return queryUrl()
}

export function setExternalPixelOutputPreference(enabled: boolean): void {
  const store = storage()
  try {
    if (enabled) store?.setItem(SESSION_ENABLED_KEY, '1')
    else store?.removeItem(SESSION_ENABLED_KEY)
  } catch {
    // sessionStorage is a convenience only; the URL flag remains authoritative.
  }
}

function clampByte(v: number): number {
  if (!Number.isFinite(v) || v <= 0) return 0
  if (v >= 1) return 255
  return Math.round(v * 255)
}

// A browser tab with hardware output is titled "PXLBLZ-IDE~ArtNet", so it can
// be told apart from a normal PXLBLZ tab. Runs as soon as this module loads
// (also on Gallery / Docs routes of the output-enabled tab).
const ARTNET_TITLE = 'PXLBLZ-IDE~ArtNet'
function markArtNetTitle(enabled: boolean): void {
  if (typeof document === 'undefined') return
  if (enabled) document.title = ARTNET_TITLE
  else if (document.title === ARTNET_TITLE) document.title = 'PXLBLZ-IDE'
}

// The same adapter also feeds the Fadecandy/L3D daemon (pxlblz-fadecandy on
// ws://127.0.0.1:9981/pixels). That target has no Art-Net router status page,
// so the Art-Net title and badge are only used for other targets (default
// 9980), or when ?pxoutStatus= names a status page explicitly.
const FADECANDY_WS_PORT = '9981'
function isArtNetTarget(): boolean {
  if (typeof window === 'undefined') return false
  if (new URLSearchParams(window.location.search).get('pxoutStatus')?.trim()) return true
  try {
    return new URL(queryUrl()).port !== FADECANDY_WS_PORT
  } catch {
    return true
  }
}
markArtNetTitle(queryEnabled() && isArtNetTarget())

// --- Art-Net status badge ---------------------------------------------------
// A small collapsible badge (bottom left) in the output-enabled tab: router
// reachable, frames per second arriving at the router, controllers, running
// test pattern, and a link to the router's configuration page. It only reads
// the router's GET /status (CORS-enabled); nothing in PXLBLZ itself changes.
const ROUTER_PAGE_DEFAULT = 'http://127.0.0.1:9988'
const BADGE_COLLAPSED_KEY = 'pxlblz:pxout:badgeCollapsed'

function routerPageUrl(): string {
  if (typeof window === 'undefined') return ROUTER_PAGE_DEFAULT
  const raw = new URLSearchParams(window.location.search).get('pxoutStatus')
  return (raw?.trim() || ROUTER_PAGE_DEFAULT).replace(/\/+$/, '')
}

interface RouterStatusDoc {
  uptime_s: number
  rx_frames: number
  rx_invalid?: number
  pixel_count?: number
  variable_size?: boolean
  last_invalid_pixels?: number
  ws_clients: number
  controllers?: { name: string; target_ip: string; stale: boolean }[]
  test_pattern?: string
}

let badgeStarted = false
function startArtNetBadge(): void {
  if (badgeStarted || typeof document === 'undefined') return
  badgeStarted = true
  const page = routerPageUrl()
  const el = document.createElement('div')
  el.setAttribute('data-pxlblz-artnet-badge', '')
  el.style.cssText = [
    'position:fixed', 'left:10px', 'bottom:10px', 'z-index:2147483000',
    'display:flex', 'align-items:center', 'gap:8px', 'padding:4px 8px',
    'font:11px/1.3 ui-monospace,Consolas,monospace', 'color:#d8d8e0',
    'background:rgba(16,16,22,.88)', 'border:1px solid #34343f', 'border-radius:6px',
    'box-shadow:0 2px 8px rgba(0,0,0,.35)', 'user-select:none',
  ].join(';')
  const dot = document.createElement('span')
  dot.style.cssText = 'width:8px;height:8px;border-radius:50%;background:#777;flex:none'
  const label = document.createElement('span')
  label.textContent = 'ArtNet'
  label.style.cssText = 'font-weight:600;cursor:pointer'
  label.title = 'Ein-/ausklappen'
  const text = document.createElement('span')
  const link = document.createElement('a')
  link.href = page + '/'
  link.target = '_blank'
  link.rel = 'noopener'
  link.textContent = 'Einstellungen ↗'
  link.style.cssText = 'color:#8fb0ff;text-decoration:none'
  el.append(dot, label, text, link)

  let collapsed = false
  try { collapsed = window.sessionStorage.getItem(BADGE_COLLAPSED_KEY) === '1' } catch { /* optional */ }
  const applyCollapsed = () => {
    text.style.display = collapsed ? 'none' : ''
    link.style.display = collapsed ? 'none' : ''
  }
  label.addEventListener('click', () => {
    collapsed = !collapsed
    try { window.sessionStorage.setItem(BADGE_COLLAPSED_KEY, collapsed ? '1' : '0') } catch { /* optional */ }
    applyCollapsed()
  })
  applyCollapsed()

  let prev: RouterStatusDoc | null = null
  const set = (color: string, msg: string, title: string) => {
    dot.style.background = color
    text.textContent = msg
    el.title = title
  }
  const poll = async () => {
    try {
      const res = await fetch(page + '/status', { cache: 'no-store', signal: AbortSignal.timeout(1500) })
      const st = (await res.json()) as RouterStatusDoc
      const dt = prev ? st.uptime_s - prev.uptime_s : 0
      const fps = prev && dt > 0 && st.rx_frames >= prev.rx_frames ? (st.rx_frames - prev.rx_frames) / dt : null
      const rejected = prev && dt > 0 && (st.rx_invalid ?? 0) > (prev.rx_invalid ?? 0)
        ? ((st.rx_invalid ?? 0) - (prev.rx_invalid ?? 0)) / dt : 0
      prev = st
      const ctrls = st.controllers ?? []
      if (rejected > 1 && (fps === null || fps < 1)) {
        // The router drops every frame: almost always the pattern's pixel count
        // differs from the router config (PXLBLZ keeps a pixel count per pattern,
        // a custom map does not override it).
        const sent = st.last_invalid_pixels
        const want = st.pixel_count
        set('#ff6b6b',
          sent && want ? `Router verwirft Bilder: ${sent} Pixel statt ${want}` : 'Router verwirft Bilder (falsche Größe)',
          want
            ? `Der Router erwartet genau ${want} Pixel pro Bild.\nIm Pattern die Pixelzahl auf ${want} stellen (Pixelzahl-Feld der Vorschau), oder in den Router-Einstellungen „jede Pixelzahl annehmen“ einschalten (fehlende Pixel bleiben dann schwarz).`
            : 'Pixelzahl des Patterns und der Router-Config prüfen.')
        return
      }
      const parts = [
        fps === null ? 'Router ✓' : `${fps.toFixed(0)} Bilder/s`,
        `${ctrls.length} Controller`,
      ]
      if (st.test_pattern) parts.push(`Test: ${st.test_pattern}`)
      const flowing = fps !== null && fps > 1
      set(st.test_pattern ? '#f0b44c' : flowing ? '#5fd38f' : '#f0b44c', parts.join(' · '),
        ctrls.map(c => `${c.name} ${c.target_ip}${c.stale ? ' (kein Bild)' : ''}`).join('\n') || 'keine Controller')
    } catch {
      prev = null
      set('#ff6b6b', 'Router nicht erreichbar', `Kein Router unter ${page} - Desktop-Verknüpfung „PXLBLZ-IDE - ArtNet“ starten`)
    }
  }
  const mount = () => {
    document.body.appendChild(el)
    void poll()
    window.setInterval(() => { void poll() }, 2000)
  }
  if (document.body) mount()
  else document.addEventListener('DOMContentLoaded', mount, { once: true })
}
if (queryEnabled() && isArtNetTarget()) startArtNetBadge()

export function createExternalPixelOutput(pixelCount: number): ExternalPixelOutput {
  const enabled = queryEnabled()
  const url = queryUrl()
  markArtNetTitle(enabled && isArtNetTarget())

  if (!enabled) {
    return {
      enabled: false,
      url,
      sendPacked: () => undefined,
      stats: emptyStats,
      close: () => undefined,
    }
  }

  const expectedValues = pixelCount * 3
  const rgb = new Uint8Array(expectedValues)
  let ws: WebSocket | null = null
  let reconnectTimer: number | null = null
  let closed = false
  let warnedLength = false

  let totalSent = 0
  let totalSkippedBusy = 0
  let totalNotConnected = 0
  let totalWrongSize = 0
  let totalConnectAttempts = 0

  let intervalSent = 0
  let intervalSkippedBusy = 0
  let lastLog = performance.now()

  const snapshotStats = (): ExternalPixelOutputStats => ({
    sent: totalSent,
    skippedBackpressure: totalSkippedBusy,
    notConnected: totalNotConnected,
    wrongSize: totalWrongSize,
    connectAttempts: totalConnectAttempts,
  })

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

    totalConnectAttempts++
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
        totalWrongSize++
        if (!warnedLength) {
          warnedLength = true
          console.warn(`[pxout] frame length ${frame.length} != expected ${expectedValues}; output skipped`)
        }
        return
      }

      const socket = ws
      if (!socket || socket.readyState !== WebSocket.OPEN) {
        totalNotConnected++
        connect()
        return
      }

      // Browser WebSocket is TCP. Do not allow it to become a hidden frame FIFO.
      // One full frame already waiting is enough reason to drop this newer render.
      if (socket.bufferedAmount >= rgb.byteLength) {
        totalSkippedBusy++
        intervalSkippedBusy++
        return
      }

      for (let i = 0; i < expectedValues; i++) {
        rgb[i] = clampByte(frame[i])
      }

      socket.send(rgb)
      totalSent++
      intervalSent++

      const now = performance.now()
      if (now - lastLog >= 5000) {
        if (intervalSkippedBusy > 0) {
          console.info(`[pxout] sent=${intervalSent} skipped-browser-backpressure=${intervalSkippedBusy}`)
        }
        intervalSent = 0
        intervalSkippedBusy = 0
        lastLog = now
      }
    },

    stats: snapshotStats,

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
