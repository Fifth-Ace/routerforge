// Pure projection for native Svelte List Inspector and Cross-List Analysis.
// Domain/IP relationship algorithms remain in the backend's read-only comparator
// until they can be lifted without changing parity. This module has no DOM or IO.
const filters=new Set(['all','conflict','domain','network']);
export function listInspectorView(record,findings=[]){
  const rows=Array.isArray(findings)?findings:[];
  const file=String(record?.file||'');
  return {value:String(record?.value||''),meta:record?`${file}:${record.line} · ${record.type==='domain'?'DOMAIN':String(record.type||'').toUpperCase()}`:'—',
    scope:record?`${file}:${record.line} ↔ остальные списки`:'—',
    role:file.toLowerCase().includes('exclude')?'EXCLUDE':'INCLUDE',
    excluded:file.toLowerCase().includes('exclude'),
    total:rows.length,conflicts:rows.filter(x=>x.severity==='conflict').length,
    files:new Set(rows.map(x=>x.b?.file).filter(Boolean)).size,
    rows:rows.map(x=>({kind:String(x.kind||''),detail:String(x.detail||''),
      file:String(x.b?.file||''),line:Number(x.b?.line)||1,
      severity:x.severity==='conflict'?'conflict':'warn'}))};
}
function sideRows(findings,file){
  const byLine=new Map();
  for(const f of findings){for(const rec of [f.a,f.b]){
    if(rec?.file!==file)continue;
    const line=Number(rec.line);
    if(!Number.isSafeInteger(line)||line<1)continue;
    let row=byLine.get(line);
    if(!row){row={line,value:String(rec.value||''),kinds:new Set(),severity:'warn'};byLine.set(line,row)}
    row.kinds.add(String(f.kind||''));if(f.severity==='conflict')row.severity='conflict';
  }}
  return [...byLine.values()].sort((a,b)=>a.line-b.line).map(x=>({...x,kinds:[...x.kinds].join(' · ')}));
}
export function listAnalysisView({selected='',findings=[],peer='',filter='all'}={}){
  const safe=Array.isArray(findings)?findings.filter(f=>f?.a?.file&&f?.b?.file):[],chosen=String(selected||'');
  const scoped=safe.filter(f=>f.a?.file===chosen||f.b?.file===chosen);
  const peers=[...new Set(scoped.map(f=>f.a?.file===chosen?f.b?.file:f.a?.file).filter(x=>x&&x!==chosen))].sort((a,b)=>a.localeCompare(b));
  const target=peers.includes(peer)?peer:peers[0]||'';
  const pair=scoped.filter(f=>(f.a.file===chosen&&f.b.file===target)||(f.b.file===chosen&&f.a.file===target));
  const validFilter=filters.has(filter)?filter:'all';
  const visible=pair.filter(f=>validFilter==='all'||validFilter==='conflict'&&f.severity==='conflict'||validFilter===f.scope);
  const files=new Set(scoped.flatMap(f=>[f.a?.file,f.b?.file]).filter(Boolean));
  const left=sideRows(visible,chosen),right=sideRows(visible,target);
  const allLeft=sideRows(pair,chosen),allRight=sideRows(pair,target);
  return {selected:chosen,peer:target,peers,filter:validFilter,
    total:scoped.length,conflicts:scoped.filter(f=>f.severity==='conflict').length,
    warnings:scoped.filter(f=>f.severity!=='conflict').length,files:files.size,
    left,right,allLeft,allRight,
    pairTotal:pair.length,
    pairMeta:target?`${pair.length} пересечений между файлами · ${allLeft.length} строк слева · ${allRight.length} строк справа`:scoped.length?'Выбери пару списков.':'Пересечений не найдено.'};
}
export function removeIntersectingLines(content,lineNumbers){
  if(typeof content!=='string')throw Error('NFQWS_ANALYSIS_SOURCE_INVALID');
  if(!Array.isArray(lineNumbers)||lineNumbers.some(x=>!Number.isSafeInteger(x)||x<1))throw Error('NFQWS_ANALYSIS_LINE_INVALID');
  const newline=content.includes('\r\n')?'\r\n':'\n',trailing=content.endsWith('\n'),rows=content.split(/\r?\n/);
  if(trailing&&rows.at(-1)==='')rows.pop();
  const remove=new Set(lineNumbers);
  if([...remove].some(x=>x>rows.length))throw Error('NFQWS_ANALYSIS_LINE_STALE');
  const kept=rows.filter((_,index)=>!remove.has(index+1));
  return kept.join(newline)+(trailing?newline:'');
}
