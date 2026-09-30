// Hardware throughput ramp against ONE real Art-Net controller that exposes
// GET /api/status (ESP32 / Teensy web firmware: packets, framesComplete,
// framesIncomplete, droppedPackets, sequenceErrors, timeouts, outputFrames).
//
// For every FPS step the router sends a test pattern for N seconds; the
// controller counters are read before and after, so "sent by router" can be
// compared with "received complete by controller".
//
//   node perf-test/hardware-ramp.mjs --config router/config/routes.esp-test-172.json \
//        --controller http://172.20.10.2 --steps 30,45,60,90,120 --seconds 20
import { spawn } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import process from 'node:process'
import { fileURLToPath } from 'node:url'

const here = path.dirname(fileURLToPath(import.meta.url))
const repoRoot = path.resolve(here, '..')

function arg(name, fallback) {
  const i = process.argv.indexOf(`--${name}`)
  return i >= 0 ? process.argv[i + 1] : fallback
}

const configPath = path.resolve(arg('config', path.join(repoRoot, 'router', 'config', 'routes.esp-test-172.json')))
const controller = arg('controller', 'http://172.20.10.2').replace(/\/$/, '')
const steps = arg('steps', '30,45,60,90,120').split(',').map(Number)
const seconds = Number(arg('seconds', '20'))
const pattern = arg('pattern', 'rainbow')
const routerBin = path.resolve(arg('router', path.join(repoRoot, 'bin', 'windows-x64', process.platform === 'win32' ? 'pxlblz-router.exe' : 'pxlblz-router')))
const stamp = new Date().toISOString().replace(/[:.]/g, '-')
const out = path.resolve(arg('out', path.join(here, 'results', `${stamp}-hardware-ramp`)))
fs.mkdirSync(out, { recursive: true })

const cfg = JSON.parse(fs.readFileSync(configPath, 'utf8'))
const universesPerFrame = cfg.routes.filter(r => r.enabled).reduce((n, r) => n + Math.ceil(r.pixel_count / 170), 0)

async function status() {
  const res = await fetch(`${controller}/api/status`, { signal: AbortSignal.timeout(3000) })
  if (!res.ok) throw new Error(`status HTTP ${res.status}`)
  return res.json()
}

function runRouter(fps) {
  return new Promise((resolve, reject) => {
    const child = spawn(routerBin, [
      '--config', configPath, '--input', 'pattern', '--pattern', pattern,
      '--fps', String(fps), '--duration', `${seconds}s`, '--status-listen', '',
    ], { windowsHide: true })
    let stdout = ''
    child.stdout.on('data', d => { stdout += d })
    child.stderr.on('data', d => { stdout += d })
    child.on('error', reject)
    child.on('exit', code => code === 0 ? resolve(stdout) : reject(new Error(`router exited ${code}\n${stdout}`)))
  })
}

const wait = ms => new Promise(r => setTimeout(r, ms))
const counters = ['packets', 'framesComplete', 'framesIncomplete', 'droppedPackets', 'sequenceErrors', 'timeouts', 'outputFrames']

console.log(`controller ${controller} | ${universesPerFrame} universes/frame | ${seconds}s per step | pattern ${pattern}`)
console.log('fps  sent_frames  rx_complete  complete%  rx_pkts/sent  incompl  dropped  seqErr  timeouts  out_frames  ctl_frame_us  send_avg_ms')
const rows = []
for (const fps of steps) {
  const before = await status()
  const log = await runRouter(fps)
  await wait(1500) // let the last packets and the controller's own accounting settle
  const after = await status()
  const tx = /Final TX: (\d+) frames, (\d+) packets, ([\d.]+) avg fps, ([\d.]+) packets\/s, ([\d.]+) avg send ms, ([\d.]+) max send ms, [\d.]+ MB heap, (\d+) send errors/.exec(log)
  if (!tx) throw new Error(`no Final TX line for ${fps} fps\n${log}`)
  const d = Object.fromEntries(counters.map(k => [k, (after[k] ?? 0) - (before[k] ?? 0)]))
  const row = {
    fps,
    sentFrames: Number(tx[1]),
    sentPackets: Number(tx[2]),
    sentFps: Number(tx[3]),
    sendAvgMs: Number(tx[5]),
    sendMaxMs: Number(tx[6]),
    sendErrors: Number(tx[7]),
    ...Object.fromEntries(Object.entries(d).map(([k, v]) => [`rx_${k}`, v])),
    completeRatio: Number(tx[1]) ? d.framesComplete / Number(tx[1]) : 0,
    packetRatio: Number(tx[2]) ? d.packets / Number(tx[2]) : 0,
    controllerFrameTimeUs: after.frameTimeUs,
    controllerHeapFree: after.heapFree,
  }
  rows.push(row)
  console.log([
    String(fps).padStart(3), String(row.sentFrames).padStart(12), String(d.framesComplete).padStart(12),
    (row.completeRatio * 100).toFixed(2).padStart(9), `${d.packets}/${row.sentPackets}`.padStart(13),
    String(d.framesIncomplete).padStart(8), String(d.droppedPackets).padStart(8), String(d.sequenceErrors).padStart(7),
    String(d.timeouts).padStart(9), String(d.outputFrames).padStart(11), String(after.frameTimeUs).padStart(13),
    row.sendAvgMs.toFixed(3).padStart(12),
  ].join('  '))
  await wait(1000)
}

const summary = { controller, config: configPath, universesPerFrame, seconds, pattern, steps: rows, generatedAt: new Date().toISOString() }
fs.writeFileSync(path.join(out, 'hardware-ramp.json'), JSON.stringify(summary, null, 2) + '\n')
console.log(`results: ${path.join(out, 'hardware-ramp.json')}`)
