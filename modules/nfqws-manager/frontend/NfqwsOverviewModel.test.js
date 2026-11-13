import test from 'node:test';import assert from 'node:assert/strict';
import {nfqwsOverview,nfqwsSize} from './NfqwsOverviewModel.js';

test('read-only overview status is projected without mutation',()=>{
  const s={available:true,running:true,mutation_ready:true,config_exists:true,config_size:4096,
    config_sha256:'c'.repeat(64),config_path:'/opt/etc/nfqws2/nfqws2.conf',target_name:'NFQWS',
    target_id:'nfqws',lists:[{name:'a',size:123},{name:'b',size:456}],extra:{field:1}};
  const before=structuredClone(s),o=nfqwsOverview(s);
  assert.deepEqual(s,before);assert.equal(o.running,true);assert.equal(o.ready,true);
  assert.equal(o.listCount,2);assert.equal(o.activity.length,5);
  assert.equal(o.activity.filter(x=>x.ok).length,5);assert.equal(o.configSHA,'c'.repeat(64));
});
test('unavailable, missing or partial backend never shows ready',()=>{
  const o=nfqwsOverview(null);assert.equal(o.ready,false);assert.equal(o.running,false);
  assert.equal(o.activity[0].ok,false);assert.equal(o.activity[4].ok,false);
  const partial=nfqwsOverview({available:true,config_truncated:true,mutation_ready:false});
  assert.equal(partial.activity[3].ok,false);assert.equal(partial.mutation,'not ready');
});
test('untrusted backend text stays ordinary string for Svelte escaping',()=>{
  const bad='<img src=x onerror=alert(1)>',o=nfqwsOverview({target_name:bad,config_path:bad});
  assert.equal(o.target.includes(bad),true);assert.equal(o.configPath,bad);
  assert.equal(o.activity.length,5);
});
test('size formatting preserves existing NFQWS behavior and refuses invalid numbers',()=>{
  assert.equal(nfqwsSize(1024),'1.0 KB');assert.equal(nfqwsSize(1048576),'1.0 MB');
  assert.equal(nfqwsSize(-5),'—');
});
