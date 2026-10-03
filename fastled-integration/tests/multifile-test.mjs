// Multi-file sketches through the compile service (POST /compile with `files`) and natively.
//   node tests/multifile-test.mjs
// Covers the sketch-folder model of toolchain/toolchain.mjs (README "Mehrdatei-Sketches"):
// root .cpp TU, subfolder TUs (src/, shared/), relative includes from subfolders, a secondary
// .ino tab (Arduino concatenation + auto prototypes), a .cpp with #define before FastLED.h
// (no PCH for that TU), diagnostics on subfolder files, rejected path names, and
// wasm == native parity over 300 frames.
import { spawn } from 'node:child_process';
import fs from 'node:fs';
import net from 'node:net';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { instantiateFastLedHost } from '../runtime/fastledWasmHost.ts';
import { loadConfig, compileSketch, run } from '../toolchain/toolchain.mjs';
import { makeSchedule, writeSchedule, runWasmTrace, compareTraces } from './trace.mjs';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const cfg = loadConfig();
const results = [];
const check = (name, cond, info = '') => { results.push({ name, ok: !!cond }); console.log(`${cond ? 'PASS' : 'FAIL'}  ${name}${info ? '  ' + info : ''}`); };

const MAIN = `// Multi-file test sketch
#include <FastLED.h>
#include "helper.h"
#include "src/wave.h"
#include "shared/config.h"

CRGB leds[NUM_LEDS];

void setup() {
  FastLED.addLeds<WS2812B, 2, GRB>(leds, NUM_LEDS);
  FastLED.setBrightness(BRIGHT);
}

void loop() {
  uint8_t t = (uint8_t)(millis() / 10);
  for (int i = 0; i < NUM_LEDS; ++i) leds[i] = blendHelper(i, waveAt(i, t), tabHue(i));
  FastLED.show();
  delay(20);
}
`;
const FILES = {
  'helper.h': '#pragma once\n#include <FastLED.h>\nCRGB blendHelper(int i, uint8_t v, uint8_t hue);\nint helperMagic();\n',
  'helper.cpp': '#include "helper.h"\nCRGB blendHelper(int i, uint8_t v, uint8_t hue) {\n  CHSV c(hue + i, 255, v);\n  CRGB out; hsv2rgb_rainbow(c, out);\n  return out;\n}\nint helperMagic() { return 4242; }\n',
  'src/wave.h': '#pragma once\n#include <stdint.h>\nuint8_t waveAt(int i, uint8_t t);\n',
  'src/wave.cpp': '#include "wave.h"\n#include "../helper.h"\n#include <FastLED.h>\nuint8_t waveAt(int i, uint8_t t) { return sin8((uint8_t)(i * 16 + t)) / 2 + 64 + (helperMagic() == 4242 ? 0 : 1); }\n',
  'shared/config.h': '#pragma once\n#include "limits.h"\n#define BRIGHT 200\n',
  'shared/limits.h': '#pragma once\n#define NUM_LEDS 24\n',
  // #define before FastLED.h in a secondary TU -> this TU is compiled without the PCH
  'shared/defs_first.cpp': '#define PXL_TEST_LOCAL_DEFINE 1\n#include <FastLED.h>\nint definedFirst() { return PXL_TEST_LOCAL_DEFINE; }\n',
  // secondary Arduino tab: appended to the main sketch, uses a type from the main file
  'Tab2.ino': 'uint8_t tabHue(int i) {\n  return (uint8_t)(i * 7 + helperMagic() % 3);\n}\n',
};

// Expected frame computed independently (same formulas, FastLED-free reimplementation is
// overkill): instead compare against a single-file version of the same sketch.
const SINGLE = MAIN.replace('#include "helper.h"\n#include "src/wave.h"\n#include "shared/config.h"\n',
  '#define NUM_LEDS 24\n#define BRIGHT 200\nint helperMagic() { return 4242; }\n' +
  'CRGB blendHelper(int i, uint8_t v, uint8_t hue) { CHSV c(hue + i, 255, v); CRGB out; hsv2rgb_rainbow(c, out); return out; }\n' +
  'uint8_t waveAt(int i, uint8_t t) { return sin8((uint8_t)(i * 16 + t)) / 2 + 64 + (helperMagic() == 4242 ? 0 : 1); }\n' +
  'uint8_t tabHue(int i) { return (uint8_t)(i * 7 + helperMagic() % 3); }\n');

const freePort = () => new Promise((r) => { const s = net.createServer(); s.listen(0, '127.0.0.1', () => { const p = s.address().port; s.close(() => r(p)); }); });
const port = await freePort();
const base = `http://127.0.0.1:${port}`;
const srv = spawn(process.execPath, [path.join(ROOT, 'service', 'server.mjs'), '--port', String(port)], { stdio: ['ignore', 'pipe', 'pipe'] });
let srvOut = '';
srv.stdout.on('data', (d) => (srvOut += d));
srv.stderr.on('data', (d) => (srvOut += d));
const post = async (body) => {
  const r = await fetch(base + '/compile', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify(body) });
  return { status: r.status, ...(await r.json()) };
};
try {
  for (let i = 0; i < 300; i++) {
    try { const r = await fetch(base + '/health'); if (r.ok && (await r.json()).ready) break; } catch {}
    await new Promise((r) => setTimeout(r, 200));
  }
  const uniq = `// ${Date.now()}\n`;
  const multi = await post({ source: uniq + MAIN, files: FILES, options: { fileName: 'MultiFile.ino' } });
  check('multi-file sketch compiles (root .cpp, src/*.cpp, shared/*.cpp, Tab2.ino)', multi.ok,
    multi.ok ? `${(multi.size / 1024).toFixed(0)} KB, prototypes: ${multi.prototypes.join(' ')}` : JSON.stringify(multi.diagnostics || multi.error).slice(0, 800));
  const single = await post({ source: uniq + SINGLE, options: { fileName: 'Single.ino' } });
  check('single-file equivalent compiles', single.ok, single.ok ? '' : JSON.stringify(single.diagnostics).slice(0, 500));
  if (multi.ok && single.ok) {
    const sched = makeSchedule(300);
    const a = await runWasmTrace(Buffer.from(multi.wasm, 'base64'), sched, [], 1);
    const b = await runWasmTrace(Buffer.from(single.wasm, 'base64'), sched, [], 1);
    const cmp = compareTraces(b.trace, a.trace);
    check('multi-file output == single-file output (300 frames, L1/L2/raw)', cmp.l1Diff + cmp.l2Diff + cmp.rawDiff + cmp.metaDiff === 0 && cmp.leds === 24,
      `leds ${cmp.leds}, L1 diff ${cmp.l1Diff}, L2 diff ${cmp.l2Diff}`);
    const h = await instantiateFastLedHost(Buffer.from(multi.wasm, 'base64'));
    h.init(1); h.frame(0);
    check('helper TUs really ran (non-black, 24 LEDs)', h.ledCount() === 24 && h.getLeds().some((v) => v > 0));

    // native reference of the same multi-file sketch
    const n = await compileSketch(cfg, { source: uniq + MAIN, files: FILES, target: 'native', options: { fileName: 'MultiFile.ino' } });
    check('multi-file sketch compiles natively', n.ok, n.ok ? '' : JSON.stringify(n.diagnostics).slice(0, 500));
    if (n.ok) {
      const work = path.join(cfg.buildDir, 'multifile-test');
      fs.mkdirSync(work, { recursive: true });
      writeSchedule(path.join(work, 'schedule.bin'), sched);
      const r = await run(n.path, [path.join(work, 'schedule.bin'), path.join(work, 'native.trace'), '-', '1', 'eager']);
      const c2 = r.code === 0 ? compareTraces(fs.readFileSync(path.join(work, 'native.trace')), a.trace) : null;
      check('multi-file wasm == native reference (300 frames)', c2 && c2.l1Diff + c2.l2Diff + c2.rawDiff + c2.metaDiff === 0,
        c2 ? `L1 ${c2.l1Diff} L2 ${c2.l2Diff} raw ${c2.rawDiff} meta ${c2.metaDiff}` : r.err.slice(0, 200));
    }
  }

  const bad = await post({ source: uniq + MAIN, files: { ...FILES, 'src/wave.cpp': FILES['src/wave.cpp'] + '\nint broken() { return undefinedThing; }\n' } });
  const d = (bad.diagnostics || []).find((x) => x.severity === 'error');
  check('error in subfolder TU -> diagnostic with file "src/wave.cpp" and its line', !bad.ok && d && d.file === 'src/wave.cpp' && d.line === 6 && d.inSketch,
    d ? `${d.file}:${d.line}: ${d.message}` : JSON.stringify(bad).slice(0, 300));
  const badTab = await post({ source: uniq + MAIN, files: { ...FILES, 'Tab2.ino': FILES['Tab2.ino'] + 'int x = nope;\n' } });
  const d2 = (badTab.diagnostics || []).find((x) => x.severity === 'error');
  check('error in secondary .ino tab -> diagnostic on "Tab2.ino" line 4', !badTab.ok && d2 && d2.file === 'Tab2.ino' && d2.line === 4,
    d2 ? `${d2.file}:${d2.line}: ${d2.message}` : JSON.stringify(badTab).slice(0, 300));
  for (const evil of ['../evil.h', '/abs.h', 'a//b.h', '.hidden/x.h', 'C:\\x.h', 'x.exe']) {
    const r = await post({ source: MAIN, files: { [evil]: '' } });
    check(`rejects file name ${JSON.stringify(evil)}`, r.status === 400 && !r.ok && /invalid sketch file name/.test(r.error || ''), r.error);
  }
} finally {
  srv.kill();
}
const failed = results.filter((r) => !r.ok).length;
console.log(failed ? `\n${failed} FAILED` : '\nALL PASSED');
if (failed) console.log(srvOut.slice(-2000));
process.exit(failed ? 1 : 0);
