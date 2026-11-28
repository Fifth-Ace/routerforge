// Presentation only: never replays, modifies or clears historical operations.
const terminal = new Set(['completed','failed','cancelled','recovery_required']);
const iso = value => {const date=new Date(Number(value)*1000);return Number(value)>0 && Number.isFinite(date.getTime())?date.toISOString():'';};
const state = value => value==='completed'?'succeeded':value==='cancelled'?'cancelled':'failed';
function normalize(row, locale) {
  const count=Number(row.package_count)||0;
  return {id:`transaction:${row.id}`,transaction_id:row.id,target:locale==='ru'?'RouterForge · обновление':'RouterForge · update',
    action:'update',state:state(row.state),transaction_state:row.state,channel:'dev',method:'OPKG',target_version:row.version||'',
    started_at:iso(row.started_at),completed_at:iso(row.finished_at),recorded_at:iso(row.recorded_at),
    package_count:count,verified_count:row.verified_count,commit:row.commit||'',error:row.error_code||'',
    packages:Array.isArray(row.packages)?row.packages:[]};
}
export function mergeOperationHistory(legacy, snapshot, locale='ru') {
  const byId=new Map();
  for(const row of snapshot?.history||[]) {
    if(row?.id) byId.set(row.id,normalize(row,locale));
  }
  const slot=snapshot?.slot;
  if(slot?.plan?.id && !slot.busy && terminal.has(slot.state)) {
    const p=slot.plan,j=slot.journal||{};
    const packages=Array.isArray(p.packages)?p.packages:[];
    byId.set(p.id,normalize({id:p.id,commit:p.commit,state:terminal.has(j.phase)?j.phase:slot.state,package_count:packages.length,
      version:packages[0]?.version,packages:packages.map(x=>x.name),verified_count:j.next,started_at:j.started_at,
      finished_at:j.finished_at,error_code:j.error_code},locale));
  }
  return [...(legacy||[]),...byId.values()].sort((a,b)=>(Date.parse(b.started_at||b.recorded_at||'')||0)-(Date.parse(a.started_at||a.recorded_at||'')||0)||String(a.id).localeCompare(String(b.id)));
}
export function transactionHistoryDetail(job, detail) {
  const slot=detail?.slot;
  if(slot?.plan?.id!==job.transaction_id) throw new Error('History identity mismatch');
  return {...job,packages:(slot.plan.packages||[]).map(p=>p.name),lines:typeof detail.log==='string'?detail.log.split('\n'):[],
    log_truncated:detail.log_truncated===true,log_available:detail.log_available===true};
}
