const defaults = {
  uiLevel: 'expert',
  compact: false,
  refreshMs: 2000,
  theme: 'custom',
  accent: '#1eff00',
  background: '#000000',
  text: '#ffffff'
};

export function loadSettings() {
  try { return { ...defaults, ...JSON.parse(localStorage.getItem('dnsmon.settings') || '{}') }; }
  catch { return { ...defaults }; }
}
export function saveSettings(s) { localStorage.setItem('dnsmon.settings', JSON.stringify(s)); }

const themes = {
  legacy: { accent:'#7aa2f7', background:'#1a1b26', text:'#c0caf5', secondary:'#16161e', tertiary:'#24283b', hover:'#292e42', muted:'#737aa2', border:'#3b4261' },
  neo:    { accent:'#d7ff44', background:'#060707', text:'#f2f3f3', secondary:'#0d0e0e', tertiary:'#181a1a', hover:'#202323', muted:'#747a7a', border:'#2a2e2e' },
  mint:   { accent:'#82c8d6', background:'#20252b', text:'#e9eef1', secondary:'#293139', tertiary:'#3d4854', hover:'#4b5866', muted:'#a1adb8', border:'#53616e' }
};

function mix(hex1, hex2, t) {
  const p=x=>parseInt(x,16), a=hex1.replace('#',''), b=hex2.replace('#','');
  const out=[0,2,4].map(i=>Math.round(p(a.slice(i,i+2))*(1-t)+p(b.slice(i,i+2))*t).toString(16).padStart(2,'0')).join('');
  return '#'+out;
}
function luminance(hex) { const c=hex.replace('#',''); return [0,2,4].map(i=>parseInt(c.slice(i,i+2),16)).reduce((s,v,idx)=>s+v*[.2126,.7152,.0722][idx],0)/255; }

export function applySettings(s) {
  const root=document.documentElement;
  root.dataset.layoutCompact=String(!!s.compact);
  root.dataset.uiLevel=s.uiLevel || 'expert';
  root.dataset.theme=s.theme || 'custom';
  let t=themes[s.theme];
  if (!t) {
    const bg=s.background||'#000000', text=s.text||'#ffffff', accent=s.accent||'#1eff00';
    const light=luminance(bg)>.55;
    t={ accent, background:bg, text,
      secondary:mix(bg,text,light?.045:.055), tertiary:mix(bg,text,light?.09:.105), hover:mix(bg,text,light?.13:.15),
      muted:mix(text,bg,.48), border:mix(bg,text,light?.18:.19) };
  }
  const style=root.style;
  style.setProperty('--color-accent',t.accent);
  style.setProperty('--color-accent-hover',mix(t.accent,t.text,.18));
  style.setProperty('--color-bg-primary',t.background);
  style.setProperty('--color-bg-secondary',t.secondary);
  style.setProperty('--color-bg-tertiary',t.tertiary);
  style.setProperty('--color-bg-hover',t.hover);
  style.setProperty('--color-text-primary',t.text);
  style.setProperty('--color-text-secondary',mix(t.text,t.background,.28));
  style.setProperty('--color-text-muted',t.muted);
  style.setProperty('--color-border',t.border);
  style.setProperty('--color-border-hover',mix(t.border,t.text,.25));
}
export { defaults, themes };
