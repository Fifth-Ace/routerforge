<script>
  import {listInspectorView,listAnalysisView} from './NfqwsListToolsModel.js';
  let {inspector={},staticPath={},analysis={},onAction=()=>{}}=$props();
  let filter=$state('all');
  let peer=$state('');
  let cleanupOpen=$state(false);
  const detail=$derived(listInspectorView(inspector.record,inspector.findings));
  const comparison=$derived(listAnalysisView({selected:analysis.selected,findings:analysis.findings,peer,filter}));
  const canCleanup=$derived(!analysis.busy&&!analysis.incomplete&&!analysis.error&&comparison.peer&&comparison.pairTotal>0);
  function action(name,payload){onAction(name,payload);}
  function closeInspector(){action('inspectorClose');}
  function closeAnalysis(){cleanupOpen=false;action('analysisClose');}
</script>
{#if inspector.open}
  <div id="entryInspectorModal" class="modal open" role="presentation" onclick={e=>{if(e.target===e.currentTarget)closeInspector();}}>
    <div class="dialog" role="dialog" aria-modal="true" aria-label="Инспектор записи">
      <div class="dialog-head"><div><strong>Инспектор записи</strong><div class="section-kicker mono">{detail.scope}</div></div><button type="button" onclick={closeInspector}>Закрыть</button></div>
      <div class="dialog-body">
        <div class="inspector-source"><div><div class="inspector-entry">{detail.value||'—'}</div><div class="inspector-source-meta">{detail.meta}</div></div><span class={`config-badge ${detail.excluded?'empty':'active'}`}>{inspector.record?detail.role:'—'}</span></div>
        <div class="inspector-summary"><div class="inspector-stat"><span>Связей</span><strong>{detail.total}</strong></div><div class="inspector-stat"><span>Конфликтов</span><strong class="bad-text">{detail.conflicts}</strong></div><div class="inspector-stat"><span>Списков</span><strong>{detail.files}</strong></div></div>
        <div class="muted small" style="margin-bottom:8px" role="status">{inspector.progress||''}</div>
        {#if inspector.error}<div class="analysis-empty bad-text" role="alert">{inspector.error}</div>
        {:else if inspector.busy}<div class="analysis-empty">Ищу связи только для выбранной записи…</div>
        {:else if detail.rows.length}
          <div class="inspector-results">
            {#each detail.rows as row,i (`${row.file}:${row.line}:${row.kind}:${i}`)}
              <div class={`inspector-row ${row.severity}`}><div class="inspector-kind">{row.kind}</div><div><div class="inspector-value">{row.detail}</div><div class="inspector-meta">{row.file}:{row.line} · {row.severity==='conflict'?'include/exclude conflict':'overlap / redundancy'}</div></div><button class="inspector-open" type="button" onclick={()=>action('openLine',{name:row.file,line:row.line})}>Открыть</button></div>
            {/each}
          </div>
        {:else}<div class="analysis-empty">Связей с другими списками не найдено.</div>{/if}
      </div>
      <div class="dialog-foot"><span class="muted small">Read-only: анализируется только запись под курсором.</span><button type="button" class="primary" disabled={inspector.busy} onclick={()=>action('inspectorRerun')}>Пересчитать</button></div>
    </div>
  </div>
{/if}
{#if staticPath.open}
  <div id="staticPathModal" class="modal open" role="presentation" onclick={e=>{if(e.target===e.currentTarget)action('staticPathClose');}}>
    <div class="dialog" role="dialog" aria-modal="true" aria-label="Effective Static Path">
      <div class="dialog-head"><div><strong>Effective Static Path</strong><div class="section-kicker mono">{staticPath.record?`${staticPath.record.file}:${staticPath.record.line} · active config static analysis`:'—'}</div></div><button type="button" onclick={()=>action('staticPathClose')}>Закрыть</button></div>
      <div class="dialog-body">
        <div class="inspector-source"><div><div class="inspector-entry">{staticPath.record?.value||'—'}</div><div class="inspector-source-meta">{staticPath.record?`${staticPath.record.file}:${staticPath.record.line} · ${String(staticPath.record.type||'').toUpperCase()}`:'—'}</div></div><span class={`config-badge ${staticPath.result?.sourceRole?.impact==='exclude'?'empty':staticPath.result?.sourceRole?.impact==='include'?'active':''}`}>{staticPath.result?.sourceRole?.label||'—'}</span></div>
        <div class={`static-path-verdict ${staticPath.result?.tone||'neutral'}`}><div><strong>{staticPath.result?.verdict||'—'}</strong><div class="static-path-reason">{staticPath.result?.reason||'Статический анализ загружается…'}</div></div><span class="static-path-mode">{staticPath.result?.mode||'MODE ?'}</span></div>
        <div class="muted small" style="margin-bottom:8px" role="status">{staticPath.progress||''}</div>
        {#if staticPath.error}<div class="analysis-empty bad-text" role="alert">{staticPath.error}</div>{/if}
        <div class="static-path-steps">
          {#each staticPath.result?.hits||[] as hit,i (`${hit.file}:${hit.line}:${i}`)}
            <div class={`static-path-step ${hit.effective}`}><div class="static-path-index">{i+1}</div><div><div class={`static-path-role ${hit.effective}`}>{hit.role?.label||'—'}</div><div class="inspector-meta">{hit.relation}</div></div><div><div class="static-path-value">{hit.value}</div><div class="static-path-meta">{hit.file}:{hit.line} · {hit.role?.reason||''}{hit.detail?` · ${hit.detail}`:''}</div></div><button class="static-path-open" type="button" onclick={()=>action('openLine',{name:hit.file,line:hit.line})}>Открыть</button></div>
          {:else}<div class="analysis-empty">{staticPath.busy?'Сопоставляю активные списки…':'Статических совпадений не найдено.'}</div>{/each}
        </div>
        <div class="static-path-footnote">Статический разбор конфигурации и list-файлов. Он не утверждает фактический packet path, состояние NFQUEUE, DNS, conntrack или результат DPI-обхода.</div>
      </div>
      <div class="dialog-foot"><span class="muted small">Read-only · выбранная несохранённая строка учитывается.</span><button type="button" class="primary" disabled={staticPath.busy} onclick={()=>action('staticPathRerun')}>Пересчитать</button></div>
    </div>
  </div>
{/if}
{#if analysis.open}
  <div id="analysisModal" class="modal open" role="presentation" onclick={e=>{if(e.target===e.currentTarget)closeAnalysis();}}>
    <div class="dialog" role="dialog" aria-modal="true" aria-label="Пересечения списков">
      <div class="dialog-head"><div><strong>Пересечения списков</strong><div class="section-kicker mono">{comparison.selected?`${comparison.selected} ↔ найденные пересечения`:'—'}</div></div><button type="button" onclick={closeAnalysis}>Закрыть</button></div>
      <div class="dialog-body">
        <div class="analysis-summary"><div class="analysis-stat"><span>Найдено</span><strong>{comparison.total}</strong></div><div class="analysis-stat"><span>Конфликты</span><strong class="bad-text">{comparison.conflicts}</strong></div><div class="analysis-stat"><span>Пересечения</span><strong class="warn-text">{comparison.warnings}</strong></div><div class="analysis-stat"><span>Списки</span><strong>{comparison.files}</strong></div></div>
        <div class="analysis-filters">
          {#each [['all','Все'],['conflict','Конфликты'],['domain','Домены'],['network','IP / CIDR']] as choice (choice[0])}
            <button type="button" class:active={filter===choice[0]} onclick={()=>filter=choice[0]}>{choice[1]}</button>
          {/each}
        </div>
        <div class="analysis-pairbar"><select class="search analysis-peer" aria-label="Парный список" value={comparison.peer} onchange={e=>{peer=e.currentTarget.value;cleanupOpen=false;}} disabled={!comparison.peers.length||analysis.busy}>
          {#each comparison.peers as name (name)}<option value={name}>{comparison.selected} ↔ {name}</option>{:else}<option value="">Пересечений не найдено</option>{/each}
        </select><button type="button" disabled={!comparison.peer||analysis.busy} onclick={()=>action('openLine',{name:comparison.peer,line:comparison.allRight[0]?.line||1})}>Открыть правый список</button></div>
        <div class="muted small" style="margin-bottom:8px" role="status">{analysis.progress||''}</div>
        {#if analysis.error}<div class="analysis-empty bad-text" role="alert">{analysis.error}</div>{/if}
        {#if analysis.incomplete}<div class="analysis-empty bad-text" role="alert">Часть соседних списков не прочитана. Очистка заблокирована до полного пересчёта.</div>{/if}
        <div class="analysis-commander">
          {#each [{name:comparison.selected,rows:comparison.left},{name:comparison.peer,rows:comparison.right}] as side,i (i)}
            <section class="analysis-pane"><div class="analysis-pane-head"><div class="analysis-pane-title">{side.name||'—'}</div><div class="analysis-pane-count">{side.rows.length} строк</div></div>
              <div class="analysis-pane-lines">
                {#each side.rows as row (row.line)}<div class={`analysis-line ${row.severity}`}><span class="analysis-line-num">{row.line}</span><span class="analysis-line-value">{row.value}</span><span class="analysis-line-badge">{row.kinds}</span></div>
                {:else}<div class="analysis-empty">По текущему фильтру строк нет.</div>{/each}
              </div>
            </section>
          {/each}
        </div>
        <div class="analysis-pair-meta">{comparison.pairMeta}</div>
        {#if cleanupOpen}
          <div id="analysisCleanup" class="analysis-cleanup open"><div class="analysis-cleanup-title">Из какого файла удалить все найденные пересекающиеся строки в текущей паре?</div>
            <div class="analysis-cleanup-actions"><button type="button" class="danger" disabled={!canCleanup||analysis.dirty||!comparison.allLeft.length} onclick={()=>{cleanupOpen=false;action('analysisCleanup',{side:'left',peer:comparison.peer});}}>Удалить из {comparison.selected} ({comparison.allLeft.length})</button><button type="button" class="danger" disabled={!canCleanup||!comparison.allRight.length} onclick={()=>{cleanupOpen=false;action('analysisCleanup',{side:'right',peer:comparison.peer});}}>Удалить из {comparison.peer} ({comparison.allRight.length})</button><button type="button" onclick={()=>cleanupOpen=false}>Отмена</button></div>
            <div class="analysis-cleanup-note">{analysis.dirty?`В ${comparison.selected} есть несохранённые изменения: очистка слева заблокирована.`:'Перед изменением RouterForge создаст safety backup выбранного list-файла.'}</div>
          </div>
        {/if}
      </div>
      <div class="dialog-foot"><span class="muted small">Показываются только строки, участвующие в пересечениях.</span><div class="toolbar"><button type="button" class="danger" disabled={!canCleanup} onclick={()=>cleanupOpen=!cleanupOpen}>Очистить пересечения</button><button type="button" class="primary" disabled={analysis.busy} onclick={()=>{cleanupOpen=false;action('analysisRerun');}}>Пересчитать</button></div></div>
    </div>
  </div>
{/if}
