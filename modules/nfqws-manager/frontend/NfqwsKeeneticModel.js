// Pure read-only projection: never place router-provided fields in HTML strings.
export function keeneticBytes(n){
 const v=Number(n||0);if(!Number.isFinite(v)||v<=0)return '—';
 if(v<1048576)return `${Math.round(v/1024)} KB`;
 if(v<1073741824)return `${(v/1048576).toFixed(1)} MB`;
 return `${(v/1073741824).toFixed(1)} GB`;
}
export function keeneticProjection(d){
 if(!d||typeof d!=='object')return {state:'IDLE',network:[],binding:[],storage:[],policy:[],defaultInterface:'',platform:'—',ndmc:'—',rci:'—',entware:'—',model:'—'};
 const row=(severity,title,detail)=>({severity,title:String(title||''),detail:String(detail||'')});
 const iface=String(d.config_isp_interface||''),def=String(d.default_ipv4_interface||''),match=!!d.config_isp_matches_default;
 const used=d.entware_total_bytes?Math.max(0,Number(d.entware_total_bytes)-Number(d.entware_free_bytes||0)):0;
 return {
  state:d.detected?'KEENETIC':'GENERIC',platform:d.detected?'KEENETIC':'GENERIC',
  ndmc:d.ndmc_available?'READY':'NO',rci:d.rci_available?'READY':'NO',entware:d.opkg_available?'OPKG':'CHECK',
  model:[d.model||'model unknown',d.version||'version unknown'].join(' · '),defaultInterface:def,
  network:[row(def?'good':'warn','Default IPv4 route',def||'не определён'),
   ...(Array.isArray(d.interfaces)?d.interfaces:[]).map(x=>row(x.default_v4?'good':x.up?'info':'warn',`${x.name}${x.default_v4?' · DEFAULT':''}`,`operstate=${x.oper_state||'unknown'}`)),
   ...(Array.isArray(d.warnings)?d.warnings:[]).map(x=>row('warn','Read warning',x))],
  binding:[row(iface?'info':'warn','Configured ISP_INTERFACE',iface||'не задан'),row(def?'info':'warn','Kernel default-route interface',def||'не определён'),
   row(iface&&def?(match?'good':'warn'):'warn',match?'Binding MATCH':'Binding CHECK',iface&&def?(match?'ISP_INTERFACE совпадает с default IPv4 route.':'Значения различаются. Это может быть нормально при policy routing/VLAN/PPP, поэтому RouterForge ничего не меняет автоматически.'):'Недостаточно данных для сравнения.')],
  storage:[row(d.entware_mounted?'good':'warn','/opt mount',d.entware_mounted?`${d.entware_device||'device unknown'} · ${d.entware_fs_type||'fs unknown'}`:'отдельный mount не подтверждён'),row(d.opkg_available?'good':'warn','Entware opkg',d.opkg_available?'/opt/bin/opkg executable':'/opt/bin/opkg не найден или не executable'),row(d.entware_total_bytes?'info':'warn','Filesystem',d.entware_total_bytes?`${keeneticBytes(used)} used · ${keeneticBytes(d.entware_free_bytes)} free · ${keeneticBytes(d.entware_total_bytes)} total`:'statfs /opt недоступен')],
  policy:[row(d.rci_available?'good':'warn','RCI read endpoint',d.rci_available?'GET /rci/show/ip/policy доступен':'endpoint недоступен'),row(d.policy_identity?'info':'warn','Policy identity',d.policy_identity||'не получен'),row(d.policy_shape?'info':'warn','JSON shape',d.policy_shape?`${d.policy_shape} · top-level items ${d.policy_items||0}`:'не определён')]
 };
}
