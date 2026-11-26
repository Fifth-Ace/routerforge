import test from 'node:test';
import assert from 'node:assert/strict';
import {protectedEvidence,plainEvidence,evidenceCounts,qualityText} from './dnsHealthEvidence.js';
const now=Date.parse('2026-11-26T11:00:00Z'), stamp=ms=>new Date(now+ms).toISOString(), minute=now/60000;
test('active or old UP is not fresh health evidence',()=>{
  assert.equal(protectedEvidence({active:true,health_status:'UP'},now),'unknown');
  assert.equal(protectedEvidence({health_status:'UP',last_health_check:stamp(-181000)},now),'unknown');
  assert.equal(protectedEvidence({health_status:'UP',last_observed:stamp(1000)},now),'unknown');
  assert.equal(protectedEvidence({health_status:'UP',last_health_check:stamp(-1000)},now),'healthy');
  assert.equal(protectedEvidence({health_status:'DOWN',last_health_check:stamp(-1000)},now),'problem');
});
test('plain recent completed outcomes only; no false healthy from pending or failure',()=>{
  const row=(responses,errors,timeouts,m=minute)=>({health_buckets:[{minute:m,responses,errors,timeouts,requests:10}]});
  assert.equal(plainEvidence(row(0,0,0),{},now,5),'unknown');
  assert.equal(plainEvidence(row(1,1,0),{},now,5),'unknown');
  assert.equal(plainEvidence(row(0,0,2),{cls:'error'},now,5),'problem');
  assert.equal(plainEvidence(row(1,0,0),{},now,5),'healthy');
  assert.equal(plainEvidence(row(1,0,0,minute-5),{},now,5),'unknown');
});
test('partial data never becomes healthy by subtraction',()=>{
  const protectedRows=[{health_status:'UP',last_observed:stamp(-1000)}];
  const plainRows=[{health_buckets:[{minute,responses:1,errors:0}]}];
  assert.deepEqual(evidenceCounts(protectedRows,plainRows,[{}],now,5,true,false),{healthy:1,problem:0,unknown:1});
  assert.deepEqual(evidenceCounts(protectedRows,plainRows,[{}],now,5,false,false),{healthy:0,problem:0,unknown:2});
});
test('no samples or malformed percent never renders 100%',()=>{
  assert.equal(qualityText({quality_pct:100},'en'),'no data');
  assert.equal(qualityText({success:1,quality_pct:100},'en'),'100.0%');
  assert.equal(qualityText({timeouts:2,quality_pct:0}),'0.0%');
  assert.equal(qualityText({success:1,quality_pct:null}),'нет данных');
});
