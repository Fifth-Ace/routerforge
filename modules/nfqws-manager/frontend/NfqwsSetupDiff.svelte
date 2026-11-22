<script>
  let {snapshot={open:false,lines:[]},onAction=()=>{}}=$props();
</script>
<div id="diffModal" class="modal" class:open={snapshot.open} data-svelte-owned="nfqws-setup-diff" role="dialog" aria-modal="true" aria-label="Изменения nfqws2.conf" tabindex="-1"
  onkeydown={e=>{if(e.key==='Escape')onAction('close')}} onclick={e=>{if(e.target===e.currentTarget)onAction('close')}}>
  <div class="dialog"><div class="dialog-head"><strong>Изменения nfqws2.conf</strong><button type="button" onclick={()=>onAction('close')}>Закрыть</button></div>
    <pre class="diff">{#if !snapshot.lines.length}<span class="same">No changes</span>{:else}{#each snapshot.lines as line,i (i)}<span class={line.c}>{line.t}</span>{'\n'}{/each}{/if}</pre>
    <div class="dialog-foot"><button type="button" onclick={()=>onAction('close')}>Отмена</button><button type="button" class="primary" onclick={()=>onAction('apply')}>Сохранить и применить</button></div>
  </div>
</div>
