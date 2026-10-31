'use strict';
const fs=require('node:fs'),p=require('node:path'),test=require('node:test'),assert=require('node:assert/strict');
const root=p.resolve(__dirname,'..');
const read=s=>fs.readFileSync(p.join(root,s),'utf8');
const path='modules/dns-router-runtime/';
const ui=read(path+'frontend/app.js'),html=read(path+'frontend/index.html'),init=read(path+'packaging/S95routerforge-dns-router-dev');
const main=read(path+'cmd/routerforge-dns-router/main.go'),api=read(path+'api.go');
const n=read(path+'transaction_n.go'),o2=read(path+'managed_o2.go'),o3=read(path+'managed_o3.go'),listener=read(path+'listener_l.go');
test('P10 is real O2 durable apply, not a click-through to expert UI',()=>{
 assert.match(ui,/request\('keenetic\/durable\/prepare'/);
 assert.match(ui,/request\('keenetic\/durable\/apply'/);
 assert.match(ui,/request\('keenetic\/durable\/disable'/);
 assert.match(ui,/request\('shadow\/control'/);
 assert.match(ui,/p10Activate/);
 assert.match(html,/id="drP92Apply">▶ Применить DNS/);
 assert.match(html,/id="drP10Disable"/);
 assert.doesNotMatch(ui,/p92Notify\('Реальное применение DNS выполняется отдельно/);
});
test('P10 System only until PolicyN isolation is verified',()=>{
 assert.match(ui,/if\(p88\.policy!=='System'\)/);
 assert.match(ui,/native:\/\/System\//);
 assert.match(ui,/ownedHere/);
});
test('P10 does not hide partial failure or overwrite owned zone ledger',()=>{
 assert.match(ui,/ownership\.state!=='disabled'/);
 assert.match(ui,/ledger_hash/);
 assert.match(ui,/baseline_restored/);
 assert.match(ui,/If-Match/);
});
test('P10 63-capacity exists ONLY for durable O2; N trial stays 8, old plan stays 32',()=>{
 assert.match(n,/const managedMaxZonesN = 8/);
 assert.match(n,/if m\.MaxZones > 0/);
 assert.match(n,/Extended63/);
 assert.match(n,/len\(servers\)\+len\(zones\) > 64/);
 assert.match(o2,/MaxZones: 63/);
 assert.match(o2,/Extended63: true/);
 assert.match(o3,/managedZonesLimitedN\(cfg, 63\)/);
 assert.match(listener,/BuildDomainSplitPlanLWithLimit\(cfg, 32\)/);
});
test('P10 DSTest-only capability is not shadow auto-activation on fresh install',()=>{
 assert.match(init,/uname -n.*DSTest/);
 assert.match(init,/ndmc-durable-experimental/);
 assert.doesNotMatch(init,/--shadow-forward/);
 assert.match(main,/owned\.State == "active"/);
 assert.match(main,/enableShadow = true/);
 assert.match(api,/LifetimeEnabled/);
 assert.match(api,/ArmLifetimeO3B/);
 assert.match(main,/lifetimeOptIn && lifecycle.Status\(\).Active/);
});
