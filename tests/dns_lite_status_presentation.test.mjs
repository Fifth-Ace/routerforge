import test from 'node:test';
import assert from 'node:assert/strict';
import {dnsLiveBadge} from '../modules/dns-router-lite/frontend/DnsLiteStatusModel.js';

test('unloaded status does not claim working DNS',()=>{
  assert.deepEqual(dnsLiveBadge(null),{text:'LIVE · ожидание',tone:''});
});
test('stopped service remains stopped after a successful log poll',()=>{
  assert.deepEqual(dnsLiveBadge({active:false,requests:150}),{text:'LIVE · DNS остановлен',tone:'bad'});
});
test('active listener with no traffic cannot claim requests or client E2E',()=>{
  assert.deepEqual(dnsLiveBadge({active:true,requests:0}),{text:'LIVE · нет запросов',tone:'good'});
});
test('observed RouterForge requests are not labeled a verified client route',()=>{
  assert.deepEqual(dnsLiveBadge({active:true,requests:7}),{text:'LIVE · запросы поступают',tone:'good'});
});
test('paused status is explicit, even if DNS is no longer active',()=>{
  assert.deepEqual(dnsLiveBadge({active:false,requests:1},true),{text:'ПАУЗА · снимок журнала',tone:'bad'});
});
