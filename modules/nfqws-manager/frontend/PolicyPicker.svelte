<script>
  import { rfUi } from './rf-localization.js';
  import { onDestroy, untrack } from 'svelte';
  let { consumer, disabled = false, onChoose = () => {} } = $props();
  let opened = $state(false), busy = $state(false), error = $state(''), groups = $state([]), etag = $state(''), chosen = $state(''), preview = $state(null), target = $state('');
  let serial = 0, alive = true, controller = null;
  const base = '/api/platform/foundation/consumers';
  onDestroy(() => { alive = false; serial++; controller?.abort(); });
  function reset(){serial++;controller?.abort();busy=false;error='';groups=[];etag='';chosen='';preview=null;target='';opened=false;}
  let previous = null;
  $effect(()=>{const mode=consumer;untrack(()=>{if(mode!==previous){previous=mode;reset();}})});
  async function request(params){
    controller?.abort(); const c=new AbortController();controller=c;
    const timer=setTimeout(()=>c.abort(),10000);
    try{const r=await fetch(base+'?'+new URLSearchParams(params),{method:'GET',cache:'no-store',credentials:'same-origin',redirect:'error',signal:c.signal});
      if(!r.ok)throw Error('HTTP '+r.status);if(!r.headers.get('content-type')?.includes('application/json'))throw Error('INVALID_RESPONSE');
      const data=await r.json();if(data.schemaVersion!==1||!/^[a-f0-9]{64}$/.test(data.etag))throw Error('INVALID_RESPONSE');return data;
    }finally{clearTimeout(timer)}
  }
  async function load(){if(disabled||busy)return;reset();opened=true;busy=true;const id=++serial,mode=consumer;
    try{const d=await request({consumer:mode});if(!alive||id!==serial)return;if(!Array.isArray(d.groups)||d.groups.length>100)throw Error('INVALID_GROUPS');groups=d.groups;etag=d.etag;}
    catch(e){if(alive&&id===serial)error=String(e.message||e)}finally{if(alive&&id===serial)busy=false;}}
  async function inspect(){if(disabled||busy||!chosen||!etag)return;const id=++serial,mode=consumer,key=chosen;busy=true;error='';preview=null;target='';
    try{const d=await request({consumer:mode,id:key,etag});if(!alive||id!==serial)return;
      if(d.etag!==etag||d.consumer!==mode||d.group?.id!==key||!Array.isArray(d.values)||!Array.isArray(d.blocked)||d.values.length>4096||d.values.some(v=>typeof v!=='string'||/[\s\\]/.test(v)))throw Error('INVALID_PREVIEW');preview=d;}
    catch(e){if(alive&&id===serial)error=String(e.message||e)}finally{if(alive&&id===serial)busy=false;}}
  async function choose(){if(disabled||busy||!preview||preview.blocked.length||!preview.values.length)return;
    const old=preview, id=++serial,mode=consumer,selected=target;busy=true;error='';
    try{const d=await request({consumer:mode,id:old.group.id,etag:old.etag});if(!alive||id!==serial||disabled)return;
      if(d.etag!==old.etag||d.consumer!==mode||d.group?.id!==old.group.id||JSON.stringify(d.values)!==JSON.stringify(old.values)||d.blocked?.length!==0)throw Error('STALE_PREVIEW');
      if(mode==='network'&&!old.values.includes(selected))throw Error('SELECT_TARGET');
      if(onChoose(mode==='network'?[selected]:old.values)===false)throw Error('DRAFT_NOT_CHANGED');reset();}
    catch(e){if(alive&&id===serial)error=String(e.message||e)}finally{if(alive&&id===serial)busy=false;}}
</script>
<div class="rf-policy-picker">
  <button type="button" disabled={disabled||busy} onclick={load}>{rfUi('Из общего каталога','From shared catalogue')}</button>
  {#if opened}
    <button type="button" onclick={reset}>{rfUi('Закрыть','Close')}</button>
    <label>{rfUi('Группа','Group')} <select bind:value={chosen} disabled={disabled||busy} onchange={()=>{preview=null;target='';error=''}}><option value="">—</option>{#each groups as g (g.id)}<option value={g.id}>{g.name} · r{g.revision}</option>{/each}</select></label>
    <button type="button" onclick={inspect} disabled={disabled||busy||!chosen}>{rfUi('Предпросмотр','Preview')}</button>
    {#if busy}<small>{rfUi('Загрузка…','Loading…')}</small>{/if}
    {#if !busy&&!groups.length&&!error}<small>{rfUi('Нет подходящих сохранённых групп.','No matching saved groups.')}</small>{/if}
    {#if error}<small role="alert">{error} · {rfUi('Обновите каталог.','Reload catalogue.')}</small>{/if}
    {#if preview}
      {#if preview.blocked.length}<small role="alert">{rfUi('Группа не поддерживается целиком: ','Group cannot be projected in full: ')}{preview.blocked.join(', ')}</small>
      {:else}
        <small>{rfUi('Элементов: ','Entries: ')}{preview.values.length} · r{preview.group.revision}</small>
        {#if consumer==='network'}<label>{rfUi('Узел','Host')} <select bind:value={target} disabled={busy||disabled}><option value="">—</option>{#each preview.values as v}<option value={v}>{v}</option>{/each}</select></label>
        {:else}<pre>{preview.values.slice(0,20).join('\n')}{preview.values.length>20?'\n…':''}</pre>{/if}
        <button type="button" disabled={disabled||busy||!preview.values.length||(consumer==='network'&&!target)} onclick={choose}>{consumer==='network'?rfUi('Подставить узел','Use host'):rfUi('Добавить в черновик','Append to draft')}</button>
        <small>{rfUi('Без сохранения, применения и запуска проверки.','No save, apply or probe execution.')}</small>
      {/if}
    {/if}
  {/if}
</div>
<style>
  .rf-policy-picker{display:flex;gap:8px;flex-wrap:wrap;align-items:center;margin:8px 0;min-width:0}.rf-policy-picker label{display:flex;gap:6px;align-items:center;max-width:100%}.rf-policy-picker select{max-width:100%;min-width:0}.rf-policy-picker pre{flex-basis:100%;max-height:180px;overflow:auto;white-space:pre-wrap;overflow-wrap:anywhere}.rf-policy-picker small{overflow-wrap:anywhere}
</style>
