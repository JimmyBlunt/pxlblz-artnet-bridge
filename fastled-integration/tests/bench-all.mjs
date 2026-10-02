// Compile-time and frame-time benchmark at the installation size (4,593 LEDs).
//   node tests/bench-all.mjs [--leds 4593] [--frames 600]
// Examples are compiled with their NUM_LEDS replaced by --leds (text substitution only).
import fs from 'node:fs';
import path from 'node:path';
import { loadConfig, compileSketch, ROOT } from '../toolchain/toolchain.mjs';
import { instantiateFastLedHost } from '../runtime/fastledWasmHost.ts';

const args = process.argv.slice(2);
const opt = (k, d) => { const i = args.indexOf(k); return i >= 0 ? args[i + 1] : d; };
const LEDS = +opt('--leds', 4593), FRAMES = +opt('--frames', 600);
const cfg = loadConfig();
const ex = (n) => path.join(cfg.fastledDir, 'examples', n, n + '.ino');
const cases = ['Pride2015', 'Pacifica', 'Fire2012WithPalette', 'DemoReel100', 'TwinkleFox', 'ColorPalette', 'Cylon']
  .map((n) => ({ name: n, src: fs.readFileSync(ex(n), 'utf8').replace(/#define\s+NUM_LEDS\s+\d+/, `#define NUM_LEDS ${LEDS}`) }));
cases.push({ name: 'PipelineTest', src: fs.readFileSync(path.join(ROOT, 'tests/sketches/PipelineTest.ino'), 'utf8'), defines: { PXLBLZ_NUM_LEDS: LEDS - 40 } });

const rows = [];
for (const c of cases) {
  const uniq = `// bench ${Date.now()} ${Math.random()}\n`;
  const cold = await compileSketch(cfg, { source: uniq + c.src, defines: c.defines || {} });
  if (!cold.ok) { console.log(c.name, 'compile failed', cold.diagnostics.slice(0, 3)); continue; }
  const warm = await compileSketch(cfg, { source: uniq + c.src, defines: c.defines || {} });
  const host = await instantiateFastLedHost(fs.readFileSync(cold.path));
  host.init(1);
  for (let i = 0; i < 30; i++) host.frame(16.667);
  const t = new Float64Array(FRAMES);
  for (let i = 0; i < FRAMES; i++) {
    const t0 = performance.now();
    host.frame(16.667);
    host.getWire();
    t[i] = performance.now() - t0;
  }
  t.sort();
  const avg = t.reduce((a, b) => a + b, 0) / FRAMES;
  const row = { name: c.name, leds: host.ledCount(), compileMs: cold.timings.totalMs, cc: cold.timings.compileMs, link: cold.timings.linkMs,
    asyncify: cold.timings.asyncifyMs, cachedMs: warm.timings.totalMs, sizeKB: cold.size / 1024, avg, p50: t[FRAMES >> 1], p99: t[Math.floor(FRAMES * 0.99)],
    showsPerFrame: host.showCount() / (FRAMES + 30) };
  rows.push(row);
  console.log(`${c.name.padEnd(20)} leds ${row.leds}  compile ${row.compileMs.toFixed(0)} ms (cc ${row.cc.toFixed(0)}, link ${row.link.toFixed(0)}, asyncify ${row.asyncify.toFixed(0)}), cached ${row.cachedMs.toFixed(0)} ms, ${row.sizeKB.toFixed(0)} KB, frame avg ${avg.toFixed(3)} ms p50 ${row.p50.toFixed(3)} p99 ${row.p99.toFixed(3)} (${row.showsPerFrame.toFixed(2)} show/frame)`);
}
fs.writeFileSync(path.join(cfg.buildDir, 'bench.json'), JSON.stringify({ leds: LEDS, frames: FRAMES, node: process.version, rows }, null, 2));
