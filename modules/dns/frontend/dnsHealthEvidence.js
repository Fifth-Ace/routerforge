// UI evidence only. Does not change native resolver health or routing.
const fresh = (value, now, maxAge) => {
  if (typeof value !== 'string' || value.startsWith('0001-')) return false;
  const t = Date.parse(value);
  return Number.isFinite(t) && t <= now && now - t <= maxAge;
};
export function protectedEvidence(row, now) {
  if (!fresh(row?.last_health_check, now, 180000) && !fresh(row?.last_observed, now, 180000)) return 'unknown';
  if (row.health_status === 'UP') return 'healthy';
  if (['DOWN','DEGRADED'].includes(row.health_status)) return 'problem';
  return 'unknown';
}
export function plainEvidence(row, status, now, minutes) {
  const end = Math.floor(now / 60000);
  const window = Number.isFinite(minutes) ? Math.max(1, Math.min(15, minutes)) : 5;
  const measured = Array.isArray(row?.health_buckets) && row.health_buckets.some(b =>
    Number.isInteger(b?.minute) && b.minute <= end && b.minute > end - window &&
    [b.responses,b.timeouts].some(n => typeof n === 'number' && Number.isFinite(n) && n > 0));
  if (!measured) return 'unknown';
  if (['warn','error'].includes(status?.cls)) return 'problem';
  // Some attempts can fail without crossing the existing warning threshold.
  // Require at least one observed successful response for a positive count.
  const good = row.health_buckets.some(b => Number.isInteger(b?.minute) && b.minute <= end && b.minute > end-window &&
    typeof b.responses === 'number' && typeof b.errors === 'number' && b.responses > b.errors && b.errors >= 0);
  return good ? 'healthy' : 'unknown';
}
export function evidenceCounts(protectedRows, plainRows, plainStates, now, minutes, protectedFresh, plainFresh) {
  const out = {healthy:0,problem:0,unknown:0};
  protectedRows.forEach(row => out[protectedFresh ? protectedEvidence(row,now) : 'unknown']++);
  plainRows.forEach((row,i) => out[plainFresh ? plainEvidence(row,plainStates[i],now,minutes) : 'unknown']++);
  return out;
}
export function qualityText(window, locale='ru') {
  const attempts = ['success','errors','timeouts'].reduce((n,k)=>n+(typeof window?.[k]==='number' && Number.isFinite(window[k]) && window[k]>0?window[k]:0),0);
  const q = window?.quality_pct;
  return attempts > 0 && typeof q === 'number' && Number.isFinite(q) && q>=0 && q<=100 ? q.toFixed(1)+'%' : (locale==='en'?'no data':'нет данных');
}
