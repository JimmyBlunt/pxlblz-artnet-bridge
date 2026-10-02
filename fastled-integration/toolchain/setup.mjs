// One-time setup (idempotent):  node toolchain/setup.mjs   (or: npm run setup)
//  1. FastLED source at the pinned commit (+ toolchain/fastled-local.patch) -> config fastledDir
//     (default vendor/fastled, shallow clone of exactly that commit)
//  2. zig (PyPI package "ziglang", pinned) in a venv via uv, or python -m venv + pip -> config zig
//  3. npm dependencies (binaryen)
//  4. writes config.local.json with the resolved paths (machine specific, not committed)
// Options: --fastled <dir>  --zig <zig.exe>  --venv <dir>
import { execFileSync, spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const PIN = JSON.parse(fs.readFileSync(path.join(ROOT, 'toolchain', 'pins.json'), 'utf8'));
const argv = process.argv.slice(2);
const arg = (k) => { const i = argv.indexOf(k); return i >= 0 ? argv[i + 1] : undefined; };
const read = (f) => (fs.existsSync(f) ? JSON.parse(fs.readFileSync(f, 'utf8')) : {});
const base = read(path.join(ROOT, 'config.json'));
const local = read(path.join(ROOT, 'config.local.json'));
const cfg = { ...base, ...local };
const abs = (p) => path.resolve(ROOT, p);
const rel = (p) => path.relative(ROOT, p).replace(/\\/g, '/') || '.';
const sh = (cmd, args, opts = {}) => {
  console.log(`> ${cmd} ${args.join(' ')}`);
  const r = spawnSync(cmd, args, { stdio: 'inherit', shell: false, ...opts });
  if (r.status !== 0) throw new Error(`${cmd} failed (${r.status ?? r.error})`);
};
const has = (cmd, args = ['--version']) => spawnSync(cmd, args, { stdio: 'ignore' }).status === 0;

// ---------------------------------------------------------------- 1. FastLED
let fastledDir = abs(arg('--fastled') || cfg.fastledDir || 'vendor/fastled');
const headOf = (d) => { try { return execFileSync('git', ['-C', d, 'rev-parse', 'HEAD']).toString().trim(); } catch { return ''; } };
if (!fs.existsSync(path.join(fastledDir, 'src', 'FastLED.h'))) {
  console.log(`FastLED not found at ${fastledDir}: cloning ${PIN.fastled.repo} @ ${PIN.fastled.commit}`);
  fs.mkdirSync(fastledDir, { recursive: true });
  sh('git', ['-C', fastledDir, 'init', '-q']);
  sh('git', ['-C', fastledDir, 'remote', 'add', 'origin', PIN.fastled.repo]);
  sh('git', ['-C', fastledDir, 'fetch', '--depth', '1', 'origin', PIN.fastled.commit]);
  sh('git', ['-C', fastledDir, 'checkout', '-q', 'FETCH_HEAD']);
}
const head = headOf(fastledDir);
if (head && head !== PIN.fastled.commit) console.warn(`WARNING: ${fastledDir} is at ${head}, pinned is ${PIN.fastled.commit}`);
const patch = path.join(ROOT, 'toolchain', 'fastled-local.patch');
const applied = spawnSync('git', ['-C', fastledDir, 'apply', '--reverse', '--check', patch], { stdio: 'ignore' }).status === 0;
if (!applied) {
  if (spawnSync('git', ['-C', fastledDir, 'apply', '--check', patch], { stdio: 'ignore' }).status === 0) sh('git', ['-C', fastledDir, 'apply', patch]);
  else console.warn('WARNING: toolchain/fastled-local.patch neither applied nor applicable (check manually)');
} else console.log('FastLED local patch: already applied');

// ---------------------------------------------------------------- 2. zig
let zig = arg('--zig') ? abs(arg('--zig')) : (cfg.zig ? abs(cfg.zig) : '');
const zigOk = (z) => z && fs.existsSync(z) && spawnSync(z, ['version'], { encoding: 'utf8' }).stdout?.trim() === PIN.zig.version;
if (!zigOk(zig)) {
  const venv = abs(arg('--venv') || '.venv');
  const py = process.platform === 'win32' ? path.join(venv, 'Scripts', 'python.exe') : path.join(venv, 'bin', 'python');
  console.log(`installing zig ${PIN.zig.version} (PyPI ziglang) into ${venv}`);
  if (has('uv')) {
    if (!fs.existsSync(py)) sh('uv', ['venv', venv]);
    sh('uv', ['pip', 'install', '--python', py, `ziglang==${PIN.zig.version}`], { env: { ...process.env, UV_CACHE_DIR: path.join(ROOT, '.cache', 'uv') } });
  } else {
    const sys = ['python', 'python3', 'py'].find((p) => has(p));
    if (!sys) throw new Error('neither uv nor python found; install uv (https://docs.astral.sh/uv/) or Python 3');
    if (!fs.existsSync(py)) sh(sys, ['-m', 'venv', venv]);
    sh(py, ['-m', 'pip', 'install', `ziglang==${PIN.zig.version}`]);
  }
  const site = process.platform === 'win32' ? path.join(venv, 'Lib', 'site-packages') : fs.readdirSync(path.join(venv, 'lib')).map((d) => path.join(venv, 'lib', d, 'site-packages'))[0];
  zig = path.join(site, 'ziglang', process.platform === 'win32' ? 'zig.exe' : 'zig');
  if (!zigOk(zig)) throw new Error(`zig not usable at ${zig}`);
}
console.log(`zig ${PIN.zig.version}: ${zig}`);

// ---------------------------------------------------------------- 3. npm
if (!fs.existsSync(path.join(ROOT, 'node_modules', 'binaryen'))) {
  const npm = process.platform === 'win32' ? 'npm.cmd' : 'npm';
  sh(npm, ['install', '--no-audit', '--no-fund'], { cwd: ROOT, shell: process.platform === 'win32' });
}

// ---------------------------------------------------------------- 4. config.local.json
const out = { ...local, fastledDir: rel(fastledDir), zig: rel(zig) };
fs.writeFileSync(path.join(ROOT, 'config.local.json'), JSON.stringify(out, null, 2) + '\n');
console.log(`wrote config.local.json: ${JSON.stringify(out)}`);
console.log('next: npm run build   (precompiles FastLED for wasm + native, ~2-3 min cold)');
