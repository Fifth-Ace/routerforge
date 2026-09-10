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

  $: lineCount = Math.max(1, String(content ?? '').split('\n').length);
  $: lineNumbers = Array.from({ length: lineCount }, (_, index) => index + 1).join('\n');
  $: filename = file?.path?.split('/').pop() || 'untitled';
  $: mode = editorMode(filename);
  $: copy = locale === 'ru' ? {
    editor:'Редактор', save:'Сохранить', close:'Закрыть', dirty:'ИЗМЕНЕНО',
    saved:'СИНХРОНИЗИРОВАНО', readOnly:'ТОЛЬКО ЧТЕНИЕ',
    discard:'Закрыть редактор и потерять несохранённые изменения?',
    hint:'Ctrl+S — сохранить · Tab — 2 пробела · Esc — закрыть'
  } : {
    editor:'Editor', save:'Save', close:'Close', dirty:'MODIFIED', saved:'SYNCED',
    readOnly:'READ ONLY', discard:'Close editor and discard unsaved changes?',
    hint:'Ctrl+S — save · Tab — 2 spaces · Esc — close'
  };

  function editorMode(name) {
    const lower=name.toLowerCase();
    if(lower.endsWith('.json')) return 'JSON';
    if(lower.endsWith('.js')||lower.endsWith('.mjs')) return 'JS';
    if(lower.endsWith('.css')) return 'CSS';
    if(lower.endsWith('.html')||lower.endsWith('.htm')) return 'HTML';
    if(lower.endsWith('.go')) return 'GO';
    if(lower.endsWith('.sh')||lower.endsWith('.conf')||lower.endsWith('.ini')) return 'SHELL / CONF';
    return 'TEXT';
  }
  function syncScroll(event){ if(gutter) gutter.scrollTop=event.currentTarget.scrollTop; }
  function requestClose(){ if(dirty&&!confirm(copy.discard)) return; onClose(); }
  function handleKeydown(event){
    if((event.ctrlKey||event.metaKey)&&event.key.toLowerCase()==='s'){
      event.preventDefault(); if(dirty&&!busy&&!readOnly) onSave(); return;
    }
    if(event.key==='Escape'){ event.preventDefault(); requestClose(); return; }
    if(event.key==='Tab'&&!readOnly){
      event.preventDefault();
      const start=textarea.selectionStart,end=textarea.selectionEnd;
      content=`${content.slice(0,start)}  ${content.slice(end)}`;
      requestAnimationFrame(()=>{textarea.selectionStart=start+2;textarea.selectionEnd=start+2;});
    }
  }
</script>

<div class="editor-backdrop" aria-hidden="true"></div>
<section class="editor-sheet" aria-label={copy.editor}>
  <header class="editor-topbar">
    <div class="editor-title">
      <div class="editor-kicker"><span class="editor-dot"></span><span>{copy.editor}</span><em>{mode}</em></div>
      <strong title={file?.path || ''}>{filename}</strong>
      <span class="editor-path" title={file?.path || ''}>{file?.path || ''}</span>
    </div>
    <div class="editor-actions">
      <span class:dirty class:readonly={readOnly} class="editor-state">{readOnly ? copy.readOnly : (dirty ? copy.dirty : copy.saved)}</span>
      <button type="button" onclick={onSave} disabled={!dirty||busy||readOnly}>{busy?'…':copy.save}</button>
      <button class="close" type="button" onclick={requestClose}>×</button>
    </div>
  </header>
  <div class="editor-meta"><span>{file?.mode||'—'}</span><span>{file?.size||0} B</span><span>UTF-8</span><span>LF</span><span class="spacer"></span><span>{lineCount} lines</span></div>
  <div class="editor-surface">
    <pre class="editor-gutter" bind:this={gutter} aria-hidden="true">{lineNumbers}</pre>
    <textarea bind:this={textarea} bind:value={content} class="editor-text" spellcheck="false" readonly={readOnly} onscroll={syncScroll} onkeydown={handleKeydown} aria-label={file?.path||copy.editor}></textarea>
  </div>
  <footer class="editor-footer"><span>{copy.hint}</span><span class="spacer"></span><button type="button" onclick={requestClose}>{copy.close}</button></footer>
</section>

<style>
  .editor-backdrop{position:fixed;inset:0;z-index:998;background:rgba(2,7,12,.55);backdrop-filter:blur(1.5px)}
  .editor-sheet{position:fixed;z-index:999;top:0;left:0;right:0;height:75vh;display:grid;grid-template-rows:auto auto 1fr auto;background:#0b1017;color:#d8dee9;border-bottom:1px solid #2a3948;box-shadow:0 24px 65px rgba(0,0,0,.46);font-family:Inter,system-ui,sans-serif;animation:sheet-in .18s ease-out}
  @keyframes sheet-in{from{transform:translateY(-22px);opacity:.72}to{transform:translateY(0);opacity:1}}
  .editor-topbar{display:flex;justify-content:space-between;gap:1rem;align-items:center;padding:.85rem 1rem;background:#111923;border-bottom:1px solid #263442}
  .editor-title{min-width:0;display:flex;flex-direction:column;gap:.16rem}
  .editor-title strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-family:"Roboto Mono","Cascadia Mono",Consolas,monospace;font-size:.9rem;color:#e6edf3}
  .editor-kicker{display:flex;align-items:center;gap:.42rem;color:#5de4c7;font-size:.66rem;font-weight:800;letter-spacing:.065em;text-transform:uppercase}
  .editor-kicker em{border:1px solid #345064;border-radius:.32rem;padding:.08rem .32rem;color:#8bc8ff;font-size:.56rem;font-style:normal}
  .editor-dot{width:.46rem;height:.46rem;border-radius:50%;background:#7bd88f;box-shadow:0 0 0 3px rgba(123,216,143,.1)}
  .editor-path{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:#667687;font-family:"Roboto Mono","Cascadia Mono",Consolas,monospace;font-size:.65rem}
  .editor-actions{display:flex;align-items:center;gap:.42rem;flex:none}
  button{border:1px solid #314253;border-radius:.4rem;background:#17212c;color:#c7d0da;padding:.38rem .58rem;font:inherit;font-size:.7rem;cursor:pointer}
  button:hover:not(:disabled){background:#202d3a;border-color:#4b657d} button:disabled{opacity:.4;cursor:not-allowed}
  button.close{width:2rem;height:2rem;padding:0;font-size:1.15rem;line-height:1;border-color:#583941;color:#ff9da4}
  .editor-state{border:1px solid #2e4a40;border-radius:.34rem;color:#7bd88f;padding:.18rem .38rem;font:700 .58rem/1 "Roboto Mono","Cascadia Mono",Consolas,monospace;letter-spacing:.04em}
  .editor-state.dirty{color:#ffd866;border-color:#5b512d}.editor-state.readonly{color:#c099ff;border-color:#59446f}
  .editor-meta,.editor-footer{min-height:1.85rem;display:flex;align-items:center;gap:.85rem;padding:0 .85rem;background:#101721;color:#6f7f90;border-bottom:1px solid #1f2c38;font:.61rem/1 "Roboto Mono","Cascadia Mono",Consolas,monospace}
  .editor-footer{border-top:1px solid #1f2c38;border-bottom:0;min-height:2.2rem}.editor-footer button{padding:.28rem .5rem}.spacer{flex:1}
  .editor-surface{min-height:0;display:grid;grid-template-columns:auto 1fr;background:#0b1017;overflow:hidden}
  .editor-gutter{box-sizing:border-box;margin:0;min-width:3.8rem;height:100%;overflow:hidden;padding:.9rem .72rem .9rem .4rem;border-right:1px solid #1e2a36;background:#0e151e;color:#4f6275;text-align:right;user-select:none;white-space:pre;font:12.5px/1.55 "Roboto Mono","Cascadia Mono",Consolas,monospace}
  .editor-text{box-sizing:border-box;width:100%;height:100%;min-height:0;resize:none;overflow:auto;border:0;outline:0;margin:0;padding:.9rem 1rem;background:#0b1017;color:#d8dee9;caret-color:#7bd88f;tab-size:2;white-space:pre;font:12.5px/1.55 "Roboto Mono","Cascadia Mono",Consolas,monospace}
  .editor-text::selection{background:#29445f;color:#fff}.editor-text:focus{box-shadow:inset 2px 0 0 #5de4c7}.editor-text:read-only{color:#94a0ac}
  @media(max-width:760px){.editor-sheet{height:78vh}.editor-meta span:nth-child(3),.editor-meta span:nth-child(4){display:none}.editor-footer>span:first-child{display:none}}
  @media(prefers-reduced-motion:reduce){.editor-sheet{animation:none}}
</style>
