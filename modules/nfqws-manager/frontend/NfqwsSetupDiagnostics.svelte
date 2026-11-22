<script>
  let {value={items:[],state:'CHECK',stateClass:'warn',message:'',bad:0,warn:0,good:0}}=$props();
</script>
<div data-svelte-owned="nfqws-setup-diagnostics">
  <div class="diagnostic-summary">
    <div class="diagnostic-state" class:bad={value.stateClass==='bad'} class:warn={value.stateClass==='warn'} class:good={value.stateClass==='good'}>
      <span>{value.state}</span><strong>{value.message}</strong>
    </div>
    <div class="diagnostic-counts"><span class="bad">Ошибки <b>{value.bad}</b></span><span class="warn">Предупреждения <b>{value.warn}</b></span><span class="good">Проверки <b>{value.good}</b></span></div>
  </div>
  <div class="diagnostics">{#each value.items as item,i (i)}<div class="diag" class:good={item[0]==='good'} class:warn={item[0]==='warn'} class:bad={item[0]==='bad'}><span class="diag-mark">{item[0]==='good'?'✓':'!'}</span><span>{item[1]}</span></div>{/each}</div>
</div>
