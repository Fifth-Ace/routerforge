import test from 'node:test';
import assert from 'node:assert/strict';
import {
  createLiteSettingsFields,hydrateLiteSettings,refreshLiteSettingsOptions,
  buildLiteServicePrefs,mergeLiteServicePrefs,parseLiteDomains,IP_MODES
} from './DnsLiteSettingsModel.js';

const config=()=>({schema_version:1,
  upstreams:[{id:'dns-1',transport:'doh',server_name:'dns.example.org',address:'203.0.113.10:443'},
    {id:'dns-2',transport:'dot',server_name:'other.example.org',address:'203.0.113.11:853'},
    {id:'native',transport:'native',address:'native://System/./System'}],
  lists:[{id:'list-1',upstream:'dns-1',name:'Video'},{id:'list-2',upstream:'dns-2',name:'TV'}],
  service:{listen:'127.0.0.1:55358',cache_bytes:2*1024*1024,default_upstream:'dns-1',allowed_networks:['127.0.0.0/8'],
    protection:{rate_limit_rps:55,refuse_any:true,bogus_nxdomain:['0.0.0.0']},
    upstream_selection:{enabled:true,mode:'parallel',timeout_ms:2800},
    performance:{enabled:true,cache_min_ttl:15,cache_max_ttl:3600,optimistic_cache:true,pending_requests:'on',max_concurrent:20},
    privacy:{enabled:true,ecs_mode:'fixed',ecs_address:'8.8.8.8',dnssec_mode:'request_do'},
    dns64:{enabled:true,prefix:'64:ff9b::/96',nat64_confirmed:true},
    reverse_dns:{enabled:true,upstream:'dns-2'},
    ip_intelligence:{enabled:true,mode:'health_aware',scope:'lists',list_ids:['list-1','list-2'],
      health:{enabled:true,protocol:'tls',port:443,interval_seconds:100,timeout_ms:900,failure_threshold:3,recovery_threshold:4,min_down_seconds:45,http_path:'/health'}}
  }});
function hydrated(c=config()){const f=createLiteSettingsFields();hydrateLiteSettings(f,c);return f;}

test('all nine IP modes are distinct and explicitly enumerated',()=>{
  assert.equal(IP_MODES.length,9);assert.equal(new Set(IP_MODES).size,9);
});
test('hydration does not mutate saved config, binds 38 fields and exact list options',()=>{
  const c=config(),before=structuredClone(c),f=hydrated(c);
  assert.deepEqual(c,before);
  assert.equal(f.defaultDNS,'dns-1');assert.equal(f.cacheSize,2);assert.equal(f.p28Mode,'parallel');
  assert.equal(f.p30ECSMode,'fixed');assert.equal(f.p243MinDown,45);
  assert.deepEqual(f.p243ListIDs,['list-1','list-2']);
  assert.deepEqual(f.reverseOptions.map(x=>x.id),['dns-1','dns-2']);
});
test('roundtrip service fields preserves original CAS payload and all optional feature groups',()=>{
  const c=config(),f=hydrated(c),pref=buildLiteServicePrefs(f,c);
  assert.deepEqual(pref,c.service);
});
test('Svelte draft edits are pure until explicit serialization and do not modify original config',()=>{
  const c=config(),orig=structuredClone(c),f=hydrated(c);
  f.p241IPMode='sticky';f.p27RefuseAny=false;f.p243ListIDs=['list-2'];
  assert.deepEqual(c,orig);
  const next=buildLiteServicePrefs(f,c);
  assert.equal(next.ip_intelligence.mode,'sticky');assert.deepEqual(next.ip_intelligence.list_ids,['list-2']);
  assert.equal(next.protection.refuse_any,false);
});
test('all IP modes serialize without cross-DNS mutation',()=>{
  const c=config();for(const mode of IP_MODES){const f=hydrated(c);f.p241IPMode=mode;const out=buildLiteServicePrefs(f,c);assert.equal(out.ip_intelligence.mode,mode);}
});
test('all, domains, and lists scope are mutually exclusive and validated',()=>{
  const c=config(),f=hydrated(c);
  f.p243Scope='domains';f.p243Domains='Example.org\n#comment\nsub.example.net';
  let out=buildLiteServicePrefs(f,c).ip_intelligence;
  assert.deepEqual(out.domains,['example.org','sub.example.net']);assert.equal(out.list_ids,undefined);
  f.p243Scope='all';out=buildLiteServicePrefs(f,c).ip_intelligence;
  assert.equal(out.scope,undefined);assert.equal(out.list_ids,undefined);assert.equal(out.domains,undefined);
  f.p243Scope='lists';f.p243ListIDs=['list-1'];
  out=buildLiteServicePrefs(f,c).ip_intelligence;assert.deepEqual(out.list_ids,['list-1']);
});
test('stale list selection is hard failure, never silently changed to another DNS',()=>{
  const c=config(),f=hydrated(c);f.p243ListIDs=['removed'];
  assert.throws(()=>buildLiteServicePrefs(f,c),/список изменился/);
  refreshLiteSettingsOptions(f,{...c,lists:[c.lists[1]]});
  assert.deepEqual(f.p243ListIDs,['removed']);
  assert.throws(()=>buildLiteServicePrefs(f,{...c,lists:[c.lists[1]]}),/список изменился/);
});
test('invalid or duplicate list IDs refused before persistence',()=>{
  const c=config(),f=hydrated(c);
  for(const ids of [[],['list-1','list-1'],['native'],['list-1','none']]){
    f.p243ListIDs=ids;
    assert.throws(()=>buildLiteServicePrefs(f,c));
  }
});
test('disabled IP Intelligence removes its settings, disabled health refuses without IP enabled',()=>{
  const c=config(),f=hydrated(c);
  f.p241IPEnabled=false;f.p242HealthEnabled=false;
  const next=buildLiteServicePrefs(f,c);assert.equal(next.ip_intelligence,undefined);
  assert.equal(mergeLiteServicePrefs(c.service,next).ip_intelligence,undefined);
  f.p242HealthEnabled=true;assert.throws(()=>buildLiteServicePrefs(f,c),/Сначала включи/);
});
test('disabled feature groups are removed while unchanged configuration metadata survives',()=>{
  const c=config(),f=hydrated(c);f.p28Enabled=false;f.p29Enabled=false;f.p30Enabled=false;
  f.p31DNS64Enabled=false;f.p31ReverseEnabled=false;f.p27RefuseAny=false;
  f.p27RateLimit=0;f.p27BogusIPs='';
  const merged=mergeLiteServicePrefs({...c.service,extra_field:'keep',default_group:'grp-1'},buildLiteServicePrefs(f,c));
  for(const k of ['upstream_selection','performance','privacy','dns64','reverse_dns','protection'])assert.equal(merged[k],undefined,k);
  assert.equal(merged.extra_field,'keep');assert.equal(merged.default_group,'grp-1');
});
test('new default invalidates default_group without mutating original config',()=>{
  const c=config(),f=hydrated(c);f.defaultDNS='dns-2';
  const pref=buildLiteServicePrefs(f,c),merged=mergeLiteServicePrefs({...c.service,default_group:'grp-1'},pref);
  assert.equal(merged.default_group,undefined);assert.equal(merged.default_upstream,'dns-2');
  assert.equal(c.service.default_upstream,'dns-1');
});
test('missing default allowed only for local draft, not for Save/Apply',()=>{
  const c=config(),f=hydrated(c);f.defaultDNS='';
  assert.throws(()=>buildLiteServicePrefs(f,c),/DNS по умолчанию/);
  assert.equal(buildLiteServicePrefs(f,c,false).default_upstream,'');
});
test('unknown upstream selection / IP / health protocol fail closed',()=>{
  const c=config(),f=hydrated(c);
  for(const [key,value] of [['p28Mode','automatic'],['p241IPMode','unknown'],['p242HealthProtocol','icmp']]){
    const g=structuredClone(f);g[key]=value;
    assert.throws(()=>buildLiteServicePrefs(g,c),/неизвестный режим/);
  }
});
test('numeric bounds are validated at persistence boundary',()=>{
  const c=config(),f=hydrated(c);
  for(const [key,value] of [['cacheSize',9],['p28Timeout',10],['p27RateLimit',50000],['p29Concurrent',999],['p242HealthPort',0],['p243MinDown',-1]]){
    const g=structuredClone(f);g[key]=value;
    assert.throws(()=>buildLiteServicePrefs(g,c),/допустимо/);
  }
});
test('domain parser normalizes, deduplicates and rejects invalid inputs',()=>{
  assert.deepEqual(parseLiteDomains('Example.com example.com\nsub.example.net #comment'),['example.com','sub.example.net']);
  for(const text of ['http://example.com','x..example.org','invalid-domain','x'.repeat(257*1024)])
    assert.throws(()=>parseLiteDomains(text));
});
test('invalid config type never initializes reactive UI',()=>{
  assert.throws(()=>hydrateLiteSettings({}, {upstreams:null}),/INVALID/);
  assert.throws(()=>buildLiteServicePrefs({}, {upstreams:null}),/INVALID/);
});
