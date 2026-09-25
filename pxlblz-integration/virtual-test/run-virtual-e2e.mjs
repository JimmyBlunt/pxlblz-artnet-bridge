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
  const r = spawnSync(command, args, { cwd: opts.cwd ?? repoRoot, encoding: 'utf8', stdio: opts.stdio ?? 'pipe' })
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
const tscCommand = process.platform === 'win32' ? 'tsc.cmd' : 'tsc'
run(tscCommand, [
  path.join(repoRoot, 'pxlblz-integration', 'src', 'externalPixelOutput.ts'),
  '--target', 'ES2022', '--module', 'ES2022', '--moduleResolution', 'bundler',
  '--lib', 'ES2022,DOM', '--outDir', buildDir,
])

console.log('=== Adapter unit/self-test ===')
run(process.execPath, [path.join(here, 'adapter-selftest.mjs')], { stdio: 'inherit' })

console.log('=== Go unit tests ===')
run('go', ['test', './...'], { cwd: routerRoot, stdio: 'inherit' })

const routerBin = path.join(tmpDir, process.platform === 'win32' ? 'pxlblz-router.exe' : 'pxlblz-router')
const listenerBin = path.join(tmpDir, process.platform === 'win32' ? 'artnet-listener.exe' : 'artnet-listener')
run('go', ['build', '-o', routerBin, './cmd/pxlblz-router'], { cwd: routerRoot })
run('go', ['build', '-o', listenerBin, './cmd/artnet-listener'], { cwd: routerRoot })

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

console.log('ALL_VIRTUAL_TESTS_PASS')
