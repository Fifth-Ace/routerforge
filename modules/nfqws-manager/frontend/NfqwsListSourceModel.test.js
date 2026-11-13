import test from 'node:test';
import assert from 'node:assert/strict';
import {nfqwsListSources} from './NfqwsListSourceModel.js';

test('NFQWS list-source view compares exact SHA, never guesses list availability',()=>{
 const local=[{name:'a.list',sha256:'abcd',size:16,referenced:true},{name:'b.list',sha256:'bbbb',size:32,referenced:false}];
 const upstream=[{name:'a.list',sha256:'ABCD',path:'strategies/lists/a.list'},{name:'b.list',sha256:'ffff',path:'strategies/lists/b.list'},{name:'c.list',sha256:'cccc',path:'strategies/lists/c.list'}];
 const view=nfqwsListSources({local,upstream,ref:'a'.repeat(40),resolution:{state:'fresh',source:'GitHub'}});
 assert.deepEqual(view.upstream.map(x=>x.installState),['ok','update','missing']);
 assert.equal(view.changedCount,2);assert.equal(view.localCount,2);assert.equal(view.referencedCount,1);assert.equal(view.state,'SYNCED');
 assert.match(view.upstreamMeta,/exact ref aaaaaaaaaaaa/);
 assert.equal(local[0].sha256,'abcd');
});
test('NFQWS list-source gracefully degrades to local inventory when upstream rate limited',()=>{
 const view=nfqwsListSources({local:[{name:'mine.list'}],upstream:[],error:'403',resolution:{state:'rate_limited'}});
 assert.equal(view.state,'RATE LIMITED');assert.equal(view.localCount,1);assert.equal(view.changedCount,0);
});
test('NFQWS list-source never claims current when sha is missing',()=>{
 const view=nfqwsListSources({local:[{name:'a.list'}],upstream:[{name:'a.list',sha256:'sha'}]});
 assert.equal(view.upstream[0].installState,'update');
});
