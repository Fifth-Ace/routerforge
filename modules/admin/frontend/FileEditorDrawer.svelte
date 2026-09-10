<script>
  export let file = null;
  export let content = '';
  export let dirty = false;
  export let readOnly = false;
  export let busy = false;
  export let locale = 'ru';
  export let onSave = () => {};
  export let onClose = () => {};

  let textarea;
  let gutter;
  let highlightLayer;
  let closing = false;
  let closeTimer = null;

  $: lineCount = Math.max(1, String(content ?? '').split('\n').length);
  $: lineNumbers = Array.from({ length: lineCount }, (_, index) => index + 1).join('\n');
  $: filename = file?.path?.split('/').pop() || 'untitled';
  $: mode = editorMode(filename);
  $: highlighted = highlightSource(String(content ?? ''), mode) + (String(content ?? '').endsWith('\n') ? ' ' : '');
  $: copy = locale === 'ru' ? {
    editor: 'Редактор', save: 'Сохранить', close: 'Закрыть', dirty: 'ИЗМЕНЕНО',
    saved: 'СИНХРОНИЗИРОВАНО', readOnly: 'ТОЛЬКО ЧТЕНИЕ',
    discard: 'Закрыть редактор и потерять несохранённые изменения?',
    hint: 'Ctrl+S — сохранить · Tab — 2 пробела · Esc — закрыть'
  } : {
    editor: 'Editor', save: 'Save', close: 'Close', dirty: 'MODIFIED', saved: 'SYNCED',
    readOnly: 'READ ONLY', discard: 'Close editor and discard unsaved changes?',
    hint: 'Ctrl+S — save · Tab — 2 spaces · Esc — close'
  };

  function editorMode(name) {
    const lower = name.toLowerCase();
    if (lower.endsWith('.json')) return 'JSON';
    if (lower.endsWith('.js') || lower.endsWith('.mjs') || lower.endsWith('.ts') || lower.endsWith('.tsx') || lower.endsWith('.jsx')) return 'JS';
    if (lower.endsWith('.css') || lower.endsWith('.scss')) return 'CSS';
    if (lower.endsWith('.html') || lower.endsWith('.htm') || lower.endsWith('.xml')) return 'HTML';
    if (lower.endsWith('.go')) return 'GO';
    if (lower.endsWith('.py')) return 'PYTHON';
    if (lower.endsWith('.sql')) return 'SQL';
    if (lower.endsWith('.yaml') || lower.endsWith('.yml')) return 'YAML';
    if (lower.endsWith('.toml')) return 'TOML';
    if (lower.endsWith('.md') || lower.endsWith('.markdown')) return 'MARKDOWN';
    if (lower.endsWith('.sh') || lower.endsWith('.bash') || lower.endsWith('.ash') || lower.startsWith('s99') || lower.startsWith('s51') || lower === 'rc.local') return 'SHELL';
    if (lower.endsWith('.conf') || lower.endsWith('.ini') || lower.endsWith('.env') || lower === 'hosts' || lower === 'fstab' || lower === 'resolv.conf') return 'CONF';
    return 'TEXT';
  }

  function escapeHtml(value) {
    return value.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }

  function token(value, cls) { return `<span class="tok-${cls}">${escapeHtml(value)}</span>`; }
  function isIdentStart(ch) { return /[A-Za-z_$]/.test(ch || ''); }
  function isIdent(ch) { return /[A-Za-z0-9_$-]/.test(ch || ''); }
  function isDigit(ch) { return /[0-9]/.test(ch || ''); }

  const keywords = {
    JS: new Set(['const','let','var','function','return','if','else','for','while','switch','case','break','continue','class','new','async','await','try','catch','finally','throw','import','from','export','default','extends','typeof','instanceof','in','of','true','false','null','undefined']),
    GO: new Set(['package','import','func','return','if','else','for','range','switch','case','break','continue','type','struct','interface','map','chan','go','defer','select','var','const','true','false','nil']),
    SHELL: new Set(['if','then','else','elif','fi','for','while','until','do','done','case','esac','in','function','select','time','export','local','readonly','return','exit','break','continue','true','false']),
    PYTHON: new Set(['def','class','return','if','elif','else','for','while','break','continue','try','except','finally','raise','import','from','as','with','lambda','yield','async','await','pass','True','False','None','and','or','not','in','is']),
    SQL: new Set(['select','from','where','join','left','right','inner','outer','on','insert','into','update','delete','create','alter','drop','table','view','index','values','set','and','or','not','null','as','group','by','order','having','limit','offset','union','all','distinct','case','when','then','else','end'])
  };

  function highlightLine(line, mode) {
    if (mode === 'MARKDOWN') {
      if (/^\s*#{1,6}\s/.test(line)) return token(line, 'section');
      if (/^\s*```/.test(line)) return token(line, 'keyword');
      if (/^\s*[-*+]\s/.test(line)) {
        const mark = line.match(/^(\s*[-*+]\s)(.*)$/);
        return `${token(mark[1], 'operator')}${highlightLine(mark[2], 'TEXT')}`;
      }
      return highlightLine(line, 'TEXT');
    }

    if (mode === 'CONF' || mode === 'YAML' || mode === 'TOML' || mode === 'TEXT') {
      if (/^\s*[#;]/.test(line)) return token(line, 'comment');
      const section = line.match(/^(\s*)(\[[^\]]+\])(.*)$/);
      if (section) return `${escapeHtml(section[1])}${token(section[2], 'section')}${escapeHtml(section[3])}`;
      const kv = line.match(/^(\s*)([^=:#\s][^=:]*?)(\s*[=:]\s*)(.*)$/);
      if (kv) return `${escapeHtml(kv[1])}${token(kv[2], 'key')}${token(kv[3], 'operator')}${highlightLine(kv[4], 'GENERIC_VALUE')}`;
    }

    if (mode === 'GENERIC_VALUE' && /^(true|false|yes|no|on|off|null|none)$/i.test(line.trim())) {
      return token(line, 'boolean');
    }

    if (mode === 'HTML') {
      let out = ''; let i = 0;
      while (i < line.length) {
        if (line.startsWith('<!--', i)) { const end = line.indexOf('-->', i + 4); const stop = end < 0 ? line.length : end + 3; out += token(line.slice(i, stop), 'comment'); i = stop; continue; }
        if (line[i] === '<') { const end = line.indexOf('>', i + 1); const stop = end < 0 ? line.length : end + 1; out += token(line.slice(i, stop), 'tag'); i = stop; continue; }
        out += escapeHtml(line[i]); i += 1;
      }
      return out;
    }

    let out = ''; let i = 0;
    while (i < line.length) {
      const ch = line[i];
      const next = line[i + 1] || '';
      const rest = line.slice(i);

      if ((mode === 'SHELL' || mode === 'PYTHON' || mode === 'GENERIC_VALUE' || mode === 'TEXT') && ch === '#') { out += token(line.slice(i), 'comment'); break; }
      if ((mode === 'JS' || mode === 'GO' || mode === 'CSS' || mode === 'SQL' || mode === 'TEXT') && ch === '/' && next === '/') { out += token(line.slice(i), 'comment'); break; }
      if ((mode === 'JS' || mode === 'GO' || mode === 'CSS') && ch === '/' && next === '*') {
        const end = line.indexOf('*/', i + 2); const stop = end < 0 ? line.length : end + 2;
        out += token(line.slice(i, stop), 'comment'); i = stop; continue;
      }

      const url = rest.match(/^(?:https?|ftp):\/\/[^\s"'<>]+/i);
      if (url) { out += token(url[0], 'link'); i += url[0].length; continue; }

      const ipv4 = rest.match(/^(?:\d{1,3}\.){3}\d{1,3}(?::\d+)?/);
      if (ipv4) { out += token(ipv4[0], 'address'); i += ipv4[0].length; continue; }

      const path = rest.match(/^\/(?:[A-Za-z0-9._@+-]+\/?)+/);
      if (path) { out += token(path[0], 'path'); i += path[0].length; continue; }

      if (ch === '"' || ch === "'" || ((mode === 'JS' || mode === 'GO' || mode === 'SHELL' || mode === 'PYTHON') && ch === '`')) {
        const quote = ch; let j = i + 1;
        while (j < line.length) { if (line[j] === '\\') { j += 2; continue; } if (line[j] === quote) { j += 1; break; } j += 1; }
        const value = line.slice(i, j);
        let cls = 'string';
        if (mode === 'JSON' && /^\s*:/.test(line.slice(j))) cls = 'key';
        out += token(value, cls); i = j; continue;
      }

      if (ch === '$') {
        let j = i + 1;
        if (line[j] === '{') { j += 1; while (j < line.length && line[j] !== '}') j += 1; if (line[j] === '}') j += 1; }
        else while (j < line.length && /[A-Za-z0-9_?@#$*!-]/.test(line[j])) j += 1;
        if (j > i + 1) { out += token(line.slice(i, j), 'variable'); i = j; continue; }
      }

      if (isDigit(ch) || ((ch === '-' || ch === '+') && isDigit(next))) {
        let j = i + 1; while (j < line.length && /[0-9A-Fa-fxX._eE+-]/.test(line[j])) j += 1;
        out += token(line.slice(i, j), 'number'); i = j; continue;
      }

      if (isIdentStart(ch)) {
        let j = i + 1; while (j < line.length && isIdent(line[j])) j += 1;
        const word = line.slice(i, j);
        const normalized = mode === 'SQL' ? word.toLowerCase() : word;
        if (mode === 'JSON' && (word === 'true' || word === 'false' || word === 'null')) out += token(word, word === 'null' ? 'null' : 'boolean');
        else if (keywords[mode]?.has(normalized)) out += token(word, ['true','false','null','nil','undefined','True','False','None'].includes(word) ? 'boolean' : 'keyword');
        else if ((mode === 'JS' || mode === 'GO' || mode === 'PYTHON') && /^\s*\(/.test(line.slice(j))) out += token(word, 'function');
        else if (/^(true|false|yes|no|on|off|null|none)$/i.test(word)) out += token(word, 'boolean');
        else out += escapeHtml(word);
        i = j; continue;
      }

      if ('=:+-*/%!<>|&'.includes(ch)) { out += token(ch, 'operator'); i += 1; continue; }
      if (mode === 'CSS' && ch === '#') { let j = i + 1; while (j < line.length && /[A-Za-z0-9_-]/.test(line[j])) j += 1; out += token(line.slice(i, j), 'selector'); i = j; continue; }
      out += escapeHtml(ch); i += 1;
    }
    return out;
  }
  function highlightSource(source, mode) {
    return source.split('\n').map((line) => highlightLine(line, mode)).join('\n');
  }

  function syncScroll(event) {
    if (gutter) gutter.scrollTop = event.currentTarget.scrollTop;
    if (highlightLayer) {
      highlightLayer.scrollTop = event.currentTarget.scrollTop;
      highlightLayer.scrollLeft = event.currentTarget.scrollLeft;
    }
  }

  function requestClose() {
    if (closing) return;
    if (dirty && !confirm(copy.discard)) return;
    closing = true;
    clearTimeout(closeTimer);
    closeTimer = setTimeout(() => onClose(), 260);
  }

  function handleKeydown(event) {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
      event.preventDefault(); if (dirty && !busy && !readOnly) onSave(); return;
    }
    if (event.key === 'Escape') { event.preventDefault(); requestClose(); return; }
    if (event.key === 'Tab' && !readOnly) {
      event.preventDefault();
      const start = textarea.selectionStart; const end = textarea.selectionEnd;
      content = `${content.slice(0, start)}  ${content.slice(end)}`;
      requestAnimationFrame(() => { textarea.selectionStart = start + 2; textarea.selectionEnd = start + 2; });
    }
  }
</script>

<div class:closing class="editor-backdrop" aria-hidden="true"></div>
<section class:closing class="editor-sheet" aria-label={copy.editor}>
  <header class="editor-topbar">
    <div class="editor-title">
      <div class="editor-kicker"><span class="editor-dot"></span><span>{copy.editor}</span><em>{mode}</em></div>
      <strong title={file?.path || ''}>{filename}</strong>
      <span class="editor-path" title={file?.path || ''}>{file?.path || ''}</span>
    </div>
    <div class="editor-actions">
      <span class:dirty class:readonly={readOnly} class="editor-state">{readOnly ? copy.readOnly : (dirty ? copy.dirty : copy.saved)}</span>
      <button type="button" onclick={onSave} disabled={!dirty || busy || readOnly}>{busy ? '…' : copy.save}</button>
      <button class="close" type="button" onclick={requestClose}>×</button>
    </div>
  </header>
  <div class="editor-meta"><span>{file?.mode || '—'}</span><span>{file?.size || 0} B</span><span>UTF-8</span><span>LF</span><span class="spacer"></span><span>{lineCount} lines</span></div>
  <div class="editor-surface">
    <pre class="editor-gutter" bind:this={gutter} aria-hidden="true">{lineNumbers}</pre>
    <div class="editor-code-wrap">
      <pre class="editor-highlight" bind:this={highlightLayer} aria-hidden="true">{@html highlighted}</pre>
      <textarea bind:this={textarea} bind:value={content} class="editor-text" spellcheck="false" readonly={readOnly} onscroll={syncScroll} onkeydown={handleKeydown} aria-label={file?.path || copy.editor}></textarea>
    </div>
  </div>
  <footer class="editor-footer"><span>{copy.hint}</span><span class="spacer"></span><button type="button" onclick={requestClose}>{copy.close}</button></footer>
</section>

<style>
  .editor-backdrop{position:fixed;inset:0;z-index:998;background:rgba(2,7,12,.58);backdrop-filter:blur(1.5px);animation:editor-backdrop-in .30s ease both}
  .editor-backdrop.closing{animation:editor-backdrop-out .24s ease both}
  .editor-sheet{position:fixed;z-index:999;top:0;left:0;right:0;height:75vh;display:grid;grid-template-rows:auto auto 1fr auto;background:#0b1017;color:#d8dee9;border-bottom:1px solid #2a3948;box-shadow:0 24px 65px rgba(0,0,0,.46);font-family:Inter,system-ui,sans-serif;animation:sheet-in .32s cubic-bezier(.22,.72,.2,1) both;will-change:transform,opacity}
  .editor-sheet.closing{pointer-events:none;animation:sheet-out .24s cubic-bezier(.4,0,.8,.2) both}
  @keyframes editor-backdrop-in{from{opacity:0}to{opacity:1}}
  @keyframes editor-backdrop-out{from{opacity:1}to{opacity:0}}
  @keyframes sheet-in{from{transform:translateY(-42px);opacity:0}to{transform:translateY(0);opacity:1}}
  @keyframes sheet-out{from{transform:translateY(0);opacity:1}to{transform:translateY(-36px);opacity:0}}
  .editor-topbar{display:flex;justify-content:space-between;gap:1rem;align-items:center;padding:.85rem 1rem;background:#111923;border-bottom:1px solid #263442}.editor-title{min-width:0;display:flex;flex-direction:column;gap:.16rem}.editor-title strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-family:"Roboto Mono","Cascadia Mono",Consolas,monospace;font-size:.9rem;color:#e6edf3}.editor-kicker{display:flex;align-items:center;gap:.42rem;color:#5de4c7;font-size:.66rem;font-weight:800;letter-spacing:.065em;text-transform:uppercase}.editor-kicker em{border:1px solid #345064;border-radius:.32rem;padding:.08rem .32rem;color:#8bc8ff;font-size:.56rem;font-style:normal}.editor-dot{width:.46rem;height:.46rem;border-radius:50%;background:#7bd88f;box-shadow:0 0 0 3px rgba(123,216,143,.1)}.editor-path{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:#667687;font-family:"Roboto Mono","Cascadia Mono",Consolas,monospace;font-size:.65rem}.editor-actions{display:flex;align-items:center;gap:.42rem;flex:none}
  button{border:1px solid #314253;border-radius:.4rem;background:#17212c;color:#c7d0da;padding:.38rem .58rem;font:inherit;font-size:.7rem;cursor:pointer}button:hover:not(:disabled){background:#202d3a;border-color:#4b657d}button:disabled{opacity:.4;cursor:not-allowed}button.close{width:2rem;height:2rem;padding:0;font-size:1.15rem;line-height:1;border-color:#583941;color:#ff9da4}.editor-state{border:1px solid #2e4a40;border-radius:.34rem;color:#7bd88f;padding:.18rem .38rem;font:700 .58rem/1 "Roboto Mono","Cascadia Mono",Consolas,monospace;letter-spacing:.04em}.editor-state.dirty{color:#ffd866;border-color:#5b512d}.editor-state.readonly{color:#c099ff;border-color:#59446f}
  .editor-meta,.editor-footer{min-height:1.85rem;display:flex;align-items:center;gap:.85rem;padding:0 .85rem;background:#101721;color:#6f7f90;border-bottom:1px solid #1f2c38;font:.61rem/1 "Roboto Mono","Cascadia Mono",Consolas,monospace}.editor-footer{border-top:1px solid #1f2c38;border-bottom:0;min-height:2.2rem}.editor-footer button{padding:.28rem .5rem}.spacer{flex:1}
  .editor-surface{min-height:0;display:grid;grid-template-columns:auto 1fr;background:#0b1017;overflow:hidden}.editor-gutter{box-sizing:border-box;margin:0;min-width:3.8rem;height:100%;overflow:hidden;padding:.9rem .72rem .9rem .4rem;border-right:1px solid #1e2a36;background:#0e151e;color:#4f6275;text-align:right;user-select:none;white-space:pre;font:12.5px/1.55 "Roboto Mono","Cascadia Mono",Consolas,monospace}.editor-code-wrap{position:relative;min-width:0;min-height:0;overflow:hidden;background:#0b1017}.editor-highlight,.editor-text{box-sizing:border-box;position:absolute;inset:0;width:100%;height:100%;margin:0;padding:.9rem 1rem;overflow:auto;white-space:pre;tab-size:2;font:12.5px/1.55 "Roboto Mono","Cascadia Mono",Consolas,monospace}.editor-highlight{z-index:1;border:0;background:#0b1017;color:#c9d1d9;pointer-events:none;scrollbar-width:none}.editor-highlight::-webkit-scrollbar{display:none}.editor-text{z-index:2;resize:none;border:0;outline:0;background:transparent;color:transparent;caret-color:#e6edf3;-webkit-text-fill-color:transparent}.editor-text::selection{background:rgba(72,118,157,.55);-webkit-text-fill-color:transparent}.editor-text:focus{box-shadow:inset 2px 0 0 #5de4c7}.editor-text:read-only{caret-color:#94a0ac}
  :global(.tok-comment){color:#5c7080;font-style:italic}:global(.tok-string){color:#ffd866}:global(.tok-key){color:#8bc8ff}:global(.tok-number){color:#c099ff}:global(.tok-boolean),:global(.tok-null){color:#ff9da4}:global(.tok-keyword){color:#ff7ab2;font-weight:650}:global(.tok-function){color:#82d2ce}:global(.tok-variable){color:#7bd88f}:global(.tok-operator){color:#89ddff}:global(.tok-section){color:#c099ff;font-weight:700}:global(.tok-tag){color:#ff7ab2}:global(.tok-selector){color:#7bd88f}:global(.tok-link){color:#5de4c7;text-decoration:underline}:global(.tok-address){color:#ffb86c}:global(.tok-path){color:#82d2ce}
  @media(max-width:760px){.editor-sheet{height:78vh}.editor-meta span:nth-child(3),.editor-meta span:nth-child(4){display:none}.editor-footer>span:first-child{display:none}}@media(prefers-reduced-motion:reduce){.editor-sheet,.editor-sheet.closing,.editor-backdrop,.editor-backdrop.closing{animation-duration:.01ms}}
</style>
