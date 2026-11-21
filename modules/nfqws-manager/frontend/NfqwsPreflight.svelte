<script>
  import { rfUi } from './rf-localization.js';
  let {view={state:'BLOCKED',dirty:false,busy:false,bad:0,warn:0,good:0,groups:[]},onRefresh=()=>{},onJump=()=>{}}=$props();
</script>
<div class="preflight-shell" data-svelte-owned="nfqws-preflight">
  <div class="preflight-hero">
    <div><div class="preflight-title-row"><h2 style="margin:0">{rfUi("Проверка готовности","Readiness checks")}</h2><span class={`preflight-verdict ${view.state==='READY'?'ready':view.state==='BLOCKED'?'blocked':'check'}`}>{view.state}</span><span class={`config-badge ${view.dirty?'empty':'active'}`}>{view.dirty?'DRAFT':'ACTIVE'}</span></div><div class="preflight-sub">{view.subtitle||'Читаю статус и конфигурацию…'}</div></div>
    <div><div class="preflight-counts"><div class="preflight-count"><span>{rfUi("Ошибки","Errors")}</span><strong class="bad-text">{view.bad||0}</strong></div><div class="preflight-count"><span>{rfUi("Проверить","Review")}</span><strong class="warn-text">{view.warn||0}</strong></div><div class="preflight-count"><span>PASS</span><strong class="good-text">{view.good||0}</strong></div></div><div class="toolbar" style="justify-content:flex-end;margin-top:8px"><button type="button" disabled={view.busy} onclick={onRefresh}>{rfUi("Обновить runtime","Refresh status")}</button></div></div>
  </div>
  <div class="preflight-groups">
    {#each view.groups||[] as group (group.group)}
      <section class="preflight-group"><div class="preflight-group-head"><span class="preflight-group-title">{group.group}</span><span class="preflight-group-meta">{group.summary}</span></div>
        <div class="preflight-items">
          {#each group.items as item,i (`${group.group}:${i}`)}
            <div class={`preflight-item ${item.severity}`}><span class="preflight-mark">{item.severity==='good'?'✓':item.severity==='bad'?'!':item.severity==='warn'?'!':'i'}</span><div><div class="preflight-item-title">{item.title}</div><div class="preflight-item-detail">{item.detail}</div></div>{#if item.tab}<button type="button" class="preflight-jump" onclick={()=>onJump(item.tab)}>{rfUi("Открыть","Open")}</button>{/if}</div>
          {/each}
        </div>
      </section>
    {:else}<div class="analysis-empty">{view.busy?rfUi('Обновляю состояние…','Refreshing status…'):rfUi('Нет данных о текущем состоянии. Готовность пока неизвестна.','Current status is unavailable. Readiness has not been determined.')}</div>{/each}
  </div>
  <div class="preflight-note">{rfUi("Это preflight конфигурации и доступных runtime-сигналов RouterForge. Он не доказывает фактический захват конкретного пакета NFQUEUE и не заменяет live-тест стратегии.","This readiness check covers configuration and available service signals. It does not prove that a specific packet passed through NFQUEUE and does not replace a live strategy test.")}</div>
</div>
