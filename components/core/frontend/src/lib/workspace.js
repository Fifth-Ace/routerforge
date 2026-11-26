// F15 Widget/Layout Contract v1. No API access, executable content or credentials.
export const STORAGE_KEY = 'routerforge:workspace:v1';
const metric = ['1x1','2x1','2x2','4x2'];
const shortcut = ['1x1','2x1'];
const panel = ['2x2','4x2','4x4'];
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
 w('modules','core','Модули и интеграции','Modules and integrations','APP',panel,'4x4','panel','','catalog.installed'),
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
   if(tile.showDetail!==undefined){if(typeof tile.showDetail!=='boolean')throw Error('INVALID_DETAIL');if(!tile.showDetail)t.showDetail=false;}
   if(tile.x!==undefined||tile.y!==undefined){t.x=tile.x;t.y=tile.y;if(!validPosition(t))throw Error('INVALID_POSITION');}
   tiles.push(t);
  }
  const placed=tiles.filter(t=>t.x!==undefined);for(let i=0;i<placed.length;i++)if(placed.slice(0,i).some(t=>overlaps(t,placed[i])))throw Error('OVERLAPPING_TILES');
  const result=tiles.map(t=>{if(t.x!==undefined)return t;const p=firstFree(t,placed);placed.push(p);return p;});
  return {id:s.id,name:s.name.trim(),profile:['default','network','admin'].includes(s.profile)?s.profile:'default',tiles:result};
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
