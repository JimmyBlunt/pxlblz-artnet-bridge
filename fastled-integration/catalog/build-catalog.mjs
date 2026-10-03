// Builds the PXLBLZ-IDE FastLED example catalog from the pinned FastLED checkout.
//
//   node catalog/build-catalog.mjs [--frames 600] [--only Blink,Fx/FxCylon] [--port 9997]
//                                  [--service http://127.0.0.1:9997] [--no-parity] [--jobs 3]
//                                  [--bundle-only]
//
// 1. walks every sketch folder in <fastled>/examples (a folder with <Name>/<Name>.ino),
// 2. copies its text files verbatim to catalog/examples/<id>/ (binaries / files > 256 KB are
//    listed as omittedFiles), records sha256/bytes,
// 3. compiles each sketch through the compile service (POST /compile, the IDE's code path;
//    a private service instance is started on --port unless --service is given),
// 4. runs it for --frames frames in a worker via runtime/fastledWasmHost.ts with the standard
//    frame schedule (tests/trace.mjs) -> run status, LEDs, strips, UI, screen maps,
// 5. builds the same sketch natively (x86_64 reference, native/harness.cpp) and compares
//    L1/L2/raw bytes per frame (parity),
// 6. writes catalog/fastled-examples.json, catalog/screenmaps/<id>.json, catalog/CATALOG_RESULTS.md.
// --bundle-only does 1+2 and keeps the previous results of examples whose files did not change.
import { spawn, fork, execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { loadConfig, buildLib, compileSketch, fastledRevision, zigVersion, run, ROOT } from '../toolchain/toolchain.mjs';
import { makeSchedule, writeSchedule, compareTraces } from '../tests/trace.mjs';
import { CATEGORIES, EXAMPLE_META, categorize, SCHEMA_VERSION } from './catalog-meta.mjs';

const args = process.argv.slice(2);
const opt = (k, d) => { const i = args.indexOf(k); return i >= 0 ? args[i + 1] : d; };
const FRAMES = +opt('--frames', 600);
const ONLY = opt('--only', '');
const PORT = +opt('--port', 9997);
const JOBS = +opt('--jobs', 1);  // memory: big sketches need ~1-2 GB per zig/clang process
const PARITY = !args.includes('--no-parity');
const BUNDLE_ONLY = args.includes('--bundle-only');
const SEED = 1;
const RUN_TIMEOUT_MS = 90000;
const NATIVE_TIMEOUT_MS = 120000;
const MAX_FILE_BYTES = 256 * 1024;

const cfg = loadConfig();
const CAT = path.join(ROOT, 'catalog');
const EXAMPLES = path.join(cfg.fastledDir, 'examples');
const OUT_JSON = path.join(CAT, 'fastled-examples.json');
const BUNDLE = path.join(CAT, 'examples');
const SMDIR = path.join(CAT, 'screenmaps');
const WORK = path.join(cfg.buildDir, 'catalog');
const log = (s) => process.stdout.write(s + '\n');
const sha256 = (b) => createHash('sha256').update(b).digest('hex');
const rev = fastledRevision(cfg);
if (rev.commit === 'unknown') throw new Error(`FastLED checkout not found at ${cfg.fastledDir}`);

// ------------------------------------------------------------------ discovery
const TEXT_ROLE = [
  [/\.(ino|pde)$/i, 'sketch'], [/\.(cpp|cc|cxx|c)$/i, 'source'], [/\.(h|hh|hpp|hxx|inc|ipp|tpp)$/i, 'header'],
  [/\.(json|txt|csv)$/i, 'data'], [/\.md$/i, 'doc'], [/\.(py|mjs|js|sh|ps1)$/i, 'tool'],
];
const COMPILE_RE = /\.(ino|pde|cpp|cc|cxx|c|h|hh|hpp|hxx|inc|ipp|tpp|json|txt|csv)$/i;

function walk(dir, base = dir, out = []) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name))) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) walk(p, base, out); else out.push(path.relative(base, p).replace(/\\/g, '/'));
  }
  return out;
}

function discover() {
  const sketches = [];
  const visit = (dir) => {
    const name = path.basename(dir);
    const main = path.join(dir, name + '.ino');
    if (fs.existsSync(main) && dir !== EXAMPLES) { sketches.push(dir); return; }  // a sketch folder owns its subfolders
    for (const e of fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name))) {
      if (e.isDirectory()) visit(path.join(dir, e.name));
    }
  };
  visit(EXAMPLES);
  const byName = new Map();
  for (const d of sketches) byName.set(path.basename(d), (byName.get(path.basename(d)) || 0) + 1);
  return sketches.map((dir) => {
    const upstreamPath = path.relative(EXAMPLES, dir).replace(/\\/g, '/');
    const name = path.basename(dir);
    const id = byName.get(name) > 1 ? upstreamPath.replace(/\//g, '-') : name;
    return { id, name, dir, upstreamPath };
  });
}

function isBinary(buf) {
  const n = Math.min(buf.length, 8000);
  for (let i = 0; i < n; i++) if (buf[i] === 0) return true;
  return false;
}

function bundle(ex) {
  const files = [], omitted = [];
  const target = path.join(BUNDLE, ex.id);
  fs.rmSync(target, { recursive: true, force: true });
  for (const rel of walk(ex.dir)) {
    const buf = fs.readFileSync(path.join(ex.dir, rel));
    const roleEntry = TEXT_ROLE.find(([re]) => re.test(rel));
    const reason = buf.length > MAX_FILE_BYTES ? `> ${MAX_FILE_BYTES / 1024} KB` : isBinary(buf) ? 'binary' : !roleEntry ? 'not a source/data file' : null;
    if (reason) { omitted.push({ path: rel, bytes: buf.length, sha256: sha256(buf), reason }); continue; }
    const role = rel === ex.name + '.ino' ? 'main' : roleEntry[1];
    const dst = path.join(target, ...rel.split('/'));
    fs.mkdirSync(path.dirname(dst), { recursive: true });
    fs.writeFileSync(dst, buf);  // verbatim (no newline conversion)
    files.push({ path: rel, role, bytes: buf.length, sha256: sha256(buf), compile: role !== 'doc' && role !== 'tool' && COMPILE_RE.test(rel) });
  }
  return { files, omitted };
}

// ------------------------------------------------------------------ service
async function startService() {
  const ext = opt('--service', '');
  if (ext) return { base: ext.replace(/\/$/, ''), stop: () => {} };
  const svc = { base: `http://127.0.0.1:${PORT}`, proc: null };
  svc.stop = () => svc.proc?.kill();
  svc.restart = async () => { svc.stop(); svc.proc = (await spawnService()).proc; };
  svc.proc = (await spawnService()).proc;
  return svc;
}

async function spawnService() {
  const srv = spawn(process.execPath, [path.join(ROOT, 'service', 'server.mjs'), '--port', String(PORT)], { stdio: ['ignore', 'pipe', 'pipe'] });
  let out = '';
  srv.stdout.on('data', (d) => (out += d));
  srv.stderr.on('data', (d) => (out += d));
  const base = `http://127.0.0.1:${PORT}`;
  for (let i = 0; i < 1500; i++) {
    if (srv.exitCode !== null) throw new Error('compile service exited:\n' + out);
    try { const r = await fetch(base + '/health'); if (r.ok && (await r.json()).ready) { srv.on('exit', (c) => { if (c) log('  compile service exited (' + c + '): ' + out.slice(-500)); }); return { base, proc: srv }; } } catch {}
    await new Promise((r) => setTimeout(r, 200));
  }
  srv.kill();
  throw new Error('compile service did not become ready:\n' + out);
}

async function compileViaService(svc, source, files, fileName) {
  const body = JSON.stringify({ source, files, options: { fileName } });
  for (let attempt = 0; ; attempt++) {
    try {
      const r = await fetch(svc.base + '/compile', { method: 'POST', headers: { 'content-type': 'application/json' }, body });
      return await r.json();
    } catch (e) {
      // the service process died (e.g. out of memory): restart our private instance once
      if (attempt > 0 || !svc.restart) throw e;
      log(`  compile service unreachable (${e.message}); restarting`);
      await svc.restart();
    }
  }
}

// ------------------------------------------------------------------ run in worker
function runWorker(wasm, traceFile, id) {
  const wasmFile = path.join(WORK, id + '.wasm');
  const jobFile = path.join(WORK, id + '.job.json');
  fs.writeFileSync(wasmFile, wasm);
  fs.writeFileSync(jobFile, JSON.stringify({ wasmFile, frames: FRAMES, seed: SEED, traceFile }));
  return new Promise((resolve) => {
    const child = fork(fileURLToPath(new URL('./run-worker.mjs', import.meta.url)), [jobFile],
      { execArgv: ['--max-old-space-size=2048'], stdio: ['ignore', 'ignore', 'pipe', 'ipc'] });
    let err = '', done = false;
    child.stderr.on('data', (d) => { if (err.length < 4000) err += d; });
    const finish = (r) => {
      if (done) return; done = true; clearTimeout(t);
      if (child.exitCode === null) child.kill('SIGKILL');
      fs.rmSync(wasmFile, { force: true }); fs.rmSync(jobFile, { force: true });
      resolve(r);
    };
    const t = setTimeout(() => finish({ ok: false, timeout: true, error: `no result after ${RUN_TIMEOUT_MS / 1000} s` }), RUN_TIMEOUT_MS);
    child.on('message', finish);
    child.on('error', (e) => finish({ ok: false, error: String(e.message || e) }));
    child.on('exit', (code) => setTimeout(() => finish({ ok: false, error: `runner exited (${code}) ${err.slice(-300)}` }), 50));
  });
}

// ------------------------------------------------------------------ geometry
function summarizeScreenMaps(maps) {
  return maps.map((m) => {
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    for (let i = 0; i < m.x.length; i++) {
      minX = Math.min(minX, m.x[i]); maxX = Math.max(maxX, m.x[i]); minY = Math.min(minY, m.y[i]); maxY = Math.max(maxY, m.y[i]);
    }
    const o = { strip: m.strip, ledOffset: m.ledOffset, length: m.length, diameter: m.diameter };
    if (m.xyWidth) Object.assign(o, { xyWidth: m.xyWidth, xyHeight: m.xyHeight, xyType: ['serpentine', 'lineByLine', 'function', 'lookupTable'][m.xyType] ?? m.xyType });
    if (m.x.length) o.bounds = { minX, minY, maxX, maxY };
    o.uniqueX = new Set(m.x.map((v) => v.toFixed(3))).size;
    o.uniqueY = new Set(m.y.map((v) => v.toFixed(3))).size;
    return o;
  });
}

// Width/height constants in the source. Sketches often have several variants (#if branches),
// so all candidates are collected and the pair whose product equals the LED count wins.
function sourceDims(texts, ledCount) {
  const all = texts.join('\n');
  const grab = (res) => res.flatMap((re) => [...all.matchAll(re)].map((m) => +m[1]));
  const ws = grab([/^\s*#define\s+(?:MATRIX_|GRID_|k)?(?:WIDTH|Width|COLS|NUM_COLS)\s+\(?(\d+)/gm,
    /\b(?:const|constexpr)\s+(?:static\s+)?[\w:]+\s+(?:k?Matrix|MATRIX_|k|GRID_)?(?:Width|WIDTH|kWidth)\s*=\s*(\d+)\s*;/g]);
  const hs = grab([/^\s*#define\s+(?:MATRIX_|GRID_|k)?(?:HEIGHT|Height|ROWS|NUM_ROWS)\s+\(?(\d+)/gm,
    /\b(?:const|constexpr)\s+(?:static\s+)?[\w:]+\s+(?:k?Matrix|MATRIX_|k|GRID_)?(?:Height|HEIGHT|kHeight)\s*=\s*(\d+)\s*;/g]);
  if (!ws.length || !hs.length) return null;
  for (let i = 0; i < Math.min(ws.length, hs.length); i++) if (ledCount && ws[i] * hs[i] === ledCount) return { width: ws[i], height: hs[i] };
  for (const w of ws) for (const h of hs) if (ledCount && w * h === ledCount) return { width: w, height: h };
  return { width: ws[0], height: hs[0] };
}

function geometry(runRes, texts) {
  const maps = runRes?.screenMaps || [];
  if (maps.length) {
    const s = summarizeScreenMaps(maps);
    const xy = s.find((m) => m.xyWidth && m.xyHeight > 1);
    if (xy && s.length === 1) return { dim: 2, width: xy.xyWidth, height: xy.xyHeight, source: 'xymap' };
    const flat = s.every((m) => m.uniqueY <= 1 || m.uniqueX <= 1);
    if (flat && s.length === 1) return { dim: 1, source: 'screenmap', shape: 'line' };
    if (maps.length === 1 && maps[0].x.length >= 8) {  // ring: all points at the same distance from the centroid
      const m = maps[0], n = m.x.length;
      const cx = m.x.reduce((a, v) => a + v, 0) / n, cy = m.y.reduce((a, v) => a + v, 0) / n;
      const d = m.x.map((x, i) => Math.hypot(x - cx, m.y[i] - cy));
      const mean = d.reduce((a, v) => a + v, 0) / n;
      if (mean > 0 && (Math.max(...d) - Math.min(...d)) / mean < 0.05) return { dim: 2, source: 'screenmap', shape: 'ring', bounds: s[0].bounds, ledCount: n };
    }
    // several maps or an irregular layout: report the union bounds
    const b = s.reduce((a, m) => m.bounds ? { minX: Math.min(a.minX, m.bounds.minX), minY: Math.min(a.minY, m.bounds.minY),
      maxX: Math.max(a.maxX, m.bounds.maxX), maxY: Math.max(a.maxY, m.bounds.maxY) } : a, { minX: Infinity, minY: Infinity, maxX: -Infinity, maxY: -Infinity });
    const ux = s.reduce((a, m) => a + m.uniqueX, 0), uy = s.reduce((a, m) => a + m.uniqueY, 0);
    const grid = s.length === 1 && s[0].uniqueX * s[0].uniqueY === s[0].length;
    return grid ? { dim: 2, width: s[0].uniqueX, height: s[0].uniqueY, source: 'screenmap', shape: 'grid' }
      : { dim: 2, source: 'screenmap', shape: 'free', bounds: b, maps: s.length, approxWidth: Math.round(b.maxX - b.minX + 1), approxHeight: Math.round(b.maxY - b.minY + 1), uniqueX: ux, uniqueY: uy };
  }
  const sd = sourceDims(texts, runRes?.ledCount);
  if (sd && runRes?.ledCount && sd.width * sd.height <= runRes.ledCount * 4) return { dim: 2, ...sd, source: 'source' };
  if (sd && !runRes?.ledCount) return { dim: 2, ...sd, source: 'source' };
  return runRes?.ledCount ? { dim: 1, width: runRes.ledCount, height: 1, source: 'strip' } : { dim: null, source: 'none' };
}

// ------------------------------------------------------------------ classification
function firstErrors(diags, n = 3) {
  return (diags || []).filter((d) => d.severity === 'error').slice(0, n).map((d) => ({ file: d.file, line: d.line, message: d.message.slice(0, 300) }));
}

async function classify(ex, rec, svc, schedFile) {
  const mainText = fs.readFileSync(path.join(ex.dir, ex.name + '.ino'), 'utf8');
  const files = {};
  for (const f of rec.files) if (f.compile && f.role !== 'main') files[f.path] = fs.readFileSync(path.join(ex.dir, ...f.path.split('/')), 'utf8');
  const out = { compile: null, run: null, parity: null };
  const c = await compileViaService(svc, mainText, files, ex.name + '.ino');
  if (!c.ok) {
    let errors = c.error ? [{ message: c.error }] : firstErrors(c.diagnostics);
    if (!errors.length) errors = [{ message: (c.log || 'unknown error').split(/\r?\n/).filter(Boolean).slice(-4).join(' | ').slice(0, 400) }];
    out.compile = { status: 'error', errors };
    return out;
  }
  out.compile = { status: 'ok', wasmBytes: c.size, key: c.key, warnings: (c.diagnostics || []).filter((d) => d.severity === 'warning' && d.inSketch).length };
  const wasm = Buffer.from(c.wasm, 'base64');
  const wasmTrace = PARITY ? path.join(WORK, ex.id + '.wasm.trace') : null;
  const r = await runWorker(wasm, wasmTrace, ex.id);
  out.runRaw = r;
  if (!r.ok) {
    out.run = { status: r.timeout ? 'timeout' : 'crash', error: r.error, stage: r.stage, framesRun: r.framesRun ?? 0, console: r.console };
    return out;
  }
  const status = !r.ledCount || !r.showCount ? 'no-leds' : r.budgetFrames > 0 ? 'budget' : 'ok';
  out.run = {
    status, frames: FRAMES, ledCount: r.ledCount, showCount: r.showCount, budgetFrames: r.budgetFrames,
    litFrames: r.litFrames, changingFrames: r.changingFrames, virtualMs: Math.round(r.virtualMs),
    frameMsAvg: +r.frameMsAvg.toFixed(3), frameMsMax: +r.frameMsMax.toFixed(1),
    console: r.console,
  };
  out.strips = r.strips;
  out.ui = r.ui;
  out.screenMaps = r.screenMaps;
  if (PARITY && r.trace) {
    const n = await compileSketch(cfg, { source: mainText, files, target: 'native', options: { fileName: ex.name + '.ino' } });
    if (!n.ok) {
      out.parity = { status: 'native-compile-error', errors: firstErrors(n.diagnostics) };
    } else {
      const nativeTrace = path.join(WORK, ex.id + '.native.trace');
      const ac = new AbortController();
      const t = setTimeout(() => ac.abort(), NATIVE_TIMEOUT_MS);
      const nr = await run(n.path, [schedFile, nativeTrace, '-', String(SEED), 'eager'], { signal: ac.signal });
      clearTimeout(t);
      if (nr.code !== 0 || !fs.existsSync(nativeTrace)) {
        out.parity = { status: ac.signal.aborted ? 'native-timeout' : 'native-crash', error: `exit ${nr.code} ${nr.err.slice(0, 200)}` };
      } else {
        try {
          const cmp = compareTraces(fs.readFileSync(nativeTrace), fs.readFileSync(r.trace));
          out.parity = {
            status: cmp.l1Diff + cmp.l2Diff + cmp.rawDiff + cmp.metaDiff === 0 && !cmp.frameCountMismatch ? 'identical' : 'diff',
            frames: cmp.frames, l1DiffBytes: cmp.l1Diff, l2DiffBytes: cmp.l2Diff, rawDiffBytes: cmp.rawDiff, metaDiffs: cmp.metaDiff,
            l1Bytes: cmp.l1Bytes, firstDiffFrame: cmp.firstDiff, diffFrames: cmp.diffFrames,
          };
        } catch (e) {
          out.parity = { status: 'trace-error', error: String(e.message || e) };
        }
        fs.rmSync(nativeTrace, { force: true });
        fs.rmSync(r.trace, { force: true });
      }
    }
  }
  return out;
}

// ------------------------------------------------------------------ main
const sketches = discover().filter((e) => !ONLY || ONLY.split(',').some((o) => o === e.id || o === e.upstreamPath));
log(`FastLED ${rev.version} @ ${rev.commit.slice(0, 10)}: ${sketches.length} sketch folders in ${EXAMPLES}`);
fs.mkdirSync(BUNDLE, { recursive: true });
// provenance: FastLED's MIT license file at the pinned commit
try { fs.writeFileSync(path.join(CAT, 'LICENSE.FastLED'), execFileSync('git', ['-C', cfg.fastledDir, 'show', rev.commit + ':LICENSE'])); }
catch (e) { log('warning: could not read LICENSE from the FastLED checkout: ' + e.message); }
fs.mkdirSync(SMDIR, { recursive: true });
fs.mkdirSync(WORK, { recursive: true });

let previous = {};
try { for (const e of JSON.parse(fs.readFileSync(OUT_JSON, 'utf8')).examples) previous[e.id] = e; } catch {}

const records = sketches.map((ex) => {
  const { files, omitted } = bundle(ex);
  const texts = files.filter((f) => f.role !== 'doc' && f.role !== 'tool').map((f) => fs.readFileSync(path.join(ex.dir, ...f.path.split('/')), 'utf8'));
  const main = texts[files.findIndex((f) => f.role === 'main')] || '';
  const brief = /@brief\s+(.*)/.exec(main)?.[1]?.trim() || null;
  const filter = /@filter:?\s*(.*)/.exec(main)?.[1]?.replace(/\s*\/\/.*$/, '').trim() || null;
  return { ex, texts, rec: {
    id: ex.id, title: ex.name, category: categorize(ex), upstreamPath: ex.upstreamPath, mainFile: ex.name + '.ino',
    description: brief, upstreamFilter: filter,
    files, omittedFiles: omitted,
    fastled: { version: rev.version, commit: rev.commit },
    license: 'MIT (FastLED)',
    upstreamUrl: `https://github.com/FastLED/FastLED/tree/${rev.commit}/examples/${ex.upstreamPath}`,
  } };
});

const filesHash = (rec) => sha256(JSON.stringify(rec.files.map((f) => [f.path, f.sha256])));
let svc = null;
const schedFile = path.join(WORK, `schedule-${FRAMES}.bin`);
writeSchedule(schedFile, makeSchedule(FRAMES));
if (!BUNDLE_ONLY) {
  await buildLib(cfg, 'wasm', { log });
  if (PARITY) await buildLib(cfg, 'native', { log });
  svc = await startService();
  log(`compile service: ${svc.base}`);
}

const t0 = performance.now();
let idx = 0, doneCount = 0;
const results = new Map();
try {
  await Promise.all(Array.from({ length: BUNDLE_ONLY ? 0 : JOBS }, async () => {
    while (idx < records.length) {
      const { ex, rec } = records[idx++];
      const ta = performance.now();
      let r;
      try { r = await classify(ex, rec, svc, schedFile); } catch (e) { r = { compile: { status: 'error', errors: [{ message: 'builder: ' + String(e.message || e) }] } }; }
      results.set(ex.id, r);
      doneCount++;
      log(`[${String(doneCount).padStart(3)}/${records.length}] ${ex.upstreamPath.padEnd(48)} compile ${r.compile?.status ?? '-'}` +
        (r.run ? `  run ${r.run.status}  leds ${r.run.ledCount ?? '-'}  lit ${r.run.litFrames ?? '-'}` : '') +
        (r.parity ? `  parity ${r.parity.status}${r.parity.status === 'diff' ? ` (L1 ${r.parity.l1DiffBytes}, L2 ${r.parity.l2DiffBytes})` : ''}` : '') +
        `  ${((performance.now() - ta) / 1000).toFixed(1)} s` +
        (r.compile?.status === 'error' ? `\n      ${r.compile.errors.map((e) => `${e.file ?? ''}:${e.line ?? ''} ${e.message}`).join('\n      ')}` : '') +
        (r.run?.error ? `\n      ${r.run.error}` : ''));
    }
  }));
} finally {
  svc?.stop();
}

// ------------------------------------------------------------------ assemble
const examples = records.map(({ ex, texts, rec }) => {
  let r = results.get(ex.id);
  const prev = previous[ex.id];
  if (!r && prev && prev.filesHash === filesHash(rec) && prev.fastled?.commit === rev.commit) {
    // --bundle-only: keep previous classification
    const { id, title, upstreamPath, mainFile, files, omittedFiles, fastled, license, upstreamUrl, description, upstreamFilter, filesHash: fh, category, ...rest } = prev;
    const out = { ...rec, filesHash: fh, ...rest, category: rec.category };
    const meta = EXAMPLE_META[ex.id] || {};  // re-apply hand-maintained overrides
    out.platform = meta.platform || out.platformAuto;
    if (meta.note) out.note = meta.note;
    if (meta.geometry) out.geometry = { ...out.geometry, ...meta.geometry, source: meta.geometry.source || 'manual' };
    if (meta.reason) out.reason = meta.reason; else delete out.reason;
    return out;
  }
  const meta = EXAMPLE_META[ex.id] || {};
  const geo = meta.geometry ? { ...geometry(r?.runRaw, texts), ...meta.geometry, source: meta.geometry.source || 'manual' } : geometry(r?.runRaw, texts);
  if (r?.screenMaps?.length) fs.writeFileSync(path.join(SMDIR, ex.id + '.json'), JSON.stringify(r.screenMaps));
  else fs.rmSync(path.join(SMDIR, ex.id + '.json'), { force: true });
  const runOk = r?.run && (r.run.status === 'ok' || r.run.status === 'budget');
  const autoPlatform = runOk && r.run.litFrames > 0 ? 'pc' : 'hardware';
  const platform = meta.platform || autoPlatform;
  const out = {
    ...rec,
    filesHash: filesHash(rec),
    compile: r?.compile ?? { status: 'not-run' },
    run: r?.run ?? (r?.compile?.status === 'ok' ? { status: 'not-run' } : null),
    ledCount: r?.run?.ledCount ?? null,
    strips: r?.strips ?? [],
    uiElements: r?.ui ?? [],
    screenMap: r?.screenMaps?.length ? { file: `screenmaps/${ex.id}.json`, maps: summarizeScreenMaps(r.screenMaps) } : null,
    geometry: geo,
    platform,
    platformAuto: autoPlatform,
    note: meta.note || autoNote(platform, r, geo),
    parity: r?.parity ?? null,
  };
  if (meta.reason) out.reason = meta.reason;
  if (out.run) delete out.run.console?.sampleRaw;
  return out;
});

function autoNote(platform, r, geo) {
  if (!r || r.compile?.status !== 'ok') return 'Lässt sich für den PC nicht kompilieren (hardware-spezifischer Code).';
  if (r.run?.status === 'crash') return 'Kompiliert, stürzt beim Ausführen ab.';
  if (r.run?.status === 'timeout') return 'Kompiliert, blockiert beim Ausführen (Endlosschleife ohne Zeitfunktion).';
  if (r.run?.status === 'no-leds') return 'Kompiliert, registriert aber keine LEDs (nichts darzustellen).';
  if (platform === 'hardware') return 'Kompiliert und läuft, zeigt ohne passende Hardware aber nichts Sinnvolles.';
  const g = geo.dim === 2 && geo.width ? `${geo.width}×${geo.height}-Matrix` : geo.dim === 2 ? '2D-Layout' : `${r.run.ledCount} LEDs`;
  return `Läuft im PC-Modus (${g}).`;
}

const byCat = Object.fromEntries(CATEGORIES.map((c) => [c.id, 0]));
for (const e of examples) byCat[e.category] = (byCat[e.category] || 0) + 1;
const count = (f) => examples.filter(f).length;
const summary = {
  total: examples.length,
  compileOk: count((e) => e.compile.status === 'ok'),
  compileError: count((e) => e.compile.status === 'error'),
  run: Object.fromEntries(['ok', 'budget', 'crash', 'no-leds', 'timeout', 'not-run'].map((s) => [s, count((e) => e.run?.status === s)])),
  platform: { pc: count((e) => e.platform === 'pc'), hardware: count((e) => e.platform === 'hardware') },
  parity: {
    identical: count((e) => e.parity?.status === 'identical'),
    diff: count((e) => e.parity?.status === 'diff'),
    other: count((e) => e.parity && !['identical', 'diff'].includes(e.parity.status)),
  },
  byCategory: byCat,
};
const catalog = {
  schemaVersion: SCHEMA_VERSION,
  generatedAt: new Date().toISOString(),
  generator: 'fastled-integration/catalog/build-catalog.mjs',
  fastled: { version: rev.version, commit: rev.commit, repository: 'https://github.com/FastLED/FastLED', localPatch: rev.localPatch,
    license: 'MIT', licenseFile: 'LICENSE.FastLED' },
  toolchain: { zig: zigVersion(cfg), abiVersion: 1, frames: FRAMES, seed: SEED, schedule: 'tests/trace.mjs makeSchedule(frames, 1337)' },
  categories: CATEGORIES,
  summary,
  examples,
};
fs.writeFileSync(OUT_JSON, JSON.stringify(catalog, null, 1) + '\n');
log(`\nwrote ${path.relative(ROOT, OUT_JSON)} (${examples.length} examples) in ${((performance.now() - t0) / 1000).toFixed(0)} s`);
log(JSON.stringify(summary));
const { writeResultsMd } = await import('./results-md.mjs');
writeResultsMd(catalog, path.join(CAT, 'CATALOG_RESULTS.md'));
