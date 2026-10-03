// Child process for catalog/build-catalog.mjs: runs one compiled sketch module for N frames
// through runtime/fastledWasmHost.ts (the same host the IDE uses) with the standard frame
// schedule of tests/trace.mjs and reports what it saw. A separate process (not a worker thread)
// so that a sketch that never yields (e.g. while(true){} without millis/delay/show) can always
// be killed by the parent (timeout).
//   node catalog/run-worker.mjs <job.json>   job: {wasmFile, frames, seed, traceFile|null}; result via IPC
import { instantiateFastLedHost, FRAME_STATUS } from '../runtime/fastledWasmHost.ts';
import { makeSchedule } from '../tests/trace.mjs';

import fs from 'node:fs';
const job = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
const { frames, seed, traceFile } = job;
const wasm = fs.readFileSync(job.wasmFile);
const wantTrace = !!traceFile;
const consoleLines = [];
let consoleCount = 0;
const res = { stage: 'instantiate' };
const post = (extra) => process.send({ ...res, ...extra }, () => process.exit(0));

class Writer {
  constructor() { this.chunks = []; this.size = 0; }
  u32(v) { const b = Buffer.alloc(4); b.writeUInt32LE(v >>> 0); this.chunks.push(b); this.size += 4; }
  bytes(b) { const c = Buffer.from(b); this.chunks.push(c); this.size += c.length; }
  buffer() { return Buffer.concat(this.chunks); }
}

try {
  const host = await instantiateFastLedHost(new Uint8Array(wasm), {
    onConsole: (text) => { consoleCount++; if (consoleLines.length < 12) consoleLines.push(text.slice(0, 200)); },
  });
  res.stage = 'setup';
  const t0 = performance.now();
  host.init(seed);
  res.setupMs = performance.now() - t0;
  res.stage = 'frames';
  const schedule = makeSchedule(frames);
  const w = wantTrace ? new Writer() : null;
  if (w) { w.bytes(Buffer.from('PXLT')); w.u32(schedule.length); w.u32(0); }
  let budgetFrames = 0, litFrames = 0, changingFrames = 0, maxFrameMs = 0, sumFrameMs = 0;
  let prev = null;
  res.framesRun = 0;
  for (let f = 0; f < schedule.length; f++) {
    const ta = performance.now();
    const r = host.frame(schedule[f] / 1000);
    const dt = performance.now() - ta;
    sumFrameMs += dt; if (dt > maxFrameMs) maxFrameMs = dt;
    if (r.status === FRAME_STATUS.BUDGET) budgetFrames++;
    const wire = host.getWire();
    const l1 = host.getLeds();
    // lit: any non-black LED in L1 (leds[]) or L2 (wire; SPI chipsets have L1 only)
    let lit = false;
    for (let i = 0; i < l1.length && !lit; i++) if (l1[i] || wire[i]) lit = true;
    if (lit) litFrames++;
    const cur = Buffer.from(l1);
    if (prev && (prev.length !== cur.length || !prev.equals(cur))) changingFrames++;
    prev = cur;
    if (w) {
      const raw = host.getWireRaw();
      const ex = host['ex'];
      w.u32(0x464c5850); w.u32(r.status); w.u32(host.showCount());
      w.u32(ex.pxl_now_us_lo()); w.u32(ex.pxl_now_us_hi());
      w.u32(host.ledCount()); w.u32(raw.length);
      w.bytes(host.getLeds()); w.bytes(wire); w.bytes(raw);
    }
    res.framesRun = f + 1;
  }
  res.stage = 'done';
  const ui = host.getUi().map((e) => {
    const o = { name: e.name, type: e.type, kind: e.kind };
    if (e.group) o.group = e.group;
    if (e.min !== undefined) o.min = e.min;
    if (e.max !== undefined) o.max = e.max;
    if (e.step !== undefined) o.step = e.step;
    if (e.options) o.options = e.options;
    if (e.defaultValue !== undefined && typeof e.defaultValue !== 'object') o.defaultValue = e.defaultValue;
    return o;
  });
  post({
    ok: true,
    ledCount: host.ledCount(),
    showCount: host.showCount(),
    loopCount: host.loopCount(),
    virtualMs: host.millis(),
    strips: host.getStrips().map((s) => ({ ledOffset: s.ledOffset, ledCount: s.ledCount, pin: s.pin, colorOrder: s.colorOrderName,
      rgbw: s.rgbw, isSpi: s.isSpi, enabled: s.enabled, wireBytes: s.wireLength })),
    ui,
    screenMaps: host.getScreenMaps(),
    budgetFrames, litFrames, changingFrames,
    frameMsAvg: sumFrameMs / schedule.length, frameMsMax: maxFrameMs,
    console: { lines: consoleCount, sample: consoleLines },
    trace: w ? (fs.writeFileSync(traceFile, w.buffer()), traceFile) : null,
  });
} catch (e) {
  post({ ok: false, error: String(e && e.message || e).slice(0, 500), console: { lines: consoleCount, sample: consoleLines } });
}
