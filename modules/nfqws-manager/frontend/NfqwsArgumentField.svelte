<script>
  import {highlightCustomStrategy} from './NfqwsCustomSyntax.js';
  import {strategyResources,safeResourceName} from './NfqwsArgumentResourcesModel.js';
  let {id, value='', large=false, busy=false, kind='strategy', baseArgs='', knownLists=[], onEdit, onOpenResource=()=>{}} = $props();
  let resources=$derived(strategyResources(value,{kind,baseArgs,knownLists}));
  let highlight;
  function scroll(e){if(highlight){highlight.scrollTop=e.currentTarget.scrollTop;highlight.scrollLeft=e.currentTarget.scrollLeft;}}
  function tabKey(e){if(e.key!=='Tab')return;e.preventDefault();const t=e.currentTarget,a=t.selectionStart,b=t.selectionEnd;
    onEdit(t.value.slice(0,a)+'  '+t.value.slice(b));
    queueMicrotask(()=>{if(t.isConnected){t.focus();t.setSelectionRange(a+2,a+2);}});
  }
</script>
<div class="syntax-editor-shell" data-svelte-owned="nfqws-argument-field">
  <div class="syntax-editor-toolbar"><span class="syntax-editor-caption"><strong>{id}</strong><span class="syntax-editor-hint">NFQWS syntax · Ctrl+S → Diff</span></span></div>
  <div class="syntax-editor-body">
    <pre class="syntax-editor-highlight" aria-hidden="true" bind:this={highlight}>{@html highlightCustomStrategy(value)}</pre>
    <textarea {id} class="setup-textarea" class:large disabled={busy} spellcheck="false" {value} oninput={e=>onEdit(e.currentTarget.value)} onscroll={scroll} onkeydown={tabKey}></textarea>
  </div>
</div>
{#if resources.length}
  <div class="syntax-editor-resources" aria-label="Ссылки на ресурсы только для просмотра">
    <span class="resource-label">Ресурсы</span>
    {#each resources as res (res.type+':'+res.name+':'+res.path)}
      {#if (res.type==='list'||res.type==='lua')&&safeResourceName(res.name)}
        <button type="button" class:good={res.status==='available'} class:warn={res.status==='unverified'} class="resource-chip" disabled={busy} title={res.path} onclick={()=>onOpenResource(res.type,res.name)}>{res.type} {res.name} {res.status==='available'?'✓':res.status==='unverified'?'?':''}</button>
      {:else}<span class="resource-chip" class:warn={res.status==='unverified'} class:info={res.type==='blobdef'||res.type==='lua'} title={res.path}>{res.type} {res.name} {res.status==='unverified'?'?':'✓'}</span>{/if}
    {/each}
  </div>
{/if}
