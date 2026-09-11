<script>
  import { onDestroy, onMount } from 'svelte';
  import { Terminal } from '@xterm/xterm';
  import { FitAddon } from '@xterm/addon-fit';
  import '@xterm/xterm/css/xterm.css';

  export let locale = 'ru';

  let host;
  let terminal;
  let fitAddon;
  let resizeObserver;
  let socket;
  let resizeTimer;
  let connected = false;
  let connecting = false;
  let statusText = 'OFFLINE';
  let dimensions = '80×24';
  let disposed = false;
  let trimInitialPTYBreaks = true;
  let terminalMode = 'entware';

  $: copy = locale === 'ru' ? {
    title: 'Entware Terminal',
    keeneticTitle: 'Keenetic NDM Console',
    connect: 'Подключить',
    disconnect: 'Отключить',
    reconnect: 'Переподключить',
    clear: 'Очистить',
    entware: 'Entware',
    keenetic: 'Keenetic',
    keeneticHint: 'Keenetic NDM Console подключим отдельным guarded backend.',
    connecting: 'CONNECTING',
    connected: 'CONNECTED',
    disconnected: 'DISCONNECTED'
  } : {
    title: 'Entware Terminal',
    keeneticTitle: 'Keenetic NDM Console',
    connect: 'Connect',
    disconnect: 'Disconnect',
    reconnect: 'Reconnect',
    clear: 'Clear',
    entware: 'Entware',
    keenetic: 'Keenetic',
    keeneticHint: 'Keenetic NDM Console will use a separate guarded backend.',
    connecting: 'CONNECTING',
    connected: 'CONNECTED',
    disconnected: 'DISCONNECTED'
  };

  function cssToken(name, fallback) {
    if (typeof document === 'undefined') return fallback;
    const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    return value || fallback;
  }

  function terminalTheme() {
    const accent = cssToken('--rf-accent', '#38bdf8');
    const background = cssToken('--rf-bg', '#0b0d10');
    const foreground = cssToken('--rf-text', '#f5f7fa');
    const muted = cssToken('--rf-muted', '#8d98a4');
    return {
      background,
      foreground,
      cursor: foreground,
      cursorAccent: background,
      selectionBackground: cssToken('--rf-accent-soft', 'rgba(56, 189, 248, .18)'),
      black: background,
      red: '#ff6b6b',
      green: '#7bd88f',
      yellow: '#ffd866',
      blue: accent,
      magenta: '#c099ff',
      cyan: accent,
      white: foreground,
      brightBlack: muted,
      brightRed: '#ff8787',
      brightGreen: '#97e6a8',
      brightYellow: '#ffe58f',
      brightBlue: accent,
      brightMagenta: '#d0b3ff',
      brightCyan: accent,
      brightWhite: '#ffffff'
    };
  }
  function setState(value, label) {
    connected = value;
    statusText = label;
  }

  function terminalBanner() {
    const label = terminalMode === 'keenetic' ? 'Keenetic NDM Console' : 'Entware Terminal';
    if (terminal) terminal.writeln(`\x1b[1;36mRouterForge ${label}\x1b[0m`);
  }
  function writeServerOutput(data) {
    if (!trimInitialPTYBreaks) {
      terminal.write(data instanceof ArrayBuffer ? new Uint8Array(data) : data);
      return;
    }

    if (data instanceof ArrayBuffer) {
      const bytes = new Uint8Array(data);
      let start = 0;
      while (start < bytes.length && (bytes[start] === 10 || bytes[start] === 13)) start += 1;
      if (start >= bytes.length) return;
      trimInitialPTYBreaks = false;
      terminal.write(bytes.subarray(start));
      return;
    }

    const text = String(data || '').replace(/^[\r\n]+/, '');
    if (!text) return;
    trimInitialPTYBreaks = false;
    terminal.write(text);
  }

  function socketURL() {
    const scheme = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const cols = terminal ? terminal.cols : 80;
    const rows = terminal ? terminal.rows : 24;
    const params = new URLSearchParams({
      mode: terminalMode,
      cols: String(cols),
      rows: String(rows)
    });
    if (terminalMode === 'entware') params.set('cwd', '/opt');
    return `${scheme}//${window.location.host}/api/modules/admin/terminal/ws?${params.toString()}`;
  }
  function sendResize() {
    clearTimeout(resizeTimer);
    resizeTimer = null;
    if (!terminal) return;
    dimensions = `${terminal.cols}×${terminal.rows}`;
    if (!socket || socket.readyState !== WebSocket.OPEN) return;
    socket.send(JSON.stringify({ type: 'resize', cols: terminal.cols, rows: terminal.rows }));
  }

  function fitAndResize() {
    if (!terminal || !fitAddon) return;
    try { fitAddon.fit(); } catch {}
    dimensions = `${terminal.cols}×${terminal.rows}`;
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(sendResize, 80);
  }

  function connect() {
    if (!terminal || connecting || (socket && socket.readyState === WebSocket.OPEN)) return;
    connecting = true;
    setState(false, copy.connecting);

    const ws = new WebSocket(socketURL());
    socket = ws;
    ws.binaryType = 'arraybuffer';

    ws.onopen = () => {
      if (socket !== ws) return;
      connecting = false;
      setState(true, copy.connected);
      sendResize();
      terminal.focus();
    };

    ws.onmessage = (event) => {
      if (socket !== ws) return;
      if (event.data instanceof ArrayBuffer || typeof event.data === 'string') {
        writeServerOutput(event.data);
      }
    };

    ws.onerror = () => {
      if (socket !== ws) return;
      connecting = false;
      setState(false, copy.disconnected);
    };

    ws.onclose = (event) => {
      if (socket !== ws) return;
      socket = null;
      connecting = false;
      setState(false, `${copy.disconnected}${event.code ? ` · ${event.code}` : ''}`);
    };
  }

  function disconnect() {
    const ws = socket;
    socket = null;
    connecting = false;
    setState(false, copy.disconnected);
    if (ws && ws.readyState < WebSocket.CLOSING) ws.close(1000, 'user disconnect');
  }

  function reconnect() {
    disconnect();
    setTimeout(connect, 40);
  }

  function switchTerminalMode(nextMode) {
    if (nextMode !== 'entware' && nextMode !== 'keenetic') return;
    if (terminalMode === nextMode) {
      if (terminal) terminal.focus();
      return;
    }
    terminalMode = nextMode;
    trimInitialPTYBreaks = true;
    if (terminal) terminal.clear();
    terminalBanner();
    reconnect();
  }
  onMount(() => {
    terminal = new Terminal({
      allowProposedApi: false,
      convertEol: false,
      cursorBlink: true,
      cursorStyle: 'block',
      fontFamily: '"Roboto Mono", "Cascadia Mono", Consolas, monospace',
      fontSize: 13,
      fontWeight: '400',
      fontWeightBold: '700',
      lineHeight: 1.14,
      scrollback: 5000,
      tabStopWidth: 4,
      theme: terminalTheme()
    });
    fitAddon = new FitAddon();
    terminal.loadAddon(fitAddon);
    terminal.open(host);
    terminalBanner();

    terminal.onData((data) => {
      if (socket && socket.readyState === WebSocket.OPEN) {
        socket.send(new TextEncoder().encode(data));
      }
    });
    terminal.onResize(() => {
      dimensions = `${terminal.cols}×${terminal.rows}`;
    });

    resizeObserver = new ResizeObserver(fitAndResize);
    resizeObserver.observe(host);
    fitAndResize();
    connect();
  });

  onDestroy(() => {
    disposed = true;
    clearTimeout(resizeTimer);
    resizeObserver?.disconnect();
    if (socket && socket.readyState < WebSocket.CLOSING) socket.close(1000, 'view closed');
    socket = null;
    terminal?.dispose();
  });
</script>

<section class="terminal-shell">
  <header class="terminal-topbar">
    <div class="terminal-tabs">
      <button class="terminal-tab" class:active={terminalMode === 'entware'} type="button" onclick={() => switchTerminalMode('entware')}>
        <span class="terminal-dot" class:connected-dot={connected && terminalMode === 'entware'}></span>{copy.entware}
      </button>
      <button class="terminal-tab" class:active={terminalMode === 'keenetic'} type="button" onclick={() => switchTerminalMode('keenetic')}>
        <span class="terminal-dot" class:connected-dot={connected && terminalMode === 'keenetic'}></span>{copy.keenetic}
      </button>
    </div>    <div class="terminal-head-status">
      <span class:online={connected} class="status-dot"></span>
      <strong>{statusText}</strong><span class="mono">{dimensions}</span>
    </div>
  </header>

  <div class="terminal-titlebar">
    <div><strong>{terminalMode === 'keenetic' ? copy.keeneticTitle : copy.title}</strong></div>
    <div class="terminal-actions">
      <button type="button" onclick={() => terminal?.clear()}>{copy.clear}</button>
      {#if connected}
        <button type="button" onclick={reconnect}>{copy.reconnect}</button>
        <button class="danger" type="button" onclick={disconnect}>{copy.disconnect}</button>
      {:else}
        <button type="button" onclick={connect} disabled={connecting}>{connecting ? '…' : copy.connect}</button>
      {/if}
    </div>
  </div>

  <div class="terminal-bezel"><div class="terminal-canvas" bind:this={host}></div></div>

  <footer class="terminal-statusbar">
    {#if terminalMode === 'keenetic'}
      <span><b>ndm</b>@keenetic</span><span>NDM CLI</span>
    {:else}
      <span><b>root</b>@entware</span><span>/opt</span>
    {/if}
    <span>UTF-8</span><span>WS / PTY</span>
    <span class="terminal-status-spacer"></span><span>{dimensions}</span>
  </footer>
</section>

<style>
  :global(.terminal-shell .xterm){height:100%}
  :global(.terminal-shell .xterm-viewport){scrollbar-color:var(--rf-border-strong,#36414d) var(--rf-bg,#0b0d10)}
  .terminal-shell{overflow:hidden;border:1px solid var(--rf-border,#29313a);border-radius:var(--rf-radius-panel,.72rem);background:var(--rf-bg,#0b0d10);color:var(--rf-text,#f5f7fa);box-shadow:none}
  .terminal-topbar{min-height:2.35rem;display:flex;align-items:stretch;justify-content:space-between;background:var(--rf-surface,#12151a);border-bottom:1px solid var(--rf-border,#29313a)}
  .terminal-tabs{display:flex;min-width:0}
  .terminal-tab{display:flex;align-items:center;gap:.48rem;padding:0 .9rem;border:0;border-right:1px solid var(--rf-border,#29313a);background:var(--rf-surface,#12151a);color:var(--rf-muted,#8d98a4);font:inherit;font-size:.8rem}
  .terminal-tab.active{color:var(--rf-text,#f5f7fa);background:var(--rf-bg,#0b0d10);box-shadow:inset 0 2px 0 var(--rf-accent,#38bdf8)}
  .terminal-tab:disabled{cursor:not-allowed;opacity:.52}
  .terminal-tab em{padding:.08rem .3rem;border:1px solid var(--rf-border-strong,#36414d);border-radius:.3rem;font-size:.58rem;font-style:normal;text-transform:uppercase}
  .terminal-dot,.status-dot{width:.45rem;height:.45rem;border-radius:50%;background:var(--rf-muted,#8d98a4);flex:none}
  .connected-dot,.status-dot.online{background:var(--good,#2ea043);box-shadow:0 0 0 3px color-mix(in srgb,var(--good,#2ea043) 14%,transparent)}
  .terminal-head-status{display:flex;align-items:center;gap:.55rem;padding:0 .85rem;color:var(--rf-muted,#8d98a4);font-size:.68rem;letter-spacing:.045em}
  .terminal-head-status strong{color:color-mix(in srgb,var(--rf-text,#f5f7fa) 70%,var(--rf-muted,#8d98a4));font-size:.65rem}.terminal-head-status .online+strong{color:var(--good,#2ea043)}
  .terminal-titlebar{display:flex;align-items:center;justify-content:space-between;gap:1rem;padding:.72rem .9rem;background:var(--rf-surface-2,#171b21);border-bottom:1px solid var(--rf-border,#29313a);color:var(--rf-text,#f5f7fa)}
  .terminal-titlebar>div:first-child{display:flex;flex-direction:column;gap:.15rem;min-width:0}.terminal-titlebar strong{font-size:.88rem}.terminal-titlebar span{color:var(--rf-muted,#8d98a4);font-size:.7rem;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .terminal-actions{display:flex;gap:.4rem;flex-wrap:wrap;justify-content:flex-end}.terminal-actions button{border:1px solid var(--rf-border-strong,#36414d);border-radius:var(--rf-radius-control,.38rem);background:var(--rf-surface,#12151a);color:color-mix(in srgb,var(--rf-text,#f5f7fa) 86%,var(--rf-muted,#8d98a4));padding:.31rem .52rem;font:inherit;font-size:.68rem;cursor:pointer}.terminal-actions button:hover{background:var(--rf-hover,#1d2229);border-color:var(--rf-accent-border,rgba(56,189,248,.30))}.terminal-actions button.danger{border-color:rgba(248,81,73,.38);color:var(--bad,#f85149)}.terminal-actions button:disabled{opacity:.45;cursor:not-allowed}
  .terminal-bezel{padding:.65rem .72rem .35rem;background:var(--rf-bg,#0b0d10)}.terminal-canvas{width:100%;height:clamp(30rem,61vh,48rem);overflow:hidden}
  .terminal-statusbar{display:flex;align-items:center;gap:.9rem;min-height:1.85rem;padding:0 .8rem;border-top:1px solid var(--rf-border,#29313a);background:var(--rf-surface,#12151a);color:var(--rf-muted,#8d98a4);font-family:"Roboto Mono","Cascadia Mono",Consolas,monospace;font-size:.62rem}.terminal-statusbar b{color:var(--rf-accent,#38bdf8);font-weight:700}.terminal-status-spacer{flex:1}
  @media(max-width:800px){.terminal-head-status span.mono{display:none}.terminal-titlebar span{display:none}.terminal-canvas{height:62vh}}
</style>
