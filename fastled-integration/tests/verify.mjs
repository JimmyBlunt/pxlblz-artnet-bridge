// Golden-frame verification: native FastLED (x86_64, stub platform, Win32 fiber) vs. the
// wasm32 build (Asyncify, fastledWasmHost.ts) for unmodified FastLED examples, with the same
// virtual frame schedule, seed and UI events. Prints a table and writes RESULTS.md.
//   node tests/verify.mjs [--frames 1200] [--only Fire2012,Blink] [--no-md]
import fs from 'node:fs';
import path from 'node:path';
import { loadConfig, buildLib, compileSketch, toolInfo, ROOT, run } from '../toolchain/toolchain.mjs';
import { makeSchedule, writeSchedule, runWasmTrace, compareTraces, parseEvents } from './trace.mjs';

const args = process.argv.slice(2);
const opt = (k, d) => { const i = args.indexOf(k); return i >= 0 ? args[i + 1] : d; };
const FRAMES = +opt('--frames', 1200);
const ONLY = opt('--only', '');
const SEED = 1;
const cfg = loadConfig();
const ex = (n) => path.join(cfg.fastledDir, 'examples', n, n + '.ino');
const CASES = [
  ...['Blink', 'Cylon', 'Fire2012', 'Fire2012WithPalette', 'ColorPalette', 'DemoReel100', 'Pride2015',
    'Pacifica', 'TwinkleFox', 'NoisePlusPalette', 'XYMatrix', 'Noise', 'RGBCalibrate'].map((n) => ({ name: n, file: ex(n) })),
  { name: 'PipelineTest', file: path.join(ROOT, 'tests', 'sketches', 'PipelineTest.ino'),
    events: '300 {"Speed": 120}\n450 {"Sparkle": false}\n600 {"Hue shift": -40, "Brightness": 77}\n900 {"Brightness": 255}' },
  { name: 'PipelineTest@4593', file: path.join(ROOT, 'tests', 'sketches', 'PipelineTest.ino'), defines: { PXLBLZ_NUM_LEDS: 4593 } },
].filter((c) => !ONLY || ONLY.split(',').includes(c.name));

const work = path.join(cfg.buildDir, 'verify');
fs.mkdirSync(work, { recursive: true });
const log = (s) => process.stdout.write(s + '\n');
log(`FastLED ${toolInfo(cfg).fastled.commit.slice(0, 7)} (${toolInfo(cfg).fastled.version}), zig ${toolInfo(cfg).zig}, ${FRAMES} frames, seed ${SEED}`);
await buildLib(cfg, 'wasm', { log });
await buildLib(cfg, 'native', { log });

const schedule = makeSchedule(FRAMES);
const schedFile = path.join(work, 'schedule.bin');
writeSchedule(schedFile, schedule);

const rows = [];
for (const c of CASES) {
  if (!fs.existsSync(c.file)) { rows.push({ name: c.name, error: 'example not found' }); continue; }
  const source = fs.readFileSync(c.file, 'utf8');
  const defines = c.defines || {};
  const dir = path.dirname(c.file);
  const files = Object.fromEntries(fs.readdirSync(dir).filter((x) => /\.(h|hpp|cpp|c)$/.test(x)).map((x) => [x, fs.readFileSync(path.join(dir, x), 'utf8')]));
  const evFile = path.join(work, c.name.replace(/\W/g, '_') + '.events.txt');
  fs.writeFileSync(evFile, c.events || '');
  const [w, n] = await Promise.all([
    compileSketch(cfg, { source, defines, files, target: 'wasm' }),
    compileSketch(cfg, { source, defines, files, target: 'native' }),
  ]);
  if (!w.ok || !n.ok) {
    const d = (w.ok ? n : w).diagnostics.filter((x) => x.severity === 'error').slice(0, 3).map((x) => `${x.file}:${x.line}: ${x.message}`).join(' | ');
    rows.push({ name: c.name, error: `compile failed (${w.ok ? 'native' : 'wasm'}): ${d}` });
    log(`${c.name}: ${rows.at(-1).error}`);
    continue;
  }
  const nativeTrace = path.join(work, c.name.replace(/\W/g, '_') + '.native.trace');
  const r = await run(n.path, [schedFile, nativeTrace, evFile, String(SEED), 'eager'], {});
  if (r.code !== 0) { rows.push({ name: c.name, error: `native run failed (${r.code}): ${r.err.slice(0, 200)}` }); log(rows.at(-1).error); continue; }
  const wasm = fs.readFileSync(w.path);
  const t0 = performance.now();
  const { trace, frameMs } = await runWasmTrace(wasm, schedule, parseEvents(c.events), SEED, { timing: true });
  const wallMs = performance.now() - t0;
  fs.writeFileSync(path.join(work, c.name.replace(/\W/g, '_') + '.wasm.trace'), trace);
  const cmp = compareTraces(fs.readFileSync(nativeTrace), trace);
  const row = { name: c.name, ...cmp, wasmSize: w.size, frameMsAvg: frameMs / FRAMES, wallMs, compileMs: w.timings.totalMs };
  rows.push(row);
  log(`${c.name.padEnd(22)} frames ${cmp.frames}  leds ${String(cmp.leds).padStart(4)}  shows ${String(cmp.shows).padStart(6)}  ` +
    `L1 diff ${cmp.l1Diff}/${cmp.l1Bytes}  L2 diff ${cmp.l2Diff}/${cmp.l2Bytes}  raw diff ${cmp.rawDiff}  meta diff ${cmp.metaDiff}  ` +
    `(L2!=L1: ${(100 * cmp.l2NeL1 / Math.max(1, cmp.l1Bytes)).toFixed(1)}%, ${row.frameMsAvg.toFixed(3)} ms/frame)`);
}

const pass = rows.every((r) => !r.error && r.l1Diff === 0 && r.l2Diff === 0 && r.rawDiff === 0 && r.metaDiff === 0 && !r.frameCountMismatch);
log(pass ? '\nRESULT: PASS (0 differing bytes)' : '\nRESULT: FAIL');

if (!args.includes('--no-md')) {
  const info = toolInfo(cfg);
  // UTF-8 without BOM; ASCII punctuation only (umlauts are UTF-8). Windows PowerShell 5.1 needs
  // "Get-Content -Encoding UTF8 RESULTS.md" to display it correctly.
  const lines = [
    '# Golden-Frame-Verifikation (generiert von `npm run verify`)', '',
    `Stand: ${new Date().toISOString()} | FastLED ${info.fastled.commit.slice(0, 10)} (${info.fastled.version}${info.fastled.localPatch ? ', lokaler Patch ' + info.fastled.localPatch : ''}) | zig ${info.zig} | Node ${info.node}`, '',
    'Referenz: dasselbe unveränderte Beispiel nativ (x86_64-windows-gnu, Stub-Plattform, gleiche Defines, Win32-Fiber, ' +
    'FastLEDs eager Encode-Pfad). Kandidat: wasm32-wasi + Asyncify über runtime/fastledWasmHost.ts in Node (Lazy-Encode). ' +
    `${FRAMES} Frames, identischer Frame-Zeitplan (ca. 60 fps mit Jitter, dt=0-Frames inkl. Frame 0, 250-ms-Hänger), Seed ${SEED}, ` +
    'UI-Ereignisse bei PipelineTest. Verglichen wird pro Frame: Status, show()-Zähler, virtuelle Uhr (µs), ' +
    'L1 (leds[] beim letzten show()), L2 RGB (getWire()) und L2 roh (Leitungsreihenfolge, getWireRaw()).', '',
    '| Sketch | Frames | LEDs | show()-Aufrufe | virt. Zeit | L1 abweichend | L2 (RGB) abweichend | L2 roh abweichend | Meta abweichend | L2 != L1 | wasm | ms/Frame (Node, ohne getWire) |',
    '|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|',
    ...rows.map((r) => r.error ? `| ${r.name} | - | - | - | - | **${r.error}** | | | | | | |`
      : `| ${r.name} | ${r.frames} | ${r.leds} | ${r.shows} | ${(r.virtualMs / 1000).toFixed(1)} s | **${r.l1Diff}** / ${r.l1Bytes} | **${r.l2Diff}** / ${r.l2Bytes} | **${r.rawDiff}** / ${r.rawBytes} | ${r.metaDiff} | ${(100 * r.l2NeL1 / Math.max(1, r.l1Bytes)).toFixed(1)} % | ${(r.wasmSize / 1024).toFixed(0)} KB | ${r.frameMsAvg.toFixed(3)} |`),
    '', `**Ergebnis: ${pass ? 'PASS - 0 abweichende Bytes' : 'FAIL'}**`, '',
    '"L2 != L1" = Anteil der Bytes, in denen die Leitungsdaten (nach Helligkeit, Farbkorrektur/-temperatur, Dithering) von leds[] abweichen; zeigt, dass die Pipeline tatsächlich angewendet wird. ' +
    'RGBCalibrate hat im Original alle addLeds()-Zeilen auskommentiert (0 LEDs).', '',
  ];
  fs.writeFileSync(path.join(ROOT, 'RESULTS.md'), lines.join('\n'), { encoding: 'utf8' });
}
process.exit(pass ? 0 : 1);
