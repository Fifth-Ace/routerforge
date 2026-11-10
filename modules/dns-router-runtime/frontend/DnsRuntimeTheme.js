// Presentation-only host theme and frame height integration. No router IO.
export function setupRuntimeTheme(){
  const root = document.documentElement;
  const workspace = document.getElementById('dnsRouterWorkspace');
  const params = new URLSearchParams(window.location.search);
  const themeTokens = [
    '--rf-bg','--rf-surface','--rf-surface-2','--rf-hover','--rf-text','--rf-muted',
    '--rf-border','--rf-border-strong','--rf-accent','--rf-accent-soft',
    '--rf-accent-border','--rf-accent-hover','--rf-radius-panel','--rf-radius-card',
    '--rf-radius-control','--rf-panel-head-h','--rf-control-gap',
    '--ui-body','--ui-small','--ui-xs','--ui-micro','--ui-title',
    '--ui-panel-title','--rf-type-metric'
  ];
  const palette = {
    forge: ['#0b0d10','#12151a','#171b21','#1d2229','#f5f7fa','#8d98a4','#29313a','#36414d'],
    midnight: ['#08111b','#0f1824','#152131','#1b2a3c','#edf5ff','#8ca0b5','#25384a','#34516a'],
    graphite: ['#101113','#17191c','#1d2024','#25292e','#f1f2f4','#9299a2','#30353c','#414850']
  };
  function validColor(value, fallback) {
    return /^#[0-9a-fA-F]{6}$/.test(String(value || '')) ? value : fallback;
  }
  // Match Antiscan's frame-side custom palette when the host theme is not accessible.
  function mix(a, b, weight) {
    return '#' + [1,3,5].map(i => Math.round(
      parseInt(a.slice(i,i+2),16)*(1-weight) + parseInt(b.slice(i,i+2),16)*weight
    ).toString(16).padStart(2,'0')).join('');
  }
  function customPalette(background, text) {
    const rgb = [1,3,5].map(i => parseInt(background.slice(i,i+2),16));
    const light = (rgb[0]*.2126 + rgb[1]*.7152 + rgb[2]*.0722) / 255 > .55;
    return [background,
      mix(background,text,light?.045:.055),
      mix(background,text,light?.09:.105),
      mix(background,text,light?.13:.15),
      text,mix(text,background,.48),
      mix(background,text,light?.18:.19),
      mix(background,text,light?.27:.29)];
  }
  function fallbackTheme() {
    const id = params.get('theme') || 'forge';
    const background = validColor(params.get('background'), '#0b0d10');
    const text = validColor(params.get('text'), '#f5f7fa');
    const colors = id === 'custom' ? customPalette(background,text) : (palette[id] || palette.forge);
    const names = ['--rf-bg','--rf-surface','--rf-surface-2','--rf-hover','--rf-text','--rf-muted','--rf-border','--rf-border-strong'];
    names.forEach((key,i) => root.style.setProperty(key,colors[i]));
    root.style.setProperty('--rf-accent',validColor(params.get('accent'),'#38bdf8'));
    root.dataset.theme = id;
    if (['compact','normal','comfortable'].includes(params.get('density'))) root.dataset.density = params.get('density');
    if (['sharp','default','soft'].includes(params.get('radius'))) root.dataset.radius = params.get('radius');
    root.style.colorScheme = (id === 'custom' ? background : colors[0]).match(/^#[0-9a-f]{6}$/i)
      && (parseInt((id==='custom'?background:colors[0]).slice(1,3),16)*.2126
       + parseInt((id==='custom'?background:colors[0]).slice(3,5),16)*.7152
       + parseInt((id==='custom'?background:colors[0]).slice(5,7),16)*.0722) > 140 ? 'light' : 'dark';
  }
  function syncHostTheme() {
    if (window.parent === window) return false;
    try {
      const hostRoot = window.parent.document.documentElement;
      const hostStyle = window.parent.getComputedStyle(hostRoot);
      themeTokens.forEach(token => {
        const v = hostStyle.getPropertyValue(token).trim();
        if (v) root.style.setProperty(token, v);
      });
      ['theme','density','radius','uiScale'].forEach(key => {
        if (hostRoot.dataset[key]) root.dataset[key] = hostRoot.dataset[key];
      });
      root.style.colorScheme = hostStyle.colorScheme === 'light' ? 'light' : 'dark';
      return true;
    } catch (_) { return false; }
  }
  fallbackTheme();
  syncHostTheme();
  let heightFrame = 0;
  function reportHeight() {
    if (window.parent === window || !workspace || heightFrame) return;
    heightFrame = requestAnimationFrame(() => {
      heightFrame = 0;
      const height = Math.ceil(Math.max(360, workspace.scrollHeight, workspace.getBoundingClientRect().height) + 6);
      window.parent.postMessage({type:'routerforge-module-height',moduleId:'dns-router',height}, window.location.origin);
    });
  }

  const observers=[];
  if(workspace && 'ResizeObserver' in window){
    const obs=new ResizeObserver(reportHeight);obs.observe(workspace);observers.push(obs);
  }
  try{
    if(window.parent!==window && 'MutationObserver' in window){
      const obs=new MutationObserver(()=>{syncHostTheme();reportHeight();});
      obs.observe(window.parent.document.documentElement,{attributes:true,
        attributeFilter:['style','class','data-theme','data-density','data-radius','data-ui-scale']});
      observers.push(obs);
    }
  }catch(_){}
  window.addEventListener('resize',reportHeight);
  reportHeight();
  return ()=>{
    window.removeEventListener('resize',reportHeight);
    for(const obs of observers)obs.disconnect();
    if(heightFrame)cancelAnimationFrame(heightFrame);
  };
}
