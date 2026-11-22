import {test} from 'node:test';
import assert from 'node:assert/strict';
import {createLiteSettingsFields,hydrateLiteSettings,buildLiteServicePrefs} from './DnsLiteSettingsModel.js';
const config={upstreams:[{id:'default',transport:'udp',address:'9.9.9.9:53'}],lists:[],service:{default_upstream:'default',listen:'192.168.10.1:55358',allowed_networks:['192.168.10.0/24'],cache_bytes:1048576}};
test('P33 opt-in fields preserve prior configurations',()=>{
 const fields=createLiteSettingsFields();hydrateLiteSettings(fields,config);
 assert.equal(fields.p33DeviceCache,false);
 assert.equal(buildLiteServicePrefs(fields,config).performance,undefined);
});
test('P33 optimistic limits and policy cache serialized safely',()=>{
 const fields=createLiteSettingsFields();hydrateLiteSettings(fields,config);
 fields.p29Enabled=true;fields.p29Optimistic=true;fields.p33MaxAge=120;fields.p33AnswerTTL=10;fields.p33DeviceCache=true;
 const prefs=buildLiteServicePrefs(fields,config);
 assert.equal(prefs.performance.optimistic_max_age_seconds,120);
 assert.equal(prefs.performance.optimistic_answer_ttl_seconds,10);
 assert.equal(prefs.performance.device_policy_cache,true);
 fields.p29Optimistic=false;
 assert.throws(()=>buildLiteServicePrefs(fields,config),/P33/);
});
