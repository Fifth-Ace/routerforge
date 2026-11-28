// The server owns eligibility, package identity and release commit.
export function previewSelection(snapshot) {
  const commit = snapshot?.release_commit;
  if (typeof commit !== 'string' || !/^[a-f0-9]{40}$/.test(commit)) return [];
  const rows = Array.isArray(snapshot?.eligible_packages) ? snapshot.eligible_packages : [];
  const seen = new Set();
  return rows.filter((row) => {
    if (typeof row?.package !== 'string' || !/^routerforge-[a-z0-9-]+$/.test(row.package) || seen.has(row.package)) return false;
    seen.add(row.package);
    return typeof row.installed_version === 'string' && typeof row.target_version === 'string';
  });
}
export function selectionKey(snapshot) {
  return JSON.stringify([snapshot?.release_commit || '', previewSelection(snapshot).map((row) => [row.package, row.installed_version, row.target_version])]);
}
export function previewMatches(preview, commit, selected) {
  if (typeof preview?.execution_available !== 'boolean' || preview?.plan?.commit !== commit || preview?.plan?.channel !== 'dev' || preview?.plan?.target !== 'aarch64-3.10' || !/^[a-f0-9]{64}$/.test(preview?.plan_sha256 || '')) return false;
  if (!Array.isArray(preview?.plan?.packages)) return false;
  const names = preview.plan.packages.map((p) => p?.name);
  return Array.isArray(names) && names.length === selected.length && new Set(names).size === names.length && names.every((name) => selected.includes(name));
}
export function transactionProgress(slot) {
  const total = slot?.plan?.packages?.length;
  const verified = slot?.journal?.next;
  if (!Number.isInteger(total) || total < 1 || !Number.isInteger(verified) || verified < 0 || verified > total) return null;
  return { verified, total };
}

// Only an explicitly admitted preview may be submitted. The server rebuilds
// the exact plan under its lease; the client cannot supply executable metadata.
export function executionRequest(snapshot, preview, selected) {
  if (snapshot?.execution_available !== true || preview?.execution_available !== true || !previewMatches(preview,snapshot.release_commit,selected) || !/^[a-f0-9]{32}$/.test(preview.plan.id || '') || (snapshot?.slot?.busy || !['idle','completed','failed','cancelled','recovery_required'].includes(snapshot?.slot?.state))) return null;
  if (selected.some((name) => !previewSelection(snapshot).some((row) => row.package === name && row.target_version === preview.plan.packages.find((p) => p.name === name)?.version))) return null;
  return { id:preview.plan.id, expected_commit:preview.plan.commit, packages:[...selected], plan_sha256:preview.plan_sha256, confirm:'EXECUTE:'+preview.plan_sha256 };
}

export function updateRows(slot) {
  const packages=slot?.plan?.packages || []; const j=slot?.journal; const done=j?.next || 0; const downloaded=j?.downloaded || 0;
  const failed=['recovery_required','failed','cancelled'].includes(slot?.state);
  const downloadFailure=j?.error_code==='prepare_failed'||(failed&&j?.phase==='preparing');
  return packages.map((p,i)=>({ ...p, state:i<done?'completed':failed?(downloadFailure?(i<downloaded?'downloaded':i===downloaded?'failed':'queued'):(i===done?'failed':'queued')):slot.state==='preparing'?(i<downloaded?'downloaded':i===downloaded?'downloading':'queued'):i===done&&slot.state==='applying'?'applying':i===done&&slot.state==='verifying'?'verifying':'queued' }));
}
export function updateRunning(slot){return !!slot && ['queued','preparing','prepared','applying','verifying'].includes(slot.state);}

// Server-reported bytes, never synthetic package percentages. Old journal rows
// remain readable during the rolling upgrade of Core and Worker.
export function downloadStats(slot, downloads = []) {
  const packages=slot?.plan?.packages || [];
  const map=new Map((downloads || []).filter(r=>r.id===slot?.plan?.id).map(r=>[r.name,r]));
  const allDownloaded=['prepared','applying','verifying','completed'].includes(slot?.state) || (slot?.journal?.downloaded===packages.length && packages.length>0);
  const rows=packages.map((p,i)=>{
    const size=Number(p.size)||0;
    const d=map.get(p.name);
    const done=allDownloaded || d?.state==='downloaded' || (!map.size && i<(slot?.journal?.downloaded||0));
    return {...p,bytes:done?size:Math.max(0,Math.min(size,Number(d?.bytes)||0)),downloadState:done?'downloaded':d?.state||'queued',source:d?.source||'',rate:!done && d?.state==='downloading'?Math.max(0,Number(d.rate)||0):0};
  });
  const bytes=rows.reduce((n,p)=>n+p.bytes,0), total=rows.reduce((n,p)=>n+(Number(p.size)||0),0);
  return {rows,bytes,total,rate:rows.reduce((n,p)=>n+p.rate,0),percent:total?Math.floor(100*bytes/total):0,finished:rows.filter(p=>p.downloadState==='downloaded').length};
}
export function progressRows(slot,downloads) {
  const stats=downloadStats(slot,downloads);
  const base=updateRows(slot);
  if (!downloads?.length) return base;
  return base.map((p,i)=>{
    const d=stats.rows[i];
    let state=p.state;
    if(slot?.state==='preparing' || slot?.journal?.error_code==='prepare_failed') state=d.downloadState;
    return {...p,...d,state};
  });
}
