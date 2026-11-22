// RouterForge NFQWS iframe/theme/height adapter.
// Mounted and disposed only by NFQWSManagerApp.svelte; no service mutations.
export function mountNfqwsHostBridge() {
  const mix=(a,b,t)=>{const A=a.replace('#',''),B=b.replace('#','');const p=(s,i)=>parseInt(s.slice(i,i+2),16);return '#'+[0,2,4].map(i=>Math.round(p(A,i)*(1-t)+p(B,i)*t).toString(16).padStart(2,'0')).join('')};
  const lum=h=>{const x=h.replace('#','');return [0,2,4].map(i=>parseInt(x.slice(i,i+2),16)).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0)/255};

  function applyRouterForgeTheme(){
    const q=new URLSearchParams(location.search), root=document.documentElement;
    const palettes={
      forge:{background:'#0b0d10',text:'#f5f7fa',surface:'#12151a',surface2:'#171b21',hover:'#1d2229',muted:'#8d98a4',border:'#29313a',borderStrong:'#36414d'},
      midnight:{background:'#08111b',text:'#edf5ff',surface:'#0f1824',surface2:'#152131',hover:'#1b2a3c',muted:'#8ca0b5',border:'#25384a',borderStrong:'#34516a'},
      graphite:{background:'#101113',text:'#f1f2f4',surface:'#17191c',surface2:'#1d2024',hover:'#25292e',muted:'#9299a2',border:'#30353c',borderStrong:'#414850'}
    };
    const theme=q.get('theme')||'forge', accent=q.get('accent')||'#38bdf8';
    let p=palettes[theme]||palettes.forge;
    if(theme==='custom'){
      const bg=q.get('background')||'#0b0d10', text=q.get('text')||'#f5f7fa', light=lum(bg)>.55;
      p={background:bg,text,surface:mix(bg,text,light?.045:.055),surface2:mix(bg,text,light?.09:.105),hover:mix(bg,text,light?.13:.15),muted:mix(text,bg,.48),border:mix(bg,text,light?.18:.19),borderStrong:mix(bg,text,light?.27:.29)};
    }
    const st=root.style;
    for(const [k,v] of Object.entries({'--bg':p.background,'--surface':p.surface,'--surface2':p.surface2,'--hover':p.hover,'--text':p.text,'--muted':p.muted,'--border':p.border,'--border-strong':p.borderStrong,'--accent':accent,'--code':mix(p.background,p.text,lum(p.background)>.55?.015:.018)}))st.setProperty(k,v);
    root.dataset.density=q.get('density')||'normal';
    root.style.setProperty('--radius',({sharp:'3px',default:'8px',soft:'12px'})[q.get('radius')||'default']);
    try{root.lang=q.get('locale')||'ru'}catch(_){}
  }
  applyRouterForgeTheme();
  let disposed = false;
  let heightReportRaf = 0;
  function reportHeight() {
    if (disposed) return;
    if (heightReportRaf) cancelAnimationFrame(heightReportRaf);
    heightReportRaf = requestAnimationFrame(() => {
      heightReportRaf = 0;
      if (disposed) return;
      try {
        const shell = document.querySelector('.shell');
        const height = Math.ceil((shell?.scrollHeight || shell?.getBoundingClientRect().height || 720) + 4);
        parent.postMessage({type:'routerforge-module-height',moduleId:'nfqws-manager',height},location.origin);
      } catch (_) {}
    });
  }
  const heightObserver = typeof ResizeObserver === 'function' ? new ResizeObserver(reportHeight) : null;
  heightObserver?.observe(document.querySelector('.shell') || document.body);
  window.addEventListener('load', reportHeight);
  window.addEventListener('resize', reportHeight);
  reportHeight();
  return {
    reportHeight,
    dispose() {
      if (disposed) return;
      disposed = true;
      if (heightReportRaf) cancelAnimationFrame(heightReportRaf);
      heightObserver?.disconnect();
      window.removeEventListener('load', reportHeight);
      window.removeEventListener('resize', reportHeight);
    }
  };
}
