<script>
  let {mode='basic',page='dns',section='devices',busy=false,onNavigate=()=>{}}=$props();
  const english=document.documentElement.lang==='en'||new URLSearchParams(location.search).get('lang')==='en';
  const t=(ru,en)=>english?en:ru;
  function portal(node){
    const target=document.getElementById('rf-dns-lite-svelte-advanced-nav');
    if(!target)throw Error('P22_NAV_SLOT_MISSING');
    target.replaceChildren(node);
    return {destroy(){if(node.parentNode===target)node.remove();}};
  }
</script>
<div use:portal style="display:contents">
  {#each [{id:'p21Nav',section:'devices',ru:'Устройства',en:'Devices'}, {id:'p22NavGroups',section:'groups',ru:'Группы',en:'Groups'}, {id:'p22NavRules',section:'rules',ru:'Правила домена',en:'Domain rules'}] as tab (tab.id)}
    <button type="button" id={tab.id} class={tab.section==='devices'?'tab p21-advanced-only':'tab p22-advanced-tab'} class:selected={page==='devices'&&section===tab.section} aria-current={page==='devices'&&section===tab.section?'page':undefined} disabled={busy} onclick={()=>onNavigate(tab.section)}>{t(tab.ru,tab.en)}</button>
  {/each}
</div>
