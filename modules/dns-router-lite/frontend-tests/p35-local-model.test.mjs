import test from 'node:test';
import assert from 'node:assert/strict';
import {parseP35Records,formatP35Records} from '../frontend/DnsP35LocalModel.js';
test('P35 editor: records, roundtrip, default inert',()=>{
  const rows=parseP35Records('nas.rf.home.arpa | A | 192.168.10.8 | 300\nalias.rf.home.arpa | CNAME | nas.rf.home.arpa | 300');
  assert.equal(rows.length,2);
  assert.deepEqual(parseP35Records(formatP35Records(rows)),rows);
  assert.deepEqual(parseP35Records(''),[]);
});
test('P35 rejects stock names, records count, invalid TTL and duplicates',()=>{
  for(const src of ['router.lan | A | 192.168.10.1 | 300','nas.home.arpa | A | 192.168.10.2 | 300','nas.rf.home.arpa | A | 192.168.10.2 | 0','nas.rf.home.arpa | A | 192.168.10.2 | 300\nnas.rf.home.arpa | A | 192.168.10.3 | 300'])assert.throws(()=>parseP35Records(src));
});
