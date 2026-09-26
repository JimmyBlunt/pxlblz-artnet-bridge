import { spawn, spawnSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import process from 'node:process'
import { fileURLToPath } from 'node:url'

const here = path.dirname(fileURLToPath(import.meta.url))
const repoRoot = path.resolve(here, '..', '..')
const routerRoot = path.join(repoRoot, 'router')
const buildDir = path.join(here, 'build')
const tmpDir = path.join(here, '.tmp')
fs.rmSync(buildDir, { recursive: true, force: true })
fs.rmSync(tmpDir, { recursive: true, force: true })
fs.mkdirSync(buildDir, { recursive: true })
fs.mkdirSync(tmpDir, { recursive: true })

function run(command, args, opts = {}) {
  const r = spawnSync(command, args, { cwd: opts.cwd ?? repoRoot, encoding: 'utf8', stdio: opts.stdio ?? 'pipe', shell: opts.shell ?? false })
  if (r.status !== 0) {
    throw new Error(`${command} ${args.join(' ')} failed (${r.status})\nSTDOUT:\n${r.stdout ?? ''}\nSTDERR:\n${r.stderr ?? ''}`)
  }
  return r
}

function wait(ms) { return new Promise(resolve => setTimeout(resolve, ms)) }

function start(command, args, cwd) {
  const child = spawn(command, args, { cwd, stdio: ['ignore', 'pipe', 'pipe'] })
  let stdout = ''
  let stderr = ''
  child.stdout.on('data', d => { stdout += d.toString(); process.stdout.write(d) })
  child.stderr.on('data', d => { stderr += d.toString(); process.stderr.write(d) })
  return { child, get stdout() { return stdout }, get stderr() { return stderr } }
}

async function waitForText(proc, text, timeoutMs = 8000) {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    if (proc.stdout.includes(text) || proc.stderr.includes(text)) return
    if (proc.child.exitCode !== null) throw new Error(`process exited before '${text}'\n${proc.stdout}\n${proc.stderr}`)
    await wait(50)
  }
  throw new Error(`timeout waiting for '${text}'\n${proc.stdout}\n${proc.stderr}`)
}

async function stop(proc) {
  if (!proc || proc.child.exitCode !== null) return
  proc.child.kill('SIGTERM')
  await Promise.race([
    new Promise(resolve => proc.child.once('exit', resolve)),
    wait(1200),
  ])
  if (proc.child.exitCode === null) proc.child.kill('SIGKILL')
}

console.log('=== Compile exact externalPixelOutput.ts ===')
run('tsc', [
  path.join(repoRoot, 'pxlblz-integration', 'src', 'externalPixelOutput.ts'),
  '--target', 'ES2022', '--module', 'ES2022', '--moduleResolution', 'bundler',
  '--lib', 'ES2022,DOM', '--outDir', buildDir,
], { shell: process.platform === 'win32' })

console.log('=== Adapter unit/self-test ===')
run(process.execPath, [path.join(here, 'adapter-selftest.mjs')], { stdio: 'inherit' })

console.log('=== Go unit tests ===')
run('go', ['test', './...'], { cwd: routerRoot, stdio: 'inherit' })

const routerBin = path.join(tmpDir, process.platform === 'win32' ? 'pxlblz-router.exe' : 'pxlblz-router')
const listenerBin = path.join(tmpDir, process.platform === 'win32' ? 'artnet-listener.exe' : 'artnet-listener')
const virtualControllerBin = path.join(tmpDir, process.platform === 'win32' ? 'pxlblz-virtual-controller.exe' : 'pxlblz-virtual-controller')
const receiverProbeBin = path.join(tmpDir, process.platform === 'win32' ? 'pxlblz-receiver-probe.exe' : 'pxlblz-receiver-probe')
run('go', ['build', '-o', routerBin, './cmd/pxlblz-router'], { cwd: routerRoot })
run('go', ['build', '-o', listenerBin, './cmd/artnet-listener'], { cwd: routerRoot })
run('go', ['build', '-o', virtualControllerBin, './cmd/pxlblz-virtual-controller'], { cwd: routerRoot })
run('go', ['build', '-o', receiverProbeBin, './cmd/pxlblz-receiver-probe'], { cwd: routerRoot })

let listener
let router
try {
  console.log('=== Virtual E2E A: exact TS adapter -> WS -> router -> UDP Art-Net listener ===')
  listener = start(listenerBin, ['--bind', '127.0.0.1:6454'], routerRoot)
  await waitForText(listener, 'Art-Net listener on')
  router = start(routerBin, [
    '--config', path.join(routerRoot, 'config', 'routes.loopback.json'),
    '--input', 'ws', '--duration', '7s',
  ], routerRoot)
  await waitForText(router, 'Waiting for first valid input frame')
  await wait(250)
  run(process.execPath, [path.join(here, 'virtual-pxlblz-driver.mjs'), '--pixels', '8000', '--fps', '90', '--seconds', '5'], { stdio: 'inherit' })
  await new Promise(resolve => router.child.once('exit', resolve))

  if (!/invalid 0\/s/.test(router.stdout)) throw new Error('Loopback router reported invalid websocket frames')
  if (!/errors 0/.test(router.stdout)) throw new Error('Loopback router reported send errors')
  if (!/Final RX: \d+ valid frames, \d+ replaced before output observation, 0 invalid/.test(router.stdout)) {
    throw new Error('Loopback final RX validation failed')
  }
  if (!/U0:\d+/.test(listener.stdout) || !/U47:\d+/.test(listener.stdout)) {
    throw new Error('Art-Net listener did not observe full U0..U47 range')
  }
  console.log('VIRTUAL_E2E_A_PASS exact_adapter=true websocket=true artnet_udp=true universes=48 invalid=0')
} finally {
  await stop(router)
  await stop(listener)
}

try {
  console.log('=== Virtual E2E B: production BACK_PANEL route in dry-run ===')
  router = start(routerBin, [
    '--config', path.join(routerRoot, 'config', 'routes.backpanel-all.json'),
    '--input', 'ws', '--dry-run', '--duration', '7s',
  ], routerRoot)
  await waitForText(router, 'Waiting for first valid input frame')
  await wait(250)
  run(process.execPath, [path.join(here, 'virtual-pxlblz-driver.mjs'), '--pixels', '8186', '--fps', '60', '--seconds', '5'], { stdio: 'inherit' })
  await new Promise(resolve => router.child.once('exit', resolve))

  if (!/invalid 0\/s/.test(router.stdout)) throw new Error('BACK_PANEL dry-run reported invalid websocket frames')
  if (!/TX\s+30\.0 fps\s+870 pkt\/s/.test(router.stdout)) throw new Error('BACK_PANEL did not sustain expected 30 FPS / 870 pkt/s')
  if (!/Final RX: \d+ valid frames, \d+ replaced before output observation, 0 invalid/.test(router.stdout)) {
    throw new Error('BACK_PANEL final RX validation failed')
  }
  console.log('VIRTUAL_E2E_B_PASS pixels=8186 frame_bytes=24558 universes_per_frame=29 tx_fps=30 packets_per_second=870 invalid=0')
} finally {
  await stop(router)
}

try {
  console.log('=== Virtual E2E C: all known installation controllers -> real loopback UDP ===')
  const knownConfig = JSON.parse(fs.readFileSync(path.join(routerRoot, 'config', 'routes.installation-known.json'), 'utf8'))
  for (const route of knownConfig.routes) route.target_ip = '127.0.0.1'
  const knownLoopbackConfig = path.join(tmpDir, 'routes.installation-known.loopback.json')
  fs.writeFileSync(knownLoopbackConfig, JSON.stringify(knownConfig, null, 2) + '\n')

  listener = start(listenerBin, ['--bind', '127.0.0.1:6454'], routerRoot)
  await waitForText(listener, 'Art-Net listener on')
  router = start(routerBin, [
    '--config', knownLoopbackConfig,
    '--input', 'ws', '--duration', '7s',
  ], routerRoot)
  await waitForText(router, 'Waiting for first valid input frame')
  await wait(250)
  run(process.execPath, [path.join(here, 'virtual-pxlblz-driver.mjs'), '--pixels', '8186', '--fps', '60', '--seconds', '5'], { stdio: 'inherit' })
  await new Promise(resolve => router.child.once('exit', resolve))

  if (!/invalid 0\/s/.test(router.stdout)) throw new Error('Known-installation loopback reported invalid websocket frames')
  if (!/TX\s+30\.0 fps\s+1320 pkt\/s/.test(router.stdout)) throw new Error('Known-installation routing did not sustain expected 30 FPS / 1320 pkt/s')
  if (!/Final RX: \d+ valid frames, \d+ replaced before output observation, 0 invalid/.test(router.stdout)) {
    throw new Error('Known-installation final RX validation failed')
  }
  for (const required of ['GRB', 'RGB', 'BGR']) {
    if (!router.stdout.includes(required)) throw new Error(`Known-installation route summary missing color order ${required}`)
  }
  for (const universe of [0, 3, 6, 7, 12, 14, 120, 149, 156, 161]) {
    if (!new RegExp(`U${universe}:\\d+`).test(listener.stdout)) {
      throw new Error(`Known-installation listener did not observe universe U${universe}`)
    }
  }
  if (/invalid [1-9]\d*/.test(listener.stdout)) throw new Error('Known-installation listener reported invalid Art-Net packets')
  console.log('VIRTUAL_E2E_C_PASS pixels=8186 known_controllers=3 universes_per_frame=44 tx_fps=30 packets_per_second=1320 udp_loopback=true invalid=0')
} finally {
  await stop(router)
  await stop(listener)
}

let virtualController
try {
  console.log('=== Virtual E2E D: exact BACK_PANEL Teensy receiver emulation ===')
  const sourceConfigPath = path.join(routerRoot, 'config', 'routes.backpanel-all.json')
  const senderConfig = JSON.parse(fs.readFileSync(sourceConfigPath, 'utf8'))
  senderConfig.artnet.udp_port = 6455
  for (const route of senderConfig.routes) route.target_ip = '127.0.0.1'
  const senderConfigPath = path.join(tmpDir, 'routes.backpanel.virtual-sender.json')
  fs.writeFileSync(senderConfigPath, JSON.stringify(senderConfig, null, 2) + '\n')
  const summaryPath = path.join(tmpDir, 'virtual-controller-summary.json')

  virtualController = start(virtualControllerBin, [
    '--config', sourceConfigPath,
    '--target-ip', '10.0.0.253',
    '--listen', '127.0.0.1:6455',
    '--web', '',
    '--duration', '7s',
    '--summary-json', summaryPath,
  ], routerRoot)
  await waitForText(virtualController, 'Receiver model: Teensy runtime_receiver / artnet_run_policy compatible')

  router = start(routerBin, [
    '--config', senderConfigPath,
    '--input', 'ws',
    '--duration', '5s',
  ], routerRoot)
  await waitForText(router, 'Waiting for first valid input frame')
  await wait(250)
  run(process.execPath, [path.join(here, 'virtual-pxlblz-driver.mjs'), '--pixels', '8186', '--fps', '60', '--seconds', '4'], { stdio: 'inherit' })

  await new Promise(resolve => router.child.once('exit', resolve))
  await new Promise(resolve => virtualController.child.once('exit', resolve))

  const summary = JSON.parse(fs.readFileSync(summaryPath, 'utf8'))
  const counters = summary.counters
  if (counters.complete < 100) throw new Error(`Virtual Teensy completed only ${counters.complete} frames`)
  if (counters.rejected !== 0 || counters.ignored !== 0 || counters.stale !== 0 || counters.incomplete !== 0) {
    throw new Error(`Virtual Teensy protocol counters not clean: ${JSON.stringify(counters)}`)
  }
  if (counters.frames_submitted < 80 || counters.dma_completed < 80) {
    throw new Error(`Virtual Teensy run policy did not submit/complete enough frames: ${JSON.stringify(counters)}`)
  }
  if (summary.wire_guard_us !== 26700) throw new Error(`Virtual Teensy wire guard ${summary.wire_guard_us} != 26700 us`)
  if (counters.blackouts !== 2) throw new Error(`Virtual Teensy expected startup + post-stream blackouts, got ${counters.blackouts}`)
  console.log(
    `VIRTUAL_E2E_D_PASS complete=${counters.complete} submitted=${counters.frames_submitted} dma=${counters.dma_completed}`
    + ` rejected=${counters.rejected} ignored=${counters.ignored} incomplete=${counters.incomplete}`
    + ` wire_guard_us=${summary.wire_guard_us} blackouts=${counters.blackouts}`
  )
} finally {
  await stop(router)
  await stop(virtualController)
}

console.log('=== Virtual E2E E: UDP fault-injection probe against receiver emulator ===')
const faultCases = [
  { name: 'short-tail', expect: c => c.rejected === 1 && c.complete === 0 && c.incomplete === 1 },
  { name: 'missing', expect: c => c.rejected === 0 && c.complete === 0 && c.incomplete === 1 },
  { name: 'packet-seq', expect: c => c.complete === 0 && c.incomplete > 0 },
  { name: 'extras', expect: c => c.ignored === 4 && c.complete === 1 },
  { name: 'seq0', expect: c => c.complete === 1 && c.rejected === 0 },
  { name: 'duplicate0', expect: c => c.duplicates === 1 && c.incomplete === 1 && c.complete === 1 },
]
for (let i = 0; i < faultCases.length; i++) {
  const testCase = faultCases[i]
  const port = 6460 + i
  const summaryPath = path.join(tmpDir, `fault-${testCase.name}.json`)
  virtualController = start(virtualControllerBin, [
    '--config', path.join(routerRoot, 'config', 'routes.backpanel-all.json'),
    '--target-ip', '10.0.0.253',
    '--listen', `127.0.0.1:${port}`,
    '--web', '',
    '--duration', '800ms',
    '--summary-json', summaryPath,
  ], routerRoot)
  await waitForText(virtualController, 'Receiver model: Teensy runtime_receiver / artnet_run_policy compatible')
  // Let the emulated controller complete the same initial black DMA handoff
  // performed by controller::start() before fault packets arrive.
  await wait(60)
  run(receiverProbeBin, [
    '--config', path.join(routerRoot, 'config', 'routes.backpanel-all.json'),
    '--profile-ip', '10.0.0.253',
    '--target', `127.0.0.1:${port}`,
    '--scenario', testCase.name,
  ], { cwd: routerRoot, stdio: 'inherit' })
  await new Promise(resolve => virtualController.child.once('exit', resolve))
  const summary = JSON.parse(fs.readFileSync(summaryPath, 'utf8'))
  if (!testCase.expect(summary.counters)) {
    throw new Error(`fault scenario ${testCase.name} counters unexpected: ${JSON.stringify(summary.counters)}`)
  }
  console.log(`VIRTUAL_FAULT_PASS scenario=${testCase.name} counters=${JSON.stringify(summary.counters)}`)
  virtualController = null
}

console.log('ALL_VIRTUAL_TESTS_PASS')
