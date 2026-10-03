// Local login for PXLBLZ-IDE~FastLED: mints the local developer session (no Google/GitHub),
// verifies it against the API (5174) and the FastLED IDE (5175), then opens Chrome/Edge with
// its own profile through a one-shot localhost redirect that sets the HttpOnly cookie.
//   node Open-Local-IDE.mjs --config launcher-config.json [--check]
import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
import { randomBytes } from 'node:crypto';
import { spawn } from 'node:child_process';
import { pathToFileURL, fileURLToPath } from 'node:url';

const args = process.argv.slice(2);
const checkOnly = args.includes('--check');
const configPath = args[args.indexOf('--config') + 1];
const launcherDir = path.dirname(fileURLToPath(import.meta.url));
const config = JSON.parse(fs.readFileSync(configPath, 'utf8').replace(/^﻿/, ''));
const main = path.join(config.workspaceRoot, 'PXLBLZ-IDE-main');
const ide = path.join(config.workspaceRoot, config.ideFolder);
const outputUrl = config.outputUrl || 'ws://127.0.0.1:9980/pixels';
const origin = 'http://localhost:5174';
const studioUrl = 'http://localhost:5175/PXLBLZ-IDE/studio?pxout=1&pxoutUrl=' + encodeURIComponent(outputUrl);

const same = (a, b) => path.resolve(a).toLowerCase() === path.resolve(b).toLowerCase();
async function identity(port) {
  const response = await fetch(`http://localhost:${port}/__identity`, { signal: AbortSignal.timeout(15000) });
  return response.json();
}

const { createSessionCookie } = await import(pathToFileURL(path.join(main, 'src/cloudflare/auth.ts')));
const { localSessionUser, readDevVarsFile } = await import(pathToFileURL(path.join(main, 'scripts/dev-runtime-auth.ts')));
const manifest = JSON.parse(fs.readFileSync(path.join(main, 'dev-runtime.json'), 'utf8'));
const vars = readDevVarsFile(path.join(main, '.dev.vars'));
if (!vars.SESSION_SECRET) throw new Error('Der lokale SESSION_SECRET fehlt in PXLBLZ-IDE-main/.dev.vars.');

const apiId = await identity(5174);
if (apiId.project !== 'pxlblz-ide' || !same(apiId.worktree, main)) throw new Error('Auf Port 5174 laeuft eine andere Installation.');
const ideId = await identity(5175);
if (ideId.project !== 'pxlblz-ide' || !same(ideId.worktree, ide)) throw new Error('Die IDE auf Port 5175 ist nicht die FastLED-Version (' + ideId.worktree + ').');

const user = localSessionUser({ developer: true }, { assignments: [] }, manifest);
const cookie = await createSessionCookie(user, vars.SESSION_SECRET, { secure: false });
for (const port of [5174, 5175]) {
  const me = await fetch(`http://localhost:${port}/api/me`, { headers: { Cookie: cookie.split(';')[0] }, signal: AbortSignal.timeout(20000) });
  const account = await me.json();
  if (!me.ok || !account.authenticated || account.user?.id !== user.userId) {
    throw new Error(`Die lokale Sitzung wird auf Port ${port} nicht angenommen.`);
  }
}

const noncePath = '/open/' + randomBytes(24).toString('hex');
let timer;
const server = http.createServer((req, res) => {
  if (req.method !== 'GET' || req.url !== noncePath || req.headers.host !== `localhost:${server.address().port}`) {
    res.writeHead(404); res.end(); return;
  }
  res.writeHead(303, {
    'Set-Cookie': cookie,
    Location: studioUrl,
    'Cache-Control': 'no-store',
    'Referrer-Policy': 'no-referrer',
    'Content-Length': '0',
  });
  res.end();
  clearTimeout(timer);
  server.close();
  console.log('Lokaler Zugang bestaetigt und Studio geoeffnet: ' + user.userId);
});
await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '::1', resolve); });
const bootstrapUrl = `http://localhost:${server.address().port}${noncePath}`;
timer = setTimeout(() => { server.close(); process.exitCode = 1; console.error('Das Browserfenster hat den lokalen Start nicht abgerufen.'); }, 90000);

if (checkOnly) {
  const result = await fetch(bootstrapUrl, { redirect: 'manual', signal: AbortSignal.timeout(10000) });
  const setCookie = result.headers.get('set-cookie') || '';
  if (result.status !== 303 || result.headers.get('location') !== studioUrl || !setCookie.includes('HttpOnly')) {
    throw new Error('Die lokale Anmeldeweiterleitung ist fehlgeschlagen.');
  }
  const verify = await fetch('http://localhost:5175/api/me', { headers: { Cookie: setCookie.split(';')[0] }, signal: AbortSignal.timeout(20000) });
  const verified = await verify.json();
  if (!verified.authenticated || verified.user?.id !== user.userId) throw new Error('Browser-Sitzungspruefung fehlgeschlagen.');
  console.log('CHECK OK: lokaler Login ' + user.userId + ' (kein Google/GitHub), HttpOnly-Cookie, API ueber 5174 und 5175.');
} else {
  const candidates = [
    path.join(process.env['ProgramFiles(x86)'] || '', 'Google/Chrome/Application/chrome.exe'),
    path.join(process.env.ProgramFiles || '', 'Google/Chrome/Application/chrome.exe'),
    path.join(process.env.LOCALAPPDATA || '', 'Google/Chrome/Application/chrome.exe'),
    path.join(process.env['ProgramFiles(x86)'] || '', 'Microsoft/Edge/Application/msedge.exe'),
  ];
  const browser = candidates.find(candidate => fs.existsSync(candidate));
  if (!browser) { clearTimeout(timer); server.close(); throw new Error('Chrome oder Edge wurde nicht gefunden.'); }
  const child = spawn(browser, [
    '--user-data-dir=' + path.join(launcherDir, 'BrowserProfile'),
    '--no-first-run', '--no-default-browser-check', '--new-window', bootstrapUrl,
  ], { detached: true, stdio: 'ignore', windowsHide: true });
  child.on('error', error => { clearTimeout(timer); server.close(); console.error(error.message); process.exitCode = 1; });
  child.unref();
}
