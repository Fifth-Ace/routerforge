// Antiscan host-frame adapter: theme and iframe-height behavior only.
// Created by the Svelte root on mount and always torn down on unmount.
export function mountAntiscanHostBridge(workspace) {
  if (!workspace) throw new Error('ANTISCAN_HOST_WORKSPACE_REQUIRED');
  let disposed = false;
  const routerForgeParams = new URLSearchParams(window.location.search);
  let routerForgeResizeObserver = null;
  let routerForgeHeightFrame = 0;
  let routerForgeHostObserver = null;

  const routerForgeHostTokens = [
    '--rf-bg',
    '--rf-surface',
    '--rf-surface-2',
    '--rf-hover',
    '--rf-text',
    '--rf-muted',
    '--rf-border',
    '--rf-border-strong',
    '--rf-accent',
    '--rf-accent-soft',
    '--rf-accent-hover',
    '--rf-accent-border',
    '--rf-radius-panel',
    '--rf-radius-control',
    '--rf-radius-card',
    '--rf-panel-head-h',
    '--rf-control-gap',
    '--ui-body',
    '--ui-small',
    '--ui-xs',
    '--ui-micro',
    '--ui-panel-title',
    '--rf-type-metric'
  ];

  function syncRouterForgeHostTheme() {
    if (window.parent === window) return false;
    try {
      const root = document.documentElement;
      const hostRoot = window.parent.document.documentElement;
      const hostStyle = window.parent.getComputedStyle(hostRoot);
      routerForgeHostTokens.forEach((token) => {
        const value = hostStyle.getPropertyValue(token).trim();
        if (value) root.style.setProperty(token, value);
      });
      ['theme', 'density', 'radius', 'uiScale'].forEach((key) => {
        const value = hostRoot.dataset[key];
        if (value) root.dataset[key] = value;
      });
      return true;
    } catch (_) {
      return false;
    }
  }

  const routerForgeThemes = {
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
    }
  };

  function routerForgeHex(value, fallback) {
    const normalized = String(value || '').trim().toLowerCase();
    return /^#[0-9a-f]{6}$/i.test(normalized) ? normalized : fallback;
  }

  function routerForgeMix(hex1, hex2, t) {
    const a = routerForgeHex(hex1, '#000000').slice(1);
    const b = routerForgeHex(hex2, '#ffffff').slice(1);
    const part = (value) => parseInt(value, 16);
    return '#' + [0, 2, 4].map((i) =>
      Math.round(
        part(a.slice(i, i + 2)) * (1 - t) +
        part(b.slice(i, i + 2)) * t
      ).toString(16).padStart(2, '0')
    ).join('');
  }

  function routerForgeLuminance(hex) {
    const value = routerForgeHex(hex, '#000000').slice(1);
    return [0, 2, 4].map((i) => parseInt(value.slice(i, i + 2), 16))
      .reduce((sum, channel, index) => sum + channel * [.2126, .7152, .0722][index], 0) / 255;
  }

  function routerForgeCustomTheme(background, text) {
    const light = routerForgeLuminance(background) > .55;
    return {
      background,
      text,
      surface: routerForgeMix(background, text, light ? .045 : .055),
      surface2: routerForgeMix(background, text, light ? .09 : .105),
      hover: routerForgeMix(background, text, light ? .13 : .15),
      muted: routerForgeMix(text, background, .48),
      border: routerForgeMix(background, text, light ? .18 : .19),
      borderStrong: routerForgeMix(background, text, light ? .27 : .29)
    };
  }

  function applyRouterForgeFrameSettings() {
    const root = document.documentElement;
    const theme = routerForgeParams.get('theme') || 'forge';
    const background = routerForgeHex(routerForgeParams.get('background'), '#0b0d10');
    const text = routerForgeHex(routerForgeParams.get('text'), '#f5f7fa');
    const accent = routerForgeHex(routerForgeParams.get('accent'), '#38bdf8');
    const palette = theme === 'custom'
      ? routerForgeCustomTheme(background, text)
      : (routerForgeThemes[theme] || routerForgeThemes.forge);

    root.dataset.theme = theme;
    root.style.setProperty('--rf-bg', palette.background);
    root.style.setProperty('--rf-surface', palette.surface);
    root.style.setProperty('--rf-surface-2', palette.surface2);
    root.style.setProperty('--rf-hover', palette.hover);
    root.style.setProperty('--rf-text', palette.text);
    root.style.setProperty('--rf-muted', palette.muted);
    root.style.setProperty('--rf-border', palette.border);
    root.style.setProperty('--rf-border-strong', palette.borderStrong);
    root.style.setProperty('--rf-accent', accent);

    const density = routerForgeParams.get('density');
    if (density === 'compact' || density === 'normal' || density === 'comfortable') {
      root.dataset.density = density;
    }

    const radius = routerForgeParams.get('radius');
    if (radius === 'sharp' || radius === 'default' || radius === 'soft') {
      root.dataset.radius = radius;
      root.style.setProperty('--rf-radius', radius === 'sharp' ? '2px' : radius === 'soft' ? '12px' : '8px');
    }

    syncRouterForgeHostTheme();
  }

  function reportRouterForgeModuleHeight() {
    if (disposed || window.parent === window) return;
    if (routerForgeHeightFrame) cancelAnimationFrame(routerForgeHeightFrame);
    routerForgeHeightFrame = requestAnimationFrame(() => {
      routerForgeHeightFrame = 0;
      if (disposed || !workspace) return;
      const height = Math.ceil(Math.max(workspace.scrollHeight, workspace.getBoundingClientRect().height, 360) + 2);
      window.parent.postMessage({
        type: 'routerforge-module-height',
        moduleId: 'antiscan-manager',
        height
      }, window.location.origin);
    });
  }

  function startRouterForgeBridge() {
    applyRouterForgeFrameSettings();
    reportRouterForgeModuleHeight();

    if (window.parent !== window && 'MutationObserver' in window) {
      try {
        const hostRoot = window.parent.document.documentElement;
        routerForgeHostObserver = new MutationObserver(() => syncRouterForgeHostTheme());
        routerForgeHostObserver.observe(hostRoot, {
          attributes: true,
          attributeFilter: ['style', 'class', 'data-theme', 'data-density', 'data-radius', 'data-ui-scale']
        });
      } catch (_) {
        routerForgeHostObserver = null;
      }
    }

    if (workspace && 'ResizeObserver' in window) {
      routerForgeResizeObserver = new ResizeObserver(reportRouterForgeModuleHeight);
      routerForgeResizeObserver.observe(workspace);
    }
    window.addEventListener('resize', reportRouterForgeModuleHeight);
  }

  startRouterForgeBridge();
  return {
    reportHeight: reportRouterForgeModuleHeight,
    dispose() {
      if (disposed) return;
      disposed = true;
      if (routerForgeHeightFrame) cancelAnimationFrame(routerForgeHeightFrame);
      routerForgeHeightFrame = 0;
      routerForgeResizeObserver?.disconnect();
      routerForgeHostObserver?.disconnect();
      window.removeEventListener('resize', reportRouterForgeModuleHeight);
    }
  };
}
