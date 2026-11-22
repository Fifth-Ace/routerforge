import test from 'node:test';
import assert from 'node:assert/strict';
import {listInspectorView,listAnalysisView,removeIntersectingLines} from './NfqwsListToolsModel.js';

const entry=(file,line,value='example.org')=>({type:'domain',file,line,value,domain:value});
const finding=(a,b,kind='EXACT_DOMAIN',severity='warn',scope='domain')=>({kind,detail:'example.org',a,b,severity,scope});
const left=entry('user.list',2),right=entry('exclude.list',5),peer2=entry('auto.list',3);

test('inspector projects conflicts and file counts without mutating original',()=>{
  const rows=[finding(left,right,'EXACT_DOMAIN','conflict'),finding(left,peer2)];
  const before=structuredClone(rows);const view=listInspectorView(left,rows);
  assert.equal(view.total,2);assert.equal(view.conflicts,1);assert.equal(view.files,2);
  assert.deepEqual(rows,before);assert.deepEqual(view.rows.map(x=>x.file),['exclude.list','auto.list']);
});
test('inspector safely handles no selection and no findings',()=>{
  const view=listInspectorView(null,null);assert.equal(view.total,0);assert.equal(view.scope,'—');assert.equal(view.rows.length,0);
});
test('cross-list projection computes peer selection and exact source row numbers',()=>{
  const findings=[finding(left,right,'EXACT_DOMAIN','conflict'),finding(left,peer2,'DOMAIN_PARENT','warn')];
  const view=listAnalysisView({selected:'user.list',findings,peer:'exclude.list'});
  assert.deepEqual(view.peers,['auto.list','exclude.list']);assert.equal(view.peer,'exclude.list');
  assert.equal(view.total,2);assert.equal(view.conflicts,1);assert.equal(view.warnings,1);
  assert.deepEqual(view.left.map(x=>x.line),[2]);assert.deepEqual(view.right.map(x=>x.line),[5]);
});
test('filters limit visible rows but never change cleanup rows',()=>{
  const findings=[finding(left,right,'EXACT_DOMAIN','conflict'),finding(entry('user.list',4),entry('exclude.list',8),'CIDR_OVERLAP','warn','network')];
  const view=listAnalysisView({selected:'user.list',findings,peer:'exclude.list',filter:'domain'});
  assert.equal(view.left.length,1);assert.equal(view.allLeft.length,2);assert.equal(view.allRight.length,2);
  assert.equal(view.pairTotal,2);
});
test('unknown peer and invalid filter fail to known deterministic scope',()=>{
  const view=listAnalysisView({selected:'user.list',findings:[finding(left,right)],peer:'../../etc/passwd',filter:'evil'});
  assert.equal(view.peer,'exclude.list');assert.equal(view.filter,'all');assert.equal(view.pairTotal,1);
});
test('no phantom peers when list not in findings',()=>{
  const view=listAnalysisView({selected:'other.list',findings:[finding(left,right)]});
  assert.deepEqual(view.peers,[]);assert.equal(view.total,0);assert.equal(view.pairTotal,0);
});
test('multiple kinds on same file line retain one row and conflict severity',()=>{
  const findings=[finding(left,right,'DOMAIN_CHILD','warn'),finding(left,right,'EXACT_DOMAIN','conflict')];
  const view=listAnalysisView({selected:'user.list',findings,peer:'exclude.list'});
  assert.equal(view.right.length,1);assert.equal(view.right[0].severity,'conflict');
  assert.match(view.right[0].kinds,/EXACT_DOMAIN/);assert.match(view.right[0].kinds,/DOMAIN_CHILD/);
});
test('removeIntersectingLines preserves CRLF, blank lines, comment and trailing newline',()=>{
  const content='# Начало\r\nfoo.org\r\n\r\nbar.org\r\n';
  assert.equal(removeIntersectingLines(content,[2,4]),'# Начало\r\n\r\n');
  assert.equal(removeIntersectingLines(content,[]),content);
});
test('removeIntersectingLines fails closed for stale or invalid indexes',()=>{
  for(const x of [[0],[-1],[1.5],[5],['2'],[Number.NaN]])assert.throws(()=>removeIntersectingLines('a\nb\n',x));
  assert.throws(()=>removeIntersectingLines(null,[1]));
});
test('model keeps untrusted list text as plain string, no DOM or network',()=>{
  const hostile=entry('user.list',1,'<img src=x onerror=1>');
  assert.equal(listInspectorView(hostile,[]).value,hostile.value);
  assert.equal(typeof globalThis.document,'undefined');
});

test('malformed evidence cannot create phantom analysis matches',()=>{const v=listAnalysisView({selected:'user.list',findings:[null,{}, {a:left}]});assert.equal(v.total,0);assert.deepEqual(v.peers,[]);});
