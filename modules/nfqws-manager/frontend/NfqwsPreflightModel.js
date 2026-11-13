// Read-only projection. Preflight verdict is not a packet-path or DPI success claim.
const order=['Runtime','Config','Traffic','Lists','Observability','Safety'];
export function preflightView(items=[],{dirty=false,available=false,busy=false}={}){
  const rows=Array.isArray(items)?items.filter(x=>x&&typeof x==='object'):[];
  const bad=rows.filter(x=>x.severity==='bad').length;
  const warn=rows.filter(x=>x.severity==='warn').length;
  const good=rows.filter(x=>x.severity==='good').length;
  const state=!available||bad?'BLOCKED':warn?'CHECK':'READY';
  const groups=order.map(group=>{
    const values=rows.filter(x=>x.group===group).map(x=>({
      severity:['bad','warn','good','info'].includes(x.severity)?x.severity:'info',
      title:String(x.title||''),detail:String(x.detail||''),tab:String(x.tab||'')}));
    return{group,items:values,summary:values.some(x=>x.severity==='bad')?'BLOCK':values.some(x=>x.severity==='warn')?'CHECK':'PASS'};
  }).filter(x=>x.items.length);
  return{state,dirty:!!dirty,busy:!!busy,bad:bad+(!available?1:0),warn,good,groups,
    subtitle:dirty?'Проверяется текущий setup draft + свежий runtime snapshot. Ничего не применяется.':'Проверяется active canonical config + текущий runtime snapshot. Ничего не применяется.'};
}
