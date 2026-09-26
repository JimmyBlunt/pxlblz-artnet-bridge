import { spawn, spawnSync } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import process from 'node:process'
import { fileURLToPath } from 'node:url'

const here = path.dirname(fileURLToPath(import.meta.url))
const repoRoot = path.resolve(here, '..')
const routerRoot = path.join(repoRoot, 'router')
const adapterSource = path.join(repoRoot, 'pxlblz-integration', 'src', 'externalPixelOutput.ts')
const driverSource = path.join(repoRoot, 'pxlblz-integration', 'virtual-test', 'virtual-pxlblz-driver.mjs')

function arg(name, fallback) {
  const i = process.argv.indexOf(`--${name}`)
  return i >= 0 ? process.argv[i + 1] : fallback
}

const profiles = {
  smoke:  { pixels: 8186,  inputFps: 60,  outputFps: 30,  seconds: 10, config: 'backpanel' },
  perf:   { pixels: 8186,  inputFps: 120, outputFps: 60,  seconds: 60, config: 'backpanel' },
  soak:   { pixels: 8186,  inputFps: 60,  outputFps: 30,  seconds: 900, config: 'backpanel' },
  stress: { pixels: 32768, inputFps: 120, outputFps: 120, seconds: 60, config: 'synthetic' },
}

const profileName = arg('profile', 'smoke')
if (!profiles[profileName]) throw new Error(`unknown --profile ${profileName}; use smoke|perf|soak|stress`)
const base = profiles[profileName]
const settings = {
  pixels: Number(arg('pixels', base.pixels)),
  inputFps: Number(arg('input-fps', base.inputFps)),
  outputFps: Number(arg('output-fps', base.outputFps)),
  seconds: Number(arg('seconds', base.seconds)),
  config: base.config,
}
if (!Number.isInteger(settings.pixels) || settings.pixels <= 0) throw new Error('pixels must be a positive integer')
for (const key of ['inputFps', 'outputFps', 'seconds']) {
  if (!Number.isFinite(settings[key]) || settings[key] <= 0) throw new Error(`${key} must be > 0`)
}

const stamp = new Date().toISOString().replace(/[:.]/g, '-')
const resultRoot = path.resolve(arg('out', path.join(here, 'results', `${stamp}-${profileName}-${process.platform}`)))
const tmp = path.join(resultRoot, 'tmp')
const build = path.join(tmp, 'build')
fs.mkdirSync(build, { recursive: true })

const udpPort = Number(arg('udp-port', '16454'))
const wsPort = Number(arg('ws-port', '19980'))
const configPath = path.join(tmp, 'routes.json')
const probeReportPath = path.join(resultRoot, 'probe.json')
const routerLogPath = path.join(resultRoot, 'router.log')
const probeLogPath = path.join(resultRoot, 'probe.log')
const driverLogPath = path.join(resultRoot, 'driver.log')
const benchPath = path.join(resultRoot, 'microbench.txt')
const adapterBenchPath = path.join(resultRoot, 'adapter-microbench.json')
const samplesPath = path.join(resultRoot, 'router-samples.csv')
const summaryPath = path.join(resultRoot, 'summary.json')
const machinePath = path.join(resultRoot, 'machine.json')

function run(command, args, opts = {}) {
  const r = spawnSync(command, args, {
    cwd: opts.cwd ?? repoRoot,
    encoding: 'utf8',
    shell: opts.shell ?? false,
    maxBuffer: 64 * 1024 * 1024,
  })
  if (r.status !== 0) {
    throw new Error(`${command} ${args.join(' ')} failed (${r.status})\nSTDOUT:\n${r.stdout ?? ''}\nSTDERR:\n${r.stderr ?? ''}`)
  }
  return r
}

function start(command, args, cwd) {
  const child = spawn(command, args, { cwd, stdio: ['ignore', 'pipe', 'pipe'] })
  let stdout = ''
  let stderr = ''
  child.stdout.on('data', d => { stdout += d.toString(); process.stdout.write(d) })
  child.stderr.on('data', d => { stderr += d.toString(); process.stderr.write(d) })
  return { child, get stdout() { return stdout }, get stderr() { return stderr } }
}

function wait(ms) { return new Promise(resolve => setTimeout(resolve, ms)) }

async function waitForText(proc, text, timeoutMs = 10000) {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    if (proc.stdout.includes(text) || proc.stderr.includes(text)) return
    if (proc.child.exitCode !== null) throw new Error(`process exited before '${text}'\n${proc.stdout}\n${proc.stderr}`)
    await wait(50)
  }
  throw new Error(`timeout waiting for '${text}'`)
}

async function stop(proc) {
  if (!proc || proc.child.exitCode !== null) return
  proc.child.kill('SIGTERM')
  await Promise.race([new Promise(resolve => proc.child.once('exit', resolve)), wait(1500)])
  if (proc.child.exitCode === null) proc.child.kill('SIGKILL')
}

function percentile(values, q) {
  if (!values.length) return 0
  const sorted = [...values].sort((a,b) => a-b)
  const idx = Math.max(0, Math.min(sorted.length - 1, Math.round((sorted.length - 1) * q)))
  return sorted[idx]
}

function average(values) {
  return values.length ? values.reduce((a,b) => a+b, 0) / values.length : 0
}

function expectedUniverseCount(cfg) {
  return cfg.routes.filter(r => r.enabled).reduce((sum, r) => sum + Math.ceil(r.pixel_count / 170), 0)
}

function parseBenchmarks(text) {
  const result = {}
  const line = /^(Benchmark\S+?)(?:-\d+)?\s+\d+\s+([\d.]+)\s+ns\/op\s+([\d.]+)\s+MB\/s\s+(\d+)\s+B\/op\s+(\d+)\s+allocs\/op$/gm
  for (const m of text.matchAll(line)) {
    result[m[1]] = {
      nsPerOp: Number(m[2]),
      mbPerSec: Number(m[3]),
      bytesPerOp: Number(m[4]),
      allocsPerOp: Number(m[5]),
    }
  }
  return result
}

function createConfig() {
  if (settings.config === 'backpanel') {
    const source = JSON.parse(fs.readFileSync(path.join(routerRoot, 'config', 'routes.backpanel-all.json'), 'utf8'))
    source.input.pixel_count = settings.pixels
    source.input.fps_target = settings.outputFps
    source.artnet.udp_port = udpPort
    for (const route of source.routes) route.target_ip = '127.0.0.1'
    return source
  }
  return {
    version: 1,
    input: { pixel_count: settings.pixels, fps_target: settings.outputFps },
    artnet: { udp_port: udpPort, unicast: true },
    routes: [{
      name: 'PERF_STRESS',
      enabled: true,
      target_ip: '127.0.0.1',
      physical_port: 1,
      pixel_start: 0,
      pixel_count: settings.pixels,
      universe_start: 0,
      color_order: 'RGB',
    }],
  }
}

const machine = {
  generatedAt: new Date().toISOString(),
  platform: process.platform,
  arch: process.arch,
  node: process.version,
  cpus: os.cpus().length,
  cpuModel: os.cpus()[0]?.model ?? null,
  totalMemoryGB: Number((os.totalmem() / 1024 / 1024 / 1024).toFixed(2)),
  hostname: os.hostname(),
}
fs.writeFileSync(machinePath, JSON.stringify(machine, null, 2) + '\n')

const cfg = createConfig()
fs.writeFileSync(configPath, JSON.stringify(cfg, null, 2) + '\n')
const universes = expectedUniverseCount(cfg)
const expectedPps = universes * settings.outputFps

console.log('=== Performance profile ===')
console.log(JSON.stringify({ profile: profileName, ...settings, universes, expectedPps, resultRoot }, null, 2))

console.log('=== Compile exact PXLBLZ adapter ===')
run('tsc', [
  adapterSource,
  '--target', 'ES2022', '--module', 'ES2022', '--moduleResolution', 'bundler',
  '--lib', 'ES2022,DOM', '--outDir', build,
], { shell: process.platform === 'win32' })

console.log('=== Exact PXLBLZ adapter conversion microbenchmark ===')
const adapterBenchRun = run(process.execPath, [
  path.join(here, 'adapter-benchmark.mjs'),
  '--module', path.join(build, 'externalPixelOutput.js'),
  '--pixels', String(settings.pixels),
])
const adapterBenchMatch = /^ADAPTER_BENCHMARK_JSON (.+)$/m.exec(adapterBenchRun.stdout)
if (!adapterBenchMatch) throw new Error('adapter microbenchmark did not emit JSON')
const adapterMicrobench = JSON.parse(adapterBenchMatch[1])
fs.writeFileSync(adapterBenchPath, JSON.stringify(adapterMicrobench, null, 2) + '\n')
console.log(JSON.stringify(adapterMicrobench, null, 2))

// The shared virtual driver imports ./build/externalPixelOutput.js from its own
// directory. Copy it into this result-local harness so every run is immutable.
const driverLocal = path.join(tmp, 'driver.mjs')
let driverText = fs.readFileSync(driverSource, 'utf8')
driverText = driverText.replace("'./build/externalPixelOutput.js'", "'./build/externalPixelOutput.js'")
fs.writeFileSync(driverLocal, driverText)

console.log('=== Router microbenchmarks ===')
const routerBench = run('go', [
  'test', '-run', '^$', '-bench', 'BenchmarkSendFrameDryRun', '-benchmem',
  '-benchtime=1s', './internal/router',
], { cwd: routerRoot })
const wsBench = run('go', [
  'test', '-run', '^$', '-bench', 'BenchmarkReadFrameIntoReuse98K', '-benchmem',
  '-benchtime=1s', './internal/wsmini',
], { cwd: routerRoot })
const benchText = routerBench.stdout + routerBench.stderr + '\n' + wsBench.stdout + wsBench.stderr
fs.writeFileSync(benchPath, benchText)
process.stdout.write(routerBench.stdout)
process.stdout.write(wsBench.stdout)
const microbench = parseBenchmarks(benchText)

const routerBin = path.join(tmp, process.platform === 'win32' ? 'pxlblz-router.exe' : 'pxlblz-router')
const probeBin = path.join(tmp, process.platform === 'win32' ? 'artnet-probe.exe' : 'artnet-probe')
run('go', ['build', '-o', routerBin, './cmd/pxlblz-router'], { cwd: routerRoot })
run('go', ['build', '-o', probeBin, './cmd/artnet-probe'], { cwd: routerRoot })

let probe
let router
let driver
try {
  console.log('=== Start frame-aware Art-Net probe ===')
  probe = start(probeBin, [
    '--config', configPath,
    '--bind', `127.0.0.1:${udpPort}`,
    '--duration', `${settings.seconds}s`,
    '--report', probeReportPath,
    '--quiet',
  ], routerRoot)
  await waitForText(probe, 'Art-Net performance probe')

  console.log('=== Start router ===')
  router = start(routerBin, [
    '--config', configPath,
    '--input', 'ws',
    '--fps', String(settings.outputFps),
    '--ws-listen', `127.0.0.1:${wsPort}`,
    '--duration', `${settings.seconds + 4}s`,
  ], routerRoot)
  await waitForText(router, 'Waiting for first valid input frame')
  await wait(250)

  console.log('=== Start exact PXLBLZ adapter load ===')
  driver = start(process.execPath, [
    driverLocal,
    '--pixels', String(settings.pixels),
    '--fps', String(settings.inputFps),
    '--seconds', String(settings.seconds + 2),
    '--url', `ws://127.0.0.1:${wsPort}/pixels`,
    '--mode', 'static',
  ], tmp)

  await new Promise((resolve, reject) => {
    probe.child.once('exit', code => code === 0 ? resolve() : reject(new Error(`probe exited ${code}`)))
  })
  await new Promise((resolve, reject) => {
    if (driver.child.exitCode !== null) return driver.child.exitCode === 0 ? resolve() : reject(new Error(`driver exited ${driver.child.exitCode}`))
    driver.child.once('exit', code => code === 0 ? resolve() : reject(new Error(`driver exited ${code}`)))
  })
  await new Promise((resolve, reject) => {
    if (router.child.exitCode !== null) return router.child.exitCode === 0 ? resolve() : reject(new Error(`router exited ${router.child.exitCode}`))
    router.child.once('exit', code => code === 0 ? resolve() : reject(new Error(`router exited ${code}`)))
  })
} finally {
  fs.writeFileSync(routerLogPath, (router?.stdout ?? '') + (router?.stderr ?? ''))
  fs.writeFileSync(probeLogPath, (probe?.stdout ?? '') + (probe?.stderr ?? ''))
  fs.writeFileSync(driverLogPath, (driver?.stdout ?? '') + (driver?.stderr ?? ''))
  await stop(driver)
  await stop(router)
  await stop(probe)
}

const probeReport = JSON.parse(fs.readFileSync(probeReportPath, 'utf8'))

const routerSamples = []
const rx = /RX\s+([\d.]+) fps \| replaced\s+(\d+)\/s invalid (\d+)\/s clients (\d+) \| TX\s+([\d.]+) fps\s+(\d+) pkt\/s \| send avg\s+([\d.]+) ms last\s+([\d.]+) ms max\s+([\d.]+) ms \| heap\s+([\d.]+) MB gc (\d+) goroutines (\d+) \| errors (\d+)/g
for (const match of router.stdout.matchAll(rx)) {
  routerSamples.push({
    sample: routerSamples.length + 1,
    rxFps: Number(match[1]),
    replacedPerSec: Number(match[2]),
    invalidPerSec: Number(match[3]),
    clients: Number(match[4]),
    txFps: Number(match[5]),
    packetsPerSec: Number(match[6]),
    sendAvgMs: Number(match[7]),
    sendLastMs: Number(match[8]),
    sendMaxMs: Number(match[9]),
    heapMB: Number(match[10]),
    gc: Number(match[11]),
    goroutines: Number(match[12]),
    errors: Number(match[13]),
  })
}
if (!routerSamples.length) throw new Error('no router telemetry samples parsed')

const csvHeader = Object.keys(routerSamples[0])
const csv = [csvHeader.join(','), ...routerSamples.map(s => csvHeader.map(k => s[k]).join(','))].join('\n') + '\n'
fs.writeFileSync(samplesPath, csv)

const sendAvg = routerSamples.map(s => s.sendAvgMs).filter(v => v > 0)
const heaps = routerSamples.map(s => s.heapMB)
const framePeriodMs = 1000 / settings.outputFps
const adapterMatch = /VIRTUAL_PXLBLZ_DONE pixels=(\d+) produced=(\d+) elapsed=([\d.]+) avg_fps=([\d.]+) mode=(\w+) sent=(\d+) skipped=(\d+) not_connected=(\d+) wrong_size=(\d+) connect_attempts=(\d+)/.exec(driver.stdout)
const finalRx = /Final RX: (\d+) valid frames, (\d+) replaced before output observation, (\d+) invalid/.exec(router.stdout)
const finalTx = /Final TX: (\d+) frames, (\d+) packets, ([\d.]+) avg fps, ([\d.]+) packets\/s, ([\d.]+) avg send ms, ([\d.]+) max send ms, ([\d.]+) MB heap, (\d+) send errors/.exec(router.stdout)

const hardChecks = {
  probeInvalidPackets: probeReport.invalid_packets === 0,
  probeUnexpectedUniverses: probeReport.unexpected_universes === 0,
  probePayloadMismatches: probeReport.payload_mismatches === 0,
  probeIncompleteFrames: probeReport.incomplete_frames === 0,
  probeDuplicateUniverses: probeReport.duplicate_universes === 0,
  probeSequenceGaps: probeReport.sequence_gap_events === 0,
  probeLateSequencePackets: probeReport.late_same_sequence_packets === 0,
  frameRate: probeReport.frames_per_second >= settings.outputFps * 0.97,
  packetRate: probeReport.packets_per_second >= expectedPps * 0.97,
  routerInvalidInput: finalRx ? Number(finalRx[3]) === 0 : false,
  routerSendErrors: finalTx ? Number(finalTx[8]) === 0 : routerSamples.every(s => s.errors === 0),
  sendDuty: percentile(sendAvg, .99) < framePeriodMs * 0.75,
  assemblyP99: probeReport.assembly.p99_ms < Math.min(25, framePeriodMs * 0.90),
  adapterDuty: adapterMicrobench.msPerFrame < framePeriodMs * 0.75,
  adapterWrongSize: adapterMatch ? Number(adapterMatch[9]) === 0 : false,
}
const warnings = []
const heapGrowthMB = heaps.length ? heaps.at(-1) - heaps[0] : 0
if (heapGrowthMB > 32) warnings.push(`router heap grew by ${heapGrowthMB.toFixed(1)} MB`)
if (probeReport.frame_interval.p99_ms > framePeriodMs * 1.5) {
  warnings.push(`frame interval p99 ${probeReport.frame_interval.p99_ms.toFixed(3)} ms exceeds 1.5x target period`)
}
if (adapterMatch) {
  const produced = Number(adapterMatch[2])
  const skipped = Number(adapterMatch[7])
  const notConnected = Number(adapterMatch[8])
  if (produced > 0 && skipped / produced > 0.05) {
    warnings.push(`adapter backpressure skipped ${skipped}/${produced} frames (${(skipped / produced * 100).toFixed(1)}%)`)
  }
  if (notConnected > 2) warnings.push(`adapter observed ${notConnected} not-connected send attempts`)
}

const summary = {
  profile: profileName,
  settings,
  expected: {
    universesPerFrame: universes,
    packetsPerSecond: expectedPps,
    framePeriodMs,
  },
  machine,
  adapterMicrobench,
  microbench,
  adapter: adapterMatch ? {
    pixels: Number(adapterMatch[1]),
    producedFrames: Number(adapterMatch[2]),
    elapsedSeconds: Number(adapterMatch[3]),
    averageFps: Number(adapterMatch[4]),
    mode: adapterMatch[5],
    sentFrames: Number(adapterMatch[6]),
    skippedBackpressure: Number(adapterMatch[7]),
    notConnected: Number(adapterMatch[8]),
    wrongSize: Number(adapterMatch[9]),
    connectAttempts: Number(adapterMatch[10]),
    sendRatio: Number(adapterMatch[2]) > 0 ? Number(adapterMatch[6]) / Number(adapterMatch[2]) : 0,
  } : null,
  router: {
    telemetrySamples: routerSamples.length,
    rxFpsAvg: average(routerSamples.map(s => s.rxFps)),
    txFpsAvg: average(routerSamples.map(s => s.txFps)),
    packetsPerSecAvg: average(routerSamples.map(s => s.packetsPerSec)),
    sendAvgMsP50: percentile(sendAvg, .50),
    sendAvgMsP95: percentile(sendAvg, .95),
    sendAvgMsP99: percentile(sendAvg, .99),
    sendMaxMsObserved: Math.max(...routerSamples.map(s => s.sendMaxMs)),
    heapFirstMB: heaps[0],
    heapLastMB: heaps.at(-1),
    heapMaxMB: Math.max(...heaps),
    heapGrowthMB,
    gcLast: routerSamples.at(-1).gc,
    goroutinesMax: Math.max(...routerSamples.map(s => s.goroutines)),
  },
  probe: probeReport,
  hardChecks,
  warnings,
  pass: Object.values(hardChecks).every(Boolean),
  files: {
    summary: summaryPath,
    samples: samplesPath,
    probe: probeReportPath,
    routerLog: routerLogPath,
    probeLog: probeLogPath,
    driverLog: driverLogPath,
    microbench: benchPath,
    adapterMicrobench: adapterBenchPath,
    machine: machinePath,
  },
}
fs.writeFileSync(summaryPath, JSON.stringify(summary, null, 2) + '\n')

console.log('=== PERFORMANCE SUMMARY ===')
console.log(JSON.stringify(summary, null, 2))
console.log(`PERF_RESULT ${summary.pass ? 'PASS' : 'FAIL'} profile=${profileName} results=${resultRoot}`)
if (!summary.pass) process.exitCode = 1
