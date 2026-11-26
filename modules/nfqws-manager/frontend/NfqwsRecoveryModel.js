// Existing E06.2 native list recovery. Never fall back to legacy restore.
const hash = v => typeof v==='string' && /^[a-f0-9]{64}$/.test(v);
const protectedNames = new Set(['user.list','exclude.list','auto.list','ipset.list','ipset_exclude.list']);
const targetName = entry => typeof entry?.target==='string' && entry.target.startsWith('/opt/etc/nfqws2/lists/') ? entry.target.slice('/opt/etc/nfqws2/lists/'.length) : '';
export function bindingBackup(entry) {return entry?.kind==='list' && /(?:^|\/)rf-binding-/i.test(String(entry.target||''));}
export function nativeListTarget(entry) {
  const name=targetName(entry);
  return entry?.kind==='list' && /^[A-Za-z0-9_.-]+\.list$/i.test(name) &&
    !protectedNames.has(name.toLowerCase()) && !bindingBackup(entry);
}
export function nativeListRequired(entry) {return nativeListTarget(entry) && entry.state!=='MISSING';}
export function safeListRecovery(entry) {
  return nativeListTarget(entry) && entry.restorable===true && entry.state==='CHANGED' &&
    typeof entry.id==='string' && entry.id.length>0 && entry.id.length<=128 &&
    hash(entry.sha256) && hash(entry.current_sha256) && entry.sha256!==entry.current_sha256;
}
export function checkedRecoveryPlan(entry, plan) {
  if (!safeListRecovery(entry) || !plan || plan.id!==entry.id || plan.target!==targetName(entry) ||
      plan.current_sha256!==entry.current_sha256 || plan.target_sha256!==entry.sha256 ||
      plan.can_commit!==true || !hash(plan.token) || plan.runtime!=='NOT_VERIFIED' ||
      !Number.isSafeInteger(plan.size) || plan.size!==entry.size || plan.size<0)
    throw Error('NFQWS_RECOVERY_PREVIEW_MISMATCH');
  return {id:entry.id,token:plan.token,confirm:'NFQWS_RECOVERY_LIST_SAVE'};
}
export function verifiedRecoveryResult(entry, result) {
  if (!result || result.ok!==true || result.readback_verified!==true || result.sha256!==entry.sha256 ||
      result.service_restarted!==false || result.runtime!=='NOT_VERIFIED' || typeof result.safety_backup!=='string' || !result.safety_backup)
    throw Error('NFQWS_RECOVERY_RESULT_UNCERTAIN');
  return result.safety_backup;
}
export async function recoverReviewedList(entry, api, confirm, current) {
  if (!safeListRecovery(entry) || !current()) throw Error('NFQWS_RECOVERY_SELECTION_STALE');
  const plan=await api('/backups/recovery-preview?id='+encodeURIComponent(entry.id));
  const body=checkedRecoveryPlan(entry,plan);
  if (!current()) throw Error('NFQWS_RECOVERY_SELECTION_STALE');
  if (!confirm(plan)) return null;
  if (!current()) throw Error('NFQWS_RECOVERY_SELECTION_STALE');
  const result=await api('/backups/recovery-save',{method:'POST',body:JSON.stringify(body)});
  return verifiedRecoveryResult(entry,result);
}
