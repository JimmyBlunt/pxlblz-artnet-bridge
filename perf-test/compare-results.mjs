import fs from 'node:fs'
import path from 'node:path'

const [baselinePath, candidatePath] = process.argv.slice(2)
if (!baselinePath || !candidatePath) {
  console.error('Usage: node perf-test/compare-results.mjs <baseline-summary.json> <candidate-summary.json>')
  process.exit(2)
}

const baseline = JSON.parse(fs.readFileSync(path.resolve(baselinePath), 'utf8'))
const candidate = JSON.parse(fs.readFileSync(path.resolve(candidatePath), 'utf8'))

function pct(base, next) {
  if (!Number.isFinite(base) || !Number.isFinite(next) || base === 0) return null
  return ((next - base) / base) * 100
}

function row(name, base, next, lowerIsBetter = false) {
  const delta = pct(base, next)
  let trend = ''
  if (delta !== null) {
    const improved = lowerIsBetter ? delta < 0 : delta > 0
    const degraded = lowerIsBetter ? delta > 0 : delta < 0
    trend = improved ? 'better' : degraded ? 'worse' : 'same'
  }
  return { metric: name, baseline: base, candidate: next, deltaPct: delta, trend }
}

const rows = [
  row('probe.frames_per_second', baseline.probe?.frames_per_second, candidate.probe?.frames_per_second),
  row('probe.packets_per_second', baseline.probe?.packets_per_second, candidate.probe?.packets_per_second),
  row('probe.assembly.p99_ms', baseline.probe?.assembly?.p99_ms, candidate.probe?.assembly?.p99_ms, true),
  row('probe.frame_interval.p99_ms', baseline.probe?.frame_interval?.p99_ms, candidate.probe?.frame_interval?.p99_ms, true),
  row('router.sendAvgMsP50', baseline.router?.sendAvgMsP50, candidate.router?.sendAvgMsP50, true),
  row('router.sendAvgMsP95', baseline.router?.sendAvgMsP95, candidate.router?.sendAvgMsP95, true),
  row('router.sendAvgMsP99', baseline.router?.sendAvgMsP99, candidate.router?.sendAvgMsP99, true),
  row('router.sendMaxMsObserved', baseline.router?.sendMaxMsObserved, candidate.router?.sendMaxMsObserved, true),
  row('router.heapMaxMB', baseline.router?.heapMaxMB, candidate.router?.heapMaxMB, true),
  row('router.heapGrowthMB', baseline.router?.heapGrowthMB, candidate.router?.heapGrowthMB, true),
]

const benchNames = new Set([
  ...Object.keys(baseline.microbench ?? {}),
  ...Object.keys(candidate.microbench ?? {}),
])
for (const name of [...benchNames].sort()) {
  rows.push(row(
    `microbench.${name}.nsPerOp`,
    baseline.microbench?.[name]?.nsPerOp,
    candidate.microbench?.[name]?.nsPerOp,
    true,
  ))
  rows.push(row(
    `microbench.${name}.mbPerSec`,
    baseline.microbench?.[name]?.mbPerSec,
    candidate.microbench?.[name]?.mbPerSec,
    false,
  ))
}

const format = value => Number.isFinite(value) ? Number(value).toFixed(3) : 'n/a'
const formatDelta = value => Number.isFinite(value) ? `${value >= 0 ? '+' : ''}${value.toFixed(2)}%` : 'n/a'

console.log(`Baseline:  ${baselinePath}`)
console.log(`Candidate: ${candidatePath}`)
console.log('')
console.log('Metric'.padEnd(54), 'Baseline'.padStart(12), 'Candidate'.padStart(12), 'Delta'.padStart(10), 'Trend'.padStart(8))
console.log('-'.repeat(100))
for (const r of rows) {
  console.log(
    r.metric.padEnd(54),
    format(r.baseline).padStart(12),
    format(r.candidate).padStart(12),
    formatDelta(r.deltaPct).padStart(10),
    String(r.trend).padStart(8),
  )
}

const correctnessKeys = new Set([
  ...Object.keys(baseline.hardChecks ?? {}),
  ...Object.keys(candidate.hardChecks ?? {}),
])
const regressions = []
for (const key of correctnessKeys) {
  if (baseline.hardChecks?.[key] === true && candidate.hardChecks?.[key] !== true) regressions.push(key)
}

console.log('')
console.log(`Baseline pass:  ${baseline.pass}`)
console.log(`Candidate pass: ${candidate.pass}`)
if (regressions.length) {
  console.log(`Correctness regressions: ${regressions.join(', ')}`)
  process.exitCode = 1
} else {
  console.log('Correctness regressions: none')
}
