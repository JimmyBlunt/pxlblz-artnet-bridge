// Frame-time benchmark in Node: node tests/bench.mjs <sketch.wasm> [frames] [dtMs]
import fs from 'node:fs';
import { instantiateFastLedHost } from '../runtime/fastledWasmHost.ts';
const file = process.argv[2], N = +(process.argv[3] || 600), dt = +(process.argv[4] || 16.667);
const host = await instantiateFastLedHost(fs.readFileSync(file));
host.init(1);
for (let i = 0; i < 30; i++) host.frame(dt); // warm-up (JIT tier-up)
const t = new Float64Array(N);
let sink = 0;
for (let i = 0; i < N; i++) {
  const t0 = performance.now();
  host.frame(dt);
  const w = host.getWire(); sink += w[0];
  t[i] = performance.now() - t0;
}
t.sort();
const avg = t.reduce((a, b) => a + b, 0) / N;
console.log(JSON.stringify({ file: file.split(/[\/]/).slice(-2).join('/'), leds: host.ledCount(), frames: N, avgMs: +avg.toFixed(3), p50: +t[N >> 1].toFixed(3), p99: +t[Math.floor(N * 0.99)].toFixed(3), showsPerFrame: +(host.showCount() / (N + 30)).toFixed(2), sink }));
