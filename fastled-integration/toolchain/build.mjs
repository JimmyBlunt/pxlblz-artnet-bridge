// CLI: node toolchain/build.mjs <lib|native|all> [--force]
//      node toolchain/build.mjs sketch <file.ino> [--native] [-DNAME=VAL ...]
import { loadConfig, buildLib, compileSketch } from './toolchain.mjs';
import fs from 'node:fs';
const cfg = loadConfig();
const [cmd = 'all', ...rest] = process.argv.slice(2);
const force = rest.includes('--force');
if (cmd === 'lib' || cmd === 'all') await buildLib(cfg, 'wasm', { force });
if (cmd === 'native' || cmd === 'all') await buildLib(cfg, 'native', { force });
if (cmd === 'sketch') {
  const file = rest.find((a) => !a.startsWith('-'));
  const defines = Object.fromEntries(rest.filter((a) => a.startsWith('-D')).map((a) => { const [k, v] = a.slice(2).split('='); return [k, v === undefined ? true : (isNaN(+v) ? v : +v)]; }));
  const r = await compileSketch(cfg, { source: fs.readFileSync(file, 'utf8'), defines, target: rest.includes('--native') ? 'native' : 'wasm', options: { log: console.log } });
  for (const d of r.diagnostics || []) console.log(`${d.file}:${d.line}:${d.col}: ${d.severity}: ${d.message}`);
  console.log(JSON.stringify({ ok: r.ok, path: r.path, size: r.size, cached: r.cached, timings: r.timings, prototypes: r.prototypes }, null, 2));
  process.exit(r.ok ? 0 : 1);
}
