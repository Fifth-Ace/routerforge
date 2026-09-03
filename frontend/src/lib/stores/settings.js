import { browser } from '$app/environment';
import { writable } from 'svelte/store';

export const defaults = {
  uiLevel: 'normal',
  uiScale: 'auto',
  refreshMs: 2000,
  theme: 'forge',
  accent: '#38bdf8',
  background: '#0b0d10',
  text: '#f5f7fa',
  density: 'normal',
  radius: 'default',
  brandMode: 'brand-only'
};

export const themes = {
  forge: {
    background: '#0b0d10', text: '#f5f7fa', surface: '#12151a', surface2: '#171b21',
    hover: '#1d2229', muted: '#8d98a4', border: '#29313a', borderStrong: '#36414d'
  },
  midnight: {
    background: '#08111b', text: '#edf5ff', surface: '#0f1824', surface2: '#152131',
    hover: '#1b2a3c', muted: '#8ca0b5', border: '#25384a', borderStrong: '#34516a'
  },
  graphite: {
    background: '#101113', text: '#f1f2f4', surface: '#17191c', surface2: '#1d2024',
    hover: '#25292e', muted: '#9299a2', border: '#30353c', borderStrong: '#414850'
  },
  custom: null
};

const validDensity = new Set(['compact', 'normal', 'comfortable']);
const validRadius = new Set(['sharp', 'default', 'soft']);
const validBrandMode = new Set(['brand-only', 'extended']);
const legacyThemes = {
  console: 'forge',
  legacy: 'midnight',
  neo: 'graphite',
  mint: 'graphite'
};

function safeHex(value, fallback) {
  const normalized = String(value || '').trim();
  return /^#[0-9a-f]{6}$/i.test(normalized) ? normalized.toLowerCase() : fallback;
}

function normalize(value = {}) {
  const next = { ...value };
  if (next.uiLevel === 'expert') next.uiLevel = 'advanced';
  if (next.uiLevel !== 'advanced') next.uiLevel = 'normal';
  if (legacyThemes[next.theme]) next.theme = legacyThemes[next.theme];
  if (!themes[next.theme] && next.theme !== 'custom') next.theme = defaults.theme;
  if (!validDensity.has(next.density)) next.density = defaults.density;
  if (!validRadius.has(next.radius)) next.radius = defaults.radius;
  if (!validBrandMode.has(next.brandMode)) next.brandMode = defaults.brandMode;
  next.accent = safeHex(next.accent, defaults.accent);
  next.background = safeHex(next.background, defaults.background);
  next.text = safeHex(next.text, defaults.text);
  delete next.compact;
  return { ...defaults, ...next };
}

function load() {
  if (!browser) return { ...defaults };
  try {
    const current = localStorage.getItem('routerforge.settings');
    const legacy = localStorage.getItem('dnsmon.settings');
    return normalize(JSON.parse(current || legacy || '{}'));
  } catch {
    return { ...defaults };
  }
}

function mix(hex1, hex2, t) {
  const a = safeHex(hex1, '#000000').slice(1);
  const b = safeHex(hex2, '#ffffff').slice(1);
  const p = (x) => parseInt(x, 16);
  return '#' + [0, 2, 4].map((i) =>
    Math.round(p(a.slice(i, i + 2)) * (1 - t) + p(b.slice(i, i + 2)) * t).toString(16).padStart(2, '0')
  ).join('');
}

function rgba(hex, alpha) {
  const raw = safeHex(hex, '#38bdf8').slice(1);
  const rgb = [0, 2, 4].map((i) => parseInt(raw.slice(i, i + 2), 16));
  return `rgba(${rgb[0]}, ${rgb[1]}, ${rgb[2]}, ${alpha})`;
}

function luminance(hex) {
  const c = safeHex(hex, '#000000').slice(1);
  return [0, 2, 4].map((i) => parseInt(c.slice(i, i + 2), 16))
    .reduce((sum, v, index) => sum + v * [.2126, .7152, .0722][index], 0) / 255;
}

function customTheme(normalized) {
  const bg = normalized.background;
  const text = normalized.text;
  const light = luminance(bg) > .55;
  return {
    background: bg,
    text,
    surface: mix(bg, text, light ? .045 : .055),
    surface2: mix(bg, text, light ? .09 : .105),
    hover: mix(bg, text, light ? .13 : .15),
    muted: mix(text, bg, .48),
    border: mix(bg, text, light ? .18 : .19),
    borderStrong: mix(bg, text, light ? .27 : .29)
  };
}

export function applySettings(value) {
  if (!browser) return;
  const normalized = normalize(value);
  const root = document.documentElement;
  root.dataset.uiLevel = normalized.uiLevel;
  root.dataset.uiScale = normalized.uiScale || 'auto';
  root.dataset.theme = normalized.theme;
  root.dataset.density = normalized.density;
  root.dataset.radius = normalized.radius;
  root.dataset.brandMode = normalized.brandMode;

  const palette = normalized.theme === 'custom' ? customTheme(normalized) : themes[normalized.theme];
  const accent = normalized.accent;
  const brand = '#d7824b';
  const brandUI = normalized.brandMode === 'extended' ? brand : accent;

  const style = root.style;

  // Compatibility variables used by the existing RouterForge UI.
  style.setProperty('--accent', accent);
  style.setProperty('--bg', palette.background);
  style.setProperty('--surface', palette.surface);
  style.setProperty('--surface-2', palette.surface2);
  style.setProperty('--hover', palette.hover);
  style.setProperty('--text', palette.text);
  style.setProperty('--muted', palette.muted);
  style.setProperty('--border', palette.border);

  // RouterForge semantic design tokens.
  style.setProperty('--rf-bg', palette.background);
  style.setProperty('--rf-surface', palette.surface);
  style.setProperty('--rf-surface-2', palette.surface2);
  style.setProperty('--rf-hover', palette.hover);
  style.setProperty('--rf-text', palette.text);
  style.setProperty('--rf-muted', palette.muted);
  style.setProperty('--rf-border', palette.border);
  style.setProperty('--rf-border-strong', palette.borderStrong);
  style.setProperty('--rf-accent', accent);
  style.setProperty('--rf-accent-soft', rgba(accent, .10));
  style.setProperty('--rf-accent-hover', rgba(accent, .16));
  style.setProperty('--rf-accent-border', rgba(accent, .30));
  style.setProperty('--rf-brand', brand);
  style.setProperty('--rf-brand-ui', brandUI);
  style.setProperty('--rf-brand-soft', rgba(brandUI, .11));
  style.setProperty('--rf-brand-border', rgba(brandUI, .30));
}

const inner = writable(load());

function persist(next) {
  if (!browser) return;
  localStorage.setItem('routerforge.settings', JSON.stringify(next));
}

export const settings = {
  subscribe: inner.subscribe,
  set(value) {
    const next = normalize(value);
    persist(next);
    applySettings(next);
    inner.set(next);
  },
  update(fn) {
    inner.update((current) => {
      const next = normalize(fn(current));
      persist(next);
      applySettings(next);
      return next;
    });
  }
};

if (browser) applySettings(load());
