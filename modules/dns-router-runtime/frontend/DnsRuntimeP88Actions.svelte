<script>
  import {simple,draft,shell,choices,nativeChoices,stageDomains,cancelEditing,feedback} from './DnsRuntimeStore.svelte.js';
  import {configFlow,scanNative} from './DnsRuntimeCommands.js';
  import {apiRequest} from './DnsRuntimeApi.js';
  import {nativeProbeTarget,probeMessage} from './DnsRuntimeP88ActionLogic.js';
  let {scope}=$props();
  let probing=$state(false);
  let saving=$state(false);
  let scanning=$state(false);
  let probeText=$state('DNS-запрос ещё не выполнялся.');
  let available=$derived(Boolean(choices().length));
  let dirty=$derived(draft.dirty);
  let editing=$derived(Boolean(simple.editingId));
  async function scan(){
    if(scanning)return;scanning=true;
    try{await scanNative();}finally{scanning=false;}
  }
  function add(){
    try{stageDomains(message=>window.confirm(message));}
    catch(e){simple.status=String(e?.message||e);feedback(simple.status);}
  }
  function cancel(){
    if(simple.domains.trim()&&!window.confirm('Выйти из редактирования списка без сохранения текста?'))return;
    cancelEditing();
  }
  async function save(){
    if(saving)return;saving=true;
    try{await configFlow.save();}catch(e){simple.status=String(e?.message||e);feedback(simple.status);}
    finally{saving=false;}
  }
  function advanced(){simple.editingId=null;shell.mode='advanced';}
  async function probe(){
    if(probing)return;probing=true;
    try{
      const path=nativeProbeTarget(simple.selected,nativeChoices());
      probeText='Проверяю DNS-запросом example.com…';
      probeText=probeMessage(await apiRequest(path));
    }catch(e){probeText='Ошибка проверки: '+String(e?.message||e);}
    finally{probing=false;}
  }
</script>
{#if scope==='scan'}
  <button type="button" class="button" id="drSimpleScan" disabled={scanning} onclick={scan}>↻ Обновить политики и DNS</button>
{:else if scope==='probe'}
  <div class="dr-simple-probe" data-p88-actions="probe"><button type="button" class="button" id="drSimpleProbe" disabled={probing||!simple.probeEligible} onclick={probe}>Проверить DNS</button><span id="drSimpleProbeResult">{simple.probeNotice||probeText}</span></div>
{:else if scope==='draft'}
  <button type="button" class="button primary" id="drSimpleAdd" disabled={!available} onclick={add}>{editing ? '✓ Заменить список в черновике' : '+ Добавить домены'}</button>
  {#if editing}
    <button type="button" class="button" id="drP10CancelListEdit" onclick={cancel}>Отменить редактирование</button>
  {/if}
  <button type="button" class="button primary" id="drSimpleSave" disabled={!dirty||saving} onclick={save}>Сохранить черновик</button>
{:else if scope==='advanced'}
  <button id="drSimpleAdvanced" type="button" class="button" onclick={advanced}>Открыть расширенные настройки</button>
{/if}
