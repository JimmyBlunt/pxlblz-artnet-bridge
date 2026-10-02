// PXLBLZ FastLED compile service (local only). No dependencies besides Node >= 22.
//   node service/server.mjs [--port 9996] [--host 127.0.0.1]
// Endpoints:
//   GET  /health                 liveness + versions (for offline detection in the IDE)
//   GET  /version                FastLED commit, zig version, ABI version, ...
//   POST /compile                {source, defines?, files?, options?} -> JSON {ok, wasm(base64), diagnostics, ...}
//                                (Accept: application/wasm -> raw bytes on success, JSON on failure)
//   GET  /artifact/<key>.wasm    cached module by key (immutable)
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { loadConfig, buildLib, compileSketch, toolInfo, asyncify, libDir } from '../toolchain/toolchain.mjs';

const SERVICE_VERSION = '1.0.0';
const ABI_VERSION = 1; // must match PXL_ABI_VERSION (runtime/pxl_runtime.cpp)
const MAX_BODY = 4 * 1024 * 1024;

const cfg = loadConfig();
const argv = process.argv.slice(2);
const arg = (k) => { const i = argv.indexOf(k); return i >= 0 ? argv[i + 1] : undefined; };
const svc = cfg.service || {};
const HOST = arg('--host') || process.env.PXL_FASTLED_HOST || svc.host || '127.0.0.1';
const PORT = +(arg('--port') || process.env.PXL_FASTLED_PORT || svc.port || 9996);
const ORIGINS = new Set(svc.origins || ['http://localhost:5174', 'http://localhost:5175']);
const MAX_PARALLEL = svc.maxParallel || 2;

const log = (...a) => console.log(new Date().toISOString().slice(11, 19), ...a);

// ------------------------------------------------------------------ compile queue
let active = 0;
const waiting = [];
const inflight = new Map(); // key -> promise (dedupe identical concurrent requests)
async function withSlot(fn) {
  if (active >= MAX_PARALLEL) await new Promise((r) => waiting.push(r));
  active++;
  try { return await fn(); } finally { active--; waiting.shift()?.(); }
}

let libReady = { wasm: false };
let libError = null;

function cors(req, res) {
  const origin = req.headers.origin;
  if (origin && (ORIGINS.has(origin) || ORIGINS.has('*'))) {
    res.setHeader('Access-Control-Allow-Origin', origin);
    res.setHeader('Vary', 'Origin');
    res.setHeader('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
    res.setHeader('Access-Control-Allow-Headers', 'content-type, accept');
    res.setHeader('Access-Control-Expose-Headers', 'x-pxl-key, x-pxl-cached, x-pxl-abi');
    res.setHeader('Access-Control-Max-Age', '600');
    if (req.headers['access-control-request-private-network'] === 'true') {
      res.setHeader('Access-Control-Allow-Private-Network', 'true');
    }
  }
}

function sendJson(res, status, obj) {
  const body = JSON.stringify(obj);
  res.writeHead(status, { 'content-type': 'application/json; charset=utf-8', 'cache-control': 'no-store' });
  res.end(body);
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    let n = 0;
    req.on('data', (c) => {
      n += c.length;
      if (n > MAX_BODY) { reject(Object.assign(new Error('request body too large'), { status: 413 })); req.destroy(); return; }
      chunks.push(c);
    });
    req.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')));
    req.on('error', reject);
  });
}

function versionInfo() {
  const t = toolInfo(cfg);
  return {
    service: 'pxlblz-fastled-compile', serviceVersion: SERVICE_VERSION, abiVersion: ABI_VERSION,
    fastled: t.fastled, fastledVersion: t.fastled.version, fastledCommit: t.fastled.commit,
    zig: t.zig, node: t.node, target: 'wasm32-wasi + binaryen asyncify', platform: t.platform,
  };
}

async function handleCompile(req, res) {
  let body;
  try { body = JSON.parse(await readBody(req)); } catch (e) {
    return sendJson(res, e.status || 400, { ok: false, error: e.status ? e.message : 'invalid JSON body' });
  }
  const { source, defines = {}, files = {}, options = {} } = body || {};
  if (typeof source !== 'string' || !source.trim()) return sendJson(res, 400, { ok: false, error: 'source (string) required' });
  if (typeof defines !== 'object' || Array.isArray(defines)) return sendJson(res, 400, { ok: false, error: 'defines must be an object' });
  const opts = {
    optimize: ['fast', 'size', 'full'].includes(options.optimize) ? options.optimize : 'fast',
    autoPrototypes: options.autoPrototypes !== false,
    fileName: typeof options.fileName === 'string' && /^[\w.\- ]+$/.test(options.fileName) ? options.fileName : 'sketch.ino',
  };
  const ac = new AbortController();
  res.on('close', () => { if (!res.writableEnded) ac.abort(); });
  const t0 = performance.now();
  let result;
  try {
    const dedupeKey = JSON.stringify([source, defines, files, opts]);
    let p = inflight.get(dedupeKey);
    if (!p) {
      p = withSlot(() => compileSketch(cfg, { source, defines, files, target: 'wasm', options: opts, signal: ac.signal }));
      inflight.set(dedupeKey, p);
      p.finally(() => inflight.delete(dedupeKey));
    }
    result = await p;
  } catch (e) {
    return sendJson(res, 400, { ok: false, error: String(e.message || e) });
  }
  if (ac.signal.aborted) return;
  const ms = performance.now() - t0;
  log(`compile ${result.ok ? 'ok ' : 'ERR'} key=${result.key} ${result.cached ? '(cached) ' : ''}${ms.toFixed(0)} ms` +
    (result.ok ? ` ${(result.size / 1024).toFixed(0)} KB` : ` ${result.diagnostics?.filter((d) => d.severity === 'error').length} errors`));
  const meta = {
    ok: result.ok, key: result.key, cached: !!result.cached, abiVersion: ABI_VERSION,
    diagnostics: (result.diagnostics || []).filter((d) => d.inSketch || d.severity === 'error'),
    prototypes: result.prototypes || [], timings: { ...result.timings, serviceMs: ms }, size: result.size,
    wasmUrl: result.ok ? `/artifact/${result.key}.wasm` : undefined,
    log: result.ok ? undefined : (result.log || '').split(/\r?\n/).filter((l) => !/nullability|_Nonnull|_Nullable/.test(l)).slice(0, 200).join('\n'),
  };
  if (result.ok && (req.headers.accept || '').includes('application/wasm')) {
    const bytes = fs.readFileSync(result.path);
    res.writeHead(200, { 'content-type': 'application/wasm', 'x-pxl-key': result.key, 'x-pxl-cached': String(!!result.cached), 'x-pxl-abi': String(ABI_VERSION) });
    return res.end(bytes);
  }
  if (result.ok && options.inline !== false) meta.wasm = fs.readFileSync(result.path).toString('base64');
  return sendJson(res, 200, meta);
}

function handleArtifact(req, res, key) {
  if (!/^[0-9a-f]{24}$/.test(key)) return sendJson(res, 400, { ok: false, error: 'bad key' });
  const file = path.join(cfg.cacheDir, 'sketch', 'wasm', key, 'sketch.wasm');
  if (!fs.existsSync(file)) return sendJson(res, 404, { ok: false, error: 'unknown artifact' });
  res.writeHead(200, { 'content-type': 'application/wasm', 'cache-control': 'public, max-age=31536000, immutable', 'x-pxl-abi': String(ABI_VERSION) });
  fs.createReadStream(file).pipe(res);
}

const server = http.createServer(async (req, res) => {
  cors(req, res);
  const url = new URL(req.url, 'http://x');
  try {
    if (req.method === 'OPTIONS') { res.writeHead(204); return res.end(); }
    if (req.method === 'GET' && url.pathname === '/health') {
      return sendJson(res, 200, { ok: !libError, ready: libReady.wasm, error: libError, busy: active, queued: waiting.length,
        version: SERVICE_VERSION, abiVersion: ABI_VERSION, fastledVersion: toolInfo(cfg).fastled.version });
    }
    if (req.method === 'GET' && url.pathname === '/version') return sendJson(res, 200, versionInfo());
    if (req.method === 'POST' && url.pathname === '/compile') return await handleCompile(req, res);
    const m = /^\/artifact\/([0-9a-f]+)\.wasm$/.exec(url.pathname);
    if (req.method === 'GET' && m) return handleArtifact(req, res, m[1]);
    if (req.method === 'GET' && url.pathname === '/') {
      res.writeHead(200, { 'content-type': 'text/plain; charset=utf-8' });
      return res.end('PXLBLZ FastLED compile service\nGET /health  GET /version  POST /compile  GET /artifact/<key>.wasm\n');
    }
    sendJson(res, 404, { ok: false, error: 'not found' });
  } catch (e) {
    log('internal error', e);
    if (!res.headersSent) sendJson(res, 500, { ok: false, error: String(e.message || e) });
  }
});

server.on('error', (e) => {
  if (e.code === 'EADDRINUSE') {
    console.error(`\nERROR: ${HOST}:${PORT} is already in use by another program.\n` +
      `The FastLED compile service did not start. Choose another port with\n` +
      `  node service/server.mjs --port <n>   or   set PXL_FASTLED_PORT=<n>   or   "service": {"port": <n>} in config.local.json\n`);
  } else console.error('server error:', e);
  process.exit(1);
});

server.listen(PORT, HOST, async () => {
  log(`PXLBLZ FastLED compile service listening on http://${HOST}:${PORT} (origins: ${[...ORIGINS].join(', ')})`);
  const v = versionInfo();
  log(`FastLED ${v.fastled.version} @ ${v.fastled.commit.slice(0, 10)}, zig ${v.zig}, ABI ${ABI_VERSION}`);
  try {
    await buildLib(cfg, 'wasm', { log: (s) => log(s) });
    libReady.wasm = true;
    // warm up binaryen (module load + JIT) with the smallest cached artifact, if any
    const dir = path.join(cfg.cacheDir, 'sketch', 'wasm');
    const raw = fs.existsSync(dir) ? fs.readdirSync(dir).map((k) => path.join(dir, k, 'sketch.raw.wasm')).find((f) => fs.existsSync(f)) : null;
    if (raw) await asyncify(fs.readFileSync(raw));
    log(`ready (library ${libDir(cfg, 'wasm')})`);
  } catch (e) {
    libError = String(e.message || e);
    log('library build failed:', libError);
  }
});
