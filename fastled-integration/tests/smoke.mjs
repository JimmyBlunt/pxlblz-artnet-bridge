// Quick manual check: node tests/smoke.mjs <sketch.wasm> [frames]
import fs from 'node:fs';
import { instantiateFastLedHost } from '../runtime/fastledWasmHost.ts';
const host = await instantiateFastLedHost(fs.readFileSync(process.argv[2]), { onConsole: (t, s) => console.log(`[${s}] ${t}`) });
host.init(1);
console.log('abi', host.abiVersion(), 'leds', host.ledCount());
console.log('ui', JSON.stringify(host.getUi().map(({ raw, ...e }) => e)));
const N = +(process.argv[3] || 120);
for (let i = 0; i < N; i++) {
  if (i === 60) { console.log('setUi Brightness=10 ->', host.setUi('Brightness', 10)); }
  const r = host.frame(16.667);
  if (i % 30 === 0 || i === 61) console.log(i, JSON.stringify(r), 'L1', [...host.getLeds().slice(0, 6)], 'L2', [...host.getWire().slice(0, 6)], 'raw', [...host.getWireRaw().slice(0, 6)], 'bri', host.brightness());
}
console.log(JSON.stringify(host.getStrips()));
console.log('ui after', JSON.stringify(host.getUi().map((e) => [e.name, e.value, e.defaultValue])));
