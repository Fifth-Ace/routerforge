<script>
  import {editCustom, customSummaryTokens} from './NfqwsCustomStrategiesModel.js';
  import {highlightCustomStrategy} from './NfqwsCustomSyntax.js';
  let {snapshot, onChange, busy=false} = $props();
  let fullscreen = $state(-1);
  let highlights = {};
  function syncScroll(e,i) {const pre=highlights[i];if(pre){pre.scrollTop=e.currentTarget.scrollTop;pre.scrollLeft=e.currentTarget.scrollLeft;}}
  function act(command,index=-1,value='') {
    if(busy)return;
    if(command==='remove'&&!window.confirm(`Удалить custom-блок ${index+1}?`))return;
    onChange(editCustom(snapshot,command,index,value));
    if(command==='remove'&&fullscreen===index)fullscreen=-1;
  }
  function tabKey(e,index) {
    if(e.key==='Escape'&&fullscreen===index){fullscreen=-1;return;}
    if(e.key!=='Tab')return;
    e.preventDefault();const el=e.currentTarget;
    const a=el.selectionStart,b=el.selectionEnd;
    act('text',index,el.value.slice(0,a)+'  '+el.value.slice(b));
    queueMicrotask(()=>{if(el.isConnected){el.focus();el.setSelectionRange(a+2,a+2);}});
  }
</script>
<div id="customBlocks" class="custom-blocks" data-svelte-owned="custom-strategies">
  {#each snapshot.blocks as value, i (i)}
    <div class="custom-block" data-index={i}>
      <div class="custom-block-head">
        <div><div class="custom-block-title">{snapshot.labels[i] || `Custom-блок ${i+1}`}</div><div class="muted small">NFQWS_ARGS_CUSTOM{snapshot.blocks.length>1 ? ` · позиция ${i+1}/${snapshot.blocks.length}` : ''}</div></div>
        <div class="custom-block-actions">
          <button type="button" class="custom-up" disabled={busy||i===0} onclick={()=>act('up',i)} aria-label={`Поднять блок ${i+1}`}>↑</button>
          <button type="button" class="custom-down" disabled={busy||i===snapshot.blocks.length-1} onclick={()=>act('down',i)} aria-label={`Опустить блок ${i+1}`}>↓</button>
          <button type="button" class="custom-clone" disabled={busy} onclick={()=>act('clone',i)}>Клонировать</button>
          <button type="button" class="custom-delete danger" disabled={busy} onclick={()=>act('remove',i)}>Удалить</button>
        </div>
      </div>
      <div class="custom-label-row"><label for={`nfqws-custom-label-${i}`}>Название блока</label><input id={`nfqws-custom-label-${i}`} class="custom-label-input" value={snapshot.labels[i] || ''} maxlength="120" disabled={busy} placeholder="Например: YouTube / основной обход" oninput={e=>act('label',i,e.currentTarget.value)} /></div>
      <div class="custom-live-summary setup-chips">
        {#each customSummaryTokens(value) as chip (chip.key)}<span class={`setup-chip ${chip.type}`}>{chip.label} {chip.value}</span>{:else}<span class="setup-chip">Без краткого summary</span>{/each}
      </div>
      <div class="syntax-editor-shell" class:syntax-fullscreen={fullscreen===i}>
        <div class="syntax-editor-toolbar"><span class="syntax-editor-caption"><strong>{snapshot.labels[i] || `CUSTOM #${i+1}`}</strong><span class="syntax-editor-hint">NFQWS syntax · Ctrl+S → Diff</span></span><div class="syntax-editor-actions"><button type="button" disabled={busy} onclick={()=>fullscreen=fullscreen===i?-1:i}>{fullscreen===i?'Закрыть fullscreen':'На весь экран'}</button></div></div>
        <div class="syntax-editor-body"><pre class="syntax-editor-highlight" aria-hidden="true" bind:this={highlights[i]}>{@html highlightCustomStrategy(value)}</pre><textarea spellcheck="false" disabled={busy} aria-label={`Аргументы custom-блока ${i+1}`} value={value} oninput={e=>act('text',i,e.currentTarget.value)} onkeydown={e=>tabKey(e,i)} onscroll={e=>syncScroll(e,i)}></textarea></div>
      </div>
      <div class="label-help">Название сохраняется в raw-конфиге как безопасный комментарий <span class="mono"># RouterForge:</span> и не передаётся nfqws2 как аргумент.</div>
    </div>
  {:else}<div class="empty">Custom-стратегий нет. Это нормально — базовые HTTPS / QUIC / UDP выше работают независимо.</div>{/each}
</div>
<button type="button" id="addCustomBlock" class="custom-add" disabled={busy} onclick={()=>act('add')}>+ Добавить custom-блок</button>
