import path from 'node:path';
import {pathToFileURL} from 'node:url';
const {default:WebSocket}=await import(pathToFileURL(path.join(process.argv[2], 'node_modules/ws/wrapper.mjs')));
const ws=new WebSocket('ws://127.0.0.1:7890/');
const timer=setTimeout(()=>{console.error('Fadecandy antwortet nicht.');ws.terminate();process.exitCode=1;},6000);
ws.on('open',()=>ws.send(JSON.stringify({type:'list_connected_devices'})));
ws.on('message',raw=>{
 const data=JSON.parse(raw.toString());
 if(data.type!=='list_connected_devices') return;
 clearTimeout(timer);
 const devices=data.devices?.filter(device=>device.type==='fadecandy') || [];
 if(!devices.length) {console.error('Kein Fadecandy-USB-Geraet angeschlossen.');process.exitCode=1;}
 else console.log('Fadecandy USB: '+devices.map(device=>device.serial+' / Firmware '+device.version).join(', '));
 ws.close();
});
ws.on('error',error=>{clearTimeout(timer);console.error(error.message);process.exitCode=1;});
