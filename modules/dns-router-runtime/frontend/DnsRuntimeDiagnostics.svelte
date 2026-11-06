<script>
  import {onMount} from 'svelte';
  let busy=$state(false);
  let error=$state('');
  let shadow=$state(null);
  let resources=$state(null);
  let discovered=$state(null);
  let updated=$state('');
  let stopped=false;
  let sequence=0;
  async function get(path){
    const response=await fetch('../'+path,{method:'GET',cache:'no-store',credentials:'same-origin',headers:{Accept:'application/json'}});
    if(!response.ok)throw Error('HTTP '+response.status+' · '+path);
    if(!(response.headers.get('content-type')||'').toLowerCase().includes('json'))throw Error('Ожидался JSON · '+path);
    return response.json();
  }
  async function refresh(){
    if(busy||stopped)return;
    const call=++sequence;
    busy=true;error='';
    const results=await Promise.allSettled([get('shadow/status'),get('runtime/resources'),get('keenetic/discovery')]);
    if(stopped||sequence!==call)return;
    shadow=results[0].status==='fulfilled'?results[0].value:null;
    resources=results[1].status==='fulfilled'?results[1].value:null;
    discovered=results[2].status==='fulfilled'?results[2].value:null;
    const failures=results.map((x,i)=>x.status==='rejected'?[['Shadow DNS','Ресурсы','Keenetic'][i],String(x.reason?.message||x.reason)]:null).filter(Boolean);
    error=failures.map(x=>x.join(': ')).join(' · ');
    updated=new Date().toLocaleTimeString('ru-RU',{hour12:false});
    busy=false;
  }
  onMount(()=>{void refresh();const id=window.setInterval(()=>{if(!document.hidden && !document.getElementById('dr-panel-diagnostics')?.hidden)void refresh();},15000);return()=>{stopped=true;++sequence;window.clearInterval(id);};});
</script>
<section class="panel" aria-label="Svelte 5: независимая диагностика DNS">
  <div class="panel-head">
    <div><span class="section-kicker">DNS / ДИАГНОСТИКА ТОЛЬКО ЧТЕНИЕ</span><h2>Состояние модулей</h2><p>Отдельные GET-проверки. Без применения, запуска и изменения Keenetic.</p></div>
    <button class="button" type="button" disabled={busy} onclick={refresh}>↻ Проверить</button>
  </div>
  <div class="action-bar" role="status" aria-live="polite">
    <span class="state neutral">Shadow DNS: {shadow ? 'ответ получен' : 'не подтверждён'}</span>
    <span class="state neutral">Ресурсы: {resources ? 'ответ получен' : 'не подтверждены'}</span>
    <span class="state neutral">Keenetic: {discovered ? 'ответ получен' : 'не подтверждён'}</span>
  </div>
  {#if error}<p class="muted">{error}</p>{/if}
  <small class="muted">{updated ? 'Последняя проверка: '+updated : 'Проверка ещё не выполнялась.'} Ответ API не означает, что DNS-трафик реально перехватывается.</small>
</section>
