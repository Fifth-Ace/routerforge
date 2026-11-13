// Pure read-only status projection: do not treat stale or incomplete API values as READY.
export function dpiBytes(input){
  const n=Number(input);if(!Number.isSafeInteger(n)||n<=0)return '—';
  if(n<1048576)return `${Math.round(n/1024)} KB`;
  if(n<1073741824)return `${(n/1048576).toFixed(1)} MB`;
  return `${(n/1073741824).toFixed(1)} GB`;
}
const value=(input,fallback)=>typeof input==='string'&&input.trim()?input:fallback;
export function dpiStatusProjection(snapshot,loading=false,error=''){
  const invalid=snapshot!=null&&(typeof snapshot!=='object'||Array.isArray(snapshot));
  const d=invalid?null:snapshot;
  const failed=!!error||invalid;
  const installed=d?.installed===true;
  const ready=!loading&&!failed&&installed&&d?.execution_ready===true;
  const state=loading?'LOADING':failed?'ERROR':!d?'IDLE':!installed?'NOT INSTALLED':ready?'READY':'INCOMPATIBLE';
  const tab=loading?'':installed?(ready?'READY':'CHECK'):'';
  const items=loading?[]:failed?[{kind:'bad',title:'Status',detail:value(error,'Некорректный ответ DPI Detector API.')}]:!d?[]:installed?[
    {kind:ready?'good':'warn',title:'Installed',detail:`${value(d.path,'path unknown')} · ${dpiBytes(d.size_bytes)}`},
    {kind:ready?'good':'warn',title:'Execution',detail:ready?'ready':value(d.compatibility_reason,value(d.warning,'not ready'))},
    {kind:'info',title:'Upstream',detail:`${value(d.upstream,'Runnin4ik/dpi-detector')} · arch ${value(d.host_arch,'unknown')}`},
    {kind:'info',title:'Required interpreter',detail:value(d.required_interpreter,'none / static')}
  ]:[{kind:'info',title:'Optional module',detail:'DPI Detector v5 не установлен через App Center.'}];
  const summary=loading?[{kind:'',text:'Загрузка состояния…'}]:failed?[{kind:'warn',text:'Не удалось проверить DPI Detector'}]:!d?[{kind:'',text:'Состояние не загружено'}]:installed?[
    {kind:ready?'good':'warn',text:ready?'● READY':'● CHECK'},
    {kind:'',text:value(d.version,value(d.warning,'unknown'))},
    {kind:'mono',text:`${value(d.path,'unknown')} · ${dpiBytes(d.size_bytes)}`}
  ]:[{kind:'warn',text:'DPI Detector v5 не установлен'}];
  return {state,ready,tab,items,summary};
}
