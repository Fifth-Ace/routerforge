<script>
  import { rfUi } from './rf-localization.js';
  import {onMount} from 'svelte';
  import {configFlow} from './DnsRuntimeCommands.js';
  import {draft,feedback} from './DnsRuntimeStore.svelte.js';
  let {kind} = $props();
  let busy=$state(false);
  let error=$state('');
  let outcome=$state(rfUi('Нет импорта',"No import"));
  let domainText=$state('');
  let upstream=$state('');
  let match=$state('suffix');
  let mode=$state('append');
  let upstreams=$derived((draft.config.upstreams||[]).map(u=>String(u.id||'')).filter(Boolean));
  $effect(()=>{if(!upstreams.includes(upstream))upstream=upstreams[0]||'';});
  let stopped=false;
  onMount(()=>()=>{stopped=true;});

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
  <button id="saveButton" type="button" class="button primary" disabled={busy} onclick={()=>perform('save')}>{rfUi("Сохранить изменения","Save changes")}</button>
{:else if kind==='reload'}
  <button id="reloadButton" type="button" class="button" disabled={busy} onclick={()=>perform('reload')}>{rfUi("↻ Обновить","↻ Refresh")}</button>
{:else if kind==='import'}
  <form id="importForm" class="form-grid" onsubmit={e=>{e.preventDefault();void perform('import');}}>
    <label class="span-all"><span>{rfUi("Домены — по одному на строку, комментарии через #","One domain per line; use # for comments")}</span><textarea name="text" rows="6" required placeholder="example.com&#10;another.net" bind:value={domainText}></textarea></label>
    <label><span>Upstream</span><select name="upstream" id="importUpstream" required bind:value={upstream}>
      {#each upstreams as item (item)}<option value={item}>{item}</option>{/each}
    </select></label>
    <label><span>{rfUi("Совпадение","Match type")}</span><select name="match" bind:value={match}><option value="suffix">{rfUi("Домен и поддомены","Domain and subdomains")}</option><option value="exact">{rfUi("Точное","Exact match")}</option><option value="subdomains">{rfUi("Только поддомены","Subdomains only")}</option></select></label>
    <label><span>{rfUi("Режим","Mode")}</span><select name="mode" bind:value={mode}><option value="append">{rfUi("Дополнить","Append")}</option><option value="replace">{rfUi("Заменить правила","Replace rules")}</option></select></label>
    <div class="form-actions"><button class="button" type="submit" disabled={busy||!upstream}>{rfUi("Предпросмотр импорта","Import preview")}</button></div>
  </form>
  <pre class="console" id="importResult">{outcome}</pre>
{/if}
{#if error}<p class="notice bad" role="alert">{error}</p>{/if}
