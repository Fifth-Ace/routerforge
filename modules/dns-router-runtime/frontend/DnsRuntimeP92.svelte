<script>
  import {untrack} from 'svelte';
  import {simple} from './DnsRuntimeStore.svelte.js';
  import {parseDomainBatch,previewDomainBatch,mergeDomainFile,exportDomainText} from './DnsRuntimeP92Logic.js';
  let message=$state('Предпросмотр не меняет текущие правила DNS.');
  let messageState=$state('info');
  let count=$state('0 доменов');
  let preview=$state('Вставь домены выше — здесь появится предпросмотр.');
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
    notify(ok?'Проверка списка завершена. Добавь домены в черновик перед сохранением.':'Исправь ошибки в списке.',ok?'success':'error');
  }
  function clear(){
    if(!read().trim())return;
    if(!confirm('Очистить только текстовое поле? Уже добавленные правила останутся.'))return;
    setText('');notify('Поле очищено. Правила DNS Router не изменены.');
  }
  function canonicalize(action){
    const parsed=parseDomainBatch(read());
    if(parsed.error){refresh();notify(parsed.error,'error');return;}
    setText(parsed.domains.join('\n'));
    notify(action==='normalize'?'Регистр, пробелы и дубликаты нормализованы.':'Повторяющиеся домены удалены.');
  }
  async function importFile(){
    const file=fileInput?.files?.[0];if(!file)return;
    busy=true;
    try{
      // Reject before calling file.text() to avoid reading oversized uploads.
      if(file.size>256*1024)throw Error('Файл слишком большой (максимум 256 КБ).');
      const raw=await file.text();
      const result=mergeDomainFile(read(),raw,file.size);
      setText(result);notify('Импортировано. Проверь список перед добавлением.','success');
    }catch(e){notify('Импорт: '+String(e?.message||e),'error');}
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
      notify('Список экспортирован.');
    }finally{URL.revokeObjectURL(url);}
  }
  $effect(()=>{void simple.domains;untrack(refresh);});
</script>
<div data-p92-native="true">
  <div class="dr-p92-tools" aria-label="Действия со списком доменов">
    <button type="button" class="button" id="drP92Check" disabled={busy} onclick={check}>✓ Проверить</button>
    <button type="button" class="button" id="drP92Clear" disabled={busy} onclick={clear}>Очистить</button>
    <button type="button" class="button" id="drP92Deduplicate" disabled={busy} onclick={()=>canonicalize('deduplicate')}>Удалить дубликаты</button>
    <button type="button" class="button" id="drP92Normalize" disabled={busy} onclick={()=>canonicalize('normalize')}>Нормализовать</button>
    <button type="button" class="button" id="drP92Import" disabled={busy} onclick={()=>fileInput?.click()}>Импорт</button>
    <button type="button" class="button" id="drP92Export" disabled={busy} onclick={exportFile}>Экспорт</button>
    <input id="drP92File" bind:this={fileInput} class="dr-p92-import" type="file" accept=".txt,text/plain" aria-label="Импорт доменов" onchange={importFile}/>
  </div>
  <p class="dr-p92-message" id="drP92Message" role="status" aria-live="polite" data-state={messageState}>{message}</p>
  <div class="dr-p92-preview" aria-label="Предпросмотр списка">
    <div class="dr-p92-preview-head"><strong>Предпросмотр</strong><span id="drP92Count">{count}</span></div>
    <pre id="drP92Preview">{preview}</pre>
  </div>
</div>
