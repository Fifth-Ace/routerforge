<script>
  let {snapshot={open:false,rows:[],total:0,ok:0,fail:0,progress:0,busy:false},onAction=()=>{}}=$props();
</script>
<div id="checkModal" class="modal" class:open={snapshot.open} data-svelte-owned="nfqws-domain-batch-check" role="dialog" aria-label="Проверка доступности доменов" aria-modal="true" tabindex="-1" onkeydown={e=>{if(e.key==='Escape')onAction('close')}} onclick={e=>{if(e.target===e.currentTarget)onAction('close')}}>
  <div class="dialog">
    <div class="dialog-head"><strong>Проверка доступности доменов</strong><button type="button" onclick={()=>onAction('close')}>Закрыть</button></div>
    <div class="dialog-body">
      <div class="domain-summary"><div class="domain-stat"><span>Всего</span><strong>{snapshot.total}</strong></div><div class="domain-stat"><span>Доступны</span><strong class="good-text">{snapshot.ok}</strong></div><div class="domain-stat"><span>Недоступны</span><strong class="bad-text">{snapshot.fail}</strong></div><div class="domain-stat"><span>Прогресс</span><strong>{snapshot.progress}%</strong></div></div>
      <div class="progress"><span style:width={`${snapshot.progress}%`}></span></div>
      <div class="domain-results">
        {#each snapshot.rows as row (row.domain)}
          <div class="domain-row" class:pending={row.state==='pending'} class:ok={row.state==='ok'} class:fail={row.state==='fail'}><span>{row.domain}</span><span class="result" class:ok={row.state==='ok'} class:fail={row.state==='fail'}>{row.state==='pending'?'WAIT':row.state==='ok'?`OK ${row.code}`:'FAIL'}</span></div>
        {/each}
      </div>
    </div>
    <div class="dialog-foot"><button type="button" disabled={!snapshot.busy} onclick={()=>onAction('stop')}>Остановить</button><button type="button" class="primary" disabled={snapshot.busy} onclick={()=>onAction('retry')}>Проверить снова</button></div>
  </div>
</div>
