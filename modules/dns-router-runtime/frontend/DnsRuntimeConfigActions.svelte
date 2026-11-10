<script>
  import {configFlow} from './DnsRuntimeCommands.js';
  import {draft,feedback} from './DnsRuntimeStore.svelte.js';
  let {kind} = $props();
  let busy=$state(false);
  let error=$state('');
  let outcome=$state('Нет импорта');
  let domainText=$state('');
  let upstream=$state('');
  let match=$state('suffix');
  let mode=$state('append');
  let upstreams=$derived((draft.config.upstreams||[]).map(u=>String(u.id||'')).filter(Boolean));
  $effect(()=>{if(!upstreams.includes(upstream))upstream=upstreams[0]||'';});
  let stopped=false;

  const flow=configFlow;
  async function perform(action) {
    if(busy||stopped)return;
    busy=true;error='';
    try {
      if(action==='save')await flow.save();
      else if(action==='reload')await flow.reload();
      else if(action==='import'){
        const result=await flow.previewImport({text:domainText,upstream,match,mode});
        if(!stopped)outcome=result.summary;
      } else throw Error('UNKNOWN_CONFIG_ACTION');
    } catch(e) {
      if(!stopped){error=String(e?.message||e);feedback(error);}
    } finally {if(!stopped)busy=false;}
  }
</script>

{#if kind==='save'}
  <button id="saveButton" type="button" class="button primary" disabled={busy} onclick={()=>perform('save')}>Сохранить изменения</button>
{:else if kind==='reload'}
  <button id="reloadButton" type="button" class="button" disabled={busy} onclick={()=>perform('reload')}>↻ Обновить</button>
{:else if kind==='import'}
  <form id="importForm" class="form-grid" onsubmit={e=>{e.preventDefault();void perform('import');}}>
    <label class="span-all"><span>Домены — по одному на строку, комментарии через #</span><textarea name="text" rows="6" required placeholder="example.com&#10;another.net" bind:value={domainText}></textarea></label>
    <label><span>Upstream</span><select name="upstream" id="importUpstream" required bind:value={upstream}>
      {#each upstreams as item (item)}<option value={item}>{item}</option>{/each}
    </select></label>
    <label><span>Совпадение</span><select name="match" bind:value={match}><option value="suffix">Домен и поддомены</option><option value="exact">Точное</option><option value="subdomains">Только поддомены</option></select></label>
    <label><span>Режим</span><select name="mode" bind:value={mode}><option value="append">Дополнить</option><option value="replace">Заменить правила</option></select></label>
    <div class="form-actions"><button class="button" type="submit" disabled={busy||!upstream}>Предпросмотр импорта</button></div>
  </form>
  <pre class="console" id="importResult">{outcome}</pre>
{/if}
{#if error}<p class="notice bad" role="alert">{error}</p>{/if}
