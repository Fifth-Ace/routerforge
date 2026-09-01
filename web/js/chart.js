import { esc } from './utils.js';

function niceMax(v){ if(v<=5)return Math.max(1,Math.ceil(v)); const p=10**Math.floor(Math.log10(v)); const n=v/p; const m=n<=1?1:n<=2?2:n<=5?5:10; return m*p; }
function smoothPath(pts){
  if(!pts.length)return '';
  if(pts.length===1)return `M ${pts[0][0]} ${pts[0][1]}`;
  let d=`M ${pts[0][0].toFixed(1)} ${pts[0][1].toFixed(1)}`;
  for(let i=0;i<pts.length-1;i++){
    const p0=pts[i-1]||pts[i], p1=pts[i], p2=pts[i+1], p3=pts[i+2]||p2;
    const c1x=p1[0]+(p2[0]-p0[0])/12, c1y=p1[1]+(p2[1]-p0[1])/12;
    const c2x=p2[0]-(p3[0]-p1[0])/12, c2y=p2[1]-(p3[1]-p1[1])/12;
    d+=` C ${c1x.toFixed(1)} ${c1y.toFixed(1)}, ${c2x.toFixed(1)} ${c2y.toFixed(1)}, ${p2[0].toFixed(1)} ${p2[1].toFixed(1)}`;
  }
  return d;
}

export function historyChart(history={}) {
  const points=history?.points||[];
  if (!points.length) return '<div class="empty">История пока пуста</div>';
  if (history.sufficient===false) {
    const pct=Math.round(Number(history.coverage||0)*100);
    return `<div class="empty chart-empty"><b>Недостаточно данных за выбранный период</b><span>Накоплено ${pct}% истории. График появится после 50% покрытия периода.</span></div>`;
  }
  const W=1000,H=250,pad={l:42,r:14,t:14,b:46};
  const plotBottom=H-pad.b-18;
  const maxReq=niceMax(Math.max(1,...points.map(p=>Number(p.requests||0))));
  const x=i=>pad.l+(W-pad.l-pad.r)*(points.length<=1?0:i/(points.length-1));
  const y=v=>plotBottom-(plotBottom-pad.t)*(Number(v||0)/maxReq);
  const reqPts=points.map((p,i)=>[x(i),y(p.requests)]);
  const line=smoothPath(reqPts);
  const area=reqPts.length>1?`${line} L ${reqPts.at(-1)[0].toFixed(1)} ${plotBottom} L ${reqPts[0][0].toFixed(1)} ${plotBottom} Z`:'';
  const ticks=[0,.25,.5,.75,1].map(f=>{ const yy=pad.t+(plotBottom-pad.t)*f,val=Math.round(maxReq*(1-f)); return `<line x1="${pad.l}" y1="${yy}" x2="${W-pad.r}" y2="${yy}" class="chart-grid"/><text x="2" y="${yy+3}" class="chart-label">${val}</text>`; }).join('');
  const labelIdx=[0,Math.floor((points.length-1)/2),points.length-1].filter((v,i,a)=>a.indexOf(v)===i);
  const labels=labelIdx.map(i=>`<text x="${x(i)}" y="${H-5}" text-anchor="${i===0?'start':i===points.length-1?'end':'middle'}" class="chart-label">${esc(new Date(points[i].time).toLocaleTimeString('ru-RU',{hour:'2-digit',minute:'2-digit'}))}</text>`).join('');
  const evBase=plotBottom+15;
  const events=points.map((p,i)=>{
    const xx=x(i); let o='';
    const add=(v,cls,offset)=>{ if(Number(v||0)>0){ const h=Math.min(14,3+Math.log2(Number(v)+1)*3); o+=`<line x1="${xx+offset}" y1="${evBase}" x2="${xx+offset}" y2="${evBase-h}" class="${cls}" stroke-width="2" vector-effect="non-scaling-stroke"/>`; }};
    add(p.fallbacks,'chart-event-fallback',-2); add(p.errors,'chart-event-error',0); add(p.timeouts,'chart-event-timeout',2); return o;
  }).join('');
  return `<svg class="chart-svg" viewBox="0 0 ${W} ${H}" preserveAspectRatio="none"><defs><linearGradient id="reqFill" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stop-color="var(--color-accent)" stop-opacity=".24"/><stop offset="100%" stop-color="var(--color-accent)" stop-opacity="0"/></linearGradient></defs>${ticks}${labels}${area?`<path d="${area}" fill="url(#reqFill)"/>`:''}<path d="${line}" fill="none" stroke="var(--color-accent)" stroke-width="2" vector-effect="non-scaling-stroke"/>${events}<line x1="${pad.l}" y1="${evBase}" x2="${W-pad.r}" y2="${evBase}" class="chart-grid"/></svg>`;
}
