import test from 'node:test';
import assert from 'node:assert/strict';
import {isMockOriginRequest,isKnownAdGuardInjection,redactNetworkUrl,auditBrowserBoundary} from './browser-network-boundary.mjs';
const origin='http://127.0.0.1:52560';
const adguard=[
 'http://local.adguard.org/?ts=1794395016314&type=content-script&dmn=127.0.0.1:52560',
 'http://local.adguard.org/?type=user-script&name=AdGuard%20Assistant',
 'http://local.adguard.org/adguard-ajax-api/api?type=gm-xml-http-request&unique_name=Web%20of%20Trust&key=SENSITIVE_TEST_TOKEN'
];
const event=(url,initiator={type:'script'})=>({method:'Network.requestWillBeSent',params:{request:{url},initiator}});
const blocked=url=>({url,reason:'BlockedByClient'});

test('RTC3-B1 exact localhost origin only; prefix and other ports are rejected',()=>{
 assert.equal(isMockOriginRequest(origin+'/v1/config',origin),true);
 for(const u of ['http://127.0.0.1:52561/v1/config','http://127.0.0.1:52560.evil.test/v1/config','http://localhost:52560/v1/config','https://127.0.0.1:52560/v1/config','http://192.168.10.1:52560/v1/config'])
  assert.equal(isMockOriginRequest(u,origin),false,u);
});
test('RTC3-B2 locally generated data/about/blob resources are accepted; external blob rejected',()=>{
 for(const u of ['about:blank','data:image/png;base64,AAAA','blob:http://127.0.0.1:52560/f00d']) assert.ok(isMockOriginRequest(u,origin),u);
 assert.equal(isMockOriginRequest('blob:https://bad.test/f00d',origin),false);
});
test('RTC3-B3 exact captured AdGuard injected URLs are classifiable as environment noise',()=>{
 for(const u of adguard)assert.equal(isKnownAdGuardInjection(u,{type:'script'},origin),true,u);
});
test('RTC3-B4 unrecognized AdGuard paths and external hosts never bypass gate',()=>{
 for(const u of ['http://local.adguard.org/svelte-mode.js','http://local.adguard.org/?type=anything','https://local.adguard.org/?type=user-script','https://rating.adguard.com/ping','http://bad.example/foo'])
  assert.equal(isKnownAdGuardInjection(u,{type:'script'},origin),false,u);
});
test('RTC3-B5 own Svelte bundle cannot disguise external request as AdGuard injection',()=>{
 const initiator={type:'script',stack:{callFrames:[{url:origin+'/v1/ui/svelte-mode.js',lineNumber:1}]}};
 assert.equal(isKnownAdGuardInjection(adguard[0],initiator,origin),false);
});
test('RTC3-B6 event requires corresponding pre-wire blocked request',()=>{
 const ev=[event(adguard[0])];
 const a=auditBrowserBoundary(ev,[],origin);
 assert.ok(a.bypassed.length===1);
 const b=auditBrowserBoundary(ev,[blocked(adguard[0])],origin);
 assert.deepEqual(b.bypassed,[]);assert.deepEqual(b.unknown,[]);assert.equal(b.environmentBlocked,1);
});
test('RTC3-B7 unexpected external blocked-but-attempted request still FAILs classification',()=>{
 const u='https://other.example/path?token=SENSITIVE_TEST_TOKEN';
 const a=auditBrowserBoundary([event(u)],[blocked(u)],origin);
 assert.deepEqual(a.bypassed,[]);assert.equal(a.unknown.length,1);
 assert.ok(!JSON.stringify(a).includes('SENSITIVE_TEST_TOKEN'));
});
test('RTC3-B8 diagnostic URL sanitizer hides sensitive query and giant data URLs',()=>{
 assert.ok(!redactNetworkUrl(adguard[2]).includes('SENSITIVE_TEST_TOKEN'));
 assert.equal(redactNetworkUrl('data:image/png;base64,'+'a'.repeat(10000)),'data:[redacted-inline-resource]');
});
