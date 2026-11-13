// Svelte-owned DOM actions. Not a parallel NFQWS business controller.
// Only UI state/layout and local workspace size; no API, service writes or autostart.
const STORAGE = Object.freeze({
  listWorkspace: 'routerforge.nfqws.workspace.lists.height',
  scriptWorkspace: 'routerforge.nfqws.workspace.scripts.height'
});
export function workspaceHeight(node) {
  const key = STORAGE[node?.id];
  if (!key) throw Error('NFQWS_WORKSPACE_UNKNOWN');
  let saved=0;
  try { saved=Number(localStorage.getItem(key)||0); } catch (_) {}
  if (saved>=420 && saved<=900) node.style.height=`${Math.round(saved)}px`;
  let timer=0;
  const observer=typeof ResizeObserver==='function' ? new ResizeObserver(()=>{
    const h=Math.round(node.getBoundingClientRect().height);
    if (h<420 || h>900) return;
    clearTimeout(timer);
    timer=setTimeout(()=>{try{localStorage.setItem(key,String(h));}catch(_){}},120);
  }) : null;
  observer?.observe(node);
  return {destroy(){observer?.disconnect();clearTimeout(timer);}};
}

// Preserves legacy accordion's visual DOM structure/classes, but lifetime and
// event ownership now belong to the mounted Svelte component via use:.
export function setupAccordion(section,index) {
  if (section.dataset.fixed==='true') return {destroy(){}};
  const head=section.querySelector(':scope > .setup-head');
  const body=section.querySelector(':scope > .setup-body');
  if (!head || !body) return {destroy(){}};
  const wrapper=document.createElement('div'),inner=document.createElement('div');
  wrapper.className='setup-section-collapse';inner.className='setup-section-collapse-inner';
  body.parentNode.insertBefore(wrapper,body);wrapper.append(inner);inner.append(body);
  section.classList.add('collapsible-section');
  let closed=index>=4,closing=false,timer=0;
  if(closed){section.classList.add('collapsed');wrapper.hidden=true;}
  const sync=()=>head.setAttribute('aria-expanded',closed||closing?'false':'true');
  const height=()=>window.dispatchEvent(new Event('resize'));
  const toggle=()=>{
    clearTimeout(timer);
    if(closed||closing){
      closed=false;closing=false;
      section.classList.remove('collapsed','closing');
      wrapper.hidden=false;wrapper.classList.remove('closing');
      void wrapper.offsetHeight;wrapper.classList.add('opening');sync();height();
      timer=setTimeout(()=>{wrapper.classList.remove('opening');height();},230);
    }else{
      closing=true;wrapper.hidden=false;wrapper.classList.remove('opening');
      section.classList.add('closing');void wrapper.offsetHeight;
      wrapper.classList.add('closing');sync();height();
      timer=setTimeout(()=>{
        closing=false;closed=true;section.classList.remove('closing');
        section.classList.add('collapsed');wrapper.classList.remove('closing');
        wrapper.hidden=true;sync();height();
      },230);
    }
  };
  const click=e=>{if(e.target.closest('button,input,textarea,select,a,label'))return;toggle();};
  const keydown=e=>{if(e.target!==head)return; if(e.key==='Enter'||e.key===' '){e.preventDefault();toggle();}};
  head.setAttribute('role','button');head.tabIndex=0;sync();
  head.addEventListener('click',click);head.addEventListener('keydown',keydown);
  return {destroy(){
    clearTimeout(timer);head.removeEventListener('click',click);head.removeEventListener('keydown',keydown);
    head.removeAttribute('role');head.removeAttribute('tabindex');head.removeAttribute('aria-expanded');
    // Restore body DOM before Svelte tears down its own compiled markup.
    if(wrapper.parentNode){wrapper.parentNode.insertBefore(body,wrapper);wrapper.remove();}
    section.classList.remove('collapsible-section','collapsed','closing');
  }};
}
