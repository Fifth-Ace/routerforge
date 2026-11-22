import test from 'node:test';
import assert from 'node:assert/strict';
import {startDnsLiteShellBridge} from './DnsLiteShellBridge.js';
test('DNS Lite shell bridge cleans up resize/theme observers on Svelte unmount',()=>{
  const events=new Map(),sizes=[],mutations=[],messages=[];
  class ResizeObserver{constructor(fn){this.fn=fn;sizes.push(this);}observe(){}disconnect(){this.off=true;}}
  class MutationObserver{constructor(fn){this.fn=fn;mutations.push(this);}observe(){}disconnect(){this.off=true;}}
  const root={style:{setProperty(){}}},panel={scrollHeight:412,getBoundingClientRect(){return {height:400}}};
  const parent={document:{documentElement:{}},getComputedStyle:()=>({getPropertyValue:()=>'',colorScheme:'light'}),postMessage:v=>messages.push(v)};
  const win={parent,location:{origin:'http://localhost'},ResizeObserver,MutationObserver,addEventListener:(n,fn)=>events.set(n,fn),removeEventListener:(n)=>events.delete(n)};
  const doc={documentElement:root,getElementById:id=>id==='dnsLiteApp'?panel:null};
  const bridge=startDnsLiteShellBridge(win,doc);
  assert.equal(events.has('resize'),true);
  bridge.reportHeight();assert.equal(messages[0].height,418);
  bridge.dispose();bridge.dispose();
  assert.equal(events.has('resize'),false);assert.equal(sizes[0].off,true);assert.equal(mutations[0].off,true);
  bridge.reportHeight();assert.equal(messages.length,1);
});
test('DNS Lite shell bridge does not create network requests or require iframe parent',()=>{
  const events=new Map();
  const doc={documentElement:{},getElementById:()=>({scrollHeight:200,getBoundingClientRect:()=>({height:200})})};
  const win={location:{origin:'http://localhost'},addEventListener:(x,fn)=>events.set(x,fn),removeEventListener:x=>events.delete(x)};
  win.parent=win;
  const bridge=startDnsLiteShellBridge(win,doc);bridge.reportHeight();bridge.dispose();
  assert.equal(events.size,0);
});
