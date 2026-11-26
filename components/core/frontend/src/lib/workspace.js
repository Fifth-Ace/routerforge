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
const makeTiles = ids => ids.map(id=>({id,size:registry.get(id).size}));
const profiles = Object.freeze({default:['cpu','ram','thermal','storage','route','dns','core','updates','attention','network','manage','routing','apps','modules'],network:['route','dns','core','attention','network','routing','policies','modules'],admin:['cpu','ram','thermal','storage','core','updates','attention','manage','monitoring','apps','modules']});
export const profileTiles = key => makeTiles(profiles[key] || profiles.default);
export function defaultLayout(){return {schemaVersion:1,active:'home',spaces:[{id:'home',name:'',profile:'default',tiles:profileTiles('default')}]};}
export function normalizeLayout(value){
 if (!value || value.schemaVersion!==1 || !Array.isArray(value.spaces) || value.spaces.length<1 || value.spaces.length>8) throw Error('INVALID_LAYOUT');
 const seen=new Set();
 const spaces=value.spaces.map(s=>{
  if(!s||typeof s.id!=='string'||!/^[-a-zA-Z0-9_]{1,48}$/.test(s.id)||seen.has(s.id)||typeof s.name!=='string'||s.name.length>64||/[\u0000-\u001f\u007f]/.test(s.name)||!Array.isArray(s.tiles)||s.tiles.length>64)throw Error('INVALID_SPACE');
  seen.add(s.id);const ids=new Set();const tiles=[];
  for(const tile of s.tiles){if(!tile||typeof tile.id!=='string'||typeof tile.size!=='string')throw Error('INVALID_TILE');const widget=getWidget(tile.id);if(!widget)continue;if(ids.has(tile.id)||!widget.sizes.includes(tile.size))throw Error('INVALID_TILE');ids.add(tile.id);tiles.push({id:tile.id,size:tile.size});}
  return {id:s.id,name:s.name.trim(),profile:['default','network','admin'].includes(s.profile)?s.profile:'default',tiles};
 });
 if(!seen.has(value.active))throw Error('INVALID_ACTIVE');return {schemaVersion:1,active:value.active,spaces};
}
export function parseLayout(raw){if(typeof raw!=='string'||raw.length>65536)throw Error('INVALID_LAYOUT_SIZE');return normalizeLayout(JSON.parse(raw));}
export function updateSpace(layout,id,fn){return normalizeLayout({...layout,spaces:layout.spaces.map(s=>s.id===id?fn(s):s)});}
export function reorder(tiles,from,to){const a=tiles.slice();const i=a.findIndex(t=>t.id===from),j=a.findIndex(t=>t.id===to);if(i<0||j<0||i===j)return a;const [t]=a.splice(i,1);a.splice(j,0,t);return a;}
export function resize(tiles,id){const widget=getWidget(id);return tiles.map(t=>t.id===id?{...t,size:widget.sizes[(widget.sizes.indexOf(t.size)+1)%widget.sizes.length]}:t);}
export function finiteMetric(n){return typeof n==='number'&&Number.isFinite(n)?n:null;}
export function boundedPercent(n){return finiteMetric(n)!==null&&n>=0&&n<=100?n:null;}
