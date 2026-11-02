import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { deriveDNSRouterAwareness } from './dnsRouterAwareness.js';
function status(overrides={}) {
  const base={mode:'independent-dns',active:true,interception:true,system_gate:{
    state:'ACTIVE',active:true,desired:true,last_error:'',hardware_e2e:'PENDING',
    families:[{family:'inet',prefixes:['192.168.10.0/24']}],
    coverage:{ipv4:'active_last_verified',ipv6_routed:'excluded_no_managed_routed_prefix',ipv6_link_local:'excluded_unproven_interface_scope',hardware_e2e:'PENDING'}
  }};
  return {...base,...overrides,system_gate:{...base.system_gate,...(overrides.system_gate||{})}};
}
test('P20: active IPv4 Gate reports only verified IPv4 LAN scope',()=>{
  const r=deriveDNSRouterAwareness(status());
  assert.equal(r.kind,'intercepting'); assert.deepEqual(r.ipv4Prefixes,['192.168.10.0/24']);
  assert.deepEqual(r.ipv6RoutedPrefixes,[]); assert.equal(r.ipv6LinkLocalExcluded,true); assert.equal(r.hardwareE2E,'PENDING');
});
test('P20: routed IPv6 appears only with active_last_verified coverage',()=>{
  const r=deriveDNSRouterAwareness(status({system_gate:{families:[{family:'inet',prefixes:['192.168.10.0/24']},{family:'inet6',prefixes:['2001:db8:10::/64']}],coverage:{ipv4:'active_last_verified',ipv6_routed:'active_last_verified'}}}));
  assert.deepEqual(r.ipv6RoutedPrefixes,['2001:db8:10::/64']);
});
test('P20: engine active with Gate OFF is neutral',()=>{
  const r=deriveDNSRouterAwareness(status({interception:false,system_gate:{active:false,desired:false,state:'STOCK'}}));
  assert.equal(r.kind,'engine-only'); assert.equal(r.gateActive,false);
});
test('P20: Gate active while engine stopped is critical',()=>{
  const r=deriveDNSRouterAwareness(status({active:false}));
  assert.equal(r.kind,'problem'); assert.equal(r.gateActive,true); assert.equal(r.engineActive,false);
});
test('P20: desired but unverified Gate never claims traffic redirection',()=>{
  const r=deriveDNSRouterAwareness(status({interception:false,system_gate:{active:false,desired:true,state:'DEGRADED',last_error:'competing rule'}}));
  assert.equal(r.kind,'degraded'); assert.equal(r.gateActive,false); assert.equal(r.lastError,'competing rule');
});
test('P20: fully inactive DNS Router produces no banner',()=>{
  const r=deriveDNSRouterAwareness(status({active:false,interception:false,system_gate:{active:false,desired:false,state:'STOCK'}}));
  assert.equal(r,null);
});
test('P20: active Gate without trusted prefix data does not invent LAN scope',()=>{
  const r=deriveDNSRouterAwareness(status({system_gate:{families:[],coverage:{ipv4:'configured_not_active',ipv6_routed:'excluded_no_managed_routed_prefix'}}}));
  assert.equal(r.kind,'intercepting'); assert.equal(r.hasActiveScope,false); assert.deepEqual(r.ipv4Prefixes,[]);
});
test('P20: frontend uses Core GET proxy only and links to DNS Router',()=>{
  const app=readFileSync(new URL('./DNSModuleApp.svelte',import.meta.url),'utf8');
  const proxy=readFileSync(new URL('../../../components/core/module_proxy.go',import.meta.url),'utf8');
  const manager=readFileSync(new URL('../../dns-router-lite/manager.go',import.meta.url),'utf8');
  assert.match(app,/\/api\/modules\/dns-router\/service\/status/); assert.match(app,/target="_top"/); assert.match(app,/aria-live="polite"/);
  const start=app.indexOf('async function refreshDNSRouterAwareness()'); const end=app.indexOf('function onDNSRouterVisibilityChange()',start);
  assert.ok(start>=0&&end>start); const probe=app.slice(start,end);
  assert.match(probe,/method:'GET'/); assert.doesNotMatch(probe,/method:\s*['"](?:POST|PUT|PATCH|DELETE)/); assert.doesNotMatch(probe,/X-RouterForge-Action/);
  assert.match(proxy,/"dns-router":\s*\{[^}]*routerforge-dns-router\.sock/s);
  assert.match(proxy,/if moduleID == "dns-router" \{\s*return method == http\.MethodGet/s);
  assert.match(proxy,/return "\/v1" \+ clean, true/);
  assert.match(manager,/case "\/v1\/service\/status"/); assert.match(manager,/s\["system_gate"\]\s*=\s*m\.Gate\.Status\(\)/);
});
