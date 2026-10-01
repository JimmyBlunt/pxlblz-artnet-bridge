// Temporary loopback-only WebSocket pass-through, for measuring browser output.
// Does not create or modify pixel frames; forwards to the existing bridge.
import net from 'node:net';
let frames = 0, changed = 0, bytes = 0, lit = 0, clients = 0;
const server = net.createServer(client => {
  clients++;
  const upstream = net.connect(9981, '127.0.0.1');
  let pending = Buffer.alloc(0), handshake = true, previous;
  client.pipe(upstream); upstream.pipe(client);
  client.on('data', data => {
    pending = Buffer.concat([pending, data]);
    if (handshake) {
      const end = pending.indexOf('\r\n\r\n');
      if (end < 0) return;
      pending = pending.subarray(end + 4); handshake = false;
    }
    while (pending.length >= 2) {
      const opcode = pending[0] & 15, masked = (pending[1] & 128) !== 0;
      let length = pending[1] & 127, offset = 2;
      if (length === 126) {
        if (pending.length < 4) return;
        length = pending.readUInt16BE(2); offset = 4;
      } else if (length === 127) {
        if (pending.length < 10) return;
        const big = pending.readBigUInt64BE(2);
        if (big > 1048576n) { client.destroy(); return; }
        length = Number(big); offset = 10;
      }
      const maskOffset = offset;
      if (masked) offset += 4;
      if (pending.length < offset + length) return;
      if (opcode === 2) {
        const payload = Buffer.from(pending.subarray(offset, offset + length));
        if (masked) for (let i = 0; i < length; i++) payload[i] ^= pending[maskOffset + (i % 4)];
        frames++; bytes += length;
        if (!previous || !previous.equals(payload)) changed++;
        previous = payload;
        lit = 0;
        for (let i = 0; i + 2 < length; i += 3) if (payload[i] || payload[i + 1] || payload[i + 2]) lit++;
      }
      pending = pending.subarray(offset + length);
    }
  });
  client.on('close', () => { clients--; upstream.destroy(); });
  client.on('error', error => console.error('client:', error.message));
  upstream.on('error', error => { console.error('bridge:', error.message); client.destroy(); });
});
server.listen(9982, '127.0.0.1', () => console.log('Measuring ws://127.0.0.1:9982/pixels -> 9981'));
let last = performance.now();
const timer = setInterval(() => {
  const now = performance.now(), seconds = (now - last) / 1000;
  console.log(JSON.stringify({time: new Date().toISOString(), clients, fps: +(frames / seconds).toFixed(1), changed, frameBytes: frames ? bytes / frames : 0, litPixels: lit}));
  frames = changed = bytes = 0; last = now;
}, 1000);
server.on('error', error => { clearInterval(timer); console.error(error.message); process.exitCode = 1; });
