/* Pure NFQWS overview projection for the one Svelte owner. GET/status only.
 * The source response is never interpreted as HTML or treated as an action.
 */
export function nfqwsSize(bytes) {
  const n=Number(bytes||0);
  if(!Number.isFinite(n)||n<0)return '—';
  if(n<1024)return `${n} B`;
  if(n<1048576)return `${(n/1024).toFixed(1)} KB`;
  return `${(n/1048576).toFixed(1)} MB`;
}
export function nfqwsOverview(status) {
  const s=status||{},lists=Array.isArray(s.lists)?s.lists:[],
    available=Boolean(s.available),running=Boolean(s.running),
    ready=Boolean(s.mutation_ready),configExists=Boolean(s.config_exists);
  const activity=[
    {ok:available,title:'External target detected',description:'required nfqws paths'},
    {ok:running,title:'Runtime running',description:'nfqws process'},
    {ok:configExists,title:'Config readable',description:`${nfqwsSize(s.config_size)} · SHA verified`},
    {ok:!s.config_truncated,title:'Config fully loaded',description:'full config available'},
    {ok:ready,title:'Safe mutation ready',description:'confirm + atomic write + backup'}
  ];
  return {
    running,ready,configExists,
    configSummary:`${nfqwsSize(s.config_size)} · ${String(s.config_sha256||'').slice(0,12)||'no SHA'}`,
    listCount:lists.length,listSummary:`${nfqwsSize(lists.reduce((a,b)=>a+Math.max(0,Number(b.size)||0),0))} total`,
    target:`${String(s.target_name||'—')} (${String(s.target_id||'—')})`,
    initScript:String(s.init_script||'—'),configPath:String(s.config_path||'—'),
    configSHA:String(s.config_sha256||'—'),listsRoot:String(s.lists_root||'—'),
    logPath:String(s.log_path||'—'),mutation:ready?'Core-guarded · ready':'not ready',
    activity
  };
}
