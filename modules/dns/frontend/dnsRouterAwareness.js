const ACTIVE='active_last_verified';
function prefixesFor(families,family,enabled) {
  if(!enabled||!Array.isArray(families)) return [];
  const values=[];
  for(const item of families) {
    if(!item||item.family!==family||!Array.isArray(item.prefixes)) continue;
    for(const prefix of item.prefixes) if(typeof prefix==='string'&&prefix.trim()) values.push(prefix.trim());
  }
  return [...new Set(values)];
}
export function deriveDNSRouterAwareness(status) {
  if(!status||typeof status!=='object'||Array.isArray(status)) return null;
  const engineActive=status.mode==='independent-dns'&&status.active===true;
  const gate=status.system_gate&&typeof status.system_gate==='object'?status.system_gate:null;
  if(!gate) return engineActive?{kind:'engine-only'}:null;
  const gateActive=gate.active===true;
  const interceptionFlag=status.interception===true;
  const state=String(gate.state||'').toUpperCase();
  const coverage=gate.coverage&&typeof gate.coverage==='object'?gate.coverage:{};
  const families=Array.isArray(gate.families)?gate.families:[];
  const ipv4Prefixes=prefixesFor(families,'inet',coverage.ipv4===ACTIVE);
  const ipv6RoutedPrefixes=prefixesFor(families,'inet6',coverage.ipv6_routed===ACTIVE);
  const lastError=String(gate.last_error||status.last_error||'').trim();
  const common={engineActive,gateActive,gateState:state,lastError,ipv4Prefixes,ipv6RoutedPrefixes,
    hasActiveScope:ipv4Prefixes.length>0||ipv6RoutedPrefixes.length>0,
    ipv6LinkLocalExcluded:coverage.ipv6_link_local==='excluded_unproven_interface_scope',
    hardwareE2E:String(gate.hardware_e2e||coverage.hardware_e2e||'unknown')};
  if(gateActive&&(!engineActive||!interceptionFlag)) return {...common,kind:'problem'};
  if(gateActive!==interceptionFlag) return {...common,kind:'degraded'};
  if(gateActive&&engineActive) return {...common,kind:'intercepting'};
  if(gate.desired===true||state==='DEGRADED') return {...common,kind:'degraded'};
  if(engineActive) return {...common,kind:'engine-only'};
  return null;
}
