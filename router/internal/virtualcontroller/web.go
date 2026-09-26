package virtualcontroller

import (
  "encoding/json"
  "fmt"
  "net/http"
  "time"
)

func (c *Controller) Handler() http.Handler {
  mux := http.NewServeMux()
  mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Cache-Control", "no-store")
    _ = json.NewEncoder(w).Encode(c.Snapshot(time.Now()))
  })
  mux.HandleFunc("/api/frame", func(w http.ResponseWriter, r *http.Request) {
    frame, err := c.FrameRGB(r.URL.Query().Get("stage"))
    if err != nil {
      http.Error(w, err.Error(), http.StatusBadRequest)
      return
    }
    w.Header().Set("Content-Type", "application/octet-stream")
    w.Header().Set("Cache-Control", "no-store")
    _, _ = w.Write(frame)
  })
  mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/" { http.NotFound(w, r); return }
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    _, _ = fmt.Fprint(w, visualizerHTML)
  })
  return mux
}

const visualizerHTML = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>PXLBLZ Virtual Art-Net Controller</title>
<style>
:root{color-scheme:dark;font-family:ui-monospace,SFMono-Regular,Consolas,monospace;background:#09090b;color:#e4e4e7}
*{box-sizing:border-box}body{margin:0;padding:16px;background:#09090b}h1{font-size:18px;margin:0 0 4px}.sub{color:#a1a1aa;font-size:12px;margin-bottom:14px}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(155px,1fr));gap:8px;margin-bottom:12px}.card{border:1px solid #27272a;background:#111113;padding:8px;border-radius:6px}.k{font-size:10px;color:#71717a;text-transform:uppercase}.v{font-size:18px;margin-top:2px}.ok{color:#4ade80}.warn{color:#fbbf24}.bad{color:#fb7185}
#ports{width:100%;border:1px solid #27272a;background:#050506;border-radius:6px;display:block}.section{margin-top:12px}.row{display:flex;gap:14px;flex-wrap:wrap;font-size:11px;color:#a1a1aa}.row b{color:#e4e4e7}.bar{height:7px;background:#27272a;border-radius:9px;overflow:hidden;margin-top:4px}.bar>i{display:block;height:100%;background:#4ade80;width:0%}
table{border-collapse:collapse;width:100%;font-size:11px;margin-top:8px}th,td{border-bottom:1px solid #27272a;padding:5px;text-align:left}th{color:#71717a}.footer{font-size:10px;color:#52525b;margin-top:12px}.universes{display:flex;gap:5px;flex-wrap:wrap;margin-top:8px}.u{font-size:10px;border:1px solid #27272a;border-radius:4px;padding:4px 6px;background:#111113;color:#71717a}.u.seen{color:#4ade80;border-color:#14532d}.u.missing{color:#fbbf24;border-color:#713f12}.u.recent{box-shadow:0 0 0 1px #16a34a inset}
</style>
</head>
<body>
<h1>PXLBLZ Virtual Art-Net Controller</h1>
<div class="sub" id="identity">receiver emulator</div>
<div class="grid">
<div class="card"><div class="k">state</div><div class="v" id="state">-</div></div>
<div class="card"><div class="k">packets / s</div><div class="v" id="pps">0</div></div>
<div class="card"><div class="k">complete / s</div><div class="v" id="cps">0</div></div>
<div class="card"><div class="k">DMA / s</div><div class="v" id="dps">0</div></div>
<div class="card"><div class="k">candidate</div><div class="v" id="candidate">0 / 0</div><div class="bar"><i id="candidateBar"></i></div></div>
<div class="card"><div class="k">sequence</div><div class="v" id="sequence">-</div></div>
<div class="card"><div class="k">assembly p99</div><div class="v" id="assembly">0 us</div></div>
<div class="card"><div class="k">ingest p99</div><div class="v" id="ingest">0 us</div></div>
</div>
<div class="row" id="counters"></div>
<div class="section"><canvas id="ports" width="1400" height="300"></canvas></div>
<div class="section"><table><thead><tr><th>Port</th><th>Pixels</th><th>Universes</th><th>Order</th><th>Level</th></tr></thead><tbody id="routeRows"></tbody></table></div>
<div class="footer">Display shows the simulated physical output buffer after complete-frame assembly and run-policy/DMA scheduling. Updates are polled locally; no external assets are used.</div>
<script>
let last=null,lastAt=performance.now(),status=null,frame=null;
const $=id=>document.getElementById(id);
function n(v,d=1){return Number(v||0).toFixed(d)}
async function getStatus(){
  const s=await fetch('/api/status',{cache:'no-store'}).then(r=>r.json());
  const now=performance.now(); const dt=Math.max(.001,(now-lastAt)/1000);
  let pps=0,cps=0,dps=0;
  if(last){pps=(s.counters.packets-last.counters.packets)/dt;cps=(s.counters.complete-last.counters.complete)/dt;dps=(s.counters.dma_completed-last.counters.dma_completed)/dt}
  $('identity').textContent='target '+s.target_ip+' · '+s.expected_universes+' expected universes · '+s.output_fps+' FPS · wire guard '+s.wire_guard_us+' us';
  $('state').textContent=s.state; $('state').className='v '+(s.state==='running-artnet'?'ok':s.state==='blackout'?'bad':'warn');
  $('pps').textContent=n(pps); $('cps').textContent=n(cps); $('dps').textContent=n(dps);
  $('candidate').textContent=s.candidate_received+' / '+s.expected_universes;
  $('candidateBar').style.width=((s.expected_universes?s.candidate_received/s.expected_universes:0)*100)+'%';
  $('sequence').textContent=s.sequence_mode+' / '+s.sequence+(s.sequence_sealed?' sealed':'');
  $('assembly').textContent=n(s.timing.assembly_p99_us,1)+' us'; $('ingest').textContent=n(s.timing.ingest_p99_us,1)+' us';
  const c=s.counters; $('counters').innerHTML=['accepted','rejected','ignored','stale','duplicates','complete','incomplete','complete_replaced','frames_submitted','dma_completed','blackouts'].map(k=>'<span><b>'+k+'</b> '+c[k]+'</span>').join('');
  $('universes').innerHTML=(s.universes||[]).map(u=>{const recent=u.last_seen_age_ms>=0&&u.last_seen_age_ms<1000;const cls=u.received?'seen':(recent?'seen recent':'missing');return '<span class="u '+cls+'" title="data '+u.data_bytes+' bytes · min wire '+u.min_payload+' · packets '+u.packets+' · last '+n(u.last_seen_age_ms,1)+' ms">U'+u.universe+(u.received?' ✓':'')+'</span>'}).join('');
  if(!status || JSON.stringify(status.routes)!==JSON.stringify(s.routes)){
    $('routeRows').innerHTML=s.routes.map(r=>'<tr><td>P'+r.physical_port+' '+r.name+'</td><td>'+r.pixel_count+'</td><td>U'+r.universe_start+'..U'+r.universe_end+'</td><td>'+r.color_order+'</td><td>'+n(r.brightness,2)+'</td></tr>').join('');
    $('ports').height=Math.max(90,s.routes.length*42+24);
  }
  last=s;lastAt=now;status=s;
}
async function getFrame(){
  if(!status)return; const stage=$('stage').value; const b=await fetch('/api/frame?stage='+encodeURIComponent(stage),{cache:'no-store'}).then(r=>r.arrayBuffer());frame=new Uint8Array(b);draw();
}
function draw(){
  if(!status||!frame)return;const c=$('ports'),x=c.getContext('2d');x.clearRect(0,0,c.width,c.height);
  x.font='12px ui-monospace,monospace';
  status.routes.forEach((r,ri)=>{const y=18+ri*42;x.fillStyle='#a1a1aa';x.fillText('P'+r.physical_port+' '+r.name+'  '+r.pixel_count+'px  U'+r.universe_start+'..U'+r.universe_end,8,y);
    const left=8,top=y+7,w=c.width-16,h=15,step=Math.max(1,Math.ceil(r.pixel_count/w));
    for(let px=0;px<r.pixel_count;px+=step){let rr=0,gg=0,bb=0,count=0;for(let j=0;j<step&&px+j<r.pixel_count;j++){const o=r.frame_offset+(px+j)*3;rr+=frame[o]||0;gg+=frame[o+1]||0;bb+=frame[o+2]||0;count++}rr=Math.round(rr/count);gg=Math.round(gg/count);bb=Math.round(bb/count);x.fillStyle='rgb('+rr+','+gg+','+bb+')';const xx=left+(px/r.pixel_count)*w;const ww=Math.max(1,(step/r.pixel_count)*w+1);x.fillRect(xx,top,ww,h)}
  });
}
async function loopStatus(){try{await getStatus()}catch(e){}setTimeout(loopStatus,250)}
async function loopFrame(){try{await getFrame()}catch(e){}setTimeout(loopFrame,100)}
loopStatus();loopFrame();
</script>
</body>
</html>`
