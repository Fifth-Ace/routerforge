// Svelte lifecycle owns host iframe measurement and theme synchronization.
// This is not a DOM form/controller bridge: no API calls, event delegation or
// mutation of RouterForge DNS state are performed here.
export function startDnsLiteShellBridge(win=window, doc=document) {
  const root=doc.documentElement;
  const panel=doc.getElementById('dnsLiteApp');
  if(!panel)throw Error('DNS_LITE_SHELL_MISSING');
  let stopped=false;
  const tokens=['--rf-bg','--rf-surface','--rf-surface-2','--rf-text','--rf-muted','--rf-border','--rf-border-strong','--rf-hover','--rf-accent','--rf-accent-soft','--rf-success','--rf-danger','--rf-radius-panel','--rf-radius-control','--rf-ui-panel-radius','--rf-ui-control-radius','--rf-ui-control-h'];
  function syncTheme(){
    if(stopped || win.parent===win)return;
    try{
      const host=win.parent.document.documentElement;
      const style=win.parent.getComputedStyle(host);
      for(const key of tokens){const value=style.getPropertyValue(key).trim();if(value)root.style.setProperty(key,value);}
      root.style.colorScheme=style.colorScheme==='light'?'light':'dark';
    }catch(_){} // Cross-origin parents cannot expose theme.
  }
  function reportHeight(){
    if(stopped||win.parent===win)return;
    try{
      const height=Math.ceil(Math.max(350,panel.scrollHeight,panel.getBoundingClientRect().height)+6);
      win.parent.postMessage({type:'routerforge-module-height',moduleId:'dns-router',height},win.location.origin);
    }catch(_){}
  }
  syncTheme();
  let parentObserver=null;
  let sizeObserver=null;
  try{
    if(win.parent!==win && win.MutationObserver){
      parentObserver=new win.MutationObserver(()=>{syncTheme();reportHeight();});
      parentObserver.observe(win.parent.document.documentElement,{attributes:true,attributeFilter:['style','class','data-theme']});
    }
  }catch(_){parentObserver?.disconnect();parentObserver=null;}
  if(win.ResizeObserver){sizeObserver=new win.ResizeObserver(reportHeight);sizeObserver.observe(panel);}
  win.addEventListener('resize',reportHeight);
  return {reportHeight,dispose(){if(stopped)return;stopped=true;parentObserver?.disconnect();sizeObserver?.disconnect();win.removeEventListener('resize',reportHeight);}};
}
