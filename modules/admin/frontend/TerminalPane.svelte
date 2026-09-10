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

  $: copy = locale === 'ru' ? {
    title: 'Entware Terminal',
    hint: 'WebSocket · PTY · /opt/bin/sh -il · ANSI / UTF-8',
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
    hint: 'WebSocket · PTY · /opt/bin/sh -il · ANSI / UTF-8',
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

  const theme = {
    background: '#0b1017', foreground: '#d8dee9', cursor: '#f8f8f2',
    cursorAccent: '#0b1017', selectionBackground: '#29445f',
    black: '#1b1f24', red: '#ff6b6b', green: '#7bd88f', yellow: '#ffd866',
    blue: '#6cb6ff', magenta: '#c099ff', cyan: '#5de4c7', white: '#e6edf3',
    brightBlack: '#59636e', brightRed: '#ff8787', brightGreen: '#97e6a8',
    brightYellow: '#ffe58f', brightBlue: '#8bc8ff', brightMagenta: '#d0b3ff',
    brightCyan: '#82ead5', brightWhite: '#ffffff'
  };

  function setState(value, label) {
    connected = value;
    statusText = label;
  }

  function terminalBanner() {
    terminal?.writeln('\x1b[1;36mRouterForge Entware Terminal\x1b[0m');
    terminal?.writeln('\x1b[38;5;244mWebSocket · PTY · /opt/bin/sh -il\x1b[0m');
    terminal?.writeln('');
  }

  function socketURL() {
    const scheme = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const cols = terminal?.cols || 80;
    const rows = terminal?.rows || 24;
    return `${scheme}//${window.location.host}/api/modules/admin/terminal/ws?cwd=${encodeURIComponent('/opt')}&cols=${cols}&rows=${rows}`;
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
      if (event.data instanceof ArrayBuffer) {
        terminal.write(new Uint8Array(event.data));
      } else if (typeof event.data === 'string') {
        terminal.write(event.data);
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
      theme
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
      <button class="terminal-tab active" type="button">
        <span class="terminal-dot connected-dot"></span>{copy.entware}
      </button>
      <button class="terminal-tab" type="button" disabled title={copy.keeneticHint}>
        <span class="terminal-dot"></span>{copy.keenetic}<em>next</em>
      </button>
    </div>
    <div class="terminal-head-status">
      <span class:online={connected} class="status-dot"></span>
      <strong>{statusText}</strong><span class="mono">{dimensions}</span>
    </div>
  </header>

  <div class="terminal-titlebar">
    <div><strong>{copy.title}</strong><span>{copy.hint}</span></div>
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
    <span><b>root</b>@entware</span><span>/opt</span><span>UTF-8</span><span>WS / PTY</span>
    <span class="terminal-status-spacer"></span><span>{dimensions}</span>
  </footer>
</section>

<style>
  :global(.terminal-shell .xterm){height:100%}
  :global(.terminal-shell .xterm-viewport){scrollbar-color:#3b5065 #0b1017}
  .terminal-shell{overflow:hidden;border:1px solid #263442;border-radius:.72rem;background:#0b1017;box-shadow:0 18px 50px rgba(0,0,0,.22)}
  .terminal-topbar{min-height:2.35rem;display:flex;align-items:stretch;justify-content:space-between;background:#151c25;border-bottom:1px solid #263442}
  .terminal-tabs{display:flex;min-width:0}
  .terminal-tab{display:flex;align-items:center;gap:.48rem;padding:0 .9rem;border:0;border-right:1px solid #263442;background:#111821;color:#8d9aaa;font:inherit;font-size:.8rem}
  .terminal-tab.active{color:#e6edf3;background:#0b1017;box-shadow:inset 0 2px 0 #5de4c7}
  .terminal-tab:disabled{cursor:not-allowed;opacity:.52}
  .terminal-tab em{padding:.08rem .3rem;border:1px solid #344556;border-radius:.3rem;font-size:.58rem;font-style:normal;text-transform:uppercase}
  .terminal-dot,.status-dot{width:.45rem;height:.45rem;border-radius:50%;background:#59636e;flex:none}
  .connected-dot,.status-dot.online{background:#7bd88f;box-shadow:0 0 0 3px rgba(123,216,143,.1)}
  .terminal-head-status{display:flex;align-items:center;gap:.55rem;padding:0 .85rem;color:#7c8997;font-size:.68rem;letter-spacing:.045em}
  .terminal-head-status strong{color:#9ca9b7;font-size:.65rem}
  .terminal-head-status .online+strong{color:#7bd88f}
  .terminal-titlebar{display:flex;align-items:center;justify-content:space-between;gap:1rem;padding:.72rem .9rem;background:#101721;border-bottom:1px solid #1e2a36;color:#d8dee9}
  .terminal-titlebar>div:first-child{display:flex;flex-direction:column;gap:.15rem;min-width:0}
  .terminal-titlebar strong{font-size:.88rem}
  .terminal-titlebar span{color:#728090;font-size:.7rem;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .terminal-actions{display:flex;gap:.4rem;flex-wrap:wrap;justify-content:flex-end}
  .terminal-actions button{border:1px solid #314253;border-radius:.38rem;background:#17212c;color:#c7d0da;padding:.31rem .52rem;font:inherit;font-size:.68rem;cursor:pointer}
  .terminal-actions button:hover{background:#202d3a;border-color:#476078}
  .terminal-actions button.danger{border-color:#603b43;color:#ff9da4}
  .terminal-actions button:disabled{opacity:.45;cursor:not-allowed}
  .terminal-bezel{padding:.65rem .72rem .35rem;background:#0b1017}
  .terminal-canvas{width:100%;height:clamp(30rem,61vh,48rem);overflow:hidden}
  .terminal-statusbar{display:flex;align-items:center;gap:.9rem;min-height:1.85rem;padding:0 .8rem;border-top:1px solid #1e2a36;background:#101721;color:#687787;font-family:"Roboto Mono","Cascadia Mono",Consolas,monospace;font-size:.62rem}
  .terminal-statusbar b{color:#7bd88f;font-weight:700}
  .terminal-status-spacer{flex:1}
  @media(max-width:800px){.terminal-head-status span.mono{display:none}.terminal-titlebar span{display:none}.terminal-canvas{height:62vh}}
</style>
