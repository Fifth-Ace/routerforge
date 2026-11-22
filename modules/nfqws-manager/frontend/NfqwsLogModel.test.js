import test from 'node:test';
import assert from 'node:assert/strict';
import {nfqwsLogModel,nfqwsLogTokens} from './NfqwsLogModel.js';

test('NFQWS log parse/filter/statistics preserve original severity classes',()=>{
  const src=[
    '01.11.2026 10:00:00 : example.com : profile default : client 192.0.2.2 : proto QUIC : started',
    '01.11.2026 10:00:01 : other.com : profile a : client 192.0.2.3 : proto UDP : ERROR failed',
    '01.11.2026 10:00:02 : third.com : profile a : client 192.0.2.4 : proto TCP : fail counter 2'
  ].join('\n');
  const all=nfqwsLogModel(src);assert.equal(all.stats.rows,3);
  assert.equal(all.stats.quic,1);assert.equal(all.stats.tcp,1);assert.equal(all.stats.udp,1);
  assert.equal(all.stats.errors,1);assert.equal(all.stats.fails,1);
  assert.equal(nfqwsLogModel(src,'other','errors').rows.length,1);
  assert.equal(nfqwsLogModel(src,'example','warnings').rows.length,0);
});

test('raw lines remain escaped by Svelte and tokenization never returns HTML',()=>{
  const original='<script>alert(1)</script> timeout 300';
  const m=nfqwsLogModel(original);
  assert.equal(m.rows[0].raw,original);assert.equal(m.rows[0].time,'');
  const tokens=nfqwsLogTokens(original);
  assert.equal(tokens.map(x=>x.text).join(''),original);
  assert(tokens.some(x=>x.kind==='log-token-key'));
  assert(tokens.every(x=>!('html' in x)));
});
