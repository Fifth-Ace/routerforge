export const esc = (v='') => String(v).replace(/[&<>'"]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]));
export const clamp = (n,a,b) => Math.max(a,Math.min(b,n));
export const fmtInt = n => new Intl.NumberFormat('ru-RU').format(Number(n||0));
export const fmtPct = n => `${Number(n||0).toFixed(n >= 10 ? 1 : 2)}%`;
export const fmtMs = n => n > 0 ? `${Math.round(n)}ms` : '—';
export const fmtAgo = (iso) => {
  if (!iso || iso.startsWith('0001-')) return '—';
  const s = Math.max(0, Math.round((Date.now()-new Date(iso).getTime())/1000));
  if (s < 5) return 'только что';
  if (s < 60) return `${s} сек назад`;
  if (s < 3600) return `${Math.floor(s/60)} мин назад`;
  if (s < 86400) return `${Math.floor(s/3600)} ч назад`;
  return `${Math.floor(s/86400)} дн назад`;
};
export const fmtDuration = sec => {
  sec = Math.max(0, Number(sec||0));
  const d=Math.floor(sec/86400), h=Math.floor(sec%86400/3600), m=Math.floor(sec%3600/60);
  return [d?`${d}д`:null,h?`${h}ч`:null,m?`${m}м`:null].filter(Boolean).join(' ') || `${Math.floor(sec)}с`;
};
export function statusFor(u) {
  const h=(u.health_status||'').toUpperCase();
  if (h==='DOWN') return { cls:'down', label:'Недоступен', pill:'error' };
  if (h==='DEGRADED') return { cls:'degraded', label:'Деградация', pill:'warn' };
  if (u.active) return { cls:'', label:'Активен', pill:'ok' };
  return { cls:'idle', label:'Доступен', pill:'ok' };
}
export function errorCount(u) { return Number(u.servfail||0)+Number(u.refused||0)+Number(u.other_errors||0); }
export function quality(u, windowKey='stats_5m') {
  const w=u?.[windowKey];
  if (w && (Number(w.requests||0) || Number(w.responses||0) || Number(w.timeouts||0))) return Number(w.quality_pct ?? 100);
  const r=Number(u.responses||0); if (!r) return 100;
  return clamp(100 - errorCount(u)/r*100,0,100);
}
export function qualityClass(w) {
  const s=(w?.quality_status||'NO_DATA').toUpperCase();
  return s==='BAD'?'bad':s==='WARN'?'warn':s==='GOOD'?'good':'';
}
export function windowErrors(w) { return Number(w?.errors||0)+Number(w?.timeouts||0); }

export function latencyClass(n) { return n >= 1000 ? 'bad' : n >= 300 ? 'slow' : ''; }
export function profileOrder(a,b) {
  if (a==='System') return 1; if (b==='System') return -1;
  const na=Number((a.match(/\d+/)||[999])[0]), nb=Number((b.match(/\d+/)||[999])[0]); return na-nb;
}
export function groupBy(arr,key) { return arr.reduce((m,x)=>((m[x[key]]??=[]).push(x),m),{}); }
export function total(arr,key) { return arr.reduce((s,x)=>s+Number(x[key]||0),0); }
export function timeOnly(iso) { try { return new Date(iso).toLocaleTimeString('ru-RU',{hour:'2-digit',minute:'2-digit',second:'2-digit'}); } catch { return '—'; } }
export function dateTime(iso) { try { return new Date(iso).toLocaleString('ru-RU'); } catch { return '—'; } }
export function bytes(n) { n=Number(n||0); if(n<1024)return `${n} B`; if(n<1048576)return `${(n/1024).toFixed(1)} KB`; return `${(n/1048576).toFixed(1)} MB`; }
