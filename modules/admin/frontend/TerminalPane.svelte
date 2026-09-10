<script>
  import { onDestroy, onMount } from 'svelte';
  import { Terminal } from '@xterm/xterm';
  import { FitAddon } from '@xterm/addon-fit';
  import '@xterm/xterm/css/xterm.css';
  import {
    adminTerminalCreate,
    adminTerminalClose,
    adminTerminalInput,
    adminTerminalOutput,
    adminTerminalResize
  } from '$lib/api.js';

  export let locale = 'ru';

  const SESSION_KEY = 'routerforge.admin.terminal.session';
  const CURSOR_KEY = 'routerforge.admin.terminal.cursor';
  const CLIENT_KEY = 'routerforge.admin.terminal.client';

  let host;
  let terminal;
  let fitAddon;
  let resizeObserver;
  let sessionID = '';
  let cursor = 0;
  let connected = false;
  let connecting = false;
  let statusText = 'OFFLINE';
  let dimensions = '80×24';
  let pollTimer;
  let inputTimer;
  let resizeTimer;
  let pendingInput = '';
  let disposed = false;
  let clientID = '';

  $: copy = locale === 'ru' ? {
    title: 'Entware Terminal',
    hint: 'Интерактивная root-shell с PTY · ANSI / UTF-8 · среда /opt',
    connect: 'Подключить',
    disconnect: 'Отключить',
    reconnect: 'Переподключить',
    clear: 'Очистить',
    entware: 'Entware',
    keenetic: 'Keenetic',
    keeneticHint: 'Консоль Keenetic будет подключена отдельным безопасным адаптером.',
    connecting: 'CONNECTING',
    connected: 'CONNECTED',
    disconnected: 'DISCONNECTED',
    expired: 'SESSION EXPIRED'
  } : {
    title: 'Entware Terminal',
    hint: 'Interactive root PTY shell · ANSI / UTF-8 · /opt environment',
    connect: 'Connect',
    disconnect: 'Disconnect',
    reconnect: 'Reconnect',
    clear: 'Clear',
    entware: 'Entware',
    keenetic: 'Keenetic',
    keeneticHint: 'Keenetic console will use a separate guarded adapter.',
    connecting: 'CONNECTING',
    connected: 'CONNECTED',
    disconnected: 'DISCONNECTED',
    expired: 'SESSION EXPIRED'
  };

  const theme = {
    background: '#0b1017',
    foreground: '#d8dee9',
    cursor: '#f8f8f2',
    cursorAccent: '#0b1017',
    selectionBackground: '#29445f',
    black: '#1b1f24',
    red: '#ff6b6b',
    green: '#7bd88f',
    yellow: '#ffd866',
    blue: '#6cb6ff',
    magenta: '#c099ff',
    cyan: '#5de4c7',
    white: '#e6edf3',
    brightBlack: '#59636e',
    brightRed: '#ff8787',
    brightGreen: '#97e6a8',
    brightYellow: '#ffe58f',
    brightBlue: '#8bc8ff',
    brightMagenta: '#d0b3ff',
    brightCyan: '#82ead5',
    brightWhite: '#ffffff'
  };

  function decodeBase64(value) {
    if (!value) return new Uint8Array(0);
    const raw = atob(value);
    const bytes = new Uint8Array(raw.length);
    for (let i = 0; i < raw.length; i += 1) bytes[i] = raw.charCodeAt(i);
    return bytes;
  }

  function safeStoredCursor() {
    const value = Number(sessionStorage.getItem(CURSOR_KEY) || '0');
    return Number.isFinite(value) && value >= 0 ? value : 0;
  }

  function stableClientID() {
    const stored = localStorage.getItem(CLIENT_KEY) || '';
    if (/^[0-9a-f]{32}$/.test(stored)) return stored;

    const bytes = new Uint8Array(16);
    crypto.getRandomValues(bytes);
    const value = Array.from(bytes, (item) => item.toString(16).padStart(2, '0')).join('');
    localStorage.setItem(CLIENT_KEY, value);
    return value;
  }

  function rememberSession() {
    if (sessionID) sessionStorage.setItem(SESSION_KEY, sessionID);
    else sessionStorage.removeItem(SESSION_KEY);
    sessionStorage.setItem(CURSOR_KEY, String(cursor));
  }

  function setConnection(value, label) {
    connected = value;
    statusText = label;
  }

  function terminalBanner() {
    terminal?.writeln('\x1b[1;36mRouterForge Entware Terminal\x1b[0m');
    terminal?.writeln('\x1b[38;5;244mPTY · xterm-256color · /opt environment\x1b[0m');
    terminal?.writeln('');
  }

  async function createSession() {
    if (!terminal || connecting || connected) return;
    connecting = true;
    setConnection(false, copy.connecting);
    try {
      fitAddon?.fit();
      const result = await adminTerminalCreate('/opt', terminal.cols || 80, terminal.rows || 24, clientID);
      sessionID = result.session_id;
      cursor = 0;
      rememberSession();
      terminal.clear();
      terminalBanner();
      setConnection(true, copy.connected);
      schedulePoll(0);
    } catch (error) {
      setConnection(false, copy.disconnected);
      terminal?.writeln(`\x1b[1;31m${error?.payload?.error || error?.message || 'Terminal connection failed'}\x1b[0m`);
    } finally {
      connecting = false;
    }
  }

  async function resumeSession() {
    const stored = sessionStorage.getItem(SESSION_KEY) || '';
    if (!stored) {
      await createSession();
      return;
    }
    sessionID = stored;
    cursor = safeStoredCursor();
    setConnection(false, copy.connecting);
    try {
      const result = await adminTerminalOutput(sessionID, cursor);
      if (result.dropped) {
        terminal?.writeln('\x1b[1;33m[scrollback window advanced while this view was detached]\x1b[0m');
      }
      const data = decodeBase64(result.data_base64);
      if (data.length) terminal.write(data);
      cursor = Number(result.cursor || cursor);
      rememberSession();
      if (result.closed) {
        setConnection(false, copy.expired);
        sessionStorage.removeItem(SESSION_KEY);
        return;
      }
      setConnection(true, copy.connected);
      await sendResize();
      schedulePoll(0);
    } catch (error) {
      if (error?.status === 404) {
        sessionID = '';
        cursor = 0;
        sessionStorage.removeItem(SESSION_KEY);
        sessionStorage.removeItem(CURSOR_KEY);
        await createSession();
        return;
      }
      setConnection(false, copy.disconnected);
      terminal?.writeln(`\x1b[1;31m${error?.payload?.error || error?.message || 'Terminal reconnect failed'}\x1b[0m`);
    }
  }

  function schedulePoll(delay = 120) {
    clearTimeout(pollTimer);
    if (disposed || !sessionID) return;
    pollTimer = setTimeout(pollOutput, delay);
  }

  async function pollOutput() {
    if (disposed || !sessionID) return;
    try {
      const result = await adminTerminalOutput(sessionID, cursor);
      if (result.dropped) {
        terminal?.writeln('\r\n\x1b[1;33m[output truncated: terminal scrollback window advanced]\x1b[0m');
      }
      const data = decodeBase64(result.data_base64);
      if (data.length) terminal.write(data);
      cursor = Number(result.cursor || cursor);
      rememberSession();

      if (result.closed) {
        setConnection(false, `${copy.disconnected} · exit ${result.exit_code ?? 0}`);
        sessionStorage.removeItem(SESSION_KEY);
        sessionID = '';
        return;
      }
      setConnection(true, copy.connected);
    } catch (error) {
      if (error?.status === 404) {
        setConnection(false, copy.expired);
        sessionStorage.removeItem(SESSION_KEY);
        sessionID = '';
        return;
      }
      setConnection(false, copy.disconnected);
    }
    schedulePoll();
  }

  function queueInput(data) {
    if (!connected || !sessionID) return;
    pendingInput += data;
    if (pendingInput.length >= 512) {
      flushInput();
      return;
    }
    if (!inputTimer) {
      inputTimer = setTimeout(flushInput, 18);
    }
  }

  async function flushInput() {
    clearTimeout(inputTimer);
    inputTimer = null;
    if (!sessionID || !pendingInput) return;
    const data = pendingInput.slice(0, 4096);
    pendingInput = pendingInput.slice(data.length);
    try {
      await adminTerminalInput(sessionID, data);
    } catch (error) {
      setConnection(false, copy.disconnected);
      terminal?.writeln(`\r\n\x1b[1;31m${error?.payload?.error || error?.message || 'input failed'}\x1b[0m`);
    }
    if (pendingInput) inputTimer = setTimeout(flushInput, 0);
  }

  async function sendResize() {
    clearTimeout(resizeTimer);
    resizeTimer = null;
    if (!terminal) return;
    dimensions = `${terminal.cols}×${terminal.rows}`;
    if (!sessionID || !connected) return;
    try {
      await adminTerminalResize(sessionID, terminal.cols, terminal.rows);
    } catch {}
  }

  function queueResize() {
    if (!terminal || !fitAddon) return;
    try { fitAddon.fit(); } catch {}
    dimensions = `${terminal.cols}×${terminal.rows}`;
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(sendResize, 90);
  }

  async function disconnect() {
    const id = sessionID;
    sessionID = '';
    cursor = 0;
    connected = false;
    sessionStorage.removeItem(SESSION_KEY);
    sessionStorage.removeItem(CURSOR_KEY);
    clearTimeout(pollTimer);
    if (id) {
      try { await adminTerminalClose(id); } catch {}
    }
    terminal?.writeln(`\r\n\x1b[38;5;244m[${copy.disconnected}]\x1b[0m`);
    statusText = copy.disconnected;
  }

  async function reconnect() {
    await disconnect();
    terminal?.clear();
    terminalBanner();
    await createSession();
  }

  onMount(() => {
    clientID = stableClientID();

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
      letterSpacing: 0,
      scrollback: 5000,
      tabStopWidth: 4,
      theme
    });

    fitAddon = new FitAddon();
    terminal.loadAddon(fitAddon);
    terminal.open(host);
    terminalBanner();

    terminal.onData(queueInput);
    terminal.onResize(() => {
      dimensions = `${terminal.cols}×${terminal.rows}`;
    });

    resizeObserver = new ResizeObserver(queueResize);
    resizeObserver.observe(host);
    queueResize();
    resumeSession();
  });

  onDestroy(() => {
    disposed = true;
    clearTimeout(pollTimer);
    clearTimeout(inputTimer);
    clearTimeout(resizeTimer);
    resizeObserver?.disconnect();
    terminal?.dispose();
  });
</script>

<section class="terminal-shell">
  <header class="terminal-topbar">
    <div class="terminal-tabs">
      <button class="terminal-tab active" type="button">
        <span class="terminal-dot connected-dot"></span>
        {copy.entware}
      </button>
      <button class="terminal-tab" type="button" disabled title={copy.keeneticHint}>
        <span class="terminal-dot"></span>
        {copy.keenetic}
        <em>next</em>
      </button>
    </div>

    <div class="terminal-head-status">
      <span class:online={connected} class="status-dot"></span>
      <strong>{statusText}</strong>
      <span class="mono">{dimensions}</span>
    </div>
  </header>

  <div class="terminal-titlebar">
    <div>
      <strong>{copy.title}</strong>
      <span>{copy.hint}</span>
    </div>
    <div class="terminal-actions">
      <button type="button" onclick={() => terminal?.clear()}>{copy.clear}</button>
      {#if connected}
        <button type="button" onclick={reconnect}>{copy.reconnect}</button>
        <button class="danger" type="button" onclick={disconnect}>{copy.disconnect}</button>
      {:else}
        <button type="button" onclick={createSession} disabled={connecting}>{connecting ? '…' : copy.connect}</button>
      {/if}
    </div>
  </div>

  <div class="terminal-bezel">
    <div class="terminal-canvas" bind:this={host}></div>
  </div>

  <footer class="terminal-statusbar">
    <span><b>root</b>@entware</span>
    <span>/opt</span>
    <span>UTF-8</span>
    <span>xterm-256color</span>
    <span class="terminal-status-spacer"></span>
    <span>{dimensions}</span>
  </footer>
</section>

<style>
  :global(.terminal-shell .xterm){height:100%}
  :global(.terminal-shell .xterm-viewport){scrollbar-color:#3b5065 #0b1017}
  :global(.terminal-shell .xterm-screen){padding:.15rem 0}

  .terminal-shell{
    overflow:hidden;
    border:1px solid #263442;
    border-radius:.72rem;
    background:#0b1017;
    box-shadow:0 18px 50px rgba(0,0,0,.22), inset 0 1px 0 rgba(255,255,255,.035);
  }
  .terminal-topbar{
    min-height:2.35rem;
    display:flex;
    align-items:stretch;
    justify-content:space-between;
    background:#151c25;
    border-bottom:1px solid #263442;
  }
  .terminal-tabs{display:flex;min-width:0}
  .terminal-tab{
    display:flex;
    align-items:center;
    gap:.48rem;
    padding:0 .9rem;
    border:0;
    border-right:1px solid #263442;
    background:#111821;
    color:#8d9aaa;
    font:inherit;
    font-size:.8rem;
  }
  .terminal-tab.active{
    color:#e6edf3;
    background:#0b1017;
    box-shadow:inset 0 2px 0 #5de4c7;
  }
  .terminal-tab:disabled{cursor:not-allowed;opacity:.52}
  .terminal-tab em{
    padding:.08rem .3rem;
    border:1px solid #344556;
    border-radius:.3rem;
    font-size:.58rem;
    font-style:normal;
    text-transform:uppercase;
  }
  .terminal-dot,.status-dot{
    width:.45rem;
    height:.45rem;
    border-radius:50%;
    background:#59636e;
    flex:none;
  }
  .connected-dot,.status-dot.online{
    background:#7bd88f;
    box-shadow:0 0 0 3px rgba(123,216,143,.1);
  }
  .terminal-head-status{
    display:flex;
    align-items:center;
    gap:.55rem;
    padding:0 .85rem;
    color:#7c8997;
    font-size:.68rem;
    letter-spacing:.045em;
  }
  .terminal-head-status strong{color:#9ca9b7;font-size:.65rem}
  .terminal-head-status .online+strong{color:#7bd88f}

  .terminal-titlebar{
    display:flex;
    align-items:center;
    justify-content:space-between;
    gap:1rem;
    padding:.72rem .9rem;
    background:#101721;
    border-bottom:1px solid #1e2a36;
    color:#d8dee9;
  }
  .terminal-titlebar>div:first-child{display:flex;flex-direction:column;gap:.15rem;min-width:0}
  .terminal-titlebar strong{font-size:.88rem}
  .terminal-titlebar span{color:#728090;font-size:.7rem;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .terminal-actions{display:flex;gap:.4rem;flex-wrap:wrap;justify-content:flex-end}
  .terminal-actions button{
    border:1px solid #314253;
    border-radius:.38rem;
    background:#17212c;
    color:#c7d0da;
    padding:.31rem .52rem;
    font:inherit;
    font-size:.68rem;
    cursor:pointer;
  }
  .terminal-actions button:hover{background:#202d3a;border-color:#476078}
  .terminal-actions button.danger{border-color:#603b43;color:#ff9da4}
  .terminal-actions button:disabled{opacity:.45;cursor:not-allowed}

  .terminal-bezel{padding:.65rem .72rem .35rem;background:#0b1017}
  .terminal-canvas{
    width:100%;
    height:clamp(30rem,61vh,48rem);
    overflow:hidden;
  }
  .terminal-statusbar{
    display:flex;
    align-items:center;
    gap:.9rem;
    min-height:1.85rem;
    padding:0 .8rem;
    border-top:1px solid #1e2a36;
    background:#101721;
    color:#687787;
    font-family:"Roboto Mono","Cascadia Mono",Consolas,monospace;
    font-size:.62rem;
  }
  .terminal-statusbar b{color:#7bd88f;font-weight:700}
  .terminal-status-spacer{flex:1}

  @media(max-width:800px){
    .terminal-head-status span.mono{display:none}
    .terminal-titlebar span{display:none}
    .terminal-canvas{height:62vh}
    .terminal-statusbar span:nth-child(3),
    .terminal-statusbar span:nth-child(4){display:none}
  }
</style>
