import test from 'node:test';import assert from 'node:assert/strict';
import {preflightView} from './NfqwsPreflightModel.js';
const rows=[{group:'Runtime',severity:'good',title:'Working',detail:'x'},
 {group:'Config',severity:'warn',title:'Check',detail:'y',tab:'config'},
 {group:'Lists',severity:'bad',title:'Missing',detail:'foo.list',tab:'lists'}];
test('preflight refuses READY when runtime has not been observed',()=>{
 const result=preflightView([],{});assert.equal(result.state,'BLOCKED');assert.equal(result.bad,1);
});
test('preflight shows BLOCKED when a bad gate is present, without mutating backend rows',()=>{
 const snapshot=structuredClone(rows);const result=preflightView(rows,{available:true});assert.equal(result.state,'BLOCKED');
 assert.equal(result.bad,1);assert.equal(result.warn,1);assert.equal(result.good,1);assert.deepEqual(rows,snapshot);
});
test('preflight CHECK and READY are evidence-based, not defaults',()=>{
 assert.equal(preflightView(rows.slice(0,2),{available:true}).state,'CHECK');
 assert.equal(preflightView(rows.slice(0,1),{available:true}).state,'READY');
});
test('preflight preserves exact diagnostics and registered navigation tabs as strings',()=>{
 const item=preflightView(rows,{available:true,dirty:true}).groups.find(x=>x.group==='Lists').items[0];
 assert.deepEqual(item,{severity:'bad',title:'Missing',detail:'foo.list',tab:'lists'});
});
test('preflight uses fixed group ordering, never treats unknown backend group as ready evidence',()=>{
 const result=preflightView([...rows,{group:'Alien',severity:'good',title:'No',detail:'x'}],{available:true});
 assert.deepEqual(result.groups.map(x=>x.group),['Runtime','Config','Lists']);
});
