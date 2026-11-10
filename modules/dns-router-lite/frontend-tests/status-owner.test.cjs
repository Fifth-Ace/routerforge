'use strict';
const test=require('node:test');
const assert=require('node:assert/strict');
const {fixture,until}=require('./fixture.cjs');
const mutation=x=>!['GET','HEAD'].includes(x.opts?.method||'GET');

test('Svelte status owner preserves DOM IDs, read-only boot and accurate stopped state',async()=>{
  const f=fixture(),d=f.w.document;
  try {
    await until(()=>d.getElementById('serviceBadge')?.textContent==='DNS остановлен' && d.getElementById('healthServers')?.textContent==='1/2');
    for(const id of ['serviceBadge','healthGate','healthRequests','healthServers','healthP95','gateBadge','gateCompactBadge','gateAdvice','routeAdvice','runStatus','metricRequests','logCounter','logVisible','liveBadge']) {
      assert.equal(d.querySelectorAll('#'+id).length,1,'unique status '+id);
    }
    d.getElementById('navLogs').click();
    await until(()=>d.getElementById('logUpdated').textContent.includes('Обновлено'));
    assert.equal(d.getElementById('liveBadge').textContent,'LIVE · DNS остановлен');
    assert.equal(d.getElementById('metricRunning').textContent,'Выкл.');
    assert.equal(d.getElementById('gateBadge').textContent,'Выключен');
    assert.deepEqual(f.calls.filter(mutation),[]);
    assert.deepEqual(f.errors,[]);
  } finally {f.w.close();}
});

test('Svelte live badge distinguishes running listener with no requests from actual requests',async()=>{
  const f=fixture({active:true,requests:0}),d=f.w.document;
  try {
    await until(()=>d.getElementById('liveBadge').textContent==='LIVE · нет запросов');
    assert.equal(d.getElementById('serviceBadge').textContent,'DNS работает');
    assert.equal(d.getElementById('metricRunning').textContent,'Работает');
    f.setRequests(4);
    d.getElementById('navLogs').click();
    d.getElementById('refreshLogs').click();
    await until(()=>d.getElementById('liveBadge').textContent==='LIVE · запросы поступают');
    assert.match(d.getElementById('routeAdvice').textContent,/4 запросов/);
    assert.deepEqual(f.calls.filter(mutation),[]);
    assert.deepEqual(f.errors,[]);
  } finally {f.w.close();}
});

test('Gate recovery status is reactive but does not activate Gate',async()=>{
  const f=fixture({active:true}),d=f.w.document;
  try {
    await until(()=>d.getElementById('serviceBadge').textContent==='DNS работает');
    f.setGate({state:'RECOVERY_REQUIRED',active:false,desired:true,last_error:'xtables lock',lease_seconds:60});
    d.getElementById('navLogs').click();
    d.getElementById('refreshLogs').click();
    await until(()=>d.getElementById('gateBadge').textContent==='Требуется проверка');
    assert.equal(d.getElementById('gateCompactBadge').textContent,'Требуется проверка');
    assert.equal(d.getElementById('systemGatePanel').dataset.state,'recovery');
    assert.match(d.getElementById('gateAdvice').textContent,/xtables lock/);
    assert.equal(d.getElementById('gateEnable').disabled,true);
    assert.deepEqual(f.calls.filter(x=>x.path==='service/gate/control'),[]);
    assert.deepEqual(f.errors,[]);
  } finally {f.w.close();}
});
