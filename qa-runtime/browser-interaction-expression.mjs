// Test-only CDP interaction source. Generates browser code from JSON-quoted
// selectors/messages and is independently syntax-checked by Node before Chrome.
// Never imported by any RouterForge production frontend or API.
export function pointExpression(selector, expectDisabled=false) {
  if (typeof selector !== 'string' || !selector.trim()) throw Error('CDP_SELECTOR_INVALID');
  if (typeof expectDisabled !== 'boolean') throw Error('CDP_DISABLED_EXPECTATION_INVALID');
  const selectorJS=JSON.stringify(selector);
  const disabledMessageJS=JSON.stringify(expectDisabled?'ELEMENT_NOT_DISABLED: ':'ELEMENT_DISABLED: ');
  return `(async()=>{
    const e=document.querySelector(${selectorJS});
    if(!e)throw Error('ELEMENT_MISSING: '+${selectorJS});
    for(let n=e;n;n=n.parentElement){
      const style=getComputedStyle(n);
      if(n.hidden||n.getAttribute('aria-hidden')==='true'||style.display==='none'||style.visibility==='hidden')
        throw Error('ELEMENT_HIDDEN: '+${selectorJS});
    }
    if(Boolean(e.matches(':disabled'))!==${expectDisabled})
      throw Error(${disabledMessageJS}+${selectorJS});
    e.scrollIntoView({block:'center',inline:'nearest'});
    await new Promise(resolve=>requestAnimationFrame(resolve));
    const r=e.getBoundingClientRect();
    if(!r.width||!r.height||r.left>=innerWidth||r.right<=0||r.top>=innerHeight||r.bottom<=0)
      throw Error('ELEMENT_OUTSIDE_VIEWPORT: '+${selectorJS});
    const x=Math.max(0,Math.min(innerWidth-1,r.left+r.width/2));
    const y=Math.max(0,Math.min(innerHeight-1,r.top+r.height/2));
    const at=document.elementFromPoint(x,y);
    if(!at || !(at===e || e.contains(at)))throw Error('ELEMENT_OBSCURED: '+${selectorJS});
    return {x,y};
  })()`;
}
