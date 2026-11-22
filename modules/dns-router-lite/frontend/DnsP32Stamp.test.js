import assert from 'node:assert/strict';
import {test} from 'node:test';
import {parseP32Stamp} from './DnsP32Stamp.js';
test('P32 parses pinned DNSCrypt stamps offline and rejects missing pins',()=>{
 const b=Buffer.concat([Buffer.from([1,0,0,0,0,0,0,0,0,11]),Buffer.from('9.9.9.9:443'),Buffer.from([1,120])]);
 const s='sdns://'+b.toString('base64url');
 assert.equal(parseP32Stamp(s).address,'9.9.9.9:443');
 assert.throws(()=>parseP32Stamp('sdns://AAAA'));
});
