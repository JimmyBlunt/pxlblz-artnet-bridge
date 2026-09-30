// Reopen the existing browser profile through Chrome/Edge itself. A short-lived
// helper tab closes itself; it never loads the output page or another sender.
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import {randomBytes} from 'node:crypto';
import {spawn} from 'node:child_process';
const directory=path.dirname(fileURLToPath(import.meta.url));
const candidates=[
 path.join(process.env['ProgramFiles(x86)']||'', 'Google/Chrome/Application/chrome.exe'),
 path.join(process.env.ProgramFiles||'', 'Google/Chrome/Application/chrome.exe'),
 path.join(process.env.LOCALAPPDATA||'', 'Google/Chrome/Application/chrome.exe'),
 path.join(process.env['ProgramFiles(x86)']||'', 'Microsoft/Edge/Application/msedge.exe'),
];
const browser=candidates.find(candidate=>fs.existsSync(candidate));
if(!browser) throw new Error('Chrome/Edge wurde nicht gefunden.');
const route='/restore/'+randomBytes(24).toString('hex');
let timer;
const server=http.createServer((req,res)=>{
 if(req.method!=='GET'||req.url!==route||req.headers.host!==`localhost:${server.address().port}`){res.writeHead(404);res.end();return;}
 res.writeHead(200,{'Content-Type':'text/html; charset=utf-8','Cache-Control':'no-store','Referrer-Policy':'no-referrer'});
 res.end('<!doctype html><meta charset="utf-8"><title>PXLBLZ-IDE ist bereits gestartet</title><h1>PXLBLZ-IDE l&auml;uft bereits</h1><p>Dieser Hilfstab schlie&szlig;t sich automatisch. Falls der Browser das verhindert, schlie&szlig;e diesen Tab, um zum vorhandenen IDE-Tab zur&uuml;ckzukehren.</p><script>window.close()</script>');
 clearTimeout(timer);server.close();console.log('Browserprofil ueber den normalen Browserstart geoeffnet; kein zweiter Ausgabesender.');
});
await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'::1',resolve)});
timer=setTimeout(()=>{server.close();console.error('Browser hat den Wiederherstellungstab nicht geoeffnet.');process.exitCode=1;},30000);
const child=spawn(browser,['--user-data-dir='+path.join(directory,'BrowserProfile'),'--profile-directory=Default','--no-first-run',`http://localhost:${server.address().port}${route}`],{detached:true,stdio:'ignore',windowsHide:false});
child.on('error',error=>{clearTimeout(timer);server.close();console.error(error.message);process.exitCode=1;});child.unref();
