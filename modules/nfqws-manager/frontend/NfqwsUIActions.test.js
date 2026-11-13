import test from 'node:test';
import assert from 'node:assert/strict';
import {setupAccordion,workspaceHeight} from './NfqwsUIActions.js';

function virtualSection({fixed=false,withBody=true}={}) {
  const attributes=new Map(),events=new Map();
  const head={attributes,events,tabIndex:-1,
    setAttribute(k,v){attributes.set(k,v)},removeAttribute(k){attributes.delete(k)},
    addEventListener(k,f){if(events.has(k))throw Error('DUPLICATE_UI_LISTENER');events.set(k,f)},
    removeEventListener(k,f){assert.equal(events.get(k),f);events.delete(k)},
  };
  const body={parentNode:null};
  const section={dataset:{fixed:String(fixed)},classes:new Set(),
    classList:{add(...keys){keys.forEach(k=>section.classes.add(k))},remove(...keys){keys.forEach(k=>section.classes.delete(k))}},
    querySelector(k){return k.includes('setup-head')?head:withBody?body:null}
  };
  const wrapper={hidden:false,classes:new Set(),offsetHeight:100,parentNode:null,
    classList:{add(...keys){keys.forEach(k=>wrapper.classes.add(k))},remove(...keys){keys.forEach(k=>wrapper.classes.delete(k))}},
    append(inner){inner.parentNode=wrapper},remove(){wrapper.parentNode=null}};
  const inner={append(child){body.parentNode=inner}};
  const parent={insertBefore(item){item.parentNode=parent}};
  body.parentNode=parent;
  return {head,body,section,wrapper,inner,parent};
}

test('accordion avoids duplicate writers for fixed sections, no timers or events',()=>{
  const item=virtualSection({fixed:true});
  const action=setupAccordion(item.section,0);
  assert.equal(item.head.events.size,0);
  action.destroy();
});

test('accordion ignores invalid markup and keeps keyboard event ownership separate from nested buttons',()=>{
  const item=virtualSection({withBody:false});
  const action=setupAccordion(item.section,1);
  assert.equal(item.head.events.size,0);action.destroy();
});

test('workspace action requires exact known IDs and has no router or fetch capability',()=>{
  assert.throws(()=>workspaceHeight({id:'../bad'}),/NFQWS_WORKSPACE_UNKNOWN/);
  assert.throws(()=>workspaceHeight({id:'geoAssets'}),/NFQWS_WORKSPACE_UNKNOWN/);
});

test('setup accordion keyboard guard avoids nested-control toggles and unregisters listeners',()=>{
  const source=setupAccordion.toString();
  assert.match(source,/e\.target!==head/);
  assert.match(source,/removeEventListener\('click',click\)/);
  assert.match(source,/removeEventListener\('keydown',keydown\)/);
  assert.match(source,/clearTimeout\(timer\)/);
});

test('actual accordion lifecycle: initial closed, keyboard nested ignores, open, destroy releases DOM and events',()=>{
  const item=virtualSection(),beforeDoc=globalThis.document,beforeWindow=globalThis.window;
  const attrs=new Map(),classes=new Set();
  const create=()=>({parentNode:null,hidden:false,offsetHeight:10,className:'',
    classList:{add(...x){x.forEach(k=>classes.add(k))},remove(...x){x.forEach(k=>classes.delete(k))}},
    append(x){x.parentNode=this;},remove(){this.parentNode=null;}});
  globalThis.document={createElement:create};
  let resizeEvents=0;globalThis.window={dispatchEvent:()=>{resizeEvents++}};
  try{
    const action=setupAccordion(item.section,6);
    assert.equal(item.section.classes.has('collapsed'),true);
    assert.equal(item.head.attributes.get('aria-expanded'),'false');
    assert.equal(item.head.events.size,2);
    const nested={key:'Enter',target:{},preventDefault(){throw Error('NESTED_KEY_INTERCEPTED')}};
    item.head.events.get('keydown')(nested);
    assert.equal(item.section.classes.has('collapsed'),true);
    const target=item.head;
    let prevented=false;
    item.head.events.get('keydown')({key:'Enter',target,preventDefault(){prevented=true}});
    assert.equal(prevented,true);
    assert.equal(item.section.classes.has('collapsed'),false);
    assert.equal(item.head.attributes.get('aria-expanded'),'true');
    assert.ok(resizeEvents>=1);
    action.destroy();
    assert.equal(item.head.events.size,0);
    assert.equal(item.head.attributes.has('aria-expanded'),false);
    assert.equal(item.section.classes.has('collapsible-section'),false);
  }finally{globalThis.document=beforeDoc;globalThis.window=beforeWindow;}
});
