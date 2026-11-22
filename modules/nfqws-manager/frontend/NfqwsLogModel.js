// Pure log presentation model. No DOM access and no network operations.
export function parseNfqwsLogLine(raw) {
  const text = String(raw ?? '');
  const parts = text.split(/\s+:\s+/);
  if (parts.length >= 6 && /^\d{2}\.\d{2}\.\d{4}\s+\d{2}:\d{2}:\d{2}$/.test(parts[0])) {
    return {raw:text,time:parts[0],host:parts[1],profile:parts[2].replace(/^profile\s+/i,''),client:parts[3].replace(/^client\s+/i,''),proto:parts[4].replace(/^proto\s+/i,''),event:parts.slice(5).join(' : ')};
  }
  return {raw:text,time:'',host:'',profile:'',client:'',proto:'',event:text};
}
export function nfqwsLogSeverity(entry) {
  const text = `${entry.raw} ${entry.event}`;
  if (/\b(?:ERROR|FATAL|CRITICAL|PANIC)\b/i.test(text) || /\bfailed\b/i.test(text)) return 'error';
  if (/\bfail counter\b/i.test(text) || /\b(?:WARN|WARNING|retry|timeout)\b/i.test(text)) return 'warn';
  if (/\b(?:udp_in|udp_out|started|running|success|recovered|added)\b/i.test(text)) return 'good';
  return 'info';
}
export function nfqwsLogModel(raw, query='', mode='all') {
  const entries = String(raw ?? '').split(/\r?\n/).filter(Boolean).map(parseNfqwsLogLine);
  const stats = {rows:entries.length,errors:0,warnings:0,fails:0,quic:0,tcp:0,udp:0};
  for (const entry of entries) {
    const severity = nfqwsLogSeverity(entry);
    if (severity==='error')stats.errors++;
    if (severity==='warn')stats.warnings++;
    if (/fail counter/i.test(entry.raw))stats.fails++;
    const proto=entry.proto.toLowerCase();
    if (Object.hasOwn(stats,proto) && ['quic','tcp','udp'].includes(proto))stats[proto]++;
  }
  const needle=String(query).trim().toLowerCase();
  return {stats,rows:entries.filter(entry=>{
    const severity=nfqwsLogSeverity(entry);
    return (mode==='all'||(mode==='errors'&&severity==='error')||(mode==='warnings'&&severity==='warn')) &&
      (!needle||entry.raw.toLowerCase().includes(needle));
  }).map(entry=>({...entry,severity:nfqwsLogSeverity(entry)}))};
}
export function nfqwsLogTokens(value) {
  const source=String(value||'');
  const regex=/(fail counter|udp_in|udp_out|retrans|timeout|retry|counter|threshold|\d+(?:\/\d+)?|<=|>=|==>|<==|<=>)/gi;
  const parts=[];let last=0,m;
  while((m=regex.exec(source))!==null){
    if(m.index>last)parts.push({text:source.slice(last,m.index),kind:''});
    const v=m[0],kind=/^\d/.test(v)?'log-token-number':/^(?:<=|>=|==>|<==|<=>)$/.test(v)?'log-token-op':'log-token-key';
    parts.push({text:v,kind});last=m.index+v.length;
  }
  if(last<source.length)parts.push({text:source.slice(last),kind:''});
  return parts;
}
