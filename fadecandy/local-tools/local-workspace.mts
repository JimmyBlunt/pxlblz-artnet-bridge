// Loopback-only entry point for the repository's documented Local Dev identity.
// Uses the existing local D1 server and signed session implementation.
// Never contacts an OAuth provider or reads/writes browser cookie databases.
import http from 'node:http';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { readFileSync } from 'node:fs';
import { createSessionCookie, createSessionToken, readSessionToken, parseCookieHeader, sessionCookieName } from '../PXLBLZ-IDE-main/src/cloudflare/auth.ts';
import { readDevVarsFile, localSessionUser } from '../PXLBLZ-IDE-main/scripts/dev-runtime-auth.ts';

const here = path.dirname(fileURLToPath(import.meta.url));
const main = path.resolve(here, '../PXLBLZ-IDE-main');
const vars = readDevVarsFile(path.join(main, '.dev.vars'));
const secret = vars.SESSION_SECRET;
if (!secret) throw new Error('The existing local server needs SESSION_SECRET.');
const manifest = JSON.parse(readFileSync(path.join(main, 'dev-runtime.json'), 'utf8'));
const user = localSessionUser({ developer: true }, { assignments: [] } as never, manifest);
const token = await createSessionToken(user, secret);
const cookie = `${sessionCookieName}=${encodeURIComponent(token)}`;
for (const endpoint of ['me', 'patterns', 'maps', 'shows']) {
  const response = await fetch(`http://localhost:5174/api/${endpoint}`, { headers: { Cookie: cookie } });
  if (!response.ok) throw new Error(`Local API ${endpoint}: HTTP ${response.status}`);
  const result = await response.json() as { authenticated?: boolean; user?: { id?: string } };
  if (endpoint === 'me' && (!result.authenticated || result.user?.id !== user.userId)) throw new Error('Local session was not accepted.');
  console.log(`Local API ${endpoint}: OK`);
}

const origin = 'http://127.0.0.1:5176';
const studio = '/PXLBLZ-IDE/studio/patterns/GyroidGlow3D?pxout=1&pxoutUrl=ws%3A%2F%2F127.0.0.1%3A9981%2Fpixels';
const server = http.createServer(async (req, res) => {
  try {
    if (req.headers.host !== '127.0.0.1:5176' || (req.headers.origin && req.headers.origin !== origin) || req.headers['sec-fetch-site'] === 'cross-site') {
      res.writeHead(403); res.end('This workspace is available only from its own loopback origin.'); return;
    }
    const url = new URL(req.url ?? '/', origin);
    if (url.pathname === '/local-workspace-login' || url.pathname === '/api/auth/login') {
      res.setHeader('Cache-Control', 'no-store');
      if (req.method === 'GET') {
        res.setHeader('Content-Type', 'text/html; charset=utf-8');
        res.setHeader('Content-Security-Policy', "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'");
        res.end('<!doctype html><html lang="de"><meta charset="utf-8"><title>PXLBLZ – lokaler Workspace</title><style>body{background:#101014;color:#eee;font:18px system-ui;max-width:650px;margin:12vh auto;padding:24px}button{font:inherit;padding:14px;background:#e8b83c;border:0;border-radius:8px}p{line-height:1.6}</style><h1>Lokaler PXLBLZ-Workspace</h1><p>Benutzer: <b>Local Dev</b>. Patterns, Maps und Shows werden in der vorhandenen lokalen Datenbank auf diesem PC gespeichert. Dieser Workspace wird nicht mit GitHub oder der Online-Version synchronisiert.</p><form method="post" action="/local-workspace-login"><button>Lokalen Workspace öffnen</button></form></html>');
        return;
      }
      if (req.method !== 'POST' || req.headers.origin !== origin) { res.writeHead(403); res.end('Same-origin login required.'); return; }
      const old = parseCookieHeader(req.headers.cookie ?? '')[sessionCookieName];
      if (old) {
        const session = await readSessionToken(old, secret);
        if (!session || session.userId !== user.userId) { res.writeHead(409); res.end('Another session exists on this origin; it has not been replaced.'); return; }
      }
      res.setHeader('Set-Cookie', await createSessionCookie(user, secret, { secure: false }));
      res.writeHead(303, { Location: studio }); res.end(); return;
    }
    const isApi = url.pathname === '/api' || url.pathname.startsWith('/api/');
    const upstream = http.request({ hostname: 'localhost', port: isApi ? 5174 : 5175, path: req.url, method: req.method, headers: { ...req.headers, host: `localhost:${isApi ? 5174 : 5175}` } }, response => {
      res.writeHead(response.statusCode ?? 502, response.headers); response.pipe(res);
    });
    upstream.on('error', () => { if (!res.headersSent) res.writeHead(502); res.end('Local PXLBLZ server is unavailable.'); });
    req.pipe(upstream);
  } catch (error) { console.error(error instanceof Error ? error.message : 'Local workspace error'); if (!res.headersSent) res.writeHead(500); res.end('Local workspace error.'); }
});
server.on('upgrade', (req, socket, head) => {
  if (req.headers.host !== '127.0.0.1:5176' || (req.headers.origin && req.headers.origin !== origin)) { socket.destroy(); return; }
  const upstream = http.request({ hostname:'localhost', port:5175, path:req.url, headers:{ ...req.headers, host:'localhost:5175' } });
  upstream.on('upgrade', (response, remote, remoteHead) => {
    socket.write(`HTTP/1.1 ${response.statusCode} ${response.statusMessage}\r\n` + Object.entries(response.headers).map(([k,v]) => `${k}: ${v}\r\n`).join('') + '\r\n');
    if (remoteHead.length) socket.write(remoteHead);
    if (head.length) remote.write(head);
    socket.pipe(remote); remote.pipe(socket);
    socket.on('error', () => remote.destroy()); remote.on('error', () => socket.destroy());
  });
  upstream.on('error', () => socket.destroy()); upstream.end();
});
server.listen(5176, '127.0.0.1', () => console.log(`${origin}/local-workspace-login`));
server.on('error', error => { console.error(error.message); process.exitCode = 1; });
