// DPI Detector V5 Web: pure request and report projections, no network or DOM.
// Backend owns the final safety check; client must never equate a truncated stream
// or an unconfirmed `done` with successful production-state verification.
export const dpiTestOptions = Object.freeze([
  {id:'0',name:'Network',detail:'IP · ASN · DNS · gateway · tunnels'},
  {id:'1',name:'DNS',detail:'DoH · DoT · UDP · substitutions'},
  {id:'2',name:'Domains',detail:'DNS → HTTP → TLS 1.2 / 1.3'},
  {id:'3',name:'TCP16',detail:'16–20 KB behavior matrix'},
  {id:'4',name:'Whitelist SNI',detail:'white-SNI / AS detection'},
  {id:'5',name:'Telegram',detail:'DC reachability · transfers'},
  {id:'6',name:'Fingerprint Burst',detail:'TLS shapes · ALPN · overlaps'}
]);
const SHA=/^[a-f0-9]{64}$/i;
const DOMAIN=/^(?=.{1,253}$)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/;
export function dpiStreamPath(pathname){
  if(typeof pathname!=='string')throw Error('DPI_PATH_INVALID');
  const marker='/ui/',p=pathname.indexOf(marker);
  const prefix=p>=0?pathname.slice(0,p):'/api/modules/nfqws-manager';
  if(!/^\/api\/modules\/nfqws-manager$/.test(prefix))throw Error('DPI_PATH_INVALID');
  return prefix+'/dpi-detector/v5/stream';
}
function whole(value,min,max,label){
  const n=Number(value);
  if(!Number.isSafeInteger(n)||n<min||n>max)throw Error('DPI_'+label+'_INVALID');
  return n;
}
function inputText(value,max,label){
  if(typeof value!=='string'||value.length>max||/[\u0000-\u0008\u000b\u000c\u000e-\u001f]/.test(value))throw Error('DPI_'+label+'_INVALID');
  return value.trim();
}
export function dpiWebPayload(form,sha){
  if(!SHA.test(String(sha||'')))throw Error('DPI_CONFIG_SHA_MISSING');
  if(!form||typeof form!=='object'||Array.isArray(form))throw Error('DPI_FORM_INVALID');
  const chosen=Array.isArray(form.tests)?form.tests:[];
  if(chosen.some(x=>typeof x!=='string'||!/^[0-6]$/.test(x)))throw Error('DPI_TEST_INVALID');
  const tests=Array.from(new Set(chosen)).sort().join('');
  if(!tests)throw Error('DPI_TEST_REQUIRED');
  const domains=inputText(form.domains||'',32768,'DOMAINS').split(/[\n,;]+/).map(x=>x.trim().toLowerCase().replace(/\.$/,'' )).filter(Boolean);
  if(domains.length>128||domains.some(d=>!DOMAIN.test(d)||d.endsWith('.local')||d.endsWith('.localhost')))throw Error('DPI_DOMAINS_INVALID');
  if(!['ru','en','zh','fa'].includes(form.language))throw Error('DPI_LANGUAGE_INVALID');
  if(!['ru','global','ir','cn'].includes(form.profile))throw Error('DPI_PROFILE_INVALID');
  const tls=String(form.burstTLS||'1.3+1.2'),alpn=String(form.burstALPN||'h2');
  if(!['1.3+1.2','1.3','1.2'].includes(tls)||!['h2','http/1.1'].includes(alpn))throw Error('DPI_BURST_PROTOCOL_INVALID');
  const fingerprints=['','rustls','chrome146','chrome131','chrome131_android','chrome123','chrome116','chrome107','chrome99_android','edge101','firefox147','firefox133','safari260','safari260_ios','safari184_ios','safari180','safari172_ios','safari170','safari155','safari153','tor145'];
  if(!fingerprints.includes(form.fingerprint||''))throw Error('DPI_FINGERPRINT_INVALID');
  return {
    tests,domains,domains_file:inputText(form.domainsFile||'',2048,'DOMAINS_FILE'),tcp16_file:inputText(form.tcp16File||'',2048,'TCP16_FILE'),
    concurrency:whole(form.concurrency,1,100,'CONCURRENCY'),language:form.language,profile:form.profile,
    interface:inputText(form.interface||'',256,'INTERFACE'),proxy:inputText(form.proxy||'',2048,'PROXY'),fingerprint:form.fingerprint||'',
    burst:whole(form.burst,1,100,'BURST'),burst_timeout:whole(form.burstTimeout,1,120,'BURST_TIMEOUT'),
    burst_profiles:inputText(form.burstProfiles||'',2048,'BURST_PROFILES'),burst_tls:tls,burst_alpn:alpn,trace:false,
    expected_config_sha256:String(sha).toLowerCase(),confirm:'ROUTERFORGE_DPI_DETECTOR_V5_RUN'
  };
}
export function dpiEndProof(event,expectedSHA){
  if(event?.type!=='done')throw Error('DPI_DONE_REQUIRED');
  if(event.production_config_unchanged!==true)throw Error('DPI_PRODUCTION_STATE_UNPROVEN');
  if(!SHA.test(String(event.production_config_sha256||''))||event.production_config_sha256.toLowerCase()!==expectedSHA.toLowerCase())throw Error('DPI_PRODUCTION_SHA_MISMATCH');
  return {duration:Math.max(0,Number(event.duration_ms)||0),bytes:Math.max(0,Number(event.report_bytes)||0),cut:!!event.report_cut,exitWarning:String(event.exit_warning||''),environmentWarning:String(event.environment_warning||''),configSHA:expectedSHA};
}
// Parse only navigational section headings, never synthesize 'PASS' from
// diagnostic prose. The original upstream report remains available verbatim.
export function dpiReportSections(report){
  const text=String(report||'').replace(/\x1b\[[0-?]*[ -/]*[@-~]/g,'').replace(/\x1b\][^\x07]*(?:\x07|\x1b\\)/g,'').replace(/\r\n?/g,'\n');
  if(!text.trim())return [];
  const headings=[/Информация о сети и системе|Network\s*&?\s*system information|Network information/i,/Проверка доступности DNS-серверов|DNS server availability|DNS availability/i,/Проверка доступности доменов|Результаты проверки доменов|Website availability|Domain availability/i,/Проверка TCP\s*16|TCP\s*16.?20|CDN.*hosting availability/i,/Поиск белых SNI|Whitelist SNI|white.?SNI/i,/Проверка доступности Telegram|Telegram availability|Telegram reachability/i,/^\s*Fingerprint(?:\/|[\s|:]|$)|Сибирская блокировка|Fingerprint burst/i];
  const lines=text.split('\n'),markers=[];
  for(let i=0;i<lines.length;i++){
    const id=headings.findIndex(re=>re.test(lines[i]));
    if(id>=0&&(!markers.length||markers[markers.length-1].id!==id))markers.push({at:i,id});
  }
  if(!markers.length)return[{id:'all',title:'Отчёт upstream',text}];
  const sections=[];
  if(markers[0].at>0)sections.push({id:'intro',title:'Введение',text:lines.slice(0,markers[0].at).join('\n')});
  for(let i=0;i<markers.length;i++){
    const marker=markers[i],end=i+1<markers.length?markers[i+1].at:lines.length;
    sections.push({id:`${marker.id}-${i}`,title:dpiTestOptions[marker.id].name,text:lines.slice(marker.at,end).join('\n')});
  }
  return sections.filter(s=>s.text.trim());
}
