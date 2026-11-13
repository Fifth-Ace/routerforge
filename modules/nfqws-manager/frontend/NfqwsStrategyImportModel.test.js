import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {normalizeImportedResult,normalizeImportName,importedArgTokens,importStrategyItems,confirmedStrategyImport,candidateFromImport,importDiff,defaultImportName,IMPORT_CONFIRM,CONFIG_CREATE_CONFIRM} from './NfqwsStrategyImportModel.js';
const active=`# system remains\nISP_INTERFACE="wan0"\nNFQWS_ARGS="--filter-tcp=443"\nNFQWS_ARGS_QUIC="--filter-udp=443"\nNFQWS_ARGS_UDP=""\nNFQWS_ARGS_IPSET=""\nNFQWS_ARGS_CUSTOM=""\nTCP_PORTS="80,443"\nUDP_PORTS="443"\nLOG_LEVEL="info"\n`;
const parsed=normalizeImportedResult({type:'batch',commands:['cmd'],profiles:['--filter-tcp=443 --hostlist=/opt/etc/nfqws2/lists/a.list','--filter-udp=443 --dpi-desync=fake'],tcp:'80,443',udp:'443',preview:'NFQWS_ARGS_CUSTOM=...',ready:true,transforms:[{kind:'WF',text:'ports'}],warnings:[],deps:[{kind:'LIST',path:'a.list'}]});
test('only server-verified READY can produce inactive library candidate',()=>{assert.equal(candidateFromImport({...parsed,ready:false},active).ok,false);assert.equal(candidateFromImport(parsed,'').ok,false)});
test('seven required canonical NFQWS assignments must exist, do not invent them',()=>{assert.equal(candidateFromImport(parsed,active.replace('NFQWS_ARGS_QUIC=', 'OTHER=')).ok,false)});
test('candidate replaces strategy slots and preserves system configuration',()=>{const result=candidateFromImport(parsed,active);assert.equal(result.ok,true);assert.match(result.content,/ISP_INTERFACE="wan0"/);assert.match(result.content,/LOG_LEVEL="info"/);assert.doesNotMatch(result.content,/NFQWS_ARGS="--filter-tcp=443"/);assert.match(result.content,/NFQWS_ARGS_CUSTOM="--filter-tcp=443/);assert.match(result.content,/--new/);assert.equal(active.includes('NFQWS_ARGS_CUSTOM=""'),true)});
test('candidate preserves original when no TCP or UDP override',()=>{const result=candidateFromImport({...parsed,tcp:'',udp:''},active);assert.match(result.content,/TCP_PORTS="80,443"/);assert.match(result.content,/UDP_PORTS="443"/)});
test('candidate rejects no profiles and missing keys',()=>{assert.equal(candidateFromImport({...parsed,profiles:[]},active).ok,false);assert.equal(candidateFromImport(parsed,'').ok,false)});
test('normalizer rejects malformed backend shape',()=>{assert.throws(()=>normalizeImportedResult(null));assert.throws(()=>normalizeImportedResult({profiles:'bad'}));assert.throws(()=>normalizeImportedResult({warnings:{}}));});
test('allow only normalized local inactive .conf names',()=>{for(const bad of ['','../evil','a/b','evil;touch','.conf','ab|cd','../../config.conf'])assert.equal(normalizeImportName(bad),'');assert.equal(normalizeImportName('candidate'),'candidate.conf');assert.equal(normalizeImportName('MyConfig.conf'),'MyConfig.conf')});
test('imported argv keeps shell-quoted argument boundaries',()=>{const args=importedArgTokens('--filter-tcp=443 --dpi-desync-fake-tls="foo bar" --hostlist=/opt/abc.list');assert.deepEqual(args,['--filter-tcp=443','--dpi-desync-fake-tls="foo bar"','--hostlist=/opt/abc.list']);});
test('library strategy import holds original profiles order and explicit token',()=>{const items=importStrategyItems(parsed);assert.equal(items.length,2);assert.equal(items[0].name,'Импортированный профиль 1');assert.equal(IMPORT_CONFIRM,'ROUTERFORGE_V2_STRATEGY_IMPORT');assert.equal(CONFIG_CREATE_CONFIRM,'NFQWS_CONFIG_LIBRARY_CREATE')});
test('library strategy import refuses stale empty profile',()=>{assert.throws(()=>importStrategyItems({...parsed,profiles:['']}));assert.throws(()=>importStrategyItems({...parsed,ready:false}))});
test('diff retains original plus changes as plain text',()=>{const rows=importDiff('a\nb','a\nc');assert.deepEqual(rows.map(r=>r.kind),['same','del','add']);assert.equal(rows[2].text,'+ c')});
test('default name is deterministic from supplied local datetime',()=>{assert.equal(defaultImportName(new Date(2026,10,12,1,15)),'strategy-import-20261112-0115.conf')});
test('Svelte owns all strategy-import handlers; legacy does not write view DOM',()=>{const runtime=readFileSync(new URL('./NfqwsRuntime.js',import.meta.url),'utf8'),app=readFileSync(new URL('./NFQWSManagerApp.svelte',import.meta.url),'utf8'),native=readFileSync(new URL('./NfqwsStrategyImport.svelte',import.meta.url),'utf8');assert.doesNotMatch(runtime,/\$\('strategyImport(?:Input|Run|LibrarySave|StrategyImport|SandboxOpen)'\)/);assert.match(app,/<NfqwsStrategyImport/);assert.match(native,/data-svelte-owned="nfqws-strategy-import"/);assert.doesNotMatch(native,/<script[^>]*src=/);});
test('mutations require direct clicks, explicit user confirmation and status SHA preflight',()=>{const source=readFileSync(new URL('./NfqwsStrategyImport.svelte',import.meta.url),'utf8');assert.match(source,/if\(!globalThis\.confirm\(/);assert.match(source,/latest\?\.config_sha256/);assert.match(source,/onRequest\('\/configs\/create'/);assert.match(source,/onRequest\('\/v2\/strategies\/import'/);assert.match(source,/onRequest\('\/v2\/import\/parse'/);assert.match(source,/requestAbort\?\.abort\(\)/)});

test('server import ack is authoritative before invalidating native registry and library',()=>{
  const valid={ok:true,added:2,existing:0,count:7,strategies:[{id:'s-a'},{id:'s-b'}],source:'import',production_mutation:false};
  assert.deepEqual(confirmedStrategyImport(valid,2),{added:2,existing:0,count:7});
  assert.deepEqual(confirmedStrategyImport({...valid,added:1,existing:0,strategies:[{id:'s-a'}]},2),{added:1,existing:0,count:7}); // request may contain duplicate fingerprints
  for(const patch of [{ok:false},{production_mutation:true},{source:'external'},{added:-1},{added:3},{existing:1},{count:0},{strategies:[]},{strategies:[{}]}]){
    assert.throws(()=>confirmedStrategyImport({...valid,...patch},2),/NFQWS_IMPORT_/);
  }
  assert.throws(()=>confirmedStrategyImport(null,2),/NFQWS_IMPORT_ACK_INVALID/);
});
test('confirmed import triggers both native Svelte data owners, never the legacy controller',()=>{
  const native=readFileSync(new URL('./NfqwsStrategyImport.svelte',import.meta.url),'utf8');
  const app=readFileSync(new URL('./NFQWSManagerApp.svelte',import.meta.url),'utf8');
  const registry=readFileSync(new URL('./NfqwsStrategyRegistry.svelte',import.meta.url),'utf8');
  const library=readFileSync(new URL('./NfqwsStrategyLibrary.svelte',import.meta.url),'utf8');
  const legacy=readFileSync(new URL('./NfqwsRuntime.js',import.meta.url),'utf8');
  assert.match(native,/confirmedStrategyImport\(result,items\.length\);[\s\S]*onRegistryInvalidate\(\)/);
  assert.match(app,/onRegistryInvalidate=\{importedStrategiesChanged\}/);
  assert.match(app,/function importedStrategiesChanged\(\)\{[\s\S]*registryRefreshSeq\+=1;[\s\S]*strategyLibraryRefreshSeq\+=1;/);
  assert.match(app,/<NfqwsStrategyRegistry active=.+?refreshSeq=\{registryRefreshSeq\}/);
  assert.match(app,/<NfqwsStrategyLibrary active=.+?refreshSeq=\{strategyLibraryRefreshSeq\}/);
  assert.match(registry,/\$effect\(\(\)=>\{if\(refreshSeq>lastRefreshSeq\)/);
  assert.match(library,/appliedRefreshSeq!==refreshSeq/);
  assert.doesNotMatch(legacy,/onRegistryInvalidate/);
});
