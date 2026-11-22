// Pure, DOM-free Backup Center projection. Backend retains the authoritative
// snapshot/restore ledger and mandatory confirmation contracts.
export const backupBytes = value => {
  const n = Number(value);
  if (!Number.isFinite(n) || n < 0) return '—';
  if (n < 1024) return `${Math.floor(n)} B`;
  if (n < 1048576) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1048576).toFixed(1)} MB`;
};
export function backupDiff(current, saved) {
  const a = String(current ?? '').split(/\r?\n/);
  const b = String(saved ?? '').split(/\r?\n/);
  const rows = [];
  for (let i = 0; i < Math.max(a.length, b.length); i++) {
    if (a[i] === b[i]) { if (a[i] !== undefined) rows.push({c:'same',t:`  ${a[i]}`}); continue; }
    if (a[i] !== undefined) rows.push({c:'del',t:`- ${a[i]}`});
    if (b[i] !== undefined) rows.push({c:'add',t:`+ ${b[i]}`});
  }
  return rows;
}
export function backupProjection(snapshot={}, term='') {
  const all = Array.isArray(snapshot.files) ? snapshot.files.filter(x=>x && typeof x === 'object') : [];
  const query = String(term).trim().toLowerCase();
  const filtered = all.filter(x => !query || [x.label,x.kind,x.target,x.sha256,x.state].map(v=>String(v ?? '')).join(' ').toLowerCase().includes(query));
  const entry = snapshot.detail?.entry || null;
  return {
    files:filtered, total:all.length,
    changed:all.filter(x=>x.state==='CHANGED'||x.state==='MISSING').length,
    restorable:all.filter(x=>Boolean(x.restorable)).length,
    selected:String(snapshot.selected || ''), entry,
    diff: typeof snapshot.detail?.content === 'string'
      ? backupDiff(snapshot.detail.current_content, snapshot.detail.content) : [],
    isText:typeof snapshot.detail?.content === 'string',
    root:String(snapshot.root||'/opt/var/lib/routerforge/nfqws-manager/backups'),
    loading:Boolean(snapshot.loading),error:String(snapshot.error||''),
    detailError:String(snapshot.detailError||''),safetyCount:Number(snapshot.safetyCount||0),
    detailLoading:Boolean(snapshot.detailLoading)
  };
}
