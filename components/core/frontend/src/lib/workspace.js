// F15 Widget/Layout Contract v1. No API access, executable content or credentials.
export const STORAGE_KEY = 'routerforge:workspace:v1';
const metric = ['1x1','2x1','2x2','4x2'];
const shortcut = ['1x1','2x1'];
const panel = ['1x1','2x2','4x2','4x4'];
const w = (id, owner, ru, en, icon, sizes, size, type, route, source) => Object.freeze({id,owner,ru,en,icon,sizes:Object.freeze([...sizes]),size,type,route,source});
export const WIDGETS = Object.freeze([
 w('cpu','monitoring','Процессор','Processor','CPU',metric,'1x1','metric','/monitoring?tab=system','overview.cpu'),
 w('ram','monitoring','Память','Memory','RAM',metric,'1x1','metric','/monitoring?tab=system','overview.memory'),
 w('thermal','monitoring','Температура','Temperature','°C',metric,'1x1','metric','/monitoring?tab=thermal','overview.thermal'),
 w('storage','monitoring','Накопитель','Storage','USB',metric,'1x1','metric','/monitoring?tab=storage','overview.platform.opt'),
 w('route','network-tools','Маршрут IPv4','IPv4 route','WAN',metric,'2x1','metric','/network-tools','overview.networkRoutes'),
 w('dns','dns','DNS Manager','DNS Manager','DNS',metric,'2x1','metric','/dns','snapshot.upstreams'),
 w('core','core','Состояние Core','Core status','RF',metric,'2x1','metric','/apps','snapshot.backend'),
 w('updates','core','Обновления','Updates','↑',metric,'1x1','metric','/apps?tab=updates','catalog.updates'),
 w('traffic','monitoring','Трафик маршрута','Default route traffic','↕',metric,'4x2','metric','/monitoring?tab=network','overview.networkInterfaces'),
 w('interfaces','monitoring','Интерфейсы','Interfaces','ETH',metric,'2x2','metric','/network-tools','overview.networkInterfaces'),
 w('uptime','core','Время работы','Uptime','◷',metric,'2x1','metric','/monitoring?tab=system','overview.platform.uptime_seconds'),
 w('services','core','Службы','Services','▤',metric,'2x2','metric','/apps?tab=installed','catalog.modules'),
 w('dns-router-status','dns-router','Служба DNS Router','DNS Router service','DNS',metric,'2x1','metric','/routing','catalog.modules'),
 w('nfqws-status','nfqws-manager','Служба NFQWS','NFQWS service','NFQ',metric,'2x1','metric','/apps?tab=installed','catalog.modules'),
 w('attention','core','Предупреждения','Attention','!',panel,'4x2','panel','','home.issues'),
 w('modules','core','Модули и интеграции','Modules and integrations','APP',panel,'4x2','panel','','catalog.installed'),
 w('monitoring','monitoring','Мониторинг','Monitoring','▥',shortcut,'1x1','shortcut','/monitoring',''),
 w('network','network-tools','Сетевые инструменты','Network tools','↔',shortcut,'1x1','shortcut','/network-tools',''),
 w('manage','admin','Управление','Management','⚙',shortcut,'1x1','shortcut','/manage',''),
 w('routing','dns-router','DNS Router','DNS Router','⇄',shortcut,'1x1','shortcut','/routing',''),
 w('policies','core','Объекты политик','Policy objects','▦',shortcut,'1x1','shortcut','/policy-objects',''),
 w('apps','core','Приложения','Applications','▣',shortcut,'1x1','shortcut','/apps','')
]);
const registry = new Map(WIDGETS.map(x=>[x.id,x]));
export const getWidget = id => registry.get(id);
export const ACCENTS=Object.freeze(['auto','green','blue','violet','amber']);
export const COLS=4, MAX_ROWS=96;
export const dimensions=size=>size.split('x').map(Number);
const linkPattern=/^link-[a-z0-9]{1,40}$/;
export function safeExternal(raw){
 if(typeof raw!=='string'||raw.length>512||!/^https?:\/\//i.test(raw)||/[\s\\\u0000-\u001f\u007f]/.test(raw))throw Error('INVALID_LINK');
 const u=new URL(raw);if(!['http:','https:'].includes(u.protocol)||!u.hostname||u.username||u.password||u.search||u.hash||/[?#]/.test(raw))throw Error('INVALID_LINK');
 return u.href;
}
export function widgetFor(tile){
 if(!tile)return undefined;
 if(linkPattern.test(tile.id)&&typeof tile.url==='string')return w(tile.id,'core','Внешняя ссылка','External link','↗',shortcut,'1x1','external','','');
 return getWidget(tile.id);
}
export function overlaps(a,b){const [aw,ah]=dimensions(a.size),[bw,bh]=dimensions(b.size);return a.x<b.x+bw&&a.x+aw>b.x&&a.y<b.y+bh&&a.y+ah>b.y;}
export function validPosition(t){const [width,height]=dimensions(t.size);return Number.isInteger(t.x)&&Number.isInteger(t.y)&&t.x>=0&&t.y>=0&&t.x+width<=COLS&&t.y+height<=MAX_ROWS;}
function firstFree(tile,occupied){const [width,height]=dimensions(tile.size);for(let y=0;y<=MAX_ROWS-height;y++)for(let x=0;x<=COLS-width;x++){const p={...tile,x,y};if(!occupied.some(o=>overlaps(p,o)))return p;}throw Error('GRID_FULL');}
export function packTiles(tiles){const placed=[];for(const tile of tiles)placed.push(firstFree(tile,placed));return placed;}
export function spatialOrder(tiles){return tiles.slice().sort((a,b)=>a.y-b.y||a.x-b.x);}
export function placeTile(tiles,id,x,y,size){
 const old=tiles.find(t=>t.id===id);if(!old)return tiles.slice();const next={...old,size:size||old.size,x,y};
 if(!widgetFor(old).sizes.includes(next.size)||!validPosition(next))throw Error('INVALID_POSITION');
 const still=tiles.filter(t=>t.id!==id&&!overlaps(t,next));const displaced=tiles.filter(t=>t.id!==id&&overlaps(t,next));
 const placed=[...still,next];for(const t of spatialOrder(displaced))placed.push(firstFree(t,placed));
 const byId=new Map(placed.map(t=>[t.id,t]));return tiles.map(t=>byId.get(t.id));
}
export function fitSize(tiles,id,size){const t=tiles.find(x=>x.id===id);if(!t)return tiles.slice();const [width,height]=dimensions(size);return placeTile(tiles,id,Math.min(t.x,COLS-width),Math.min(t.y,MAX_ROWS-height),size);}
export function nearestSize(widget,width,height){return widget.sizes.reduce((best,size)=>{const score=s=>{const [x,y]=dimensions(s);return (x-width)**2+(y-height)**2;};return score(size)<score(best)?size:best;},widget.size);}
const makeTiles = ids => packTiles(ids.map(id=>({id,size:registry.get(id).size})));
const profiles = Object.freeze({default:['cpu','ram','thermal','storage','route','dns','core','updates','traffic','attention','network','manage','routing','apps','modules'],network:['route','dns','core','traffic','interfaces','attention','network','routing','policies','modules'],admin:['cpu','ram','thermal','storage','core','updates','attention','manage','monitoring','apps','modules']});
export const profileTiles = key => makeTiles(profiles[key] || profiles.default);
export function defaultLayout(){return {schemaVersion:1,active:'home',spaces:[{id:'home',name:'',profile:'default',tiles:profileTiles('default')}]};}
const validLabel=s=>typeof s==='string'&&s.length<=64&&!/[\u0000-\u001f\u007f]/.test(s);
export function normalizeLayout(value){
 if (!value || value.schemaVersion!==1 || !Array.isArray(value.spaces) || value.spaces.length<1 || value.spaces.length>8) throw Error('INVALID_LAYOUT');
 const seen=new Set();
 const spaces=value.spaces.map(s=>{
  if(!s||typeof s.id!=='string'||!/^[-a-zA-Z0-9_]{1,48}$/.test(s.id)||seen.has(s.id)||!validLabel(s.name)||!Array.isArray(s.tiles)||s.tiles.length>64)throw Error('INVALID_SPACE');
  seen.add(s.id);const ids=new Set(),tiles=[];let links=0;
  for(const tile of s.tiles){
   if(!tile||typeof tile.id!=='string'||typeof tile.size!=='string')throw Error('INVALID_TILE');
   let external=linkPattern.test(tile.id);const widget=external?widgetFor({...tile,url:safeExternal(tile.url)}):getWidget(tile.id);if(!widget)continue;
   if(ids.has(tile.id)||!widget.sizes.includes(tile.size)||external&&++links>12)throw Error('INVALID_TILE');ids.add(tile.id);
   const t={id:tile.id,size:tile.size};if(external)t.url=safeExternal(tile.url);
   if(tile.title!==undefined){if(!validLabel(tile.title))throw Error('INVALID_TITLE');if(tile.title.trim())t.title=tile.title.trim();}
   if(tile.accent!==undefined){if(!ACCENTS.includes(tile.accent))throw Error('INVALID_ACCENT');if(tile.accent!=='auto')t.accent=tile.accent;}
   if(tile.expandedWidth!==undefined){if(!canExpand(widget)||![1,2,4].includes(tile.expandedWidth))throw Error('INVALID_EXPANDED_WIDTH');t.expandedWidth=tile.expandedWidth;}
   if(tile.locked!==undefined){if(typeof tile.locked!=='boolean')throw Error('INVALID_LOCK');if(tile.locked)t.locked=true;}
   if(tile.anchors!==undefined){if(!tile.anchors||typeof tile.anchors!=='object'||Array.isArray(tile.anchors))throw Error('INVALID_ANCHORS');t.anchors={};for(const [key,a] of Object.entries(tile.anchors)){const c=Number(key);if(!['1','2','4','6','8'].includes(key)||!a||!Number.isInteger(a.x)||!Number.isInteger(a.y)||a.x<0||a.x>=c||a.y<0||a.y>1023)throw Error('INVALID_ANCHOR');t.anchors[key]={x:a.x,y:a.y};}}
   if(tile.expanded!==undefined){if(!canExpand(widget)||typeof tile.expanded!=='boolean')throw Error('INVALID_EXPANSION');if(tile.expanded)t.expanded=true;}
   if(tile.showDetail!==undefined){if(typeof tile.showDetail!=='boolean')throw Error('INVALID_DETAIL');if(!tile.showDetail)t.showDetail=false;}
   if(tile.x!==undefined||tile.y!==undefined){t.x=tile.x;t.y=tile.y;if(!validPosition(t))throw Error('INVALID_POSITION');}
   tiles.push(t);
  }
  const placed=tiles.filter(t=>t.x!==undefined);for(let i=0;i<placed.length;i++)if(placed.slice(0,i).some(t=>overlaps(t,placed[i])))throw Error('OVERLAPPING_TILES');
  const result=tiles.map(t=>{if(t.x!==undefined)return t;const p=firstFree(t,placed);placed.push(p);return p;});
  if(s.autoAlign!==undefined&&typeof s.autoAlign!=='boolean')throw Error('INVALID_ALIGNMENT');if(s.density!==undefined&&!['compact','normal','large'].includes(s.density))throw Error('INVALID_DENSITY');
  return {...(s.autoAlign===false?{autoAlign:false}:{}),...(s.density&&s.density!=='normal'?{density:s.density}:{}),id:s.id,name:s.name.trim(),profile:['default','network','admin'].includes(s.profile)?s.profile:'default',tiles:result};
 });
 if(!seen.has(value.active))throw Error('INVALID_ACTIVE');const result={schemaVersion:1,active:value.active,spaces};if(JSON.stringify(result).length>60000)throw Error('INVALID_LAYOUT_SIZE');return result;
}
export function parseLayout(raw){if(typeof raw!=='string'||raw.length>65536)throw Error('INVALID_LAYOUT_SIZE');return normalizeLayout(JSON.parse(raw));}
export function updateSpace(layout,id,fn){return normalizeLayout({...layout,spaces:layout.spaces.map(s=>s.id===id?fn(s):s)});}
export function reorder(tiles,from,to){const a=spatialOrder(tiles);const i=a.findIndex(t=>t.id===from),j=a.findIndex(t=>t.id===to);if(i<0||j<0||i===j)return a;const [t]=a.splice(i,1);a.splice(j,0,t);return packTiles(a);}
export function resize(tiles,id){const tile=tiles.find(t=>t.id===id);if(!tile)return tiles.slice();const widget=widgetFor(tile);return fitSize(tiles,id,widget.sizes[(widget.sizes.indexOf(tile.size)+1)%widget.sizes.length]);}
export function finiteMetric(n){return typeof n==='number'&&Number.isFinite(n)?n:null;}
export function boundedPercent(n){return finiteMetric(n)!==null&&n>=0&&n<=100?n:null;}
// Browser-session history only. The source identity changes on real overview samples,
// not every SSE redraw. Null samples create gaps instead of invented zero readings.
export function recordHistory(history,keys,metrics,sampleKeys,online,now=Date.now()){
 const next={...history},seen={...keys};for(const id of ['cpu','ram','thermal','traffic-rx','traffic-tx']){
  const key=sampleKeys?.[id];if(!online){if(seen[id]!==null){seen[id]=null;next[id]=[...(next[id]||[]),{time:now,value:null}].slice(-40);}continue;}
  if(!key){if(seen[id]){seen[id]=null;next[id]=[...(next[id]||[]),{time:now,value:null}].slice(-40);}continue;}if(key===seen[id])continue;if(id.startsWith('traffic-')&&typeof seen[id]==='string'&&typeof key==='string'&&seen[id].split('|')[0]!==key.split('|')[0])next[id]=[];seen[id]=key;const m=metrics[id];const v=id.startsWith('traffic-')?finiteMetric(metrics.traffic?.[id==='traffic-rx'?'rx':'tx']):id==='thermal'?finiteMetric(m?.numeric):boundedPercent(m?.percent);
  next[id]=[...(next[id]||[]),{time:now,value:v}].slice(-40);
 }return {history:next,keys:seen};
}
export function chartSegments(samples,id,sharedMax){
 if(!Array.isArray(samples)||samples.length<2)return [];const finite=samples.filter(s=>finiteMetric(s.value)!==null).map(s=>s.value);if(finite.length<2)return [];
 const lo=id==='thermal'?Math.min(0,...finite):0,hi=id.startsWith('traffic-')?Math.max(1,sharedMax||0,...finite):id==='thermal'?Math.max(100,...finite):100;const out=[];let current=[];
 samples.forEach((s,i)=>{if(finiteMetric(s.value)===null){if(current.length>1)out.push(current.join(' '));current=[];return;}current.push(`${(i/(samples.length-1)*200).toFixed(2)},${(38-(s.value-lo)/(hi-lo)*34).toFixed(2)}`);});if(current.length>1)out.push(current.join(' '));return out;
}

// Smart packing is a presentation layer over the original v1 storage contract.
// Persist only the canonical four-column grid; viewport projection never writes storage.
export function columnsForWidth(width){return width>=1800?8:width>=1250?6:width>=720?4:width>=360?2:1;}
export function compactTiles(tiles){return spatialOrder(packTiles(tiles.map(t=>({...t}))));}
export function compactReorder(tiles,id,target){return spatialOrder(reorder(tiles,id,target));}
export function compactLayout(value){const d=normalizeLayout(value);return normalizeLayout({...d,spaces:d.spaces.map(s=>({...s,tiles:s.autoAlign===false?s.tiles:compactTiles(spatialOrder(s.tiles))}))});}
export function canExpand(widget){return !!widget&&(widget.type==='panel'||['cpu','ram','thermal','storage','traffic','dns','updates','interfaces','services','route','core'].includes(widget.id));}
export function projectTiles(tiles,cols,panelRows=null){
 if(![1,2,4,6,8].includes(cols))throw Error('INVALID_COLUMNS');
 const out=[],used=new Set();
 for(const t of spatialOrder(tiles)){
  const [width,height]=dimensions(t.size),w=Math.min(t.expanded&&t.expandedWidth?t.expandedWidth:width,cols),kind=widgetFor(t).type;
  const h=panelRows===null?(cols===1?Math.min(height,2):height):kind==='panel'?Math.max(1,Math.min(96,panelRows[t.id]||2)):kind==='shortcut'||kind==='external'?1:t.expanded?Math.max(2,Math.min(96,panelRows[t.id]||4)):2*(cols===1?Math.min(height,2):height);
  let found=null;
  for(let y=0;y<1024&&!found;y++)for(let x=0;x<=cols-w;x++){
   let free=true;for(let dy=0;dy<h;dy++)for(let dx=0;dx<w;dx++)if(used.has((y+dy)*cols+x+dx))free=false;
   if(!free)continue;found={...t,x,y,w,h};for(let dy=0;dy<h;dy++)for(let dx=0;dx<w;dx++)used.add((y+dy)*cols+x+dx);break;
  }
  if(!found)throw Error('PROJECT_GRID_FULL');out.push(found);
 }
 return out.sort((a,b)=>a.y-b.y||a.x-b.x);
}
export function compactResize(tiles,id,size){
 const t=tiles.find(t=>t.id===id);if(!t)return tiles.slice();if(!widgetFor(t).sizes.includes(size))throw Error('INVALID_SIZE');
 return compactTiles(spatialOrder(tiles).map(t=>t.id===id?{...t,size}:t));
}
export function moveToCell(tiles,visual,id,x,y){
 const from=visual.find(t=>t.id===id);if(!from)return tiles.slice();
 const others=visual.filter(t=>t.id!==id);
 // Prefer an actual hit; otherwise choose the nearest occupied cell. A drag into
 // trailing space goes to the last position, never creates a permanent hole.
 let target=others.find(t=>x>=t.x&&x<t.x+t.w&&y>=t.y&&y<t.y+t.h);
 if(!target&&x===from.x&&y===from.y)return tiles.slice();
 if(!target&&others.length){let score=Infinity;for(const t of others){const dx=Math.max(t.x-x,0,x-(t.x+t.w-1)),dy=Math.max(t.y-y,0,y-(t.y+t.h-1));const s=dy*16+dx;if(s<score){score=s;target=t;}}}
 return target?compactReorder(tiles,id,target.id):tiles.slice();
}
export function metricStats(samples){const values=(samples||[]).map(s=>s.value).filter(v=>finiteMetric(v)!==null);return values.length?{mean:values.reduce((a,b)=>a+b,0)/values.length,peak:Math.max(...values),count:values.length}:null;}
export function widgetCategory(w){if(w.type==='shortcut'||w.type==='external')return 'shortcuts';if(w.id==='attention')return 'events';if(w.id==='dns'||w.id==='dns-router-status')return 'dns';if(['traffic','route','interfaces'].includes(w.id))return 'network';if(['cpu','ram','thermal','storage','uptime'].includes(w.id))return 'system';return 'management';}

// Visual tracks are half-height; the persisted v1 coordinates and sizes remain unchanged.
export function panelTrackCount(height,row=56,gap=9){return Number.isFinite(height)&&height>0&&row>0&&gap>=0?Math.max(1,Math.min(96,Math.ceil((height+2+gap)/(row+gap)))):2;}
export function chartDomain(series,id){
 const all=series.flat().filter(s=>Number.isFinite(s?.time));const vals=all.map(s=>s.value).filter(v=>Number.isFinite(v));
 const start=all.length?Math.min(...all.map(s=>s.time)):0,end=all.length?Math.max(...all.map(s=>s.time)):start;
 let lo=0,hi=100;
 if(id==='traffic'){const max=Math.max(0,...vals);const base=max>0?10**Math.floor(Math.log10(max)):1;hi=([1,2,5,10].find(n=>n*base>=max)||10)*base;}
 if(id==='thermal'){lo=Math.min(0,...vals);hi=Math.max(100,...vals);}
 return {start,end:Math.max(start+1,end),lo,hi};
}
export function chartGeometry(samples,domain){
 const parts=[];let points=[];const flush=()=>{if(points.length){const line=points.map(p=>`${p.x.toFixed(2)},${p.y.toFixed(2)}`).join(' ');parts.push({line,area:points.length>1?`M${points[0].x.toFixed(2)},60 L${line.replaceAll(' ',' L')} L${points.at(-1).x.toFixed(2)},60 Z`:'',points});points=[];}};
 for(const s of samples||[]){if(!Number.isFinite(s?.time)||!Number.isFinite(s?.value)){flush();continue;}const x=Math.max(0,Math.min(240,(s.time-domain.start)/(domain.end-domain.start)*240)),y=Math.max(2,Math.min(58,58-(s.value-domain.lo)/(domain.hi-domain.lo)*56));points.push({x,y,time:s.time,value:s.value});}flush();return parts;
}
export function nearestChartPoint(parts,x){let best=null;for(const part of parts)for(const p of part.points)if(!best||Math.abs(p.x-x)<Math.abs(best.x-x))best=p;return best;}

// Free anchors are visual positions per breakpoint; canonical v1 coordinates remain intact.
export function freeProject(tiles,cols,rows={}){
 const collapsed=projectTiles(tiles.map(t=>({...t,expanded:false})),cols,{}),live=projectTiles(tiles,cols,rows);
 const wanted=live.map(t=>{const base=collapsed.find(x=>x.id===t.id),a=t.anchors?.[cols]||base;return {...t,x:Math.min(a.x,cols-t.w),y:a.y};});
 const out=[];const intersects=(a,b)=>a.x<b.x+b.w&&a.x+a.w>b.x&&a.y<b.y+b.h&&a.y+a.h>b.y;
 for(const t of spatialOrder(wanted)){let y=t.y;while(out.some(o=>intersects({...t,y},o)))y=Math.max(y+1,...out.filter(o=>intersects({...t,y},o)).map(o=>o.y+o.h));out.push({...t,y});}
 return spatialOrder(out);
}
export function visualLayout(tiles,cols,rows={},auto=true){return auto===false?freeProject(tiles,cols,rows):projectTiles(tiles,cols,rows);}
export function seedAnchors(tiles,cols,keep=false){const view=projectTiles(tiles.map(t=>({...t,expanded:false})),cols,{});return tiles.map(t=>{const p=view.find(x=>x.id===t.id);return keep&&t.anchors?.[cols]?t:{...t,anchors:{...t.anchors,[cols]:{x:p.x,y:p.y}}};});}
export function moveFree(tiles,cols,id,x,y){
 const target=tiles.find(t=>t.id===id);if(!target||target.locked)return tiles.slice();
 const base=freeProject(tiles.map(t=>({...t,expanded:false})),cols,{}),old=base.find(t=>t.id===id);
 const moved={...old,x:Math.max(0,Math.min(cols-old.w,x)),y:Math.max(0,Math.min(1023,y))};
 const hit=(a,b)=>a.x<b.x+b.w&&a.x+a.w>b.x&&a.y<b.y+b.h&&a.y+a.h>b.y;
 const locked=base.filter(t=>t.id!==id&&t.locked);if(locked.some(t=>hit(t,moved)))return tiles.slice();
 const placed=[...locked,moved];for(const t of base.filter(t=>t.id!==id&&!t.locked)){let y=t.y;while(placed.some(o=>hit({...t,y},o)))y=Math.max(y+1,...placed.filter(o=>hit({...t,y},o)).map(o=>o.y+o.h));if(y>1023)return tiles.slice();placed.push({...t,y});}
 return tiles.map(t=>{const a=placed.find(p=>p.id===t.id);return {...t,anchors:{...t.anchors,[cols]:{x:a.x,y:a.y}}};});
}

// Width choices are independent of the collapsed size/height. Never use nearestSize for an expanded card.
export function resizeWidth(width,cols){
 const choices=[1,2,4].filter(w=>w<=cols);return choices.reduce((best,w)=>Math.abs(w-width)<Math.abs(best-width)?w:best,choices[0]);
}
export function resizeChoice(tile,dx,dy,cols){
 const widget=widgetFor(tile);if(tile.expanded&&canExpand(widget))return resizeWidth(tile.w+dx,cols);
 if(widget.type==='panel'){const w=resizeWidth(tile.w+dx,cols);return w===1?'1x1':w===2?'2x2':'4x2';}
 const [,h]=dimensions(tile.size),targetW=resizeWidth(tile.w+dx,cols),targetH=Math.max(1,h+Math.round(dy/2));
 // First choose a supported width, then the nearest height within that width.
 const sizes=widget.sizes.filter(s=>dimensions(s)[0]<=cols);const choices=sizes.length?sizes:widget.sizes;
 const score=s=>{const [w,r]=dimensions(s);return Math.abs(w-targetW)*100+Math.abs(r-targetH);};
 return choices.reduce((best,s)=>score(s)<score(best)?s:best,choices[0]);
}

// A real time series needs at least two distinct finite observation times.
export function chartReady(series){return Array.isArray(series)&&series.some(samples=>new Set((samples||[]).filter(s=>Number.isFinite(s?.time)&&Number.isFinite(s?.value)).map(s=>s.time)).size>=2);}
// The overview has two core rows; the module retains the full list.
export function corePreview(rows){const all=(Array.isArray(rows)?rows:[]).map(row=>{const parts=String(row).split(' · '),tail=parts.pop();const percent=typeof tail==='string'&&/^\d+(?:\.\d+)?%$/.test(tail)?boundedPercent(Number(tail.slice(0,-1))):null;return percent===null?null:{label:parts.join(' · '),percent};}).filter(Boolean);return {rows:all.slice(0,2),more:Math.max(0,all.length-2)};}
