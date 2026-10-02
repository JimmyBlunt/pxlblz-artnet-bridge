// PXLBLZ FastLED toolchain: zig (clang) -> wasm32-wasi + binaryen Asyncify.
// Used by toolchain/build.mjs (CLI), service/server.mjs and tests/verify.mjs.
// All paths are resolved relative to this folder's parent (the integration root)
// or come from config.json / config.local.json there.
import { spawn, execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import { fileURLToPath } from 'node:url';

export const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

export function loadConfig() {
  const read = (f) => (fs.existsSync(f) ? JSON.parse(fs.readFileSync(f, 'utf8')) : {});
  const cfg = { ...read(path.join(ROOT, 'config.json')), ...read(path.join(ROOT, 'config.local.json')) };
  const abs = (p) => (p ? path.resolve(ROOT, p) : p);
  cfg.fastledDir = abs(process.env.PXL_FASTLED_DIR || cfg.fastledDir);
  cfg.zig = abs(process.env.PXL_ZIG || cfg.zig);
  cfg.cacheDir = abs(cfg.cacheDir || '.cache');
  cfg.buildDir = abs(cfg.buildDir || 'build');
  cfg.jobs = cfg.jobs || 3;
  return cfg;
}

// ------------------------------------------------------------------ flags
// Identical FastLED configuration for the wasm build and the native reference.
export function commonFlags(cfg) {
  return [
    '-std=gnu++17', '-O2', '-ffp-contract=off', '-fno-exceptions', '-fno-rtti',
    '-DSTUB_PLATFORM', '-DFASTLED_STUB_IMPL', '-DFASTLED_USE_STUB_ARDUINO',
    '-DFASTLED_MULTITHREADED=0', '-DFL_STUB_HAS_MULTITHREADED=0',
    // deterministic Arduino random()/rand() (runtime/pxl_runtime.cpp)
    '-Drand=pxl_rand', '-Dsrand=pxl_srand', '-Drandom=pxl_random',
    '-I' + path.join(ROOT, 'toolchain', 'override'),
    '-isystem', path.join(cfg.fastledDir, 'src'),
    '-include', path.join(ROOT, 'toolchain', 'pxl_prelude.h'),
  ];
}
export const TARGETS = {
  wasm: { triple: 'wasm32-wasi', extra: ['-U__wasm__'] },
  native: { triple: 'x86_64-windows-gnu', extra: ['-mcpu=baseline'] },
};
export function targetFlags(cfg, target) {
  const t = TARGETS[target];
  return ['-target', t.triple, ...t.extra, ...commonFlags(cfg)];
}

// FastLED unity build units that are never needed by sketches (smaller cold build).
// Linking uses an archive, so unused units would be dropped anyway.
export const EXCLUDED_UNITS = new Set(['fl.test']);

export function libUnits(cfg) {
  const dir = path.join(cfg.fastledDir, 'src', 'fl', 'build');
  return fs.readdirSync(dir).filter((f) => f.endsWith('.cpp'))
    .map((f) => ({ name: f.replace(/\.cpp$/, '').replace(/\+/g, ''), file: path.join(dir, f) }))
    .filter((u) => !EXCLUDED_UNITS.has(u.name));
}

// ------------------------------------------------------------------ process helpers
export function zigEnv(cfg) {
  const zc = path.join(cfg.cacheDir, 'zig');
  return { ...process.env, ZIG_GLOBAL_CACHE_DIR: zc, ZIG_LOCAL_CACHE_DIR: zc };
}
export function run(cmd, args, opts = {}) {
  return new Promise((resolve) => {
    const t0 = performance.now();
    const p = spawn(cmd, args, { env: opts.env, cwd: opts.cwd, windowsHide: true });
    let out = '', err = '';
    p.stdout.on('data', (d) => (out += d));
    p.stderr.on('data', (d) => (err += d));
    if (opts.signal) opts.signal.addEventListener('abort', () => p.kill(), { once: true });
    p.on('error', (e) => resolve({ code: -1, out, err: err + String(e), ms: performance.now() - t0 }));
    p.on('close', (code) => resolve({ code, out, err, ms: performance.now() - t0 }));
  });
}
async function pool(items, n, fn) {
  const res = new Array(items.length);
  let i = 0;
  await Promise.all(Array.from({ length: Math.min(n, items.length) }, async () => {
    while (i < items.length) { const k = i++; res[k] = await fn(items[k], k); }
  }));
  return res;
}
const sha = (s) => createHash('sha256').update(s).digest('hex');

let _zigVersion;
export function zigVersion(cfg) {
  if (!_zigVersion) _zigVersion = execFileSync(cfg.zig, ['version'], { env: zigEnv(cfg) }).toString().trim();
  return _zigVersion;
}
let _fastledRev;
export function fastledRevision(cfg) {
  if (_fastledRev) return _fastledRev;
  const git = (a) => { try { return execFileSync('git', ['-C', cfg.fastledDir, ...a]).toString(); } catch { return ''; } };
  const commit = git(['rev-parse', 'HEAD']).trim() || 'unknown';
  const diff = git(['diff', '--', 'src']);
  let version = 'unknown';
  try { version = /version=(.*)/.exec(fs.readFileSync(path.join(cfg.fastledDir, 'library.properties'), 'utf8'))[1].trim(); } catch {}
  _fastledRev = { commit, version, localPatch: diff ? sha(diff).slice(0, 12) : null };
  return _fastledRev;
}

function dirHash(dir) {
  const h = createHash('sha256');
  const walk = (d) => {
    for (const e of fs.readdirSync(d, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name))) {
      const p = path.join(d, e.name);
      if (e.isDirectory()) walk(p); else { h.update(path.relative(dir, p)); h.update(fs.readFileSync(p)); }
    }
  };
  walk(dir);
  return h.digest('hex');
}

// Key of the precompiled library: compiler, flags, FastLED revision, overrides, runtime.
export function libKey(cfg, target) {
  return sha(JSON.stringify({
    zig: zigVersion(cfg), flags: targetFlags(cfg, target).map((f) => f.replace(ROOT, '<root>').replace(cfg.fastledDir, '<fastled>')),
    fastled: fastledRevision(cfg), excluded: [...EXCLUDED_UNITS],
    tool: dirHash(path.join(ROOT, 'toolchain', 'override')) + dirHash(path.join(ROOT, 'toolchain', 'include')) +
      sha(fs.readFileSync(path.join(ROOT, 'toolchain', 'pxl_prelude.h'))),
    runtime: sha(fs.readFileSync(path.join(ROOT, 'runtime', 'pxl_runtime.cpp'))) +
      sha(fs.readFileSync(path.join(ROOT, 'native', 'fiber_win.cpp'))) + sha(fs.readFileSync(path.join(ROOT, 'native', 'harness.cpp'))),
  })).slice(0, 16);
}

export function libDir(cfg, target) { return path.join(cfg.buildDir, target); }

// Builds (once, cached) the FastLED archive + runtime object for a target.
export async function buildLib(cfg, target, { log = console.log, force = false } = {}) {
  const dir = libDir(cfg, target);
  const objDir = path.join(dir, 'obj');
  fs.mkdirSync(objDir, { recursive: true });
  const key = libKey(cfg, target);
  const stampFile = path.join(dir, 'stamp.json');
  const archive = path.join(dir, 'libfastled.a');
  const runtimeObj = path.join(dir, 'pxl_runtime.o');
  const extraObjs = target === 'native' ? [path.join(dir, 'fiber_win.o'), path.join(dir, 'harness.o')] : [];
  const pch = path.join(dir, 'pxl_sketch_fastled.pch');
  let stamp = {};
  try { stamp = JSON.parse(fs.readFileSync(stampFile, 'utf8')); } catch {}
  if (!force && stamp.key === key && [archive, runtimeObj, pch, ...extraObjs].every((f) => fs.existsSync(f))) {
    return { archive, runtimeObj, extraObjs, pch, key, cached: true, ms: 0 };
  }
  const t0 = performance.now();
  const flags = targetFlags(cfg, target);
  const env = zigEnv(cfg);
  const jobs = [
    ...libUnits(cfg).map((u) => ({ src: u.file, obj: path.join(objDir, u.name + '.o'), name: u.name })),
    { src: path.join(ROOT, 'runtime', 'pxl_runtime.cpp'), obj: runtimeObj, name: 'pxl_runtime' },
    { src: path.join(ROOT, 'toolchain', 'include', 'pxl_sketch_fastled.h'), obj: pch, name: 'pch', pch: true },
  ];
  if (target === 'native') {
    jobs.push({ src: path.join(ROOT, 'native', 'fiber_win.cpp'), obj: extraObjs[0], name: 'fiber_win', plain: true });
    jobs.push({ src: path.join(ROOT, 'native', 'harness.cpp'), obj: extraObjs[1], name: 'harness', plain: true });
  }
  log(`[${target}] compiling ${jobs.length} units (key ${key}, ${cfg.jobs} parallel) ...`);
  const results = await pool(jobs, cfg.jobs, async (j) => {
    const args = j.plain ? ['c++', '-target', TARGETS[target].triple, ...TARGETS[target].extra, '-O2', '-c', j.src, '-o', j.obj]
      : j.pch ? ['c++', ...flags, '-I' + path.join(ROOT, 'toolchain', 'include'), '-x', 'c++-header', j.src, '-o', j.obj]
      : ['c++', ...flags, '-w', '-c', j.src, '-o', j.obj];
    const r = await run(cfg.zig, args, { env });
    log(`  ${r.code === 0 ? 'ok  ' : 'FAIL'} ${j.name} (${(r.ms / 1000).toFixed(1)} s)`);
    if (r.code !== 0) log(r.err.split('\n').filter((l) => /error/.test(l)).slice(0, 10).join('\n'));
    return r;
  });
  if (results.some((r) => r.code !== 0)) throw new Error(`[${target}] library build failed`);
  if (fs.existsSync(archive)) fs.rmSync(archive);
  const unitObjs = jobs.filter((j) => !['pxl_runtime', 'fiber_win', 'harness', 'pch'].includes(j.name)).map((j) => j.obj);
  const ar = await run(cfg.zig, ['ar', 'rcs', archive, ...unitObjs], { env });
  if (ar.code !== 0) throw new Error('zig ar failed: ' + ar.err);
  const ms = performance.now() - t0;
  fs.writeFileSync(stampFile, JSON.stringify({ key, builtAt: new Date().toISOString(), ms: Math.round(ms) }, null, 2));
  log(`[${target}] library ready in ${(ms / 1000).toFixed(1)} s`);
  return { archive, runtimeObj, extraObjs, pch, key, cached: false, ms };
}

// ------------------------------------------------------------------ sketch preprocessing
// Arduino IDE semantics: an implicit #include <Arduino.h> (force-include) and
// automatically generated prototypes for functions defined at top level, so
// sketches may call functions defined further down. Prototypes are inserted
// before the first function definition; #line keeps diagnostics on user lines.
export function preprocessSketch(source, fileName = 'sketch.ino') {
  const src = source.replace(/\r\n?/g, '\n');
  const lines = src.split('\n');
  // Mask comments and string literals (keep length / newlines) for scanning.
  const masked = src.replace(/\/\*[\s\S]*?\*\/|\/\/[^\n]*|"(?:\\.|[^"\\\n])*"|'(?:\\.|[^'\\\n])*'/g,
    (m) => m.replace(/[^\n]/g, ' '));
  const protos = [];
  let firstDefLine = -1;
  // top-level definitions: depth 0, "type name(args) {"
  let depth = 0;
  const re = /([A-Za-z_][\w:<>,\s\*&]*?[\s\*&])([A-Za-z_]\w*)\s*\(([^()]*(?:\([^()]*\)[^()]*)*)\)\s*(?:const\s*)?\{/y;
  const declared = new Set();
  const lineOf = (idx) => masked.slice(0, idx).split('\n').length; // 1-based
  for (let i = 0; i < masked.length; i++) {
    const c = masked[i];
    if (c === '{') { depth++; continue; }
    if (c === '}') { depth = Math.max(0, depth - 1); continue; }
    if (depth !== 0) continue;
    if (i > 0 && !/[\n;}]/.test(masked[i - 1]) && !(masked[i - 1] === ' ' && /\n\s*$/.test(masked.slice(Math.max(0, i - 200), i)))) continue;
    re.lastIndex = i;
    const m = re.exec(masked);
    if (!m) continue;
    const ret = m[1].trim(), name = m[2], args = m[3];
    const lineStart = masked.lastIndexOf('\n', i) + 1;
    const lineText = masked.slice(lineStart, masked.indexOf('\n', i) === -1 ? undefined : masked.indexOf('\n', i));
    if (/^\s*#/.test(lineText)) continue;
    if (/\b(if|for|while|switch|return|else|do|sizeof|catch)\b/.test(name) || /\b(class|struct|enum|union|namespace|typedef|template|operator|return|new|delete|static_assert)\b/.test(ret)) continue;
    if (ret.includes('::') || name === 'main' || /=/.test(ret)) continue;
    if (/\b(setup|loop)\b/.test(name) && args.trim() === '' ) { if (firstDefLine < 0) firstDefLine = lineOf(i); continue; }
    if (firstDefLine < 0) firstDefLine = lineOf(i);
    // Skip if a parameter has a default value or uses a type declared later (can't know): keep simple.
    if (/=/.test(args)) continue;
    const proto = `${ret} ${name}(${args.replace(/\s+/g, ' ').trim()});`;
    if (!declared.has(proto)) { declared.add(proto); protos.push(proto); }
    i = re.lastIndex - 1;
    depth = 1;
  }
  if (!protos.length || firstDefLine < 0) {
    return { code: `#line 1 "${fileName}"\n${src}\n`, prototypes: [] };
  }
  const before = lines.slice(0, firstDefLine - 1).join('\n');
  const after = lines.slice(firstDefLine - 1).join('\n');
  const code = `#line 1 "${fileName}"\n${before}\n// --- auto-generated prototypes (Arduino IDE semantics)\n${protos.join('\n')}\n#line ${firstDefLine} "${fileName}"\n${after}\n`;
  return { code, prototypes: protos };
}

// ------------------------------------------------------------------ diagnostics
export function parseDiagnostics(stderr, fileName = 'sketch.ino', extraNames = []) {
  const diags = [];
  const re = /^(.*?):(\d+):(\d+): (fatal error|error|warning|note): (.*)$/;
  for (const line of stderr.split(/\r?\n/)) {
    const m = re.exec(line);
    if (!m) continue;
    const file = m[1].replace(/\\/g, '/');
    const base = file.split('/').pop();
    const own = file === fileName || file.endsWith('/' + fileName) ? fileName : (extraNames.includes(base) ? base : null);
    const inSketch = !!own;
    diags.push({
      file: own || file.replace(/^.*?\/(src|override|include)\//, '$1/'),
      line: +m[2], col: +m[3], severity: m[4] === 'fatal error' ? 'error' : m[4], message: m[5], inSketch,
    });
  }
  return diags;
}

// ------------------------------------------------------------------ asyncify
let _binaryen;
async function binaryen() {
  if (!_binaryen) _binaryen = (await import('binaryen')).default;
  return _binaryen;
}
// level 'fast' (default): Asyncify only (~0.6 s, ~2x size, same frame time in practice);
// 'size': + cheap local cleanups (~1.4 s); 'full': binaryen -O2 on the whole module (~10+ s, smallest).
export async function asyncify(rawBytes, level = 'fast') {
  const b = await binaryen();
  const mod = b.readBinary(rawBytes);
  mod.setFeatures(b.Features.MVP | b.Features.MutableGlobals | b.Features.SignExt | b.Features.BulkMemory |
    b.Features.BulkMemoryOpt | b.Features.CallIndirectOverlong | b.Features.NontrappingFPToInt |
    b.Features.Multivalue | b.Features.ReferenceTypes);
  b.setPassArgument('asyncify-imports', 'env.pxl_suspend');
  if (level === 'full') {
    b.setOptimizeLevel(2); b.setShrinkLevel(1);
    mod.runPasses(['asyncify']);
    mod.optimize();
  } else {
    b.setOptimizeLevel(0); b.setShrinkLevel(0);
    mod.runPasses(level === 'size'
      ? ['asyncify', 'simplify-locals', 'vacuum', 'merge-blocks', 'remove-unused-brs', 'coalesce-locals', 'reorder-locals', 'vacuum']
      : ['asyncify']);
  }
  const out = mod.emitBinary();
  mod.dispose();
  return out;
}

// ------------------------------------------------------------------ compile a sketch
export const WASM_LINK_FLAGS = [
  '-mexec-model=reactor', '-s',
  '-Wl,-z,stack-size=4194304', '-Wl,--gc-sections',
];

function definesToFlags(defines = {}) {
  const out = [];
  for (const [k, v] of Object.entries(defines)) {
    if (!/^[A-Za-z_]\w*$/.test(k)) throw new Error(`invalid define name: ${k}`);
    if (v === true || v === undefined || v === null) out.push(`-D${k}`);
    else if (typeof v === 'number' && Number.isFinite(v)) out.push(`-D${k}=${v}`);
    else if (typeof v === 'string' && /^[\w .+\-"'()]*$/.test(v)) out.push(`-D${k}=${v}`);
    else throw new Error(`invalid define value for ${k}`);
  }
  return out;
}

// True if the sketch #defines something before its first FastLED include: then FastLED.h must
// not be pre-included (precompiled), so the define can still influence it.
export function definesBeforeFastLED(source) {
  for (const line of source.split(/\r?\n/)) {
    if (/^\s*#\s*include\s*[<"](FastLED\.h|fastled\.h)[>"]/.test(line)) return false;
    const m = /^\s*#\s*define\s+(\w+)/.exec(line);
    if (m && m[1] !== 'FASTLED_INTERNAL') return true;
  }
  return false;
}

export function sketchKey(cfg, target, source, defines, options = {}, files = {}) {
  return sha(JSON.stringify({ lib: libKey(cfg, target), source, defines: defines || {}, files, p: options.autoPrototypes !== false, o: options.optimize || 'fast', n: options.fileName || '' })).slice(0, 24);
}

// Compiles + links a sketch. Returns { ok, wasm|exe, diagnostics, timings, key, cached }.
// files: additional sketch-folder files { 'name.h': text, 'other.cpp': text } (.cpp/.c are compiled and linked too).
export async function compileSketch(cfg, { source, defines = {}, files = {}, target = 'wasm', options = {}, signal, outName, extraSources = [] }) {
  for (const name of Object.keys(files)) {
    if (!/^[\w.-]+\.(h|hpp|hh|inc|c|cpp|cc)$/.test(name) || name.startsWith('.') || name === 'sketch.ino.cpp') {
      throw new Error(`invalid sketch file name: ${name}`);
    }
  }
  const t0 = performance.now();
  const lib = await buildLib(cfg, target, { log: options.log || (() => {}) });
  const key = sketchKey(cfg, target, source, defines, options, files);
  const dir = path.join(cfg.cacheDir, 'sketch', target, key);
  const out = path.join(dir, outName || (target === 'wasm' ? 'sketch.wasm' : 'sketch.exe'));
  const metaFile = path.join(dir, 'result.json');
  if (fs.existsSync(out) && fs.existsSync(metaFile)) {
    const meta = JSON.parse(fs.readFileSync(metaFile, 'utf8'));
    return { ...meta, ok: true, path: out, key, cached: true, timings: { totalMs: performance.now() - t0 } };
  }
  fs.mkdirSync(dir, { recursive: true });
  const fileName = options.fileName || 'sketch.ino';
  const pre = options.autoPrototypes === false ? { code: `#line 1 "${fileName}"\n${source}\n`, prototypes: [] } : preprocessSketch(source, fileName);
  const srcFile = path.join(dir, 'sketch.ino.cpp');
  fs.writeFileSync(srcFile, pre.code);
  for (const [name, text] of Object.entries(files)) fs.writeFileSync(path.join(dir, name), text);
  const env = zigEnv(cfg);
  const flags = targetFlags(cfg, target);
  const timings = {};
  const tus = [{ src: srcFile, obj: path.join(dir, 'sketch.o'), diagName: fileName }];
  for (const name of Object.keys(files)) {
    if (/\.(c|cpp|cc)$/.test(name)) tus.push({ src: path.join(dir, name), obj: path.join(dir, name + '.o'), diagName: name });
  }
  const usePch = options.pch !== false && !definesBeforeFastLED(source);
  timings.pch = usePch;
  const ccs = await Promise.all(tus.map((tu) => run(cfg.zig, ['c++', ...flags, '-I' + path.join(ROOT, 'toolchain', 'include'),
    ...(usePch ? ['-include-pch', lib.pch] : ['-include', path.join(ROOT, 'toolchain', 'include', 'pxl_sketch.h')]), ...definesToFlags(defines),
    '-Wall', '-Wno-unused-variable', '-Wno-unused-function', '-fno-caret-diagnostics',
    ...(tu.src.endsWith('.c') ? ['-x', 'c++'] : []), '-c', tu.src, '-o', tu.obj], { env, cwd: dir, signal })));
  timings.compileMs = Math.max(...ccs.map((c) => c.ms));
  let diagnostics = ccs.flatMap((c) => parseDiagnostics(c.err, fileName, Object.keys(files)));
  const cc = { code: ccs.some((c) => c.code !== 0) ? 1 : 0, err: ccs.map((c) => c.err).join('') };
  if (cc.code !== 0) {
    return { ok: false, diagnostics, log: cc.err, key, timings: { ...timings, totalMs: performance.now() - t0 }, prototypes: pre.prototypes };
  }
  const objs = [...tus.map((t) => t.obj), ...extraSources];
  let link;
  if (target === 'wasm') {
    const raw = path.join(dir, 'sketch.raw.wasm');
    link = await run(cfg.zig, ['c++', '-target', 'wasm32-wasi', '-O2', ...WASM_LINK_FLAGS,
      ...objs, lib.runtimeObj, lib.archive, '-o', raw], { env, cwd: dir, signal });
    timings.linkMs = link.ms;
    if (link.code === 0) {
      const ta = performance.now();
      const bytes = await asyncify(fs.readFileSync(raw), options.optimize || 'fast');
      fs.writeFileSync(out, bytes);
      timings.asyncifyMs = performance.now() - ta;
      timings.rawBytes = fs.statSync(raw).size;
    }
  } else {
    link = await run(cfg.zig, ['c++', '-target', TARGETS[target].triple, ...TARGETS[target].extra, '-O2',
      ...objs, lib.runtimeObj, ...lib.extraObjs, lib.archive, '-lws2_32', '-lwinmm', '-lbcrypt', '-ladvapi32', '-o', out], { env, cwd: dir, signal });
    timings.linkMs = link.ms;
  }
  diagnostics = diagnostics.concat(parseDiagnostics(link.err, fileName));
  if (link.code !== 0) {
    const undef = [...link.err.matchAll(/undefined symbol: (.*)/g)].map((m) => m[1]);
    for (const u of undef) diagnostics.push({ file: fileName, line: 0, col: 0, severity: 'error', message: `undefined symbol: ${u}`, inSketch: true });
    if (!undef.length) diagnostics.push({ file: fileName, line: 0, col: 0, severity: 'error', message: link.err.trim().split('\n').slice(0, 5).join(' | '), inSketch: true });
    return { ok: false, diagnostics, log: cc.err + link.err, key, timings: { ...timings, totalMs: performance.now() - t0 } };
  }
  timings.totalMs = performance.now() - t0;
  const meta = { diagnostics, prototypes: pre.prototypes, size: fs.statSync(out).size, timings, target, lib: lib.key };
  fs.writeFileSync(metaFile, JSON.stringify(meta, null, 2));
  return { ...meta, ok: true, path: out, key, cached: false };
}

export function toolInfo(cfg) {
  return { zig: zigVersion(cfg), fastled: fastledRevision(cfg), platform: `${os.platform()}-${os.arch()}`, node: process.version };
}
