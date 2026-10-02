// End-to-end test of service/server.mjs (spawns it on a free port) + host runtime semantics.
//   node tests/service-test.mjs
import { spawn } from 'node:child_process';
import fs from 'node:fs';
import net from 'node:net';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { instantiateFastLedHost, FRAME_STATUS } from '../runtime/fastledWasmHost.ts';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const freePort = () => new Promise((r) => { const s = net.createServer(); s.listen(0, '127.0.0.1', () => { const p = s.address().port; s.close(() => r(p)); }); });
const port = await freePort();
const base = `http://127.0.0.1:${port}`;
const srv = spawn(process.execPath, [path.join(ROOT, 'service', 'server.mjs'), '--port', String(port)], { stdio: ['ignore', 'pipe', 'pipe'] });
let srvOut = '';
srv.stdout.on('data', (d) => (srvOut += d));
srv.stderr.on('data', (d) => (srvOut += d));
const results = [];
const check = (name, cond, info = '') => { results.push({ name, ok: !!cond }); console.log(`${cond ? 'PASS' : 'FAIL'}  ${name}${info ? '  ' + info : ''}`); };
const sketch = (f) => fs.readFileSync(path.join(ROOT, 'tests', 'sketches', f), 'utf8');
const post = async (body, headers = {}) => {
  const t0 = performance.now();
  const r = await fetch(base + '/compile', { method: 'POST', headers: { 'content-type': 'application/json', ...headers }, body: JSON.stringify(body) });
  const j = await r.json();
  j.clientMs = performance.now() - t0;
  return j;
};
try {
  for (let i = 0; i < 150; i++) {
    try { const r = await fetch(base + '/health'); if (r.ok && (await r.json()).ready) break; } catch {}
    await new Promise((r) => setTimeout(r, 200));
  }
  const health = await (await fetch(base + '/health')).json();
  check('GET /health', health.ok && health.ready && health.abiVersion === 1, JSON.stringify(health));
  const version = await (await fetch(base + '/version')).json();
  check('GET /version', version.fastled?.commit && version.zig, `${version.fastled.version} ${version.fastled.commit.slice(0, 10)} zig ${version.zig}`);
  const pre = await fetch(base + '/compile', { method: 'OPTIONS', headers: { origin: 'http://localhost:5174', 'access-control-request-method': 'POST',
    'access-control-request-headers': 'content-type', 'access-control-request-private-network': 'true' } });
  check('CORS preflight + Private Network Access', pre.status === 204 && pre.headers.get('access-control-allow-origin') === 'http://localhost:5174' &&
    pre.headers.get('access-control-allow-private-network') === 'true');
  const bad = await fetch(base + '/health', { headers: { origin: 'http://evil.example' } });
  check('CORS: foreign origin not allowed', !bad.headers.get('access-control-allow-origin'));

  const err = await post({ source: sketch('CompileError.ino') }, { origin: 'http://localhost:5175' });
  const e1 = (err.diagnostics || []).filter((d) => d.severity === 'error');
  check('compile error -> diagnostics on sketch lines', !err.ok && e1.some((d) => d.line === 10 && /hue/.test(d.message)) &&
    e1.some((d) => d.line === 12 && /undefinedHelper/.test(d.message)), e1.map((d) => `${d.file}:${d.line}:${d.col} ${d.message}`).join(' | '));

  const uniq = `// ${Date.now()}\n`;
  const src = uniq + sketch('PipelineTest.ino');
  const cold = await post({ source: src, defines: { PXLBLZ_NUM_LEDS: 136 } });
  check('compile with defines (uncached)', cold.ok && !cold.cached, cold.ok ? `service ${cold.timings.serviceMs.toFixed(0)} ms (compile ${cold.timings.compileMs.toFixed(0)}, link ${cold.timings.linkMs.toFixed(0)}, asyncify ${cold.timings.asyncifyMs.toFixed(0)}), ${(cold.size / 1024).toFixed(0)} KB` : JSON.stringify(cold).slice(0, 500));
  const warm = await post({ source: src, defines: { PXLBLZ_NUM_LEDS: 136 } });
  check('compile cached by hash', warm.ok && warm.cached && warm.key === cold.key, `${warm.timings.serviceMs.toFixed(0)} ms`);
  const bin = await fetch(base + cold.wasmUrl);
  check('GET /artifact/<key>.wasm', bin.ok && bin.headers.get('content-type') === 'application/wasm');
  const raw = await fetch(base + '/compile', { method: 'POST', headers: { 'content-type': 'application/json', accept: 'application/wasm' }, body: JSON.stringify({ source: src, defines: { PXLBLZ_NUM_LEDS: 136 } }) });
  check('POST /compile with Accept: application/wasm', raw.ok && raw.headers.get('content-type') === 'application/wasm' && (await raw.arrayBuffer()).byteLength === cold.size);

  const host = await instantiateFastLedHost(Buffer.from(cold.wasm, 'base64'));
  host.init(7);
  check('defines reach the sketch (PXLBLZ_NUM_LEDS=136 -> 136+24+16 LEDs)', host.ledCount() === 176, String(host.ledCount()));
  const st0 = host.frame(0);
  check('first frame(0) renders the first frame', st0.status === FRAME_STATUS.OK && host.showCount() > 0, JSON.stringify(st0));
  const shows = host.showCount();
  const st1 = host.frame(0);
  check('later frame(0) does not run loop()', st1.status === FRAME_STATUS.SKIPPED && host.showCount() === shows);
  const ui = host.getUi();
  check('UI elements', ui.length === 4 && ui[0].name === 'Speed' && ui[0].min === 1 && ui[0].max === 200 && ui[0].defaultValue === 40,
    ui.map((e) => `${e.name}:${e.kind}[${e.min ?? ''}..${e.max ?? ''}] def=${e.defaultValue}`).join(', '));
  host.setUi('Brightness', 300);
  check('setUi by name clamps like FastLED', host.getUi().find((e) => e.name === 'Brightness').value === 255);
  host.frame(16.7);
  check('UI value reaches the sketch (FastLED.getBrightness() == 255)', host.brightness() === 255);
  const strips = host.getStrips();
  check('strips + color orders', strips.map((s) => s.colorOrderName).join(',') === 'GRB,BGR,RGB', JSON.stringify(strips.map((s) => [s.pin, s.ledCount, s.colorOrderName])));
  const wire = host.getWire(), wraw = host.getWireRaw();
  check('getWire() is RGB order of getWireRaw() (GRB strip)', wire[0] === wraw[1] && wire[1] === wraw[0] && wire[2] === wraw[2]);

  const run = await post({ source: sketch('Runaway.ino') });
  const h2 = await instantiateFastLedHost(Buffer.from(run.wasm, 'base64'));
  h2.init(1);
  let t0 = performance.now(), frames = 0;
  while (h2.millis() < 2000 && frames < 1000) { h2.frame(16.667); frames++; }
  check('busy-wait on millis() does not freeze and follows virtual time', h2.showCount() >= 3 && performance.now() - t0 < 5000,
    `${frames} frames, ${h2.showCount()} shows, virtual ${h2.millis().toFixed(0)} ms, wall ${(performance.now() - t0).toFixed(0)} ms`);
  h2.setUi('Runaway', true);
  t0 = performance.now();
  let st;
  for (let i = 0; i < 60; i++) { st = h2.frame(16.667); if (st.status === FRAME_STATUS.BUDGET) break; }
  check('runaway loop() -> FRAME_STATUS.BUDGET (tab stays responsive)', st.status === FRAME_STATUS.BUDGET, `wall ${(performance.now() - t0).toFixed(0)} ms`);
  const st2 = h2.frame(16.667);
  check('runaway sketch stays resumable', st2.status === FRAME_STATUS.BUDGET);
} finally {
  srv.kill();
}
const failed = results.filter((r) => !r.ok).length;
console.log(failed ? `\n${failed} FAILED` : '\nALL PASSED');
if (failed) console.log(srvOut.slice(-3000));
process.exit(failed ? 1 : 0);
