// FPS test on the real installation through the RUNNING router (no second
// sender): for each scenario the controllers' router FPS is set live via the
// configuration API, a router test pattern replaces the PXLBLZ input, and the
// router's per-controller counters are compared with the controllers' own
// counters (ESP32 web firmware and Teensy41 octo firmware). The original
// router config is restored at the end.
//
//   node perf-test/installation-fps-test.mjs --router http://127.0.0.1:9988 --seconds 20
//   options: --fps 30,60  --pattern rainbow  --controller-fps-ip 10.0.0.251=60 (apply only, not saved, restored)
import process from 'node:process'

function arg(name, fallback) {
  const i = process.argv.indexOf(`--${name}`)
  return i >= 0 ? process.argv[i + 1] : fallback
}
const router = arg('router', 'http://127.0.0.1:9988').replace(/\/$/, '')
const seconds = Number(arg('seconds', '20'))
const fpsSteps = arg('fps', '30,60').split(',').map(Number)
const pattern = arg('pattern', 'rainbow')
// optional: also set a controller's own targetFps for the run (ESP firmware only)
const ctrlFps = Object.fromEntries((arg('controller-fps-ip', '') || '').split(',').filter(Boolean).map(s => s.split('=')).map(([ip, f]) => [ip, Number(f)]))

const sleep = ms => new Promise(r => setTimeout(r, ms))
async function j(url, opts = {}) {
  const res = await fetch(url, { ...opts, signal: AbortSignal.timeout(8000) })
  const body = await res.json()
  if (!res.ok) throw new Error(`${url}: ${body.error || res.status}`)
  return body
}
const put = (url, obj) => j(url, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(obj) })
const post = (url, obj) => j(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: obj === undefined ? undefined : JSON.stringify(obj) })

// normalise both firmware status formats
function counters(st) {
  if ('artnet_complete' in st) { // Teensy41 octo firmware
    return { kind: 'teensy', packets: st.artnet_packets, complete: st.artnet_complete, incomplete: st.artnet_incomplete,
      rejected: st.artnet_rejected + st.artnet_ignored, stale: st.artnet_stale, output: st.dma_completed,
      outputUs: st.dma_observed_us, targetFps: st.target_fps }
  }
  return { kind: 'esp', packets: st.packets, complete: st.framesComplete, incomplete: st.framesIncomplete,
    rejected: (st.droppedPackets || 0) + (st.sequenceErrors || 0), stale: st.timeouts || 0, output: st.outputFrames,
    outputUs: st.outputTimeUs, targetFps: null }
}
async function ctrlStatus(ip) {
  try { return counters(await j(`http://${ip}/api/status`)) } catch (e) { return { error: e.message } }
}

const original = (await j(`${router}/api/config`)).config
const ips = original.controllers.filter(c => c.enabled !== false).map(c => c.target_ip)
const espOriginalFps = {}
for (const ip of Object.keys(ctrlFps)) {
  const cfg = await j(`http://${ip}/api/config`)
  espOriginalFps[ip] = cfg
}

const results = []
try {
  for (const fps of fpsSteps) {
    const cfg = structuredClone(original)
    for (const c of cfg.controllers) c.fps_target = fps
    await put(`${router}/api/config`, cfg)
    for (const [ip, f] of Object.entries(ctrlFps)) {
      const c = structuredClone(espOriginalFps[ip])
      c.targetFps = f
      for (const o of c.outputs) o.targetFps = f
      await post(`http://${ip}/api/config`, c) // apply only - NOT saved
    }
    await post(`${router}/api/test?pattern=${pattern}&seconds=${seconds + 6}`)
    await sleep(3000) // settle
    const r0 = await j(`${router}/status`)
    const c0 = Object.fromEntries(await Promise.all(ips.map(async ip => [ip, await ctrlStatus(ip)])))
    await sleep(seconds * 1000)
    const r1 = await j(`${router}/status`)
    const c1 = Object.fromEntries(await Promise.all(ips.map(async ip => [ip, await ctrlStatus(ip)])))
    const dt = r1.uptime_s - r0.uptime_s
    for (const ip of ips) {
      const a = r0.controllers.find(c => c.target_ip === ip), b = r1.controllers.find(c => c.target_ip === ip)
      const x = c0[ip], y = c1[ip]
      const row = { fps, ip, name: b?.name, routerFps: (b.frames - a.frames) / dt, routerErrors: b.send_errors - a.send_errors }
      if (x.error || y.error) row.error = x.error || y.error
      else Object.assign(row, {
        kind: y.kind,
        completeFps: (y.complete - x.complete) / dt,
        incompletePerS: (y.incomplete - x.incomplete) / dt,
        rejected: y.rejected - x.rejected,
        stale: y.stale - x.stale,
        outputFps: (y.output - x.output) / dt,
        outputMs: y.outputUs / 1000,
        lossPct: 100 * (1 - (y.complete - x.complete) / Math.max(1, b.frames - a.frames)),
      })
      results.push(row)
    }
  }
} finally {
  await post(`${router}/api/test/stop`).catch(() => {})
  await put(`${router}/api/config`, original).catch(e => console.error('RESTORE FAILED', e.message))
  for (const [ip, c] of Object.entries(espOriginalFps)) await post(`http://${ip}/api/config`, c).catch(e => console.error('ESP RESTORE FAILED', ip, e.message))
}

console.log(`pattern ${pattern}, ${seconds} s per step, measured via ${router}`)
console.log('fps  controller               router fps  complete fps  loss %  incompl/s  rejected  LED out fps  output ms')
for (const r of results) {
  if (r.error) { console.log(`${String(r.fps).padStart(3)}  ${(r.name + ' ' + r.ip).padEnd(24)} ${r.routerFps.toFixed(1).padStart(9)}   ERROR ${r.error}`); continue }
  console.log([String(r.fps).padStart(3), (r.name + ' ' + r.ip).slice(0, 24).padEnd(24), r.routerFps.toFixed(1).padStart(9),
    r.completeFps.toFixed(1).padStart(12), r.lossPct.toFixed(2).padStart(7), r.incompletePerS.toFixed(2).padStart(9),
    String(r.rejected).padStart(8), r.outputFps.toFixed(1).padStart(11), r.outputMs.toFixed(1).padStart(10)].join('  '))
}
console.log('JSON ' + JSON.stringify(results))
