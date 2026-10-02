// Frame trace format shared by native/harness.cpp and the wasm runner below.
//   "PXLT" u32 frames u32 uiLen uiJson
//   per frame: u32 'PXLF' status shows nowUsLo nowUsHi ledCount rawLen, L1[3n], L2rgb[3n], L2raw[rawLen]
import fs from 'node:fs';
import { instantiateFastLedHost } from '../runtime/fastledWasmHost.ts';

// Deterministic IDE-like frame schedule (µs): ~60 fps with jitter, dt=0 repaints, long hitches.
export function makeSchedule(frames, seed = 1337) {
  const out = new Uint32Array(frames);
  let s = seed >>> 0;
  for (let i = 0; i < frames; i++) {
    s = (Math.imul(s, 1664525) + 1013904223) >>> 0;
    let dt = 16667 + ((s >>> 16) % 9001) - 4500;
    if (i % 97 === 0) dt = 0; // incl. frame 0: first frame(0) must render
    if (i % 233 === 120) dt = 250000;
    out[i] = dt;
  }
  return out;
}

export function parseEvents(text) {
  return (text || '').split(/\r?\n/).filter(Boolean).map((l) => {
    const sp = l.indexOf(' ');
    return { frame: +l.slice(0, sp), json: l.slice(sp + 1) };
  });
}

class Writer {
  constructor() { this.chunks = []; }
  u32(v) { const b = Buffer.alloc(4); b.writeUInt32LE(v >>> 0); this.chunks.push(b); }
  bytes(b) { this.chunks.push(Buffer.from(b)); }
  buffer() { return Buffer.concat(this.chunks); }
}

export async function runWasmTrace(wasmBytes, schedule, events = [], seed = 1, { timing = false } = {}) {
  const host = await instantiateFastLedHost(wasmBytes);
  host.init(seed);
  const w = new Writer();
  w.bytes(Buffer.from('PXLT'));
  w.u32(schedule.length);
  const uiJson = Buffer.from(JSON.stringify(host.getUi().map((e) => e.raw)));
  // the native harness writes FastLED's own JSON string; compare semantically instead
  w.u32(0);
  let frameMs = 0;
  for (let f = 0; f < schedule.length; f++) {
    for (const e of events) if (e.frame === f) host.setUiMany(JSON.parse(e.json));
    const t0 = timing ? performance.now() : 0;
    const r = host.frame(schedule[f] / 1000);
    if (timing) frameMs += performance.now() - t0;
    const n = host.ledCount();
    const raw = host.getWireRaw();
    w.u32(0x464c5850); w.u32(r.status); w.u32(host.showCount());
    // exact µs clock (avoid float ms)
    const ex = host['ex'];
    w.u32(ex.pxl_now_us_lo()); w.u32(ex.pxl_now_us_hi());
    w.u32(n); w.u32(raw.length);
    w.bytes(host.getLeds()); w.bytes(host.getWire()); w.bytes(raw);
  }
  return { trace: w.buffer(), ui: host.getUi(), uiJson, frameMs, host };
}

export function parseTrace(buf) {
  let o = 0;
  const u32 = () => { const v = buf.readUInt32LE(o); o += 4; return v; };
  if (buf.toString('latin1', 0, 4) !== 'PXLT') throw new Error('bad trace');
  o = 4;
  const frames = u32();
  const uiLen = u32();
  const ui = buf.toString('utf8', o, o + uiLen); o += uiLen;
  const out = [];
  for (let f = 0; f < frames; f++) {
    if (u32() !== 0x464c5850) throw new Error(`bad frame magic at ${f}`);
    const fr = { status: u32(), shows: u32(), nowLo: u32(), nowHi: u32(), n: u32(), rawLen: u32() };
    fr.l1 = buf.subarray(o, o + fr.n * 3); o += fr.n * 3;
    fr.l2 = buf.subarray(o, o + fr.n * 3); o += fr.n * 3;
    fr.raw = buf.subarray(o, o + fr.rawLen); o += fr.rawLen;
    out.push(fr);
  }
  return { frames: out, ui };
}

const diffBytes = (a, b) => {
  let d = Math.abs(a.length - b.length);
  const n = Math.min(a.length, b.length);
  for (let i = 0; i < n; i++) if (a[i] !== b[i]) d++;
  return d;
};

export function compareTraces(refBuf, candBuf) {
  const A = parseTrace(refBuf).frames, B = parseTrace(candBuf).frames;
  const r = { frames: Math.min(A.length, B.length), frameCountMismatch: A.length !== B.length,
    l1Bytes: 0, l1Diff: 0, l2Bytes: 0, l2Diff: 0, rawBytes: 0, rawDiff: 0, metaDiff: 0, diffFrames: 0, firstDiff: -1,
    shows: 0, budgetFrames: 0, l2NeL1: 0 };
  for (let f = 0; f < r.frames; f++) {
    const a = A[f], b = B[f];
    const m = (a.status !== b.status) + (a.shows !== b.shows) + (a.nowLo !== b.nowLo) + (a.nowHi !== b.nowHi) + (a.n !== b.n);
    const d1 = diffBytes(a.l1, b.l1), d2 = diffBytes(a.l2, b.l2), d3 = diffBytes(a.raw, b.raw);
    r.metaDiff += m; r.l1Diff += d1; r.l2Diff += d2; r.rawDiff += d3;
    r.l1Bytes += a.l1.length; r.l2Bytes += a.l2.length; r.rawBytes += a.raw.length;
    r.l2NeL1 += diffBytes(a.l1, a.l2);
    if (a.status === 1) r.budgetFrames++;
    if (m + d1 + d2 + d3 && r.firstDiff < 0) r.firstDiff = f;
    if (m + d1 + d2 + d3) r.diffFrames++;
  }
  r.shows = A.length ? A[A.length - 1].shows : 0;
  const last = A[A.length - 1];
  r.virtualMs = last ? (last.nowHi * 4294967296 + last.nowLo) / 1000 : 0;
  r.leds = last ? last.n : 0;
  return r;
}

export function writeSchedule(file, schedule) {
  fs.writeFileSync(file, Buffer.from(schedule.buffer, schedule.byteOffset, schedule.byteLength));
}
