import fs from 'node:fs';
import path from 'node:path';
import http from 'node:http';
import { randomBytes } from 'node:crypto';
import { spawn } from 'node:child_process';
import { pathToFileURL, fileURLToPath } from 'node:url';

const project = process.argv[2];
const checkOnly = process.argv.includes('--check');
const launcherDir = path.dirname(fileURLToPath(import.meta.url));
const origin = 'http://localhost:5174';
const studioUrl = 'http://localhost:5175/PXLBLZ-IDE/studio/patterns/GyroidGlow3D?pxout=1&pxoutUrl=ws%3A%2F%2F127.0.0.1%3A9981%2Fpixels';
const { createSessionCookie } = await import(pathToFileURL(path.join(project, 'src/cloudflare/auth.ts')));
const { localSessionUser, readDevVarsFile } = await import(pathToFileURL(path.join(project, 'scripts/dev-runtime-auth.ts')));
const manifest = JSON.parse(fs.readFileSync(path.join(project, 'dev-runtime.json'), 'utf8'));
const vars = readDevVarsFile(path.join(project, '.dev.vars'));
if (!vars.SESSION_SECRET) throw new Error('Der lokale SESSION_SECRET fehlt.');
const identityResponse = await fetch(origin + '/__identity', {signal: AbortSignal.timeout(15000)});
const identity = await identityResponse.json();
if (identity.project !== 'pxlblz-ide' || path.resolve(identity.worktree).toLowerCase() !== path.resolve(project).toLowerCase()) {
  throw new Error('Auf Port 5174 laeuft eine andere Installation.');
}
const user = localSessionUser({developer: true}, {assignments: []}, manifest);
const cookie = await createSessionCookie(user, vars.SESSION_SECRET, {secure: false});
const me = await fetch(origin + '/api/me', {
  headers: {Cookie: cookie.split(';')[0]}, signal: AbortSignal.timeout(20000),
});
const account = await me.json();
if (!me.ok || !account.authenticated || account.user?.id !== user.userId) {
  throw new Error('Die lokale Sitzung konnte nicht bestaetigt werden.');
}
const outputIdentity = await (await fetch('http://localhost:5175/__identity', {signal: AbortSignal.timeout(15000)})).json();
const expectedOutput = path.join(path.dirname(project), 'PXLBLZ-IDE');
if (outputIdentity.project !== 'pxlblz-ide' || path.resolve(outputIdentity.worktree).toLowerCase() !== path.resolve(expectedOutput).toLowerCase()) {
  throw new Error('Die IDE auf Port 5175 stammt nicht aus der Fadecandy-Version.');
}
const outputMe = await fetch('http://localhost:5175/api/me', {
  headers: {Cookie: cookie.split(';')[0]}, signal: AbortSignal.timeout(20000),
});
const outputAccount = await outputMe.json();
if (!outputMe.ok || !outputAccount.authenticated || outputAccount.user?.id !== user.userId) {
  throw new Error('Die Fadecandy-IDE erreicht den lokalen Login nicht.');
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
await new Promise((resolve, reject) => {
  server.once('error', reject);
  server.listen(0, '::1', resolve);
});
const bootstrapUrl = `http://localhost:${server.address().port}${noncePath}`;
timer = setTimeout(() => { server.close(); process.exitCode = 1; console.error('Das Browserfenster hat den lokalen Start nicht abgerufen.'); }, 90000);
if (checkOnly) {
  const result = await fetch(bootstrapUrl, {redirect: 'manual', signal: AbortSignal.timeout(10000)});
  if (result.status !== 303 || result.headers.get('location') !== studioUrl || !result.headers.get('set-cookie')?.includes('HttpOnly')) {
    throw new Error('Die lokale Anmeldeweiterleitung ist fehlgeschlagen.');
  }
  const verify = await fetch(origin + '/api/me', {
    headers: {Cookie: result.headers.get('set-cookie').split(';')[0]}, signal: AbortSignal.timeout(20000),
  });
  const verified = await verify.json();
  if (!verified.authenticated || verified.user?.id !== user.userId) throw new Error('Browser-Sitzungspruefung fehlgeschlagen.');
  console.log('CHECK OK: Weiterleitung, HttpOnly-Cookie und angemeldeter API-Zugriff.');
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
  ], {detached: true, stdio: 'ignore', windowsHide: true});
  child.on('error', error => { clearTimeout(timer); server.close(); console.error(error.message); process.exitCode = 1; });
  child.unref();
}
