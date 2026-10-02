// Feasibility spike: FastLED lib8tion (C semantics) vs. Pixelblaze-language port
// executed by the PXLBLZ-IDE's own engine in Fast (float64) and Precise (16.16) mode.
// Read-only use of the IDE source; nothing in the IDE repo is modified.
import { readFileSync, writeFileSync } from 'node:fs'
import { bundle } from '../../PXLBLZ-IDE/src/engine/bundle'
import { loadPattern } from '../../PXLBLZ-IDE/src/engine/loadPattern'
import { createFxShim, createShim, planeShimConfig } from '../../PXLBLZ-IDE/src/engine/shim'

// ---------- Reference: exact C semantics, transcribed from FastLED master
// (src/platforms/shared/scale8.h, trig8.h, src/fl/math/random8.h, beat.h) ----------
const u8 = (x: number) => x & 0xff
const u16 = (x: number) => x & 0xffff
const i16 = (x: number) => (x << 16) >> 16
const i8 = (x: number) => (x << 24) >> 24
const ref = {
  scale8: (i: number, sc: number) => (i * (1 + sc)) >> 8,
  qadd8: (a: number, b: number) => Math.min(a + b, 255),
  qsub8: (a: number, b: number) => Math.max(a - b, 0),
  sin8(theta: number) {
    const T = [0, 49, 49, 41, 90, 27, 117, 10]
    let offset = theta
    if (theta & 0x40) offset = u8(255 - offset)
    offset &= 0x3f
    let secoffset = offset & 0x0f
    if (theta & 0x40) secoffset++
    const s2 = (offset >> 4) * 2
    const b = T[s2], m16 = T[s2 + 1]
    const mx = u8((m16 * secoffset) >> 4)
    let y = i8(mx + b)
    if (theta & 0x80) y = i8(-y)
    y = i8(y + 128)
    return u8(y)
  },
  sin16(theta: number) {
    const base = [0, 6393, 12539, 18204, 23170, 27245, 30273, 32137]
    const slope = [49, 48, 44, 38, 31, 23, 14, 4]
    let offset = (theta & 0x3fff) >> 3
    if (theta & 0x4000) offset = 2047 - offset
    const section = (offset / 256) | 0
    const b = base[section], m = slope[section]
    const secoffset8 = u8(offset) >> 1
    const mx = u16(m * secoffset8)
    let y = i16(mx + b)
    if (theta & 0x8000) y = i16(-y)
    return y
  },
  seed: 1337,
  random16() { this.seed = u16(this.seed * 2053 + 13849); return this.seed },
  random8() { this.random16(); return u8((this.seed & 0xff) + (this.seed >> 8)) },
  beat88(bpm88: number, ms: number) {
    // ((u32 millis * bpm88 * 280) >> 16) as u16 ; u32 wrap
    const p = Math.imul(Math.imul(ms >>> 0, bpm88), 280) >>> 0
    return u16(p >>> 16)
  },
  beat16(bpm: number, ms: number) { if (bpm < 256) bpm <<= 8; return this.beat88(bpm, ms) },
  beat8(bpm: number, ms: number) { return this.beat16(bpm, ms) >> 8 },
  beatsin8(bpm: number, lo: number, hi: number, ms: number, phase: number) {
    const beat = this.beat8(bpm, ms)
    const bs = this.sin8(u8(beat + phase))
    const range = u8(hi - lo)
    return u8(lo + this.scale8(bs, range))
  },
}

// ---------- Candidate ----------
const src = readFileSync(new URL('./fl8-pixelblaze.js', import.meta.url), 'utf8')
const { code, fxCode, metadata } = bundle(src, {})
type Mode = 'fast' | 'precise'
function makeRunner(mode: Mode) {
  const cfg = { ...planeShimConfig({ rows: 1, cols: 1 }), getVirtualTime: () => 0 }
  const shim = mode === 'precise' ? createFxShim(cfg) : createShim(cfg)
  const h = loadPattern(mode === 'precise' ? fxCode : code, metadata, shim.builtins)
  const names = ['P1', 'P2', 'P3', 'P4', 'P5', 'P6'] as const
  return (op: number, ...args: number[]) => {
    args.forEach((v, k) => h.setPatternVar(names[k], shim.encodeScalar(v)))
    h.setPatternVar('MODE', shim.encodeScalar(op))
    h.render(shim.encodeScalar(0))
    return shim.decodeScalar(h.getExports().OUT as number)
  }
}
const asI16 = (x: number) => i16(x)

interface Row { test: string; cases: number; fastMismatch: number; preciseMismatch: number; example?: string }
const rows: Row[] = []
function check(test: string, cases: Iterable<number[]>, refFn: (...a: number[]) => number, op: number,
               toCand: (a: number[]) => number[] = a => a, norm: (v: number) => number = v => v) {
  const fast = makeRunner('fast'), precise = makeRunner('precise')
  let n = 0, fm = 0, pm = 0, example: string | undefined
  for (const a of cases) {
    n++
    const want = refFn(...a)
    const f = norm(fast(op, ...toCand(a)))
    const p = norm(precise(op, ...toCand(a)))
    if (f !== want) { fm++; if (!example) example = `fast ${test}(${a}) = ${f}, want ${want}` }
    if (p !== want) { pm++; example = example ?? `precise ${test}(${a}) = ${p}, want ${want}` }
  }
  rows.push({ test, cases: n, fastMismatch: fm, preciseMismatch: pm, example })
}
function* grid2(n: number, m: number) { for (let a = 0; a < n; a++) for (let b = 0; b < m; b++) yield [a, b] }
function* range1(n: number, step = 1) { for (let a = 0; a < n; a += step) yield [a] }

check('scale8', grid2(256, 256), ref.scale8, 1)
check('qadd8', grid2(256, 256), ref.qadd8, 2)
check('qsub8', grid2(256, 256), ref.qsub8, 3)
check('sin8', range1(256), ref.sin8, 4)
check('sin16', range1(65536), ref.sin16, 5, a => [asI16(a[0])])

// random sequences (stateful): seed both, then draw N
for (const [label, op, refDraw, norm] of [
  ['random16 x100000', 7, () => ref.random16(), (v: number) => u16(v)],
  ['random8 x100000', 6, () => ref.random8(), (v: number) => v],
] as const) {
  for (const mode of ['fast', 'precise'] as Mode[]) void mode
  const fast = makeRunner('fast'), precise = makeRunner('precise')
  ref.seed = 1337; fast(8, 1337); precise(8, 1337)
  let fm = 0, pm = 0, example: string | undefined
  const N = 100000
  for (let k = 0; k < N; k++) {
    const want = refDraw()
    const f = norm(fast(op)), p = norm(precise(op))
    if (f !== want) { fm++; example = example ?? `fast draw#${k} = ${f}, want ${want}` }
    if (p !== want) { pm++; example = example ?? `precise draw#${k} = ${p}, want ${want}` }
  }
  rows.push({ test: label, cases: N, fastMismatch: fm, preciseMismatch: pm, example })
}

// beat8 / beatsin8 over 2^32 ms wrap region and normal uptime
function* beatCases() {
  const bpms = [1, 10, 30, 60, 62, 120, 140, 200, 255]
  const times: number[] = []
  for (let t = 0; t < 600000; t += 997) times.push(t)                 // first 10 min
  for (let t = 86_400_000; t < 86_400_000 + 200000; t += 991) times.push(t) // after 1 day
  for (let t = 4294967295 - 100000; t <= 4294967295; t += 983) times.push(t) // u32 wrap (49.7 d)
  for (const bpm of bpms) for (const t of times) yield [bpm, t]
}
const split = (t: number) => [asI16(t >>> 16), asI16(t & 0xffff)]
check('beat8', beatCases(), (bpm, t) => ref.beat8(bpm, t), 9, ([bpm, t]) => [bpm, ...split(t)])
check('beatsin8', (function* () { for (const [bpm, t] of beatCases()) yield [bpm, 20, 230, t, 64] })(),
  (bpm, lo, hi, t, ph) => ref.beatsin8(bpm, lo, hi, t, ph), 10,
  ([bpm, lo, hi, t, ph]) => [bpm, lo, hi, ...split(t), ph])

// "naive" Pixelblaze idioms people use when porting by hand (forum approach)
function naiveCompare() {
  // beatsin8 via wave(time()) approximation as commonly posted
  let maxErr = 0, mism = 0, n = 0
  for (const [bpm, t] of beatCases()) {
    if (t > 600000) continue
    n++
    const want = ref.beatsin8(bpm, 20, 230, t, 64)
    const phase = ((t / 60000) * bpm + 64 / 256) % 1
    const w = (1 + Math.sin(phase * 2 * Math.PI)) / 2
    const got = Math.floor(20 + w * 210)
    const e = Math.abs(got - want)
    if (e) mism++
    maxErr = Math.max(maxErr, e)
  }
  rows.push({ test: 'beatsin8 naive wave()-idiom (float)', cases: n, fastMismatch: mism, preciseMismatch: NaN, example: `max abs error ${maxErr} levels` })
}
naiveCompare()

const lines = ['| Test | Fälle | Abweichungen Fast (float64) | Abweichungen Precise (16.16) | Beispiel |', '|---|---:|---:|---:|---|']
for (const r of rows) lines.push(`| ${r.test} | ${r.cases} | ${r.fastMismatch} | ${Number.isNaN(r.preciseMismatch) ? '–' : r.preciseMismatch} | ${r.example ?? ''} |`)
const out2 = lines.join('\n')
console.log(out2)

// ---------- Validate the JS C-semantics reference against the real, natively
// compiled FastLED (native-math-hashes.txt from ref-native-math.exe) ----------
{
  const OFF = 1469598103934665603n, PRIME = 1099511628211n, MASK = (1n << 64n) - 1n
  let h = OFF
  const mix = (v: number) => { for (let k = 0; k < 4; k++) { h ^= BigInt((v >>> (8 * k)) & 0xff); h = (h * PRIME) & MASK } }
  const out: Record<string, string> = {}
  const done = (name: string) => { out[name] = h.toString(16).padStart(16, '0'); h = OFF }
  for (let a = 0; a < 256; a++) for (let b = 0; b < 256; b++) mix(ref.scale8(a, b)); done('scale8')
  for (let a = 0; a < 256; a++) for (let b = 0; b < 256; b++) mix(ref.qadd8(a, b)); done('qadd8')
  for (let a = 0; a < 256; a++) for (let b = 0; b < 256; b++) mix(ref.qsub8(a, b)); done('qsub8')
  for (let a = 0; a < 256; a++) mix(ref.sin8(a)); done('sin8')
  for (let a = 0; a < 65536; a++) mix(u16(ref.sin16(a))); done('sin16')
  ref.seed = 1337; for (let k = 0; k < 100000; k++) mix(ref.random16()); done('random16')
  ref.seed = 1337; for (let k = 0; k < 100000; k++) mix(ref.random8()); done('random8')
  for (const [bpm, t] of beatCases()) mix(ref.beat8(bpm, t)); done('beat8')
  for (const [bpm, t] of beatCases()) mix(ref.beatsin8(bpm, 20, 230, t, 64)); done('beatsin8')
  let nativeTxt = ''
  try { nativeTxt = readFileSync(new URL('./native-math-hashes.txt', import.meta.url), 'utf8') } catch { /* not built */ }
  const native = Object.fromEntries(nativeTxt.trim().split(/\r?\n/).filter(Boolean).map(l => l.trim().split(/\s+/)))
  const vl = ['', '| Funktion | JS-Referenz FNV | natives FastLED FNV | identisch |', '|---|---|---|---|']
  for (const k of Object.keys(out)) vl.push(`| ${k} | ${out[k]} | ${native[k] ?? '–'} | ${native[k] ? (native[k] === out[k] ? 'ja' : '**NEIN**') : '–'} |`)
  console.log(vl.join('\n'))
  writeFileSync(new URL('./spike-results.md', import.meta.url), out2 + '\n' + vl.join('\n') + '\n')
}
