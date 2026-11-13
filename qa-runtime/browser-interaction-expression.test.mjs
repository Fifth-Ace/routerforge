import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {pointExpression} from './browser-interaction-expression.mjs';

function createBrowser(options={}){
 const parent={parentElement:null,hidden:Boolean(options.parentHidden),getAttribute:()=>options.parentAriaHidden?'true':null};
 const element={
  parentElement: parent,
  hidden:Boolean(options.hidden),
  getAttribute:()=> options.ariaHidden?'true':null,
  matches:()=>Boolean(options.disabled),
  scrollIntoView(){this.scrolled=true},
  getBoundingClientRect:()=>options.rect||{left:10,top:20,width:80,height:30,right:90,bottom:50},
  contains:node=>node===element||Boolean(options.containsChild)
 };
 const context={
  document:{querySelector:()=>options.missing?null:element,elementFromPoint:()=>options.obscured?{}:element},
  getComputedStyle:node=>({display:node===parent&&options.parentDisplayNone?'none':'block',visibility:'visible'}),
  requestAnimationFrame:callback=>callback(),
  innerWidth:1280,innerHeight:720,Error,Boolean,Math,Promise,
 };
 return {context,element};
}
async function evaluate(selector,disabled,options={}){
 const browser=createBrowser(options);
 const source=pointExpression(selector,disabled);
 new vm.Script(source,{filename:'generated-cdp-point.js'});
 return {result:await vm.runInNewContext(source,browser.context),element:browser.element};
}

test('RTC3-I01 both generated CDP expressions compile as JavaScript before Chromium',()=>{
 for(const disabled of [false,true])new vm.Script(pointExpression('#testVisible',disabled));
});
test('RTC3-I02 selector quoting survives apostrophes, quotes and backslashes',async()=>{
 const selector=`[data-test="it's \\ visible"]`;
 const {result}=await evaluate(selector,false);
 assert.deepEqual({x:result.x,y:result.y},{x:50,y:35});
});
test('RTC3-I03 visible enabled element receives an actual viewport point',async()=>{
 const {result,element}=await evaluate('#visible',false);
 assert.deepEqual({x:result.x,y:result.y},{x:50,y:35});
 assert.equal(element.scrolled,true);
});
test('RTC3-I04 disabled element fails closed with distinct error',async()=>{
 await assert.rejects(()=>evaluate('#disabled',false,{disabled:true}),/ELEMENT_DISABLED/);
});
test('RTC3-I05 explicitly expected disabled element is allowed but enabled one is rejected',async()=>{
 await evaluate('#disabled',true,{disabled:true});
 await assert.rejects(()=>evaluate('#enabled',true),/ELEMENT_NOT_DISABLED/);
});
test('RTC3-I06 no element is never a synthetic click PASS',async()=>{
 await assert.rejects(()=>evaluate('#missing',false,{missing:true}),/ELEMENT_MISSING/);
});
test('RTC3-I07 hidden element or hidden parent is never clickable',async()=>{
 for(const options of [{hidden:true},{ariaHidden:true},{parentHidden:true},{parentAriaHidden:true},{parentDisplayNone:true}]){
  await assert.rejects(()=>evaluate('#hidden',false,options),/ELEMENT_HIDDEN/);
 }
});
test('RTC3-I08 offscreen and obscured elements are rejected',async()=>{
 await assert.rejects(()=>evaluate('#far',false,{rect:{left:1800,right:1900,top:30,bottom:60,width:100,height:30}}),/ELEMENT_OUTSIDE_VIEWPORT/);
 await assert.rejects(()=>evaluate('#covered',false,{obscured:true}),/ELEMENT_OBSCURED/);
});
test('RTC3-I09 invalid selector/expectation inputs fail before browser evaluation',()=>{
 assert.throws(()=>pointExpression(''),/CDP_SELECTOR_INVALID/);
 assert.throws(()=>pointExpression('#ok','false'),/CDP_DISABLED_EXPECTATION_INVALID/);
});
