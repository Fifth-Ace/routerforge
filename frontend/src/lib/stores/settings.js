import { browser } from '$app/environment';
import { writable } from 'svelte/store';

export const defaults = {
  uiLevel: 'normal',
  uiScale: 'auto',
  refreshMs: 2000,
  theme: 'console',
  accent: '#38bdf8',
  background: '#0c0d10',
  text: '#f5f7fa'
};

export const themes = {
  console: { accent:'#38bdf8', background:'#0c0d10', text:'#f5f7fa', secondary:'#14161a', tertiary:'#191b21', hover:'#1f2229', muted:'#8b949e', border:'#242831' },
  legacy:  { accent:'#7aa2f7', background:'#1a1b26', text:'#c0caf5', secondary:'#16161e', tertiary:'#24283b', hover:'#292e42', muted:'#737aa2', border:'#3b4261' },
  neo:     { accent:'#d7ff44', background:'#060707', text:'#f2f3f3', secondary:'#0d0e0e', tertiary:'#181a1a', hover:'#202323', muted:'#747a7a', border:'#2a2e2e' },
  mint:    { accent:'#82c8d6', background:'#20252b', text:'#e9eef1', secondary:'#293139', tertiary:'#3d4854', hover:'#4b5866', muted:'#a1adb8', border:'#53616e' }
};

function normalize(value = {}) {
  const next = { ...value };
  if (next.uiLevel === 'expert') next.uiLevel = 'advanced';
  if (next.uiLevel !== 'advanced') next.uiLevel = 'normal';
  delete next.compact;
  return { ...defaults, ...next };
}

function load() {
  if (!browser) return { ...defaults };
  try { return normalize(JSON.parse(localStorage.getItem('dnsmon.settings') || '{}')); }
  catch { return { ...defaults }; }
}

function mix(hex1, hex2, t) {
  const a = String(hex1).replace('#', '');
  const b = String(hex2).replace('#', '');
  const p = (x) => parseInt(x, 16);
  return '#' + [0,2,4].map((i) =>
    Math.round(p(a.slice(i,i+2))*(1-t)+p(b.slice(i,i+2))*t).toString(16).padStart(2,'0')
  ).join('');
}

function luminance(hex) {
  const c = String(hex).replace('#','');
  return [0,2,4].map((i)=>parseInt(c.slice(i,i+2),16))
    .reduce((sum,v,index)=>sum+v*[.2126,.7152,.0722][index],0)/255;
}

export function applySettings(value) {
  if (!browser) return;
  const normalized = normalize(value);
  const root = document.documentElement;
  root.dataset.uiLevel = normalized.uiLevel;
  root.dataset.uiScale = normalized.uiScale || 'auto';
  root.dataset.theme = normalized.theme || 'console';

  let t = themes[normalized.theme];
  if (!t) {
    const bg = normalized.background || defaults.background;
    const text = normalized.text || defaults.text;
    const accent = normalized.accent || defaults.accent;
    const light = luminance(bg) > .55;
    t = {
      accent,
      background:bg,
      text,
      secondary:mix(bg,text,light?.045:.055),
      tertiary:mix(bg,text,light?.09:.105),
      hover:mix(bg,text,light?.13:.15),
      muted:mix(text,bg,.48),
      border:mix(bg,text,light?.18:.19)
    };
  }

  const style = root.style;
  style.setProperty('--accent',t.accent);
  style.setProperty('--bg',t.background);
  style.setProperty('--surface',t.secondary);
  style.setProperty('--surface-2',t.tertiary);
  style.setProperty('--hover',t.hover);
  style.setProperty('--text',t.text);
  style.setProperty('--muted',t.muted);
  style.setProperty('--border',t.border);
}

const inner = writable(load());

export const settings = {
  subscribe: inner.subscribe,
  set(value) {
    const next = normalize(value);
    if (browser) localStorage.setItem('dnsmon.settings', JSON.stringify(next));
    applySettings(next);
    inner.set(next);
  },
  update(fn) {
    inner.update((current) => {
      const next = normalize(fn(current));
      if (browser) localStorage.setItem('dnsmon.settings', JSON.stringify(next));
      applySettings(next);
      return next;
    });
  }
};

if (browser) applySettings(load());
