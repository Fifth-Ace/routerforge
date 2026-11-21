<script>
  import { rfUi } from './rf-localization.js';
  import {untrack} from 'svelte';
  import {simple} from './DnsRuntimeStore.svelte.js';
  import {parseDomainBatch,previewDomainBatch,mergeDomainFile,exportDomainText} from './DnsRuntimeP92Logic.js';
  let message=$state(rfUi('Предпросмотр не меняет текущие правила DNS.',"Preview does not change current DNS rules."));
  let messageState=$state('info');
  let count=$state(rfUi('0 доменов',"0 domains"));
  let preview=$state(rfUi('Вставь домены выше — здесь появится предпросмотр.',"Enter domains above to display a preview."));
  let busy=$state(false);
  let fileInput;
  const read=()=>simple.domains;
  function notify(s,state='info'){message=String(s||'');messageState=state;}
  function refresh(){
    const result=previewDomainBatch(read());
    count=result.count;preview=result.preview;
    return result.ok;
  }
  function setText(s){
    simple.domains=s;refresh();
  }
  function check(){
    const ok=refresh();
    notify(ok?rfUi('Проверка списка завершена. Добавь домены в черновик перед сохранением.',"List checked. Add domains to the draft before saving."):rfUi('Исправь ошибки в списке.',"Correct the errors in the list."),ok?'success':'error');
  }
  function clear(){
    if(!read().trim())return;
    if(!confirm(rfUi('Очистить только текстовое поле? Уже добавленные правила останутся.',"Clear the text field only? Already added rules will remain.")))return;
    setText('');notify('Поле очищено. Правила DNS Router не изменены.');
  }
  function canonicalize(action){
    const parsed=parseDomainBatch(read());
    if(parsed.error){refresh();notify(parsed.error,'error');return;}
    setText(parsed.domains.join('\n'));
    notify(action==='normalize'?rfUi('Регистр, пробелы и дубликаты нормализованы.',"Case, whitespace and duplicates normalized."):rfUi('Повторяющиеся домены удалены.',"Duplicate domains removed."));
  }
  async function importFile(){
    const file=fileInput?.files?.[0];if(!file)return;
    busy=true;
    try{
      // Reject before calling file.text() to avoid reading oversized uploads.
      if(file.size>256*1024)throw Error(rfUi('Файл слишком большой (максимум 256 КБ).',"File too large (maximum 256 KB)."));
      const raw=await file.text();
      const result=mergeDomainFile(read(),raw,file.size);
      setText(result);notify('Импортировано. Проверь список перед добавлением.','success');
    }catch(e){notify(rfUi('Импорт: ',"Import:")+String(e?.message||e),'error');}
    finally{if(fileInput)fileInput.value='';busy=false;}
  }
  function exportFile(){
    let output;
    try{output=exportDomainText(read());}
    catch(e){refresh();notify(String(e?.message||e),'error');return;}
    const blob=new Blob([output],{type:'text/plain;charset=utf-8'});
    const url=URL.createObjectURL(blob);
    try{
      const a=document.createElement('a');a.href=url;a.download='routerforge-dns-domains.txt';
      a.hidden=true;document.body.appendChild(a);a.click();a.remove();
      notify(rfUi('Список экспортирован.',"List exported."));
    }finally{URL.revokeObjectURL(url);}
  }
  $effect(()=>{void simple.domains;untrack(refresh);});
</script>
<div data-p92-native="true">
  <div class="dr-p92-tools" aria-label={rfUi("Действия со списком доменов","Domain list actions")}>
    <button type="button" class="button" id="drP92Check" disabled={busy} onclick={check}>{rfUi("✓ Проверить","✓ Check")}</button>
    <button type="button" class="button" id="drP92Clear" disabled={busy} onclick={clear}>{rfUi("Очистить","Clear")}</button>
    <button type="button" class="button" id="drP92Deduplicate" disabled={busy} onclick={()=>canonicalize('deduplicate')}>{rfUi("Удалить дубликаты","Remove duplicates")}</button>
    <button type="button" class="button" id="drP92Normalize" disabled={busy} onclick={()=>canonicalize('normalize')}>{rfUi("Нормализовать","Normalize")}</button>
    <button type="button" class="button" id="drP92Import" disabled={busy} onclick={()=>fileInput?.click()}>{rfUi("Импорт","Import")}</button>
    <button type="button" class="button" id="drP92Export" disabled={busy} onclick={exportFile}>{rfUi("Экспорт","Export")}</button>
    <input id="drP92File" bind:this={fileInput} class="dr-p92-import" type="file" accept=".txt,text/plain" aria-label={rfUi("Импорт доменов","Import domains")} onchange={importFile}/>
  </div>
  <p class="dr-p92-message" id="drP92Message" role="status" aria-live="polite" data-state={messageState}>{message}</p>
  <div class="dr-p92-preview" aria-label={rfUi("Предпросмотр списка","List preview")}>
    <div class="dr-p92-preview-head"><strong>{rfUi("Предпросмотр","Preview")}</strong><span id="drP92Count">{count}</span></div>
    <pre id="drP92Preview">{preview}</pre>
  </div>
</div>
